---
titulo: "Acuses y recolección de código/archivos"
descripcion: "Cómo cuenta la consola quién recibió cada envío (código, avisos y archivos) y cómo recoge el trabajo de cada alumno de una sola carpeta acordada"
audiencia: [profesor, desarrollador, auditor]
pantallas: [codigo, archivos]
secciones_manual: [v1/administracion/archivos, protocol, privacidad]
enlaces: [deliveries=eventos-ui]
---

# Acuses y recolección de código/archivos

¿Cómo sabe la consola cuántos alumnos recibieron un envío, y cómo recoge de vuelta el ejercicio
de cada uno sin leer nada fuera de la carpeta acordada?

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
        reco["Archivos › Recolecciones<br/>Crear carpeta · Recoger"]
        collector["collect.Collector<br/>4 alumnos a la vez<br/>ensambla y comprueba sha256"]
        disco["Documentos\Aula\recolecciones\<br/>fecha\actividad\alumno\<br/>+ aula-recoleccion.json"]
    end

    subgraph agente["Agente"]
        recv["codedist.Receiver<br/>filepush.Receiver"]
        resp["collect.Responder<br/>aviso antes de leer<br/>un archivo a la vez"]
        carpeta["Documentos\Aula\recolecciones\<br/>fecha\actividad<br/>solo esa carpeta, sin enlaces"]
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

    reco --> collector
    collector -- "file.collect.open<br/>fecha y actividad, nunca una ruta" --> resp
    resp -- "crea la carpeta" --> carpeta
    collector -- "file.collect.request<br/>ronda, tipos y límites" --> resp
    carpeta -- "lista y lee<br/>como el alumno" --> resp
    resp -- "file.collect.ack<br/>cuántos y qué quedó fuera" --> collector
    resp -- "file.collect.response<br/>partes de 512 KiB" --> collector
    collector -- "file.collect.response.ack<br/>por archivo" --> resp
    collector -- "guarda al llegar" --> disco
    collector --> reco

    classDef externo fill:#dbeafe,stroke:#1d4ed8,color:#111827
    classDef aula fill:#dcfce7,stroke:#15803d,color:#111827
    classDef datos fill:#ffedd5,stroke:#c2410c,color:#111827
    classDef seguridad fill:#fee2e2,stroke:#b91c1c,color:#111827
    class envio,bcast,recv,ui,pusher,reco,collector,resp aula
    class deliveries,registro,disco datos
    class carpeta seguridad
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
- **La recolección no pasa por `Deliveries`.** Tiene su propia tabla por alumno en
  Archivos › Recolecciones (`collect.Summary`): carpeta lista, recogido, carpeta vacía o no se
  pudo, con el motivo.
- **La consola nunca manda una ruta.** `file.collect.open` y `.request` llevan la fecha y el
  nombre de la actividad; el agente arma `recolecciones/<fecha>/<actividad>` y la comprueba con las
  reglas de ruta de §5.20. En modo servicio lee el ayudante, con el token del alumno, y vuelve a
  comprobar que la ruta es una carpeta de actividad (`collect.CheckWorkPath`).
- **Al revés que `file.push`:** aquí el agente es quien trocea y espera el acuse de cada archivo
  antes del siguiente; la consola pide de 4 en 4 alumnos para no saturar la red. Las partes no
  llegan antes de que la consola prepare dónde escribirlas: el `ack` `sending` las precede.
- **El alumno sabe antes y después** (`docs/privacidad.md` reglas 61–62): la tarjeta al abrir, un
  aviso antes de leer y otro con lo recogido; nada de eso le quita el foco.
- **Cada ronda reemplaza la copia del profesor** de ese alumno en un solo cambio (carpeta temporal
  y renombrado): una ronda a medias no deja archivos sueltos. El original del alumno no cambia.
- `code.collect` (entregar desde el overlay) sigue reservado (`docs/protocol.md` §6).
- Hay otros acuses con su propio rastreo, fuera de `Deliveries`: `app.block.ack` y
  `web.filter.ack` (diagrama 15) y `power.ack` (diagrama 16).

## Fuentes en el código

- `internal/codedist/deliveries.go`
- `internal/filepush/pusher.go`, `apps/teacher/files.go`
- `internal/collect/collector.go`, `internal/collect/responder.go`, `internal/collect/store.go`,
  `apps/teacher/collect.go`, `apps/student/collect.go`
- `internal/platform/workfolder_read_windows.go`, `internal/agentsvc/proxy_collect.go`
- `internal/session/console.go` (`Broadcast`, `route`), `internal/session/registry.go`
  (`RecordCodeAck`)
- `apps/teacher/app.go` (`broadcast`, `onAck`), `apps/teacher/class.go`
- `internal/codedist/receiver.go`
- `docs/protocol.md` §5.2, §5.5, §5.20 y §5.21
