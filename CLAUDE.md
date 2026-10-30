# Aula: gestión de aula multiplataforma (Go + Wails)

## Qué es
Software para que un maestro de programación monitoree y controle los equipos de su
laboratorio (como LanSchool / Veyon) y, sobre todo, distribuya código que los alumnos reciben
formateado y copian con un clic. Herramienta de aula transparente, no vigilancia oculta.

Dos aplicaciones, mismo monorepo:
- **Profesor** (`apps/teacher`): consola Wails. Descubre, emite snippets, monitorea, controla.
- **Estudiante** (`apps/student`): agente Wails. Recibe código, muestra overlay, responde a comandos.

## Stack (no cambiar sin actualizar este archivo)
- **Go** estable. Concurrencia con goroutines y channels.
- **Wails v2** para ambas apps (backend Go + frontend web; WebView2 en Windows, WebKit en Linux/macOS).
- Red tiempo real: WebSocket con `github.com/coder/websocket` (sucesor oficial de
  `nhooyr.io/websocket`; justificación en `docs/decisiones.md`, D-005).
- Descubrimiento LAN: mDNS/Zeroconf (`github.com/grandcat/zeroconf`), con canal en los TXT records.
- Streaming de pantalla (Fase 2): WebRTC (`github.com/pion/webrtc`).
- Captura: `github.com/kbinani/screenshot`. Portapapeles: `golang.design/x/clipboard`.
- Servicio de Windows: `golang.org/x/sys/windows/svc` y `.../svc/mgr`; canal local con el ayudante
  por named pipe con `github.com/Microsoft/go-winio` (D-016).
- Serialización del protocolo: JSON (`encoding/json`).
- Editor en el cliente: Monaco Editor (fallback Prism.js si Monaco pesa demasiado en el overlay).

## Arquitectura (regla de oro)
- Monorepo, un solo módulo Go en la raíz.
  - `internal/protocol`: tipos de mensajes y (de)serialización. Sin red, sin SO.
  - `internal/discovery`: anuncio y descubrimiento mDNS con canales.
  - `internal/transport`: servidor y cliente WebSocket, TLS.
  - `internal/session`: registro de sesiones y ciclo de vida.
  - `internal/security`: claves Ed25519, reto/firma de autenticación.
  - `internal/platform`: funciones que tocan el SO (captura, portapapeles, overlay,
    servicio, hotkey) detrás de interfaces Go, con archivos `*_windows.go`, `*_linux.go`,
    `*_darwin.go` por build constraints y un fallback que devuelve "no soportado".
  - `internal/codedist`: captura y distribución de snippets de código.
  - `internal/screen`: monitoreo de pantalla (miniaturas y pantalla en blanco) del lado del agente
    y de la consola, sobre las interfaces de `platform`.
  - `internal/broadcast`: transmisión de la pantalla del profesor (WebRTC con pion): captura y
    codificación por `platform`, una conexión por alumno y la señalización `screen.broadcast.*`.
  - `internal/agentsvc`: el agente como servicio: supervisor del ayudante en la sesión del alumno,
    canal local servicio ↔ ayudante y *proxies* de portapapeles, captura y distintivo.
  - `internal/policy`: restricciones de la clase (bloqueo de apps y filtro web por DNS local),
    su diario para deshacerlas siempre, y la parte de la consola (quién tiene qué).
  - `internal/power`: apagar, reiniciar y cerrar sesión (`power.*`): la cuenta regresiva del
    agente con su aviso y el posponer, y el registro de órdenes de la consola.
  - `apps/teacher`, `apps/student`: cada una su `main` y su frontend Wails; solo consumen `internal/*`.
- **`protocol`, `discovery`, `transport`, `session`, `codedist`, `screen`, `broadcast`, `agentsvc`, `policy`, `power` NO llaman APIs del SO.** Todo lo
  que toca pantalla, teclado, portapapeles, overlay o servicios pasa por una interfaz de
  `internal/platform`.
- El profesor anuncia su sesión por mDNS con el canal en los metadatos. **Los alumnos se
  conectan al profesor** por WebSocket sobre TLS, nunca al revés.
- Un agente solo acepta órdenes de una consola autenticada con la clave Ed25519 cuya pública
  está en su configuración: la **clave del centro**, una sola por centro de cómputo, que el
  administrador instala en las consolas. No hay claves por maestro.

## Canales / Aulas
Modelo tipo LanSchool. Un canal es un **número del 1 al 16000** que agrupa salas y maestros, no
un aula virtual. Cada computadora de alumno se instala con **su canal fijo** (p. ej. centro de
cómputo principal = 1, alterno = 2), que solo un administrador cambia. La consola se instala con
un canal predeterminado, arranca directo en él y el maestro puede cambiar de canal. El alumno
filtra por canal al descubrir y solo se conecta al que coincide. Varias salas coexisten en la
misma LAN.

## Convenciones
- Código, identificadores y comentarios técnicos en inglés; interfaz y documentación en español.
- Errores envueltos con `fmt.Errorf("...: %w", err)`; nada de `panic` fuera de `main`/arranque.
- `context.Context` en toda operación de red y en las goroutines de larga vida (cancelación limpia).
- Logs con `log/slog`; nunca registrar código de alumnos, contenido de pantalla ni claves.
- Cada mensaje nuevo del protocolo se documenta primero en `docs/protocol.md`.
- Cambios pequeños e incrementales; no refactorizar paquetes ajenos a la tarea.
- Antes de terminar: `gofmt -l` (sin salida), `go vet ./...`, `staticcheck ./...`, `go test -race ./...`.

## Integración continua (GitHub Actions)
- Remoto: `origin` = `https://github.com/bayalad7/aula.git`.
- Acciones ancladas a versión mayor (`actions/checkout@v7`, `actions/setup-go@v7`,
  `actions/setup-node@v7`, `upload/download-artifact@v7/@v8`). No usar versiones flotantes.
- `setup-go` con `go-version: '1.26.x'` y `check-latest: true` (último parche, no el mínimo de
  `go.mod`): los binarios llevan los parches de seguridad de la stdlib.
- **Workflow de CI** (`.github/workflows/ci.yml`): se dispara en `push` a `main`/`develop` y en
  `pull_request`; `permissions: contents: read`; matriz `ubuntu-latest`, `windows-latest`,
  `macos-latest`. Pasos: `go vet ./...`, `staticcheck` (vía
  `go run honnef.co/go/tools/cmd/staticcheck@<versión fija> ./...`), `go test -race ./...`
  (con `CGO_ENABLED=1`, que `-race` requiere; los binarios publicados se compilan sin cgo).
- **Workflow de release** (`.github/workflows/release.yml`): se dispara al empujar un tag `v*`.
  Compila todas las plataformas, genera los instaladores, produce `checksums.txt` firmado con
  una llave Ed25519 de releases (secreto del repo) y publica con `gh release create`
  (prerelease si el tag lleva guion, p. ej. `v1.2.0-rc.1`). La llave PÚBLICA de releases se
  versiona en el repo (`empaquetado/llave-releases.pub`) y los binarios la embeben para que la
  auto-actualización verifique las firmas (ver etapa 15).
- Secretos solo en el workflow de release y todos opcionales salvo la llave de releases; la firma
  de código de Windows y la de macOS se activan solo si el secreto existe.

## Control de versiones (estricto)
- **Solo dos ramas: `develop` y `main`.** No se crean ramas de feature ni de ningún otro tipo.
  - `develop`: integración y trabajo diario. Todo commit de desarrollo va aquí.
  - `main`: solo releases estables y probados, traídos desde `develop`. Nunca se commitea directo a `main`.
- **Versionado semántico `x.y.z` (SemVer).** Cada release a `main` se etiqueta `vX.Y.Z`:
  - `z` (patch): correcciones que no cambian el comportamiento externo.
  - `y` (minor): funciones nuevas compatibles hacia atrás.
  - `x` (major): cambios incompatibles, incluidos cambios de ruptura en el protocolo o en el
    formato de configuración.
- La versión del agente que viaja en `hello` es esta misma `x.y.z`.

## Plataformas
Prioridad: Windows 10/11 → Linux (X11) → macOS. Toda función que toca el SO define primero su
interfaz en `platform` y una implementación por defecto que devuelve "no soportado".

## Privacidad (no negociable)
Monitoreo transparente: cuando la pantalla del alumno se ve o transmite, su equipo muestra un
indicador visible. El agente se identifica siempre; sin modo oculto ni evasión de detección.
No se guardan capturas en disco salvo petición explícita del profesor. Reglas y textos de aviso
en `docs/privacidad.md`.

## Progreso
Estado actual en `docs/progreso.md`. Actualízalo al final de cada sesión.