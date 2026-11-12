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

**Go manda estados, no frases** (prompt 19). Los textos de la ventana están en
`frontend/dist/textos.es-MX.js` y la interfaz pone el título de la ventana, que también se ve en
la barra de tareas (`runtime.WindowSetTitle`), a partir de `Status`, de las restricciones y de la
orden de energía (`docs/privacidad.md`, reglas 1, 6, 12, 20 y 26). `TestTextsAreComplete` exige un
texto por cada estado, acción de energía, nivel de aviso y modo de restricción que Go puede mandar.
El único texto que sigue en Go es el del distintivo nativo de la regla 6 (`screen.IndicatorText`),
que no es parte de la página.

### Eventos (Go → interfaz)

| Evento | Datos | Cuándo |
|---|---|---|
| `aula:status` | `Status` | Cambia la conexión: buscando, conectado, desconectado, sin configurar. |
| `aula:board` | `Board` | Llega `code.share` o `code.clear`. |
| `aula:notices` | `Notices` | Llega `msg.show`, el profesor toma una captura de esta pantalla (aviso `kind: "snapshot"`, con `notify`, nunca `raise`) o el alumno cierra un aviso. |
| `aula:broadcast` | `BroadcastView` | Empieza o termina la transmisión de la pantalla del profesor. Llega **antes** que la oferta: la vista y su franja se muestran primero. |
| `aula:broadcast-signal` | `broadcast.Signal` | Una oferta SDP (`kind: "offer"`) o un candidato ICE del profesor (`kind: "ice"`), ya validados por Go. |
| — | `View().capabilities` | Las capacidades que este equipo declara (las de `hello`): la ventana arma «Qué puede hacer Aula en este equipo» con la misma grilla que la ficha de la consola. Se piden al abrir el diálogo (nota de interfaz 13). |
| `aula:present` | `broadcast.PresentView` | Tu pantalla mostrada a la clase (§5.24): `{phase, presentId}`. `phase: "live"` (la banda, y `Status.presenting`; Go trae la ventana al frente con `raise` antes del primer cuadro) o `""` (nada). La ventana no tiene binding para detenerla: solo el profesor. |
| `aula:restrictions` | `policy.State` | Empiezan, cambian o terminan las restricciones de la clase. Llega **antes** de aplicarlas. `{apps, web}`: cada uno `null` o `{policyId, mode, items (≤ 15), total}`. |
| `aula:app-closed` | `AppClosed` | El agente cerró una app bloqueada: `{app: "steam.exe"}`, como mucho uno por app cada 30 s. |
| `aula:compact` | `Compact` | Go sacó la ventana del modo compacto porque el profesor hizo algo que el alumno debe ver (energía, restricciones, transmisión): `{on: false}`. |
| `aula:power` | `PowerView` | Llega, se pospone, se ejecuta, falla o se anula una orden de energía (`power.*`). Llega **antes** de que la consola reciba el `power.ack`. `notice` es `null` cuando no hay aviso. |
| `aula:participation` | `participation.DeskView` | La mano del alumno sube o baja (también si la baja el profesor: `hand.lower`), o uno de sus mensajes se envía, llega a la consola (`msg.student.ack`) o se rechaza (docs/protocol.md §5.18). Con `notify`, nunca `raise`. |
| `aula:timer` | `classtimer.View` = `{state, timerId, label, description, remainingMs, justEnded}` (`description`: lo que hay que hacer, para la tarjeta `#tarea`; vacía, sin tarjeta) | El temporizador del profesor empieza, se extiende, se detiene, termina o se quita (docs/protocol.md §5.19). `state`: `none`, `running` o `ended`. La ventana cuenta los segundos por su cuenta desde `remainingMs` (lo que faltaba al emitirlo) con `performance.now()`, sin la hora del equipo, y arma el texto de la insignia de la tira y del modo compacto. `justEnded` viene solo en el evento del fin: la ventana muestra el aviso de la esquina una vez. Al terminar Go llama a `notify`, nunca a `raise`. |
| `aula:files` | `Files` = `{received, folder}` | Llega un archivo del profesor, guardado o no (`file.push`, docs/protocol.md §5.20). `received` es la lista de envíos, lo más nuevo primero; cada uno `filepush.Received` = `{pushId, label, files, folder, items, at}` y cada `item` `{name, path, renamed, replaced, reason…}`. `folder` (`"Aula"`) es cómo se llama la carpeta de trabajo dentro de Documentos: la ruta absoluta nunca llega a la ventana. La ventana arma la tarjeta (`notify`, nunca `raise`). |
| `aula:collections` | `Collections` = `{list, folder}` | Una recolección del profesor se abre, se pide o termina una ronda (`file.collect.*`, docs/protocol.md §5.21). `list` es `[collect.Collection]`, lo más viejo primero: `{id, date, activity, folder, patterns, openedAt, state, rounds}`; `folder` es la carpeta de la actividad **relativa** a la carpeta de trabajo (`recolecciones/2026-11-06/Ejercicio 4`); `state` `open`, `requested` o `collected`; cada ronda `{round, at, doneAt, files: [{path, size, saved, reason}], skipped: [{path, size, reason}], failed}` (`doneAt` en cero mientras se manda). El `folder` de afuera (`"Aula"`) nombra la carpeta de trabajo. Los avisos de antes y después de leer llegan en `aula:notices` con `kind` `collect-requested` y `collect-done` y `ref` = el `id` de la recolección; la ventana toma de aquí la carpeta y lo recogido (`notify`, nunca `raise`). |
| `aula:polls` | `poll.BallotView` = `{cards, arrived}` | Llega una encuesta del profesor, se cierra, llegan sus resultados o cambia el estado de la respuesta del alumno (`poll.*`, docs/protocol.md §5.23). `cards` es `[poll.Card]`, lo más nuevo primero (hasta 5): `{pollId, kind, question, options, named, open, choice, text, vote, results, at}`; `kind` `choice`, `yesno` u `open` (con `yesno`, `options` va vacío y la ventana pone «Sí» y «No»); `choice` es la respuesta enviada (−1 si ninguna o si es texto); `vote` `none`, `sending`, `accepted`, `late` (llegó cerrada), `unknown`, `unsupported` o `refused`; `results` `null` o `{counts, voters, recipients}`. `arrived` solo viene en el evento que anuncia una encuesta nueva: la ventana la marca «Nuevo» una vez. Go llama a `notify`, nunca a `raise` (privacidad, regla 71). `View().polls` trae lo mismo. |
| `aula:opened` | `Opened` = `{list}` | El profesor abrió una página o una app en este equipo (`open.*`, docs/protocol.md §5.22). `list` es `[openapp.Opened]`, lo más viejo primero (hasta 20): `{id, kind, shown, full, teacher, channel, at}`; `kind` `url` o `app`; `shown` es el dominio y la ruta de la URL sin la consulta (o el nombre de la app) y `full` la URL completa, para el tooltip. El aviso en la esquina llega en `aula:notices` con `kind` `opened` y `ref` = el `id`; la ventana arma el texto («{profesor} (canal {canal}) abrió {shown} en tu navegador») con estos datos. Solo `notify`, nunca `raise`: el navegador o la app ya tomaron el foco (privacidad, regla 67). `View().opened` trae lo mismo. |

```jsonc
// Status
{ "state": "connected",            // unconfigured | searching | connected | disconnected
  "teacher": "Lab 3 — Prof. Ayala",  // console_name; vacío mientras busca
  "channel": "1",
  "detail": "",                      // motivo técnico si no conecta («Detalles para TI»)
  "cause": "",                       // por qué no conecta, clasificado (session.Cause): noConsole,
                                     // unreachable, certMismatch, wrongKey, authIncomplete
  "viewed": false,                   // true mientras el profesor recibe esta pantalla
  "watching": false,                 // true mientras esta pantalla muestra la del profesor
  "restricted": false,               // true mientras hay restricciones de la clase
  "control": "" }                    // "active" o "paused" mientras el profesor controla este equipo

// BroadcastView: la transmisión de la pantalla del profesor
{ "active": true, "broadcastId": "7d1f…", "width": 1920, "height": 1080, "fps": 15,
  "teacher": "Lab 3 — Prof. Ayala", "channel": "1",
  "mode": "fullscreen" }             // "window" o "fullscreen" (v0.15); ausente: una consola anterior

// Signal: para el RTCPeerConnection de la interfaz; un attempt mayor reemplaza la conexión
{ "kind": "offer", "broadcastId": "7d1f…", "attempt": 1, "sdp": "v=0…" }
{ "kind": "ice", "broadcastId": "7d1f…", "attempt": 1,
  "candidate": "candidate:1 1 udp 2130706431 192.168.1.20 47831 typ host", "sdpMid": "0", "sdpMLineIndex": 0 }

// Board: lista completa, la más antigua primero (máx. 20 con "append")
{ "snippets": [ { "id": "0c9a…", "exercise": "3", "title": "Factorial", "lang": "python",
                  "content": "def f(n):\n    …", "receivedAt": "2026-10-28T10:00:00-06:00" } ] }

// PowerView: el aviso de energía (docs/privacidad.md reglas 26-28)
{ "notice": { "action_id": "c2e1…", "action": "reboot",     // shutdown | reboot | logoff
              "state": "countdown",                          // countdown | executing | failed
              "remaining_s": 30, "postpones_left": 2, "force": false },
  "teacher": "Lab 3 — Prof. Ayala", "channel": "1" }        // quién dio la orden

// participation.DeskView: la mano del alumno y sus mensajes al profesor. Go manda
// estados; la ventana arma «Tu mano está levantada…», «Tu profesor bajó tu mano.»,
// «Enviando…», «Recibido» o «No se envió».
{ "hand": "up",                      // down | up | lowered_by_teacher
  "note": "No me compila el for",    // el motivo de la mano levantada, si lo hay
  "messages": [ { "messageId": "8a2e…", "text": "¿Es para hoy?", "at": "…",
                  "status": "received" } ],   // sending | received | failed; el más antiguo primero (máx. 20)
  "unsupported": false }             // la consola no conoce estos mensajes (una versión anterior)

// Notices: avisos no cerrados, el más antiguo primero (máx. 10). Los del profesor
// (msg.show) traen su texto; los del propio agente traen kind y ningún texto, y la
// ventana los arma: "snapshot" = «Captura de pantalla · <profesor> (canal X) tomó una
// captura de tu pantalla (HH:MM)» (docs/privacidad.md regla 44). Cuentan como «sin ver».
// "replyTo" (msg.show con reply_to): la respuesta del profesor a un mensaje del alumno;
// la ventana la titula «Respuesta a tu mensaje» y cita el mensaje de DeskView.messages.
// "personal" (msg.show con personal): solo le llegó a este alumno; sin replyTo, la ventana
// lo titula «Mensaje para ti» con el icono «sobre» (prompt 10.7).
{ "notices": [ { "id": "5b0e…", "text": "Guarden su trabajo", "level": "warning", "at": "…" },
               { "id": "9a3c…", "text": "", "level": "warning", "at": "…", "kind": "snapshot" },
               { "id": "1f2e…", "text": "Sí, antes de las 12.", "level": "info", "at": "…", "replyTo": "8a2e…", "personal": true },
               { "id": "7c4d…", "text": "Revisa la línea 12", "level": "info", "at": "…", "personal": true } ] }
```

Cada evento lleva la lista **completa**, no un cambio incremental: la interfaz redibuja sin
llevar estado propio.

### Métodos enlazados (interfaz → Go)

| Método | Devuelve | Uso |
|---|---|---|
| `App.View()` | `View` = `{status, snippets, notices, broadcast, restrictions, power, participation, timer, files, collections, opened, polls}` | Estado completo para el primer dibujo. |
| `App.Copy(snippetId)` | error (promesa rechazada con texto en español) | Copia el fragmento al portapapeles del sistema. |
| `App.DismissNotice(noticeId)` | — | Cierra un aviso; emite `aula:notices`. |
| `App.ReportMedia(webrtc, h264)` | — | Al cargar: si hay `RTCPeerConnection` y decodificador H.264. Go espera este aviso (máx. 5 s) antes de conectarse y solo entonces declara `screen.broadcast`. |
| `App.BroadcastAnswer(broadcastId, attempt, sdp)` | error | La respuesta SDP de la interfaz. Go comprueba que sea solo recepción de video y la envía con el `ref_id` de la oferta. |
| `App.BroadcastCandidate(broadcastId, attempt, candidate, sdpMid, sdpMLineIndex)` | error | Un candidato ICE local; `""` cierra la lista. |
| `App.BroadcastFailed(broadcastId, attempt)` | error | La interfaz no pudo abrir la conexión: Go responde `error` `internal` a la oferta y la consola reintenta. |
| `App.PowerPostpone()` | error | "Posponer 5 min": suma 5 minutos, como máximo 2 veces; emite `aula:power` y avisa a la consola. |
| `App.DismissPower()` | — | Cierra el aviso de una orden de energía que falló. |
| `App.SetCompact(on)` | error | Modo compacto: encoge la ventana a 380×112 en la esquina inferior derecha de su monitor, sin activarla ni dejarla encima, o la devuelve a su lugar. Actúa sobre la ventana de este proceso (también en el ayudante del servicio: no se reenvía). |
| `App.ReportWindow(title)` | — | El título que escribió la página, para el texto del icono del área de notificación. Local, como `SetCompact`. |
| `App.RaiseHand(note)` | `participation.DeskView` / error | **Levantar la mano** (botón de la tira, también en el modo compacto), con un motivo opcional (≤ 140). Con la mano arriba, solo cambia el motivo: conserva su lugar. En modo servicio se reenvía al servicio, que tiene la conexión. |
| `App.LowerHand()` | `participation.DeskView` / error | **Bajar la mano**: `hand.lower {by: student}`. Se puede aun sin conexión. |
| `App.SendTeacherMessage(text)` | `participation.DeskView` / error | **Mensaje al profesor** (la tarjeta de arriba de la lista: Enter envía, Esc cancela): `msg.student`, 1–500 caracteres, uno cada 3 s y como mucho 5 sin acuse. Si se corta la red, se reenvía al reconectar con el mismo `message_id`. |
| `App.OpenReceivedFolder(pushId)` | error | **Abrir carpeta** de la tarjeta de un archivo recibido: abre en el Explorador la carpeta de ese envío, dentro de la carpeta de trabajo. La ventana solo pasa el `pushId`; Go decide la carpeta. En modo servicio se reenvía al servicio, que la abre por el ayudante (`folder.open`). |
| `App.OpenCollectionFolder(id)` | error | **Abrir carpeta** de la tarjeta de una recolección: abre en el Explorador la carpeta de esa actividad (`Documentos\Aula\recolecciones\<fecha>\<actividad>`). La ventana solo pasa el `id`; Go decide la carpeta y comprueba que esté bajo `recolecciones`. En modo servicio, por el ayudante (`folder.open`). |

| `App.AnswerPoll(pollId, choice, text)` | `poll.BallotView` / error | **Enviar** (o **Cambiar mi respuesta**) de la tarjeta de una encuesta: `choice` desde 0 en una de opción múltiple o de sí o no, `text` (1–140) en una de respuesta corta. Cuenta la última hasta que se cierre; una por segundo. Si se corta la red, se reenvía cuando la consola vuelve a mandar la encuesta. En modo servicio se reenvía al servicio. Errores (`encuesta.error.<código>`, `apps/student/poll.go`): `noConnection`, `unknown`, `closed`, `empty`, `tooLong`, `badVote`, `tooSoon`, `unsupported`. |

Los errores de `RaiseHand`, `LowerHand` y `SendTeacherMessage` llegan como `aula:<código>` y la
ventana los dice con `mano.error.<código>` (`apps/student/participation.go`, `partErrorCodes`):
`noConnection`, `empty`, `tooLong`, `tooSoon`, `waiting`, `unsupported` y `notSent`.

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
- **Tu pantalla mostrada a la clase** (`docs/privacidad.md`, reglas 75–82): con `phase: "live"`,
  la ventana sale del modo compacto y muestra la banda «Tu pantalla se está mostrando a la clase»
  después de la de pantalla vista, sin botones; con la ventana compacta después, la insignia
  «Mostrando a la clase». La pantalla de un compañero
  llega como `aula:broadcast` con `source` (su equipo): la franja dice «Estás viendo la pantalla
  de LAB3-PC07». El `RTCPeerConnection` de las dos apps es `sistema/receptor-video.js`.
- **Transmisión del profesor** (`docs/privacidad.md`, reglas 12–14): con `aula:broadcast`
  activo, Go trae la ventana al frente (`raise`, sale del modo compacto) y la sujeta según `mode`
  con `platform.AppWindow.Hold`: `window`, encima de las demás y sin minimizar ni cerrar;
  `fullscreen`, cubriendo el monitor. Sin `mode`, la maximiza como antes. Mientras dura, cerrar no
  la oculta (excepción a la regla 2) y `SetCompact(true)` no hace nada. La interfaz oculta el
  código y el botón de modo compacto, y muestra la franja azul fija con `notice`, la nota del modo
  (`#bc-modo`) y el `<video>`. El `RTCPeerConnection` vive en la interfaz, sin
  servidores ICE, y solo recibe: nunca pide cámara, micrófono ni pantalla. Los candidatos que
  llegan antes de aplicar la oferta se guardan y se aplican después. Al terminar (`active:
  false`: detener, desconexión o consola muda) la interfaz cierra la conexión y Go deja la ventana
  como estaba (`Release`, y el modo compacto o el área de notificación). La CSP permite
  `media-src mediastream: blob:` para el video.
- **Manos y mensajes** (`docs/privacidad.md`, reglas 50–52): lo hace el alumno, así que nada
  quita el foco ni trae la ventana, y no hay banda. «Levantar la mano» lleva `aria-pressed`; la
  línea `#mano-estado` (arriba de la lista) dice que está levantada, con su motivo, o que el
  profesor la bajó. «Mensaje al profesor» abre la tarjeta `#redactar`; en el modo compacto, la
  ventana vuelve a su tamaño porque lo pidió el alumno. Sin conexión, los botones se desactivan
  y su `title` dice por qué.
- **Control remoto** (`docs/privacidad.md`, reglas 36–41): con `status.control` (`active` o
  `paused`), Go trae la ventana al frente al empezar y la interfaz sale del modo compacto, muestra
  la banda roja `#banda-control` después de la de energía (sin botón para cerrarla ni para cortar
  el control) y titula «Aula — <profesor> controla este equipo (canal N)»; en el modo compacto, la
  insignia «Controlado». Al volver a `""` muestra en la esquina «… dejó de controlar tu equipo»,
  sin tomar el foco. No hay métodos nuevos: el alumno no acepta ni corta el control. La entrada
  la inyecta Go (`internal/remote`), nunca la interfaz.
- **Aviso de energía** (`docs/privacidad.md`, reglas 26–28): con `aula:power`, Go trae la ventana
  al frente y la deja siempre encima mientras cuenta o se ejecuta. La franja roja fija dice quién
  y qué ("⏻ El profesor … va a reiniciar este equipo en 0:30. Guarda tu trabajo.") y la interfaz
  cuenta hacia atrás desde `remaining_s` con su propio reloj. "Posponer 5 min (te quedan N)" se
  deshabilita con 0. Con `executing` dice "está reiniciando este equipo"; con `failed`, "No se
  pudo …" y "Entendido".

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
agentes ──error (en vez del acuse)──▶ Deliveries.Reject + último error del alumno ──"aula:deliveries" y "aula:students"
Ctrl+Alt+Shift+C (o botón Capturar) ──▶ Go lee el portapapeles ──"aula:capture"──▶ editor (sin emitir)
interfaz ──App.OpenScreens()──▶ screen.Wall ──screen.thumb.start──▶ agentes con screen.thumb
agentes ──screen.thumb.frame──▶ screen.Wall (solo el start vigente) ──"aula:frame"──▶ <img> de la cuadrícula
interfaz ──App.ViewScreen(id)──▶ nuevo start a ese agente (1280 px); ViewScreen("") vuelve a 320 px
interfaz ──App.CloseScreens()──▶ screen.thumb.stop a todos; Go olvida los cuadros
interfaz ──App.StartBroadcast(monitor, modo, equipos)──▶ excluye la consola de la captura ──▶ broadcast.Broadcaster
          ──screen.broadcast.start {mode}/offer/ice──▶ agentes con screen.broadcast (todos o esos equipos) ──answer/ice──▶ Go
broadcast.Broadcaster (receptores conectados) ──"aula:broadcast"──▶ franja roja "en pantalla completa a M de N alumnos"
interfaz ──App.StopBroadcast()──▶ screen.broadcast.stop a todos; la consola vuelve a las capturas
```

### Eventos (Go → interfaz)

| Evento | Datos | Cuándo |
|---|---|---|
| `aula:students` | `Students` | Un agente entra, sale, se queda mudo, confirma código, responde a una orden de energía (`power`) o manda un error del protocolo (`lastError`). |
| `aula:history` | `History` | Tras cada emisión o reemisión. |
| `aula:deliveries` | `Deliveries` | Tras cada envío, cada acuse (`code.ack`, `msg.ack`) y cada error con que un alumno contesta a un envío (`rejected`). |
| `aula:capture` | `Capture` | Al pulsar el atajo global o «Pegar». |
| `aula:class` | `View` | Al terminar el arranque automático en el canal predeterminado. Si falló, `active` es `false` y el motivo está en `Setup().startError`. |
| `aula:frame` | `Thumb` | Por cada cuadro recibido mientras la vista de pantallas está abierta (≤ 1 por segundo y alumno). Solo cuadros del `screen.thumb.start` vigente. |
| `aula:broadcast` | `BroadcastState` | La transmisión de tu pantalla empieza, termina o cambia su número de receptores. |
| `aula:present` | `PresentState` | La proyección de un alumno empieza, cambia de fase (`connecting`, `live`) o de receptores, o termina; al terminar, `ended` dice cómo (`stopped`, `failed` con `reason`, `source_left`). |
| `aula:present-view` | `broadcast.View` | La vista propia de la consola (la Pantalla grande) empieza o termina de recibir el video (`active`, `source`). |
| `aula:present-signal` | `broadcast.Signal` | Oferta o candidato ICE para el receptor de la consola (`sistema/receptor-video.js`). |
| `aula:restrictions` | `RestrictionsView` | Cambian las restricciones de la clase: `{apps, web}`, cada uno `null` o `{policyId, mode, items, hosts}` (`hosts` vacío: todo el canal). Los acuses de cada alumno llegan en `aula:students` (`appsActive`, `webActive`). |
| `aula:simulation` | `SimulationView` | El aula simulada se abre o se cierra, o uno de sus alumnos cambia (conecta, recibe código o un aviso, se ve su pantalla, cambia de falla). Sus sesiones también llegan en `aula:students` con `simulated: true`. |
| `aula:connectcheck` | `connectcheck.Report` | Mientras corre `CheckConnection()`, cada vez que un paso empieza o termina; el último trae `done: true`. |
| `aula:remote` | `remote.Control` | El control remoto empieza (`starting`), el agente muestra su banda (`active`), se pausa porque el alumno usa su equipo (`paused`), sigue (`active`) o termina: `ended` con `reason` `teacher`, `replaced`, `idle`, `session` o `error`; `gone` si el alumno se desconectó; `failed` si el agente no pudo mostrar su aviso. `{sessionID, hostname, controlID, status, reason, viewing}`. |
| `aula:snapshots` | `Snapshots` | Una captura de un alumno se pide, llega (`ready`), falla (`failed` con `reason` `timeout`, `capture`, `busy`, `unsupported`, `tooLarge`, `corrupt` o `gone`), se guarda (`path`) o se descarta. Lista completa `{snapshots: [{id, sessionId, hostname, username, simulated, status, reason, width, height, size, requestedAt, takenAt, path}]}`, **sin la imagen** (`SnapshotImage`). |
| `aula:participation` | `Participation` | Un alumno (real o simulado) levanta o baja la mano, llega o se responde un mensaje, el profesor baja una mano o marca un mensaje como atendido, o la clase se cierra (docs/protocol.md §5.18). `arrival` solo viene en el evento que anuncia una mano o un mensaje recién llegado, para el aviso flotante (sin el texto). Las insignias de cada alumno llegan también en `aula:students` (`hand`, `newMessages`). |
| `aula:timer` | `classtimer.Snapshot` | El temporizador de la clase empieza, se extiende, se detiene, llega a cero, un alumno lo confirma (`timer.ack`) o se reenvía a quien se reconecta (docs/protocol.md §5.19). `{active, timerId, label, description, durationS, remainingMs, ended, all, hosts, maxExtendS, receivers, eligible, unsupported}`: `remainingMs` es lo que faltaba al emitirlo (la interfaz cuenta con `performance.now()`); `receivers` = `[{sessionId, hostname, username, simulated, status}]` con `status` `sent` (sin confirmar), `running` o `ended`; `unsupported`, los equipos destinatarios que no declararon `timer`. La franja y la sección de Participación se dibujan con él. |
| `aula:files` | `FilesView` | Se guarda una plantilla o se envían archivos (el historial cambia). Los acuses de cada alumno van en `aula:deliveries`. |
| `aula:collections` | `CollectionsView` | Una recolección se abre, un alumno acusa su carpeta, se pide una ronda, avanza o termina la de un alumno, o la clase se cierra (docs/protocol.md §5.21). `{list, folder, folderError, today, limits}`: `list` es `[collect.Summary]`, lo más nuevo primero, cada una `{id, date, activity, patterns, limits, openedAt, round, collecting, dir, students}` y cada alumno `{session, hostname, username, simulated, key, status, reason, round, at, files: [{path, size, sha256, modified_at}], failedFiles, skipped: [{path, size, reason}], dir}`. `status`: `waiting`, `ready`, `queued`, `collecting`, `collected`, `empty` o `failed` (`reason`: un motivo de §5.21, `timeout`, `gone`, `save` o un código de error del protocolo). `folder` es la carpeta de todas las recolecciones del profesor. Sin nombres de archivos en los registros (regla 65). |
| `aula:polls` | `poll.View` | Se lanza, se cierra o se comparte una encuesta, un alumno (real o simulado) responde o la confirma (`poll.ack`), o la clase se cierra (docs/protocol.md §5.23). `{current, closed}`: `current` es la abierta o `null`; `closed`, las cerradas de la clase, lo más nuevo primero. Cada una `poll.PollView` = `{pollId, kind, question, named, open, shared, shareable, all, hosts, openedAt, closedAt, options, answered, answeredSimulated, received, eligible, unsupported, answers, missing}`: `options` = `[{label, count, simulated}]` (`label` vacío en sí o no); `answers` = `[{hostname, username, simulated, choice, text, at}]` solo si es nominal, o en una anónima de respuesta corta los textos **sin** equipo ni hora; `missing`, quién no ha respondido (solo nominal y abierta). La franja y la sección **Encuestas** de Participación se dibujan con él. |
| `aula:opens` | `[openapp.Order]` | Se abre una página o una app, o un alumno lo acusa (`open.ack`) o responde con un error (docs/protocol.md §5.22). Las últimas 5, lo más nuevo primero: `{id, kind, label, at, targets: [{sessionId, hostname, status, reason}], unsupported: [hostname], failed: [hostname], opened, pending, refused}`. `kind` `url` o `app`; `label` es la URL o el nombre de la app (solo para el profesor; los registros nunca los llevan); `status` `sent`, `opened`, `failed` (con `reason` `not_allowed`, `not_installed`, `launch_failed` o `busy`) o `error`. Se vacía al abrir otra clase. |
| `aula:allowed-apps` | `AllowedApps` | Se guardó la lista de Ajustes › Apps permitidas. `{apps: [{id, name, path}], source, path}`: `source` `centro` (`C:\ProgramData\Aula\centro\apps-permitidas.json`, de todas las consolas), `consola` (la carpeta de esta consola) o `ejemplos` (aún sin archivo: los ejemplos de Aula); `path` es dónde se guarda. |
| `aula:remote-frame` | `Thumb` | Cada imagen de la pantalla controlada (hasta 8 por segundo con la vista abierta, 1 con la vista cerrada). No pasa por `aula:frame` ni se guarda. |

```jsonc
// Students: la lista del canal y los conteos que habilitan los botones
{ "students": [ { "id": "…", "hostname": "LAB3-PC07", "username": "alumno07", "os": "windows",
                  "osVersion": "…", "agentVersion": "0.1.0", "state": "connected",
                  "capabilities": ["code.view", "msg.show", "screen.thumb"], "canCode": true,
                  "canNotice": true, "canThumb": true, "canBroadcast": true,
                  "canPower": true,
                  "power": { "action_id": "c2e1…", "action": "reboot", "force": false,
                             "status": "postponed",   // sent | countdown | postponed | executing | cancelled | error
                             "deadline_ms": 1792771530000, "postpones_left": 1 },  // null sin orden
                  "lastCodeAck": "0c9a…", "ackedAt": "10:00:05",
                  "connectedAt": "09:58",          // hora en que abrió esta sesión
                  "lastError": { "code": "unsupported_capability", "at": "10:03:12" },  // null si no ha mandado ninguno
                  "canHand": true, "canMessage": true,   // hand y msg.student
                  "hand": 2,                       // su lugar en la fila de manos (0: abajo): «Mano levantada · 2»
                  "newMessages": 1,                // sus mensajes sin responder: «1 mensaje»
                  "simulated": false } ],          // true: alumno del aula simulada, siempre con su insignia
  "connected": 1,                      // solo alumnos reales
  "simulated": 0,                      // alumnos simulados conectados, aparte
  "canCode": 1, "canNotice": 1, "canThumb": 1,  // incluyen a los simulados
  "canBroadcast": 1, "canPower": 1 }   // solo reales: los simulados nunca los reciben

// History: fragmentos de la sesión, numerados ("Ejercicio N")
{ "entries": [ { "snippetId": "…", "number": 1, "title": "Factorial", "lang": "python",
                 "content": "…", "times": 2, "firstSentAt": "…", "lastSentAt": "…" } ] }

// Deliveries: envíos recientes, el más nuevo primero
{ "deliveries": [ { "id": "d3", "kind": "code", "label": "Ejercicio 1 · Factorial",
                    "sent": 2, "acked": 2, "failed": 0, "noCapability": 1,
                    "simSent": 2, "simAcked": 1,   // alumnos simulados, contados aparte
                    "pending": ["…"],              // sesiones que aún no lo confirman (reales y simuladas)
                    "rejected": [ { "session": "…", "code": "internal" } ],  // contestaron con un error
                    "at": "…" } ] }

// SimulationView: el aula simulada
{ "active": true, "max": 40, "capabilities": ["code.view", "msg.show", "screen.thumb"],
  "students": [ { "id": "…", "name": "Simulado 1", "os": "windows",
                  "capabilities": ["code.view", "msg.show", "screen.thumb"],
                  "connected": true, "fault": "none",   // none | slow | noack | disconnected
                  "delayMs": 0,                         // solo con slow
                  "viewed": false,                      // el profesor está viendo su pantalla
                  "snippets": [ { "exercise": "1", "title": "Factorial", "lang": "python", "lines": 6 } ],
                  "notices": 1, "lastNotice": "Guarden su trabajo",
                  "hand": "down",                       // down | up | lowered_by_teacher (participation.HandState)
                  "sent": 0 } ] }                       // mensajes que escribió al profesor

// Participation: la vista «Participación». Los alumnos se identifican por equipo,
// usuario y simulated (sobreviven a una reconexión); sessionId es la sesión vigente,
// "" si está desconectado (no se le puede responder).
{ "hands": [ { "hostname": "LAB3-PC12", "username": "alumno12", "simulated": false,
               "handId": "3f1c…", "note": "No me compila el for", "raisedAt": "…",
               "position": 1, "sessionId": "…" } ],             // en orden de llegada
  "messages": [ { "hostname": "lab-ubuntu-02", "username": "maria", "simulated": false,
                  "messageId": "8a2e…", "text": "¿Es para hoy?", "at": "…",
                  "status": "new",                               // new | replied | done
                  "reply": "", "repliedAt": "…", "sessionId": "…" } ],  // el más nuevo primero
  "pending": 2,                       // manos + mensajes nuevos: el número del menú
  "arrival": { "kind": "hand", "hostname": "LAB3-PC12", "username": "alumno12", "simulated": false } }  // o null

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

### Errores de los métodos

Un método que falla devuelve un error con **código estable**, que llega a la interfaz como
`aula:<código>: <detalle>` (por ejemplo, `aula:portInUse: el puerto 47830 está ocupado…`). La
interfaz muestra `error.<código>.titulo` y `error.<código>.texto` de `textos.es-MX.js` («qué
pasó + qué hacer») y deja el detalle en «Ver detalles». Los códigos están en
`apps/teacher/errores.go`. Las pruebas exigen que cada código tenga su texto y que los métodos
devuelvan el código correcto en sus fallos típicos. Un error sin código se muestra tal cual, como
último recurso.

Los errores que **mandan los alumnos** (mensaje `error` del protocolo, `docs/protocol.md` §2.1)
no los devuelve un método: llegan en `aula:students` (`lastError`) y, si contestan a un envío, en
`aula:deliveries` (`rejected`). La interfaz los explica con `errorAlumno.<código>` (en camelCase:
`unsupported_capability` → `errorAlumno.unsupportedCapability`) y los enlaza con
`Manual().agentErrors` (`agentErrorHelp` en `apps/teacher/agenterrors.go`).
`TestAgentErrorCodesLinkToTheManual` lee los códigos de `internal/protocol/errors.go` y exige su
texto y su sección.

Cada código enlaza además a su sección del manual (`errorHelp` en `errores.go`, que llega en
`Manual().errors`): el aviso lleva «Ver en el manual». `TestErrorCodesLinkToTheManual` exige que
cada código tenga su enlace y que la sección exista.

### Métodos enlazados (interfaz → Go)

| Método | Devuelve | Uso |
|---|---|---|
| `Setup()` | `Setup` | Canal predeterminado (`defaultChannel`), canales recientes, nombre, huella, origen de la clave (`keySource`: `centro` o `personal`) y `startError` si el arranque automático falló. |
| `StartClass(canal, nombre)` | `View` / error | Abre la clase en el canal (número 1–16000); si ya había una abierta, la cierra antes («Cambiar de canal» y «Cambiar tu nombre», con su confirmación). **El historial de código y los envíos se conservan** (son del profesor, no del canal). Usa la clave del centro o, si no hay, la personal. |
| `EndClass()` | — | Cierra la clase. La interfaz no lo llama directamente. |
| `View()` | `View` | Estado completo para el primer dibujo. `class` = `{channel, consoleName, publicKey, fingerprint, keySource, port, certExpires}`; `certExpires` (`AAAA-MM-DD`) es el vencimiento del certificado de la conexión, solo informativo: los alumnos fijan su huella y no revisan fechas. `hotkey` = `{combo, active, reason, error}`: si el atajo de captura no está activo, `reason` dice por qué (`InUse`: otro programa lo usa; `NotSupported`: el sistema no lo permite) y la interfaz muestra `ajustes.atajo<reason>`; con otra falla, `reason` va vacío y `error` trae el detalle técnico. |
| `Emit({title, lang, content, mode, targets})` | `Delivery` / error | `mode` = `replace` («Emitir») o `append` («Agregar debajo»). `targets`: IDs de sesión (reales o simulados) de la selección; vacío = todos. Crea una entrada nueva en el historial. |
| `Reemit(snippetId)` | `Delivery` / error | Reenvía un fragmento del historial con el mismo número e ID (`replace`). |
| `ClearScreens()` | `Delivery` / error | `code.clear` a todos. |
| `SendNotice(text, level)` | `Delivery` / error | `msg.show` a todos, `info` o `warning`. |
| `SendNoticeTo(text, level, targets)` | `Delivery` / error | Igual, solo a `targets` (vacío = todos). Lo usa el panel **Aviso**. |
| `SendPrivateNotice(sessionId, text, level)` | `Delivery` / error | **Mensaje privado** de la ficha y de la pantalla en grande (también en modo control: antes se sueltan las teclas presionadas y lo escrito en el campo no sale por `SendRemoteInput`): `msg.show` con `personal` solo a esa sesión (real o simulada). Su etiqueta es «Mensaje privado a <equipo>», sin el texto (`docs/privacidad.md` regla 53); el acuse llega en `aula:deliveries`. `studentGone` si no está conectado, `noticeUnsupported` si su equipo no declaró `msg.show`, `invalidNotice` si el texto no vale. |
| `Resend(deliveryId)` | `Delivery` / error | **Reenviar a los que faltan**: manda de nuevo el mismo mensaje a las sesiones de `pending` y de `rejected` que siguen conectadas (las de `rejected` vuelven a `pending`). Los acuses tardíos del primer envío ya no cuentan; los nuevos, una vez. |
| `Timer()` | `classtimer.Snapshot` | El temporizador de la clase (también en `View().timer`). |
| `StartTimer({label, description, durationS, hosts})` | `classtimer.Snapshot` / error | **Iniciar** del panel **Temporizador**: reemplaza al que corre. `hosts`: los equipos de la selección (reales o simulados); vacío = toda la clase, también quien se conecte después. `label` ≤ 40, `description` ≤ 1000 (opcional, nota de interfaz 18), `durationS` 10 s–4 h. `noTimerTargets` si ninguno de los destinatarios conectados declaró `timer`; `invalidTimer` si el nombre o la duración no valen. El nombre y la descripción no se registran (`docs/privacidad.md` regla 54). |
| `ExtendTimer(addS)` | `classtimer.Snapshot` / error | **Extender** (+1, +5 o +10 min) de la franja y de Participación: `timer.extend` con lo que falta ya extendido; revive uno terminado. `timerGone` si no hay; `timerTooLong` si pasaría de 4 h; `invalidTimer` fuera de 1–60 min. |
| `StopTimer()` | `classtimer.Snapshot` / error | **Detener** (o **Quitar**, ya terminado): `timer.stop` a quien lo tenía. `timerGone` si no hay. |
| `Polls()` | `poll.View` | Las encuestas de la clase (también en `View().polls`). |
| `OpenPoll({kind, question, options, named, hosts})` | `poll.View` / error | **Lanzar** del panel **Encuesta**: cierra la abierta y manda `poll.open` a quien declaró `poll` (reales y simulados). `kind` `choice` (2–6 opciones ≤ 80), `yesno` u `open`; `question` ≤ 200; `named` = «Ver quién respondió qué». `hosts`: la selección; vacío = toda la clase, también quien se conecte después. `noPollTargets` si ninguno puede; `invalidPoll` si la pregunta o las opciones no valen. El texto no se registra (`docs/privacidad.md` regla 74). |
| `ClosePoll()` | `poll.View` / error | **Cerrar encuesta** de la franja y de Participación: `poll.close` a quien la tenía; las respuestas que lleguen después se acusan `closed`. `pollGone` si no hay una abierta. |
| `SharePollResults(pollId)` | `poll.View` / error | **Compartir resultados** de una cerrada de opción múltiple o de sí o no: `poll.results` con los totales de los alumnos reales a quienes la recibieron (también a quien se reconecta). `pollGone`, `pollNotShareable`. |
| `Files()` | `FilesView` = `{templates, templatesError, sends}` | La vista **Archivos** del menú: las plantillas de `Documentos\Aula\plantillas` (`filepush.Template` = `{name, files: [{rel, size}], size, problem}`; `problem` dice por qué no se puede enviar: `empty`, `too_many`, `too_large`, `bad_name`) y los envíos del historial (`FileSend` = `{id, label, template, names, size, dest, overwrite, times, lastAt}`). |
| `ChooseFiles(title)` | `[FileChoice]` = `[{path, rel, size}]` / error | **Elegir archivos…**: el diálogo del sistema, con el título que pasa la interfaz (de `textos.es-MX.js`). Cancelar devuelve `[]`. |
| `SendFiles({files, template, dest, overwrite, targets})` | `Delivery` / error | **Enviar**: `file.push` a quien declaró `file.push` (reales y simulados), o solo a `targets` (sesiones). `files` = `[{path, rel}]`, o `template` (sus archivos se leen al enviar). `dest`: carpeta dentro de `Documentos\Aula` del alumno. Se cuenta en **Últimos envíos** con `kind: "file"`: un alumno lo tiene cuando guardó todos los archivos; si no, queda en `rejected` con el motivo (`no_space`, `timeout`, `gone`…). Errores: `pushNoFiles`, `pushTooLarge`, `pushPathInvalid`, `pushFileGone`, `pushUnsupported`, `templateGone`. Los nombres no se registran (`docs/privacidad.md` regla 59). |
| `ResendFiles(sendId)` | `Delivery` / error | **Reenviar** de «Archivos enviados» en el Historial: a todos, leyendo los archivos otra vez. `Resend(deliveryId)` de un envío de archivos hace lo mismo solo con los que faltan. |
| `SaveTemplate(name, files)` | `FilesView` / error | **Guardar como plantilla…**: copia los archivos a `Documentos\Aula\plantillas\<name>`. `templateExists`, `templateInvalid`. Emite `aula:files`. |
| `OpenTemplates(name)` | error | Abre en el Explorador la carpeta de la plantilla `name`, o la de todas (`""`, se crea si falta). |
| `DeleteTemplate(name)` | `FilesView` / error | **Eliminar** (la papelera de su fila, tras confirmar; nota de interfaz 16): borra solo la carpeta `Documentos\Aula\plantillas\<name>`, nunca algo fuera ni un enlace. Lo ya enviado sigue en el historial. `templateGone`, `templateInvalid`, `templateNotDeleted` (un archivo abierto en otro programa). Emite `aula:files`. |
| `Collections()` | `CollectionsView` | La pestaña **Recolecciones** de Archivos. |
| `OpenCollection({activity, patterns})` | `collect.Summary` / error | **Crear carpeta en los equipos**: con la fecha de hoy, manda `file.collect.open` a quien declaró `file.collect` (reales y simulados) y crea `Documentos\Aula\recolecciones\<fecha>\<actividad>` del profesor. `patterns` como lo escribió el profesor (`*.py, *.txt`). Quien se conecta después recibe la carpeta al llegar. Errores: `collectActivity`, `collectFolder`. |
| `Collect(id, keys)` | `collect.Summary` / error | **Recoger** (`keys` vacío: todos los conectados que pueden), **Recoger a los que faltan** o **Recoger otra vez** de un alumno (`keys` = sus `key`). Pide una ronda nueva, de 4 en 4; el avance llega en `aula:collections`. Errores: `collectGone`, `collectNoStudents`. |
| `OpenCollectionFolder(id, key)` | error | Abre en el Explorador la carpeta del profesor de un alumno (`key`), de la actividad (`key` vacío) o la de todas las recolecciones (`id` vacío, se crea si falta). `collectGone`, `collectFolder`. |
| `SaveCollectionAs(id, key, title)` | `string` / error | **Guardar como…** de un alumno: abre el diálogo de carpeta del sistema (`title`) y copia su carpeta allí sin reemplazar nada (`… (2)`). Devuelve la carpeta nueva, o `""` si se canceló. `collectNothing`, `collectGone`, `collectFolder`. |
| `InstallerInfo()` | `InstallerInfo` | Datos para quien instala: `{channel, consoleName, hostname, addresses, port, mediaPort, fingerprint, keySource, version}`. |
| `CaptureClipboard()` | `Draft` / error | Igual que el atajo global; el resultado también llega como `aula:capture`. |
| `OpenScreens()` | error | Abre la vista de pantallas (botón **Ver pantallas**): envía `screen.thumb.start` (1000 ms, calidad 50, 320 px) a cada alumno conectado con `canThumb`, y también a los que se conecten mientras siga abierta. Error si no hay clase. |
| `CloseScreens()` | — | Cierra la vista (**Cerrar** o Esc): envía `screen.thumb.stop` a todos los que transmitían y olvida los cuadros. |
| `ViewScreen(sessionId)` | — | Vista en detalle de un alumno (clic en su miniatura): nuevo `screen.thumb.start` con 1280 px y calidad 70 solo a él. `ViewScreen("")` lo devuelve a miniatura. |
| `Displays()` | `DisplayInfo[]` | Monitores para el selector de la transmisión. |
| `StartBroadcast(monitor, mode, hostnames)` | `BroadcastState` / error | **Mostrar mi pantalla.** `mode`: `"window"` o `"fullscreen"` (privacidad, regla 12). `hostnames`: los equipos seleccionados en Alumnos (también si se reconectan), o `[]` para todos, incluidos los que se conecten después. Excluye la ventana de la consola de la captura (si no puede, no transmite) y envía `screen.broadcast.start` (con `mode`) + oferta a cada uno con `canBroadcast`; los simulados nunca. `BroadcastState` suma `mode` y `targeted` (equipos elegidos; 0 = todos). Error si no hay clase, si el modo no es uno de los dos, si ya transmite o si falta el codificador H.264. |
| `StopBroadcast()` | `BroadcastState` | **Detener.** `screen.broadcast.stop` a todos; la consola vuelve a aparecer en las capturas. También ocurre al cambiar de canal o cerrar la clase. |
| `BroadcastState()` | `BroadcastState` | Estado actual (también viene en `View().broadcast`). |
| `StartPresent(sessionId)` | `PresentState` / error | **Proyectar a la clase** (ficha y Pantalla grande). Detiene la transmisión del profesor si la hay (la interfaz lo confirmó) y manda `student.present.start` al alumno (no se le pregunta). Errores: `noClass`, `studentGone`, `presentUnsupported`, `presentFailed`. |
| `StopPresent()` | `PresentState` | **Detener** (franja, ficha o Pantalla grande): `student.present.stop` a la fuente y `screen.broadcast.stop` a los receptores. También al cambiar de canal o cerrar la clase. |
| `PresentState()` | `PresentState` | Estado actual (también viene en `View().present`). |
| `OpenPresentView()` / `ClosePresentView()` | — | **Ver** abre la proyección en la Pantalla grande: la consola es un receptor más (no cuenta en «N viendo»). Cerrarla no detiene la proyección. |
| `PresentViewAnswer(broadcastId, attempt, sdp)`, `PresentViewCandidate(broadcastId, attempt, candidate, mid, mline)`, `PresentViewFailed(broadcastId, attempt)` | error | El receptor de la consola, como `BroadcastAnswer`, `BroadcastCandidate` y `BroadcastFailed` del alumno. |
| `Restrictions()` | `RestrictionsView` | Restricciones de la clase (también en `View().restrictions`). |
| `SetRestriction({kind, mode, items, hosts})` | `RestrictionsView` / error | `kind` `apps` o `web`; `mode` `deny` o `allow`; `items` como los escribe el profesor (Go quita rutas, `.exe` duplicados, `https://`, `www.` y rutas de URL); `hosts` los alumnos marcados, o vacío para todo el canal. Se reenvía sola a quien se reconecta. |
| `ClearRestriction(kind)` | `RestrictionsView` | Quita una de las dos. |
| `ClearAllRestrictions()` | `RestrictionsView` | **Quitar todas las restricciones.** Cerrar la clase o cambiar de canal también las quita. |
| `PowerAction({action, hosts, delayS, force})` | `PowerResult` / error | **Energía → Apagar / Reiniciar / Cerrar sesión**, después de la confirmación explícita de la interfaz. `action` `shutdown`, `reboot` o `logoff`; `hosts` los marcados o vacío (todo el canal); `delayS` 10–600 (0 = 30 s); `force` sin cuenta regresiva. Solo a quien declaró `power`. `PowerResult` = `{sent, unsupported: [hostname], failed: [hostname]}`. No se reenvía a quien se reconecta. |
| `PowerCancel(hosts)` | `PowerResult` | **Cancelar.** `power.cancel` a quien tiene una orden en cuenta regresiva (`sent`, `countdown`, `postponed`); `hosts` vacío: todo el canal. |
| `AllowedApps()` | `AllowedApps` | Ajustes › Apps permitidas y la lista de «Abrir página o app»: la del centro, la de esta consola o los ejemplos, en ese orden. |
| `DefaultAllowedApps()` | `[openapp.App]` | **Restaurar ejemplos**: los ejemplos de Aula (Bloc de notas, Calculadora, Paint, VS Code, Thonny, Scratch 3), sin guardarlos. |
| `SaveAllowedApps(apps)` | `AllowedApps` / error | **Guardar**: revisa cada `{id, name, path}` (nombre de 1 a 60 caracteres y distinto de los demás; ruta de un `.exe` que empiece con una unidad o con `%ProgramFiles%`, `%ProgramFiles(x86)%`, `%LOCALAPPDATA%` o `%SystemRoot%`, y que no sea una herramienta del sistema), da un `id` a las nuevas y la guarda donde la leyó `AllowedApps`. `allowedAppInvalid`, `allowedAppsNotSaved` (la del centro sin permisos de administrador). Emite `aula:allowed-apps`. |
| `CheckOpenPage(url, hosts)` | `OpenCheck` / error | Mientras se escribe la dirección: `{url, filtered: [hostname]}`, la URL como se enviará (con `https://` si no tenía esquema) y los destinos cuyo filtro web de la clase la bloquea, para avisar antes de abrir (privacidad, regla 69). `invalidUrl`. |
| `OpenPage({url, hosts})` | `OpenResult` / error | **Abrir → Página web**: `open.url` a los marcados (`hosts`) o a todo el canal, solo a alumnos reales que declararon `open.url`. `OpenResult` = `{openId, sent, unsupported: [hostname], failed: [hostname]}`; los acuses llegan en `aula:opens`. `invalidUrl` (no http/https, sin sitio, con usuario y contraseña), `noOpenTargets`. |
| `OpenApp({appId, hosts})` | `OpenResult` / error | **Abrir → App permitida**: `open.app` con el nombre y la ruta de la app `appId` de la lista; nunca otra cosa. `openAppGone` (ya no está en la lista), `noOpenTargets`. |
| `OpenOrders()` | `[openapp.Order]` | Las últimas órdenes de «Abrir página o app», como en `aula:opens`. |
| `Simulation()` | `SimulationView` | Estado del aula simulada. |
| `SimulationStart(n)` | `SimulationView` / error | Abre el aula simulada con `n` alumnos (0 = 2), o agrega `n` si ya está abierta. Los alumnos simulados corren dentro de la consola y se conectan por un puerto local (`127.0.0.1`) que no se anuncia: ningún alumno real lo alcanza. Error si no hay clase. |
| `SimulationAdd({os, capabilities})` | `SimulationView` / error | Agrega un alumno simulado: `os` `windows`, `linux` o `darwin`; `capabilities` entre `code.view`, `msg.show`, `screen.thumb`, `screen.snapshot`, `hand` y `msg.student` (vacío: todas). Máximo 40. |
| `SimulationRemove(id)` | `SimulationView` / error | Quita uno. |
| `SimulationFault(id, fault, delayMs)` | `SimulationView` / error | Hace fallar a uno: `slow` (contesta con `delayMs` de retraso, 500–10 000), `noack` (muestra pero no confirma), `disconnected` (se va hasta volver a `none`). |
| `Manual()` | `ManualView` / error | El manual embebido (`docs/administracion`), ya leído y validado: `{pages, diagrams, images, errors, diagnosis}`. `pages[]` = `{slug, title, summary, group, order, screens, blocks, headings}`; los bloques no traen HTML (la interfaz los dibuja con el DOM). `diagrams[id]` = `{id, title, description, audience, screens, alt, explanation, mermaid, light, dark}`: `light` y `dark` son los SVG claro y oscuro como data URI para un `<img>` (vacíos si el dibujo no está vigente: la interfaz muestra `mermaid`), `explanation` son los bloques de «Lo que no es obvio» y `screens` las vistas cuyo «?» lo muestra. `images`: `img/x.png` → data URI. `errors`: código de error → `página#sección`; `agentErrors`, lo mismo para los códigos del protocolo que mandan los alumnos. `diagnosis`: el árbol de «¿Falta un alumno?» sacado del diagrama 12 (`{start, nodes}`). |
| `TakeControl(sessionID)` | `remote.Control` / error | **Tomar control** desde la ficha o desde la pantalla en grande de Miniaturas (que deja su vista en detalle con `ViewScreen("")` y pasa a modo control): `remote.start` a ese alumno. Solo a alumnos reales conectados que declararon `remote` (nunca a simulados): si no, `remoteUnsupported`. Uno a la vez: con otro vivo, `alreadyControlling`. Sigue con `aula:remote`. |
| `ReleaseControl()` | error | **Soltar** (en la vista y en la franja): `remote.stop`. El control termina con el `remote.ack ended` del agente o con su desconexión. `remoteGone` si no había ninguno. |
| `ShowControl(visible)` | error | La vista de control se abre (`true`) o se cierra sin soltar (`false`): ajusta el video con un `remote.start` del mismo `control_id` (125 ms y 1280 px, o 1000 ms y 640 px). Sin la vista no se manda entrada. |
| `SendRemoteInput(events)` | error | Eventos de la imagen en vivo, en lotes cada 33 ms: `{t: "move", x, y}`, `{t: "button", b, down, x, y}`, `{t: "wheel", dx, dy}`, `{t: "key", code, down}` (`docs/protocol.md` §5.16). Go descarta las teclas fuera de la lista, junta los movimientos y manda como máximo 60 `remote.input` por segundo; nada sale antes de `active` ni en pausa. |
| `TakeSnapshot(sessionID)` | `snapshot.Snapshot` / error | **Tomar captura** desde la ficha o desde la pantalla en grande (también en modo control: no lo termina ni lo pausa). `screen.snapshot.request` (calidad 90, resolución nativa) a un alumno conectado, real o simulado, que declaró `screen.snapshot`; si no, `snapshotUnsupported`. Con 64 en memoria, `snapshotTooMany`. Sigue con `aula:snapshots`. |
| `TakeSnapshots(sessionIDs)` | `SnapshotBatch` / error | El panel **Captura** de la barra de acciones: pide una a cada seleccionado que puede. `{snapshots, skipped}`: `skipped` son los equipos que no la permiten. Error `snapshotUnsupported` si nadie puede. |
| `SnapshotImage(id)` | string (Base64) / error | El JPEG de una captura `ready`, para la vista previa y las miniaturas del lote. No toca el disco. `snapshotGone` si ya no está. |
| `SaveSnapshots(ids)` | `snapshot.Snapshot[]` / error | **Guardar**: escribe las `ready` en `<Documentos>\Aula\capturas\<equipo> (<usuario>)\<AAAA-MM-DD>\<HH-MM-SS>.jpg` (los simulados, en `… (simulado)`) y las devuelve con `path`. Es lo único que escribe una captura en disco (`docs/privacidad.md` regla 45). `snapshotSaveFailed` si no se pudo escribir. |
| `DiscardSnapshots(ids)` | — | **Descartar**, cerrar la vista previa o «Descartar todas»: las olvida y libera la imagen. Un archivo ya guardado se queda. |
| `SnapshotFolder()` | `SnapshotFolder` | Para Ajustes › Acerca de: `{path, exists, error}` de la carpeta `capturas`. |
| `OpenSnapshotFolder(path)` | error | **Abrir carpeta**: la de capturas (`""`) o la de una captura guardada (su `path`). Nada fuera de `capturas`. `snapshotFolder` si aún no existe. |
| `ManualSearch(q)` | `Result[]` | Búsqueda local, sin distinguir acentos ni mayúsculas, en títulos, secciones y texto de las páginas y en los títulos y descripciones de los diagramas. Hasta 12 `{slug, anchor, diagram, title, snippet}`. |
| `CheckConnection()` | `connectcheck.Report` / error | **Comprobar la conexión** (`internal/connectcheck`): recorre desde este equipo el camino de un alumno y dice en qué paso fallaría. `{channel, steps, cause, done}`; cada paso `{id, status, cause, reason, addrs, port, others, networks}` con `id` `announce`, `port`, `secure` o `network` y `status` `pending`, `running`, `ok`, `warn`, `fail` o `unknown`. El texto de un paso es `comprobar.<id>.<reason \| cause \| status>` (`connectcheck.Outcomes` los lista, y la prueba de textos exige uno por cada). `cause` es la del primer paso que falla, con los mismos valores que `session.Cause` del agente. Tarda unos segundos; no llega a ningún alumno, ni real ni simulado (la conexión de prueba se cierra antes de `hello`). Error `noClass` sin clase y `checkRunning` si ya hay una en curso. |
| `Participation()` | `Participation` | La vista «Participación» (también en `View().participation`). |
| `LowerHand({hostname, username, simulated})` | error | **Bajar la mano**: la quita de la fila y manda `hand.lower {by: teacher}` si el alumno está conectado (si no, se entera al reconectar). `handGone` si ya estaba abajo. |
| `LowerAllHands()` | int / error | **Bajar todas** (tras confirmar): devuelve cuántas bajó. |
| `ReplyToStudent({hostname, username, simulated, messageId, text})` | `Delivery` / error | **Responder**: `msg.show` (`info`, `personal`) solo a ese alumno, con `reply_to` si `messageId` no está vacío (responder a un mensaje; vacío: a una mano). Se cuenta como cualquier aviso en `aula:deliveries` y el mensaje pasa a `replied`. `studentGone` si no está conectado, `messageGone` si el mensaje ya no está, `invalidNotice` si el texto no vale. |
| `MarkMessageDone({hostname, username, simulated}, messageId)` | error | **Marcar como atendido**: el mensaje deja de contar como pendiente, sin responderlo. |
| `SimulationHand(id, up)` | `SimulationView` / error | Hace que un alumno simulado levante (`true`) o baje la mano, como uno real. `simTooSoon` si lo repite antes de 1 s. |
| `SimulationMessage(id, text)` | `SimulationView` / error | Hace que un alumno simulado escriba al profesor (`""`: una pregunta de ejemplo). `simTooSoon` antes de 3 s. |
| `SimulationStop()` | `SimulationView` | Cierra el aula simulada: sus alumnos se van y se olvidan. También ocurre al cambiar de canal o de nombre, o al cerrar la clase. Los alumnos reales no se tocan. |

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
- **Control remoto, uno a la vez y siempre a la vista:** la vista es `#pantalla-grande` en modo
  control (la imagen captura el mouse y, con el foco, el teclado; Ctrl+Alt+Fin devuelve el
  teclado; Esc va al alumno). Mientras dura, la franja roja «Controlando LAB3-PC07 · Ver · Soltar»
  está en todas las vistas; cerrar la vista no suelta. Cerrar la clase lo suelta.
- **La transmisión de la pantalla es visible para los dos lados:** la consola muestra la franja
  roja con "Detener" mientras dura y su propia ventana no aparece en el video
  (`docs/privacidad.md`, regla 15).
- **Captura: vista previa en memoria, guardar es explícito** (`docs/privacidad.md` reglas 44–49).
  La vista previa es el diálogo `#captura`; cerrarlo descarta la captura de la memoria. En modo
  control, antes de pedirla se sueltan las teclas y botones presionados, y el teclado no vuelve
  solo a la imagen al cerrar la vista previa. El lote es el panel `#panel-captura`, con una
  miniatura por alumno y casillas para elegir cuáles guardar.
- **Participación es la vista de lo que responden los alumnos en vivo** (`#participacion`, con
  el contador `#nav-participacion-contador` en el menú). Cada función que recoge respuestas
  agrega su sección; la primera es «Manos y mensajes». Las manos van en orden de llegada y los
  mensajes del más nuevo al más viejo; «Responder» abre un campo en la tarjeta (Enter envía,
  Shift+Enter otra línea, Esc cancela) que sobrevive a los repintados. Al llegar una mano o un
  mensaje, un aviso flotante dice quién, nunca qué (`docs/privacidad.md` regla 50).
- **Los cuadros solo viven en memoria:** Go guarda el último de cada alumno mientras la vista
  está abierta y la interfaz solo los tiene en sus `<img>`. Nada se escribe en disco
  (`docs/privacidad.md`, regla 7). La CSP permite `img-src data:` para mostrarlos.
