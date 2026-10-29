# Contrato backend ↔ interfaz del cliente del alumno

La app del alumno (`apps/student`) tiene un backend Go y una interfaz web (Wails). Se comunican
por **eventos** (Go → interfaz) y **métodos enlazados** (interfaz → Go). La definición está en
`apps/student/events.go`, con los nombres de evento repetidos en
`apps/student/frontend/dist/events.js`. Una prueba (`TestEventContractMatchesFrontend`) falla si
no coinciden.

## Flujo

```
consola ──code.share──▶ agente (session.Agent)
                          │ codedist.Receiver: aplica al tablero y arma el code.ack
                          ├──code.ack──▶ consola
                          └──runtime.EventsEmit("aula:board", Board)──▶ interfaz (Prism)
interfaz ──App.Copy(id)──▶ Go ──platform.Clipboard──▶ portapapeles del sistema
```

Al cargar, la interfaz **primero se suscribe** a los eventos y **después** pide el estado
completo con `App.View()`, para no perder nada que llegue entre medio.

## Eventos (Go → interfaz)

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

## Métodos enlazados (interfaz → Go)

| Método | Devuelve | Uso |
|---|---|---|
| `App.View()` | `View` = `{status, snippets, notices}` | Estado completo para el primer dibujo. |
| `App.Copy(snippetId)` | error (promesa rechazada con texto en español) | Copia el fragmento al portapapeles del sistema. |
| `App.DismissNotice(noticeId)` | — | Cierra un aviso; emite `aula:notices`. |

`Copy` recibe solo el **ID**: el texto sale de la copia que guarda Go, no de la página. Así lo
que se pega es exactamente lo que envió el profesor, sin depender de cómo lo dibujó el
navegador.

## Reglas de la interfaz

- Todo texto recibido se inserta con `textContent`. El único `innerHTML` es la salida de
  `Prism.highlight`, que escapa el código (una prueba lo verifica).
- La página tiene CSP `script-src 'self'`: sin scripts en línea ni de otros orígenes.
- El distintivo (`banner`) siempre está visible, fijo arriba (ver `docs/privacidad.md`).
