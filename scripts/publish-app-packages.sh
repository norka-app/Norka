#!/bin/sh
# Пушит cask приложения и манифест Scoop в tap-репозитории.
#
# Берёт текст из packaging/homebrew/norka.rb и packaging/scoop/norka.json,
# версию и хеши — из SHA256SUMS указанного выпуска, ссылки — на norka.dmg
# и norka.exe того же тега. Пишет только Casks/norka.rb и bucket/norka.json.
# Файлы norka-cli, файлы выпуска и SHA256SUMS не меняет.
#
# Использование: scripts/publish-app-packages.sh <tag>
# Тег — v1.6.1 или 1.6.1. Пустой TAP_GITHUB_TOKEN пропускает push и
# завершается успешно. scripts/publish-app-packages.sh --check гоняет
# локальную проверку без сети и без GitHub.
set -eu

ROOT="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"

HOMEBREW_REPO="${NORKA_HOMEBREW_REPO:-https://github.com/norka-app/homebrew-tap.git}"
SCOOP_REPO="${NORKA_SCOOP_REPO:-https://github.com/norka-app/scoop-bucket.git}"
REPO="${NORKA_REPO:-norka-app/Norka}"
HOMEBREW_DEST="Casks/norka.rb"
SCOOP_DEST="bucket/norka.json"

STAGE=""
CRED_HELPER=""

cleanup() {
  if [ -n "$STAGE" ]; then
    rm -rf "$STAGE"
  fi
  if [ -n "$CRED_HELPER" ]; then
    rm -f "$CRED_HELPER"
  fi
}
trap cleanup EXIT

die() {
  echo "publish-app-packages.sh: $*" >&2
  exit 1
}

valid_version() {
  version="$1"
  case "$version" in
    *[!0-9A-Za-z.+-]*)
      die "недопустимая версия: ${version}"
      ;;
  esac
  case "$version" in
    [0-9]*.[0-9]*.[0-9]*)
      ;;
    *)
      die "версия должна быть вроде 1.6.1, получено: ${version}"
      ;;
  esac
}

render_packages() {
  sums="$1"
  version="$2"
  tag="$3"
  out_cask="$4"
  out_scoop="$5"
  src_cask="$ROOT/packaging/homebrew/norka.rb"
  src_scoop="$ROOT/packaging/scoop/norka.json"
  if [ ! -f "$src_cask" ] || [ ! -f "$src_scoop" ]; then
    die "нет образца cask или манифеста Scoop"
  fi
  if [ ! -f "$sums" ]; then
    die "нет файла SHA256SUMS: ${sums}"
  fi
  python3 - "$src_cask" "$src_scoop" "$sums" "$version" "$tag" "$out_cask" "$out_scoop" "$REPO" <<'PY'
import re
import sys
from pathlib import Path

src_cask, src_scoop, sums_path, version, tag, out_cask, out_scoop, repo = sys.argv[1:]


def fail(message: str) -> None:
    print(f"publish-app-packages.sh: {message}", file=sys.stderr)
    raise SystemExit(1)


def sha256_of(name: str) -> str:
    found = []
    for raw in Path(sums_path).read_text(encoding="utf-8").splitlines():
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        parts = line.split()
        if len(parts) != 2:
            fail(f"строка SHA256SUMS должна быть «хеш  имя»: {raw!r}")
        digest, filename = parts
        if filename == name:
            found.append(digest.lower())
    if len(found) != 1:
        fail(f"в SHA256SUMS имя {name} встречается {len(found)} раз, нужна одна строка")
    digest = found[0]
    if re.fullmatch(r"[0-9a-f]{64}", digest) is None:
        fail(f"хеш {name} не похож на SHA256")
    return digest


def must_sub(text: str, pattern: str, repl, label: str) -> str:
    new, count = re.subn(pattern, repl, text, count=1, flags=re.M)
    if count != 1:
        fail(f"{label}: ожидалась одна замена, получилось {count}")
    return new


exe_hash = sha256_of("norka.exe")
dmg_hash = sha256_of("norka.dmg")
exe_url = f"https://github.com/{repo}/releases/download/{tag}/norka.exe"
dmg_url = f"https://github.com/{repo}/releases/download/v#{{version}}/norka.dmg"

cask = Path(src_cask).read_text(encoding="utf-8")
cask = must_sub(
    cask,
    r'^(  version ")[^"]+(")',
    lambda m, version=version: f"{m.group(1)}{version}{m.group(2)}",
    "cask version",
)
cask = must_sub(
    cask,
    r'^(  sha256 ")[0-9A-Fa-f]{64}(")',
    lambda m, dmg_hash=dmg_hash: f"{m.group(1)}{dmg_hash}{m.group(2)}",
    "cask sha256",
)
if f'url "{dmg_url}"' not in cask:
    fail(f"cask не содержит ссылку на {dmg_url}")
if 'app "norka.app"' not in cask or "com.apple.quarantine" not in cask:
    fail("cask потерял app или caveats про карантин")

scoop = Path(src_scoop).read_text(encoding="utf-8")
scoop = must_sub(
    scoop,
    r'("version": ")[^"]+(")',
    lambda m, version=version: f"{m.group(1)}{version}{m.group(2)}",
    "scoop version",
)
scoop = must_sub(
    scoop,
    r'(https://github\.com/' + re.escape(repo) + r'/releases/download/)v[0-9][^"\s]*(/norka\.exe)',
    lambda m, tag=tag: f"{m.group(1)}{tag}{m.group(2)}",
    "scoop url",
)
scoop = must_sub(
    scoop,
    r'("hash": ")[0-9A-Fa-f]{64}(")',
    lambda m, exe_hash=exe_hash: f"{m.group(1)}{exe_hash}{m.group(2)}",
    "scoop hash",
)
if exe_url not in scoop:
    fail(f"манифест Scoop не содержит ссылку на {exe_url}")
if "v$version" not in scoop:
    fail("манифест Scoop потерял autoupdate v$version")
if '"bin": "norka.exe"' not in scoop:
    fail("манифест Scoop потерял bin norka.exe")

Path(out_cask).write_text(cask, encoding="utf-8")
Path(out_scoop).write_text(scoop, encoding="utf-8")
print(f"версия {version}")
print(f"norka.dmg {dmg_hash}")
print(f"norka.exe {exe_hash}")
print(f"url {exe_url}")
PY
}

prepare_release() {
  tag_arg="$1"
  version="${tag_arg#v}"
  valid_version "$version"
  tag="v${version}"
  STAGE="$(mktemp -d)"
  if [ -n "${NORKA_SHA256SUMS:-}" ]; then
    cp "$NORKA_SHA256SUMS" "$STAGE/SHA256SUMS"
  else
    url="https://github.com/${REPO}/releases/download/${tag}/SHA256SUMS"
    if ! curl -fsSL --retry 3 --retry-delay 2 "$url" -o "$STAGE/SHA256SUMS"; then
      die "не скачался ${url}"
    fi
  fi
  render_packages "$STAGE/SHA256SUMS" "$version" "$tag" "$STAGE/norka.rb" "$STAGE/norka.json"
  if [ "${NORKA_SKIP_ASSET_CHECK:-}" != "1" ]; then
    for name in norka.dmg norka.exe; do
      url="https://github.com/${REPO}/releases/download/${tag}/${name}"
      ok=0
      attempt=1
      while [ "$attempt" -le 4 ]; do
        if curl -fsIL -o /dev/null "$url"; then
          ok=1
          break
        fi
        if [ "$attempt" -eq 4 ]; then
          break
        fi
        sleep "$attempt"
        attempt=$((attempt + 1))
      done
      if [ "$ok" -ne 1 ]; then
        die "файл выпуска не открывается: ${url}"
      fi
      echo "файл выпуска на месте: ${url}"
    done
  fi
  TAG="$tag"
}

write_cred_helper() {
  CRED_HELPER="$(mktemp)"
  cat >"$CRED_HELPER" <<'EOF'
#!/bin/sh
while IFS= read -r line; do
  [ -z "$line" ] && break
done
printf 'username=x-access-token\n'
printf 'password=%s\n' "$TAP_GITHUB_TOKEN"
printf '\n'
EOF
  chmod 700 "$CRED_HELPER"
}

git_auth() {
  git -c credential.helper= -c "credential.helper=${CRED_HELPER}" "$@"
}

publish_one() {
  url="$1"
  dest="$2"
  src="$3"
  label="$4"
  work="$(mktemp -d "$STAGE/repo.XXXXXX")"
  echo "клонирую ${label}"
  git_auth clone --quiet --depth 50 --branch main "$url" "$work"
  mkdir -p "$work/$(dirname "$dest")"
  cp "$src" "$work/$dest"
  git -C "$work" add -- "$dest"
  if git -C "$work" diff --cached --quiet; then
    echo "без изменений: ${label} ${dest}"
    return 0
  fi
  git -C "$work" config user.name "github-actions[bot]"
  git -C "$work" config user.email "41898282+github-actions[bot]@users.noreply.github.com"
  git -C "$work" commit -m "norka ${TAG}"
  attempt=1
  while [ "$attempt" -le 4 ]; do
    if git_auth -C "$work" push origin HEAD:main; then
      echo "отправлено: ${label} ${dest}"
      return 0
    fi
    if [ "$attempt" -eq 4 ]; then
      die "push ${label} не прошёл"
    fi
    sleep "$attempt"
    git_auth -C "$work" fetch origin main
    git_auth -C "$work" rebase origin/main
    attempt=$((attempt + 1))
  done
}

run_check() {
  (
    set -eu
    tmp="$(mktemp -d)"
    trap 'rm -rf "$tmp"' EXIT

    dmg_hash="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
    exe_hash="fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
    printf '%s  norka.dmg\n%s  norka.exe\n' "$dmg_hash" "$exe_hash" >"$tmp/SHA256SUMS"

    brew_seed="$tmp/brew-seed"
    scoop_seed="$tmp/scoop-seed"
    brew_bare="$tmp/brew.git"
    scoop_bare="$tmp/scoop.git"
    git init -b main "$brew_seed" >/dev/null
    git init -b main "$scoop_seed" >/dev/null
    mkdir -p "$brew_seed/Casks" "$scoop_seed/bucket"
    printf 'cask "norka-cli" do\n  # norka-cli-marker\nend\n' >"$brew_seed/Casks/norka-cli.rb"
    printf '{ "version": "0", "name": "norka-cli-marker" }\n' >"$scoop_seed/bucket/norka-cli.json"
    for seed in "$brew_seed" "$scoop_seed"; do
      git -C "$seed" config user.name "norka-test"
      git -C "$seed" config user.email "norka-test@example.com"
      git -C "$seed" add .
      git -C "$seed" commit -m "init" >/dev/null
    done
    git init -b main --bare "$brew_bare" >/dev/null
    git init -b main --bare "$scoop_bare" >/dev/null
    git -C "$brew_seed" remote add origin "file://${brew_bare}"
    git -C "$scoop_seed" remote add origin "file://${scoop_bare}"
    git -C "$brew_seed" push origin main >/dev/null
    git -C "$scoop_seed" push origin main >/dev/null

    case "$brew_bare$scoop_bare" in
      *github.com*)
        echo "publish-app-packages.sh: проверка не должна ходить на GitHub" >&2
        exit 1
        ;;
    esac

    before_templates="$(sha256sum "$ROOT/packaging/homebrew/norka.rb" "$ROOT/packaging/scoop/norka.json")"

    env \
      TAP_GITHUB_TOKEN="local-test" \
      NORKA_SHA256SUMS="$tmp/SHA256SUMS" \
      NORKA_SKIP_ASSET_CHECK=1 \
      NORKA_HOMEBREW_REPO="file://${brew_bare}" \
      NORKA_SCOOP_REPO="file://${scoop_bare}" \
      "$0" v9.9.9

    cask_body="$(git --git-dir="$brew_bare" show main:Casks/norka.rb)"
    scoop_body="$(git --git-dir="$scoop_bare" show main:bucket/norka.json)"
    printf '%s\n' "$cask_body" | grep -F 'version "9.9.9"' >/dev/null
    printf '%s\n' "$cask_body" | grep -F "sha256 \"${dmg_hash}\"" >/dev/null
    printf '%s\n' "$cask_body" | grep -F 'download/v#{version}/norka.dmg' >/dev/null
    printf '%s\n' "$cask_body" | grep -F 'com.apple.quarantine' >/dev/null
    printf '%s\n' "$cask_body" | grep -F 'app "norka.app"' >/dev/null
    printf '%s\n' "$scoop_body" | grep -F '"version": "9.9.9"' >/dev/null
    printf '%s\n' "$scoop_body" | grep -F "https://github.com/norka-app/Norka/releases/download/v9.9.9/norka.exe" >/dev/null
    printf '%s\n' "$scoop_body" | grep -F "\"hash\": \"${exe_hash}\"" >/dev/null
    printf '%s\n' "$scoop_body" | grep -F 'v$version' >/dev/null
    printf '%s\n' "$scoop_body" | grep -F '"bin": "norka.exe"' >/dev/null
    git --git-dir="$brew_bare" show main:Casks/norka-cli.rb | grep -F 'norka-cli-marker' >/dev/null
    git --git-dir="$scoop_bare" show main:bucket/norka-cli.json | grep -F 'norka-cli-marker' >/dev/null
    [ "$(git --git-dir="$brew_bare" log -1 --format=%s main)" = "norka v9.9.9" ]
    [ "$(git --git-dir="$scoop_bare" log -1 --format=%s main)" = "norka v9.9.9" ]

    brew_head="$(git --git-dir="$brew_bare" rev-parse main)"
    scoop_head="$(git --git-dir="$scoop_bare" rev-parse main)"
    env \
      TAP_GITHUB_TOKEN="local-test" \
      NORKA_SHA256SUMS="$tmp/SHA256SUMS" \
      NORKA_SKIP_ASSET_CHECK=1 \
      NORKA_HOMEBREW_REPO="file://${brew_bare}" \
      NORKA_SCOOP_REPO="file://${scoop_bare}" \
      "$0" v9.9.9
    [ "$(git --git-dir="$brew_bare" rev-parse main)" = "$brew_head" ]
    [ "$(git --git-dir="$scoop_bare" rev-parse main)" = "$scoop_head" ]

    next_dmg="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
    next_exe="bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
    printf '%s  norka.dmg\n%s  norka.exe\n' "$next_dmg" "$next_exe" >"$tmp/SHA256SUMS"
    env \
      TAP_GITHUB_TOKEN="local-test" \
      NORKA_SHA256SUMS="$tmp/SHA256SUMS" \
      NORKA_SKIP_ASSET_CHECK=1 \
      NORKA_HOMEBREW_REPO="file://${brew_bare}" \
      NORKA_SCOOP_REPO="file://${scoop_bare}" \
      "$0" v9.9.9
    git --git-dir="$brew_bare" show main:Casks/norka.rb | grep -F "sha256 \"${next_dmg}\"" >/dev/null
    git --git-dir="$scoop_bare" show main:bucket/norka.json | grep -F "\"hash\": \"${next_exe}\"" >/dev/null
    git --git-dir="$brew_bare" show main:Casks/norka-cli.rb | grep -F 'norka-cli-marker' >/dev/null
    git --git-dir="$scoop_bare" show main:bucket/norka-cli.json | grep -F 'norka-cli-marker' >/dev/null

    frozen="$(git --git-dir="$brew_bare" rev-parse main)"
    env \
      TAP_GITHUB_TOKEN="" \
      NORKA_SHA256SUMS="$tmp/SHA256SUMS" \
      NORKA_SKIP_ASSET_CHECK=1 \
      NORKA_HOMEBREW_REPO="file://${brew_bare}" \
      NORKA_SCOOP_REPO="file://${scoop_bare}" \
      "$0" v1.2.3
    [ "$(git --git-dir="$brew_bare" rev-parse main)" = "$frozen" ]

    printf 'nope  norka.dmg\n' >"$tmp/bad"
    if env \
      TAP_GITHUB_TOKEN="local-test" \
      NORKA_SHA256SUMS="$tmp/bad" \
      NORKA_SKIP_ASSET_CHECK=1 \
      NORKA_HOMEBREW_REPO="file://${brew_bare}" \
      NORKA_SCOOP_REPO="file://${scoop_bare}" \
      "$0" v9.9.9
    then
      echo "publish-app-packages.sh: битый SHA256SUMS должен завершаться ошибкой" >&2
      exit 1
    fi
    [ "$(git --git-dir="$brew_bare" rev-parse main)" = "$frozen" ] || {
      echo "publish-app-packages.sh: битый SHA256SUMS не должен пушить" >&2
      exit 1
    }

    helper="$tmp/cred"
    cat >"$helper" <<'EOF'
#!/bin/sh
while IFS= read -r line; do
  [ -z "$line" ] && break
done
printf 'username=x-access-token\n'
printf 'password=%s\n' "$TAP_GITHUB_TOKEN"
printf '\n'
EOF
    chmod 700 "$helper"
    helper_out="$(printf 'protocol=https\nhost=github.com\n\n' | TAP_GITHUB_TOKEN=secret-token "$helper" get)"
    printf '%s\n' "$helper_out" | grep -F 'username=x-access-token' >/dev/null
    printf '%s\n' "$helper_out" | grep -F 'password=secret-token' >/dev/null

    after_templates="$(sha256sum "$ROOT/packaging/homebrew/norka.rb" "$ROOT/packaging/scoop/norka.json")"
    if [ "$before_templates" != "$after_templates" ]; then
      echo "publish-app-packages.sh: проверка изменила образцы в packaging/" >&2
      exit 1
    fi
    echo "publish-app-packages.sh: проверка прошла"
  )
}

if [ "${1:-}" = "--check" ]; then
  if [ "$#" -ne 1 ]; then
    die "usage: scripts/publish-app-packages.sh --check"
  fi
  run_check
  exit 0
fi

if [ "$#" -ne 1 ] || [ -z "${1:-}" ]; then
  die "usage: scripts/publish-app-packages.sh <tag>"
fi

prepare_release "$1"

if [ -z "${TAP_GITHUB_TOKEN:-}" ]; then
  echo "TAP_GITHUB_TOKEN пуст, cask и манифест Scoop не пушатся."
  exit 0
fi

write_cred_helper
publish_one "$HOMEBREW_REPO" "$HOMEBREW_DEST" "$STAGE/norka.rb" "norka-app/homebrew-tap"
publish_one "$SCOOP_REPO" "$SCOOP_DEST" "$STAGE/norka.json" "norka-app/scoop-bucket"
