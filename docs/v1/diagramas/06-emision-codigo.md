---
titulo: "Emisión y recepción de código"
descripcion: "Qué pasa desde que el profesor pulsa Emitir hasta que ve cuántos alumnos ya tienen el código en pantalla"
audiencia: [profesor, desarrollador]
pantallas: [console]
secciones_manual: [uso-hito1, eventos-ui]
---

# Emisión y recepción de código

¿Cómo viaja un fragmento de código del editor del profesor a la pantalla del alumno, y qué
significa el acuse?

```mermaid
%%{init: {"htmlLabels": false}}%%
sequenceDiagram
    autonumber
    actor P as Profesor
    box rgb(220,252,231) Consola
        participant UI as Panel de código (Monaco)
        participant T as App (apps/teacher)
        participant H as History y Deliveries
        participant S as session.Console
    end
    box rgb(220,252,231) Agente
        participant R as codedist.Receiver
        participant W as Ventana del alumno (Prism)
    end
    actor E as Alumno

    P->>UI: Emitir a todos (o Agregar bloque)
    UI->>T: Emit {title, lang, content, mode}
    T->>T: normaliza saltos de línea y valida<br/>(≤ 256 KiB, título ≤ 200)
    T->>H: History.Add → Ejercicio N, snippet_id nuevo
    T->>S: Broadcast(code.share)
    S->>S: solo sesiones conectadas con code.view,<br/>las demás cuentan como «sin capacidad»
    S->>R: code.share {snippet_id, exercise=N, title, lang, content, mode}
    T->>H: Deliveries.Start(ids de sobre)
    T-->>UI: «enviado a N · confirmados 0/N»

    R->>R: Board.Apply: replace sustituye, append agrega<br/>(máx. 20, el mismo snippet_id se actualiza en su lugar)
    R->>W: evento aula:board, la ventana pasa al frente
    R-->>S: code.ack {ref_id, snippet_id}
    S->>S: Registry.RecordCodeAck (último código de la tarjeta)
    S->>T: OnMessage → onAck(ref_id)
    T->>H: Deliveries.Ack
    T-->>UI: «confirmados M/N ✓»

    E->>W: Copiar al portapapeles
    W->>W: platform.Clipboard (CRLF en Windows)
    Note over E,W: Copiar no genera ningún mensaje a la consola
```

## Lo que no es obvio

- **El acuse significa «se mostró», no «se leyó» ni «se copió».** Aula no avisa a la consola
  cuando el alumno copia.
- **Reemitir conserva el número y el `snippet_id`**: el alumno ve el mismo ejercicio
  actualizado, no uno duplicado. Reemitir siempre usa `replace`.
- **`code.clear`** sigue el mismo camino (acuse con `snippet_id` vacío) y **`msg.show`** también,
  con `msg.ack`. Los tres se cuentan en `Deliveries` (diagrama 07).
- **Capacidades**: la consola nunca envía `code.share` a un agente que no declaró `code.view` en
  `hello`; lo cuenta como «sin capacidad» y desactiva el botón si nadie la tiene. Si llegara de
  todos modos, el agente respondería `error unsupported_capability`.
- **Nada de esto se guarda en disco**: el historial y los acuses viven en memoria de la consola
  mientras dura la clase, y el tablero del alumno en memoria del agente (diagrama 11). Los logs
  registran tipos e IDs, nunca el código.
- Con el agente como servicio, `Receiver` corre en el servicio y la ventana es la del ayudante;
  el evento y el portapapeles cruzan el canal local (diagrama 13).

## Fuentes en el código

- `apps/teacher/app.go` (`Emit`, `Reemit`, `ClearScreens`, `SendNotice`, `broadcast`, `onAck`)
- `internal/codedist/history.go`, `internal/codedist/deliveries.go`
- `internal/session/console.go` (`Send`, `Broadcast`), `internal/session/registry.go`
- `internal/codedist/receiver.go`, `internal/codedist/board.go`
- `apps/student/app.go` (`onMessage`, `onBoard`, `Copy`), `internal/platform/clipboard.go`
- `docs/protocol.md` §5.1–5.5, `docs/eventos-ui.md`
