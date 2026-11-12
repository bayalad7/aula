---
titulo: "Terminar la clase"
descripcion: "En qué orden la consola detiene lo activo y limpia la ventana de cada alumno, qué acusa cada uno y qué pasa con quien falta"
audiencia: [profesor, alumno, desarrollador]
pantallas: [alumnos]
secciones_manual: [administracion/tareas-frecuentes, privacidad, protocol]
---

# Terminar la clase

¿Qué hace «Terminar la clase», en qué orden, y cómo se entera la consola de que cada ventana
quedó limpia, también la de un alumno que estaba desconectado?

```mermaid
%%{init: {"htmlLabels": false}}%%
sequenceDiagram
    autonumber
    actor Prof as Profesor
    box rgb(220,252,231) Consola
        participant UI as Diálogo «Terminar la clase»
        participant C as classend.Class
    end
    box rgb(220,252,231) Equipos de los alumnos
        participant A as Agente conectado
        participant D as Agente desconectado
    end

    Prof->>UI: Terminar la clase (opcional: recoger, cancelar la energía)
    opt Recoger el trabajo antes de terminar
        UI->>A: file.collect.request (§5.21), espera hasta 60 s
    end
    UI->>A: screen.broadcast.stop, student.present.stop, remote.stop
    UI->>A: screen.unfreeze, app.block.clear, web.filter.clear
    UI->>A: timer.stop, poll.close
    alt el profesor marcó «Cancelar el apagado programado»
        UI->>A: power.cancel
    else por omisión
        Note over UI: la energía pendiente se conserva
    end
    UI->>C: Begin, un end_id nuevo
    UI->>A: class.end (end_id)
    Note over A: suelta el control, quita pausa y restricciones,<br/>vacía código, avisos, archivos, encuestas y la mano,<br/>muestra «La clase terminó» (notify, sin quitar el foco)
    A-->>C: class.end.ack clean o failed (restrictions, pause)
    Note over C: D estaba desconectado, queda «lo recibe al volver»
    Note over UI: la consola queda «sin clase»: sin historial,<br/>manos, encuestas ni envíos. Sigue en el canal.
    C-->>UI: resultado por alumno (limpio, sin acuse, con error)

    rect rgb(255,237,213)
        D->>C: se reconecta (hello)
        C->>D: class.end, antes de reenviar nada
        D-->>C: class.end.ack clean
    end
    opt sin acuse en 10 s o failed
        Prof->>UI: Reintentar a los que faltan
        UI->>A: class.end otra vez
    end
```

## Lo que no es obvio

- **Primero lo que actúa sobre el equipo, después la ventana.** La consola usa la parada de
  siempre de cada función (§5.12–5.24), así el alumno ve desaparecer cada banda con su propia
  regla; `class.end` solo deja la ventana como recién abierta y repite las paradas en el agente
  por si alguna no llegó.
- **Si un paso falla, los demás siguen** y el resultado lo dice. Terminar dos veces seguidas no
  rompe nada: el agente vuelve a acusar `clean`.
- **La energía no se cancela sola** (regla 85): el profesor pudo programar el apagado de fin de
  día a propósito. Solo con «Cancelar el apagado programado» se envía `power.cancel`.
- **Quien faltaba no hereda nada.** Las restricciones y la pausa ya se quitaron solas al perder la
  conexión (reglas 10 y 24); al volver, recibe `class.end` antes que lo que la consola reenvía al
  reconectar, y no pierde nada nuevo: mientras estuvo fuera no recibió nada.
- **Un agente anterior** (sin `class.end`) recibe las paradas y `code.clear`: queda «limpio en
  parte».
- **Nunca se borra un archivo** (regla 84): ni en `Documentos\Aula` del alumno ni en las carpetas
  de la consola.
- Los alumnos simulados también lo reciben y limpian su ventana simulada.

## Fuentes en el código

- `internal/classend/classend.go` (`Class`: `Begin`, `Sent`, `Ack`, `Expire`, `Owed`, `Retry`)
- `internal/classend/agent.go` (`Removed`, `Card`, `Ack`)
- `apps/teacher/classend.go` (`FinishClass`, `runClassEnd`, `syncClassEnd`, `clearConsoleClass`)
- `apps/student/classend.go` (`endClass`), `internal/simclass/classend.go`
- `docs/protocol.md` §5.25, `docs/privacidad.md` reglas 83–86
