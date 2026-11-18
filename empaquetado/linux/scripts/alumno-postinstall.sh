#!/bin/sh
# Aula Cliente: after installing or upgrading (.deb: "configure"; .rpm: 1 or 2).
# The agent starts with every graphical session (/etc/xdg/autostart). It needs
# its channel and the site key; without them its window says what is missing.
set -e
# The agent as a systemd service (D-024): installed and started on every
# install or upgrade, like the Windows installer. Without systemd (a
# container) it keeps starting with each session, as in layer 1.
if [ -d /run/systemd/system ] && command -v systemctl >/dev/null 2>&1; then
  /usr/bin/aula-cliente servicio instalar >/dev/null
  systemctl restart aula-agente.service || true
fi
if [ ! -f /etc/Aula/alumno/config.json ]; then
  cat <<'MSG'

Aula Cliente quedó instalado. Falta configurarlo (canal y clave del centro):

  sudo aula-cliente configurar -canal 1 -clave aula-centro.pub

aula-centro.pub se obtiene en la consola con: aula-consola centro publica -o aula-centro.pub
Para ver la pantalla del alumno, la sesión debe ser X11 («Ubuntu en Xorg»).
Guía: docs/administracion/primeros-pasos.md (Instalar en Linux).

MSG
fi
exit 0
