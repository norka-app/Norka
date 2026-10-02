#!/bin/sh
# Собирает AppImage, .deb и tar.gz из уже скомпилированного build/bin/norka.
# Версия: первый аргумент или info.productVersion из wails.json.
set -eu

ROOT="$(CDPATH= cd -- "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

VERSION="${1:-}"
if [ -z "$VERSION" ]; then
  VERSION="$(sed -n 's/.*"productVersion"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' wails.json | head -n 1)"
fi
VERSION="${VERSION#v}"
if [ -z "$VERSION" ]; then
  echo "package.sh: не удалось определить версию" >&2
  exit 1
fi

BIN="$ROOT/build/bin/norka"
if [ ! -f "$BIN" ]; then
  echo "package.sh: нет $BIN — сначала wails build -platform linux/amd64 -tags webkit2_41" >&2
  exit 1
fi

command -v convert >/dev/null 2>&1 || {
  echo "package.sh: нужен ImageMagick (команда convert)" >&2
  exit 1
}

STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT

install_icons() {
  dest="$1"
  for size in 16 32 48 64 128 256 512; do
    dir="$dest/hicolor/${size}x${size}/apps"
    mkdir -p "$dir"
    convert "$ROOT/build/appicon.png" -resize "${size}x${size}" "$dir/norka.png"
  done
}

# --- AppDir / AppImage ---
APPDIR="$STAGE/AppDir"
mkdir -p "$APPDIR/usr/bin" "$APPDIR/usr/share/applications"
cp "$BIN" "$APPDIR/usr/bin/norka"
chmod 755 "$APPDIR/usr/bin/norka"
sed 's|^Exec=.*|Exec=norka|' "$ROOT/packaging/linux/norka.desktop" > "$APPDIR/norka.desktop"
cp "$APPDIR/norka.desktop" "$APPDIR/usr/share/applications/norka.desktop"
install_icons "$APPDIR/usr/share/icons"
convert "$ROOT/build/appicon.png" -resize 256x256 "$APPDIR/norka.png"
cp "$APPDIR/norka.png" "$APPDIR/.DirIcon"
cat > "$APPDIR/AppRun" <<'EOF'
#!/bin/sh
HERE="$(dirname "$(readlink -f "$0")")"
exec "$HERE/usr/bin/norka" "$@"
EOF
chmod 755 "$APPDIR/AppRun"

OUT="$ROOT/build/bin"
mkdir -p "$OUT"
APPIMAGE="$OUT/norka-x86_64.AppImage"
build_appimage() {
  tool="$STAGE/appimagetool"
  curl -fsSL -o "$tool" \
    "https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-x86_64.AppImage"
  chmod 755 "$tool"
  # Распаковка идёт в текущий каталог, поэтому работаем внутри STAGE,
  # чтобы squashfs-root не оказался в корне репозитория.
  (
    cd "$STAGE"
    if ./appimagetool --appimage-extract >/dev/null 2>&1; then
      ARCH=x86_64 ./squashfs-root/AppRun "$APPDIR" "$APPIMAGE"
    else
      ARCH=x86_64 APPIMAGE_EXTRACT_AND_RUN=1 ./appimagetool "$APPDIR" "$APPIMAGE"
    fi
  )
}

if ! build_appimage; then
  echo "package.sh: appimagetool не запустился, собираю AppImage через mksquashfs" >&2
  command -v mksquashfs >/dev/null 2>&1 || {
    echo "package.sh: нет mksquashfs (пакет squashfs-tools)" >&2
    exit 1
  }
  runtime="$STAGE/runtime-x86_64"
  curl -fsSL -o "$runtime" \
    "https://github.com/AppImage/type2-runtime/releases/download/continuous/runtime-x86_64"
  chmod 755 "$runtime"
  mksquashfs "$APPDIR" "$STAGE/norka.squashfs" -root-owned -noappend -quiet
  cat "$runtime" "$STAGE/norka.squashfs" > "$APPIMAGE"
  chmod 755 "$APPIMAGE"
fi

# --- .deb ---
DEBROOT="$STAGE/deb"
mkdir -p "$DEBROOT/DEBIAN" "$DEBROOT/usr/bin" "$DEBROOT/usr/share/applications"
cp "$BIN" "$DEBROOT/usr/bin/norka"
chmod 755 "$DEBROOT/usr/bin/norka"
sed 's|^Exec=.*|Exec=/usr/bin/norka|' "$ROOT/packaging/linux/norka.desktop" \
  > "$DEBROOT/usr/share/applications/norka.desktop"
install_icons "$DEBROOT/usr/share/icons"
sed "s/@VERSION@/${VERSION}/" "$ROOT/packaging/linux/debian/control.in" > "$DEBROOT/DEBIAN/control"
DEB="$OUT/norka_${VERSION}_amd64.deb"
dpkg-deb --root-owner-group --build "$DEBROOT" "$DEB"

# --- tar.gz ---
TARROOT="$STAGE/tar/norka"
mkdir -p "$TARROOT"
cp "$BIN" "$TARROOT/norka"
chmod 755 "$TARROOT/norka"
sed 's|^Exec=.*|Exec=norka|' "$ROOT/packaging/linux/norka.desktop" > "$TARROOT/norka.desktop"
convert "$ROOT/build/appicon.png" -resize 256x256 "$TARROOT/norka.png"
cat > "$TARROOT/README.txt" <<EOF
Norka ${VERSION} для Linux (x86_64).

Запуск: ./norka
Нужны библиотеки системы: GTK 3 и WebKitGTK 4.1
  (пакеты libgtk-3-0t64 и libwebkit2gtk-4.1-0).

Иконка трея появляется, если в сессии есть StatusNotifier
(AppIndicator, KDE, Xfce). На GNOME без расширения AppIndicator
иконки может не быть: окно при этом работает.

Файл norka.desktop можно положить в ~/.local/share/applications/,
поправив Exec на полный путь к бинарнику.
Автозапуск из настроек приложения пишет ~/.config/autostart/norka.desktop.
EOF
TARBALL="$OUT/norka_${VERSION}_linux_amd64.tar.gz"
tar -C "$STAGE/tar" -czf "$TARBALL" norka

echo "package.sh: $APPIMAGE"
echo "package.sh: $DEB"
echo "package.sh: $TARBALL"
