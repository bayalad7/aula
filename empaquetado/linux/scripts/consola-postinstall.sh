#!/bin/sh
# Aula Consola: after installing or upgrading (.deb: "configure"; .rpm: 1 or 2).
# 1. The "aula" group reads the site key (one key per centre,
#    CLAUDE.md): /etc/Aula/centro is root:aula 0750, the key 0640.
# 2. The firewall lets the students in (TCP 47830, TLS) and the mDNS questions
#    (UDP 5353), as the Windows installer does (D-012), only if it is active.
set -e
getent group aula >/dev/null 2>&1 || groupadd --system aula
install -d -m 0755 /etc/Aula
install -d -m 0750 -o root -g aula /etc/Aula/centro
if [ -f /etc/Aula/centro/clave-centro.pem ]; then
  chown root:aula /etc/Aula/centro/clave-centro.pem
  chmod 0640 /etc/Aula/centro/clave-centro.pem
fi

if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q "Status: active"; then
  ufw allow 47830/tcp comment 'Aula Consola' >/dev/null || true
  ufw allow 5353/udp comment 'Aula Consola (mDNS)' >/dev/null || true
elif command -v firewall-cmd >/dev/null 2>&1 && firewall-cmd --state >/dev/null 2>&1; then
  firewall-cmd --permanent --add-port=47830/tcp >/dev/null || true
  firewall-cmd --permanent --add-service=mdns >/dev/null || true
  firewall-cmd --reload >/dev/null || true
fi

cat <<'MSG'

Aula Consola quedó instalada. Siguientes pasos (una sola vez por equipo):

  sudo aula-consola centro crear                  # o: centro importar aula-centro.key
  sudo usermod -aG aula <usuario-del-profesor>    # cada maestro que use este equipo

El maestro debe cerrar sesión y volver a entrar para que el grupo cuente.
Guía: docs/administracion/primeros-pasos.md (Instalar en Linux).

MSG
exit 0
