---
titulo: "Monitoreo: miniaturas y pantalla"
descripcion: "Cómo ve el profesor las pantallas de sus alumnos y cómo sabe cada alumno que lo están viendo"
audiencia: [profesor, desarrollador]
pantallas: [screens]
secciones_manual: [privacidad, protocol]
---

# Monitoreo: miniaturas y pantalla

¿Qué ocurre al abrir «Ver pantallas», y qué garantiza que el alumno lo sepa?

```mermaid
%%{init: {"htmlLabels": false}}%%
sequenceDiagram
    autonumber
    actor P as Profesor
    box rgb(220,252,231) Consola
        participant W as screen.Wall
    end
    box rgb(220,252,231) Agente
        participant S as screen.Streamer
        participant I as Distintivo en pantalla
        participant K as Captura (platform)
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
```

## Lo que no es obvio

- **Solo se transmite mientras el profesor mira**: la consola envía `start` al abrir la vista y
  `stop` al cerrarla; no hay captura en segundo plano.
- **El distintivo va antes del primer cuadro**, y si no se puede mostrar no hay captura. Con el
  agente como servicio, la captura solo existe en el ayudante, que es quien muestra el distintivo
  (diagrama 13).
- **Sin disco en ningún extremo**: las imágenes solo viven en memoria; la consola muestra la
  última de cada equipo y la olvida al cerrar la vista. Los logs registran `seq` y tamaños, nunca
  la imagen.
- Si la captura falla (sesión bloqueada, por ejemplo), el agente responde **una sola vez**
  `error internal` y sigue intentando; el distintivo sigue visible.
- Hoy solo Windows declara `screen.thumb`; Linux y macOS devuelven «no soportado».
- **Pantalla en blanco (`screen.freeze`) no está implementada**: está documentada como pendiente
  (`docs/protocol.md` §5.9–5.11, D-013) y ningún agente declara su capacidad, por eso no aparece
  aquí.

## Fuentes en el código

- `internal/screen/wall.go`, `internal/screen/streamer.go`
- `internal/platform/screen.go`, `internal/platform/screen_windows.go`,
  `internal/platform/screen_other.go`
- `apps/teacher/app.go` (`OpenScreens`, `CloseScreens`, `ViewScreen`, `onFrame`)
- `apps/student/app.go` (`enableScreen`, `onViewed`)
- `docs/protocol.md` §5.6–5.8, `docs/privacidad.md` reglas 6–8
