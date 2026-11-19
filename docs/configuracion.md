# Configuración

Modelo tipo LanSchool (D-011):

- **Canal:** un número del 1 al 16000. Cada computadora de alumno tiene **su canal fijo**
  (p. ej. centro de cómputo principal = 1, alterno = 2), que solo un administrador cambia. La
  consola del profesor arranca en su **canal predeterminado** y el maestro puede cambiarlo.
- **Clave del centro:** una sola pareja Ed25519 por centro de cómputo. La privada está en los
  equipos de profesor y la pública en los de alumno. El agente solo obedece a consolas que firman
  con ella. No hay claves ni configuraciones por maestro.

## Clave del centro

| Archivo | Dónde | Quién lo puede leer |
|---|---|---|
| `C:\ProgramData\Aula\centro\clave-centro.pem` (privada) | Equipos de profesor | Administradores y SYSTEM (control total); Usuarios del equipo (solo lectura), para que cualquier maestro que inicie sesión pueda dar clase. |
| `aula-centro.pub` (pública, `aula-ed25519:…`) | Junto a `Aula-Alumno-Setup` | Cualquiera: no es secreta. |
| `aula-centro.key` (privada exportada) | Solo para instalar otra consola | **Secreta.** Bórrala después de usarla. |

El instalador de la consola la crea o la importa. También se puede hacer a mano, como
administrador:

```bat
aula-consola centro crear                      :: primera consola del centro
aula-consola centro publica -o aula-centro.pub :: para instalar los alumnos
aula-consola centro exportar aula-centro.key   :: para instalar otras consolas (secreta)
aula-consola centro importar aula-centro.key   :: en cada consola adicional
aula-consola pubkey                            :: muestra qué clave usa esta consola
```

**En Linux** (paquete `aula-consola`, prompt 13): la privada vive en
`/etc/Aula/centro/clave-centro.pem`, del dueño `root` y el grupo de sistema **`aula`** (carpeta
`0750`, archivo `0640`). El paquete crea el grupo; cada maestro que use el equipo entra con
`sudo usermod -aG aula <usuario>` y vuelve a iniciar sesión. Los comandos de arriba se corren con
`sudo`. Quitar el paquete nunca borra la clave.

Si un equipo de profesor no tiene clave del centro, la consola usa una **clave personal**
(`%AppData%\Aula\consola\identidad-ed25519.pem`) y muestra un aviso: los alumnos instalados con
la clave del centro no la reconocerán.

**Si la clave del centro se filtra:** crea otra (`centro crear -force`) y reinstala consolas y
alumnos con la nueva `aula-centro.pub`.

## Agente del alumno (`aula-cliente`)

### Formato

`C:\ProgramData\Aula\alumno\config.json`, escrito por el instalador:

```json
{
  "channel": "1",
  "console_public_key": "aula-ed25519:+O/2UOmQhSDNXGyym2YIAgdph+8QISFJJsAbNT7J61U=",
  "direct": "192.168.1.10:47830"
}
```

| Campo | Obligatorio | Descripción |
|---|---|---|
| `channel` | Sí | Canal de esta computadora, del `"1"` al `"16000"`. Solo busca consolas de este canal. |
| `console_public_key` | Sí | Clave pública **del centro**. Solo obedece a consolas firmadas con la privada correspondiente. |
| `direct` | No | `IP:puerto` de una consola. Solo si la red bloquea mDNS; con él no hay descubrimiento. |

### Dónde lo busca

Sin argumentos, el agente usa **el primero que exista**:

1. **De la máquina:** `C:\ProgramData\Aula\alumno\config.json` (Linux:
   `/etc/Aula/alumno/config.json`; macOS: `/Library/Application Support/Aula/alumno/config.json`).
   El instalador la protege: lectura para Usuarios y control total para Administradores y
   SYSTEM. Así un alumno no puede cambiar su canal ni la clave.
2. **Del usuario:** `%AppData%\Aula\alumno\config.json`, para pruebas o equipos sin instalador.

Los argumentos `-canal`, `-clave`, `-directo` y `-config archivo.json` tienen prioridad; sirven
para pruebas y afectan solo a esa ejecución.

El agente **relee la configuración cada 3 segundos**. Si un administrador reinstala con otro canal,
se reconecta al nuevo sin reiniciar. Con argumentos no relee nada.

### Cambiar el canal de una computadora

Solo administradores, volviendo a ejecutar el instalador:

```bat
Aula-Alumno-Setup-X.Y.Z.exe /VERYSILENT /SUPPRESSMSGBOXES /NORESTART /CANAL=2
```

Conserva la clave del centro que ya tenía la computadora.

**En Linux** no hay asistente: el archivo de la máquina lo escribe el subcomando `configurar`, que
pide `sudo`, valida el canal y la clave antes de guardar y solo cambia lo que se le pasa:

```sh
sudo aula-cliente configurar -canal 1 -clave aula-centro.pub     # al instalar
sudo aula-cliente configurar -canal 2                             # cambiar solo el canal
sudo aula-cliente configurar -directo 192.168.1.10:47830          # si mDNS no llega
```

El archivo queda `0644` (la configuración es pública; solo root la cambia). Quitar el paquete con
`apt purge` o `dnf remove` lo borra, como el desinstalador de Windows.

### Como servicio de systemd (Linux, D-024)

El paquete deja el agente como servicio **`aula-agente.service`** («Aula — agente del aula»), como
root: arranca con el equipo, se reinicia 5 s después de una falla y lanza el ayudante en la
sesión gráfica del alumno, con sus permisos. Se opera igual que en Windows:

```sh
sudo aula-cliente servicio instalar|desinstalar|iniciar|detener
aula-cliente servicio estado          # o: systemctl status aula-agente
```

Su registro está en `/var/log/aula/aula-agente.log`; el del ayudante, en
`~/.cache/Aula/alumno/aula-cliente.log` del alumno. El arranque con la sesión
(`/etc/xdg/autostart/aula-cliente.desktop`) se queda: con el servicio en marcha solo trae al
frente la ventana del ayudante. Sin systemd (un contenedor), ese arranque es el agente mismo.

### Si no conecta

La ventana muestra el **motivo** debajo de "buscando la clase…": no encontró ninguna consola del
canal, la encontró pero su clave no es la del centro, o no pudo conectarse a su dirección. El
mismo texto queda en `%LOCALAPPDATA%\Aula\alumno\aula-cliente.log`. Para un diagnóstico completo:

```bat
"C:\Program Files\Aula\Cliente\aula-cliente.exe" diagnostico
```

Imprime la configuración usada, qué puede hacer Aula en ese equipo y esa sesión, las consolas del
canal que ve por mDNS y el resultado de un intento de conexión. En Linux también dice si la
sesión es X11 o Wayland (en Wayland no se ve la pantalla; `docs/compatibilidad.md`) y el registro
queda en `~/.cache/Aula/alumno/aula-cliente.log`.

### Como servicio de Windows (D-016)

El instalador deja el agente como servicio **`AulaAgente`** ("Aula — agente del aula"):

| Qué | Valor |
|---|---|
| Cuenta | LocalSystem (necesaria para abrir la ventana del alumno en su sesión) |
| Arranque | Automático, con el equipo |
| Ante fallos | Se reinicia a los 5, 10 y 30 s; el contador vuelve a cero tras un día sin fallos |
| Ventana del alumno | El servicio lanza `aula-cliente.exe --ayudante …` en la sesión activa, con los permisos normales del alumno |
| Configuración | Solo la de máquina, `C:\ProgramData\Aula\alumno\config.json` |
| Log del servicio | `C:\ProgramData\Aula\alumno\logs\aula-agente.log` (los usuarios lo leen; solo administradores y SYSTEM escriben) |
| Log de la ventana | `%LOCALAPPDATA%\Aula\alumno\aula-cliente.log` |
| Firewall | Sin reglas nuevas: la de "Aula Cliente (mDNS)" es por programa y cubre al servicio |

Órdenes, como administrador (útiles en scripts y GPO):

```bat
aula-cliente.exe servicio instalar      :: o actualiza un servicio existente
aula-cliente.exe servicio iniciar
aula-cliente.exe servicio estado        :: también sin administrador
aula-cliente.exe servicio detener
aula-cliente.exe servicio desinstalar   :: lo detiene antes
```

`servicio estado` sale con **0** si está en ejecución, **3** si está detenido, **4** si no está
instalado y **1** ante un error. Un alumno sin permisos de administrador no puede detener el
servicio; sí puede cerrar la ventana, y el servicio la vuelve a abrir en segundos.

### Restricciones de la clase (D-017)

Requieren el agente como servicio. Lo que tocan en el equipo del alumno, y cómo se deshace:

| Qué | Dónde | Cuándo se deshace |
|---|---|---|
| Bloqueo de apps | Nada en disco: el servicio cierra los procesos prohibidos de la sesión del alumno cada 500 ms | Al quitar la restricción, desconectarse, cerrar sesión o detener el servicio, simplemente deja de cerrarlos |
| Filtro web: resolvedor | `127.0.0.1:53`, solo loopback (sin regla de firewall) | Al quitar el filtro |
| Filtro web: regla NRPT `.` → `127.0.0.1` | Comentario `aula-filtro-web` (`Get-DnsClientNrptRule`) | Al quitar el filtro; al arrancar el servicio si quedó de una caída; al desinstalar |
| Filtro web: DNS sobre HTTPS apagado | `HKLM\SOFTWARE\Policies\Google\Chrome` y `\Microsoft\Edge` (`DnsOverHttpsMode`), `\Mozilla\Firefox\DNSOverHTTPS` (`Enabled`) | Se restauran los valores exactos que había (o se borran si no existían) |
| Diario | `C:\ProgramData\Aula\alumno\politica-activa.json` | Se borra al deshacer todo |

Para comprobar a mano que un equipo quedó limpio:

```powershell
Get-DnsClientNrptRule | Where-Object Comment -eq 'aula-filtro-web'   # nada
Test-Path C:\ProgramData\Aula\alumno\politica-activa.json          # False
reg query HKLM\SOFTWARE\Policies\Google\Chrome /v DnsOverHttpsMode  # lo que había antes (o no existe)
```

**En Linux** (D-025, con el servicio de systemd):

| Qué | Dónde | Cuándo se deshace |
|---|---|---|
| Bloqueo de apps | Nada en disco: cierra (SIGTERM) las apps del alumno (`app.slice` en GNOME) cada 500 ms; nunca el escritorio ni lo de `/usr/lib` | Igual que en Windows |
| Filtro web: resolvedor | `127.0.0.1:53` | Al quitar el filtro |
| Filtro web: tabla `inet aula` de `nftables` | Lleva el DNS (puerto 53) de los programas del alumno al resolvedor y corta el 853; **no se guarda en disco** | Al quitar el filtro, al arrancar el servicio, al desinstalar y **siempre al reiniciar** |
| Filtro web: DNS sobre HTTPS apagado | `/etc/opt/chrome/…`, `/etc/chromium/…`, `/etc/opt/edge/policies/managed/aula-doh.json` y `/etc/firefox/policies/policies.json` (combinado) | Se devuelve el contenido exacto que había, o se borra lo que no existía |
| Diario | `/etc/Aula/alumno/politica-activa.json` | Se borra al deshacer todo |

```sh
sudo nft list table inet aula                    # «No such file»: no hay filtro
ls /etc/Aula/alumno/politica-activa.json         # no existe
ls /etc/opt/chrome/policies/managed/aula-doh.json  # no existe
```

## Consola del profesor (`aula-consola`)

### De la máquina: `C:\ProgramData\Aula\consola\config.json`

Lo escribe el instalador. Todos los usuarios lo leen y solo los administradores lo cambian.

```json
{
  "default_channel": 1,
  "port": 47830,
  "broadcast_max_kbps": 2500
}
```

| Campo | Descripción |
|---|---|
| `default_channel` | Canal en el que **arranca** la consola. El maestro puede cambiarlo con el selector de la barra superior. |
| `port` | Puerto TCP del servidor (TLS). Opcional; por defecto **47830**. Los alumnos lo descubren por mDNS. La transmisión de la pantalla del profesor usa el UDP siguiente (**47831**). |
| `broadcast_max_kbps` | Tope de la tasa de video al transmitir la pantalla del profesor, en kbit/s. Opcional; por defecto **2500**, mínimo 800. Con un enlace de 100 Mbit/s en el equipo del profesor y más de 20 alumnos, usa `50000 / alumnos` (`docs/decisiones.md` D-014). |

### Del usuario: `%AppData%\Aula\consola\`

| Archivo | Contenido |
|---|---|
| `consola.json` | El nombre que ven los alumnos y los canales recientes (los mantiene la consola). |
| `tls-cert.pem`, `tls-key.pem` | Certificado TLS autofirmado. Si se borra, se crea otro; los alumnos siguen funcionando (la huella viaja por mDNS). |
| `identidad-ed25519.pem` | Solo si el equipo no tiene clave del centro (clave personal). |

Logs: `%LOCALAPPDATA%\Aula\consola\aula-consola.log`. La variable `AULA_CONSOLE_DIR` cambia la
carpeta del usuario y `AULA_SITE_KEY_DIR` la de la clave del centro (solo para pruebas).

## Firewall de Windows

| Equipo | Qué entra | Regla |
|---|---|---|
| Profesor | TCP **47830** (agentes, TLS) y UDP **5353** (preguntas mDNS) | El instalador crea **"Aula Consola"**: entrada permitida para `aula-consola.exe` en perfiles Privado y Dominio. |
| Alumno | UDP **5353** (respuestas mDNS) | El instalador crea **"Aula Cliente (mDNS)"**: UDP de entrada para `aula-cliente.exe` en perfiles Privado y Dominio. Las conexiones al profesor son salientes. |

Por defecto las reglas **no cubren el perfil Público**, y en una red Pública los alumnos no
encuentran la consola ("No se encontró ninguna consola del canal N"). Lo recomendado es marcar la
red del laboratorio como Privada (o Dominio), como administrador:

```powershell
Get-NetConnectionProfile                                    # NetworkCategory e InterfaceAlias
Set-NetConnectionProfile -InterfaceAlias "Wi-Fi" -NetworkCategory Private
```

En muchos equipos, con la GPO *Configuración del equipo → Configuración de Windows →
Configuración de seguridad → Directivas del administrador de listas de redes* (tipo de
ubicación: Privada).

**Si la red tiene que quedar Pública**, marca en los dos instaladores **"Permitir también en
redes Públicas"** (silencioso: `/MERGETASKS="redpublica"`). La regla incluye entonces el
perfil Público.
Úsala solo en equipos que se queden en el laboratorio: en una red Pública ajena (cafetería,
hotel) cualquiera vería la consola y podría conectarse como alumno y recibir el código emitido
(D-012). Al actualizar se conserva la opción; para quitarla, reinstala con
`/MERGETASKS="!redpublica"`.

Para los portables, sin instalador, crea las reglas a mano (PowerShell como administrador;
agrega `Public` a `-Profile` solo si lo necesitas):

```powershell
New-NetFirewallRule -DisplayName "Aula Consola" -Direction Inbound -Action Allow `
  -Program "C:\ruta\aula-consola.exe" -Profile Private,Domain
New-NetFirewallRule -DisplayName "Aula Cliente (mDNS)" -Direction Inbound -Action Allow `
  -Program "C:\ruta\aula-cliente.exe" -Protocol UDP -Profile Private,Domain
```
