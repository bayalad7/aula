---
titulo: "Monitoreo: miniaturas, pantalla, captura y pausa"
descripcion: "Cómo ve el profesor las pantallas de sus alumnos, cómo toma una captura, cómo las pausa y cómo sabe cada alumno qué pasa con su equipo"
audiencia: [profesor, desarrollador]
pantallas: [alumnos]
secciones_manual: [privacidad, protocol, administracion/tareas-frecuentes]
---

# Monitoreo: miniaturas, pantalla, captura y pausa

¿Qué ocurre al abrir «Ver pantallas», al tomar una captura o al pausar las pantallas, y qué garantiza
que el alumno lo sepa?

```mermaid
%%{init: {"htmlLabels": false}}%%
sequenceDiagram
    autonumber
    actor P as Profesor
    box rgb(220,252,231) Consola
        participant W as screen.Wall
        participant Z as screen.Pauses
    end
    box rgb(220,252,231) Agente
        participant S as screen.Streamer
        participant I as Distintivo en pantalla
        participant K as Captura (platform)
        participant F as screen.Freezer
        participant V as Ventana de pausa (platform)
    end

    P->>W: Ver pantallas (Open y Plan)
    W->>S: screen.thumb.start {1000 ms, calidad 50, 320 px}<br/>solo a quien declaró screen.thumb
    S->>I: «Tu pantalla está siendo vista por el profesor»
    Note over S,I: Si el distintivo no se puede mostrar,<br/>no se captura nada
    loop cada interval_ms
        S->>K: captura del monitor principal
        K-->>S: imagen en memoria
        S->>S: escala y JPEG, baja la calidad si pasa de 640 KiB
        S-->>W: screen.thumb.frame {ref_id, seq, width, height, jpeg}
        W->>W: descarta cuadros de un start anterior,<br/>guarda solo el último por alumno
    end

    P->>W: clic en una miniatura (SetDetail)
    W->>S: screen.thumb.start {1000 ms, calidad 70, 1280 px}<br/>reemplaza parámetros y reinicia seq

    alt el profesor cierra la vista, cambia de canal o termina la clase
        W->>S: screen.thumb.stop
    else se cae la conexión
        Note over S: Streamer.Stop al desconectarse
    end
    S->>I: quita el distintivo
    W->>W: Close olvida todos los cuadros

    Note over P,K: Captura: una foto fija a resolución completa (§5.17)
    P->>W: Tomar captura (ficha, pantalla en grande o panel Captura)
    W->>S: screen.snapshot.request {calidad 90, nativa}<br/>solo a quien declaró screen.snapshot
    S->>I: «Tu profesor está tomando una captura…» 5 s<br/>(sin cambiar el texto si ya se ve la pantalla)
    S->>K: captura del monitor principal, en memoria
    S->>S: aviso «Captura de pantalla» en la ventana (notify)
    S-->>W: screen.snapshot.response, partes de 512 KiB
    W->>W: snapshot.Store ensambla y comprueba size y sha256
    W->>P: vista previa (la imagen sigue en memoria)
    alt el profesor pulsa Guardar
        W->>W: Documentos › Aula › capturas › equipo (usuario) › fecha › hora.jpg
    else Descartar, cerrar la vista previa o la clase
        W->>W: la olvida: nada en disco
    end

    Note over P,V: Pausar pantallas (§5.9–5.11)
    P->>Z: Pausar (panel, Pantalla grande o Ctrl+Mayús+P)
    Z->>F: screen.freeze {message?}<br/>solo reales con screen.freeze, nunca a quien se proyecta
    F->>V: una ventana por monitor, encima de todo, con quién y por qué
    F->>F: banda «… pausó esta computadora» y la ventana al frente (raise)
    F-->>Z: screen.freeze.ack {frozen: true}
    Z->>P: franja «N pantallas en pausa» e insignia «En pausa»
    opt transmisión, cuenta de energía o control remoto
        F->>V: Yield: queda debajo de la ventana de Aula
    end
    alt Reanudar, o tomar el control o proyectar a ese alumno
        Z->>F: screen.unfreeze
        F-->>Z: screen.freeze.ack {frozen: false}
    else se cae la conexión o se cierra el agente
        Note over F: Freezer.Stop: nadie queda en pausa
    end
    F->>V: quita las ventanas
```

## Lo que no es obvio

- **Solo se transmite mientras el profesor mira**: la consola envía `start` al abrir la vista y
  `stop` al cerrarla; no hay captura en segundo plano.
- **El distintivo va antes del primer cuadro**, y si no se puede mostrar no hay captura. Con el
  agente como servicio, la captura solo existe en el ayudante, que es quien muestra el distintivo
  (diagrama 13).
- **Las miniaturas no tocan el disco en ningún extremo**: viven en memoria; la consola muestra la
  última de cada equipo y la olvida al cerrar la vista. Los logs registran `seq` y tamaños, nunca
  la imagen.
- **La captura es la única excepción, y solo en la consola**: se escribe en disco cuando el
  profesor pulsa «Guardar» (`docs/privacidad.md` reglas 7 y 45–46). El agente la tiene en memoria
  lo justo para enviarla. Una captura nativa pesa más que el límite de 1 MiB por mensaje: va en
  partes (`protocol.SplitParts`, `protocol.Reassembly`), y la consola la descarta si no llega
  completa en 30 s o si la huella no coincide.
- **El aviso de la captura no es una banda**: es un hecho puntual. El distintivo nativo va antes
  de capturar (por eso sale en la imagen) y el aviso en la esquina de la ventana, después.
- Si la captura falla (sesión bloqueada, por ejemplo), el agente responde **una sola vez**
  `error internal` y sigue intentando; el distintivo sigue visible.
- Hoy solo Windows declara `screen.thumb`; Linux y macOS devuelven «no soportado».
- **La pausa no es un bloqueo total** (D-013): la ventana de pausa toma los clics y vuelve a quedar
  encima cada medio segundo, pero no intercepta el teclado. Toma el foco al aparecer y cuando el
  alumno le hace clic, no en cada vuelta.
- **La pausa cede el paso** a lo que el alumno debe ver: la transmisión (del profesor o de un
  compañero), la cuenta regresiva de energía y el aviso de control remoto quedan encima. Al
  terminar, la pausa vuelve a cubrir todo.
- **Nadie queda en pausa sin la clase**: al cerrarse la conexión el agente la quita, y con el
  agente como servicio el ayudante la quita también si pierde el canal local (diagrama 13). La
  consola olvida la pausa de quien se desconecta: al volver, entra sin pausa.
- **Las miniaturas muestran la pausa**: es lo que ve el alumno. Por eso, antes de tomar el control
  o de proyectar a un alumno en pausa, la consola le quita la pausa.
- El mensaje de la pausa no se registra en ningún extremo; sí que hubo una pausa y su longitud.

## Fuentes en el código

- `internal/screen/wall.go`, `internal/screen/streamer.go` (`Announce`), `internal/screen/freeze.go`
  (`Freezer`), `internal/screen/pauses.go` (`Pauses`)
- `internal/platform/pause.go`, `internal/platform/pause_windows.go`, `apps/teacher/pause.go`,
  `apps/student/pause.go`
- `internal/snapshot/agent.go` (`Taker`), `internal/snapshot/console.go` (`Store`),
  `internal/snapshot/files.go`, `internal/protocol/parts.go`
- `internal/platform/screen.go`, `internal/platform/screen_windows.go`,
  `internal/platform/screen_other.go`
- `apps/teacher/app.go` (`OpenScreens`, `CloseScreens`, `ViewScreen`, `onFrame`),
  `apps/teacher/snapshot.go` (`TakeSnapshot`, `TakeSnapshots`, `SaveSnapshots`)
- `apps/student/app.go` (`enableScreen`, `onViewed`), `apps/student/snapshot.go` (`onSnapshot`)
- `docs/protocol.md` §5.6–5.11 y §5.17, `docs/privacidad.md` reglas 6–11 y 44–49
