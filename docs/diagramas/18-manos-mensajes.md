---
titulo: "Manos y mensajes del alumno"
descripcion: "Cómo llega al profesor una mano levantada o un mensaje del alumno a su conversación, y cómo vuelven la respuesta y «Bajar la mano»"
audiencia: [profesor, alumno, desarrollador]
pantallas: [participacion]
secciones_manual: [administracion/participacion, protocol, privacidad]
---

# Manos y mensajes del alumno

¿Qué pasa desde que el alumno pulsa «Levantar la mano» o escribe un mensaje hasta que el profesor
le responde o baja su mano?

```mermaid
%%{init: {"htmlLabels": false}}%%
sequenceDiagram
    autonumber
    actor E as Alumno
    box rgb(219,234,254) Agente
        participant W as Ventana del alumno
        participant D as participation.Desk
    end
    box rgb(220,252,231) Consola
        participant B as participation.Board
        participant V as Vista Participación
    end
    actor P as Profesor

    E->>W: Levantar la mano (motivo opcional)
    W->>D: RaiseHand → hand_id nuevo
    D->>B: hand.raise {hand_id, note?}
    B->>B: fila por orden de llegada,<br/>una mano por alumno
    B->>V: aula:participation (llegó una mano)
    V-->>P: aviso «PC07 levantó la mano», contador en el menú,<br/>insignia «Mano levantada · 1» en Alumnos

    E->>W: Mensaje al profesor (Enter envía)
    W->>D: SendTeacherMessage → message_id nuevo
    D->>B: msg.student {message_id, text}
    B-->>D: msg.student.ack {message_id}
    D->>W: «Recibido» (llegó, no «leído»)
    B->>V: la conversación del alumno pasa a «Por atender»
    Note over V: bandeja: una fila por alumno, la más reciente arriba,<br/>con el foco en la lista, no se reordena

    P->>V: abre la conversación y responde (Enter envía)
    V->>B: ReplyToStudent (reply_to: el último mensaje que espera)
    B->>B: la respuesta entra a la conversación,<br/>esos mensajes quedan respondidos («Atendidas»)
    B->>W: msg.show {text, personal, reply_to = message_id}
    W-->>B: msg.ack
    W->>D: la respuesta entra al hilo de «Tus mensajes»
    Note over W: «Respuesta a tu mensaje» con la cita,<br/>sin quitar el foco (notify)

    alt el profesor baja la mano
        P->>V: Bajar la mano
        B->>D: hand.lower {hand_id, by: teacher}
        D->>W: «Tu profesor bajó tu mano»
    else el alumno la baja
        E->>W: Bajar la mano
        D->>B: hand.lower {hand_id, by: student}
    end

    opt se corta la red y vuelve
        D->>B: hand.raise con el mismo hand_id y los mensajes sin acuse
        B->>B: la mano conserva su lugar,<br/>un mensaje repetido se acusa otra vez
        Note over B,D: si el profesor la bajó mientras tanto,<br/>la consola responde hand.lower {by: teacher}
    end
```

## Lo que no es obvio

- **Lo inicia siempre el alumno, y no toca su equipo**: no hay banda ni aviso de transparencia, y
  nada trae su ventana al frente. Las capacidades `hand` y `msg.student` las declara cualquier
  agente con ventana, en cualquier sistema; también los alumnos simulados.
- **El acuse dice «llegó a la consola»**, no que el profesor lo leyó, igual que `msg.ack`.
- **La respuesta reutiliza `msg.show`** con el campo opcional `reply_to` (diagrama 06): un agente
  anterior la muestra como un aviso normal.
- **Ritmo**: el agente deja un mensaje cada 3 s y como mucho 5 sin acuse; la consola, por si
  acaso, admite ráfagas de 5 y descarta lo que pase con `error invalid_message`. Guarda 50
  líneas por conversación (mensajes del alumno y lo que el profesor le mandó); al pasar, se va lo
  más viejo ya atendido, nunca un mensaje que espera si hay otro.
- **Una conversación por alumno** (prompt 10.26): junta sus mensajes, las respuestas, lo que se le
  respondió a su mano y los «Mensaje privado» de la Pantalla grande. No hay mensaje nuevo en el
  protocolo: el agente arma su hilo con los `msg.show` `personal` y su `reply_to`. Mientras la
  pantalla del alumno se proyecta, su ventana oculta el texto del profesor.
- **Nada se guarda en disco ni en los registros**: la fila y la bandeja viven en la memoria de la
  consola hasta cerrar la clase; los logs llevan `hand_id`, `message_id` y longitudes, nunca el
  texto (`docs/privacidad.md` reglas 50–52).
- Con el agente como servicio, la ventana es la del ayudante: sus botones llegan al servicio por
  el canal local (`CallBinding`, diagrama 13), que es quien tiene la conexión con la consola.

## Fuentes en el código

- `internal/participation/board.go` (consola), `internal/participation/desk.go` (agente)
- `apps/teacher/participation.go` (`onClassroomMessage`, `LowerHand`, `ReplyToStudent`,
  `MarkConversationDone`), `apps/teacher/private.go` (`SendPrivateNotice`)
- `apps/student/app.go` (`onNotice`: el hilo de «Tus mensajes»)
- `apps/student/participation.go` (`RaiseHand`, `LowerHand`, `SendTeacherMessage`, `onConnect`)
- `internal/session/console.go` (capacidad de los mensajes que inicia el agente),
  `internal/session/agent.go` (`OnConnect`)
- `internal/simclass/participation.go` (alumnos simulados)
- `docs/protocol.md` §5.18, `docs/eventos-ui.md`
