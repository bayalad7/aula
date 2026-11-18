#!/bin/sh
# Aula Consola: after removing (.deb: "remove" or "purge"; .rpm: 0). On an
# upgrade (.deb "upgrade", .rpm 1) nothing is undone. The site key in
# /etc/Aula/centro is NEVER deleted: it is the centre's only key, and losing
# it means reinstalling every student computer.
set -e
case "$1" in
  remove|purge|0) ;;
  *) exit 0 ;;
esac
if command -v ufw >/dev/null 2>&1; then
  ufw delete allow 47830/tcp >/dev/null 2>&1 || true
  ufw delete allow 5353/udp >/dev/null 2>&1 || true
fi
if command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
  firewall-cmd --permanent --remove-port=47830/tcp >/dev/null 2>&1 || true
  firewall-cmd --permanent --remove-service=mdns >/dev/null 2>&1 || true
  firewall-cmd --reload >/dev/null 2>&1 || true
fi
if [ -d /etc/Aula/centro ] && [ "$1" != remove ]; then
  echo "La clave del centro sigue en /etc/Aula/centro: bórrala a mano solo si ya no la usa ninguna consola."
fi
exit 0
