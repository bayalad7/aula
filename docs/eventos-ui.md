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
| `aula:broadcast` | `BroadcastView` | Empieza o termina la transmisión de la pantalla del profesor. Llega **antes** que la oferta: la vista y su franja se muestran primero. |
| `aula:broadcast-signal` | `broadcast.Signal` | Una oferta SDP (`kind: "offer"`) o un candidato ICE del profesor (`kind: "ice"`), ya validados por Go. |
| `aula:restrictions` | `policy.State` | Empiezan, cambian o terminan las restricciones de la clase. Llega **antes** de aplicarlas. `{apps, web}`: cada uno `null` o `{policyId, mode, items (≤ 15), total}`. |
| `aula:app-closed` | `AppClosed` | El agente cerró una app bloqueada: `{app: "steam.exe"}`, como mucho uno por app cada 30 s. |

```jsonc
// Status
{ "state": "connected",            // unconfigured | searching | connected | disconnected
  "teacher": "Lab 3 — Prof. Ayala",  // console_name; vacío mientras busca
  "channel": "1",
  "banner": "Aula — conectado a la clase de Lab 3 — Prof. Ayala (canal 1)",
  "detail": "",                      // ayuda extra, p. ej. por qué está sin configurar
  "viewed": false,                   // true mientras el profesor recibe esta pantalla
  "watching": false }                // true mientras esta pantalla muestra la del profesor

// BroadcastView: la transmisión de la pantalla del profesor
{ "active": true, "broadcastId": "7d1f…", "width": 1920, "height": 1080, "fps": 15,
  "teacher": "Lab 3 — Prof. Ayala", "channel": "1",
  "notice": "▶ Estás viendo la pantalla de Lab 3 — Prof. Ayala (canal 1)" }

// Signal: para el RTCPeerConnection de la interfaz; un attempt mayor reemplaza la conexión
{ "kind": "offer", "broadcastId": "7d1f…", "attempt": 1, "sdp": "v=0…" }
{ "kind": "ice", "broadcastId": "7d1f…", "attempt": 1,
  "candidate": "candidate:1 1 udp 2130706431 192.168.1.20 47831 typ host", "sdpMid": "0", "sdpMLineIndex": 0 }

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
| `App.View()` | `View` = `{status, snippets, notices, broadcast}` | Estado completo para el primer dibujo. |
| `App.Copy(snippetId)` | error (promesa rechazada con texto en español) | Copia el fragmento al portapapeles del sistema. |
| `App.DismissNotice(noticeId)` | — | Cierra un aviso; emite `aula:notices`. |
| `App.ReportMedia(webrtc, h264)` | — | Al cargar: si hay `RTCPeerConnection` y decodificador H.264. Go espera este aviso (máx. 5 s) antes de conectarse y solo entonces declara `screen.broadcast`. |
| `App.BroadcastAnswer(broadcastId, attempt, sdp)` | error | La respuesta SDP de la interfaz. Go comprueba que sea solo recepción de video y la envía con el `ref_id` de la oferta. |
| `App.BroadcastCandidate(broadcastId, attempt, candidate, sdpMid, sdpMLineIndex)` | error | Un candidato ICE local; `""` cierra la lista. |
| `App.BroadcastFailed(broadcastId, attempt)` | error | La interfaz no pudo abrir la conexión: Go responde `error` `internal` a la oferta y la consola reintenta. |

`Copy` recibe solo el **ID**: el texto sale de la copia que guarda Go, no de la página. Así lo
que se pega es exactamente lo que envió el profesor, sin depender de cómo lo dibujó el
navegador.

### Reglas de la interfaz

- Todo texto recibido se inserta con `textContent`. El único `innerHTML` es la salida de
  `Prism.highlight`, que escapa el código (una prueba lo verifica).
- La página tiene CSP `script-src 'self'`: sin scripts en línea ni de otros orígenes.
- El distintivo (`banner`) siempre está visible, fijo arriba (ver `docs/privacidad.md`).
- Con `viewed: true` la barra se pone roja y dice "Aula — tu pantalla está siendo vista por
  <profesor> (canal N)". Es un aviso adicional: el principal es el distintivo nativo de Windows,
  que se ve aunque la ventana esté minimizada (`docs/privacidad.md`, regla 6).
- **Transmisión del profesor** (`docs/privacidad.md`, reglas 12–14): con `aula:broadcast`
  activo, Go trae la ventana al frente y la maximiza; la interfaz oculta el código y muestra la
  franja azul fija con `notice` y el `<video>`. El `RTCPeerConnection` vive en la interfaz, sin
  servidores ICE, y solo recibe: nunca pide cámara, micrófono ni pantalla. Los candidatos que
  llegan antes de aplicar la oferta se guardan y se aplican después. Al terminar (`active:
  false`) la interfaz cierra la conexión y Go restaura la ventana. La CSP permite
  `media-src mediastream: blob:` para el video.

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
interfaz ──App.OpenScreens()──▶ screen.Wall ──screen.thumb.start──▶ agentes con screen.thumb
agentes ──screen.thumb.frame──▶ screen.Wall (solo el start vigente) ──"aula:frame"──▶ <img> de la cuadrícula
interfaz ──App.ViewScreen(id)──▶ nuevo start a ese agente (1280 px); ViewScreen("") vuelve a 320 px
interfaz ──App.CloseScreens()──▶ screen.thumb.stop a todos; Go olvida los cuadros
interfaz ──App.StartBroadcast(monitor)──▶ excluye la consola de la captura ──▶ broadcast.Broadcaster
          ──screen.broadcast.start/offer/ice──▶ agentes con screen.broadcast ──answer/ice──▶ Go
broadcast.Broadcaster (receptores conectados) ──"aula:broadcast"──▶ franja roja "a M de N alumnos"
interfaz ──App.StopBroadcast()──▶ screen.broadcast.stop a todos; la consola vuelve a las capturas
```

### Eventos (Go → interfaz)

| Evento | Datos | Cuándo |
|---|---|---|
| `aula:students` | `Students` | Un agente entra, sale, se queda mudo o confirma código. |
| `aula:history` | `History` | Tras cada emisión o reemisión. |
| `aula:deliveries` | `Deliveries` | Tras cada envío y cada acuse (`code.ack`, `msg.ack`). |
| `aula:capture` | `Capture` | Al pulsar el atajo global o el botón Capturar. |
| `aula:class` | `View` | Al terminar el arranque automático en el canal predeterminado. Si falló, `active` es `false` y el motivo está en `Setup().startError`. |
| `aula:frame` | `Thumb` | Por cada cuadro recibido mientras la vista de pantallas está abierta (≤ 1 por segundo y alumno). Solo cuadros del `screen.thumb.start` vigente. |
| `aula:broadcast` | `BroadcastState` | La transmisión de tu pantalla empieza, termina o cambia su número de receptores. |
| `aula:restrictions` | `RestrictionsView` | Cambian las restricciones de la clase: `{apps, web}`, cada uno `null` o `{policyId, mode, items, hosts}` (`hosts` vacío: todo el canal). Los acuses de cada alumno llegan en `aula:students` (`appsActive`, `webActive`). |

```jsonc
// Students: la lista del canal y los conteos que habilitan los botones
{ "students": [ { "id": "…", "hostname": "LAB3-PC07", "username": "alumno07", "os": "windows",
                  "osVersion": "…", "agentVersion": "0.1.0", "state": "connected",
                  "capabilities": ["code.view", "msg.show", "screen.thumb"], "canCode": true,
                  "canNotice": true, "canThumb": true, "canBroadcast": true,
                  "lastCodeAck": "0c9a…", "ackedAt": "10:00:05" } ],
  "connected": 1, "canCode": 1, "canNotice": 1, "canThumb": 1, "canBroadcast": 1 }

// History: fragmentos de la sesión, numerados ("Ejercicio N")
{ "entries": [ { "snippetId": "…", "number": 1, "title": "Factorial", "lang": "python",
                 "content": "…", "times": 2, "firstSentAt": "…", "lastSentAt": "…" } ] }

// Deliveries: envíos recientes, el más nuevo primero
{ "deliveries": [ { "id": "d3", "kind": "code", "label": "Ejercicio 1 · Factorial",
                    "sent": 2, "acked": 2, "failed": 0, "noCapability": 1, "at": "…" } ] }

// Capture: o draft o error
{ "draft": { "content": "…", "lang": "python", "lines": 9, "source": "hotkey", "capturedAt": "…" },
  "error": "" }

// Thumb (screen.Thumb): el último cuadro de un alumno; la interfaz lo pinta como
// "data:image/jpeg;base64," + jpeg
{ "sessionId": "…", "seq": 12, "width": 320, "height": 180,
  "jpeg": "/9j/4AAQ…",               // JPEG en Base64
  "detail": false,                   // true si es el alumno de la vista en detalle (1280 px)
  "at": "2026-10-29T10:00:05-06:00" }

// BroadcastState: la transmisión de la pantalla del profesor
{ "active": true, "display": 0,      // monitor transmitido; -1 sin transmisión
  "receivers": 2,                    // alumnos con la conexión de video arriba
  "eligible": 3,                     // alumnos conectados que pueden recibirla
  "available": true }                // este equipo puede capturar sus monitores

// DisplayInfo (Displays()): un monitor
{ "index": 0, "label": "Monitor 1 (principal) · 1920×1080", "width": 1920, "height": 1080, "primary": true }
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
| `OpenScreens()` | error | Abre la vista de pantallas (botón **Ver pantallas**): envía `screen.thumb.start` (1000 ms, calidad 50, 320 px) a cada alumno conectado con `canThumb`, y también a los que se conecten mientras siga abierta. Error si no hay clase. |
| `CloseScreens()` | — | Cierra la vista (**Cerrar** o Esc): envía `screen.thumb.stop` a todos los que transmitían y olvida los cuadros. |
| `ViewScreen(sessionId)` | — | Vista en detalle de un alumno (clic en su miniatura): nuevo `screen.thumb.start` con 1280 px y calidad 70 solo a él. `ViewScreen("")` lo devuelve a miniatura. |
| `Displays()` | `DisplayInfo[]` | Monitores para el selector de la transmisión. |
| `StartBroadcast(monitor)` | `BroadcastState` / error | **Transmitir mi pantalla.** Excluye la ventana de la consola de la captura (si no puede, no transmite) y envía `screen.broadcast.start` + oferta a cada alumno con `canBroadcast`, y a los que se conecten después. Error si no hay clase, si ya transmite o si falta el codificador H.264. |
| `StopBroadcast()` | `BroadcastState` | **Detener.** `screen.broadcast.stop` a todos; la consola vuelve a aparecer en las capturas. También ocurre al cambiar de canal o cerrar la clase. |
| `BroadcastState()` | `BroadcastState` | Estado actual (también viene en `View().broadcast`). |
| `Restrictions()` | `RestrictionsView` | Restricciones de la clase (también en `View().restrictions`). |
| `SetRestriction({kind, mode, items, hosts})` | `RestrictionsView` / error | `kind` `apps` o `web`; `mode` `deny` o `allow`; `items` como los escribe el profesor (Go quita rutas, `.exe` duplicados, `https://`, `www.` y rutas de URL); `hosts` los alumnos marcados, o vacío para todo el canal. Se reenvía sola a quien se reconecta. |
| `ClearRestriction(kind)` | `RestrictionsView` | Quita una de las dos. |
| `ClearAllRestrictions()` | `RestrictionsView` | **Quitar todas las restricciones.** Cerrar la clase o cambiar de canal también las quita. |

### Reglas

- **Capturar nunca emite.** El atajo y el botón solo cargan el editor y proponen un lenguaje; el
  profesor lo revisa y pulsa un botón de emisión.
- **Los botones se deshabilitan** si ningún alumno conectado declaró la capacidad (`canCode`,
  `canNotice`). Cada `Delivery` informa cuántos no la tienen (`noCapability`).
- **Los acuses se cuentan por ID de sobre:** cada alumno recibe un sobre distinto y su `ref_id`
  identifica el envío. Un mismo sobre cuenta una sola vez.
- **La interfaz no usa `innerHTML`** (lo verifica una prueba).
- **Las pantallas solo se transmiten con la vista abierta.** La interfaz llama a
  `CloseScreens()` al cerrarla y al cambiar de canal o de nombre; Go además detiene todo al
  cerrar la clase (`EndClass`), y cada agente se detiene solo si se pierde la conexión.
- **La transmisión de la pantalla es visible para los dos lados:** la consola muestra la franja
  roja con "Detener" mientras dura y su propia ventana no aparece en el video
  (`docs/privacidad.md`, regla 15).
- **Los cuadros solo viven en memoria:** Go guarda el último de cada alumno mientras la vista
  está abierta y la interfaz solo los tiene en sus `<img>`. Nada se escribe en disco
  (`docs/privacidad.md`, regla 7). La CSP permite `img-src data:` para mostrarlos.
