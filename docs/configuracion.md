# Configuración

Cada app tiene un archivo JSON. Ninguno contiene secretos: la clave **privada** de la consola
vive aparte (`docs/protocol.md` §10) y nunca se copia a los alumnos.

## Agente del alumno (`aula-cliente`)

### Formato

```json
{
  "channel": "AULA_101",
  "console_public_key": "aula-ed25519:+O/2UOmQhSDNXGyym2YIAgdph+8QISFJJsAbNT7J61U=",
  "direct": "192.168.1.10:47830"
}
```

| Campo | Obligatorio | Descripción |
|---|---|---|
| `channel` | Sí | Canal del aula: 1 a 32 mayúsculas, dígitos o `_`. El agente solo busca consolas de este canal. |
| `console_public_key` | Sí | Clave pública de la consola (`aula-ed25519:…`). El agente **solo obedece** a la consola que firma con la clave privada correspondiente. |
| `direct` | No | `IP:puerto` de la consola. Solo si la red bloquea mDNS; con él no se usa el descubrimiento. |

No escribas este archivo a mano: la consola lo genera con el canal y su clave (ver más abajo).

### Dónde lo busca

Sin argumentos, el agente usa **el primero que exista**:

1. **De la máquina:** `C:\ProgramData\Aula\alumno\config.json` (Linux: `/etc/Aula/alumno/config.json`;
   macOS: `/Library/Application Support/Aula/alumno/config.json`). Lo escribe el instalador.
2. **Del usuario:** `%AppData%\Aula\alumno\config.json`, para pruebas o equipos sin instalador.

La de la máquina **gana a la del usuario**, y el instalador la protege para que solo un
administrador pueda cambiarla (lectura para Usuarios, control total para Administradores y
SYSTEM). Así, un alumno no puede redirigir el agente que arranca con su sesión hacia otra
consola. Los argumentos `-canal`, `-clave`, `-directo` y `-config archivo.json` tienen prioridad
sobre ambos; sirven para pruebas y afectan solo a esa ejecución.

Sin configuración válida, la ventana muestra **"Aula — sin configurar"** con el motivo.

### Generarlo desde la consola

- **En la ventana:** abre la clase y pulsa **Configuración de alumnos**. Se guarda
  `aula-alumno.json` con el canal abierto y la clave de esa consola.
- **En la terminal:**
  `aula-consola config-alumno -canal AULA_101 [-o aula-alumno.json] [-directo IP:47830]`.

Pon `aula-alumno.json` **junto a** `Aula-Alumno-Setup-*.exe`. El instalador lo toma de ahí, o
de `/CONFIG=ruta`, y lo copia a la configuración de la máquina.

## Consola del profesor (`aula-consola`)

Todo vive en la carpeta del usuario que usa la consola, `%AppData%\Aula\consola\`:

| Archivo | Contenido |
|---|---|
| `consola.json` | Preferencias (abajo). Se crea al abrir la primera clase. |
| `identidad-ed25519.pem` | Clave **privada** de la consola. Solo la lee ese usuario (ACL). Respáldala: si se pierde, hay que reconfigurar todos los alumnos. |
| `tls-cert.pem`, `tls-key.pem` | Certificado TLS autofirmado. Si se borra, se crea otro y los alumnos siguen funcionando (la huella viaja por mDNS). |

### `consola.json`

```json
{
  "recent_channels": ["AULA_101", "CURSO_PYTHON_A"],
  "console_name": "Prof. Ayala — Programación I",
  "port": 47830
}
```

| Campo | Descripción |
|---|---|
| `recent_channels` | Canales recientes que ofrece la pantalla de inicio (lo mantiene la consola). |
| `console_name` | Nombre que ven los alumnos ("conectado a la clase de …"). |
| `port` | Puerto TCP del servidor (TLS). Opcional; por defecto **47830**. Si lo cambias, los alumnos lo descubren solos por mDNS; solo los que usen `direct` necesitan el puerto nuevo. |

El canal se elige al abrir cada clase. La variable `AULA_CONSOLE_DIR` cambia la carpeta (solo
para pruebas).

## Firewall de Windows

| Equipo | Qué entra | Regla |
|---|---|---|
| Profesor | TCP **47830** (agentes, TLS) y UDP **5353** (preguntas mDNS) | El instalador crea **"Aula Consola"**: entrada permitida para `aula-consola.exe` en perfiles Privado y Dominio. |
| Alumno | UDP **5353** (respuestas mDNS) | El instalador crea **"Aula Cliente (mDNS)"**: UDP de entrada para `aula-cliente.exe` en perfiles Privado y Dominio. Las conexiones al profesor son salientes. |

Las reglas no cubren el perfil **Público**: marca la red del laboratorio como Privada o de
Dominio. Si usas los ejecutables portables, sin instalador, créalas a mano (PowerShell como
administrador):

```powershell
New-NetFirewallRule -DisplayName "Aula Consola" -Direction Inbound -Action Allow `
  -Program "C:\ruta\aula-consola.exe" -Profile Private,Domain
New-NetFirewallRule -DisplayName "Aula Cliente (mDNS)" -Direction Inbound -Action Allow `
  -Program "C:\ruta\aula-cliente.exe" -Protocol UDP -Profile Private,Domain
```

O por puerto, para cualquier programa: TCP 47830 entrante en el profesor y UDP 5353 entrante en
todos.
