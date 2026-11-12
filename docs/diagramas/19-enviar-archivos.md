---
titulo: "Enviar archivos y plantillas"
descripcion: "Cómo llegan los archivos del profesor a la carpeta de trabajo de cada alumno, sin salir de ella, y cómo sabe la consola quién los tiene"
audiencia: [profesor, alumno, desarrollador, auditor]
pantallas: [archivos]
secciones_manual: [administracion/archivos, protocol, privacidad]
---

# Enviar archivos y plantillas

¿Qué pasa desde que el profesor pulsa «Enviar» en la pestaña Archivos hasta que el archivo está
en la carpeta del alumno y «Últimos envíos» dice que lo tiene?

```mermaid
%%{init: {"htmlLabels": false}}%%
sequenceDiagram
    autonumber
    actor P as Profesor
    box rgb(220,252,231) Consola
        participant C as Vista Archivos
        participant U as filepush.Pusher
    end
    box rgb(219,234,254) Agente como servicio
        participant R as filepush.Receiver
    end
    box rgb(254,226,226) Sesión del alumno
        participant H as Ayudante
        participant W as Ventana del alumno
    end

    P->>C: Elegir archivos o una plantilla,<br/>«Guardar en», Enviar a
    C->>C: NewPush: nombres y carpeta válidos,<br/>≤ 20 archivos, 16 MB c/u, 32 MB en total
    C->>U: Deliver (solo a quien declaró file.push)
    loop cada archivo, uno tras otro (4 alumnos a la vez)
        U->>R: file.push parte 1 {name, dest, size, sha256, overwrite}
        R->>R: CheckName y CheckDest:<br/>si no valen, file.push.ack invalid_path
        U->>R: file.push partes 2…N (512 KiB c/u)
        R->>R: ensambla en memoria,<br/>comprueba size y sha256
        R->>H: file.write {rel, data, overwrite}
        H->>H: CheckRel otra vez, WorkFolder.Create<br/>en Documentos\Aula, sin seguir enlaces,<br/>con el token del alumno
        alt ya existe y overwrite = false
            H->>H: prueba «nombre (2)», «(3)»… hasta (99)
        end
        H-->>R: replaced o el error (exists, no_space…)
        R-->>U: file.push.ack {status, path, renamed, replaced, reason?}
        R->>W: aula:files (notify, nunca raise)
    end
    U->>C: Ack si guardó todos, o Reject con el motivo
    C-->>P: «Últimos envíos»: 7 de 8 · Quién falta (disco lleno)
    W-->>W: tarjeta «Archivo recibido: datos.csv ·<br/>Guardado en Documentos\Aula\Ejercicio 4»
    opt el alumno pulsa «Abrir carpeta»
        W->>R: OpenReceivedFolder(pushId)
        R->>H: folder.open (la carpeta de ese envío)
    end
```

## Lo que no es obvio

- **La ruta se comprueba tres veces y nunca se corrige.** La consola rechaza un nombre malo antes
  de enviar; el agente responde `invalid_path` sin tocar el disco; el ayudante vuelve a revisar la
  ruta sin limpiarla (`a/../x` se rechaza, no se convierte en `x`) y la carpeta de trabajo se niega
  a cruzar un enlace o *junction* (`docs/privacidad.md` regla 55).
- **Escribe el ayudante, no el servicio.** Así el archivo es del alumno y un enlace no le daría a
  Aula más permisos que los que ya tiene él (diagrama 13). En modo ventana, el propio agente corre
  como el alumno.
- **Nunca se pisa trabajo sin decirlo.** Sin «Reemplazar si ya lo tienen», un nombre ocupado se
  guarda como `nombre (2)`. Escribir pasa por un archivo temporal oculto que se mueve al final, así
  que nunca queda un archivo a medias.
- **El acuse va por archivo; el conteo, por alumno.** El `Pusher` espera el `file.push.ack` de un
  archivo (60 s como mucho) antes de mandar el siguiente, y cuenta al alumno en `Deliveries` solo
  cuando terminó con todos (diagrama 07). Si no responde, el motivo es `timeout`; si se fue,
  `gone`. Cerrar la clase corta lo que se estaba enviando.
- **Memoria acotada en el agente**: 4 archivos a la vez y 32 MiB; lo que no se completa en 60 s se
  descarta (`incomplete`). Nada se registra con nombres ni contenidos.
- **Los alumnos simulados** pasan por el mismo `Receiver`, con una carpeta en memoria que solo
  guarda nombres: acusan (y renombran) sin escribir en disco.
- **Las plantillas son carpetas** en `Documentos\Aula\plantillas` del profesor; se leen al enviar,
  así que un cambio hecho en el Explorador se nota en el siguiente envío o reenvío.

## Fuentes en el código

- `internal/filepush/path.go` (`CheckName`, `CheckDest`, `CheckRel`, `Alternative`),
  `internal/filepush/push.go`, `internal/filepush/pusher.go`, `internal/filepush/receiver.go`,
  `internal/filepush/inbox.go`, `internal/filepush/templates.go`
- `internal/platform/workfolder.go`, `internal/platform/workfolder_windows.go`
- `internal/agentsvc/proxy.go` (`WorkFolder`, `CallFileWrite`, `CallFolderOpen`)
- `apps/teacher/files.go`, `apps/student/files.go`, `apps/student/helper.go`
- `internal/simclass/files.go`
- `docs/protocol.md` §5.20, `docs/privacidad.md` reglas 55–59
