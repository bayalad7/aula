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
  - `internal/remote`: el control remoto de un alumno (`remote.*`): del lado del agente, el
    aviso antes de la entrada, la pausa cuando el alumno usa su equipo y soltar todo al terminar;
    del lado de la consola, uno a la vez y el ritmo de `remote.input`. Inyecta por `platform`.
  - `internal/snapshot`: la captura de la pantalla de un alumno (`screen.snapshot.*`): del lado
    del agente, el aviso antes de capturar y el envío en partes; del lado de la consola, el
    ensamblado, la vista previa en memoria y el guardado solo con «Guardar» en
    `Documentos\Aula\capturas\{alumno}\{fecha}\`. Captura por `platform`.
  - `internal/simclass`: el aula simulada: alumnos simulados dentro de la consola, con el
    protocolo real por un puerto de `127.0.0.1` que no se anuncia, y sus fallas (desconectado,
    lento, no acusa). Siempre marcados y contados aparte de los reales.
  - `internal/manual`: el manual de la consola: lee el Markdown de `docs/v1/administracion` y los
    diagramas de `docs/v1/diagramas` a bloques sin HTML (sanitizado por construcción), lo valida,
    busca y genera el asistente «¿Falta un alumno?» del diagrama 12. El paquete `docs` los embebe
    (`docs.Files()`): `docs/` es la única fuente.
  - `internal/participation`: lo que responden los alumnos en vivo (vista «Participación»): la
    fila de manos levantadas y la bandeja de mensajes de la consola (`Board`), y la mano y los
    mensajes pendientes del agente (`Desk`), con su ritmo. Solo en memoria; nunca registra el
    texto.
  - `internal/classtimer`: el temporizador de ejercicio (`timer.*`): del lado de la consola, el
    vigente, para quién es, qué se mandó y acusó cada sesión y el reenvío al reconectar
    (`Class`); del lado del agente, la cuenta por lo que falta (nunca por la hora), el fin una sola
    vez y lo que muestra la ventana (`Countdown`). Solo informa: nada cambia en el equipo.
  - `internal/filepush`: enviar archivos y plantillas (`file.push`): las reglas de ruta de los dos
    lados (`CheckName`, `CheckDest`, `CheckRel`, sin corregir nunca una ruta); del lado de la
    consola, el envío archivo por archivo con sus acuses (`Pusher`) y las plantillas en
    `Documentos\Aula\plantillas`; del lado del agente, el ensamblado en memoria y el guardado solo
    en `Documentos\Aula` del alumno, con nombre alternativo (`Receiver`, `Inbox`). Escribe por
    `platform.WorkFolder` (en servicio, el ayudante).
  - `internal/collect`: recoger el trabajo de los alumnos (`file.collect.*`): la carpeta acordada
    `recolecciones/<fecha>/<actividad>` (la consola nunca manda rutas); del lado del agente, el
    aviso antes de leer, la lectura solo de esa carpeta, el troceo y lo recogido para la ventana
    (`Responder`); del lado de la consola, de 4 en 4, el ensamblado y el guardado por alumno en
    `Documentos\Aula\recolecciones` con su `aula-recoleccion.json` (`Collector`). Lee por
    `platform.WorkFolderReader` (en servicio, el ayudante).
  - `internal/openapp`: abrir una página o una app permitida en el equipo del alumno (`open.*`):
    las reglas fijas del agente (solo URLs http/https; solo un `.exe` ya instalado en Program
    Files, Windows o `%LOCALAPPDATA%\Programs`, sin argumentos y nunca una herramienta del
    sistema), el `Opener` del agente con su ritmo y el aviso de qué se abrió; del lado de la
    consola, el catálogo de Ajustes › Apps permitidas (`Catalog`, con ejemplos) y los acuses
    (`Tracker`). Abre por `platform.Launcher` (en servicio, el ayudante, que revisa otra vez).
  - `internal/connectcheck`: el comprobador de conexión de la consola: recorre el camino de un
    alumno (anuncio mDNS, puerto, TLS y clave del centro) y lee la red y el firewall por
    `platform`; da las mismas causas que el agente (`session.Cause`).
  - `internal/ui`: el sistema de diseño compartido por las dos apps (tokens CSS con modo claro y
    oscuro, componentes e iconos en un sprite SVG), embebido con `go:embed` y servido a cada
    frontend Wails bajo `sistema/`. Una sola copia: ninguna app guarda la suya. Su prueba de
    contraste (WCAG AA) vive con él.
  - `apps/teacher`, `apps/student`: cada una su `main` y su frontend Wails; solo consumen `internal/*`.
- **`protocol`, `discovery`, `transport`, `session`, `codedist`, `screen`, `broadcast`, `agentsvc`, `policy`, `power`, `remote`, `snapshot`, `simclass`, `manual`, `connectcheck`, `participation`, `classtimer`, `filepush`, `collect`, `openapp`, `ui` NO llaman APIs del SO.** Todo lo
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

## Definición de terminado (toda función nueva o cambio visible)
Vale para cualquier prompt, aunque se haya escrito antes de las etapas 18 y 19. Lo marcado con
(prueba) lo exige `go test`; lo demás, la revisión. Cada prompt de función trae además su sección
«Encaje con lo implementado», con dónde va en cada app y qué diagramas toca.
- **Protocolo:** mensajes nuevos primero en `docs/protocol.md`. Una capacidad implementada
  aparece en la ficha del alumno, «Qué puede hacer su equipo» (prueba). Un código de error nuevo
  lleva texto `errorAlumno.*` y sección del manual en `agentErrorHelp` (prueba).
- **Ubicación y vocabulario de la interfaz** (se aprueba aparte del diseño técnico):
  - Las zonas se nombran como en `docs/v1/ux/MAPA.md` (fuente: `scripts/ux/zonas.mjs`). Una zona
    nueva se agrega a `zonas.mjs` y se regenera el mapa con `node scripts/ux/mapa.mjs`.
  - Una función con **su propio material de trabajo** (archivos, encuestas, recolecciones,
    plantillas…) es un **módulo propio del menú**; no se mete dentro de otro módulo.
  - Una **acción sobre un alumno** aparece en **todos** los lugares donde se ve a un alumno: la
    Ficha del alumno y la Pantalla grande (y la Tarjeta del alumno si tiene menú). No se agrega
    solo en uno.
  - Antes de implementar, el diseño muestra **2–3 bocetos de ubicación** con los nombres del mapa
    y una recomendación; el usuario elige. Ninguna ubicación se da por hecha.
  - Las notas del usuario sobre la interfaz llegan por el anotador (`node scripts/ux/anotar.mjs`,
    en `docs/v1/ux/notas/`) y se trabajan con `/notas-ux`.
- **Consola, dentro de la navegación actual:**
  - Lo que se aplica a alumnos va sobre la selección de Alumnos, con un panel de la barra de
    acciones o en la ficha del alumno. Nada de pantallas a pantalla completa nuevas.
  - Lo que **responden** los alumnos en vivo (manos y mensajes, encuestas, recolecciones,
    asistencia y bitácora, temporizador) va en la vista **«Participación»** del menú (grupo
    Clase), una sección por función. La creó 10.6 (manos y mensajes), con su página del manual
    `participacion.md` (`pantallas: [participacion]`); las siguientes agregan su sección a la
    vista y a esa página. Se lanza desde la barra de acciones y, si conviene, deja una insignia
    en la lista de Alumnos y suma al contador del menú.
  - Lo que sigue activo mientras la clase corre (transmitir, energía pendiente, control remoto,
    temporizador) tiene su franja global arriba del contenido, con la acción para detenerlo.
  - Solo componentes, tokens e iconos de `sistema/`, sin colores sueltos. Funciona en claro y
    oscuro y a 900×600.
- **App del alumno (`apps/student`, prompt 19):**
  - Mismo sistema de diseño (`internal/ui`); textos en `apps/student/frontend/dist/textos.es-MX.js`
    (prueba). **Go manda estados, no frases**: la ventana arma el texto y su título.
  - **No quita el foco.** Lo que llega del profesor (código, avisos, encuestas, archivos) usa
    `notify` y nunca `raise` (prueba): tarjeta en la lista con «Nuevo», aviso en la esquina o
    insignia en la tira; si la ventana está cerrada, solo se marca el icono del área de
    notificación. Solo una acción del profesor **sobre el equipo** (energía, restricciones,
    transmisión, control remoto, pausa) usa `raise`, saca la ventana del modo compacto y lo
    justifica en su regla de privacidad.
  - Lo que el **alumno** hace (levantar la mano, mandar un mensaje, votar) son botones de la tira
    de identidad (también en el modo compacto) o de la tarjeta que llegó, con teclado.
  - **Transparencia:** lo que observa o controla el equipo agrega su banda a `.bandas`, en el
    orden fijo de `docs/privacidad.md`, sin botón de cerrar, y entra en las pruebas de bandas de
    `scripts/ux/pruebas-alumno.mjs`. Todo binding nuevo de la ventana se revisa en
    `TestNoBindingTurnsOffTransparency` (prueba).
- **Textos y errores:**
  - Todo texto en `textos.es-MX.js`, sin `innerHTML` (prueba).
  - Cada error de un binding con código en `errores.go`, mensaje «qué pasó + qué hacer» y su
    sección en `errorHelp` (prueba).
- **Alumnos simulados:** decide y prueba si la función les llega. Lo que controla o cambia un
  equipo (energía, restricciones, transmisión, control remoto…) nunca les llega (prueba).
- **Privacidad:** regla y aviso visible en `docs/privacidad.md` si observa o controla al alumno.
- **Manual y diagramas:**
  - Sección en `docs/v1/administracion/`: en la página de su pantalla o en Tareas frecuentes; una
    pantalla nueva lleva su página con `pantallas` (prueba).
  - Si cambia un flujo, su diagrama en `docs/v1/diagramas/` con front-matter, y sus SVG
    regenerados con `scripts/docs/generar-diagramas` (prueba).
- **Arnés web:**
  - Bindings y eventos en `docs/eventos-ui.md` y en `scripts/ux/dobles.js` (consola) o
    `scripts/ux/dobles-alumno.js` (alumno) (prueba).
  - Una escena y una captura en `scripts/ux/recorrido.mjs` (consola) o
    `scripts/ux/recorrido-alumno.mjs` (alumno), sin errores ni violaciones serias (CI). Lo que
    cambie el comportamiento del alumno, también en `scripts/ux/pruebas-alumno.mjs` (CI).
  - Si cambia una pantalla del manual, `node scripts/ux/capturas-manual.mjs`.
  - Las capturas (arnés, mapa, notas, manual y entregas) se guardan en **WebP**, no PNG.
- **Cierre:** casos manuales en `docs/desarrollo/probar-consola-local.md` (C-xx de la consola,
  A-xx del alumno) y `docs/progreso.md`; lo nativo que el arnés no ve (ventana, icono, SO) queda
  como caso manual, sin clics sintéticos en el escritorio del usuario.

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
- **Job `ux` del CI:** en `ubuntu-latest`, `npm ci` y `node recorrido.mjs` en `scripts/ux/`. Recorre
  la consola en Chromium headless y falla con errores de JS o violaciones serias de axe-core. Sube
  las capturas como artefacto.
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