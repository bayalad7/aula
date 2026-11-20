# Compatibilidad por sistema

Qué funciona de Aula en cada sistema operativo. Una fila por función visible (no por interfaz de
código); la columna dice ✓, **parcial** (qué falta) o ✗ (por qué). Lo que un equipo no puede
hacer **no se declara**: la consola lo muestra como no disponible en la ficha del alumno («Qué
puede hacer su equipo»), y el alumno lo ve igual en «Qué puede hacer Aula en este equipo». En el
equipo del alumno, `aula-cliente diagnostico` lista lo disponible en esa sesión.

Al día con el prompt 13, capas 1 a 3 de Linux (2026-11-18). Ubuntu 24.04 LTS es la distribución de
referencia; Linux X11 se prueba en el CI con Xvfb, pero **falta la prueba en un equipo real**
(`docs/prueba-vms.md` §31). macOS todavía no se porta: compila y corre con los fallbacks, sin
probar.

**Linux: X11 o Wayland.** Ubuntu 24.04 entra por defecto en una sesión **Wayland**, donde una app
no puede ver la pantalla de las demás sin que el sistema pida permiso cada vez ni dejar una
ventana siempre encima. Por eso, para ver la pantalla del alumno, el laboratorio usa la sesión
**«Ubuntu en Xorg»** (rueda de la pantalla de inicio de sesión) o desactiva Wayland en GDM
(`WaylandEnable=false` en `/etc/gdm3/custom.conf`). Aula nunca cambia la sesión por su cuenta
(`docs/privacidad.md`, regla 97).

## Alumno (agente)

| Función | Windows 10/11 | Linux X11 | Linux Wayland | macOS |
|---|---|---|---|---|
| Descubrir el canal y conectarse (mDNS, TLS, clave del centro) | ✓ | ✓ | ✓ | sin probar |
| Recibir código y copiarlo con un clic | ✓ | ✓ | ✓ (portapapeles por XWayland; sin probar en equipo real) | sin probar |
| Avisos, mensaje privado, manos y mensajes, temporizador | ✓ | ✓ | ✓ | sin probar |
| Encuestas y cuestionarios | ✓ | ✓ | ✓ | sin probar |
| Terminar la clase, etiqueta del equipo | ✓ | ✓ | ✓ | sin probar |
| Ver su pantalla en miniatura, con el distintivo «Tu pantalla está siendo vista» | ✓ | ✓ | ✗ (Wayland no deja capturar ni dejar el distintivo encima; no se declara) | ✗ (falta el permiso de Grabación de pantalla; etapa 13 macOS) |
| Captura de su pantalla (el profesor guarda solo con «Guardar») | ✓ | ✓ | ✗ (igual que las miniaturas) | ✗ |
| Recibir archivos y plantillas en `Documentos/Aula` | ✓ | ✓ (`xdg-user-dir DOCUMENTS`: `~/Documentos/Aula`) | ✓ | ✗ (falta su carpeta de trabajo) |
| Recoger su trabajo de `Documentos/Aula/recolecciones` | ✓ | ✓ | ✓ | ✗ |
| Ver la transmisión del profesor o de un compañero | ✓ | ✗ (el WebKitGTK de Ubuntu 24.04 no trae WebRTC; el agente lo declara solo si su WebView lo tiene) | igual que X11 | sin probar |
| Proyectar su pantalla a la clase | ✓ | ✓ con openh264 (`libopenh264-7`); su ventana de Aula también se ve en lo proyectado | ✗ (Wayland) | ✗ |
| Control remoto | ✓ | ✓ (XTEST; se pausa cuando el alumno usa su teclado o ratón) | ✗ (Wayland no permite inyectar teclado y ratón sin portal) | ✗ (falta el permiso de Accesibilidad) |
| Pausar su pantalla | ✓ | ✓ (una ventana por monitor) | ✗ (Wayland no deja una ventana encima de todo) | ✗ |
| Apagar, reiniciar y cerrar sesión | ✓ | ✓ (systemd); cerrar sesión sin «forzar» deja guardar en GNOME, Cinnamon y MATE; en otros escritorios cierra los programas sin preguntar | igual que X11 | ✗ |
| Bloquear apps y filtro web | ✓ (como servicio) | ✓ como servicio: apps del alumno (`app.slice` en GNOME; nunca el escritorio) y DNS con una tabla de `nftables` que reiniciar quita | ✓ (igual que X11: no dependen de la pantalla) | ✗ |
| Abrir una página o una app permitida | ✓ | ✓ páginas con `xdg-open`; apps por su `.desktop` del sistema con `gio launch` (en Ajustes › Apps permitidas, el campo «App de Linux») | igual que X11 | ✗ |
| Encender el equipo desde la consola (Wake-on-LAN: el agente informa su MAC) | ✓ | ✓ (Wake-on-LAN activado en la BIOS y en la tarjeta) | ✓ | ✗ |
| Salud del laboratorio | ✓ | parcial: todo menos la falla del disco (S.M.A.R.T.), que necesita root y una herramienta aparte; autodiagnóstico ✓ | igual que X11 | ✗ (se declara sin datos) |
| Arranque | Servicio de Windows con ayudante en la sesión | Servicio de systemd (`aula-agente.service`) con ayudante en la sesión del alumno; arranca con el equipo, también sin sesión | igual que X11 | ✗ |
| Icono del área de notificación | ✓ | ✓ (StatusNotifierItem: GNOME de Ubuntu con la extensión AppIndicator, KDE, XFCE, Cinnamon); sin quien muestre iconos, cerrar minimiza y nunca termina el agente | igual que X11 | ✗ |
| Modo compacto | ✓ (ventana nativa) | parcial: solo con Wails, sin quedar encima | igual que X11 | sin probar |
| Diagnóstico `aula-cliente diagnostico` | ✓ | ✓ (dice si la sesión es X11 o Wayland, qué falta y la salud) | ✓ | sin probar |
| Configurar canal y clave | Instalador (asistente o `/CANAL=`) | `sudo aula-cliente configurar -canal N -clave aula-centro.pub` | igual que X11 | — |
| Paquete | `Aula-Alumno-Setup-x.y.z.exe` | `.deb` y `.rpm` | igual que X11 | ✗ (etapa 13 macOS: `.pkg`) |

## Profesor (consola)

| Función | Windows 10/11 | Linux X11 | Linux Wayland | macOS |
|---|---|---|---|---|
| Anunciar el canal y recibir a los alumnos | ✓ | ✓ (el paquete abre `ufw` o `firewalld` si están activos) | ✓ | sin probar |
| Distribuir código, avisos, encuestas, archivos, recolecciones | ✓ | ✓ | ✓ | sin probar |
| Atajo global Ctrl+Alt+Mayús+C para capturar el portapapeles | ✓ | ✓ | ✗ (Wayland no permite atajos globales a una app; queda «Pegar» en Código) | ✗ |
| Ver miniaturas y capturas de los alumnos | ✓ | ✓ | ✓ (lo que falta en Wayland es del lado del alumno) | sin probar |
| Abrir las carpetas de capturas y recolecciones | ✓ | ✓ (`xdg-open`) | ✓ | ✗ |
| Transmitir su pantalla | ✓ | ✓ con openh264 (`libopenh264-7`); la ventana de Aula también se ve y la franja lo dice | ✗ | ✗ |
| Comprobador de conexión | ✓ | ✓ con `firewalld` o sin firewall; con `ufw` activo, sus reglas solo se leen como root, y el paso de la red queda sin juzgar (Linux no tiene redes «Públicas») | igual que X11 | sin probar |
| Clave del centro (`aula-consola centro …`) | ✓ (`%ProgramData%\Aula\centro`) | ✓ (`/etc/Aula/centro`, grupo `aula`: `sudo usermod -aG aula <maestro>`) | ✓ | sin probar |
| Paquete | `Aula-Consola-Setup-x.y.z.exe` | `.deb`, `.rpm` y AppImage (la AppImage usa el WebKitGTK del sistema) | igual que X11 | ✗ |

## Cómo se mantiene esta tabla

- Una función nueva que toca el sistema agrega su fila **al implementarse**, con la columna de
  cada SO; lo que no se porta queda en ✗ con su porqué.
- El agente no declara lo que no hace: lo prueban `TestLinuxDeclaresOnlyWhatItDoes`
  (`apps/student`) y, en Windows, las pruebas de cada función.
- Cada capa de Linux o macOS que se porte cambia sus filas y la fecha de arriba, y su línea en
  `CHANGELOG.md`.
