---
titulo: "Acuses y recolección de código/archivos"
descripcion: "Cómo cuenta la consola quién recibió cada envío (código, avisos y archivos), y qué falta para recoger trabajos de los alumnos"
audiencia: [desarrollador]
pantallas: [codigo]
secciones_manual: [protocol]
enlaces: [deliveries=eventos-ui, reservado=protocol#6-reservados-para-fases-futuras]
---

# Acuses y recolección de código/archivos

¿Cómo sabe la consola cuántos alumnos recibieron un envío, y qué se recoge hoy de vuelta?

```mermaid
%%{init: {"htmlLabels": false, "flowchart": {"htmlLabels": false}}}%%
flowchart LR
    subgraph consola["Consola"]
        envio["Emitir · Limpiar · Aviso · Archivos"]
        bcast["Console.Broadcast<br/>un sobre por sesión"]
        deliveries["Deliveries<br/>20 envíos recientes<br/>sent · acked · failed · noCapability"]
        pusher["filepush.Pusher<br/>archivos: uno tras otro,<br/>4 alumnos a la vez"]
        registro["Registry<br/>último code.ack por alumno"]
        ui["Panel: «confirmados M/N ✓»<br/>tarjeta: último código"]
    end

    subgraph agente["Agente"]
        recv["codedist.Receiver<br/>filepush.Receiver"]
    end

    envio --> bcast
    bcast -- "ids de sobre" --> deliveries
    bcast -- "code.share · code.clear · msg.show" --> recv
    recv -- "code.ack / msg.ack<br/>ref_id = id del sobre" --> registro
    envio -- "Enviar archivos" --> pusher
    pusher -- "file.push (partes)" --> recv
    recv -- "file.push.ack por archivo" --> pusher
    pusher -- "Ack o Reject por alumno<br/>cuando acusó todos" --> deliveries
    registro --> deliveries
    deliveries --> ui
    registro --> ui

    reservado["code.collect · file.collect<br/>reservados, sin implementar"]
    reservado -. "etapa 10.10" .-> consola

    classDef externo fill:#dbeafe,stroke:#1d4ed8,color:#111827
    classDef aula fill:#dcfce7,stroke:#15803d,color:#111827
    classDef datos fill:#ffedd5,stroke:#c2410c,color:#111827
    classDef seguridad fill:#fee2e2,stroke:#b91c1c,color:#111827
    class envio,bcast,recv,ui,pusher aula
    class deliveries,registro datos
    class reservado externo
```

## Lo que no es obvio

- **Se cuenta por sobre, no por alumno.** Cada `Broadcast` devuelve el `id` del sobre enviado a
  cada sesión; el acuse trae ese `id` en `ref_id`. `Deliveries.Ack` cuenta cada sobre una sola
  vez e ignora referencias desconocidas o repetidas, así que un acuse tardío de un envío viejo no
  infla el conteo de otro.
- **Cuatro tipos de envío** se rastrean igual: `code` (`code.share`), `clear` (`code.clear`),
  `notice` (`msg.show`) y `file` (`file.push`, prompt 10.9). `failed` son sesiones a las que no se pudo escribir;
  `noCapability`, las que no declararon la capacidad.
- **Un envío de archivos se cuenta por alumno, no por sobre.** Cada archivo viaja en partes y
  tiene su propio `file.push.ack`; `filepush.Pusher` espera el acuse de un archivo antes de mandar
  el siguiente (60 s como mucho) y, al terminar con un alumno, cuenta un `Ack` si guardó todos o un
  `Reject` con el motivo (`no_space`, `timeout`, `gone`…), que «Quién falta» explica. El detalle
  está en el diagrama 19.
- Solo se recuerdan los **20 envíos más recientes**, en memoria; al cerrar la consola se pierden
  (diagrama 11).
- **La recolección no existe todavía.** `code.collect` y `file.collect` están reservados en el
  protocolo (§6) y sus capacidades no las declara ningún agente; se diseñan en la etapa 10.10.
  Este diagrama se amplía cuando lleguen.
- Hay otros acuses con su propio rastreo, fuera de `Deliveries`: `app.block.ack` y
  `web.filter.ack` (diagrama 15) y `power.ack` (diagrama 16).

## Fuentes en el código

- `internal/codedist/deliveries.go`
- `internal/filepush/pusher.go`, `apps/teacher/files.go`
- `internal/session/console.go` (`Broadcast`, `route`), `internal/session/registry.go`
  (`RecordCodeAck`)
- `apps/teacher/app.go` (`broadcast`, `onAck`), `apps/teacher/class.go`
- `internal/codedist/receiver.go`
- `docs/protocol.md` §5.2, §5.5 y §6
