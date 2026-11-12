# 04 · Módulo A — Descubrimiento mDNS con canales y core WebSocket

**Prerrequisitos:** etapa 03 completa (protocolo aprobado).
**Duración estimada:** 1–2 sesiones.
**Nota:** aquí la conexión va todavía **sin TLS ni autenticación**; ambas llegan en la etapa 05. Marca esos puntos con `// TODO(05)`.

## Prompt

~~~~markdown
Lee CLAUDE.md, docs/protocol.md y docs/progreso.md. Implementa el Módulo A: descubrimiento mDNS
con canales y el núcleo de comunicación WebSocket. Sin TLS ni autenticación todavía
(márcalos con `// TODO(05)`).

## Paquetes
1. `internal/discovery` (mDNS/Zeroconf con `github.com/grandcat/zeroconf`)
   - `Announcer`: el profesor registra el servicio `_aula._tcp` con los TXT records del
     protocolo (channel, console_name, ws_port, cert_fingerprint placeholder, proto).
   - `Browser`: el alumno busca el servicio, filtra por `channel` y entrega las consolas
     encontradas por un channel de Go. Maneja equipos con varias interfaces de red.
   - Cancelación limpia con `context.Context`.
2. `internal/transport` (WebSocket; usa la librería elegida en CLAUDE.md)
   - `Server`: acepta conexiones, lee la envoltura, enruta por `type`. Expone un canal de
     eventos (conexión, mensaje, desconexión).
   - `Client`: se conecta a una consola, envía mensajes, mantiene lectura concurrente.
   - Lectura y escritura con límites de tamaño del protocolo; una goroutine de lectura y un
     canal de escritura por conexión (sin escrituras concurrentes al socket).
3. `internal/session`
   - `Registry`: sesiones por `channel` con id, hostname, usuario, SO, capacidades y última
     señal de vida. Seguro para uso concurrente (mutex o goroutine dueña del estado).
   - Marca desconectada la sesión sin `heartbeat` en 15 s.

## Flujo a cablear
- El profesor: `Announcer` + `Server` + `Registry`.
- El alumno: `Browser` + `Client`, envía `hello` y `heartbeat`, reconecta con retroceso
  exponencial (1,2,4…30 s).

## Pruebas
- Integración en loopback: un servidor y dos clientes simulados se descubren (o se conectan
  por puerto directo si mDNS no corre en CI), aparecen en el registro, y al parar un cliente
  se marca desconectado tras el timeout.
- Prueba de que un anuncio de otro `channel` es ignorado por el browser.
- Ejemplo ejecutable en `examples/dos_alumnos` que imprime el registro en vivo.

## Restricciones
- Toda la lógica en `internal/*`; nada específico de SO. Sin `panic` fuera del arranque.
- No registres contenido de mensajes en logs, solo tipo y origen.

## Criterios de aceptación
- `go test ./...` pasa, incluida la prueba de descubrimiento/registro.
- En VMs reales, levantas el profesor y un alumno aparece y desaparece del registro.

Actualiza docs/progreso.md.
~~~~

## Verificación manual
- [ ] Profesor en tu laptop + alumno en una VM: ¿aparece el alumno, filtrado por canal?
- [ ] Dos canales distintos en la misma red: ¿cada alumno ve solo el suyo?
