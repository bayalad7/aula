#!/bin/sh
# Aula Cliente: after removing (.deb: "remove" or "purge"; .rpm: 0). Purging
# (.deb) or removing (.rpm) deletes the configuration "aula-cliente
# configurar" wrote, as the Windows uninstaller does; a plain .deb "remove"
# keeps it, as Debian expects. On an upgrade nothing is undone.
set -e
case "$1" in
  purge|0)
    rm -f /etc/Aula/alumno/config.json
    rm -rf /var/log/aula /run/aula
    rmdir /etc/Aula/alumno 2>/dev/null || true
    rmdir /etc/Aula 2>/dev/null || true
    ;;
esac
exit 0
