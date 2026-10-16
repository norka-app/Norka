#!/bin/sh
# Перед публикацией выпуска проверяет три условия:
# коммит тега входит в main, info.productVersion в wails.json совпадает
# с номером тега, и в коммите есть docs/releases/<тег>.md.
# Скрипт не создаёт теги и не меняет выпуски.
#
# Использование:
#   scripts/check-release-tag.sh --tag v1.7.0 [--commit HEAD] [--main origin/main] [--root DIR]
# --tag можно не передавать, если задан GITHUB_REF_NAME.
set -eu

ROOT="$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)"
TAG="${GITHUB_REF_NAME:-}"
COMMIT="HEAD"
MAIN=""

while [ "$#" -gt 0 ]; do
  case "$1" in
    --tag)
      TAG="${2:-}"
      shift 2
      ;;
    --commit)
      COMMIT="${2:-}"
      shift 2
      ;;
    --main)
      MAIN="${2:-}"
      shift 2
      ;;
    --root)
      ROOT="${2:-}"
      shift 2
      ;;
    *)
      echo "check-release-tag.sh: неизвестный аргумент: $1" >&2
      exit 2
      ;;
  esac
done

if [ -z "$TAG" ]; then
  echo "check-release-tag.sh: укажите --tag, например v1.7.0" >&2
  exit 2
fi

case "$TAG" in
  v[0-9]*)
    FILE_TAG="$TAG"
    VERSION="${TAG#v}"
    ;;
  [0-9]*)
    FILE_TAG="v${TAG}"
    VERSION="$TAG"
    ;;
  *)
    echo "check-release-tag.sh: тег должен быть вроде v1.7.0, получено: ${TAG}" >&2
    exit 1
    ;;
esac

if [ -z "$MAIN" ]; then
  if git -C "$ROOT" rev-parse --verify --quiet refs/remotes/origin/main >/dev/null; then
    MAIN="origin/main"
  else
    MAIN="main"
  fi
fi

if ! COMMIT_SHA="$(git -C "$ROOT" rev-parse --verify "${COMMIT}^{commit}" 2>/dev/null)"; then
  echo "check-release-tag.sh: нет коммита ${COMMIT}" >&2
  exit 1
fi

status=0

if ! git -C "$ROOT" rev-parse --verify --quiet "${MAIN}^{commit}" >/dev/null; then
  echo "check-release-tag.sh: нет ссылки ${MAIN}. Нужна история ветки main, не один коммит тега." >&2
  status=1
elif ! git -C "$ROOT" merge-base --is-ancestor "$COMMIT_SHA" "${MAIN}^{commit}"; then
  echo "check-release-tag.sh: коммит ${COMMIT_SHA} не входит в ${MAIN}. Тег должен указывать на коммит из main." >&2
  status=1
else
  echo "check-release-tag.sh: коммит ${COMMIT_SHA} входит в ${MAIN}."
fi

if ! json="$(git -C "$ROOT" show "${COMMIT_SHA}:wails.json" 2>/dev/null)"; then
  echo "check-release-tag.sh: в коммите ${COMMIT_SHA} нет wails.json" >&2
  status=1
else
  product="$(printf '%s\n' "$json" | sed -n 's/.*"productVersion"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)"
  if [ "$product" != "$VERSION" ]; then
    echo "check-release-tag.sh: тег ${FILE_TAG} задаёт версию ${VERSION}, а wails.json productVersion равен ${product:-<пусто>}." >&2
    status=1
  else
    echo "check-release-tag.sh: productVersion ${product} совпадает с тегом."
  fi
fi

notes="docs/releases/${FILE_TAG}.md"
if ! git -C "$ROOT" cat-file -e "${COMMIT_SHA}:${notes}" 2>/dev/null; then
  echo "check-release-tag.sh: нет ${notes}. Текст выпуска берётся только из этого файла." >&2
  status=1
else
  echo "check-release-tag.sh: найден ${notes}."
fi

exit "$status"
