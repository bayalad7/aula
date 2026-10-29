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
  "capabilities": ["code.view", "msg.show"]
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
| `screen.freeze` | `screen.freeze`, `screen.unfreeze` | Documentada, **pendiente** (§5.9) |
| `screen.broadcast` | `screen.broadcast.start`, `.offer`, `.ice`, `.stop` | v0.4 (Windows, si el WebView2 tiene RTCPeerConnection y decodificador H.264; §5.12). Linux y macOS no la declaran. |
| `app.block` | `app.block.set`, `app.block.clear` | v0.4 (§5.13). Windows, solo con el agente como servicio y su ayudante (D-016). |
| `web.filter` | `web.filter.set`, `web.filter.clear` | v0.4 (§5.14). Windows, solo con el agente como servicio y su ayudante. |
| `file.collect` | `file.collect` | Reservada |
| `code.collect` | `code.collect` | Reservada |
| `power` | `power.*` | Reservada |
| `remote` | `remote.*` | Reservada |
| `poll` | `poll.*` | Reservada |

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
| `msg.show` | P → A | `msg.show` | `text`, `level` | `msg.ack` o `error` |
| `msg.ack` | A → P | — | `ref_id` | — |
| `screen.thumb.start` | P → A | `screen.thumb` | `interval_ms`, `quality`, `max_width` | `screen.thumb.frame` periódicos o `error` |
| `screen.thumb.frame` | A → P | — | `ref_id`, `seq`, `width`, `height`, `jpeg` | — |
| `screen.thumb.stop` | P → A | `screen.thumb` | `{}` | — |
| `screen.broadcast.start` | P → A | `screen.broadcast` | `broadcast_id`, `width`, `height`, `fps` | — (sigue `screen.broadcast.offer`) o `error` |
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
  solo viven en memoria; el agente no las escribe en disco.

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
al cerrar la vista, salvo que el profesor pida explícitamente guardar una captura (etapa futura).

### 5.8 `screen.thumb.stop` (P → A)

Payload `{}`. El agente deja de capturar y quita el indicador. Es idempotente y no tiene
respuesta: la prueba de que se detuvo es que no llegan más cuadros.

### 5.9 `screen.freeze` (P → A) — pendiente

> **Documentado, no implementado** (D-013). Los agentes actuales no declaran `screen.freeze` y
> responden `unknown_type`. §5.9–5.11 describen el diseño previsto.

Pone la **pantalla en blanco**: el agente cubre todos los monitores con una ventana siempre
encima, que tapa lo que hay en pantalla, recibe los clics y explica por qué.

```json
{ "message": "Atención al pizarrón, por favor." }
```

| Campo | Tipo | Regla |
|---|---|---|
| `message` | string | **Opcional**, 0–300 caracteres, texto plano. Se muestra debajo del texto fijo. |

El texto fijo es "Tu profesor pausó esta computadora" y el nombre de la clase y el canal; así el
alumno siempre sabe quién la bloqueó. Un `screen.freeze` sobre una pantalla ya en blanco solo
cambia el mensaje.

- **Al cerrarse la conexión, el agente quita la pantalla en blanco** por sí mismo: si la consola
  se cierra o se cae la red, el alumno no queda bloqueado.
- **No intercepta el teclado** (D-013): el alumno podría cambiar de ventana con Alt+Tab y
  escribir a ciegas, aunque la pantalla en blanco vuelve a quedar encima cada medio segundo.
  Cerrar Aula Cliente (por ejemplo, desde el Administrador de tareas) la quita, como cualquier
  cierre del agente, y la consola lo ve como desconexión.

### 5.10 `screen.unfreeze` (P → A) — pendiente

Payload `{}`. Quita la pantalla en blanco. Es idempotente.

### 5.11 `screen.freeze.ack` (A → P) — pendiente

```json
{ "ref_id": "<id del screen.freeze o screen.unfreeze>", "frozen": true }
```

Confirma el estado en que quedó la pantalla: `true` tras `screen.freeze` y `false` tras
`screen.unfreeze`.

### 5.12 Transmisión de la pantalla del profesor (`screen.broadcast.*`)

> Implementado en v0.4 (Windows), aprobado el 2026-10-29. Decisión técnica en D-014;
> implementación en `internal/broadcast`.

El profesor transmite un monitor suyo como video WebRTC a todos los agentes del canal que
declararon `screen.broadcast`. El WebSocket autenticado lleva **solo la señalización**; el
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

Sin respuesta propia: la sigue `screen.broadcast.offer`. Si el agente no puede mostrar la vista,
responde `error` `internal` y no procesa la oferta.

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

---

## 6. Reservados para fases futuras

Nombres reservados; un agente v0 responde `unknown_type` o `unsupported_capability`.

| Tipo | Descripción |
|---|---|
| `screen.freeze` / `screen.unfreeze` / `screen.freeze.ack` | Pantalla en blanco: documentada en §5.9–5.11, pendiente de implementar. |
| `file.collect` | Recolectar un archivo que el alumno elige entregar. |
| `code.collect` | Recolectar el código que el alumno entrega desde el overlay. |
| `power.*` | Apagar, reiniciar o cerrar sesión de equipos (con aviso previo). |
| `remote.*` | Control remoto de teclado y ratón (con indicador visible y consentimiento). |
| `poll.*` | Encuestas y preguntas rápidas a los alumnos. |

---

## 7. Límites

| Límite | Valor v0 | Al excederlo |
|---|---|---|
| Tamaño de un mensaje WebSocket (envoltura completa) | **1 MiB** (1 048 576 bytes) | `error` `message_too_large` y cierre `1009`. El emisor debe comprobarlo antes de enviar. |
| `code.share.content` | 256 KiB en UTF-8 | `error` `message_too_large`; la sesión sigue. |
| `code.share.title` | 200 caracteres | `error` `invalid_message` |
| `code.share.exercise` | 16 caracteres | `error` `invalid_message` |
| `msg.show.text` | 2000 caracteres | `error` `invalid_message` |
| `screen.thumb.start.interval_ms` | 500–10 000 ms (≤ 2 cuadros/s) | `error` `invalid_message` |
| `screen.thumb.frame.jpeg` | 640 KiB antes de Base64 | `error` `message_too_large`; la sesión sigue. |
| `screen.freeze.message` (pendiente) | 300 caracteres | `error` `invalid_message` |
| `screen.broadcast.offer.sdp` / `answer.sdp` | 64 KiB | `error` `message_too_large`; la sesión sigue. |
| `screen.broadcast.ice.candidate` | 1024 caracteres; 50 por intento y sentido | `error` `invalid_message`; los que sobran se descartan. |
| `screen.broadcast.*.attempt` | 1–1000 | `error` `invalid_message` |
| `app.block.set.apps` | 1–100 nombres de ≤ 64 caracteres | `error` `invalid_message` |
| `web.filter.set.domains` | 1–500 dominios de ≤ 253 caracteres | `error` `invalid_message` |
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
| Video de transmisión falso o interceptado | El SDP (con la huella DTLS) viaja por la sesión autenticada (§3.2), así que el agente solo acepta medios de la consola legítima; el video va cifrado con DTLS-SRTP. |
