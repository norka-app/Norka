#!/bin/sh
# Переписывает версию и хеши в образцах winget, Scoop и Homebrew
# по файлу SHA256SUMS уже опубликованного выпуска.
# Не пушит, не ставит теги и не меняет страницы Release.
#
# Использование: scripts/update-manifests.sh <version>
# Версия — 1.2.1 или v1.2.1.
# Для локальной проверки можно задать NORKA_SHA256SUMS, NORKA_RELEASE_DATE,
# NORKA_ROOT и NORKA_REPO. В CI они не нужны.
set -eu

ROOT="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
if [ -n "${NORKA_ROOT:-}" ]; then
  ROOT="$NORKA_ROOT"
fi
cd "$ROOT"

if [ "$#" -ne 1 ] || [ -z "${1:-}" ]; then
  echo "usage: scripts/update-manifests.sh <version>" >&2
  exit 1
fi

VERSION="${1#v}"
case "$VERSION" in
  *[!0-9A-Za-z.+-]*)
    echo "update-manifests.sh: недопустимая версия: ${VERSION}" >&2
    exit 1
    ;;
esac
case "$VERSION" in
  [0-9]*.[0-9]*.[0-9]*)
    ;;
  *)
    echo "update-manifests.sh: версия должна быть вроде 1.2.1, получено: ${VERSION}" >&2
    exit 1
    ;;
esac

REPO="${NORKA_REPO:-norka-app/Norka}"
case "$REPO" in
  [A-Za-z0-9_.-]*/[A-Za-z0-9_.-]*)
    ;;
  *)
    echo "update-manifests.sh: NORKA_REPO должен быть owner/name" >&2
    exit 1
    ;;
esac

TAG="v${VERSION}"
STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT

if [ -n "${NORKA_SHA256SUMS:-}" ]; then
  cp "$NORKA_SHA256SUMS" "$STAGE/SHA256SUMS"
else
  url="https://github.com/${REPO}/releases/download/${TAG}/SHA256SUMS"
  if ! curl -fsSL "$url" -o "$STAGE/SHA256SUMS"; then
    echo "update-manifests.sh: не скачался ${url}" >&2
    exit 1
  fi
fi

RELEASE_DATE="${NORKA_RELEASE_DATE:-}"

python3 - "$ROOT" "$VERSION" "$STAGE/SHA256SUMS" "$RELEASE_DATE" "$REPO" <<'PY'
import json
import re
import sys
import urllib.request
from pathlib import Path

root = Path(sys.argv[1])
version = sys.argv[2]
sums_path = Path(sys.argv[3])
release_date = sys.argv[4]
repo = sys.argv[5]
tag = f"v{version}"


def fail(message: str) -> None:
    print(f"update-manifests.sh: {message}", file=sys.stderr)
    raise SystemExit(1)


def sha256_of(name: str) -> str:
    found = []
    for raw in sums_path.read_text(encoding="utf-8").splitlines():
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


def published_day() -> str:
    if re.fullmatch(r"\d{4}-\d{2}-\d{2}", release_date):
        return release_date
    if release_date:
        fail(f"NORKA_RELEASE_DATE должен быть YYYY-MM-DD, получено: {release_date}")
    url = f"https://api.github.com/repos/{repo}/releases/tags/{tag}"
    request = urllib.request.Request(
        url,
        headers={
            "Accept": "application/vnd.github+json",
            "User-Agent": "norka-update-manifests",
        },
    )
    try:
        with urllib.request.urlopen(request) as response:
            payload = json.load(response)
    except Exception as exc:
        fail(f"не прочиталась дата выпуска {tag}: {exc}")
    published = str(payload.get("published_at") or "")
    day = published[:10]
    if re.fullmatch(r"\d{4}-\d{2}-\d{2}", day) is None:
        fail(f"у выпуска {tag} нет published_at")
    return day


def must_sub(text: str, pattern: str, repl: str, label: str) -> str:
    new, count = re.subn(pattern, repl, text, count=1, flags=re.M)
    if count != 1:
        fail(f"{label}: ожидалась одна замена, получилось {count}")
    return new


def write_back(path: Path, text: str) -> None:
    original = path.read_text(encoding="utf-8")
    if original == text:
        print(f"без изменений: {path.relative_to(root)}")
        return
    path.write_text(text, encoding="utf-8")
    print(f"обновлён: {path.relative_to(root)}")


exe_hash = sha256_of("norka.exe")
dmg_hash = sha256_of("norka.dmg")
day = published_day()
exe_upper = exe_hash.upper()

winget = root / "packaging" / "winget"
winget_files = [
    winget / "NorkaApp.Norka.yaml",
    winget / "NorkaApp.Norka.installer.yaml",
    winget / "NorkaApp.Norka.locale.en-US.yaml",
    winget / "NorkaApp.Norka.locale.ru-RU.yaml",
]
updated: dict[Path, str] = {}
for path in winget_files:
    if not path.is_file():
        fail(f"нет файла {path}")
    text = path.read_text(encoding="utf-8")
    text = must_sub(text, r"^PackageVersion: .+$", f"PackageVersion: {version}", f"{path.name} PackageVersion")
    updated[path] = text

installer = winget / "NorkaApp.Norka.installer.yaml"
text = updated[installer]
text = must_sub(
    text,
    r"^(ReleaseDate: )\d{4}-\d{2}-\d{2}$",
    rf"\g<1>{day}",
    "ReleaseDate",
)
text = must_sub(
    text,
    r"(https://github\.com/norka-app/Norka/releases/download/)v[^/\s]+(/norka\.exe)",
    rf"\g<1>{tag}\g<2>",
    "InstallerUrl",
)
text = must_sub(
    text,
    r"^(    InstallerSha256: )[0-9A-Fa-f]{64}$",
    rf"\g<1>{exe_upper}",
    "InstallerSha256",
)
updated[installer] = text

locale = winget / "NorkaApp.Norka.locale.en-US.yaml"
text = updated[locale]
text = must_sub(
    text,
    r"(blob/)v[^/\s]+(/LICENSE)",
    rf"\g<1>{tag}\g<2>",
    "LicenseUrl",
)
text = must_sub(
    text,
    r"(releases/tag/)v\S+",
    rf"\g<1>{tag}",
    "ReleaseNotesUrl",
)
updated[locale] = text

scoop = root / "packaging" / "scoop" / "norka.json"
if not scoop.is_file():
    fail(f"нет файла {scoop}")
text = scoop.read_text(encoding="utf-8")
text = must_sub(text, r'("version": ")[^"]+(")', rf"\g<1>{version}\g<2>", "scoop version")
text = must_sub(
    text,
    r'(https://github\.com/norka-app/Norka/releases/download/)v[0-9][^"\s]*(/norka\.exe)',
    rf"\g<1>{tag}\g<2>",
    "scoop url",
)
text = must_sub(
    text,
    r'("hash": ")[0-9A-Fa-f]{64}(")',
    rf"\g<1>{exe_hash}\g<2>",
    "scoop hash",
)
updated[scoop] = text

cask = root / "packaging" / "homebrew" / "norka.rb"
if not cask.is_file():
    fail(f"нет файла {cask}")
text = cask.read_text(encoding="utf-8")
text = must_sub(text, r'^(  version ")[^"]+(")', rf"\g<1>{version}\g<2>", "cask version")
text = must_sub(
    text,
    r'^(  sha256 ")[0-9A-Fa-f]{64}(")',
    rf"\g<1>{dmg_hash}\g<2>",
    "cask sha256",
)
updated[cask] = text

for path, text in updated.items():
    write_back(path, text)

print(f"версия {version}")
print(f"norka.exe {exe_upper}")
print(f"norka.dmg {dmg_hash}")
print(f"ReleaseDate {day}")
PY
