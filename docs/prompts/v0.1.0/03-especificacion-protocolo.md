# 03 · Especificación del protocolo y paquete `protocol`

**Prerrequisitos:** etapa 02 completa.
**Duración estimada:** 1–2 sesiones. Es la etapa más importante: revisa el documento con calma antes de aceptar el código.

## Prompt

~~~~markdown
Lee CLAUDE.md y docs/progreso.md. Vamos a diseñar el protocolo v0 de Aula. Trabaja en dos
partes y DETENTE al final de la Parte A para que yo revise.

## Parte A: especificación (docs/protocol.md), en español
1. **Transporte**
   - Descubrimiento: el profesor publica un servicio mDNS (ej. `_aula._tcp`) con TXT records:
     `channel`, `console_name`, `ws_port`, `cert_fingerprint`, `proto=1`.
   - Control: el alumno abre WebSocket sobre TLS al host/puerto anunciados.
   - Explica el aislamiento por canal: con dos profesores en la red, el alumno solo se conecta
     al canal que seleccionó y a la clave configurada.
2. **Envoltura común** de todo mensaje:
   `{ "v": 1, "id": "<uuid>", "ts": <unix_ms>, "type": "<dominio.accion>", "payload": {...} }`
   y una respuesta de error estándar `error { ref_id, code, message }`.
3. **Ciclo de sesión**: `hello` (alumno → profesor: hostname, usuario, SO, versión,
   `capabilities`) → `auth.challenge` (alumno → profesor: nonce) → `auth.proof` (profesor firma
   el nonce con Ed25519) → `auth.ok` → `heartbeat` cada 5 s → reconexión con retroceso exponencial.
4. **Capacidades**: lista que el agente declara según su SO (`code.view`, `msg.show`,
   `screen.thumb`, `screen.freeze`, `screen.broadcast`...). El profesor desactiva en la interfaz
   lo que un agente no soporta.
5. **Mensajes v0** (tabla: tipo, dirección, payload, respuesta):
   - `code.share` (snippet_id, title, lang, content, mode: "replace" | "append")
   - `code.ack`, `code.clear`
   - `msg.show` (text, level: info | warning), `msg.ack`
6. **Reservados para fases futuras** (solo nombre + una línea): `screen.thumb.*`,
   `screen.freeze` / `screen.unfreeze`, `screen.broadcast.*` (WebRTC: señalización),
   `app.block`, `web.filter`, `file.collect`, `code.collect`, `power.*`, `remote.*`, `poll.*`.
7. **Límites**: tamaño máximo de mensaje (1 MiB en v0), longitud máxima de código, versión
   desconocida → error `unsupported_version`.

Muéstrame el documento y espera mi aprobación.

## Parte B: paquete `internal/protocol` (solo cuando yo apruebe)
- Define la envoltura `Envelope` y los tipos de payload de v0.
- Modela el tipo con un campo `Type` y payload; decide entre deserialización en dos pasos
  (type primero, luego payload concreto) o `json.RawMessage`. Justifica.
- `Encode(Envelope) ([]byte, error)` y `Decode([]byte) (Envelope, error)` que validen versión y límites.
- Tipos `Capability` y `HelloInfo`.
- Pruebas: ida y vuelta de cada mensaje, versión desconocida, mensaje demasiado grande, JSON
  inválido. Guarda ejemplos JSON en `internal/protocol/testdata/` y prueba que deserializan.

## Restricciones
- `protocol` solo usa la biblioteca estándar y, si hace falta, un paquete de UUID. Sin red, sin SO.

Actualiza docs/progreso.md al terminar.
~~~~

## Qué revisar en la Parte A
- [ ] ¿Se entiende el flujo sin leer código?
- [ ] ¿Falta algún mensaje que necesites en tu clase (p. ej. numerar ejercicios)?
- [ ] ¿Los nombres de los tipos son consistentes (`dominio.accion`)?
