# Protocolo Aula v0

Estado: **aprobado** (etapa 03, 2026-10-23). Implementación: `internal/protocol`. Versión de protocolo: `1`.

Este documento es la fuente de verdad del protocolo entre la **consola** (app del profesor) y
los **agentes** (app del alumno). Todo mensaje nuevo se documenta aquí antes de implementarse.

Convenciones del documento:

- **P** = consola del profesor, **A** = agente del alumno. `A → P` es "del agente al profesor".
- "Debe", "no debe" y "puede" tienen el sentido normativo de RFC 2119.
- Longitudes de texto en **caracteres Unicode** (runas), salvo que se diga "bytes".

---

## 1. Transporte

### 1.1 Descubrimiento (mDNS / DNS-SD)

La consola publica un servicio DNS-SD por mDNS:

| Campo | Valor |
|---|---|
| Tipo de servicio | `_aula._tcp` |
| Dominio | `local.` |
| Nombre de instancia | `console_name` (≤ 63 bytes en UTF-8) |
| Puerto SRV | igual que `ws_port` |

TXT records (todos obligatorios; claves en minúsculas; cada par `clave=valor` ≤ 255 bytes):

| Clave | Ejemplo | Descripción |
|---|---|---|
| `proto` | `1` | Versión mayor del protocolo. Debe coincidir con `v` de la envoltura. |
| `channel` | `1` | Canal: número del 1 al 16000 en decimal, sin ceros a la izquierda (`^[1-9][0-9]{0,4}$`, valor ≤ 16000). Comparación exacta. |
| `console_name` | `Lab 3 — Prof. Ayala` | Nombre visible para el alumno. Solo informativo. |
| `ws_port` | `47830` | Puerto TCP del servidor WebSocket/TLS. |
| `cert_fingerprint` | `sha256:9f86d0…` | SHA-256 del certificado TLS (DER) de la consola, en hex minúsculas (64 dígitos). |

El agente:

1. Explora `_aula._tcp.local.` continuamente mientras no tenga sesión.
2. Descarta toda instancia con `proto` distinto de `1` o con `channel` distinto del suyo.
3. Ignora claves TXT desconocidas (compatibilidad hacia adelante).

> **mDNS no es confiable.** Cualquiera en la LAN puede anunciar cualquier canal y cualquier
> huella. Los TXT solo sirven para *encontrar* candidatos; la confianza la da la autenticación
> Ed25519 (§3.2).

### 1.2 Canal de control (WebSocket sobre TLS)

- El agente abre `wss://<host>:<ws_port>/ws`, con `host` = dirección resuelta por mDNS
  (A/AAAA). **Siempre conecta el alumno al profesor, nunca al revés.**
- TLS 1.3 como mínimo. El certificado de la consola es autofirmado: el agente **no** valida
  contra autoridades certificadoras, sino que compara la huella SHA-256 del certificado recibido con
  `cert_fingerprint`. Si no coinciden, cierra la conexión sin enviar nada. Modelo completo y
  conexión directa sin mDNS en §10.
- Cada mensaje del protocolo viaja en **un** mensaje WebSocket de texto (UTF-8), con exactamente
  una envoltura JSON (§2). Los mensajes binarios están reservados y en v0 se rechazan.
- Códigos de cierre WebSocket usados: `1000` normal, `1002` error de protocolo,
  `1008` violación de política (autenticación fallida, canal o versión incorrectos),
  `1009` mensaje demasiado grande.

### 1.3 Aislamiento por canal

Cada agente tiene en su configuración, instalada por el administrador y que el alumno no puede
cambiar:

- `channel`: el canal de **esa computadora**, p. ej. `1` para la sala principal y `2` para la
  alterna.
- `console_public_key`: la **clave pública del centro** (§10.1), la misma en todas las
  computadoras del centro.

Ejemplo: una red para todo un piso con dos salas.

```
Consola A (sala principal)  ── anuncia channel=1, firmada con la clave del centro KC
Consola B (sala alterna)    ── anuncia channel=2, firmada con KC
Alumnos de la sala principal (40) ── channel=1, clave KC
Alumnos de la sala alterna   (12) ── channel=2, clave KC
Equipo de un alumno con una consola instalada por su cuenta ── anuncia channel=1, clave KX
```

1. **Filtro por canal (descubrimiento):** los alumnos del canal 1 ven todos los anuncios,
   descartan los del canal 2 y solo intentan conectar con consolas del canal 1.
2. **Filtro por clave (autenticación):** la consola instalada por un alumno también anuncia el
   canal 1, pero no puede firmar el reto con la clave privada del centro, así que el agente la
   rechaza (§3.2), la marca como no confiable durante 5 minutos y sigue buscando.
3. Si el maestro de la sala principal cambia su consola al canal 2, ve a los alumnos de la
   sala alterna; sus alumnos del canal 1 lo pierden hasta que vuelva.
4. Si varias consolas legítimas anuncian el mismo canal, el agente las prueba una por una y se
   queda con la primera que se autentique. **Un agente mantiene como máximo una sesión a la
   vez.**

El canal separa salas en la operación diaria y la clave del centro impide la suplantación.
Ninguno de los dos basta por separado.

---

## 2. Envoltura común

Todo mensaje, en ambas direcciones, es un objeto JSON con esta forma:

```json
{
  "v": 1,
  "id": "5b0e8f3c-2a6d-4c1e-9b7a-0f3d2e1c4b5a",
  "ts": 1792771200000,
  "type": "code.share",
  "payload": { }
}
```

| Campo | Tipo | Regla |
|---|---|---|
| `v` | entero | Versión del protocolo. En v0 es siempre `1`. Otro valor → error `unsupported_version` y cierre `1008`. |
| `id` | string | UUID v4 en minúsculas, único por mensaje y por emisor. |
| `ts` | entero | Hora de envío en milisegundos Unix (UTC). **Informativo**: no se usa para seguridad ni para ordenar (los relojes del laboratorio pueden diferir). |
| `type` | string | `dominio.accion` (o `dominio.sub.accion`), en minúsculas, `^[a-z]+(\.[a-z]+)+$`, salvo `hello`, `heartbeat` y `error`. |
| `payload` | objeto | Siempre presente; `{}` si el mensaje no lleva datos. |

Reglas generales:

- Los campos desconocidos en la envoltura o el payload se **ignoran** (compatibilidad hacia
  adelante dentro de `v: 1`).
- Un `type` desconocido se responde con `error` `unknown_type` y la sesión **continúa**.
- Las respuestas se correlacionan con el mensaje original mediante `ref_id` (el `id` del mensaje
  al que responden) dentro del payload.

### 2.1 Mensaje `error`

Cualquiera de los dos extremos lo envía en respuesta a un mensaje que no pudo procesar.

```json
{
  "v": 1, "id": "…", "ts": 1792771200123, "type": "error",
  "payload": {
    "ref_id": "5b0e8f3c-2a6d-4c1e-9b7a-0f3d2e1c4b5a",
    "code": "unsupported_capability",
    "message": "este agente no soporta screen.freeze"
  }
}
```

| Campo | Tipo | Regla |
|---|---|---|
| `ref_id` | string | `id` del mensaje que causó el error. Vacío (`""`) si no se pudo leer (JSON inválido). |
| `code` | string | Uno de la tabla siguiente. |
| `message` | string | Texto legible para logs/UI, ≤ 500 caracteres. **No** debe incluir contenido del mensaje original (código, texto del alumno). |

| `code` | Cuándo | ¿Cierra la sesión? |
|---|---|---|
| `unsupported_version` | `v` distinto de `1` | Sí (`1008`) |
| `invalid_message` | JSON inválido, campo obligatorio ausente o con tipo/valor inválido | No; sí durante el handshake (`1002`) |
| `message_too_large` | Mensaje > 1 MiB o campo por encima de su límite | Sí si excede 1 MiB (`1009`); no si es un campo |
| `unknown_type` | `type` no reconocido | No |
| `unauthorized` | Mensaje distinto del handshake antes de `auth.ok`, o firma inválida | Sí (`1008`) |
| `unsupported_capability` | El agente no declaró la capacidad que requiere el mensaje | No |
| `internal` | Fallo inesperado del receptor | No |

Nunca se responde a un `error` con otro `error` (evita bucles).

---

## 3. Ciclo de sesión

### 3.1 Resumen

```
Agente (A)                                  Consola (P)
   |  — descubre por mDNS, filtra canal —        |
   |==== TLS (huella = cert_fingerprint) =======>|
   |-- auth.challenge (nonce) ------------------>|
   |<------------------------------ auth.proof --|  firma Ed25519
   |   verifica con console_public_key           |
   |-- auth.ok --------------------------------->|
   |-- hello ----------------------------------->|  hostname, usuario, SO, versión, capacidades
   |<========== sesión autenticada ============>|
   |<-- heartbeat (cada 5 s, ambos sentidos) --->|
   |<-------------------------------- code.share-|
   |-- code.ack -------------------------------->|
```

**El agente se autentica primero y se presenta después.** Así no revela hostname ni usuario a
una consola que anuncie su canal pero no tenga la clave privada correcta.

El handshake completo (desde abrir TLS hasta recibir `hello`) debe terminar en **10 s**; si no,
el extremo que espera cierra con `1008`.

Antes de `auth.ok`:

- El agente solo acepta `auth.proof` y `error`. Cualquier otro mensaje → `error` `unauthorized`
  y cierre `1008`.
- La consola solo acepta `auth.challenge`, `auth.ok` y `error`.

Después de `auth.ok`, el primer mensaje del agente **debe** ser `hello`. Hasta recibirlo, la
consola no envía nada salvo `heartbeat`, porque todavía no conoce las capacidades del agente.
Si el primer mensaje no es `hello` → `error` `invalid_message` y cierre `1002`.

### 3.2 Autenticación: `auth.challenge` → `auth.proof` → `auth.ok`

**`auth.challenge` (A → P)**: primer mensaje de la conexión.

```json
{ "nonce": "q83vEjRWeJq8vN7wEjRWeJq8vN7wEjRWeJq8vN7wEjQ=" }
```

- `nonce`: 32 bytes aleatorios (`crypto/rand`), en Base64 estándar con relleno. Nuevo en cada
  conexión; nunca se reutiliza.

**`auth.proof` (P → A)**

```json
{ "ref_id": "<id del auth.challenge>", "signature": "<64 bytes en Base64>" }
```

La consola firma con Ed25519 (clave privada de la consola) estos bytes exactos:

```
"aula-auth-v1" || 0x00 || nonce (32 bytes) || 0x00 || channel (UTF-8) || 0x00 || cert_sha256 (32 bytes)
```

- La etiqueta `aula-auth-v1` evita que una firma de otro contexto sirva aquí.
- `channel` liga la prueba al canal que el agente busca.
- `cert_sha256` (la huella del certificado TLS **que vio el agente**) liga la prueba a esta
  conexión TLS: si un intermediario reenvía el reto a la consola legítima, esta firma *su*
  huella, que no coincide con la que vio el agente, y la verificación falla.

El agente verifica la firma con `console_public_key`. Si falla: `error` `unauthorized`, cierre
`1008` y la instancia queda marcada como no confiable (§1.3).

**`auth.ok` (A → P)**

```json
{ "ref_id": "<id del auth.proof>" }
```

Lo envía el **agente**, porque es quien verifica. Desde aquí la sesión está autenticada y el
agente envía `hello` de inmediato.

> En v0 la consola **no autentica al agente**: cualquier equipo con el agente instalado puede
> unirse al canal. La consola muestra hostname y usuario para que el profesor lo vea. La
> autenticación de agentes queda pendiente para una versión futura.

### 3.3 `hello` (A → P)

```json
{
  "hostname": "LAB3-PC07",
  "username": "alumno07",
  "os": "windows",
  "os_version": "10.0.19045",
  "agent_version": "0.1.0",
  "capabilities": ["code.view", "msg.show"],
  "macs": ["3c:52:82:1a:9f:07"]
}
```

| Campo | Tipo | Regla |
|---|---|---|
| `hostname` | string | 1–255 caracteres. |
| `username` | string | Usuario de la sesión del SO, 0–256 caracteres (vacío si no se puede obtener). Con el agente como servicio (D-016), es el usuario de la **sesión de consola activa**, y vacío si nadie inició sesión; en ese caso `capabilities` va vacío y el agente se reconecta con un `hello` nuevo cuando alguien entra. |
| `os` | string | `windows`, `linux` o `darwin`. |
| `os_version` | string | Libre, ≤ 64 caracteres. |
| `agent_version` | string | SemVer `x.y.z` del agente (la misma del release). |
| `capabilities` | string[] | §4. Sin duplicados; ≤ 64 entradas. |
| `macs` | string[] | **Opcional** (Sin publicar, §5.15.2). Direcciones físicas (MAC) de la tarjeta de red por la que el agente llega a la consola, para encenderlo con Wake-on-LAN. Formato `aa:bb:cc:dd:ee:ff` (6 bytes, hexadecimal en minúsculas); sin duplicados; ≤ 4 entradas. Ausente o vacío: un agente viejo o un alumno simulado, y su equipo no se puede encender desde la consola. |

`hello` no tiene respuesta. Si es inválido, la consola responde `error` `invalid_message` y
cierra con `1002`.

### 3.4 `heartbeat` (A ↔ P)

- Cada extremo envía `heartbeat` cada **5 s** después de `auth.ok`. Payload: `{}`.
- No tiene respuesta.
- Si un extremo no recibe **ningún** mensaje durante **15 s** (3 latidos), considera la
  sesión caída y cierra la conexión.

### 3.5 Reconexión con retroceso exponencial (agente)

Cuando la sesión se cae o un intento falla:

- Espera `min(30 s, 1 s × 2^n)`, con jitter aleatorio de ±20 %, donde `n` es el número de
  fallos consecutivos (0, 1, 2…). Secuencia típica: 1, 2, 4, 8, 16, 30, 30… s.
- `n` vuelve a 0 cuando el agente envía `auth.ok` (sesión establecida).
- Tras 3 fallos seguidos contra la misma dirección, vuelve a descubrir por mDNS (la consola
  pudo cambiar de IP o puerto).
- Un fallo de autenticación no reintenta esa instancia hasta que vence la marca de 5 minutos.

La consola no reconecta: solo espera conexiones entrantes. Al reconectar, el agente es una
sesión nueva; el estado en pantalla (el último snippet) se conserva en el agente.

---

## 4. Capacidades

El agente declara en `hello` lo que puede hacer en su SO. La consola **no debe** enviar un
mensaje cuya capacidad no se declaró, y desactiva el control correspondiente en la interfaz
para ese agente. Si de todos modos llega, el agente responde `error` `unsupported_capability`.

| Capacidad | Habilita | Estado |
|---|---|---|
| `code.view` | `code.share`, `code.clear` | v0 |
| `msg.show` | `msg.show` | v0 |
| `screen.thumb` | `screen.thumb.start`, `screen.thumb.stop` | v0.3 (Windows) |
| `screen.snapshot` | `screen.snapshot.request` | v0.7 (§5.17). Windows, con la misma condición que `screen.thumb` (captura y distintivo); también los alumnos simulados. Linux y macOS no la declaran. |
| `screen.freeze` | `screen.freeze`, `screen.unfreeze` | Sin publicar (§5.9–5.11). Windows, en modo ventana y como servicio con su ayudante (la pausa se muestra en la sesión del alumno). Nunca los alumnos simulados. Linux y macOS no la declaran. |
| `screen.broadcast` | `screen.broadcast.start`, `.offer`, `.ice`, `.stop` | v0.4 (Windows, si el WebView2 tiene RTCPeerConnection y decodificador H.264; §5.12). Linux y macOS no la declaran. |
| `app.block` | `app.block.set`, `app.block.clear` | v0.4 (§5.13). Windows, solo con el agente como servicio y su ayudante (D-016). |
| `web.filter` | `web.filter.set`, `web.filter.clear` | v0.4 (§5.14). Windows, solo con el agente como servicio y su ayudante. |
| `file.collect` | `file.collect.open`, `file.collect.request`, `file.collect.response.ack` | v0.11 (§5.21). Windows, en modo ventana y como servicio con su ayudante (lee en la sesión del alumno); también los alumnos simulados, que devuelven su último código. Linux y macOS no la declaran. |
| `code.collect` | `code.collect` | Reservada |
| `power` | `power.action`, `power.cancel` | v0.5 (§5.15). Windows, en modo ventana y como servicio (también sin sesión iniciada). Linux y macOS no la declaran. |
| `remote` | `remote.start`, `remote.input`, `remote.stop` | v0.6 (§5.16). Windows, en modo ventana y como servicio con su ayudante (la entrada se inyecta en la sesión del alumno). Linux y macOS no la declaran. |
| `hand` | `hand.raise`, `hand.lower` | v0.8 (§5.18). Todos los SO: no toca el sistema, solo la ventana del alumno. Se declara cuando hay ventana (en modo servicio, con el ayudante en la sesión del alumno); también los alumnos simulados. |
| `msg.student` | `msg.student`, `msg.student.ack` | v0.8 (§5.18). Igual que `hand`. |
| `timer` | `timer.start`, `timer.extend`, `timer.stop` | v0.9 (§5.19). Todos los SO: no toca el sistema, solo la ventana del alumno. Igual que `hand`: se declara cuando hay ventana; también los alumnos simulados. |
| `file.push` | `file.push` | v0.10 (§5.20). Windows, en modo ventana y como servicio con su ayudante (escribe en la sesión del alumno); también los alumnos simulados, que acusan sin escribir. Linux y macOS no la declaran. |
| `open.url` | `open.url` | v0.12 (§5.22). Windows, en modo ventana y como servicio con su ayudante (abre en la sesión del alumno). Nunca los alumnos simulados. Linux y macOS no la declaran. |
| `open.app` | `open.app` | v0.12 (§5.22). Igual que `open.url`. |
| `poll` | `poll.open`, `poll.close`, `poll.results`, `poll.vote.ack`; y acepta `poll.vote` | v0.13 (§5.23). Todos los SO: no toca el sistema, solo la ventana del alumno. Igual que `hand`: se declara cuando hay ventana (en modo servicio, con el ayudante); también los alumnos simulados. |
| `screen.present` | `student.present.start`, `.offer`, `.ice`, `.stop` | v0.14 (§5.24). Windows, si hay captura de monitor, codificador H.264 y la ventana se puede excluir de la captura; en modo servicio, solo con el ayudante. Nunca los alumnos simulados. Linux y macOS no la declaran. |
| `screen.present.view` | `screen.broadcast.*` con `source` (la pantalla de un compañero) | v0.14 (§5.24). La declara quien declara `screen.broadcast` y sabe mostrar de quién es la pantalla. Nunca los alumnos simulados. |
| `class.end` | `class.end` | Sin publicar (§5.25). Todos los SO: no toca el sistema, limpia la ventana. Se declara cuando hay ventana (en modo servicio, con el ayudante); también los alumnos simulados. |

`hand`, `msg.student` y `poll` habilitan mensajes que **inicia el agente**: la consola acepta
`hand.*`, `msg.student` y `poll.vote` solo de un agente que declaró la capacidad (si no, responde
`error` `unsupported_capability`), y solo le envía `hand.lower`, `msg.student.ack` y
`poll.vote.ack` a ese agente.

Las capacidades desconocidas en `hello` se ignoran (un agente más nuevo puede declarar cosas
que la consola todavía no conoce). Formato: `^[a-z]+(\.[a-z]+)*$`, ≤ 64 caracteres.

---

## 5. Mensajes v0

| Tipo | Dirección | Capacidad | Payload | Respuesta |
|---|---|---|---|---|
| `auth.challenge` | A → P | — | `nonce` | `auth.proof` |
| `auth.proof` | P → A | — | `ref_id`, `signature` | `auth.ok` o `error` |
| `auth.ok` | A → P | — | `ref_id` | — (sigue `hello`) |
| `hello` | A → P | — | `hostname`, `username`, `os`, `os_version`, `agent_version`, `capabilities` | — |
| `heartbeat` | A ↔ P | — | `{}` | — |
| `error` | A ↔ P | — | `ref_id`, `code`, `message` | — |
| `code.share` | P → A | `code.view` | `snippet_id`, `exercise`?, `title`, `lang`, `content`, `mode` | `code.ack` o `error` |
| `code.ack` | A → P | — | `ref_id`, `snippet_id` | — |
| `code.clear` | P → A | `code.view` | `{}` | `code.ack` o `error` |
| `msg.show` | P → A | `msg.show` | `text`, `level`, `reply_to`?, `personal`? | `msg.ack` o `error` |
| `msg.ack` | A → P | — | `ref_id` | — |
| `screen.thumb.start` | P → A | `screen.thumb` | `interval_ms`, `quality`, `max_width` | `screen.thumb.frame` periódicos o `error` |
| `screen.thumb.frame` | A → P | — | `ref_id`, `seq`, `width`, `height`, `jpeg` | — |
| `screen.thumb.stop` | P → A | `screen.thumb` | `{}` | — |
| `screen.freeze` | P → A | `screen.freeze` | `message`? | `screen.freeze.ack` (`frozen: true`) o `error` |
| `screen.unfreeze` | P → A | `screen.freeze` | `{}` | `screen.freeze.ack` (`frozen: false`) |
| `screen.freeze.ack` | A → P | — | `ref_id`, `frozen` | — |
| `screen.broadcast.start` | P → A | `screen.broadcast` | `broadcast_id`, `width`, `height`, `fps` (opcionales `source`, `mode`) | — (sigue `screen.broadcast.offer`) o `error` |
| `screen.broadcast.offer` | P → A | `screen.broadcast` | `broadcast_id`, `attempt`, `sdp` | `screen.broadcast.answer` o `error` |
| `screen.broadcast.answer` | A → P | — | `ref_id`, `broadcast_id`, `attempt`, `sdp` | — |
| `screen.broadcast.ice` | A ↔ P | `screen.broadcast` (P → A) | `broadcast_id`, `attempt`, `candidate`, `sdp_mid`, `sdp_mline_index` | — |
| `screen.broadcast.stop` | P → A | `screen.broadcast` | `broadcast_id` | — |
| `app.block.set` | P → A | `app.block` | `policy_id`, `mode`, `apps` | `app.block.ack` o `error` |
| `app.block.clear` | P → A | `app.block` | `{}` | `app.block.ack` |
| `app.block.ack` | A → P | — | `ref_id`, `policy_id`, `active` | — |
| `web.filter.set` | P → A | `web.filter` | `policy_id`, `mode`, `domains` | `web.filter.ack` o `error` |
| `web.filter.clear` | P → A | `web.filter` | `{}` | `web.filter.ack` |
| `web.filter.ack` | A → P | — | `ref_id`, `policy_id`, `active` | — |
| `power.action` | P → A | `power` | `action_id`, `action`, `delay_s`?, `force` | `power.ack` (uno o varios) o `error` |
| `power.cancel` | P → A | `power` | `action_id` | `power.ack` (`cancelled`); sin orden vigente se ignora |
| `power.ack` | A → P | — | `ref_id`, `action_id`, `status`, `remaining_s`, `postpones_left` | — |
| `remote.start` | P → A | `remote` | `control_id`, `interval_ms`, `quality`, `max_width` | `remote.ack` (`active`) y `screen.thumb.frame` periódicos, o `error` |
| `remote.input` | P → A | `remote` | `control_id`, `seq`, `events` | — (solo `error`) |
| `remote.stop` | P → A | `remote` | `control_id` | `remote.ack` (`ended`); sin control vigente se ignora |
| `remote.ack` | A → P | — | `ref_id`, `control_id`, `status`, `reason`? | — |
| `screen.snapshot.request` | P → A | `screen.snapshot` | `snapshot_id`, `quality`, `max_width` | `screen.snapshot.response` (1 a 32 partes) o `error` |
| `screen.snapshot.response` | A → P | — | `ref_id`, `snapshot_id`, `part`, `parts`, `data`; la parte 1 además `taken_at`, `hostname`, `username`, `width`, `height`, `size`, `sha256` | — |
| `hand.raise` | A → P | `hand` | `hand_id`, `note`? | — (solo `error`) |
| `hand.lower` | A ↔ P | `hand` | `hand_id`, `by` | — |
| `msg.student` | A → P | `msg.student` | `message_id`, `text` | `msg.student.ack` o `error` |
| `msg.student.ack` | P → A | `msg.student` | `ref_id`, `message_id` | — |
| `timer.start` | P → A | `timer` | `timer_id`, `label`, `duration_s`, `remaining_ms` (opcional `description`) | `timer.ack` o `error` |
| `timer.extend` | P → A | `timer` | `timer_id`, `add_s`, `remaining_ms` | `timer.ack`; con otro `timer_id` se ignora |
| `timer.stop` | P → A | `timer` | `timer_id` | `timer.ack` (`stopped`); sin temporizador vigente se ignora |
| `timer.ack` | A → P | — | `ref_id`, `timer_id`, `status`, `remaining_ms` | — |
| `file.push` | P → A | `file.push` | `push_id`, `file_id`, `part`, `parts`, `data`; la parte 1 además `name`, `dest`, `size`, `sha256`, `overwrite`, `label`, `index`, `files` | `file.push.ack` al completar o rechazar, o `error` |
| `file.push.ack` | A → P | — | `ref_id`, `push_id`, `file_id`, `status`, `path`, `renamed`, `replaced`, `reason`? | — |
| `file.collect.open` | P → A | `file.collect` | `collect_id`, `date`, `activity`, `patterns` | `file.collect.ack` (`ready`) o `error` |
| `file.collect.request` | P → A | `file.collect` | `collect_id`, `date`, `activity`, `round`, `patterns`, `max_file_size`, `max_total`, `max_files` | `file.collect.ack` y, con `sending`, las `file.collect.response`; o `error` |
| `file.collect.ack` | A → P | — | `ref_id`, `collect_id`, `round`, `status`, `path`?, `files`?, `total`?, `skipped`?, `reason`? | — |
| `file.collect.response` | A → P | — | `collect_id`, `round`, `file_id`, `part`, `parts`, `data`; la parte 1 además `path`, `size`, `sha256`, `modified_at`, `index`, `files` | `file.collect.response.ack` al completar o rechazar |
| `file.collect.response.ack` | P → A | `file.collect` | `ref_id`, `collect_id`, `round`, `file_id`, `status`, `reason`? | — |
| `open.url` | P → A | `open.url` | `open_id`, `url` | `open.ack` o `error` |
| `open.app` | P → A | `open.app` | `open_id`, `app_id`, `name`, `path` | `open.ack` o `error` |
| `open.ack` | A → P | — | `ref_id`, `open_id`, `status`, `reason`? | — |
| `poll.open` | P → A | `poll` | `poll_id`, `kind`, `question`, `options`?, `named` | `poll.ack` (`shown`) o `error` |
| `poll.vote` | A → P | `poll` | `poll_id`, `choice` o `text` | `poll.vote.ack` o `error` |
| `poll.vote.ack` | P → A | `poll` | `ref_id`, `poll_id`, `status` | — |
| `poll.close` | P → A | `poll` | `poll_id` | `poll.ack` (`closed`) |
| `poll.results` | P → A | `poll` | `poll_id`, `counts`, `voters`, `recipients` | `poll.ack` (`results`) |
| `poll.ack` | A → P | — | `ref_id`, `poll_id`, `status` | — |
| `student.present.start` | P → A | `screen.present` | `present_id`, `max_width`, `fps`, `max_kbps` | `student.present.ack` (`accepted` o `failed`) o `error` |
| `student.present.ack` | A → P | — | `ref_id`, `present_id`, `status`, `width`?, `height`?, `reason`? | — |
| `student.present.offer` | P → A | `screen.present` | `present_id`, `attempt`, `sdp` | `student.present.answer` o `error` |
| `student.present.answer` | A → P | — | `ref_id`, `present_id`, `attempt`, `sdp` | — |
| `student.present.ice` | A ↔ P | `screen.present` (P → A) | `present_id`, `attempt`, `candidate`, `sdp_mid`, `sdp_mline_index` | — |
| `student.present.stop` | P → A | `screen.present` | `present_id` | `student.present.ack` (`ended`); con otro id se ignora |
| `class.end` | P → A | `class.end` | `end_id` | `class.end.ack` o `error` |
| `class.end.ack` | A → P | — | `ref_id`, `end_id`, `status`, `reason`? | — |

### 5.1 `code.share` (P → A)

Muestra un fragmento de código en el overlay del alumno, listo para copiar.

```json
{
  "snippet_id": "0c9a7e52-6a4b-4f7e-8d33-2b1f4e6a9c10",
  "exercise": "3",
  "title": "Factorial recursivo",
  "lang": "python",
  "content": "def factorial(n):\n    return 1 if n <= 1 else n * factorial(n - 1)\n",
  "mode": "replace"
}
```

| Campo | Tipo | Regla |
|---|---|---|
| `snippet_id` | string | UUID v4 que asigna la consola. Identifica el snippet entre reenvíos. |
| `exercise` | string | **Opcional.** Número o etiqueta del ejercicio (`"3"`, `"3.b"`), 1–16 caracteres. El overlay lo muestra como etiqueta junto al título. Ausente o `""` = sin número. |
| `title` | string | 0–200 caracteres. |
| `lang` | string | Identificador de lenguaje de Monaco (`python`, `go`, `javascript`, `java`, `c`, `cpp`, `csharp`, `sql`, `html`, `css`, `plaintext`…). `^[a-z0-9+#-]{1,32}$`. Desconocido → el agente lo muestra como `plaintext` (no es error). |
| `content` | string | UTF-8 válido, ≤ **256 KiB** (262 144 bytes). Saltos de línea `\n`; el agente no altera el contenido. |
| `mode` | string | `replace`: sustituye lo que muestra el agente. `append`: lo agrega al final, separado por una línea en blanco, conservando el `title` nuevo. |

### 5.2 `code.ack` (A → P)

```json
{ "ref_id": "<id del code.share o code.clear>", "snippet_id": "0c9a7e52-…" }
```

Confirma que el agente **mostró** el snippet (o limpió el overlay). No indica que el alumno lo
leyó ni lo copió. `snippet_id` va vacío en respuesta a `code.clear`.

### 5.3 `code.clear` (P → A)

Payload `{}`. El agente vacía el overlay de código y responde `code.ack`.

### 5.4 `msg.show` (P → A)

```json
{ "text": "Guarden su trabajo: en 5 minutos cerramos.", "level": "warning" }
```

| Campo | Tipo | Regla |
|---|---|---|
| `text` | string | 1–2000 caracteres, texto plano (el agente no interpreta HTML ni Markdown). |
| `level` | string | `info` o `warning`. |
| `reply_to` | string | Opcional (v0.8, §5.18). `message_id` del `msg.student` al que responde. La consola solo lo pone en un aviso a **un** alumno, el autor de ese mensaje. El agente muestra el aviso como «Respuesta a tu mensaje» con la cita de lo que escribió; si no conoce ese `message_id`, lo muestra como un aviso normal. Un agente anterior lo ignora. |
| `personal` | booleano | Opcional (v0.8, prompt 10.7). `true`: el aviso va **solo a este alumno** («Mensaje privado» de la ficha y «Responder» de Participación); el agente lo titula «Mensaje para ti». La consola solo lo pone en un `msg.show` enviado a **una** sesión, nunca en uno difundido (`docs/privacidad.md` regla 53). Ausente o `false`: un aviso como los demás. Un agente anterior lo ignora y lo muestra como un aviso normal (le llega igual solo a él). |

### 5.5 `msg.ack` (A → P)

```json
{ "ref_id": "<id del msg.show>" }
```

Confirma que el mensaje se mostró. No registra si el alumno lo leyó.

### 5.6 `screen.thumb.start` (P → A)

Pide al agente que capture su **monitor principal** y envíe un cuadro cada `interval_ms`. La
consola lo envía al abrir la vista de pantallas y lo vuelve a enviar, con otros valores, para ver
un equipo en detalle: un `start` nuevo **reemplaza** los parámetros del anterior y reinicia `seq`.

```json
{ "interval_ms": 1000, "quality": 50, "max_width": 320 }
```

| Campo | Tipo | Regla |
|---|---|---|
| `interval_ms` | entero | 500–10 000. Como máximo **2 cuadros por segundo**. |
| `quality` | entero | Calidad JPEG, 10–95. |
| `max_width` | entero | Ancho máximo en píxeles, 160–1920. La imagen se reduce conservando la proporción; nunca se amplía. |

Valores de la consola: miniaturas `1000 ms / 50 / 320`; vista en detalle `1000 ms / 70 / 1280`.

Reglas del agente:

- **Indicador visible antes del primer cuadro.** Mientras haya una transmisión activa, el equipo
  muestra "Tu pantalla está siendo vista por el profesor" (`docs/privacidad.md`).
- El primer cuadro sale de inmediato; los siguientes, cada `interval_ms` contado desde que se
  encoló el anterior.
- Si no puede capturar (por ejemplo, la sesión está bloqueada), responde **una vez** `error`
  `internal` con ese `ref_id` y lo sigue intentando en cada intervalo; el indicador sigue visible.
- La transmisión termina con `screen.thumb.stop` o al **cerrarse la conexión**. Las capturas
  solo viven en memoria; el agente no las escribe en disco. Una foto fija a resolución completa
  es otro mensaje (`screen.snapshot.request`, §5.17).

### 5.7 `screen.thumb.frame` (A → P)

```json
{ "ref_id": "<id del screen.thumb.start>", "seq": 12, "width": 320, "height": 180,
  "jpeg": "/9j/4AAQSkZJRgABAQ…" }
```

| Campo | Tipo | Regla |
|---|---|---|
| `ref_id` | string | `id` del `screen.thumb.start` vigente. La consola descarta cuadros de un `start` anterior. |
| `seq` | entero | 1, 2, 3… por cada `start`. |
| `width`, `height` | entero | Tamaño de la imagen, 1–1920 y 1–1920. |
| `jpeg` | string | Imagen JPEG en Base64, ≤ **640 KiB** antes de codificar. Si la excede, el agente baja la calidad y lo reintenta. |

La consola tampoco guarda los cuadros en disco: muestra el último de cada equipo y lo descarta
al cerrar la vista. Guardar una captura es otro mensaje y una acción explícita del profesor
(§5.17).

### 5.8 `screen.thumb.stop` (P → A)

Payload `{}`. El agente deja de capturar y quita el indicador. Es idempotente y no tiene
respuesta: la prueba de que se detuvo es que no llegan más cuadros.

### 5.9 `screen.freeze` (P → A)

> Implementado (Windows, sin publicar), aprobado el 2026-11-12 (prompt 10.21). Decisión técnica en
> D-013; implementación en `internal/screen` (`Freezer` del agente, `Pauses` de la consola).

Pone la **pantalla en blanco** («Pausar pantallas»): el agente cubre todos los monitores con su
ventana de pausa, siempre encima, que tapa lo que hay en pantalla, recibe los clics y explica
por qué.

```json
{ "message": "Atención al pizarrón, por favor." }
```

| Campo | Tipo | Regla |
|---|---|---|
| `message` | string | **Opcional**, 0–300 caracteres, texto plano. Se muestra debajo del texto fijo. Nunca se registra. |

El texto fijo es «Tu profesor pausó esta computadora», con el profesor y el canal, que el
alumno siempre sabe quién la pausó (`docs/privacidad.md`, regla 9). Un `screen.freeze` sobre
una pantalla ya en pausa **solo cambia el mensaje**, sin cerrar la ventana. Responde
`screen.freeze.ack` con `frozen: true`; si no pudo mostrar la ventana, `error` `internal` con
ese `ref_id` y la pantalla queda como estaba.

Reglas del agente:

- **Una ventana por monitor**, con su DPI; si cambian los monitores, se vuelven a cubrir.
- **Vuelve a quedar encima cada medio segundo.** Toma el foco al aparecer y cuando el alumno
  hace clic en ella; no lo pide en cada vuelta (las reglas de Windows contra el robo de foco lo
  impedirían a medias, y pelearía con la cuenta regresiva de energía).
- **No intercepta el teclado** (D-013): el alumno podría cambiar de ventana con Alt+Tab y
  escribir a ciegas. Cerrar Aula Cliente (por ejemplo, desde el Administrador de tareas) la
  quita, como cualquier cierre del agente, y la consola lo ve como desconexión.
- **Cede el paso a lo que el alumno debe ver.** Mientras el equipo recibe una transmisión
  (§5.12, también la pantalla de un compañero, §5.24), hay una cuenta regresiva de energía
  (§5.15) o un control remoto (§5.16), la pausa queda justo **debajo** de la ventana de Aula en
  vez de encima de todo; al terminar, vuelve a cubrir todo.
- **Al cerrarse la conexión, el agente quita la pausa** por sí mismo: si la consola se cierra o
  se cae la red, el alumno no queda bloqueado. Como servicio, el ayudante la quita también si se
  pierde el canal local con el servicio.

Reglas de la consola:

- No lo envía a un equipo que se está proyectando a la clase (§5.24): su pantalla es la que ven
  los demás.
- Antes de `remote.start` a un equipo en pausa, envía `screen.unfreeze` (el profesor quiere ver
  y usar la pantalla real).
- Una sesión que se reconecta empieza sin pausa; la consola no la reenvía sola.

### 5.10 `screen.unfreeze` (P → A)

Payload `{}`. Quita la pantalla en blanco. Es idempotente: siempre responde `screen.freeze.ack`
con `frozen: false`.

### 5.11 `screen.freeze.ack` (A → P)

```json
{ "ref_id": "<id del screen.freeze o screen.unfreeze>", "frozen": true }
```

Confirma el estado en que quedó la pantalla: `true` tras `screen.freeze` y `false` tras
`screen.unfreeze`. La consola cuenta como «sin acusar» a quien no respondió en 5 s.

### 5.12 Transmisión de la pantalla del profesor (`screen.broadcast.*`)

> Implementado en v0.4 (Windows), aprobado el 2026-10-29. Decisión técnica en D-014;
> implementación en `internal/broadcast`.

El profesor transmite un monitor suyo como video WebRTC a los agentes del canal que
declararon `screen.broadcast` (todos o los seleccionados, §5.12.1). El WebSocket autenticado lleva **solo la señalización**; el
video va por WebRTC (SRTP sobre UDP), directo de la consola a cada alumno, dentro de la LAN.

```
Consola (P, pion)                                Agente (A, RTCPeerConnection en WebView2)
   |-- screen.broadcast.start ---------------------->|  muestra la vista y el aviso (antes de todo)
   |-- screen.broadcast.offer (attempt=1) ---------->|  valida el SDP (solo video, recvonly)
   |<--------------------- screen.broadcast.answer --|
   |<-- screen.broadcast.ice (ambos sentidos) ------>|
   |================ SRTP: video H.264 =============>|
   |-- screen.broadcast.stop ----------------------->|  cierra la conexión, quita vista y aviso
```

Reglas generales:

- **La consola ofrece y el agente responde.** La oferta es `sendonly` con **una sola** sección
  `m=video`. El agente rechaza (`error` `invalid_message`) una oferta con audio, canal de datos
  (`m=application`) o más de un video: nunca envía medios ni datos propios.
- **Sin servidores externos:** no hay STUN ni TURN; solo candidatos `host` de la LAN. La consola
  es ICE-lite y escucha en **un solo puerto UDP, `ws_port + 1`** (47831 por defecto). La regla
  de firewall del instalador de la consola es por programa, así que ya lo cubre.
- **La transmisión vive dentro de la sesión.** Si el WebSocket se cierra, el agente cierra la
  conexión WebRTC y quita la vista y el aviso. Cuando el agente reconecta (§3.5) y la
  transmisión sigue activa, la consola le envía de nuevo `start` y `offer`.
- **Reintento sin perder la sesión:** si la conexión WebRTC de un agente pasa a `failed`, o queda
  `disconnected` más de 5 s, la consola la descarta y envía una oferta nueva con `attempt + 1`,
  con retroceso 1, 2, 4… 30 s. El agente cierra la conexión anterior al recibir una oferta con un
  `attempt` mayor y descarta `ice` y `answer` de intentos viejos.
- Una sola transmisión a la vez por consola. Un `start` con otro `broadcast_id` reemplaza a la
  anterior (el agente cierra la vieja).
- **Medios** (D-014): H.264 Constrained Baseline (`profile-level-id=42e028`,
  `packetization-mode=1`), hasta 1920 px de ancho a la resolución nativa del monitor, hasta 15
  cuadros/s, tope de 2,5 Mbit/s (`broadcast_max_kbps`, `docs/configuracion.md`), cuadro clave cada 5 s y ante PLI/FIR
  (como máximo uno por segundo para todos los receptores). Una pantalla que no cambia se codifica
  una vez por segundo. Sin audio.

#### 5.12.1 `screen.broadcast.start` (P → A)

Avisa que empieza una transmisión. El agente **debe** mostrar la vista de transmisión y su aviso
(`docs/privacidad.md` regla 12) **antes** de procesar la oferta.

```json
{ "broadcast_id": "7d1f…", "width": 1920, "height": 1080, "fps": 15 }
```

| Campo | Tipo | Regla |
|---|---|---|
| `broadcast_id` | string | UUID v4 que asigna la consola a la transmisión. |
| `width`, `height` | entero | Tamaño del video, 1–4096. Informativo: el agente ajusta la vista. |
| `fps` | entero | Cuadros por segundo previstos, 1–30. Informativo. |
| `source` | objeto, opcional | Ausente: la pantalla es del profesor. `{"kind": "student", "hostname": "LAB3-PC07"}`: es la de un compañero (§5.24); la vista lo dice. Solo se envía a agentes con `screen.present.view`. `hostname`: 1–63 caracteres. |
| `mode` | string, opcional | Cómo se muestra la vista (v0.15; `docs/privacidad.md` regla 12). `"window"`: una ventana siempre encima de las demás, que el alumno puede mover y redimensionar pero no minimizar ni cerrar. `"fullscreen"`: cubre el monitor del alumno, encima de todo, sin minimizar ni cerrar. **Ausente** (una consola anterior): la ventana maximizada de v0.4, que el alumno puede minimizar. Otro valor → `error` `invalid_message`. Un agente anterior ignora el campo y usa la de v0.4. |

Sin respuesta propia: la sigue `screen.broadcast.offer`. Si el agente no puede mostrar la vista,
responde `error` `internal` y no procesa la oferta.

**A quién** (v0.15). El destino no viaja en el mensaje: la consola solo envía `start` a quien lo
recibe. Para **todos**, a toda sesión conectada con `screen.broadcast`, también a las que se unen
después. Para **los seleccionados**, a los equipos (por nombre de equipo) que el profesor tenía
seleccionados al empezar, también si se reconectan; uno que no estaba seleccionado no la recibe
aunque se una después.

**Se libera sola** (`"window"` y `"fullscreen"`, regla 10 de privacidad). La vista y lo que
impide minimizar o cerrar duran solo lo que dura la transmisión **en esta sesión**: terminan con
`screen.broadcast.stop`, con un `start` de otra transmisión, al cerrarse el WebSocket y cuando la
consola deja de responder (el plazo de inactividad de §3.4, 15 s sin mensajes, cierra la
conexión). Al terminar, la ventana del agente vuelve a como estaba antes del `start` (tamaño,
posición, modo compacto u oculta en el área de notificación); no cierra ni toca otros programas.

#### 5.12.2 `screen.broadcast.offer` (P → A)

```json
{ "broadcast_id": "7d1f…", "attempt": 1, "sdp": "v=0\r\no=- 46117…" }
```

| Campo | Tipo | Regla |
|---|---|---|
| `broadcast_id` | string | El del `start` vigente. Si no coincide → `error` `invalid_message`. |
| `attempt` | entero | 1, 2, 3… por transmisión y agente; 1–1000. |
| `sdp` | string | Oferta SDP completa, ≤ **64 KiB**. Una sola `m=video`, `a=sendonly`. |

#### 5.12.3 `screen.broadcast.answer` (A → P)

```json
{ "ref_id": "<id de la offer>", "broadcast_id": "7d1f…", "attempt": 1, "sdp": "v=0\r\n…" }
```

Respuesta SDP (`recvonly`), ≤ 64 KiB. La consola descarta respuestas de un `attempt` que ya no
es el vigente. Si el agente no puede crear la conexión, responde `error` `internal` con el
`ref_id` de la oferta y la consola reintenta con el retroceso de §5.12.

#### 5.12.4 `screen.broadcast.ice` (A ↔ P)

Un candidato ICE (trickle). La consola lo envía solo a agentes con `screen.broadcast`.

```json
{ "broadcast_id": "7d1f…", "attempt": 1,
  "candidate": "candidate:1 1 udp 2130706431 192.168.1.20 47831 typ host",
  "sdp_mid": "0", "sdp_mline_index": 0 }
```

| Campo | Tipo | Regla |
|---|---|---|
| `broadcast_id`, `attempt` | | Igual que en la oferta. Los de un intento viejo se descartan sin error. |
| `candidate` | string | Línea `candidate:` sin el prefijo `a=`, ≤ 1024 caracteres. `""` = fin de candidatos. |
| `sdp_mid` | string | ≤ 32 caracteres. |
| `sdp_mline_index` | entero | 0–15. |

Como máximo 50 candidatos por intento y sentido; los demás se descartan.

#### 5.12.5 `screen.broadcast.stop` (P → A)

```json
{ "broadcast_id": "7d1f…" }
```

El agente cierra la conexión WebRTC y quita la vista y el aviso. Idempotente y sin respuesta;
un `broadcast_id` distinto del vigente se ignora. La consola lo envía al pulsar "Detener", al
cambiar de canal y al cerrar la clase.

### 5.13 Bloqueo de aplicaciones (`app.block.*`)

> Implementado en v0.4 (Windows), aprobado el 2026-10-29. Decisión técnica en D-017;
> implementación en `internal/policy`; avisos en `docs/privacidad.md` reglas 20–25.

Reglas comunes a §5.13 y §5.14:

- **Una política por tipo**, de la clase: un `set` nuevo reemplaza la anterior del mismo tipo.
- **Vive con la sesión.** El agente la quita sola con `*.clear`, al cerrarse la conexión con la
  consola, al irse el ayudante (cierre de sesión) o al detenerse el servicio. Si el agente se
  cae, la quita al volver a arrancar (D-017). Cuando el alumno se reconecta, **la consola le
  vuelve a enviar** la política vigente de la clase.
- **Aviso antes de aplicar.** El agente muestra el aviso de restricciones activas
  (`docs/privacidad.md` regla 20) y después aplica la política.
- La consola puede enviarla a todo el canal o a alumnos concretos: es el mismo mensaje enviado a
  cada sesión.

**`app.block.set` (P → A)**

```json
{ "policy_id": "3f1c…", "mode": "deny", "apps": ["steam", "Discord.exe", "chrome"] }
```

| Campo | Tipo | Regla |
|---|---|---|
| `policy_id` | string | UUID v4 de la consola. |
| `mode` | string | `deny`: se cierran las apps de la lista. `allow`: solo pueden correr las de la lista (más las protegidas, abajo). |
| `apps` | string[] | Nombres de proceso, 1–100, sin duplicados (sin distinguir mayúsculas). Cada uno `^[A-Za-z0-9 ._+-]{1,64}$`, sin ruta. `.exe` es opcional: `chrome` y `chrome.exe` son lo mismo. |

- Solo afecta a procesos de la **sesión del alumno**, nunca a los del sistema ni de otras
  sesiones.
- **Protegidos** (nunca se cierran, aunque estén en la lista o falten en `allow`): el propio
  agente y su WebView2, los procesos del sistema (`%SystemRoot%`) y el shell (`explorer.exe`).
- Una app bloqueada que ya está abierta **se cierra**. Una que se abre después se cierra en
  menos de 1 s; el alumno ve el aviso de cuál y por qué (regla 22).

**`app.block.clear` (P → A)**: payload `{}`. Quita la política de apps; es idempotente.

**`app.block.ack` (A → P)**

```json
{ "ref_id": "<id del set o del clear>", "policy_id": "3f1c…", "active": true }
```

`active` dice si quedó una política aplicada; `policy_id` va vacío tras `clear`. Si no se pudo
aplicar, el agente responde `error` `internal`, no deja nada a medias y no envía el `ack`.

### 5.14 Filtro web (`web.filter.*`)

**`web.filter.set` (P → A)**

```json
{ "policy_id": "8a2e…", "mode": "allow", "domains": ["python.org", "docs.python.org", "w3schools.com"] }
```

| Campo | Tipo | Regla |
|---|---|---|
| `policy_id` | string | UUID v4 de la consola. |
| `mode` | string | `deny`: se bloquean estos dominios. `allow`: solo se permiten estos. |
| `domains` | string[] | 1–500 nombres de dominio, sin esquema, ruta ni puerto, en minúsculas o con mayúsculas (se comparan sin distinguir), ≤ 253 caracteres cada uno, etiquetas válidas (punycode para acentos). **Cada dominio incluye sus subdominios**: `python.org` cubre `docs.python.org`. |

- El filtro actúa **por nombre** (DNS, D-017): un dominio bloqueado no se resuelve y el
  navegador muestra que no encontró el sitio. No mira rutas ni contenido, ni descifra HTTPS.
- En `allow`, los sitios que dependen de otros dominios (fuentes, CDN) pueden verse a medias: hay
  que agregar esos dominios a la lista.

**`web.filter.clear` (P → A)**: payload `{}`. Quita el filtro y restablece el equipo; idempotente.

**`web.filter.ack` (A → P)**: igual que `app.block.ack`.

### 5.15 Energía: apagar, reiniciar y cerrar sesión (`power.*`)

> Implementado en v0.5 (Windows), aprobado el 2026-10-29. Decisión técnica en D-018;
> implementación en `internal/power`; avisos en `docs/privacidad.md` reglas 26–31.

```
Consola (P)                                   Agente (A)
   |-- power.action (reboot, 30 s) ------------->|  muestra el aviso con la cuenta
   |<---------------------- power.ack countdown --|
   |<---------------------- power.ack postponed --|  el alumno pospuso (+5 min)
   |<---------------------- power.ack executing --|  justo antes de llamar al SO
   |                                              |  reinicia
```

**`power.action` (P → A)**

```json
{ "action_id": "c2e1…", "action": "reboot", "delay_s": 30, "force": false }
```

| Campo | Tipo | Regla |
|---|---|---|
| `action_id` | string | UUID v4 de la consola; identifica la orden en `power.ack` y `power.cancel`. |
| `action` | string | `shutdown` (apagar), `reboot` (reiniciar) o `logoff` (cerrar sesión). |
| `delay_s` | entero | **Opcional.** Duración de la cuenta regresiva, 10–600 s. Ausente o `0` = **30 s**. Con `force` se ignora. |
| `force` | booleano | `true`: sin cuenta regresiva y cerrando las aplicaciones aunque tengan cambios sin guardar. `false` (o ausente): aviso con cuenta regresiva y cierre normal, en el que Windows deja que las apps pidan guardar. |

**`power.cancel` (P → A)**

```json
{ "action_id": "c2e1…" }
```

Anula la cuenta regresiva de esa orden: el agente quita el aviso y responde `power.ack`
`cancelled`. Si la orden vigente es otra, o no hay ninguna (o ya se está ejecutando), se ignora.

**`power.ack` (A → P)**

```json
{ "ref_id": "<id del power.action>", "action_id": "c2e1…", "status": "postponed",
  "remaining_s": 325, "postpones_left": 1 }
```

| `status` | Cuándo | `remaining_s` |
|---|---|---|
| `countdown` | El aviso ya se muestra y empezó la cuenta. | Segundos que faltan. |
| `postponed` | El alumno pospuso 5 minutos. | Los que faltan ahora. |
| `executing` | Se envía **justo antes** de llamar al SO. Con `force` (o sin sesión iniciada) es el único. | `0` |
| `cancelled` | Respuesta a `power.cancel` (`ref_id` = el del `cancel`), o la orden fue reemplazada por otra nueva (`ref_id` = el de la orden vieja). | `0` |

`postpones_left` dice cuántos aplazamientos le quedan al alumno (2, 1 o 0). Si el SO rechaza la
acción, el agente responde `error` `internal` con el `ref_id` de la orden y el motivo que dio el
SO en `message` (sin datos del alumno), y el aviso lo dice.

Reglas:

- **Una orden a la vez.** Una `power.action` nueva reemplaza a la vigente: el agente envía
  `cancelled` de la vieja y empieza la nueva (con `force`, se ejecuta de inmediato).
- **Posponer:** el alumno puede sumar 5 minutos a lo que falta, como máximo **2 veces** por orden.
- **La cuenta sigue si se cae la conexión.** La orden ya vino de la consola autenticada: el aviso
  sigue y se puede posponer, y al terminar se ejecuta. Los `ack` que no se pueden enviar solo
  quedan en el log del agente.
- **Sin sesión iniciada** (agente como servicio sin ayudante, D-016): el agente declara solo
  `power`. `shutdown` y `reboot` se ejecutan de inmediato, porque no hay trabajo que perder ni
  nadie que vea el aviso; `logoff` → `error` `unsupported_capability`.
- **No se reenvía al reconectar** (a diferencia de §5.13–5.14): es una orden puntual.
- Antes de `auth.ok`, como cualquier otro mensaje, → `error` `unauthorized` y cierre `1008`
  (§3.1). El agente registra la orden y envía `executing` antes de llamar al SO, y espera hasta
  2 s a que el `ack` salga.

#### 5.15.1 Permisos del sistema operativo

| Acción | Windows | Requiere |
|---|---|---|
| `shutdown`, `reboot` | `InitiateShutdownW` sin período de gracia (la cuenta ya la hizo Aula); con `force`, `SHUTDOWN_FORCE_OTHERS` y `SHUTDOWN_FORCE_SELF`. Motivo: "otro, planificado". **Sin `force` y con sesión iniciada se llama desde la sesión del alumno** (en modo servicio, el ayudante): desde SYSTEM, Windows lo rechaza con `ERROR_SHUTDOWN_USERS_LOGGED_ON` (1190) porque el alumno es "otro usuario". Con `force`, o sin sesión, lo llama el servicio. | **`SeShutdownPrivilege`**, que el agente habilita en su token. SYSTEM (el servicio) lo tiene; los usuarios estándar de un equipo de escritorio también, salvo que una GPO lo quite ("Apagar el sistema"); si la quita, el apagado sin forzar falla y la consola muestra el motivo. No se usa `SeRemoteShutdownPrivilege`: se ejecuta localmente. |
| `logoff` sin `force` | `ExitWindowsEx(EWX_LOGOFF)` desde la sesión del alumno (en modo servicio lo hace el ayudante). | Ninguno: es la propia sesión. |
| `logoff` con `force` | `WTSLogoffSession` sobre la sesión del alumno. | SYSTEM o administrador para otra sesión (el servicio); la propia sesión, sin permisos. |

Sin `force`, una app con cambios sin guardar puede detener el apagado en la pantalla de Windows
"Esta aplicación impide…". Es intencional: no se pierde trabajo, y el profesor puede reenviar la
orden con `force`.

#### 5.15.2 Encender equipos (Wake-on-LAN)

> Sin publicar (prompt 10.19). Implementación en `internal/power` (`MagicPacket`, `Machines`,
> `Waker`); privacidad en `docs/privacidad.md` regla 31 bis.

Un equipo apagado no tiene agente, así que encenderlo **no es un mensaje del protocolo**: la
consola manda el «paquete mágico» por la red local. Solo usa de este protocolo el campo `macs`
de `hello` (§3.3).

1. **El agente informa su MAC.** Antes de enviar `hello`, elige la tarjeta de red cuya subred
   contiene la dirección de la consola a la que se conectó y pone su MAC en `macs`. Si ninguna
   la contiene (la consola está tras un router), pone las de sus tarjetas activas con IPv4, sin
   la de bucle local, hasta 4. Lee las tarjetas por `platform.Interfaces` (Windows; en los demás
   SO no manda `macs`).
2. **La consola recuerda el equipo.** Por canal y por hostname (sin mayúsculas): las MAC, la
   última IP y la última vez que se vio, en `equipos-conocidos.json` de su carpeta de
   configuración. Un `hello` sin `macs` no borra las que ya tenía. Nunca guarda alumnos simulados.
3. **«Encender» manda el paquete mágico** a cada MAC del equipo: datagrama UDP al **puerto 9**
   con 6 bytes `FF` seguidos de **16 veces** la MAC (102 bytes). Va a la dirección de broadcast
   de la tarjeta de la consola cuya subred contiene la última IP del equipo; si ninguna la
   contiene, a `255.255.255.255` y la consola avisa «otra subred» (el broadcast no cruza routers).
   Se manda 3 veces, con 1 s entre cada una; como máximo **64 equipos** por orden.
4. **«Encendiendo…» dura hasta 3 minutos.** Termina cuando llega un `hello` de ese hostname en el
   canal; si no llega, queda «No respondió», con el enlace al manual.

No hay acuse: la tarjeta de red no contesta. La consola nunca manda otra cosa por UDP, y el
agente nunca manda UDP. El relevo por un agente de la misma subred queda como propuesta
(`docs/ux/PROPUESTAS.md`).

### 5.16 Control remoto (`remote.*`)

> Implementado en v0.6 (Windows), aprobado el 2026-11-05. Decisión técnica en D-019;
> implementación en `internal/remote`; avisos en `docs/privacidad.md` reglas 36–43.

El profesor ve en vivo la pantalla de **un** alumno y maneja su ratón y su teclado para ayudarlo.
El video es la vista en grande de siempre (`screen.thumb.frame`, §5.7) a un ritmo más rápido; la
entrada va en lotes de eventos por el mismo WebSocket autenticado. **No se pide consentimiento**:
es una acción del profesor sobre un equipo del centro de cómputo, como la energía o las
restricciones, y el alumno lo ve siempre (banda y distintivo, antes del primer evento).

```
Consola (P)                                   Agente (A)
   |-- remote.start (control_id, video) --------->|  raise, banda y distintivo
   |<----------------------- remote.ack active ---|
   |<== screen.thumb.frame (ref_id = remote.start)|  hasta 10 por segundo
   |-- remote.input (lotes de eventos) ---------->|  inyecta en la sesión del alumno
   |<----------------------- remote.ack paused ---|  el alumno usa su ratón o su teclado
   |<---------------------- remote.ack resumed ---|  2 s después de su último movimiento
   |-- remote.stop ------------------------------>|  suelta teclas y botones, quita la banda
   |<----------------------- remote.ack ended ----|
```

**`remote.start` (P → A)**

```json
{ "control_id": "5b0e…", "interval_ms": 125, "quality": 60, "max_width": 1280 }
```

| Campo | Tipo | Regla |
|---|---|---|
| `control_id` | string | UUID v4 de la consola; identifica el control en `remote.input`, `remote.stop` y `remote.ack`. |
| `interval_ms` | entero | 100–1000: como máximo **10 imágenes por segundo**. |
| `quality` | entero | Calidad JPEG, 10–95. |
| `max_width` | entero | 640–1920. La imagen se reduce conservando la proporción; nunca se amplía. |

Valores de la consola: `125 / 60 / 1280` con la vista abierta; `1000 / 50 / 640` con la vista
cerrada (el control sigue). Un `remote.start` con el **mismo** `control_id` solo cambia el video
(sin otro `ack`); con otro `control_id` reemplaza al anterior, que termina (`ended`, `reason`
`replaced`).

El agente envía las imágenes como `screen.thumb.frame` con `ref_id` = el `id` del
`remote.start` que abrió el control; reemplazan a las miniaturas mientras dure (la consola no le
pide `screen.thumb.start` a ese equipo, y si llega uno, el agente lo atiende al terminar el
control). Si una imagen sigue esperando a salir cuando toca la siguiente, el agente se salta esa
captura: el video nunca se atrasa.

**`remote.input` (P → A)**

```json
{ "control_id": "5b0e…", "seq": 41, "events": [
  { "t": "move", "x": 0.5123, "y": 0.3301 },
  { "t": "button", "b": "left", "down": true, "x": 0.5123, "y": 0.3301 },
  { "t": "wheel", "dx": 0, "dy": -120 },
  { "t": "key", "code": "KeyA", "down": true } ] }
```

| Campo | Tipo | Regla |
|---|---|---|
| `control_id` | string | UUID v4. Si no es el del control vigente, el mensaje se descarta sin error. |
| `seq` | entero | 1, 2, 3… por control. El agente descarta los que llegan con `seq` menor o igual al último. |
| `events` | arreglo | 1–32 eventos, en orden. |

| `t` | Campos | Regla |
|---|---|---|
| `move` | `x`, `y` | Posición absoluta, 0–1, relativa al **monitor principal** (el que se ve). |
| `button` | `b`, `down`, `x`, `y` | `b`: `left`, `right` o `middle`. Mueve a (`x`, `y`) y presiona o suelta. |
| `wheel` | `dx`, `dy` | −1200 a 1200, en unidades de la rueda (120 = un paso; `dy` positivo = hacia arriba, `dx` positivo = a la derecha, como la rueda de Windows). Al menos uno distinto de 0. |
| `key` | `code`, `down` | `code` es un `KeyboardEvent.code` de la tabla cerrada de teclas físicas de `internal/protocol` (letras, dígitos, F1–F12, flechas, edición, teclado numérico, modificadores y puntuación de un teclado de 102 teclas). Se inyecta por *scancode*: la distribución del teclado del alumno decide el carácter. Otro `code` → `invalid_message`. |

No tiene respuesta. Un mensaje mal formado → `error` `invalid_message` (la sesión sigue).

**`remote.stop` (P → A)**

```json
{ "control_id": "5b0e…" }
```

Termina el control: el agente suelta todo lo que presionó, deja de inyectar, detiene el video,
quita la banda y el distintivo y responde `remote.ack` `ended` (`reason` `teacher`). Con otro
`control_id`, o sin control vigente, se ignora.

**`remote.ack` (A → P)**

```json
{ "ref_id": "<id del remote.start o remote.stop>", "control_id": "5b0e…", "status": "paused" }
```

| `status` | Cuándo | `reason` |
|---|---|---|
| `active` | La banda y el distintivo ya se ven y el agente acepta entrada. | — |
| `paused` | El alumno movió su ratón o pulsó una tecla: la entrada del profesor se descarta. | — |
| `resumed` | Pasaron 2 s sin entrada del alumno. | — |
| `ended` | Terminó el control. | `teacher` (`remote.stop`), `replaced` (otro `remote.start`), `idle` (5 min sin `remote.input`), `session` (se fue el ayudante: la sesión del alumno se cerró o cambió), `error` (el SO rechazó la entrada durante 30 s seguidos, p. ej. con la pantalla bloqueada o el aviso de UAC). |

Todos los `ack` llevan como `ref_id` el `id` del `remote.start` que abrió el control, salvo
`ended` por `remote.stop`, que lleva el del `stop`.

Reglas:

- **Uno a la vez.** Cada consola controla como máximo un equipo; cada agente acepta un control.
- **Aviso antes de la entrada.** El agente muestra la banda y el distintivo y trae su ventana al
  frente **antes** de responder `active`; descarta toda entrada anterior a ese momento.
- **El alumno tiene prioridad, pero no corta.** Su entrada pausa la del profesor (`paused` /
  `resumed`) sin terminar el control. Desde Aula, el alumno no puede terminarlo.
- **Nada queda presionado.** Al pausar, terminar, cerrarse la conexión o cerrarse el agente, el
  agente suelta cada tecla y botón que presionó.
- **Termina con la conexión.** Si el WebSocket se cierra, el control termina de inmediato y
  **no se reanuda** al reconectar.
- **Límite de ritmo.** La consola junta los movimientos seguidos y envía como máximo 60
  `remote.input` por segundo. El agente descarta, sin error, los que excedan 60 por segundo y los
  que lleguen en pausa o fuera de un control activo, y lo registra una vez por control.
- Antes de `auth.ok`, como cualquier otro mensaje, → `error` `unauthorized` y cierre `1008`.
- **Una captura no interrumpe el control.** Durante el control, la consola puede pedir
  `screen.snapshot.request` (§5.17) al mismo equipo: no termina ni pausa el control, y el video
  sigue (como mucho se salta un cuadro mientras se codifica la captura).

#### 5.16.1 Permisos del sistema operativo

| SO | Cómo | Requiere y límites |
|---|---|---|
| Windows | `SendInput` con `MOUSEEVENTF_ABSOLUTE` y `KEYEVENTF_SCANCODE`, **desde la sesión del alumno**: en modo servicio lo hace el ayudante (D-016), con el token del alumno y sin elevar; en modo ventana, el propio agente. Sin ayudante no se declara `remote`. | Ningún privilegio. Windows **no** deja inyectar Ctrl+Alt+Supr ni entrada en el escritorio seguro (pantalla de bloqueo, aviso de UAC). Por UIPI, el ayudante no controla ventanas de administrador, y Windows no avisa del fallo. Las teclas que atiende el Windows del profesor (Win, Alt+Tab, Ctrl+Alt+Supr) no llegan a la consola y no se envían. |
| Linux (pendiente) | X11: extensión XTest. | Wayland no deja que un cliente inyecte entrada: haría falta el portal `org.freedesktop.portal.RemoteDesktop`, que pide aprobación en cada sesión. No se declara `remote`. |
| macOS (pendiente) | `CGEventPost`. | Permisos de **Accesibilidad** y **Grabación de pantalla** (TCC), que concede el usuario o un perfil MDM. No se declara `remote`. |

### 5.17 Captura de pantalla de un alumno (`screen.snapshot.*`)

> Implementado en v0.7 (Windows), aprobado el 2026-11-05. Implementación en `internal/snapshot`;
> avisos en `docs/privacidad.md` reglas 7 y 44–49.

El profesor pide **una** imagen fija, a resolución completa, de la pantalla de un alumno, para
revisarla o guardarla como evidencia. A diferencia de las miniaturas (§5.6), que son un flujo
efímero en memoria, la consola **puede guardar** esta imagen en su disco, pero solo cuando el
profesor pulsa «Guardar». El agente nunca la escribe en disco.

```
Consola (P)                                        Agente (A)
   |-- screen.snapshot.request (snapshot_id) ------->|  distintivo «tomando una captura» (5 s)
   |                                                 |  captura el monitor principal, en memoria
   |                                                 |  aviso en la ventana del alumno (notify)
   |<-- screen.snapshot.response part 1/N (+ datos) -|
   |<-- screen.snapshot.response part 2/N -----------|
   |<-- …                         part N/N ----------|  libera la imagen
   |  ensambla, comprueba size y sha256,             |
   |  vista previa; «Guardar» la escribe en disco    |
```

**`screen.snapshot.request` (P → A)**

```json
{ "snapshot_id": "9a3c…", "quality": 90, "max_width": 0 }
```

| Campo | Tipo | Regla |
|---|---|---|
| `snapshot_id` | string | UUID v4 de la consola; identifica la captura en sus partes. |
| `quality` | entero | Calidad JPEG, 50–95. La consola envía **90**. |
| `max_width` | entero | `0` = resolución nativa del monitor (lo que envía la consola), o 640–7680. La imagen se reduce conservando la proporción; nunca se amplía. |

**`screen.snapshot.response` (A → P)**

```json
{ "ref_id": "<id del request>", "snapshot_id": "9a3c…", "part": 1, "parts": 4,
  "taken_at": 1793883726000, "hostname": "LAB3-PC07", "username": "alumno07",
  "width": 1920, "height": 1080, "size": 1843211,
  "sha256": "5f0c…", "data": "/9j/4AAQSkZJRgABAQ…" }
```

| Campo | Tipo | Regla |
|---|---|---|
| `ref_id` | string | `id` del `screen.snapshot.request`. |
| `snapshot_id` | string | El del pedido. |
| `part`, `parts` | entero | 1 ≤ `part` ≤ `parts` ≤ 32. Las partes salen en orden. |
| `data` | string | Un trozo del JPEG en Base64: ≤ **512 KiB** antes de codificar, no vacío. Todos los trozos miden 512 KiB salvo el último. |
| `taken_at` | entero | Solo en la parte 1: hora de la captura en milisegundos Unix. |
| `hostname`, `username` | string | Solo en la parte 1: el equipo (1–255) y el usuario de la sesión (0–256) en el momento de la captura. |
| `width`, `height` | entero | Solo en la parte 1: tamaño de la imagen, 1–8192. |
| `size` | entero | Solo en la parte 1: bytes del JPEG completo, 1–16 MiB; `parts` = ⌈`size` / 512 KiB⌉. |
| `sha256` | string | Solo en la parte 1: huella SHA-256 del JPEG completo, 64 caracteres hexadecimales en minúsculas. |

**Troceo.** Un mensaje no puede pasar de 1 MiB (§7) y una captura nativa pesa de 0.5 a 4 MB:
el agente parte el JPEG en trozos de 512 KiB (≈ 683 KiB en Base64, con la envoltura por debajo
de 1 MiB). El troceo es común (`protocol.SplitParts` y `protocol.Reassembly`) para que lo
reutilice la recolección de archivos (`file.collect`).

Reglas del agente:

- **Aviso antes de capturar.** Muestra el distintivo nativo de la regla 6 con «Tu profesor está
  tomando una captura de tu pantalla» durante 5 s, y **después** captura: el distintivo sale en
  la imagen. Si el profesor ya ve la pantalla (miniaturas) o la controla (§5.16), el distintivo
  ya está a la vista y no cambia de texto. Si no lo puede mostrar, no captura y responde `error`
  `internal`. Tras capturar, la ventana del alumno muestra el aviso «Captura de pantalla»
  (`docs/privacidad.md` regla 44), sin quitar el foco.
- Captura el **monitor principal** a `max_width` (o nativo) y la calidad pedida. Si el JPEG pasa
  de 16 MiB, baja la calidad de 10 en 10 hasta 50; si aun así no cabe → `error`
  `message_too_large`.
- **Una a la vez.** Una petición mientras otra se captura o se envía → `error` `internal`
  («ya hay una captura en curso»). No depende de las miniaturas ni del control remoto.
- Si no puede capturar (sesión bloqueada, escritorio seguro) → `error` `internal`.
- La imagen vive **solo en memoria** y se libera al enviar la última parte o al cerrarse la
  conexión. Si la conexión se cierra a mitad, no se reanuda.

Reglas de la consola:

- Solo la pide a equipos que declararon `screen.snapshot`, reales o simulados.
- Ensambla por `snapshot_id` y descarta partes de un pedido que no es suyo, repetidas o fuera de
  rango. Al completar, comprueba `size` y `sha256`; si no coinciden, la captura falla.
- Si no llega completa en **30 s** desde el pedido, la descarta.
- La captura queda **en memoria** hasta que el profesor la guarda o la descarta; se descarta
  también al cerrar la clase o cambiar de canal. Solo «Guardar» la escribe en disco
  (`docs/privacidad.md` reglas 45–46).

#### 5.17.1 Permisos del sistema operativo

| SO | Cómo | Requiere y límites |
|---|---|---|
| Windows | GDI (`BitBlt` del escritorio), la misma captura de las miniaturas (`platform.ScreenCapture`), **desde la sesión del alumno**: en modo servicio la hace el ayudante (`CallScreenCapture`, D-016), con el token del alumno y sin elevar. | Ningún privilegio. La pantalla de bloqueo y el aviso de UAC (escritorio seguro) no se pueden capturar: `error` `internal`. Sin ayudante no se declara `screen.snapshot`. |
| Linux (pendiente) | X11: `XGetImage` del root window. | Sin permiso en X11. En **Wayland** un cliente no puede capturar la pantalla: haría falta el portal `org.freedesktop.portal.Screenshot`, que pide la aprobación del alumno en cada captura o sesión. No se declara. |
| macOS (pendiente) | `CGDisplayCreateImage` / ScreenCaptureKit. | Permiso de **Grabación de pantalla** (TCC), que concede el usuario o un perfil MDM. No se declara. |

### 5.18 Manos y mensajes del alumno (`hand.*`, `msg.student*`)

> Aprobado el 2026-11-05 (prompt 10.6), para v0.8. Implementación en `internal/participation`;
> reglas en `docs/privacidad.md` 50–52. En la consola, la vista «Participación».

El canal de regreso de `msg.show`: el alumno **levanta la mano** (con un motivo breve opcional,
«pedir ayuda») o **escribe un mensaje corto** al profesor. La consola los ordena por llegada; el
profesor baja la mano o responde con `msg.show` y `reply_to` (§5.4) a ese alumno. Lo inicia
siempre el alumno; nada de esto toca su equipo.

```
Agente (A)                                         Consola (P)
   |-- hand.raise (hand_id, note?) ----------------->|  cola de manos: posición por llegada
   |-- msg.student (message_id, text) -------------->|  bandeja: «nuevo»
   |<-- msg.student.ack (message_id) ----------------|
   |<-- msg.show (text, reply_to = message_id) ------|  «Responder»
   |-- msg.ack ------------------------------------->|
   |<-- hand.lower (hand_id, by = teacher) ----------|  «Bajar la mano»
   |-- hand.lower (hand_id, by = student) ---------->|  o la baja el alumno
```

**`hand.raise` (A → P)**

```json
{ "hand_id": "3f1c…", "note": "No me compila el for" }
```

| Campo | Tipo | Regla |
|---|---|---|
| `hand_id` | string | UUID v4 del agente. Uno nuevo cada vez que el alumno levanta la mano. |
| `note` | string | Opcional: 0–140 caracteres, texto plano. Es el «pedir ayuda» con un motivo. |

**`hand.lower` (A ↔ P)**

```json
{ "hand_id": "3f1c…", "by": "teacher" }
```

| Campo | Tipo | Regla |
|---|---|---|
| `hand_id` | string | El de la mano levantada. Si no es la mano vigente, se ignora sin error. |
| `by` | string | `student` (A → P) o `teacher` (P → A). Otro valor, o el que no corresponde al sentido → `error` `invalid_message`. |

**`msg.student` (A → P)**

```json
{ "message_id": "8a2e…", "text": "¿El ejercicio 3 es para hoy?" }
```

| Campo | Tipo | Regla |
|---|---|---|
| `message_id` | string | UUID v4 del agente; identifica el mensaje en el acuse y en `reply_to`. |
| `text` | string | 1–500 caracteres, texto plano (nadie interpreta HTML ni Markdown). |

**`msg.student.ack` (P → A)**

```json
{ "ref_id": "<id del msg.student>", "message_id": "8a2e…" }
```

Confirma que el mensaje **llegó a la consola**; no que el profesor lo leyó.

Reglas del agente:

- Una sola mano a la vez. Mientras está levantada, el botón la baja (`hand.lower` `student`).
- Si se pierde la conexión, la mano sigue levantada en su ventana: al reconectar reenvía
  `hand.raise` con el **mismo** `hand_id` y los `msg.student` que no tuvieron acuse, con su mismo
  `message_id`. Como máximo **5** mensajes esperan acuse; la ventana no deja escribir otro.
- Ritmo: como máximo un `msg.student` cada **3 s** y un `hand.*` por segundo (salvo los
  reenvíos al reconectar).
- `hand.lower` `teacher` con la mano vigente la baja y la ventana lo dice, sin quitar el foco.
- Si la consola responde `unknown_type` (una consola anterior), la ventana dice que esa consola
  no recibe manos ni mensajes y baja la mano.

Reglas de la consola:

- **Una mano por alumno**, identificado como en el resto de la consola (equipo y usuario). La
  cola se ordena por la llegada del primer `hand.raise`; un `hand.raise` repetido (mismo alumno,
  mano vigente) **conserva el lugar** y actualiza `note`. Si el alumno se desconecta, su mano
  se queda en la cola, marcada.
- Recuerda el último `hand_id` que bajó el profesor por alumno: si llega un `hand.raise` con ese
  `hand_id` (el agente lo reenvió al reconectar sin saber que se bajó), responde `hand.lower`
  `teacher` y no lo pone en la cola.
- Deduplica `msg.student` por `message_id`: un reenvío se vuelve a acusar sin duplicarse.
- **Ritmo por agente:** ráfaga de 5 `msg.student` y uno cada 3 s después; ráfaga de 3 `hand.*` y
  uno por segundo después. Lo que excede se descarta y se responde `error` `invalid_message`
  («demasiado seguido»), sin acuse.
- Guarda como máximo **20** mensajes por alumno: al pasar, descarta primero el respondido más
  viejo y, si no hay, el más viejo.
- Todo vive **en memoria**: se descarta al cerrar la clase o cambiar de canal
  (`docs/privacidad.md` reglas 50–52).

### 5.19 Temporizador de ejercicio (`timer.*`)

> Aprobado el 2026-11-05 (prompt 10.8), para v0.9. Implementación en `internal/classtimer`;
> regla en `docs/privacidad.md` 54. En la consola, el panel «Temporizador», su franja y su
> sección de la vista «Participación».

El profesor da un tiempo con nombre a la clase («Ejercicio 1 · 10 min») y cada alumno ve cuánto
falta en una insignia discreta de su ventana. Al llegar a cero, la ventana avisa sin quitar el
foco. Solo informa: nada cambia en el equipo.

**Sincronía por lo que falta, nunca por la hora.** Cada mensaje de la consola lleva
`remaining_ms`: lo que falta **en el momento de enviarlo**, medido con su reloj monotónico. El
agente fija su propio fin con su reloj monotónico (`ahora + remaining_ms`). La hora absoluta no
interviene (`ts` sigue siendo informativa), así que relojes desiguales no cambian la cuenta; en
la LAN el retraso es de milisegundos y la diferencia entre alumnos queda bajo 1 s.

```
Consola (P)                                                 Agente (A)
   |-- timer.start (timer_id, label, duration_s, remaining_ms) -->|  insignia «Ejercicio 1 · 10:00»
   |<----------------------------------- timer.ack (running) -----|
   |-- timer.extend (timer_id, add_s, remaining_ms) ------------->|  toma el nuevo restante
   |<----------------------------------- timer.ack (running) -----|
   |                                         (llega a 0 en el agente)  aviso en la esquina
   |<----------------------------------- timer.ack (ended) -------|
   |-- timer.stop (timer_id) ------------------------------------>|  quita la insignia
   |<----------------------------------- timer.ack (stopped) -----|
```

**`timer.start` (P → A)**

```json
{ "timer_id": "e4b1…", "label": "Ejercicio 1", "duration_s": 600, "remaining_ms": 600000,
  "description": "Resuelve los ejercicios 3 a 5 de la hoja." }
```

| Campo | Tipo | Regla |
|---|---|---|
| `timer_id` | string | UUID v4 de la consola; identifica el temporizador en `extend`, `stop` y `ack`. |
| `label` | string | 0–40 caracteres, texto plano. Vacío: la ventana dice «Tiempo». |
| `description` | string, opcional | Qué tienen que hacer (v0.15, nota de interfaz 18): 0–1000 caracteres, texto plano, con saltos de línea. La ventana la muestra en una tarjeta de la lista mientras el temporizador está, sin quitar el foco. Ausente o vacía: solo la insignia. Un agente anterior la ignora. Va también en el reenvío al reconectar. |
| `duration_s` | entero | El total con el que se lanzó, 10–14 400 s (4 h). Solo informa. |
| `remaining_ms` | entero | Lo que falta al enviar, 1–14 400 000 (4 h). **Manda sobre todo lo demás.** Puede pasar de `duration_s` si se extendió y se reenvía al reconectar. |

**`timer.extend` (P → A)**

```json
{ "timer_id": "e4b1…", "add_s": 300, "remaining_ms": 572000 }
```

| Campo | Tipo | Regla |
|---|---|---|
| `timer_id` | string | El del temporizador vigente. Otro → se ignora sin error. |
| `add_s` | entero | Lo que agregó el profesor, 60–3600 s. Solo informa: el agente **no suma**, toma `remaining_ms`; así un reenvío nunca extiende dos veces. |
| `remaining_ms` | entero | Lo que falta ahora, ya extendido, 1–14 400 000. |

**`timer.stop` (P → A)**

```json
{ "timer_id": "e4b1…" }
```

Quita la insignia y el aviso de fin. Si el vigente es otro, o no hay ninguno, se ignora.

**`timer.ack` (A → P)**

```json
{ "ref_id": "<id del timer.start>", "timer_id": "e4b1…", "status": "running", "remaining_ms": 599980 }
```

| `status` | Cuándo | `remaining_ms` |
|---|---|---|
| `running` | Respuesta a `start` o `extend`: la insignia ya cuenta. | Lo que falta según el agente. |
| `ended` | La cuenta del agente llegó a cero (`ref_id` = el último `start` o `extend`). | `0` |
| `stopped` | Respuesta a `stop`, o el temporizador fue reemplazado por otro `start` (`ref_id` = el `start` viejo). | `0` |

Reglas:

- **Un temporizador por clase.** Un `start` con otro `timer_id` reemplaza al vigente: el agente
  acusa `stopped` del viejo y empieza el nuevo. Un `start` con el **mismo** `timer_id` (el
  reenvío al reconectar) solo corrige lo que falta.
- **El fin lo calcula cada lado.** La consola no envía «terminó»: cuando su cuenta llega a cero,
  la marca como terminada. El agente acusa `ended`. En cero no pasa nada más: no bloquea, no
  recoge ni cambia nada en el equipo.
- `extend` sobre un temporizador terminado lo revive. Lo que falta nunca pasa de 4 h: la
  consola no ofrece extender más allá.
- **La cuenta sigue si se cae la conexión**, porque no tiene efecto en el equipo. Al reconectar
  (§3.5), la consola, si el temporizador sigue corriendo y ese alumno es destinatario, le reenvía
  `timer.start` con el mismo `timer_id` y lo que falta ahora; el agente corrige su cuenta sin
  quitar la insignia. Si en **3 s** después de `hello` no llega ese `timer.start` (se detuvo o
  terminó mientras estaba desconectado), el agente la quita.
- **Destinatarios:** «todos» (el canal, también quien se conecta a media cuenta) o una
  selección, identificada como en el resto de la consola (equipo y usuario). La consola envía el
  mismo mensaje a cada sesión, solo a las que declararon `timer`.
- La ventana del alumno quita sola la insignia de «terminó» a los 5 minutos si no llega
  `timer.stop`.
- Los registros llevan `timer_id`, `duration_s` y `remaining_ms`, nunca `label` ni `description`
  (`docs/privacidad.md` regla 54).

### 5.20 Enviar archivos a los alumnos (`file.push`)

> Aprobado el 2026-11-05 (prompt 10.9), para v0.10. Implementación en `internal/filepush`;
> reglas en `docs/privacidad.md` 55–59. En la consola, la vista «Archivos» del menú.

El profesor envía uno o varios archivos (un dataset, el esqueleto de un ejercicio, una
plantilla) a todos los alumnos o a una selección. Cada agente los guarda **solo** dentro de su
**carpeta de trabajo**, `Documentos\Aula` del usuario de la sesión, y la ventana del alumno
muestra qué llegó y dónde quedó. El agente nunca abre ni ejecuta lo que recibe.

```
Consola (P)                                               Agente (A)
   |-- file.push part 1/N (push_id, file_id, name, dest…) -->|  valida la ruta; reserva memoria
   |-- file.push part 2/N ---------------------------------->|
   |-- …                part N/N --------------------------->|  ensambla, comprueba size y sha256
   |                                                         |  escribe en Documentos\Aula\<dest>\<name>
   |<-- file.push.ack (saved, path) -------------------------|  tarjeta en la ventana (notify)
   |-- file.push (siguiente archivo del envío) ------------->|
```

Un **envío** (`push_id`) agrupa de 1 a 20 archivos; cada archivo (`file_id`) viaja en sus
propias partes y recibe su propio acuse. El troceo es el común (`protocol.SplitParts` y
`protocol.Reassembly`, §5.17), con partes de 512 KiB.

**`file.push` (P → A)**

```json
{ "push_id": "3f2a…", "file_id": "9c1d…", "part": 1, "parts": 2,
  "name": "datos.csv", "dest": "Ejercicio 4", "size": 860160,
  "sha256": "5f0c…", "overwrite": false, "label": "Ejercicio 4", "index": 1, "files": 3,
  "data": "aWQsbm9tYnJl…" }
```

| Campo | Tipo | Regla |
|---|---|---|
| `push_id` | string | UUID v4 de la consola; el mismo para todos los archivos del envío. |
| `file_id` | string | UUID v4 de la consola; identifica el archivo en sus partes y en el acuse. |
| `part`, `parts` | entero | 1 ≤ `part` ≤ `parts` ≤ 32. Las partes salen en orden. |
| `data` | string | Un trozo del archivo en Base64: ≤ **512 KiB** antes de codificar. Todos miden 512 KiB salvo el último; solo un archivo vacío (`size` 0) lleva una única parte con `data` vacío. |
| `name` | string | Solo en la parte 1. Nombre del archivo, 1–100 caracteres (reglas de ruta abajo). |
| `dest` | string | Solo en la parte 1. Subcarpeta dentro de la carpeta de trabajo, con `/` como separador, 0–200 caracteres y hasta 4 niveles. `""` = la carpeta de trabajo misma. |
| `size` | entero | Solo en la parte 1. Bytes del archivo completo, 0–16 MiB; `parts` = máx(1, ⌈`size` / 512 KiB⌉). |
| `sha256` | string | Solo en la parte 1. Huella SHA-256 del archivo completo, 64 caracteres hexadecimales en minúsculas. |
| `overwrite` | bool | Solo en la parte 1. `true`: reemplaza un archivo con el mismo nombre. `false`: si existe, se guarda con otro nombre (`datos (2).csv`). |
| `label` | string | Solo en la parte 1. Nombre del envío para la ventana del alumno (p. ej. la plantilla), 0–80 caracteres, texto plano. |
| `index`, `files` | entero | Solo en la parte 1. Posición del archivo en el envío y total de archivos, 1 ≤ `index` ≤ `files` ≤ 20. |

**`file.push.ack` (A → P)**

```json
{ "ref_id": "<id de la última parte>", "push_id": "3f2a…", "file_id": "9c1d…",
  "status": "saved", "path": "Ejercicio 4/datos (2).csv", "renamed": true, "replaced": false }
```

| Campo | Tipo | Regla |
|---|---|---|
| `ref_id` | string | `id` de la parte que completó el archivo, o de la parte con la que el agente lo rechazó. |
| `push_id`, `file_id` | string | Los del archivo. |
| `status` | string | `saved` (quedó en disco) o `failed`. |
| `path` | string | Con `saved`: la ruta final, relativa a la carpeta de trabajo, con `/`. Con `failed`: vacío. |
| `renamed` | bool | Ya existía y se guardó con otro nombre (`overwrite` era `false`). |
| `replaced` | bool | Ya existía y se reemplazó (`overwrite` era `true`). |
| `reason` | string | Solo con `failed`: uno de la tabla siguiente. |

| `reason` | Cuándo |
|---|---|
| `invalid_path` | `name` o `dest` no pasan las reglas de ruta, o la ruta cruza un enlace o *junction*. |
| `too_large` | El archivo no cabe en la memoria del agente para envíos (32 MiB en curso). |
| `busy` | Ya hay 4 archivos ensamblándose. |
| `incomplete` | No llegaron todas las partes en **60 s** desde la parte 1. |
| `corrupt` | El tamaño o el `sha256` del archivo ensamblado no coinciden. |
| `exists` | Con `overwrite` `false`, ya existen el nombre y sus alternativas hasta `(99)`. |
| `no_space` | El disco está lleno. |
| `denied` | El SO negó la escritura (permisos, archivo abierto por otro programa). |
| `path_too_long` | La ruta absoluta pasa del límite del SO (260 caracteres en Windows). |
| `unsupported` | La carpeta de trabajo no se pudo resolver (sin sesión, sin ayudante). |
| `internal` | Cualquier otro fallo. |

**Reglas de ruta.** Las comprueba la consola antes de enviar y el agente al recibir
(`filepush.CheckName` y `filepush.CheckDest`). Cada segmento de `dest` y el `name`:

- tiene 1–100 caracteres, sin caracteres de control ni `< > : " / \ | ? *`;
- no es `.` ni `..`, ni termina en punto o espacio;
- no es un nombre reservado de Windows (`CON`, `PRN`, `AUX`, `NUL`, `COM1`–`COM9`,
  `LPT1`–`LPT9`), tampoco con extensión (`nul.txt`).

`dest` no empieza ni termina en `/`, no tiene `//`, tiene como mucho 4 niveles y no es absoluta
(ni `C:`, ni UNC). Lo que no cumple se rechaza con `invalid_path`; el agente **nunca** corrige
una ruta. Todos los tipos de archivo se aceptan.

Reglas del agente:

- **Solo en la carpeta de trabajo.** La resuelve en cada archivo (`Documentos\Aula` del usuario
  de la sesión) y la crea si no existe. En modo servicio escribe el **ayudante**, con la identidad
  del alumno, nunca SYSTEM (D-016). No sigue enlaces simbólicos ni *junctions* dentro de la
  carpeta.
- **Atómico.** Escribe en un archivo temporal de la misma carpeta y lo mueve a su nombre: nunca
  queda un archivo a medias. Sin `overwrite` no reemplaza nunca: prueba `nombre (2).ext` hasta
  `(99)`, sin sobrescribir lo que aparezca mientras tanto.
- Ensambla por `file_id`, en memoria, hasta 4 archivos y 32 MiB a la vez. Descarta partes de un
  archivo que no conoce, repetidas o fuera de rango; las de un archivo rechazado se ignoran sin
  más respuesta. Si la conexión se cierra a mitad, descarta lo ensamblado.
- Un `file_id` ya resuelto en esta conexión se acusa de nuevo con el mismo resultado, sin volver
  a escribir.
- Avisa en la ventana (`docs/privacidad.md` regla 56) **después** de guardar, sin quitar el
  foco. Nunca abre ni ejecuta el archivo.

Reglas de la consola:

- Solo envía a equipos que declararon `file.push`, reales o simulados (los simulados acusan sin
  escribir en disco).
- Envía los archivos de un envío **uno tras otro** a cada sesión y atiende hasta 4 sesiones a la
  vez, para no saturar la red. Un envío pesa como máximo 32 MiB.
- Un alumno **tiene** el envío cuando acusó `saved` todos sus archivos. Si no acusa un archivo en
  60 s desde su última parte, cuenta como fallido para ese alumno. Si se reconecta a medias, no
  se reanuda: queda en «Quién falta» y el profesor lo reenvía.

#### 5.20.1 Permisos del sistema operativo

| SO | Cómo | Requiere y límites |
|---|---|---|
| Windows | `Documentos` real (`FOLDERID_Documents`, también redirigida a OneDrive) + `\Aula`, con `CreateFile` y `MoveFileEx` **desde la sesión del alumno**: en modo ventana, el propio agente; en modo servicio, el ayudante (`CallFileWrite`, D-016) con el token del alumno. | Ningún privilegio. Sin sesión iniciada o sin ayudante no se declara `file.push`. |
| Linux (pendiente) | `$XDG_DOCUMENTS_DIR` (o `~/Documents`) + `/Aula`, con `O_NOFOLLOW`. | Sin permisos especiales. No se declara. |
| macOS (pendiente) | `~/Documents/Aula`. | TCC puede pedir acceso a Documentos si el agente no está firmado. No se declara. |

### 5.21 Recoger el trabajo de los alumnos (`file.collect.*`)

> Aprobado el 2026-11-06 (prompt 10.10), para v0.11. Implementación en `internal/collect`;
> reglas en `docs/privacidad.md` 60–65. En la consola, la vista «Archivos», pestaña
> «Recolecciones».

El profesor abre una **recolección** (una fecha y una actividad) y cada equipo crea su carpeta
dentro de la carpeta de trabajo del alumno: `Documentos\Aula\recolecciones\<fecha>\<actividad>`.
El alumno guarda ahí su ejercicio. Cuando el profesor pulsa «Recoger», cada agente lee esa
carpeta (y **solo** esa), manda los archivos en partes y la consola los guarda organizados por
alumno. Se puede recoger las veces que haga falta (`round`).

```
Consola (P)                                                 Agente (A)
   |-- file.collect.open (collect_id, date, activity) -------->|  crea recolecciones/<date>/<activity>
   |<-- file.collect.ack (ready, path) ------------------------|  tarjeta «Guarda tu ejercicio en…» (notify)
   |                    … el alumno trabaja …                  |
   |-- file.collect.request (round, patterns, límites) ------->|  aviso «El profesor solicitó recoger…»
   |                                                           |  lista y lee la carpeta, en memoria
   |<-- file.collect.ack (sending, files, total, skipped) -----|
   |<-- file.collect.response archivo 1, partes 1..N ----------|
   |  ensambla, comprueba size y sha256, guarda                |
   |-- file.collect.response.ack (saved) --------------------->|
   |<-- file.collect.response archivo 2 … ---------------------|
   |-- file.collect.response.ack (saved) --------------------->|  aviso «Tu profesor recogió…» y la
   |                                                           |  tarjeta anota qué y cuándo
```

**La consola nunca manda una ruta.** Manda `date` y `activity`; el agente arma la carpeta
(`recolecciones/<date>/<activity>`, relativa a su carpeta de trabajo) y la comprueba. Las rutas
que vuelven en `file.collect.response.path` son relativas a esa carpeta de la actividad.

**`file.collect.open` (P → A)**

```json
{ "collect_id": "7d3e…", "date": "2026-11-06", "activity": "Ejercicio 4", "patterns": ["*.py"] }
```

| Campo | Tipo | Regla |
|---|---|---|
| `collect_id` | string | UUID v4 de la consola; identifica la recolección en todos sus mensajes. |
| `date` | string | `AAAA-MM-DD`, una fecha válida. La consola manda la del día en que se abrió. |
| `activity` | string | Nombre de la actividad, 1–80 caracteres, con las reglas de ruta de un segmento (§5.20, `filepush.CheckName`). Es el nombre de la carpeta. |
| `patterns` | lista | 0–10 patrones de nombre de archivo (`*.py`, `main.*`), 1–20 caracteres cada uno, sin `/` ni `\`. Vacía = todos los archivos. Solo informa al alumno qué se recogerá. |

El agente crea la carpeta si no existe (como el alumno; en modo servicio, el ayudante) y
responde `file.collect.ack` con `status` `ready` y `path`. Un `open` repetido (la consola lo
reenvía a quien se conecta tarde) no cambia nada y vuelve a acusar `ready`.

**`file.collect.request` (P → A)**

```json
{ "collect_id": "7d3e…", "date": "2026-11-06", "activity": "Ejercicio 4", "round": 1,
  "patterns": ["*.py"], "max_file_size": 1048576, "max_total": 8388608, "max_files": 20 }
```

| Campo | Tipo | Regla |
|---|---|---|
| `collect_id`, `date`, `activity`, `patterns` | | Como en `open`. Aquí `patterns` sí filtra: se recogen los archivos cuyo nombre coincide con alguno, sin distinguir mayúsculas. |
| `round` | entero | 1–1000. Cada «Recoger» de la consola sube la ronda; las partes y acuses la llevan. |
| `max_file_size` | entero | 1 – 16 MiB. Un archivo mayor no se recoge (`skipped`, `too_large`). |
| `max_total` | entero | 1 – 32 MiB. Lo que no cabe después de ordenar por ruta no se recoge (`skipped`, `too_large`). |
| `max_files` | entero | 1–50. Los que pasan del límite, por orden de ruta, no se recogen (`skipped`, `too_many`). |

Si la carpeta no existe, el agente la crea y responde `empty`. Un `request` sin `open` previo
vale (el agente reinició): crea la carpeta igual.

**`file.collect.ack` (A → P)**

```json
{ "ref_id": "<id del open o request>", "collect_id": "7d3e…", "round": 1, "status": "sending",
  "path": "recolecciones/2026-11-06/Ejercicio 4", "files": 2, "total": 3412,
  "skipped": [ { "path": "video.mp4", "size": 52428800, "reason": "too_large" } ] }
```

| Campo | Tipo | Regla |
|---|---|---|
| `ref_id` | string | `id` del `file.collect.open` o del `file.collect.request`. |
| `collect_id` | string | El de la recolección. |
| `round` | entero | `0` para `open`; la del `request`. |
| `status` | string | `ready` (respuesta a `open`), `sending` (siguen `files` archivos), `empty` (nada que recoger) o `failed`. |
| `path` | string | Con `ready`, `sending` y `empty`: la carpeta de la actividad, relativa a la carpeta de trabajo, con `/`. |
| `files`, `total` | entero | Con `sending`: cuántos archivos siguen (1–50) y cuántos bytes en total. |
| `skipped` | lista | Con `sending` y `empty`: 0–50 archivos que coincidían pero no se recogen, con `path`, `size` y `reason` (`too_large`, `too_many`, `link`, `denied`, `offline`, `invalid_path`). |
| `reason` | string | Solo con `failed`: `invalid_path` (fecha o actividad inválidas), `busy` (ya está mandando otra ronda), `denied`, `unsupported` (sin carpeta de trabajo: sin sesión o sin ayudante) o `internal`. |

**`file.collect.response` (A → P)**

```json
{ "collect_id": "7d3e…", "round": 1, "file_id": "4b8a…", "part": 1, "parts": 1,
  "path": "20231234-Ana.py", "size": 3072, "sha256": "5f0c…", "modified_at": 1793955720000,
  "index": 1, "files": 2, "data": "ZGVmIG1haW4oKTo…" }
```

| Campo | Tipo | Regla |
|---|---|---|
| `collect_id`, `round` | | Los del `request`. |
| `file_id` | string | UUID v4 del agente; identifica el archivo en sus partes y en su acuse. |
| `part`, `parts` | entero | 1 ≤ `part` ≤ `parts` ≤ 32. Las partes salen en orden. |
| `data` | string | Un trozo en Base64: ≤ **512 KiB** antes de codificar. Todos miden 512 KiB salvo el último; un archivo vacío lleva una sola parte con `data` vacío. |
| `path` | string | Solo en la parte 1. Ruta relativa a la carpeta de la actividad, con `/`, hasta 4 niveles; cada segmento cumple las reglas de ruta de §5.20. |
| `size` | entero | Solo en la parte 1. 0 – 16 MiB; `parts` = máx(1, ⌈`size` / 512 KiB⌉). |
| `sha256` | string | Solo en la parte 1. 64 caracteres hexadecimales en minúsculas. |
| `modified_at` | entero | Solo en la parte 1. Última modificación, en milisegundos Unix. |
| `index`, `files` | entero | Solo en la parte 1. 1 ≤ `index` ≤ `files` ≤ 50; `files` es el del `ack`. |

**Troceo y ensamblado.** El común (`protocol.SplitParts` y `protocol.Reassembly`, §5.17), con
partes de 512 KiB (≈ 683 KiB en Base64; la envoltura queda por debajo de 1 MiB). El agente manda
**un archivo a la vez**: todas sus partes, y espera su `file.collect.response.ack` (60 s como
mucho) antes del siguiente. La consola ensambla por `file_id` y, al completar, comprueba `size` y
`sha256`.

**`file.collect.response.ack` (P → A)**

```json
{ "ref_id": "<id de la última parte>", "collect_id": "7d3e…", "round": 1, "file_id": "4b8a…",
  "status": "saved" }
```

| Campo | Tipo | Regla |
|---|---|---|
| `ref_id` | string | `id` de la parte que completó el archivo o con la que la consola lo rechazó. |
| `collect_id`, `round`, `file_id` | | Los del archivo. |
| `status` | string | `saved` (la consola lo guardó) o `failed`. |
| `reason` | string | Solo con `failed`: `incomplete`, `corrupt`, `no_space`, `denied` o `internal`. |

Reglas del agente:

- **Solo la carpeta de la actividad.** Lista `recolecciones/<date>/<activity>` hasta 4 niveles,
  **sin seguir** enlaces ni *junctions* (se informan como `skipped` `link`). Fuera de ella no lee
  nada. En modo servicio lee el **ayudante**, con el token del alumno (D-016).
- **Aviso antes de leer.** Al llegar el `request` muestra el aviso «El profesor solicitó recoger
  tu ejercicio en …» y **después** lee (`docs/privacidad.md` regla 61). Al terminar la ronda, el
  aviso «Tu profesor recogió…» y la tarjeta anota qué se recogió y cuándo (regla 62).
- **Solo lee.** No borra, no mueve ni cambia nada. Lee todos los archivos de la ronda en memoria
  (≤ 32 MiB) antes de responder, así los `files` y `total` del `ack` son exactos. Un archivo que
  no se puede leer (abierto en exclusiva por otro programa) va en `skipped` `denied`; uno que solo
  está en la nube (OneDrive «a petición»), en `skipped` `offline`, sin descargarlo.
- **Una ronda a la vez.** Otro `request` mientras manda → `ack` `failed` `busy`. Si un archivo no
  se acusa en 60 s, o la conexión se cierra, abandona la ronda y libera la memoria.

Reglas de la consola:

- Solo manda `file.collect.*` a equipos que declararon `file.collect`, reales o simulados (los
  simulados devuelven como `<equipo>.<ext>` el último código que recibieron).
- Manda el `request` en **tandas de 4 equipos**: al terminar uno, pide al siguiente. Si el `ack`
  no llega en 30 s, o un archivo no termina de llegar en 60 s desde su parte 1, ese alumno queda
  como fallido en esa ronda.
- Ensambla en memoria y descarta partes de un archivo que no conoce, repetidas o fuera de rango.
- **Guarda al llegar** cada archivo íntegro en su carpeta personal:
  `Documentos\Aula\recolecciones\<date>\<activity>\<equipo> (<usuario>)\<path>` (`… (simulado)`
  para los simulados), y al terminar la ronda escribe allí `aula-recoleccion.json` con la ronda,
  la hora y cada archivo con su `sha256`. Una ronda nueva reemplaza lo de la anterior de ese
  alumno (lo que ya no está en la ronda nueva se borra de la copia del profesor, nunca del alumno).
- Reenvía `file.collect.open` de las recolecciones abiertas de la clase a quien se conecta tarde.
  Al cerrar la clase, las recolecciones abiertas se olvidan; las carpetas quedan.

#### 5.21.1 Permisos del sistema operativo

| SO | Cómo | Requiere y límites |
|---|---|---|
| Windows | La misma carpeta de trabajo de §5.20.1, con `CreateFile` sin seguir *reparse points* (`FILE_FLAG_OPEN_REPARSE_POINT`) **desde la sesión del alumno**: en modo ventana, el propio agente; en modo servicio, el ayudante (`CallFolderCreate`, `CallFolderList`, `CallFileRead`) con el token del alumno. | Ningún privilegio. Sin sesión iniciada o sin ayudante no se declara `file.collect`. |
| Linux (pendiente) | `$XDG_DOCUMENTS_DIR/Aula`, con `O_NOFOLLOW`. | No se declara. |
| macOS (pendiente) | `~/Documents/Aula`. | TCC puede pedir acceso a Documentos. No se declara. |

### 5.22 Abrir una página o una app (`open.*`)

> Aprobado el 2026-11-09 (prompt 10.11), para v0.12. Implementación en `internal/openapp`;
> reglas en `docs/privacidad.md` 66–70. En la consola, el panel «Abrir página o app» de la barra
> de acciones, la acción en la ficha y la Pantalla grande, y Ajustes › Apps permitidas.

El profesor abre una página web en el navegador predeterminado de los alumnos («todos abran
python.org/docs») o una app de la lista de **apps permitidas** de la consola («abran Thonny»).
Es una orden puntual: el agente abre y acusa; no sigue lo que el alumno hace después, no cierra
nada y no se reenvía al reconectar.

**Nunca un comando.** La consola no envía líneas de comandos ni argumentos. `open.url` solo
lleva una URL `http`/`https`; `open.app` lleva la ruta de un ejecutable, y el agente la acepta
solo si cumple **sus reglas fijas** (abajo), compiladas en el agente y que ninguna consola puede
cambiar. La lista de apps vive solo en la consola (Ajustes › Apps permitidas), así que no hay
nada que instalar en los equipos de los alumnos.

```
Consola (P)                                        Agente (A)
   |-- open.url (open_id, url) -------------------->|  valida la URL y su ritmo
   |                                                |  abre el navegador (en servicio, el ayudante)
   |<------------------------- open.ack (opened) ---|  aviso en la esquina: qué y quién
   |-- open.app (open_id, app_id, name, path) ------>|  valida la ruta con sus reglas
   |<------------------- open.ack (failed, reason) --|  no ejecuta nada
```

**`open.url` (P → A)**

```json
{ "open_id": "7d3a…", "url": "https://docs.python.org/3/tutorial/" }
```

| Campo | Tipo | Regla |
|---|---|---|
| `open_id` | string | UUID v4 de la consola; el mismo para todos los destinatarios de una orden. |
| `url` | string | 1–2048 caracteres. Esquema `http` o `https` (sin distinguir mayúsculas), con host, **sin** usuario ni contraseña (`usuario@`), sin espacios ni caracteres de control. Un host con acentos va en punycode o se convierte. |

**`open.app` (P → A)**

```json
{ "open_id": "1b9e…", "app_id": "thonny", "name": "Thonny",
  "path": "%ProgramFiles(x86)%\\Thonny\\thonny.exe" }
```

| Campo | Tipo | Regla |
|---|---|---|
| `open_id` | string | UUID v4 de la consola. |
| `app_id` | string | El id de la app en el catálogo de la consola, `^[a-z0-9-]{1,32}$`. Solo informa (acuses y registros). |
| `name` | string | 1–60 caracteres, texto plano: lo que ve el alumno en el aviso. |
| `path` | string | 1–260 caracteres. Ruta absoluta a un `.exe`; puede empezar con `%ProgramFiles%`, `%ProgramFiles(x86)%`, `%LOCALAPPDATA%` o `%SystemRoot%`, que el agente expande **en la sesión del alumno**. Ninguna otra variable. |

**Reglas fijas del agente para `open.app`** (la consola las aplica también, para avisar antes):

1. Tras expandir y normalizar (sin `..`, sin `\\?\`, sin rutas de red `\\servidor`), la ruta
   está **dentro** de `%ProgramFiles%`, `%ProgramFiles(x86)%`, `%SystemRoot%` o
   `%LOCALAPPDATA%\Programs` (ahí se instalan VS Code y Thonny por usuario).
2. Nunca dentro de la carpeta de trabajo (`Documentos\Aula`), Documentos, Escritorio, Descargas
   ni la carpeta temporal, aunque estuvieran bajo una raíz permitida: así ni una consola
   comprometida puede enviar un ejecutable con `file.push` y lanzarlo.
3. Termina en `.exe` y el archivo existe (el agente resuelve enlaces y comprueba que la ruta
   final siga cumpliendo 1 y 2).
4. No es un intérprete ni una herramienta del sistema: `cmd`, `powershell`, `pwsh`, `wscript`,
   `cscript`, `mshta`, `rundll32`, `regsvr32`, `msiexec`, `reg`, `schtasks`, `wmic`, `bitsadmin`,
   `certutil`, `bash`, `wsl`, `conhost`, `explorer` (por nombre de archivo, sin distinguir
   mayúsculas).
5. Se lanza **sin argumentos** y con el directorio de trabajo de su propia carpeta, como el
   alumno (en modo servicio, el ayudante con el token del alumno; nunca como SYSTEM).

**`open.ack` (A → P)**

```json
{ "ref_id": "<id del open.url o del open.app>", "open_id": "7d3a…", "status": "opened" }
```

| Campo | Tipo | Regla |
|---|---|---|
| `ref_id` | string | El `id` de la orden. |
| `open_id` | string | El de la orden. |
| `status` | string | `opened`: el SO aceptó abrirla. `failed`: no se abrió. |
| `reason` | string | Solo con `failed`: `not_allowed` (la ruta no cumple las reglas fijas), `not_installed` (el `.exe` no existe en ese equipo), `launch_failed` (el SO no pudo abrirla, o no hay navegador predeterminado) o `busy` (se pasó del ritmo). |

Reglas:

- **Ritmo:** cada agente abre como máximo 3 seguidas y luego 1 cada 3 s. Lo que se pasa se
  acusa `failed` `busy` y no se abre.
- Una URL inválida → `error` `invalid_message` (la consola ya no la envía). Una ruta que no
  cumple las reglas no es un error del protocolo: es `open.ack` `failed` `not_allowed`, para
  que la consola diga «en este equipo no se permite».
- **Aviso al alumno, siempre** (`docs/privacidad.md` regla 66): tras `opened`, la ventana
  muestra en la esquina qué se abrió y qué profesor lo pidió. Con `failed`, nada se abrió y el
  alumno no ve nada.
- El agente **no** aplica el filtro web a la orden: si el dominio está bloqueado (§5.14), el
  navegador no lo encontrará. La consola lo advierte antes de enviar.
- **Destinatarios:** la selección de Alumnos (o todos) o un alumno desde su ficha. La consola
  envía la orden solo a sesiones reales que declararon la capacidad. Los alumnos simulados
  **nunca** la declaran (cambia el equipo).
- Los registros llevan `open_id`, el tipo, `status` y `reason`. **Nunca** `url`, `app_id`,
  `name` ni `path`.

#### 5.22.1 Permisos del sistema operativo

| Orden | Windows | Requiere |
|---|---|---|
| `open.url` | `ShellExecuteW` con el verbo `open` sobre la URL ya validada, que abre el navegador predeterminado del alumno. En modo ventana, el propio agente; en modo servicio, el ayudante (`CallOpenURL`) en la sesión del alumno. | Ningún privilegio. Sin sesión iniciada o sin ayudante no se declara `open.url`. |
| `open.app` | `CreateProcessW` con la ruta validada como `lpApplicationName`, la línea de comandos vacía (solo el nombre del programa) y `lpCurrentDirectory` en su carpeta. En modo servicio, el ayudante (`CallOpenApp`), que vuelve a comprobar las reglas antes de lanzar. | Ningún privilegio. Igual que `open.url`. |
| Linux y macOS (pendiente) | `xdg-open` / `open`. | No se declaran. |

### 5.23 Encuestas en vivo (`poll.*`)

> Aprobado el 2026-11-11 (prompt 10.12), para v0.13. Implementación en `internal/poll`; reglas en
> `docs/privacidad.md` 71–74. En la consola, el panel «Encuesta» de la barra de acciones, la
> sección «Encuestas» de la vista «Participación» y su franja mientras está abierta.

El profesor hace una pregunta rápida para comprobar que se entendió («¿Qué imprime
`print(3 // 2)`?»): de **opción múltiple**, de **sí o no** o **abierta corta**. Llega a todo el
canal o a una selección como una tarjeta en la ventana del alumno, que responde cuando quiere;
el profesor ve las respuestas llegar como barras, la cierra y, si quiere, **comparte el resultado
agregado**. Lo inicia el profesor y lo responde el alumno; nada de esto toca el equipo.

```
Consola (P)                                               Agente (A)
   |-- poll.open (poll_id, kind, question, options, named) -->|  tarjeta «Encuesta · Nuevo»
   |<------------------------------------ poll.ack (shown) ---|
   |<----------------------- poll.vote (poll_id, choice|text) -|  «Enviar»
   |-- poll.vote.ack (accepted) ----------------------------->|  «Respuesta enviada»
   |<----------------------- poll.vote (otra opción) ---------|  cambia su respuesta
   |-- poll.vote.ack (accepted) ----------------------------->|
   |-- poll.close (poll_id) --------------------------------->|  «Encuesta cerrada»
   |<----------------------------------- poll.ack (closed) ---|
   |-- poll.results (poll_id, counts, voters, recipients) --->|  barras con el total
   |<---------------------------------- poll.ack (results) ---|
```

**`poll.open` (P → A)**

```json
{ "poll_id": "c2d7…", "kind": "choice", "question": "¿Qué imprime print(3 // 2)?",
  "options": ["1.5", "1", "2", "Error"], "named": false }
```

| Campo | Tipo | Regla |
|---|---|---|
| `poll_id` | string | UUID v4 de la consola; identifica la encuesta en el voto, el cierre, los resultados y los acuses. |
| `kind` | string | `choice` (opción múltiple), `yesno` (sí o no) u `open` (respuesta abierta corta). |
| `question` | string | 1–200 caracteres, texto plano. |
| `options` | arreglo | Solo con `choice`: 2–6 opciones de 1–80 caracteres, texto plano. Con `yesno` y `open` va vacío o ausente: la ventana del alumno pone «Sí» y «No» con sus propios textos. |
| `named` | bool | `false`: anónima, el profesor ve solo el total. `true`: el profesor ve qué respondió cada alumno. La tarjeta lo dice **antes** de que el alumno responda (`docs/privacidad.md` regla 72). |

**`poll.vote` (A → P)**

```json
{ "poll_id": "c2d7…", "choice": 1 }
{ "poll_id": "c2d7…", "text": "Que el for no termina" }
```

| Campo | Tipo | Regla |
|---|---|---|
| `poll_id` | string | La encuesta que responde. |
| `choice` | entero | Con `choice` y `yesno`: el índice de la opción, desde 0 (`yesno`: 0 = sí, 1 = no). |
| `text` | string | Con `open`: 1–140 caracteres, texto plano. |

Lleva exactamente uno de los dos: `choice` **o** `text`. Los dos, ninguno, un índice fuera de
las opciones, o el campo que no corresponde al tipo de la encuesta → `error` `invalid_message`.

**`poll.vote.ack` (P → A)**

```json
{ "ref_id": "<id del poll.vote>", "poll_id": "c2d7…", "status": "accepted" }
```

| `status` | Cuándo |
|---|---|
| `accepted` | La respuesta cuenta (reemplaza a la anterior de ese alumno, si la había). |
| `closed` | La encuesta ya se cerró: la respuesta **no** cuenta. |
| `unknown` | La consola no conoce esa encuesta o no era para ese alumno. No cuenta. |

**`poll.close` (P → A)**

```json
{ "poll_id": "c2d7…" }
```

La tarjeta queda como «Encuesta cerrada», con lo que el alumno respondió, y deja de aceptar
respuestas. Si el agente no conoce esa encuesta, la ignora.

**`poll.results` (P → A)**

```json
{ "poll_id": "c2d7…", "counts": [3, 14, 2, 1], "voters": 20, "recipients": 22 }
```

| Campo | Tipo | Regla |
|---|---|---|
| `poll_id` | string | Una encuesta **cerrada** que el agente conoce; otra se ignora. |
| `counts` | arreglo | Cuántos eligieron cada opción, en el orden de `options` (`yesno`: sí, no). 2–6 enteros ≥ 0. |
| `voters` | entero | Cuántos respondieron: la suma de `counts`. 0–10 000. |
| `recipients` | entero | A cuántos les llegó. `voters`–10 000. |

Solo totales: nunca quién eligió qué. Solo para `choice` y `yesno`; las respuestas abiertas no
se comparten.

**`poll.ack` (A → P)**

```json
{ "ref_id": "<id del poll.open>", "poll_id": "c2d7…", "status": "shown" }
```

| `status` | Responde a |
|---|---|
| `shown` | `poll.open`: la tarjeta está en la ventana (también al reenviarla). |
| `closed` | `poll.close`. |
| `results` | `poll.results`: el resultado se ve en la tarjeta. |

Reglas de la consola:

- **Una encuesta abierta por clase.** Abrir otra cierra la vigente primero (`poll.close` a sus
  destinatarios); el panel lo avisa antes. Las cerradas se conservan en memoria (como máximo
  **20** por clase) para verlas y compartir su resultado.
- **Un voto por alumno**, identificado como en el resto de la consola (equipo y usuario), y
  **cuenta el último**: el alumno puede cambiar su respuesta hasta el cierre. Un voto después del
  cierre se acusa `closed` y no cambia nada.
- **Destinatarios:** «todos» (el canal, también quien se conecta con la encuesta abierta) o una
  selección. Solo a las sesiones que declararon `poll`. Un voto de quien no es destinatario se
  acusa `unknown`.
- **Reconexión** (§3.5): al llegar `hello`, la consola le manda a ese alumno el estado de la
  última encuesta que le correspondía: `poll.open` con el mismo `poll_id` si sigue abierta, o
  `poll.close` (y `poll.results`, si se compartió) si se cerró mientras no estaba.
- **Ritmo por agente:** ráfaga de 3 `poll.vote` y uno por segundo después. Lo que excede se
  descarta y se responde `error` `invalid_message` («demasiado seguido»), sin acuse.
- Los votos de los **alumnos simulados** se cuentan aparte y no entran en `poll.results`
  (`docs/privacidad.md` regla 32).
- Todo vive en memoria y se descarta al cerrar la clase o cambiar de canal.

Reglas del agente:

- Un `poll.open` con un `poll_id` que ya tiene conserva la tarjeta y lo que el alumno eligió; si
  había un voto sin acuse, lo reenvía con el mismo contenido. Uno nuevo agrega su tarjeta arriba
  de la lista con «Nuevo» (`notify`, nunca `raise`); en el modo compacto cuenta como «sin ver».
- Al reconectar, espera **3 s** a que la consola reenvíe cada encuesta que tenía abierta; la que
  no llega queda cerrada (se cerró mientras no estaba, o es otra clase), y una respuesta que
  esperaba su acuse ya no cuenta.
- Guarda como máximo las **5** tarjetas de encuesta más recientes.
- Ritmo: «Enviar» acepta una respuesta por segundo.
- Si la consola responde `unknown_type` o `unsupported_capability` al voto (una consola
  anterior), la tarjeta dice que esa consola no recibe respuestas.

### 5.24 Proyectar la pantalla de un alumno (`student.present.*`)

> Aprobado el 2026-11-11 (prompt 12.1), para v0.14 (Windows). Implementación en
> `internal/broadcast` (`Projector` en la consola, `Presenter` y `Publisher` en el agente); reglas
> en `docs/privacidad.md` 75–82. En la consola, «Proyectar a la clase» en la ficha del alumno y en
> la Pantalla grande, y su franja mientras dura. Cambio del 2026-11-11 (decisión del usuario): **sin
> consentimiento**; lo decide y lo termina el profesor, como el control remoto (§5.16).

El profesor elige a un alumno y su pantalla se ve en vivo en los equipos de los demás alumnos del
canal y en la consola, para comentar su trabajo con el grupo. **No se le pregunta al alumno ni
puede detenerla**: el profesor decide cuándo empieza y cuándo termina; el alumno lo sabe por su
banda y su distintivo, que aparecen antes del primer cuadro.
Reutiliza el transporte de §5.12 con la fuente invertida: el video lo **publica** el agente del
alumno-fuente y lo **reenvía la consola**, que es el SFU de la clase (D-014).

```
Fuente (A, pion)                  Consola (P, pion, ICE-lite, un puerto UDP)         Receptores (A, WebView2)
   |<-- student.present.start --------|   ventana al frente, banda y distintivo, y después la captura
   |-- student.present.ack (accepted)>|
   |<-- student.present.offer --------|   (recvonly)
   |-- student.present.answer ------->|   (sendonly)
   |<-> student.present.ice ---------->|
   |== SRTP: video H.264 ============>|-- screen.broadcast.start (source) ---------------->|
   |                                  |-- screen.broadcast.offer / answer / ice ---------->|
   |                                  |== SRTP: los mismos paquetes RTP, reenviados ======>|
   |<-- student.present.stop ---------|-- screen.broadcast.stop -------------------------->|
   |-- student.present.ack (ended) -->|
```

**Por qué la consola reenvía y no hay conexiones entre alumnos.** Los alumnos se conectan al
profesor, nunca al revés ni entre sí (CLAUDE.md): la fuente solo hace ICE de salida hacia el puerto
UDP de la consola (`ws_port + 1`, el de §5.12, que ya tiene su regla de firewall), y no abre
puertos de entrada. La fuente codifica **una vez** y sube un solo stream (≈ 2,5 Mbit/s) haya los
receptores que haya; la consola copia los paquetes RTP a cada receptor sin decodificar ni volver
a codificar, como en la transmisión del profesor.

**Por qué mensajes propios para la fuente y `screen.broadcast.*` para los receptores.** El tramo de
la fuente tiene otra dirección de medios (el agente **envía**), otro ciclo (su banda, su distintivo
y la exclusión de la ventana de Aula) y otras validaciones. Compartir los nombres haría que un agente aceptara
enviar medios en mensajes que hoy le garantizan solo recibir (regla 13). Para los receptores, en
cambio, nada cambia salvo de quién es la pantalla: siguen con §5.12 y el campo nuevo `source` de
`screen.broadcast.start`. Por eso «detener» son dos mensajes: `student.present.stop` a la fuente y
`screen.broadcast.stop` a los receptores.

Reglas generales:

- **Uno a la vez.** Una consola proyecta a un solo alumno-fuente a la vez. Un `start` con otro
  `present_id` reemplaza al anterior: el agente cierra el viejo (`ended`) antes de empezar el
  nuevo. La proyección y la transmisión de la pantalla del profesor (§5.12) comparten el puerto
  y la vista de los receptores: empezar una termina la otra (la consola lo confirma con el
  profesor).
- **Sin consentimiento, con aviso antes de capturar.** Al llegar `start`, el agente trae su ventana
  al frente (sale del modo compacto) con la banda «Tu pantalla se está mostrando a la clase» y
  muestra el distintivo nativo; **después** prepara la captura y el codificador y responde `ack`
  `accepted` con el tamaño del video. Si no puede mostrar el aviso, capturar o codificar, responde
  `failed` y no publica. El alumno no tiene cómo negarse ni cortarla (reglas 75 y 77). Si la consola
  no recibe el `ack` en **15 s**, termina la proyección (`failed`, motivo `no_answer`).
- **La consola ofrece y la fuente responde.** La oferta es `recvonly` con **una sola** `m=video`
  H.264 (`profile-level-id=42e028`, `packetization-mode=1`). El agente rechaza (`error`
  `invalid_message`) una oferta con audio, canal de datos, más de un video o que le pida recibir;
  responde `sendonly`. Sin STUN ni TURN: solo candidatos `host`.
- **Cuadros clave.** Un PLI o FIR de cualquier receptor (o de la consola) se reenvía por RTCP a la
  fuente, como máximo uno por segundo; la fuente genera además uno cada 5 s.
- **Medios** como §5.12: hasta `max_width` px de ancho, hasta `fps` cuadros/s, tope de
  `max_kbps`; una pantalla que no cambia se codifica una vez por segundo. Monitor principal; sin
  audio. La ventana de Aula del alumno **se excluye de la captura** (regla 78): si Windows no lo
  permite, el agente responde `failed` `capture_unavailable` y no publica.
- **Reintento de la fuente:** si su conexión WebRTC pasa a `failed`, o queda `disconnected` más de
  5 s, la consola envía una oferta nueva con `attempt + 1`, con el retroceso de §5.12. Los
  receptores conservan su conexión y ven «Conectando con la pantalla de LAB3-PC07…».
- **Reintento de un receptor:** como §5.12.
- **Fin.** La proyección termina con «Detener» en la consola, con `failed` o `ended` de la fuente,
  al cerrarse la sesión WebSocket de la fuente (no se reanuda sola), al cambiar de canal y al
  cerrar la clase. La consola envía `student.present.stop` a la
  fuente y `screen.broadcast.stop` a los receptores. Si el agente pierde la conexión con la
  consola, deja de publicar y quita la banda por su cuenta.
- **Receptores:** los agentes conectados que declararon `screen.present.view`, menos la fuente.
  Un agente que solo declara `screen.broadcast` (una versión anterior) no recibe la proyección: su
  vista diría «la pantalla del profesor», y no lo es.
- **Nunca a simulados:** no declaran `screen.present` ni `screen.present.view`.

#### 5.24.1 `student.present.start` (P → A)

```json
{ "present_id": "c0a1…", "max_width": 1920, "fps": 15, "max_kbps": 2500 }
```

| Campo | Tipo | Regla |
|---|---|---|
| `present_id` | string | UUID v4 que asigna la consola a la proyección. |
| `max_width` | entero | Ancho máximo del video, 640–1920. |
| `fps` | entero | Cuadros por segundo, 1–15. |
| `max_kbps` | entero | Tope del codificador, 800–2500. |

Respuesta: `student.present.ack` `accepted` (la banda y el distintivo ya están en pantalla y la
captura lista) o `failed`.

#### 5.24.2 `student.present.ack` (A → P)

```json
{ "ref_id": "<id del start>", "present_id": "c0a1…", "status": "accepted", "width": 1920, "height": 1080 }
```

| Campo | Tipo | Regla |
|---|---|---|
| `ref_id` | string | El `id` del `start` (o del `stop`) al que responde. |
| `present_id` | string | La proyección. |
| `status` | string | `accepted`, `ended` o `failed`. |
| `width`, `height` | entero | Solo con `accepted`: el tamaño del video, 2–4096, pares. |
| `reason` | string | Solo con `failed`: `capture_unavailable`, `encoder_unavailable` o `busy`. |

`accepted` y `failed` llevan el `ref_id` del `start`; `ended`, el del `stop` (o del `start` que la
reemplazó).

#### 5.24.3 `student.present.offer` (P → A)

```json
{ "present_id": "c0a1…", "attempt": 1, "sdp": "v=0\r\n…" }
```

Como §5.12.2, con `present_id` y la oferta `recvonly`. Solo después de `accepted`; antes → `error`
`invalid_message`. Un `attempt` mayor reemplaza la conexión anterior.

#### 5.24.4 `student.present.answer` (A → P)

```json
{ "ref_id": "<id de la offer>", "present_id": "c0a1…", "attempt": 1, "sdp": "v=0\r\n…" }
```

Respuesta SDP `sendonly`, ≤ 64 KiB. Si el agente no puede crear la conexión, responde `error`
`internal` con el `ref_id` de la oferta y la consola reintenta.

#### 5.24.5 `student.present.ice` (A ↔ P)

Como §5.12.4, con `present_id` en lugar de `broadcast_id`. La consola lo envía solo a agentes con
`screen.present`. Como máximo 50 candidatos por intento y sentido.

#### 5.24.6 `student.present.stop` (P → A)

```json
{ "present_id": "c0a1…" }
```

El agente cierra la conexión, deja de capturar, quita la tarjeta (si aún preguntaba), la banda y
el distintivo, y responde `ack` `ended`. Idempotente; un `present_id` distinto del vigente se
ignora.

#### 5.24.7 Permisos del sistema operativo

- **Windows:** la captura de pantalla no pide permiso. Como servicio, el agente no captura: la
  captura, el codificador y la conexión WebRTC corren en su ayudante, en la sesión del alumno y
  con sus derechos (`internal/agentsvc`, regla 17); el servicio solo pasa la señalización.
- **Linux (Wayland):** la captura pasa por el portal `xdg-desktop-portal` (ScreenCast), que pide
  al alumno aprobar **en cada sesión**; en X11 no hay permiso. Linux no declara `screen.present`
  todavía.
- **macOS:** pide el permiso «Grabación de pantalla» (Privacidad y seguridad). macOS no declara
  `screen.present` todavía.

### 5.25 Terminar la clase (`class.end`)

> Aprobado el 2026-11-12 (prompt 10.14), sin publicar. Todos los sistemas: limpiar la ventana no
> toca el SO. Implementación en `internal/classend` (`Class` en la consola, `Clear` en el agente);
> reglas en `docs/privacidad.md` 83–86. En la consola, «Terminar la clase» bajo el Selector de
> clase del Menú, con su diálogo.

Al acabar la hora, el profesor deja el laboratorio limpio con un solo botón: la consola detiene
lo que sigue activo con los mensajes de siempre y después pide a cada agente que **deje su
ventana como recién abierta**. El siguiente grupo no hereda nada.

```
Consola (P)                                                       Agente (A)
   |  1. (opcional) Recoger el trabajo: file.collect.request ---->|  (§5.21; espera sus acuses ≤ 60 s)
   |  2. screen.broadcast.stop, student.present.stop, remote.stop>|
   |     screen.unfreeze, app.block.clear, web.filter.clear ----->|
   |  3. timer.stop, poll.close --------------------------------->|
   |  4. (solo si el profesor lo marca) power.cancel ------------>|
   |  5. class.end ---------------------------------------------->|  limpia su ventana, en orden
   |<------------------------------------------ class.end.ack ----|  clean | failed
```

Si un paso falla, los demás siguen y el resultado lo dice. Terminar dos veces seguidas no rompe
nada: lo que ya estaba detenido se ignora y el agente vuelve a acusar `clean`.

#### 5.25.1 `class.end` (P → A)

```json
{ "end_id": "5b0c…" }
```

| Campo | Tipo | Regla |
|---|---|---|
| `end_id` | string | UUID v4 de la consola; identifica este fin de clase en el acuse. |

El agente, en este orden (primero lo que actúa sobre el equipo, después la ventana):

1. Suelta el control remoto (`remote.ack` `ended`), termina la proyección de su pantalla y cierra
   la vista de la transmisión, como al cerrarse la conexión.
2. Quita la pausa (§5.9).
3. Quita las restricciones con su diario (§5.13–5.14; D-017), como al desconectarse.
4. Quita el temporizador y las tarjetas de encuestas.
5. Vacía la lista de código, los avisos, las tarjetas de archivos recibidos, de recolecciones y de
   lo que se abrió; baja la mano y descarta los mensajes al profesor que aún no salieron (la
   consola ya los olvidó).
6. Muestra la tarjeta **«La clase terminó»** con la hora y lo que se quitó (regla 83), sin quitar el
   foco.

**Nunca** toca la energía pendiente (una cuenta regresiva de §5.15 sigue; solo `power.cancel` la
detiene), ni borra, mueve o lee archivos del alumno (`Documentos\Aula` queda igual), ni cierra
la conexión: el agente sigue conectado al canal, listo para la siguiente clase.

#### 5.25.2 `class.end.ack` (A → P)

```json
{ "ref_id": "<id del class.end>", "end_id": "5b0c…", "status": "clean" }
```

| Campo | Tipo | Regla |
|---|---|---|
| `ref_id` | string | `id` del `class.end`. |
| `end_id` | string | El del pedido. |
| `status` | string | `clean`: la ventana quedó limpia. `failed`: algo no se pudo quitar; el resto sí se hizo. |
| `reason` | string | Solo con `failed`: `restrictions` (no se pudieron quitar las restricciones), `pause` (no se pudo quitar la pausa) o `internal`. Otro valor → `invalid_message`. |

#### 5.25.3 Reglas de la consola

- Lo envía a cada sesión conectada que declaró `class.end`, real o simulada. A un agente que **no
  la declara** (versión anterior) le envía, además de las paradas de arriba, `code.clear` si
  declara `code.view`; el resultado lo marca como «limpio en parte».
- Quien no acusa en **10 s** queda «sin acuse». «Reintentar a los que faltan» vuelve a enviar
  `class.end` a los que siguen conectados sin acuse o con `failed`.
- **Al reconectar.** La consola recuerda, por equipo (`hostname`), a quien no acusó: un equipo
  desconectado al terminar, o que no respondió. Cuando ese equipo vuelve a conectarse, recibe
  `class.end` **antes** que lo que la consola reenvía al reconectar (temporizador, encuesta). No
  pierde nada nuevo: mientras estuvo fuera no recibió nada. Lo recordado se olvida al acusar, al
  terminar otra vez la clase o al cerrar la consola o cambiar de canal. Quien se conecta por
  primera vez después de terminar no tiene nada que heredar: la consola ya no tiene nada activo.
- La consola queda «sin clase»: sin temporizador, encuestas, manos, mensajes, envíos de archivos,
  recolecciones en curso, aperturas, historial de código ni «Últimos envíos». Sigue anunciada en
  el mismo canal. Las carpetas guardadas (capturas, recolecciones, plantillas) no se tocan.

---

## 6. Reservados para fases futuras

Nombres reservados; un agente v0 responde `unknown_type` o `unsupported_capability`.

| Tipo | Descripción |
|---|---|
| `code.collect` | Recolectar el código que el alumno entrega desde el overlay. |

---

## 7. Límites

| Límite | Valor v0 | Al excederlo |
|---|---|---|
| Tamaño de un mensaje WebSocket (envoltura completa) | **1 MiB** (1 048 576 bytes) | `error` `message_too_large` y cierre `1009`. El emisor debe comprobarlo antes de enviar. |
| `code.share.content` | 256 KiB en UTF-8 | `error` `message_too_large`; la sesión sigue. |
| `code.share.title` | 200 caracteres | `error` `invalid_message` |
| `code.share.exercise` | 16 caracteres | `error` `invalid_message` |
| `msg.show.text` | 2000 caracteres | `error` `invalid_message` |
| `msg.student.text` | 1–500 caracteres | `error` `invalid_message` |
| `hand.raise.note` | 140 caracteres | `error` `invalid_message` |
| `msg.student` / `hand.*` por agente | ráfaga 5 y 1 cada 3 s / ráfaga 3 y 1 por s | `error` `invalid_message`; se descarta |
| Mensajes guardados por alumno en la consola | 20 | Se descarta el respondido más viejo |
| `timer.start.label` | 40 caracteres | `error` `invalid_message` |
| `timer.start.description` | 1000 caracteres | `error` `invalid_message` |
| `timer.start.duration_s` / `timer.*.remaining_ms` | 10–14 400 s / 1–14 400 000 ms (4 h; `0` solo en `timer.ack`) | `error` `invalid_message` |
| `timer.extend.add_s` | 60–3600 s | `error` `invalid_message` |
| `screen.thumb.start.interval_ms` | 500–10 000 ms (≤ 2 cuadros/s) | `error` `invalid_message` |
| `screen.thumb.frame.jpeg` | 640 KiB antes de Base64 | `error` `message_too_large`; la sesión sigue. |
| `screen.snapshot.request.quality` / `max_width` | 50–95 / 0 o 640–7680 | `error` `invalid_message` |
| `screen.snapshot.response.data` | 512 KiB por parte antes de Base64; ≤ 32 partes; 16 MiB en total | `error` `message_too_large` (parte) o `invalid_message`; la sesión sigue. |
| `file.push.data` | 512 KiB por parte antes de Base64; ≤ 32 partes; 16 MiB por archivo | `error` `message_too_large` (parte) o `invalid_message`; la sesión sigue. |
| `file.push.name` / `dest` | 1–100 caracteres / 0–200 caracteres y 4 niveles | `error` `invalid_message` (longitud); `file.push.ack` `invalid_path` (reglas de ruta) |
| `file.push.label` / `files` | 80 caracteres / 1–20 archivos por envío (32 MiB) | `error` `invalid_message` |
| `file.collect.*.activity` / `patterns` | 1–80 caracteres / 0–10 patrones de 1–20 caracteres | `error` `invalid_message` (longitud); `file.collect.ack` `failed` `invalid_path` (reglas de ruta) |
| `file.collect.request` límites | `max_file_size` 1 B – 16 MiB, `max_total` 1 B – 32 MiB, `max_files` 1–50, `round` 1–1000 | `error` `invalid_message` |
| `file.collect.response.data` | 512 KiB por parte antes de Base64; ≤ 32 partes; 16 MiB por archivo | `error` `message_too_large` (parte) o `invalid_message`; la sesión sigue. |
| `file.collect.ack.skipped` | 50 entradas | `error` `invalid_message` |
| `open.url.url` | 1–2048 caracteres, `http`/`https` | `error` `invalid_message` |
| `open.app.app_id` / `name` / `path` | 1–32 / 1–60 / 1–260 caracteres | `error` `invalid_message` (longitud); `open.ack` `failed` `not_allowed` (reglas de ruta) |
| `open.*` por agente | ráfaga 3 y 1 cada 3 s | `open.ack` `failed` `busy`; no se abre |
| `poll.open.question` / `options` | 1–200 caracteres / 2–6 opciones de 1–80 caracteres (solo `choice`) | `error` `invalid_message` |
| `poll.vote.text` | 1–140 caracteres | `error` `invalid_message` |
| `poll.vote` por agente | ráfaga 3 y 1 por s | `error` `invalid_message`; se descarta |
| `poll.results.voters` / `recipients` | 0–10 000 | `error` `invalid_message` |
| Encuestas guardadas en la consola | 20 por clase | Se descarta la cerrada más vieja |
| `screen.freeze.message` | 0–300 caracteres | `error` `invalid_message` |
| `screen.broadcast.offer.sdp` / `answer.sdp` | 64 KiB | `error` `message_too_large`; la sesión sigue. |
| `screen.broadcast.ice.candidate` | 1024 caracteres; 50 por intento y sentido | `error` `invalid_message`; los que sobran se descartan. |
| `screen.broadcast.*.attempt` | 1–1000 | `error` `invalid_message` |
| `student.present.start` | `max_width` 640–1920, `fps` 1–15, `max_kbps` 800–2500 | `error` `invalid_message` |
| `student.present.*.sdp` / `ice.candidate` / `attempt` | Como `screen.broadcast.*` | Como `screen.broadcast.*` |
| `screen.broadcast.start.source.hostname` | 1–63 caracteres | `error` `invalid_message` |
| Acuse de `student.present.start` | 15 s | La consola termina la proyección (`failed`, `no_answer`) |
| Acuse de `class.end` | 10 s | La consola lo marca «sin acuse» y se lo reenvía al reconectar |
| `app.block.set.apps` | 1–100 nombres de ≤ 64 caracteres | `error` `invalid_message` |
| `web.filter.set.domains` | 1–500 dominios de ≤ 253 caracteres | `error` `invalid_message` |
| `power.action.delay_s` | 10–600 s (0 o ausente = 30 s) | `error` `invalid_message` |
| `remote.start.interval_ms` | 100–1000 ms (≤ 10 imágenes/s) | `error` `invalid_message` |
| `remote.input.events` | 1–32 por mensaje; ≤ 60 mensajes/s | `error` `invalid_message`; los que exceden el ritmo se descartan sin error |
| `error.message` | 500 caracteres | El emisor lo trunca. |
| `hello.capabilities` | 64 entradas | `error` `invalid_message` y cierre |
| Versión `v` ≠ 1 | — | `error` `unsupported_version` y cierre `1008` |
| Handshake completo | 10 s | Cierre `1008` |
| Silencio de la sesión | 15 s | Cierre de la conexión y reconexión (§3.5) |

Por qué 256 KiB de código dentro de 1 MiB: al serializar en JSON, comillas, barras y saltos de
línea se escapan y ocupan más bytes. 256 KiB deja margen holgado para código real; aun así, el
emisor verifica el tamaño final de la envoltura.

---

## 8. Versionado del protocolo

- `v` / `proto` es la **versión mayor** del protocolo. Agregar mensajes, campos opcionales o
  capacidades **no** la cambia (se ignoran los desconocidos).
- Quitar o renombrar mensajes o campos, o cambiar su significado, la sube a `2`. Eso es un
  cambio de ruptura y sube la versión mayor `x` de la app (ver CLAUDE.md).
- En v0 no hay negociación: consola y agente deben hablar `v: 1`.

## 9. Privacidad y registros

- Los logs pueden registrar `type`, `id`, `ref_id`, `code`, tamaños y hostname. **No** deben
  registrar `content`, `text`, `title`, nonces, firmas ni claves.
- Los logs **no** registran el contenido de `screen.thumb.frame` (`jpeg`) ni `screen.freeze.message`;
  sí `seq`, tamaños y dimensiones. Ningún extremo escribe los cuadros en disco.
- De `screen.broadcast.*` se registran `broadcast_id`, `attempt`, tamaños y estados de la
  conexión WebRTC; **no** el SDP ni los candidatos (llevan direcciones de la LAN). El video de la
  transmisión no se graba en ningún extremo.
- De `student.present.*` se registran `present_id`, `attempt`, `status`, `reason`, el equipo
  fuente, tamaños y estados de las conexiones; **no** el SDP, los candidatos ni ningún cuadro. La
  consola reenvía los paquetes sin decodificarlos ni guardarlos.
- De `power.*` se registran `action`, `action_id`, `force`, `status`, los aplazamientos y el
  resultado; el agente lo registra **antes** de llamar al SO.
- De `remote.*` se registran `control_id`, `status`, `reason` y la cantidad de eventos y de
  descartes. **Nunca** qué teclas se enviaron, ni las coordenadas, ni la entrada del alumno que
  causó una pausa (el agente solo sabe *que* la hubo).
- De `screen.snapshot.*` se registran `snapshot_id`, `part`/`parts`, tamaños, dimensiones y el
  resultado. **Nunca** `data`, ni la ruta donde la consola guardó la imagen.
- De `file.push*` se registran `push_id`, `file_id`, `part`/`parts`, tamaños, `status` y
  `reason`. **Nunca** `data`, ni `name`, `dest`, `label` o la ruta final.
- De `file.collect.*` se registran `collect_id`, `round`, `file_id`, `part`/`parts`, tamaños,
  cuántos archivos y omitidos, `status` y `reason`. **Nunca** `data`, `activity`, `patterns`,
  `path` ni la carpeta donde la consola guardó (todas nombran al alumno o su trabajo).
- De `open.*` se registran `open_id`, el tipo, `status` y `reason`. **Nunca** `url`, `app_id`,
  `name` ni `path` (dicen qué se le pidió al alumno abrir).
- De `poll.*` se registran `poll_id`, `kind`, `named`, `status`, el índice elegido, la longitud
  de `text` y los totales. **Nunca** `question`, `options` ni `text` (son texto del profesor y del
  alumno), y en una encuesta anónima tampoco qué eligió cada equipo.
- De `class.end*` se registran `end_id`, `status`, `reason`, el equipo y qué se quitó (sin
  contenido: cuántas tarjetas, si había restricciones). **Nunca** el código, los avisos ni los
  nombres de los archivos que se quitaron de la ventana.
- De un `msg.show` `personal` se registran el equipo, el `id` y la longitud; **nunca** el texto.
  En «Últimos envíos» la consola lo etiqueta «Mensaje privado a <equipo>», sin el texto.
- De `hand.*` y `msg.student*` se registran `hand_id`, `message_id`, `by`, la longitud y el
  equipo. **Nunca** `text` ni `note`, ni el `text` de la respuesta (`msg.show`).
- `hello` revela al profesor el usuario y el equipo del alumno. Es intencional (monitoreo
  transparente) y se explica en `docs/privacidad.md`.

## 10. Seguridad y modelo de confianza

### 10.1 Identidades y claves

| Clave | Quién la tiene | Para qué |
|---|---|---|
| **Clave del centro** (Ed25519) | Privada: los equipos de profesor del centro, instalada por el administrador. Pública: la configuración de cada agente del centro. | Firmar `auth.proof`. Distingue las consolas del centro de cualquier otra. |
| **Certificado TLS** (ECDSA P-256, autofirmado) | Cada equipo de profesor, por usuario; se genera al primer arranque y se persiste. | Cifrar la conexión. Su huella SHA-256 viaja en el TXT `cert_fingerprint`. |

- **Una sola clave por centro de cómputo**, no por maestro. El administrador la crea al instalar
  la primera consola (`aula-consola centro crear`) y la importa en las demás
  (`aula-consola centro importar aula-centro.key`). Los alumnos reciben solo la **pública**,
  `aula-centro.pub`, con el formato `aula-ed25519:<32 bytes en Base64>`. Huella corta
  `XXXX-XXXX-XXXX-XXXX` para verificarla de palabra.
- La privada vive en `C:\ProgramData\Aula\centro\clave-centro.pem` (PEM PKCS#8). ACL:
  Administradores y SYSTEM con control total; **Usuarios del equipo del profesor, solo
  lectura**, para que cualquier maestro que inicie sesión ahí pueda dar clase. Nunca se registra
  en logs, nunca viaja por la red y nunca se instala en los equipos de alumno.
- **Riesgo aceptado:** cualquier usuario local de un equipo de profesor puede leer la clave del
  centro. Los equipos de profesor deben usarlos solo profesores. Si la clave se filtra, el
  administrador crea otra y reinstala consolas y alumnos (rotación).
- Compatibilidad y desarrollo: sin clave del centro, la consola usa una identidad por usuario
  (`%AppData%\Aula\consola\identidad-ed25519.pem`, `aula-consola keygen`).
- El certificado TLS y la clave son independientes: regenerar el certificado no obliga a
  reconfigurar agentes; cambiar la clave del centro sí.

### 10.2 Qué comprueba el agente

1. **Canal** (mDNS): solo intenta consolas que anuncian su `channel` (§1.3).
2. **Fijación del certificado** (TLS): el certificado presentado debe tener exactamente la huella
   `cert_fingerprint` anunciada; si no, cierra antes de enviar nada. No se usan autoridades
   certificadoras.
3. **Firma** (`auth.proof`, §3.2): la consola firma `"aula-auth-v1" ‖ nonce ‖ channel ‖ huella`
   con su clave privada; el agente la verifica con la clave pública configurada. La huella que
   entra en la verificación es la del certificado que **el agente vio** en esta conexión.
4. Hasta enviar `auth.ok`, el agente no procesa ningún otro mensaje: cualquier cosa distinta de
   `auth.proof` o `error` → `error` `unauthorized` y cierre `1008`. Un fallo de firma se registra
   (sin la firma ni el nonce) y la instancia queda como no confiable 5 minutos.

**Conexión directa sin mDNS** (`-directo host:puerto`): no hay huella anunciada que fijar, así que
el agente acepta el certificado presentado y usa *su* huella en la verificación de la firma. Un
intermediario que presente su propio certificado no puede producir una firma válida para esa
huella sin la clave privada de la consola, de modo que la suplantación se sigue detectando en el
paso 3.

### 10.3 Qué protege y qué no (v0)

| Amenaza | Protección |
|---|---|
| Consola falsa que anuncia el canal | Falla la firma (paso 3); el agente no revela hostname ni usuario porque `hello` va después de `auth.ok`. |
| Intermediario en la red (MITM) | Fijación del certificado y firma ligada a la huella vista por el agente. |
| Lectura del tráfico (código, mensajes) | TLS 1.3. |
| Repetición de una prueba antigua | Nonce aleatorio de 32 bytes por conexión. |
| Robo de la clave privada del equipo del profesor | **No cubierto**: quien la tenga puede suplantar la consola. Protegerla con la cuenta del SO; si se filtra, generar otra y reconfigurar agentes. |
| Agente no autorizado que se une al canal | **No cubierto en v0**: la consola no autentica agentes (§3.2). El profesor ve hostname y usuario. |
| Saturación (DoS) en la LAN | Solo límites de tamaño y timeouts (§7). |
| Orden de apagado falsa | `power.*` solo se procesa después de `auth.ok` (§3.1), como todo mensaje de la consola; antes → `unauthorized` y cierre. |
| Video de transmisión falso o interceptado | El SDP (con la huella DTLS) viaja por la sesión autenticada (§3.2), así que el agente solo acepta medios de la consola legítima; el video va cifrado con DTLS-SRTP. |
