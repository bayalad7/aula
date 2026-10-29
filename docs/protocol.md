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
| `username` | string | Usuario de la sesión del SO, 0–256 caracteres (vacío si no se puede obtener). |
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
| `screen.thumb` | `screen.thumb.*` | Reservada (Fase 2) |
| `screen.freeze` | `screen.freeze`, `screen.unfreeze` | Reservada (Fase 2) |
| `screen.broadcast` | `screen.broadcast.*` | Reservada (Fase 2) |
| `app.block` | `app.block` | Reservada |
| `web.filter` | `web.filter` | Reservada |
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

---

## 6. Reservados para fases futuras

Nombres reservados; un agente v0 responde `unknown_type` o `unsupported_capability`.

| Tipo | Descripción |
|---|---|
| `screen.thumb.*` | Solicitud y envío periódico de miniaturas de pantalla (con indicador visible en el agente). |
| `screen.freeze` / `screen.unfreeze` | Bloquear la pantalla del alumno con un aviso a pantalla completa y liberarla. |
| `screen.broadcast.*` | Señalización WebRTC (offer/answer/ICE) para transmitir la pantalla del profesor. |
| `app.block` | Lista de aplicaciones que el agente no permite abrir durante la clase. |
| `web.filter` | Lista de sitios permitidos o bloqueados durante la clase. |
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
