---
titulo: "Control remoto de un alumno"
descripcion: "Cómo toma el profesor el teclado y el mouse de un alumno, cómo lo sabe el alumno y cómo termina"
audiencia: [profesor, alumno, desarrollador]
pantallas: [alumnos]
secciones_manual: [administracion/tareas-frecuentes, protocol, privacidad]
---

# Control remoto de un alumno

¿Qué pasa desde que el profesor pulsa «Tomar control» hasta que lo suelta, y qué ve el alumno en
cada momento?

```mermaid
%%{init: {"htmlLabels": false}}%%
sequenceDiagram
    autonumber
    box rgb(220,252,231) Consola
        participant D as remote.Desk
    end
    box rgb(220,252,231) Agente (servicio)
        participant A as remote.Agent
        participant S as screen.Streamer
    end
    box rgb(219,234,254) Sesión del alumno
        participant H as Ayudante y ventana
    end

    D->>A: remote.start {control_id, 125 ms, 1280 px}
    A->>H: raise, banda «… está controlando tu equipo»
    A->>S: StartControl
    S->>H: indicator.show «El profesor está controlando tu equipo»
    A-->>D: remote.ack active
    loop hasta 8 por segundo
        S->>H: screen.capture
        S-->>D: screen.thumb.frame {ref_id del remote.start}
    end
    loop lotes de hasta 32 eventos, máximo 60 por segundo
        D->>A: remote.input {seq, events}
        A->>H: input.inject (SendInput en la sesión)
    end

    opt el profesor toma una captura durante el control
        D->>D: la consola suelta sus teclas y botones (vista previa)
        D->>A: screen.snapshot.request (no es remote.stop)
        Note over A,H: el aviso fijo sigue diciendo que lo controlan,<br/>el control no se pausa ni termina
        H-->>D: screen.snapshot.response, en partes
    end

    opt el alumno mueve su mouse o escribe
        A->>H: input.local (cada 200 ms)
        A->>H: suelta teclas y botones
        A-->>D: remote.ack paused
        Note over A: descarta la entrada del profesor
        A-->>D: remote.ack resumed (2 s sin entrada del alumno)
    end

    alt el profesor pulsa Soltar o cierra la clase
        D->>A: remote.stop
        A->>H: suelta teclas y botones, quita banda y aviso fijo
        A-->>D: remote.ack ended (teacher)
    else 5 min sin remote.input
        A->>H: suelta y quita banda y aviso fijo
        A-->>D: remote.ack ended (idle)
    else se corta la red
        Note over A: el agente termina por sí mismo, no se reanuda
        A->>H: suelta y quita banda y aviso fijo
        Note over D: la consola lo da por terminado al ver la desconexión
    end
    H->>H: aviso en la esquina «… dejó de controlar tu equipo»
```

## Lo que no es obvio

- **Sin consentimiento, pero nunca a escondidas.** El agente muestra la banda y el aviso fijo y
  trae su ventana al frente **antes** de responder `active`; descarta la entrada que llegue antes.
  El alumno no puede cerrar la banda ni cortar el control (`docs/privacidad.md` reglas 36–39).
- **El alumno tiene prioridad.** El agente no instala *hooks* de teclado: compara la última
  entrada del sistema (`GetLastInputInfo`) y la posición del puntero con lo que inyectó Aula. Sabe
  *que* el alumno usó su equipo, nunca *qué* tecleó.
- **El video es la vista en grande de siempre**, a más velocidad: el mismo `screen.Streamer` y la
  misma captura por el ayudante. Mientras dura, el agente guarda las miniaturas que pida la
  consola y las retoma al terminar. Si una imagen no salió, se salta la siguiente captura: nunca
  se atrasa.
- **Nada queda presionado.** El agente suelta lo que presionó al pausar, terminar o perder la
  conexión; el ayudante también, si el servicio desaparece.
- **Una captura no interrumpe el control** (`docs/protocol.md` §5.16 y §5.17): el profesor la toma
  desde la vista del control; mientras ve la vista previa su teclado no llega al alumno, y el
  aviso fijo no cambia de texto (`docs/privacidad.md` regla 44).
- **Uno a la vez**: `remote.Desk` rechaza un segundo control (`alreadyControlling`). Los alumnos
  simulados no declaran `remote` y nunca reciben estos mensajes.
- **Lo que Windows no deja**: Ctrl+Alt+Supr, la pantalla de bloqueo, el aviso de UAC y las ventanas
  de administrador (UIPI). Si el sistema rechaza la entrada 30 s seguidos, el control termina con
  `error`.

## Fuentes en el código

- `internal/remote/agent.go`, `internal/remote/desk.go`
- `internal/screen/streamer.go` (`StartControl`, `RetuneControl`, `StopControl`)
- `internal/platform/input.go`, `internal/platform/input_windows.go`
- `internal/agentsvc/proxy.go` (`input.inject`, `input.local`)
- `apps/teacher/remote.go`, `apps/student/remote.go`, `apps/student/helper.go`
- `docs/protocol.md` §5.16, `docs/privacidad.md` reglas 36–43, `docs/decisiones.md` D-019
