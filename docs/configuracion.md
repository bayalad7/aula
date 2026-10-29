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

### Si no conecta

La ventana muestra el **motivo** debajo de "buscando la clase…": no encontró ninguna consola del
canal, la encontró pero su clave no es la del centro, o no pudo conectarse a su dirección. El
mismo texto queda en `%LOCALAPPDATA%\Aula\alumno\aula-cliente.log`. Para un diagnóstico completo:

```bat
"C:\Program Files\Aula\Cliente\aula-cliente.exe" diagnostico
```

Imprime la configuración usada, las consolas del canal que ve por mDNS y el resultado de un
intento de conexión.

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
