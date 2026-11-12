---
titulo: "El agente como servicio de Windows"
descripcion: "Cómo se reparte el agente entre un servicio sin ventana y un ayudante visible en la sesión del alumno"
audiencia: [instalador, desarrollador, auditor]
pantallas: [falta]
secciones_manual: [administracion/como-funciona, privacidad, configuracion]
---

# El agente como servicio de Windows

¿Qué corre como SYSTEM, qué corre en la sesión del alumno y cómo se hablan?

```mermaid
%%{init: {"htmlLabels": false, "flowchart": {"htmlLabels": false}}}%%
flowchart LR
    consola["Consola del profesor"]

    subgraph svc["Servicio «Aula — agente del aula» (SYSTEM, sesión 0)"]
        agente["session.Agent<br/>conexión con la consola"]
        sup["agentsvc.Supervisor<br/>lanza, vigila y mueve al ayudante"]
        proxies["Proxies Clipboard, Capture, Indicator,<br/>Input, WorkFolder, Opener, Pause y power.run"]
        pol["policy.Manager y power.Countdown"]
    end

    pipe["Named pipe local<br/>platform.ListenLocal<br/>solo SYSTEM y el SID del alumno"]

    subgraph user["Sesión del alumno (sus permisos)"]
        ayud["aula-cliente.exe --ayudante<br/>ventana, distintivo, avisos y pausa"]
        so["Portapapeles, captura, entrada<br/>del control remoto, apagado sin force,<br/>Documentos\Aula del alumno,<br/>abrir una página o una app permitida,<br/>ventana de pausa en sus monitores"]
    end

    sesiones["platform.SessionManager<br/>sesión activa, Launch con el token del usuario"]

    consola <-- "wss:// (diagrama 04)" --> agente
    agente --> proxies
    agente --> pol
    sup --> sesiones
    sesiones -- "Launch" --> ayud
    sup -- "comprueba PeerPID" --> pipe
    proxies <-- "Link: JSON por líneas<br/>calls, replies y events" --> pipe
    pipe <--> ayud
    ayud -- "ready: capture, clipboard, broadcast, input, files, open, pause" --> pipe
    ayud --> so

    classDef externo fill:#dbeafe,stroke:#1d4ed8,color:#111827
    classDef aula fill:#dcfce7,stroke:#15803d,color:#111827
    classDef datos fill:#ffedd5,stroke:#c2410c,color:#111827
    classDef seguridad fill:#fee2e2,stroke:#b91c1c,color:#111827
    class consola,agente,sup,proxies,pol,ayud aula
    class pipe seguridad
    class so,sesiones externo
```

## Lo que no es obvio

- **El servicio no ve la pantalla.** Corre en la sesión 0, sin ventana: captura, portapapeles,
  distintivo y avisos solo existen en el ayudante, que es un `aula-cliente.exe` con los permisos
  normales del alumno. El servicio los usa a través del canal local (`agentsvc` Clipboard,
  Capture, Indicator).
- **Las capacidades siguen al ayudante.** El `hello` declara solo lo que el ayudante dijo poder
  hacer en su evento `ready`. Sin ayudante (nadie inició sesión, o acaba de cerrarla) el agente
  se reconecta declarando solo `power`; cuando alguien entra, se reconecta con un `hello` nuevo
  con su usuario.
- **Solo el ayudante que lanzó el servicio puede hablarle**: el pipe admite solo a SYSTEM y al
  SID del usuario de la sesión, y el supervisor comprueba que el PID del cliente es el del
  proceso que lanzó (`PeerPID`).
- **El canal local no es el protocolo de la consola**: es JSON por líneas que nunca sale del
  equipo (límite de 24 MiB por línea, por una captura nativa o un archivo de `file.push`).
- **Si el alumno cierra el ayudante**, el supervisor lo vuelve a abrir con retroceso; si cambia
  el usuario, lo mueve; al cerrar sesión lo detiene y las restricciones se quitan.
- **El control remoto inyecta desde el ayudante** (`input.inject`, `input.local`): `SendInput` solo
  llega al escritorio de la sesión que lo llama. El ayudante no está elevado, así que no controla
  ventanas de administrador (UIPI); recuerda lo que quedó presionado y lo suelta si el servicio
  desaparece (diagrama 17).
- **Los archivos del profesor los escribe el ayudante** (`file.write`, `folder.open`): el servicio
  arma y comprueba el archivo (`filepush.Receiver`) y el ayudante lo guarda en `Documentos\Aula`
  del alumno con su token, así que el archivo es del alumno y nunca de SYSTEM. El ayudante vuelve
  a comprobar la ruta y la carpeta de trabajo no sigue enlaces (diagrama 19, `docs/privacidad.md`
  regla 55). Un archivo de 16 MiB viaja en una sola línea del canal (≈ 21 MiB en Base64).
- **Las páginas y las apps las abre el ayudante** (`open.url`, `open.app`): desde SYSTEM se
  abrirían en la sesión 0, donde nadie las ve, y con permisos de más. El servicio valida la orden
  y el ayudante vuelve a aplicar las reglas fijas de `openapp` con las carpetas del alumno
  (Documentos, `%LOCALAPPDATA%`) antes de llamar a `ShellExecuteW` o `CreateProcessW`, sin
  argumentos (`docs/privacidad.md` reglas 68–69). Sin ayudante con `open` en su `ready`, no se
  declara `open.*`.
- **La pausa la muestra el ayudante** (`pause.show`, `pause.yield`, `pause.hide`): el servicio
  no tiene escritorio. El ayudante la quita por sí mismo si pierde el canal local, y si el ayudante
  muere sus ventanas se van con él: nadie queda en pausa (`docs/privacidad.md` regla 10). Sin
  ayudante con `pause` en su `ready`, no se declara `screen.freeze`.
- **Apagar sin `force` con alguien en sesión lo hace el ayudante** (`power.run`): desde SYSTEM,
  Windows lo rechaza con `ERROR_SHUTDOWN_USERS_LOGGED_ON` (diagrama 16).
- Solo Windows; en los demás SO `NewSessionManager` y `ListenLocal` devuelven «no soportado».

## Fuentes en el código

- `internal/agentsvc/supervisor.go`, `internal/agentsvc/link.go`, `internal/agentsvc/proxy.go`
- `internal/platform/service.go`, `internal/platform/service_windows.go`,
  `internal/platform/session_windows.go`, `internal/platform/service_other.go`
- `apps/student/service.go`, `apps/student/helper.go`
- `internal/platform/workfolder_windows.go`, `internal/filepush/receiver.go`
- `internal/agentsvc/proxy_open.go`, `internal/openapp/session.go`, `internal/platform/launcher_windows.go`
- `internal/agentsvc/proxy_pause.go`, `internal/platform/pause_windows.go`
- `docs/privacidad.md` reglas 17–19, `docs/decisiones.md` D-016, 55
