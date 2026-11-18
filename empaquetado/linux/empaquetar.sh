#!/usr/bin/env bash
# Builds the Linux packages of Aula from the binaries already compiled with
# wails build (docs/desarrollo/empaquetar-linux.md):
#   aula-cliente_<v>_amd64.deb, aula-cliente-<v>.x86_64.rpm
#   aula-consola_<v>_amd64.deb, aula-consola-<v>.x86_64.rpm
#   aula-consola-<v>-x86_64.AppImage   (the console only: the agent needs the
#                                       system install to start with each session)
#
# Usage: empaquetar.sh VERSION BIN_DIR OUT_DIR
#   BIN_DIR holds aula-cliente and aula-consola. Tools are pinned and checked
#   by SHA-256; they are cached in $AULA_TOOLS (default ~/.cache/aula-empaquetado).
set -euo pipefail

if [ $# -ne 3 ]; then
  echo "uso: $0 VERSION BIN_DIR OUT_DIR" >&2
  exit 2
fi
VERSION="$1"
BIN_DIR="$(cd "$2" && pwd)"
OUT_DIR="$(mkdir -p "$3" && cd "$3" && pwd)"
HERE="$(cd "$(dirname "$0")" && pwd)"
TOOLS="${AULA_TOOLS:-$HOME/.cache/aula-empaquetado}"
mkdir -p "$TOOLS"

# Pinned tools (update together with this guide's table).
NFPM_VERSION=v2.48.0
APPIMAGETOOL_URL=https://github.com/AppImage/appimagetool/releases/download/1.9.1/appimagetool-x86_64.AppImage
APPIMAGETOOL_SHA256=ed4ce84f0d9caff66f50bcca6ff6f35aae54ce8135408b3fa33abfc3cb384eb0
RUNTIME_URL=https://github.com/AppImage/type2-runtime/releases/download/20251108/runtime-x86_64
RUNTIME_SHA256=2fca8b443c92510f1483a883f60061ad09b46b978b2631c807cd873a47ec260d

for bin in aula-cliente aula-consola; do
  if [ ! -x "$BIN_DIR/$bin" ]; then
    echo "falta $BIN_DIR/$bin (compílalo con wails build -tags webkit2_41)" >&2
    exit 1
  fi
done

# fetch URL SHA256 FILE: downloads once and always checks the hash.
fetch() {
  local url="$1" sum="$2" file="$TOOLS/$3"
  if [ ! -f "$file" ]; then
    curl -fsSL --retry 3 -o "$file.part" "$url"
    mv "$file.part" "$file"
  fi
  if ! echo "$sum  $file" | sha256sum -c --status; then
    echo "SHA-256 de $file no coincide: se borra" >&2
    rm -f "$file"
    exit 1
  fi
  chmod +x "$file"
}

STAGE="$(mktemp -d)"
trap 'rm -rf "$STAGE"' EXIT

# 1. .deb and .rpm with nfpm, from a copy of this folder with the binaries in
#    ./bin and the icons in ./icons (the yaml paths are relative and fixed).
export VERSION
GOBIN="$TOOLS/bin" go install "github.com/goreleaser/nfpm/v2/cmd/nfpm@$NFPM_VERSION"
NFPM="$TOOLS/bin/nfpm"
cp -r "$HERE/." "$STAGE/pkg"
install -D -m 0755 "$BIN_DIR/aula-cliente" "$STAGE/pkg/bin/aula-cliente"
install -D -m 0755 "$BIN_DIR/aula-consola" "$STAGE/pkg/bin/aula-consola"
install -D -m 0644 "$HERE/../../apps/student/build/appicon.png" "$STAGE/pkg/icons/aula-cliente.png"
install -D -m 0644 "$HERE/../../apps/teacher/build/appicon.png" "$STAGE/pkg/icons/aula-consola.png"
(
  cd "$STAGE/pkg"
  for pkg in alumno consola; do
    for fmt in deb rpm; do
      "$NFPM" package --config "nfpm-$pkg.yaml" --packager "$fmt" --target "$OUT_DIR/"
    done
  done
)

# 2. AppImage of the console, with the pinned type2 runtime (no FUSE 2 needed
#    to run it on Ubuntu 24.04).
fetch "$APPIMAGETOOL_URL" "$APPIMAGETOOL_SHA256" appimagetool-1.9.1-x86_64.AppImage
fetch "$RUNTIME_URL" "$RUNTIME_SHA256" runtime-20251108-x86_64
APPDIR="$STAGE/AulaConsola.AppDir"
install -D -m 0755 "$BIN_DIR/aula-consola" "$APPDIR/usr/bin/aula-consola"
install -m 0755 "$HERE/appimage/AppRun" "$APPDIR/AppRun"
install -m 0644 "$HERE/aula-consola.desktop" "$APPDIR/aula-consola.desktop"
sed -i 's#^Exec=/usr/bin/aula-consola#Exec=aula-consola#' "$APPDIR/aula-consola.desktop"
install -m 0644 "$HERE/../../apps/teacher/build/appicon.png" "$APPDIR/aula-consola.png"
ln -s aula-consola.png "$APPDIR/.DirIcon"
APPIMAGE="$OUT_DIR/aula-consola-$VERSION-x86_64.AppImage"
# The tool is an AppImage itself: extract-and-run avoids needing FUSE here.
ARCH=x86_64 APPIMAGE_EXTRACT_AND_RUN=1 "$TOOLS/appimagetool-1.9.1-x86_64.AppImage" \
  --no-appstream --runtime-file "$TOOLS/runtime-20251108-x86_64" "$APPDIR" "$APPIMAGE"
chmod +x "$APPIMAGE"

ls -la "$OUT_DIR"
