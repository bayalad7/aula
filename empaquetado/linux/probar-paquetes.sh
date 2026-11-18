#!/usr/bin/env bash
# Installs the Linux packages on a clean machine, checks them and removes them
# (release.yml, job linux-instalar; also by hand: docs/desarrollo/empaquetar-linux.md).
#
# Usage: probar-paquetes.sh deb|rpm DIR VERSION
#   deb: Ubuntu/Debian, with apt (also checks the console's AppImage)
#   rpm: Fedora/RHEL, with dnf
# Runs as root or with sudo. It changes the machine: use a runner, a container
# or a VM, never a teacher's computer.
set -euxo pipefail

if [ $# -ne 3 ]; then
  echo "uso: $0 deb|rpm DIR VERSION" >&2
  exit 2
fi
FORMAT="$1"
DIR="$(cd "$2" && pwd)"
VERSION="$3"
SUDO=""
[ "$(id -u)" -eq 0 ] || SUDO=sudo

case "$FORMAT" in
  deb)
    $SUDO apt-get update
    $SUDO apt-get install -y "$DIR"/aula-cliente_*_amd64.deb "$DIR"/aula-consola_*_amd64.deb
    ;;
  rpm)
    $SUDO dnf install -y "$DIR"/aula-cliente-*.x86_64.rpm "$DIR"/aula-consola-*.x86_64.rpm
    ;;
  *)
    echo "formato desconocido: $FORMAT" >&2
    exit 2
    ;;
esac

# Both apps start (their libraries are there) and report the version.
test "$(aula-cliente -version)" = "aula-cliente $VERSION"
test "$(aula-consola version)" = "aula-consola $VERSION"
test -f /etc/xdg/autostart/aula-cliente.desktop
test -f /usr/share/applications/aula-cliente.desktop
test -f /usr/share/applications/aula-consola.desktop

# The "aula" group reads the site key; the guide's steps work as written.
getent group aula
test "$($SUDO stat -c '%U:%G %a' /etc/Aula/centro)" = "root:aula 750"
$SUDO aula-consola centro crear
test "$($SUDO stat -c '%U:%G %a' /etc/Aula/centro/clave-centro.pem)" = "root:aula 640"
PUB="$(mktemp)"
$SUDO aula-consola centro publica -o "$PUB"
$SUDO aula-cliente configurar -canal 1 -clave "$PUB"
grep -q '"channel": "1"' /etc/Aula/alumno/config.json
test "$(stat -c '%a' /etc/Aula/alumno/config.json)" = "644"

# The console's AppImage starts without installing anything (extract-and-run:
# no FUSE on runners and containers).
if [ "$FORMAT" = deb ]; then
  APPIMAGE="$(ls "$DIR"/aula-consola-*-x86_64.AppImage)"
  test "$(APPIMAGE_EXTRACT_AND_RUN=1 "$APPIMAGE" version)" = "aula-consola $VERSION"
fi

case "$FORMAT" in
  deb) $SUDO apt-get purge -y aula-cliente aula-consola ;;
  rpm) $SUDO dnf remove -y aula-cliente aula-consola ;;
esac
# (test, not "! command -v": set -e ignores a negated command, and bash
# remembers the path of a program it already ran.)
test ! -e /usr/bin/aula-cliente
test ! -e /usr/bin/aula-consola
test ! -e /etc/xdg/autostart/aula-cliente.desktop
# Purging the agent removes its configuration, as the Windows uninstaller does.
test ! -e /etc/Aula/alumno/config.json
# Removing Aula never deletes the site key.
$SUDO test -f /etc/Aula/centro/clave-centro.pem
echo "Paquetes $FORMAT: bien"
