# Protocolo de Aula v0

Especificación del protocolo de comunicación entre la consola del maestro y los agentes
de los alumnos.

> **Regla:** cada mensaje nuevo se documenta aquí antes de implementarse.

---

## 1. Transporte

### 1.1 Descubrimiento (UDP broadcast)

La consola envía cada **2 segundos** un datagrama UDP broadcast al puerto **47800**.
El datagrama contiene un objeto JSON codificado en UTF-8:

```json
{
  "aula": 1,
  "class_id": "laboratorio-A",
  "console_name": "Profesor García",
  "ws_port": 47801,
  "cert_fingerprint": "SHA256:<hex de 64 caracteres>"
}
```

| Campo              | Tipo   | Descripción                                                    |
|--------------------|--------|----------------------------------------------------------------|
| `aula`             | `u32`  | Versión del protocolo de descubrimiento. Siempre `1` en v0.    |
| `class_id`         | `str`  | Identificador del grupo/clase. Configurado en la consola.      |
| `console_name`     | `str`  | Nombre legible de la consola (para mostrar al alumno).         |
| `ws_port`          | `u16`  | Puerto TCP en el que la consola acepta conexiones WebSocket.   |
| `cert_fingerprint` | `str`  | Huella SHA-256 del certificado TLS de la consola, prefijo `SHA256:`. |

El agente escucha en el puerto 47800. Al recibir un anuncio:

1. Verifica que `aula == 1`; si no, lo ignora.
2. Compara `class_id` con el valor configurado localmente. Si no coincide, lo ignora.
3. Compara `cert_fingerprint` con la huella esperada (derivada de la clave pública Ed25519
   configurada). Si no coincide, lo ignora.
4. Si pasa las tres comprobaciones y el agente no tiene una conexión activa, inicia una
   conexión WebSocket TLS a la IP de origen en el `ws_port` indicado.

### 1.2 Canal de control (WebSocket sobre TLS)

El agente se conecta a la consola (nunca al revés) por WebSocket sobre TLS al puerto
indicado en el anuncio (por defecto **47801**).

- El certificado TLS de la consola es autofirmado. El agente lo valida comparando su
  huella SHA-256 contra `cert_fingerprint` del anuncio y la clave pública configurada.
- Todos los mensajes viajan como frames de texto WebSocket con contenido JSON en UTF-8.
- La conexión se mantiene abierta mientras dure la sesión.

### 1.3 Conflicto: dos consolas en la misma red

Si hay dos consolas transmitiendo en la misma red:

- **Distinto `class_id`:** el agente solo responde a la que coincide con su configuración.
  Las demás se ignoran silenciosamente.
- **Mismo `class_id` pero distinta clave pública:** el agente solo acepta la consola cuya
  clave pública coincide con la configurada. La segunda se ignora y se registra una
  advertencia con `tracing::warn!`.
- **Mismo `class_id` y misma clave (misma consola, dos interfaces de red):** el agente usa
  la primera conexión establecida exitosamente y mantiene la IP de origen para reconexión.

---

## 2. Envoltura común de mensajes

Todos los mensajes (en ambas direcciones) usan esta estructura envolvente:

```json
{
  "v": 1,
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "ts": 1728400000000,
  "type": "dominio.accion",
  "payload": { }
}
```

| Campo     | Tipo   | Descripción                                                        |
|-----------|--------|--------------------------------------------------------------------|
| `v`       | `u32`  | Versión del protocolo. `1` en v0.                                  |
| `id`      | `str`  | UUID v4 único del mensaje. Sirve para correlacionar respuestas.    |
| `ts`      | `u64`  | Marca de tiempo Unix en milisegundos (generada por el emisor).     |
| `type`    | `str`  | Tipo del mensaje en formato `dominio.accion`.                      |
| `payload` | `obj`  | Contenido específico del tipo. Puede ser `{}` si no hay datos.     |

### 2.1 Respuesta de error estándar

Cuando un mensaje no puede procesarse, el receptor responde con:

```json
{
  "v": 1,
  "id": "<nuevo uuid>",
  "ts": 1728400001000,
  "type": "error",
  "payload": {
    "ref_id": "<id del mensaje original>",
    "code": "invalid_payload",
    "message": "El campo 'lang' es obligatorio"
  }
}
```

| Campo     | Tipo  | Descripción                                              |
|-----------|-------|----------------------------------------------------------|
| `ref_id`  | `str` | `id` del mensaje que causó el error.                     |
| `code`    | `str` | Código de error legible por máquina (ver tabla abajo).   |
| `message` | `str` | Descripción legible por humanos (para logs, no para UI). |

**Códigos de error estándar:**

| Código                 | Significado                                              |
|------------------------|----------------------------------------------------------|
| `unsupported_version`  | El campo `v` tiene un valor que el receptor no soporta.  |
| `unknown_type`         | El `type` del mensaje no es reconocido.                  |
| `invalid_payload`      | El payload no cumple el esquema esperado para su tipo.   |
| `auth_required`        | Se recibió un mensaje antes de completar la autenticación. |
| `auth_failed`          | La firma del nonce no es válida.                         |
| `capability_missing`   | El agente no soporta la capacidad requerida.             |
| `rate_limited`         | Se excedió la frecuencia permitida de mensajes.          |
| `message_too_large`    | El mensaje excede el tamaño máximo permitido.            |

---

## 3. Ciclo de sesión

La sesión sigue esta secuencia tras establecer la conexión WebSocket TLS:

```
Agente                                          Consola
  │                                                │
  │─── hello ─────────────────────────────────────▶│
  │                                                │
  │◀── auth.challenge ────────────────────────────│
  │                                                │
  │◀── auth.proof ────────────────────────────────│
  │                                                │
  │─── auth.ok ──────────────────────────────────▶│
  │                                                │
  │◀── heartbeat ─────────────────────────────────│
  │─── heartbeat ─────────────────────────────────▶│
  │    (cada 5 s, ambos extremos)                  │
  │                                                │
  │    ... mensajes de la sesión ...               │
```

### 3.1 `hello` (agente → consola)

El agente envía este mensaje inmediatamente al conectarse.

```json
{
  "type": "hello",
  "payload": {
    "hostname": "PC-LAB-12",
    "username": "alumno",
    "os": "windows",
    "os_version": "10.0.19045",
    "agent_version": "0.1.0",
    "capabilities": ["code.view", "msg.show"]
  }
}
```

| Campo           | Tipo       | Descripción                                            |
|-----------------|------------|--------------------------------------------------------|
| `hostname`      | `str`      | Nombre del equipo.                                     |
| `username`      | `str`      | Nombre del usuario de sesión actual.                   |
| `os`            | `str`      | Sistema operativo: `"windows"`, `"linux"` o `"macos"`. |
| `os_version`    | `str`      | Versión del SO.                                        |
| `agent_version` | `str`      | Versión semver del agente.                             |
| `capabilities`  | `[str]`    | Lista de capacidades soportadas (ver sección 4).       |

### 3.2 `auth.challenge` (consola → agente)

La consola responde al `hello` con un reto:

```json
{
  "type": "auth.challenge",
  "payload": {
    "ref_id": "<id del hello>",
    "nonce": "<32 bytes aleatorios codificados en base64>"
  }
}
```

### 3.3 `auth.proof` (consola → agente)

La consola firma el nonce con su clave privada Ed25519 y envía la prueba:

```json
{
  "type": "auth.proof",
  "payload": {
    "ref_id": "<id del auth.challenge>",
    "signature": "<firma Ed25519 del nonce, codificada en base64>"
  }
}
```

El agente verifica la firma usando la clave pública configurada. Si falla, el agente
cierra la conexión y registra el evento con `tracing::warn!`.

### 3.4 `auth.ok` (agente → consola)

Si la firma es válida, el agente confirma:

```json
{
  "type": "auth.ok",
  "payload": {
    "ref_id": "<id del auth.proof>"
  }
}
```

A partir de este punto la sesión está autenticada y se pueden intercambiar mensajes.

### 3.5 `heartbeat` (bidireccional)

Ambos extremos envían un heartbeat cada **5 segundos**:

```json
{
  "type": "heartbeat",
  "payload": {}
}
```

- Si un extremo no recibe un heartbeat ni ningún otro mensaje en **15 segundos** (3 ciclos
  perdidos), considera la conexión muerta y la cierra.
- El heartbeat no requiere respuesta.

### 3.6 Reconexión

Cuando el agente pierde la conexión:

1. Espera **1 segundo** antes del primer reintento.
2. Cada intento fallido duplica la espera: 1 s → 2 s → 4 s → 8 s → ... hasta un máximo
   de **30 segundos**.
3. Mientras esté desconectado, sigue escuchando anuncios UDP para detectar cambios de IP
   de la consola.
4. Al reconectarse, repite el ciclo completo: `hello` → `auth.challenge` → `auth.proof`
   → `auth.ok`.

---

## 4. Capacidades

Las capacidades indican qué funciones soporta el agente en su plataforma. Se declaran
en el `hello` y la consola las usa para habilitar o deshabilitar controles en la interfaz.

| Capacidad        | Descripción                                          | Etapa |
|------------------|------------------------------------------------------|-------|
| `code.view`      | Puede recibir y mostrar fragmentos de código.        | 02    |
| `msg.show`       | Puede mostrar mensajes emergentes al alumno.         | 02    |
| `screen.thumb`   | Puede enviar miniaturas de pantalla.                 | 06    |
| `screen.lock`    | Puede bloquear la pantalla del equipo.               | 10    |
| `screen.unlock`  | Puede desbloquear la pantalla del equipo.            | 10    |
| `input.block`    | Puede bloquear teclado y ratón.                      | 10    |
| `code.collect`   | Puede recoger el código abierto del alumno.          | 08    |
| `file.collect`   | Puede recoger archivos del equipo del alumno.        | 08    |
| `app.list`       | Puede listar aplicaciones en ejecución.              | 10    |

Una capacidad ausente no es un error: la consola simplemente desactiva el botón o control
correspondiente. Si la consola envía un mensaje que requiere una capacidad no declarada,
el agente responde con el error `capability_missing`.

---

## 5. Mensajes v0

Mensajes disponibles en la versión 0 del protocolo, una vez autenticada la sesión.

### 5.1 Tabla resumen

| Tipo          | Dirección        | Descripción                            | Respuesta esperada |
|---------------|------------------|----------------------------------------|--------------------|
| `code.share`  | consola → agente | Envía un fragmento de código al alumno | `code.ack`         |
| `code.ack`    | agente → consola | Confirma recepción del código          | —                  |
| `code.clear`  | consola → agente | Limpia el código mostrado en el agente | `code.ack`         |
| `msg.show`    | consola → agente | Muestra un mensaje al alumno           | `msg.ack`          |
| `msg.ack`     | agente → consola | Confirma que el alumno vio el mensaje  | —                  |

### 5.2 `code.share` (consola → agente)

Envía un fragmento de código para que el alumno lo vea y pueda copiarlo.

```json
{
  "type": "code.share",
  "payload": {
    "snippet_id": "ejercicio-01",
    "title": "Ejemplo: Hola Mundo",
    "lang": "python",
    "content": "print('Hola, mundo')",
    "mode": "replace"
  }
}
```

| Campo        | Tipo   | Descripción                                                     |
|--------------|--------|-----------------------------------------------------------------|
| `snippet_id` | `str`  | Identificador único del fragmento (para tracking).              |
| `title`      | `str`  | Título para mostrar al alumno.                                  |
| `lang`       | `str`  | Lenguaje para resaltado de sintaxis (valor de highlight.js).    |
| `content`    | `str`  | Código fuente. Máximo 512 KiB en UTF-8.                         |
| `mode`       | `str`  | `"replace"`: sustituye el código actual. `"append"`: lo agrega. |

### 5.3 `code.ack` (agente → consola)

Confirma que el agente recibió y mostró el código (o lo limpió).

```json
{
  "type": "code.ack",
  "payload": {
    "ref_id": "<id del code.share o code.clear>"
  }
}
```

### 5.4 `code.clear` (consola → agente)

Limpia el fragmento de código mostrado en el agente.

```json
{
  "type": "code.clear",
  "payload": {}
}
```

El agente responde con `code.ack`.

### 5.5 `msg.show` (consola → agente)

Muestra un mensaje emergente al alumno.

```json
{
  "type": "msg.show",
  "payload": {
    "text": "Tienen 10 minutos para terminar el ejercicio.",
    "level": "info"
  }
}
```

| Campo   | Tipo   | Descripción                                                      |
|---------|--------|------------------------------------------------------------------|
| `text`  | `str`  | Texto del mensaje. Máximo 4 KiB en UTF-8.                       |
| `level` | `str`  | Nivel visual: `"info"` (neutro) o `"warning"` (destacado/urgente). |

### 5.6 `msg.ack` (agente → consola)

Confirma que el mensaje fue mostrado al alumno.

```json
{
  "type": "msg.ack",
  "payload": {
    "ref_id": "<id del msg.show>"
  }
}
```

---

## 6. Mensajes reservados para fases futuras

Estos tipos están reservados. No se implementan en v0 pero se documentan aquí para
garantizar que no haya colisiones de nombres.

| Tipo                   | Descripción breve                                             |
|------------------------|---------------------------------------------------------------|
| `screen.thumb.request` | La consola solicita una miniatura de la pantalla del alumno.  |
| `screen.thumb.data`    | El agente envía la miniatura (JPEG comprimido).               |
| `screen.lock`          | La consola ordena bloquear la pantalla del alumno.            |
| `screen.unlock`        | La consola ordena desbloquear la pantalla del alumno.         |
| `screen.broadcast.*`   | Transmisión de la pantalla del maestro a los alumnos.         |
| `input.block`          | Bloquea teclado y ratón del alumno.                           |
| `input.unblock`        | Desbloquea teclado y ratón del alumno.                        |
| `app.block`            | Bloquea el lanzamiento de aplicaciones no autorizadas.        |
| `web.filter`           | Aplica un filtro de URLs permitidas/bloqueadas.               |
| `file.collect`         | Recoge un archivo específico del equipo del alumno.           |
| `code.collect`         | Recoge el código actualmente abierto en el editor del alumno. |
| `power.shutdown`       | Apaga el equipo del alumno.                                   |
| `power.restart`        | Reinicia el equipo del alumno.                                |
| `power.logoff`         | Cierra la sesión del alumno.                                  |
| `remote.view`          | Inicia la visualización remota de la pantalla del alumno.     |
| `remote.control`       | Inicia el control remoto del equipo del alumno.               |

---

## 7. Límites

| Parámetro                        | Valor en v0    |
|----------------------------------|----------------|
| Tamaño máximo de mensaje (frame) | 1 MiB (1 048 576 bytes) |
| Tamaño máximo de `content` en `code.share` | 512 KiB |
| Tamaño máximo de `text` en `msg.show`      | 4 KiB   |
| Intervalo de heartbeat           | 5 s            |
| Timeout de heartbeat (conexión muerta) | 15 s     |
| Intervalo de anuncio UDP         | 2 s            |
| Backoff de reconexión mínimo     | 1 s            |
| Backoff de reconexión máximo     | 30 s           |
| Puerto UDP de descubrimiento     | 47800          |
| Puerto WebSocket TLS (por defecto) | 47801        |

### Versión desconocida

Si un extremo recibe un mensaje con `v` distinto de `1`, responde con un error
`unsupported_version` y **no** procesa el mensaje:

```json
{
  "type": "error",
  "payload": {
    "ref_id": "<id del mensaje>",
    "code": "unsupported_version",
    "message": "Solo se soporta v=1"
  }
}
```

---

## 8. Seguridad

### 8.1 Modelo de confianza

Cada instalación de la consola del maestro posee una pareja de claves Ed25519 que funciona
como su identidad criptográfica. La clave pública de la consola se incluye en la
configuración de cada agente al momento de instalarlo (distribución fuera de banda).

Los agentes solo aceptan órdenes de una consola que demuestre poseer la clave privada
Ed25519 correspondiente a la clave pública configurada.

### 8.2 TLS (capa de transporte)

- La consola genera un certificado TLS autofirmado (ECDSA P-256) en su primer arranque y
  lo guarda en disco.
- El anuncio UDP incluye el campo `cert_fingerprint` con la huella SHA-256 del certificado
  DER (formato `SHA256:<64 hex>`).
- Al conectarse, el agente valida que el certificado presentado por la consola coincida
  con la huella recibida en el anuncio UDP (**fijado de certificado / certificate pinning**).
- Si la huella no coincide, el agente rechaza la conexión TLS y registra el evento con
  `tracing::warn!`.

### 8.3 Autenticación Ed25519 (capa de aplicación)

Sobre la conexión TLS ya establecida, el protocolo realiza una autenticación adicional
para verificar que la consola posee la clave privada Ed25519 esperada:

1. El agente envía `hello`.
2. La consola responde con `auth.challenge` que contiene un nonce aleatorio de 32 bytes
   (codificado en base64). El nonce lo genera la consola.
3. La consola firma los bytes crudos del nonce con su clave privada Ed25519 y envía
   `auth.proof` con la firma codificada en base64.
4. El agente decodifica el nonce y la firma, y verifica la firma usando la clave pública
   Ed25519 configurada.
   - Si la firma es válida: envía `auth.ok` y la sesión queda autenticada.
   - Si la firma es inválida: cierra la conexión y registra el intento con
     `tracing::warn!`.

La combinación de TLS (integridad y confidencialidad del canal) con la verificación
Ed25519 (identidad de la consola) impide ataques de intermediario (MITM) y suplantación
de consola.

### 8.4 Gestión de claves

- **`console keygen`**: genera la pareja Ed25519 y muestra la clave pública en base64
  para copiarla a la configuración de los agentes.
- La clave privada se guarda en un archivo con permisos restringidos (solo lectura del
  propietario) en el equipo de la consola.
- La clave privada **nunca** se registra en logs, se incluye en binarios ni se transmite
  por la red.
- El certificado TLS y su clave privada son independientes de la pareja Ed25519.

### 8.5 Resumen de capas de seguridad

| Capa       | Mecanismo                        | Protege contra                    |
|------------|----------------------------------|-----------------------------------|
| Transporte | TLS con certificado autofirmado  | Escucha pasiva, inyección         |
| Fijado     | Huella SHA-256 en anuncio UDP    | Certificado TLS suplantado (MITM) |
| Identidad  | Firma Ed25519 del nonce          | Consola impostora con otro par de claves |
