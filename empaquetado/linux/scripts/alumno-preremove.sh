#!/bin/sh
# Aula Cliente: before removing (.deb: "remove"; .rpm: 0). The service is
# stopped and deleted, which also ends the helper. On an upgrade (.deb
# "upgrade", .rpm 1) it stays: postinstall restarts it with the new binary.
set -e
case "$1" in
  remove|0) ;;
  *) exit 0 ;;
esac
if [ -x /usr/bin/aula-cliente ] && [ -d /run/systemd/system ]; then
  /usr/bin/aula-cliente servicio desinstalar >/dev/null 2>&1 || true
fi
exit 0
