# Contrato backend ↔ interfaz de las apps Wails

Cada app (`apps/student` y `apps/teacher`) tiene un backend Go y una interfaz web (Wails). Se
comunican por **eventos** (Go → interfaz) y **métodos enlazados** (interfaz → Go). En cada app,
la definición está en `events.go`, con los nombres de evento repetidos en
`frontend/dist/events.js`. Una prueba (`TestEventContractMatchesFrontend`) falla si no coinciden
o si la interfaz llama a un método que no existe.

## Cliente del alumno (`apps/student`)

### Flujo

```
consola ──code.share──▶ agente (session.Agent)
                          │ codedist.Receiver: aplica al tablero y arma el code.ack
                          ├──code.ack──▶ consola
                          └──runtime.EventsEmit("aula:board", Board)──▶ interfaz (Prism)
interfaz ──App.Copy(id)──▶ Go ──platform.Clipboard──▶ portapapeles del sistema
```

Al cargar, la interfaz **primero se suscribe** a los eventos y **después** pide el estado
completo con `App.View()`, para no perder nada que llegue entre medio.

### Eventos (Go → interfaz)

| Evento | Datos | Cuándo |
|---|---|---|
| `aula:status` | `Status` | Cambia la conexión: buscando, conectado, desconectado, sin configurar. |
| `aula:board` | `Board` | Llega `code.share` o `code.clear`. |
| `aula:notices` | `Notices` | Llega `msg.show` o el alumno cierra un aviso. |

```jsonc
// Status
{ "state": "connected",            // unconfigured | searching | connected | disconnected
  "teacher": "Lab 3 — Prof. Ayala",  // console_name; vacío mientras busca
  "channel": "AULA_101",
  "banner": "Aula — conectado a la clase de Lab 3 — Prof. Ayala (canal AULA_101)",
  "detail": "" }                     // ayuda extra, p. ej. por qué está sin configurar

// Board: lista completa, la más antigua primero (máx. 20 con "append")
{ "snippets": [ { "id": "0c9a…", "exercise": "3", "title": "Factorial", "lang": "python",
                  "content": "def f(n):\n    …", "receivedAt": "2026-10-28T10:00:00-06:00" } ] }

// Notices: avisos no cerrados, el más antiguo primero (máx. 10)
{ "notices": [ { "id": "5b0e…", "text": "Guarden su trabajo", "level": "warning", "at": "…" } ] }
```

Cada evento lleva la lista **completa**, no un cambio incremental: la interfaz redibuja sin
llevar estado propio.

### Métodos enlazados (interfaz → Go)

| Método | Devuelve | Uso |
|---|---|---|
| `App.View()` | `View` = `{status, snippets, notices}` | Estado completo para el primer dibujo. |
| `App.Copy(snippetId)` | error (promesa rechazada con texto en español) | Copia el fragmento al portapapeles del sistema. |
| `App.DismissNotice(noticeId)` | — | Cierra un aviso; emite `aula:notices`. |

`Copy` recibe solo el **ID**: el texto sale de la copia que guarda Go, no de la página. Así lo
que se pega es exactamente lo que envió el profesor, sin depender de cómo lo dibujó el
navegador.

### Reglas de la interfaz

- Todo texto recibido se inserta con `textContent`. El único `innerHTML` es la salida de
  `Prism.highlight`, que escapa el código (una prueba lo verifica).
- La página tiene CSP `script-src 'self'`: sin scripts en línea ni de otros orígenes.
- El distintivo (`banner`) siempre está visible, fijo arriba (ver `docs/privacidad.md`).

## Consola del profesor (`apps/teacher`)

Definición en `apps/teacher/events.go`, nombres repetidos en `apps/teacher/frontend/dist/events.js`
(misma prueba de contrato). El estado (aula abierta, alumnos, historial, confirmaciones) vive en
Go; la interfaz solo lo dibuja.

### Flujo

```
Go (al arrancar, canal predeterminado) ──"aula:class"──▶ interfaz: clase abierta o error de arranque
interfaz ──App.StartClass(canal, nombre)──▶ Go: cierra la clase actual y abre la del canal pedido
agentes ──hello/heartbeat/desconexión──▶ session.Console.OnChange ──"aula:students"──▶ interfaz
interfaz ──App.Emit({title, lang, content, mode})──▶ Go: historial + Broadcast(code.share)
agentes ──code.ack / msg.ack──▶ codedist.Deliveries ──"aula:deliveries"──▶ "confirmados 2/2 ✓"
Ctrl+Alt+Shift+C (o botón Capturar) ──▶ Go lee el portapapeles ──"aula:capture"──▶ editor (sin emitir)
```

### Eventos (Go → interfaz)

| Evento | Datos | Cuándo |
|---|---|---|
| `aula:students` | `Students` | Un agente entra, sale, se queda mudo o confirma código. |
| `aula:history` | `History` | Tras cada emisión o reemisión. |
| `aula:deliveries` | `Deliveries` | Tras cada envío y cada acuse (`code.ack`, `msg.ack`). |
| `aula:capture` | `Capture` | Al pulsar el atajo global o el botón Capturar. |
| `aula:class` | `View` | Al terminar el arranque automático en el canal predeterminado. Si falló, `active` es `false` y el motivo está en `Setup().startError`. |

```jsonc
// Students: la lista del canal y los conteos que habilitan los botones
{ "students": [ { "id": "…", "hostname": "LAB3-PC07", "username": "alumno07", "os": "windows",
                  "osVersion": "…", "agentVersion": "0.1.0", "state": "connected",
                  "capabilities": ["code.view", "msg.show"], "canCode": true, "canNotice": true,
                  "lastCodeAck": "0c9a…", "ackedAt": "10:00:05" } ],
  "connected": 1, "canCode": 1, "canNotice": 1 }

// History: fragmentos de la sesión, numerados ("Ejercicio N")
{ "entries": [ { "snippetId": "…", "number": 1, "title": "Factorial", "lang": "python",
                 "content": "…", "times": 2, "firstSentAt": "…", "lastSentAt": "…" } ] }

// Deliveries: envíos recientes, el más nuevo primero
{ "deliveries": [ { "id": "d3", "kind": "code", "label": "Ejercicio 1 · Factorial",
                    "sent": 2, "acked": 2, "failed": 0, "noCapability": 1, "at": "…" } ] }

// Capture: o draft o error
{ "draft": { "content": "…", "lang": "python", "lines": 9, "source": "hotkey", "capturedAt": "…" },
  "error": "" }
```

### Métodos enlazados (interfaz → Go)

| Método | Devuelve | Uso |
|---|---|---|
| `Setup()` | `Setup` | Canal predeterminado (`defaultChannel`), canales recientes, nombre, huella, origen de la clave (`keySource`: `centro` o `personal`) y `startError` si el arranque automático falló. |
| `StartClass(canal, nombre)` | `View` / error | Abre la clase en el canal (número 1–16000); si ya había una abierta, la cierra antes (selector "Canal … Cambiar" y "Guardar" del nombre). Usa la clave del centro o, si no hay, la personal. |
| `EndClass()` | — | Cierra la clase. La interfaz no lo llama directamente. |
| `View()` | `View` | Estado completo para el primer dibujo. |
| `Emit({title, lang, content, mode})` | `Delivery` / error | `mode` = `replace` ("Emitir a todos") o `append` ("Agregar bloque"). Crea una entrada nueva en el historial. |
| `Reemit(snippetId)` | `Delivery` / error | Reenvía un fragmento del historial con el mismo número e ID (`replace`). |
| `ClearScreens()` | `Delivery` / error | `code.clear` a todos. |
| `SendNotice(text, level)` | `Delivery` / error | `msg.show` a todos, `info` o `warning`. |
| `CaptureClipboard()` | `Draft` / error | Igual que el atajo global; el resultado también llega como `aula:capture`. |

### Reglas

- **Capturar nunca emite.** El atajo y el botón solo cargan el editor y proponen un lenguaje; el
  profesor lo revisa y pulsa un botón de emisión.
- **Los botones se deshabilitan** si ningún alumno conectado declaró la capacidad (`canCode`,
  `canNotice`). Cada `Delivery` informa cuántos no la tienen (`noCapability`).
- **Los acuses se cuentan por ID de sobre:** cada alumno recibe un sobre distinto y su `ref_id`
  identifica el envío. Un mismo sobre cuenta una sola vez.
- **La interfaz no usa `innerHTML`** (lo verifica una prueba).
