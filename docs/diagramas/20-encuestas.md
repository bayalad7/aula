---
titulo: "Encuestas"
descripcion: "Cómo llega una encuesta del profesor a los alumnos, cómo vuelven sus respuestas, cómo se cierra y qué ven al compartir los resultados"
audiencia: [profesor, alumno, desarrollador, auditor]
pantallas: [participacion]
secciones_manual: [administracion/participacion, protocol, privacidad]
---

# Encuestas

¿Qué pasa desde que el profesor lanza una pregunta rápida hasta que la clase ve el resultado, y
qué ve cada quien de las respuestas?

```mermaid
%%{init: {"htmlLabels": false}}%%
sequenceDiagram
    autonumber
    actor P as Profesor
    box rgb(220,252,231) Consola
        participant V as Panel Encuesta y Participación
        participant C as poll.Class
    end
    box rgb(219,234,254) Agente
        participant B as poll.Ballot
        participant W as Ventana del alumno
    end
    actor E as Alumno

    P->>V: Lanzar (pregunta, tipo, opciones, anónima o nominal)
    V->>C: OpenPoll → cierra la que estaba abierta
    C->>B: poll.open {poll_id, kind, question, options, named}
    B-->>C: poll.ack {shown}
    B->>W: aula:polls (llegó una encuesta)
    W-->>E: tarjeta «Encuesta · Nuevo» arriba de la lista,<br/>dice si es anónima, sin quitar el foco (notify)
    C->>V: franja «Encuesta abierta» y barras en Participación

    E->>W: elige una opción y Enviar
    W->>B: AnswerPoll
    B->>C: poll.vote {poll_id, choice o text}
    C->>C: un voto por alumno (equipo y usuario),<br/>cuenta el último
    C-->>B: poll.vote.ack {accepted}
    B->>W: «Respuesta enviada»
    C->>V: las barras se mueven, con texto y número

    opt el alumno cambia su respuesta
        E->>W: otra opción y Cambiar mi respuesta
        B->>C: poll.vote (reemplaza la anterior)
        C-->>B: poll.vote.ack {accepted}
    end

    P->>V: Cerrar encuesta
    V->>C: ClosePoll
    C->>B: poll.close {poll_id}
    B-->>C: poll.ack {closed}
    B->>W: «Encuesta cerrada», sin poder cambiarla
    Note over C,B: una respuesta que llega tarde<br/>se acusa {closed} y no cuenta

    opt el profesor comparte los resultados (opción múltiple o sí o no)
        P->>V: Compartir resultados
        C->>B: poll.results {counts, voters, recipients}<br/>solo totales, sin simulados
        B-->>C: poll.ack {results}
        B->>W: barras de la clase con su respuesta marcada,<br/>nunca quién eligió qué
    end

    opt se corta la red y vuelve
        C->>B: poll.open otra vez si sigue abierta,<br/>o poll.close y poll.results si se cerró
        B->>C: la respuesta sin acuse, con lo mismo
        Note over B,W: si en 3 s no llega, la tarjeta queda cerrada
    end
```

## Lo que no es obvio

- **No toca el equipo del alumno**: no hay banda ni aviso fijo, y nada trae su ventana al frente.
  La capacidad `poll` la declara cualquier agente con ventana, en cualquier sistema, y también los
  alumnos simulados, que responden solos para probar las barras sin laboratorio.
- **Anónima o nominal lo decide el profesor en cada encuesta**, y el alumno lo lee en la tarjeta
  antes de responder. En una anónima la consola guarda internamente a quién corresponde cada voto
  (para contar uno por alumno y aceptar los reenvíos), pero la vista nunca lo muestra: solo
  totales y, en las abiertas, los textos sin nombre ni hora, en orden alfabético
  (`docs/privacidad.md` regla 72).
- **Lo compartido son solo totales**, nunca quién eligió qué, aunque la encuesta sea nominal. Las
  respuestas abiertas no se comparten. Los votos simulados no entran en ese total (regla 32).
- **Una abierta a la vez**: lanzar otra cierra la vigente. La consola conserva las 20 últimas de la
  clase en memoria y las borra al cerrarla o cambiar de canal. El agente conserva 5 tarjetas.
- **Ritmo**: la ventana acepta una respuesta por segundo, y la consola, por si acaso, admite
  ráfagas de 3 y descarta lo que pase con `error invalid_message`.
- **Nada se escribe en los registros** de la pregunta, las opciones ni las respuestas: solo
  `poll_id`, el tipo, los estados y los totales (regla 74).
- Con el agente como servicio, la ventana es la del ayudante: «Enviar» llega al servicio por el
  canal local (`CallBinding`, diagrama 13), que es quien tiene la conexión con la consola.

## Fuentes en el código

- `internal/poll/class.go` (consola), `internal/poll/ballot.go` (agente)
- `apps/teacher/poll.go` (`OpenPoll`, `ClosePoll`, `SharePollResults`, `onPollMessage`)
- `apps/student/poll.go` (`AnswerPoll`, `handlePoll`)
- `internal/simclass/poll.go` (alumnos simulados)
- `docs/protocol.md` §5.23, `docs/eventos-ui.md`
