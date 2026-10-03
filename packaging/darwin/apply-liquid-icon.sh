#!/bin/sh
# После wails build: более точный .icns из iconset и, если раннер умеет,
# Assets.car из Icon Composer (.icon) для Dock macOS 26.
# На старом Xcode actool отклоняет .icon — тогда остаётся .icns с полями,
# и сборка не падает. CFBundleIconName пишется только вместе с Assets.car,
# чтобы без каталога система брала CFBundleIconFile.
set -eu

ROOT="$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)"
APP="$ROOT/build/bin/norka.app"
RES="$APP/Contents/Resources"
PLIST="$APP/Contents/Info.plist"

if [ ! -d "$APP" ]; then
  echo "apply-liquid-icon: нет $APP" >&2
  exit 1
fi

ICONSET="$ROOT/build/darwin/norka.iconset"
if [ -d "$ICONSET" ] && command -v iconutil >/dev/null 2>&1; then
  if iconutil -c icns "$ICONSET" -o "$RES/iconfile.icns"; then
    echo "apply-liquid-icon: iconfile.icns собран из iconset"
  else
    echo "apply-liquid-icon: iconutil не собрал icns, остаётся вариант Wails" >&2
  fi
fi

ICON="$ROOT/build/darwin/AppIcon.icon"
if [ ! -d "$ICON" ]; then
  echo "apply-liquid-icon: нет $ICON, Dock на .icns"
  exit 0
fi

if ! xcrun --find actool >/dev/null 2>&1; then
  echo "apply-liquid-icon: actool недоступен, Dock на .icns с полями"
  exit 0
fi

OUT="$(mktemp -d)"
trap 'rm -rf "$OUT"' EXIT

if ! xcrun actool "$ICON" \
  --compile "$OUT" \
  --output-format human-readable-text \
  --notices --warnings --errors \
  --output-partial-info-plist "$OUT/partial.plist" \
  --app-icon AppIcon \
  --include-all-app-icons \
  --enable-on-demand-resources NO \
  --development-region en \
  --target-device mac \
  --minimum-deployment-target 26.0 \
  --platform macosx
then
  echo "apply-liquid-icon: actool не принял .icon (нужен Xcode 26). Dock остаётся на .icns с полями." >&2
  exit 0
fi

CAR="$(find "$OUT" -name Assets.car | head -n 1)"
if [ -z "$CAR" ]; then
  echo "apply-liquid-icon: actool не создал Assets.car, Dock на .icns" >&2
  exit 0
fi

cp "$CAR" "$RES/Assets.car"
NAME="AppIcon"
if [ -x /usr/libexec/PlistBuddy ] && [ -f "$OUT/partial.plist" ]; then
  FROM_PLIST="$(/usr/libexec/PlistBuddy -c 'Print :CFBundleIconName' "$OUT/partial.plist" 2>/dev/null || true)"
  if [ -n "$FROM_PLIST" ]; then
    NAME="$FROM_PLIST"
  fi
fi

if [ ! -x /usr/libexec/PlistBuddy ]; then
  echo "apply-liquid-icon: Assets.car скопирован, но нет PlistBuddy — CFBundleIconName не записан" >&2
  exit 0
fi

if /usr/libexec/PlistBuddy -c 'Print :CFBundleIconName' "$PLIST" >/dev/null 2>&1; then
  /usr/libexec/PlistBuddy -c "Set :CFBundleIconName $NAME" "$PLIST"
else
  /usr/libexec/PlistBuddy -c "Add :CFBundleIconName string $NAME" "$PLIST"
fi
echo "apply-liquid-icon: Assets.car, CFBundleIconName=$NAME"
