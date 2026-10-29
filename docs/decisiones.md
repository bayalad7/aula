# Decisiones de arquitectura

Registro breve de decisiones. Cada entrada incluye contexto, opciones, decisión y consecuencias.

## D-001: Dos apps Wails en un mismo módulo Go

**Fecha:** 2026-10-23 · **Etapa:** 02

**Contexto.** Necesitamos dos ejecutables (consola del profesor y agente del alumno) que
comparten `internal/*`, en un monorepo con un solo `go.mod` en la raíz.

**Opciones evaluadas.**

| | (a) Dos directorios de app bajo `apps/` | (b) Un binario con build tags `teacher` / `student` |
|---|---|---|
| Estructura | `apps/teacher` y `apps/student`, cada uno con `main.go`, `wails.json` y `frontend/` | Un `main` y dos frontends elegidos con `//go:build teacher` / `student` |
| `wails dev` / `wails build` | Funcionan tal cual dentro de cada directorio | Wails espera un `wails.json` y un frontend por proyecto: hay que pasar `-tags` y alternar config y frontend a mano |
| `go build ./...`, `go vet`, `staticcheck` | Compilan y analizan ambas apps en una sola pasada | Con los tags por defecto solo se analiza una variante; hay que correr todo dos veces |
| Riesgo de fuga | El agente no puede importar por accidente código de la consola (son `package main` distintos) | Un tag mal puesto mete código del profesor en el binario del alumno |
| Empaquetado | Nombre, icono, manifiesto e instalador propios por app (el `build/` de cada una) | Hay que generar metadatos distintos según el tag |
| Duplicación | Algo de boilerplate repetido en `main.go` | Mínima |

**Decisión: (a).** Dos directorios independientes bajo `apps/` comparten `internal/` y el
`go.mod` de la raíz. Es el modelo que Wails soporta de forma nativa, mantiene ambas apps
visibles para todas las herramientas de Go y separa físicamente el código que corre en el
equipo del alumno del de la consola. Esto importa por privacidad y seguridad.

**Consecuencias.**
- Cada app se desarrolla y compila desde su directorio (`cd apps/teacher && wails dev`).
- La lógica compartida va en `internal/*`; `apps/*` solo la consume y la conecta.
- En Linux, `go vet ./...` necesita las dependencias de Wails (GTK/WebKitGTK), también en CI.

## D-002: Frontend sin framework ni bundler

**Fecha:** 2026-10-23 · **Etapa:** 02

HTML/CSS/JS sin framework, servido directamente desde `frontend/dist` (`frontend:build` vacío
en `wails.json`). No hay `node_modules` ni paso de build, y el overlay del alumno queda ligero
para cuando se integre Monaco. Si la UI de la consola crece, se reevalúa; Svelte sería la
primera opción.

## D-003: Versión mínima de Go en `go.mod`

**Fecha:** 2026-10-23 · **Etapa:** 02

`go.mod` declara `go 1.26.0`, el mínimo que garantiza CI (`1.26.x`, último parche). Los equipos
de desarrollo pueden usar un Go más nuevo sin forzar a CI a descargar otro toolchain.

## D-004: Deserialización del protocolo en dos pasos

**Fecha:** 2026-10-23 · **Etapa:** 03

**Opciones.** (1) Exponer `Payload json.RawMessage` en `Envelope` y que cada consumidor haga
`json.Unmarshal` según `Type`; (2) deserializar en dos pasos dentro de `protocol`: primero la
envoltura con el payload como `json.RawMessage` *interno*, se valida `v` y `type`, y luego se
decodifica el payload en su tipo concreto mediante un registro `Type → decodificador`.

**Decisión: (2).** `Decode` devuelve `Envelope.Payload` como un valor concreto
(`CodeShare`, `HelloInfo`…) ya validado contra los límites de `docs/protocol.md`, y quien llama
hace un `switch p := e.Payload.(type)`. La validación vive en un solo lugar, en lugar de
repetirse en `transport`, `session` y las apps. `json.RawMessage` se usa solo como paso
intermedio. Agregar un mensaje implica un tipo, sus métodos `MessageType`/`Validate` y una
línea en el registro; la prueba de ida y vuelta falla si falta el ejemplo.

**Detalles.**
- Errores como centinelas (`ErrUnsupportedVersion`, `ErrInvalidMessage`, `ErrMessageTooLarge`,
  `ErrUnknownType`) envueltos con `%w`; `ErrorCodeFor` los traduce al código del mensaje `error`.
- Ante un error, `Decode` devuelve los campos de cabecera que pudo leer, para responder con
  `ref_id`. El `id` solo se devuelve si es un UUID v4 válido, así nunca se refleja contenido
  arbitrario del emisor.
- `Encode` no escapa `<`, `>` ni `&` (a diferencia de `json.Marshal`): el código está lleno de
  esos caracteres y escaparlos multiplicaría por 6 su tamaño.
- UUID v4 propio (`crypto/rand`, 15 líneas) en lugar de una dependencia externa.

## D-005: Librería de WebSocket

**Fecha:** 2026-10-23 · **Etapa:** 04

**Opciones.** CLAUDE.md proponía `nhooyr.io/websocket` o `gorilla/websocket`. `nhooyr.io/websocket`
está archivada: su mantenimiento pasó a `github.com/coder/websocket` (misma API, mismo autor
original).

**Decisión: `github.com/coder/websocket` v1.8.15.**
- Todas las operaciones reciben `context.Context`, como exige CLAUDE.md para la red; con
  gorilla hay que traducir cancelaciones a `SetReadDeadline` a mano.
- `SetReadLimit` cierra con 1009 al pasar 1 MiB, que es justo el límite del protocolo.
- Cierre con handshake (`Close(code, reason)`) y códigos tipados, que el protocolo usa (§1.2).
- Comprueba `Origin` por defecto: una página web abierta en el navegador del alumno no puede
  abrir un socket hacia la consola (CSWSH).

Aunque la librería admite escrituras concurrentes, `transport` usa una sola goroutine
escritora por conexión con una cola: así se garantiza el orden (un `error` sale antes del
frame de cierre) y una sola goroutine es dueña del socket.

## D-006: Descubrimiento con `grandcat/zeroconf` y búsqueda periódica

**Fecha:** 2026-10-23 · **Etapa:** 04

`zeroconf` v1.0.0 (2019) deja de preguntar en cuanto recibe la primera respuesta y entrega
cada anuncio una sola vez. Una consola encendida después no se vería nunca. Por eso
`discovery.Browser` reinicia la búsqueda cada 30 s (`RequeryInterval`) y deduplica por
instancia: solo reemplaza una consola si cambian sus direcciones.
- La dependencia transitiva `miekg/dns` se fija en v1.1.73 (zeroconf pedía v1.1.27, de 2020).
- Equipos con varias interfaces: se anuncia en todas las interfaces con multicast y el
  alumno ordena las direcciones recibidas. Primero van las de sus propias subredes, luego
  IPv4 antes que IPv6, y al final las link-local. Las IPv6 link-local se descartan porque mDNS
  no da la zona.
- El nombre de instancia DNS-SD es ASCII seguro más un sufijo aleatorio; el nombre visible
  viaja en el TXT `console_name`.
- Si la red bloquea multicast, `session.StaticSource` permite conectar a una dirección fija.

## D-007: TLS, identidad y almacenamiento de claves

**Fecha:** 2026-10-28 · **Etapa:** 05

- **Dos claves separadas.** La identidad Ed25519 firma `auth.proof` y es lo que se configura en
  los agentes. El certificado TLS (ECDSA P-256 autofirmado, 10 años) solo cifra. Así se puede
  regenerar el certificado sin tocar los agentes. ECDSA en lugar de Ed25519 para el certificado
  por compatibilidad con cualquier pila TLS; ambos lados son Go, pero no cuesta nada.
- **TLS 1.3 como mínimo** en servidor y cliente; sin HTTP/2 ni proxy en el cliente.
- **Verificación por huella, no por CA.** `ClientTLS` desactiva la validación de cadena y la
  reemplaza por `VerifyConnection`, que compara el SHA-256 del certificado. Además guarda ese hash
  para la firma.
- **Conexión directa sin huella** (`-directo`): se acepta el certificado, pero la firma va ligada a
  la huella vista, así que un intermediario sigue sin poder autenticarse. La prueba
  `TestRelayWithOwnCertificateDetected` lo demuestra con un relay fiel. Una mutación que quita la
  huella del mensaje firmado hace que el relay pase, lo que confirma que la prueba detecta el
  ataque.
- **Clave privada en disco** en `%AppData%\Aula\consola\identidad-ed25519.pem` (PKCS#8). En Windows
  `0600` no protege nada, así que `platform.RestrictToOwner` aplica una DACL protegida con una sola
  entrada para el usuario actual (verificado con `icacls`). En Linux/macOS usa `0600`/`0700`, y
  `LoadIdentity` rechaza claves legibles por otros, como ssh. Se escribe en un temporal ya
  restringido y se renombra, para que nunca exista legible por otros.
- **La clave no se puede filtrar por accidente.** `security.Identity` no expone la clave privada, y
  `String`, `GoString`, `Format` y `LogValue` (con receptor por valor, para cubrir también copias)
  muestran solo la huella pública. Una prueba lo encontró: con receptor puntero, `fmt.Sprint(*id)`
  imprimía los bytes de la clave.
- **`aula-consola keygen` en una app de ventana.** `wails build` produce un ejecutable sin
  consola; `platform.AttachParentConsole` lo conecta a la terminal que lo lanzó para que los
  subcomandos impriman.

## D-008: Visualización de código con Prism.js y portapapeles nativo

**Fecha:** 2026-10-28 · **Etapa:** 06

**Editor: Prism.js en lugar de Monaco.** Monaco pesa unos 13 MB solo en `min/vs`, necesita
*web workers* y un cargador AMD, y el frontend no tiene bundler (D-002). En el overlay el código
es de solo lectura, así que Monaco no aporta nada a cambio de ese peso. Prism.js 1.30.0 está
incluido en `apps/student/frontend/dist/vendor/prism/` (unos 80 KB con 16 lenguajes, licencia MIT
incluida), sin CDN, porque los laboratorios pueden no tener internet. Los colores son propios
(claro y oscuro) en lugar de un tema de Prism. Si más adelante el alumno edita código (entregas,
etapa 14), se reevalúa Monaco.

**Portapapeles desde Go** (`golang.design/x/clipboard` v0.11, vía `platform.Clipboard`), no
`navigator.clipboard`: la API del navegador exige contexto seguro y gesto del usuario. La v0.11 no
necesita cgo en Linux (X11/Wayland en Go puro), y donde no hay portapapeles `Init` devuelve
`ErrUnavailable`, que se traduce a `platform.ErrNotSupported`.
- En Windows el texto se copia con CRLF, el formato de texto del portapapeles. Algunos editores
  juntan las líneas si reciben solo LF.
- `App.Copy` recibe el ID del fragmento y copia el texto guardado en Go, no lo que muestra la
  página.

**Eventos de Wails con el estado completo** (no cambios incrementales), con el contrato en
`apps/student/events.go` y `docs/eventos-ui.md`. Redibujar una lista de a lo sumo 20 fragmentos
es barato y evita que la interfaz y Go se desincronicen.

## D-009: Consola del profesor: Monaco, atajo global y conteo de acuses

**Fecha:** 2026-10-28 · **Etapa:** 07

**Monaco en la consola (no en el alumno).** El profesor sí edita código, así que aquí Monaco
aporta lo que Prism no tiene: edición con sangría automática, espacios visibles y deshacer. Se
incluye un **subconjunto** de monaco-editor 0.52.2 (MIT) en
`apps/teacher/frontend/dist/vendor/monaco/`, sin bundler (cargador AMD):
- núcleo (`loader.js`, `editor/`, `base/worker`, fuente de íconos) y 13 gramáticas de
  `basic-languages`: unos **4,3 MB de 14 MB**;
- **fuera** los servicios de lenguaje (`vs/language/*`, 7 MB, TypeScript solo 5,6 MB) y las
  traducciones. El resaltado viene de `basic-languages`; sin servicios no hay autocompletado ni
  diagnósticos, que en clase no hacen falta. Verificado en la app: Monaco carga y colorea
  Python.
- CSP de la consola: `style-src 'unsafe-inline'` (Monaco crea estilos), `worker-src 'self' blob:`
  y `font-src 'self'`. Scripts siguen siendo solo `'self'`.

**Atajo global `Ctrl+Alt+Shift+C`** (`platform.Hotkeys`, `RegisterHotKey` con un hilo de mensajes
propio en Windows; en Linux/macOS devuelve `ErrNotSupported` y queda el botón flotante). Se evita
Ctrl+Alt solo: en teclados en español es AltGr y escribe `@`, `#`, `[`. Si otra aplicación ya lo
usa, la consola lo explica y sigue con el botón. Capturar **nunca emite**: carga el editor y
propone un lenguaje (heurística simple, editable).

**Acuses por ID de sobre.** `Console.Broadcast` ahora devuelve el ID del sobre enviado a cada
sesión. `codedist.Deliveries` cuenta los `code.ack` y `msg.ack` cuyo `ref_id` coincide, una vez
por sobre. El envío y el registro de sus IDs ocurren bajo un mismo candado que también toma el
manejador de acuses, así que un alumno muy rápido nunca responde a un envío "desconocido".

**Estado en Go, interfaz pasiva.** Alumnos, historial y confirmaciones viven en `App`; la interfaz
recibe eventos con el estado completo. `session.Console.OnChange` avisa cuando cambia el registro.

**Pruebas sin tocar el escritorio.** La prueba de pulsación real del atajo (`SendInput`) solo corre
con `AULA_HOTKEY_TEST=1`. Las pruebas de la interfaz se hacen con dobles y con la red real
(`TestLiveClassTwoAgentsAck`), no simulando clics.

## D-010: Configuración, instaladores y firma de versiones (Hito 1)

**Fecha:** 2026-10-28 · **Etapa:** 08

- **Configuración del agente de máquina primero.** El instalador escribe
  `C:\ProgramData\Aula\alumno\config.json` y la protege con `icacls` (sin herencia: lectura para
  Usuarios, control total para Administradores y SYSTEM). El agente la prefiere sobre la del
  usuario (`%AppData%`). Si fuera al revés, un alumno podría dejar un archivo propio y conectar el
  agente que arranca con su sesión a otra consola. El formato vive en un solo lugar
  (`session.AgentConfig`): la consola lo genera y el agente lo lee. Formato en
  `docs/configuracion.md`.
- **La consola genera `aula-alumno.json`** (botón o `aula-consola config-alumno`). El instalador
  lo toma de junto al setup o de `/CONFIG=`, así nadie edita JSON a mano en 30 equipos.
- **Inno Setup**, como en otros proyectos del autor. Instalación por máquina (`admin`, Program
  Files), arranque al iniciar sesión con la clave `Run` de HKLM (el servicio llega en la Fase 2) e
  instalación silenciosa para GPO. Las reglas de firewall son **por programa** y solo para los
  perfiles Privado y Dominio. `CloseApplications` cierra el agente al actualizar.
- **Instancia única** (`SingleInstanceLock` de Wails) en ambas apps: el arranque automático más
  un arranque manual no crean dos agentes, y una segunda consola no pelea por el puerto.
- **Firma de versiones con Ed25519 propia** (`tools/relsign`, paquete `empaquetado`) en lugar de
  GPG o cosign: es la misma primitiva que ya usa el protocolo, no requiere herramientas externas
  para verificar y el paquete `empaquetado` **embebe** la llave pública para la auto-actualización
  (etapa 15). La firma lleva la etiqueta `aula-checksums-v1`. La llave privada se generó con
  `relsign keygen` directo a `gh secret set` y no existe en disco.
- **Versión inyectada por `-ldflags -X main.version`** y verificada en el workflow ejecutando los
  binarios. Los metadatos del `.exe` usan la parte numérica (`0.1.0` de `0.1.0-rc.1`), porque
  Windows no admite sufijos ahí.
- **Robustez:** `TestTwelveAgentsAndConsoleRestart` (red real, 12 agentes, reinicio de la consola
  en el mismo puerto) y el ejemplo `examples/muchos_alumnos`, que en este equipo sostuvo 15
  agentes y su reconexión.

## D-011: Canales tipo LanSchool y clave del centro (v0.2.0)

**Fecha:** 2026-10-29 · **Etapa:** 08 (revisión tras la prueba real)

**Contexto.** En la prueba con la rc.1 y la rc.2, el modelo "un canal y una clave por maestro,
con `aula-alumno.json` generado por ese maestro" resultó ajeno a cómo se opera un centro de
cómputo. El usuario espera el modelo LanSchool: cada computadora de alumno tiene su canal fijo
(sala principal = 1, alterna = 2), lo fija el administrador al instalarla, y la consola arranca
en un canal predeterminado que el maestro puede cambiar. No se pasan configuraciones por maestro.

**Opciones evaluadas.**
1. **Solo canal, sin autenticación** (LanSchool clásico): cualquier consola del canal controla a
   sus alumnos. Un alumno que instale la consola podría enviar código a sus compañeros y, desde
   la etapa 09, ver sus pantallas. Rompe la regla no negociable de CLAUDE.md.
2. **Solo canal más contraseña opcional:** dos modos que mantener, y la contraseña tiende a
   quedarse sin activar.
3. **Clave del centro (elegida):** una sola pareja Ed25519 por centro, instalada por el
   administrador en las consolas. Los alumnos llevan la pública y su canal. Los maestros cambian
   de canal libremente y nadie distribuye nada por clase.

**Decisión: (3)**, con canales **numéricos 1–16000** como en LanSchool.
- El protocolo en la red no cambia: `auth.proof` sigue firmado con Ed25519 y ligado al canal y al
  certificado. Solo cambian el formato de `channel` (§1.1) y de quién es la clave (§10.1).
- La clave privada vive en `ProgramData` con lectura para los usuarios del equipo del profesor,
  para que cualquier maestro de ese equipo pueda dar clase. Riesgo aceptado y documentado en
  §10.1: los equipos de profesor son de profesores.
- **Versión 0.2.0**, un cambio incompatible: los canales `AULA_101` ya no son válidos y los
  alumnos de la rc.1 y la rc.2 se reinstalan. Aceptable en 0.x (SemVer).
- Se elimina el flujo por maestro: el botón "Configuración de alumnos", `SaveAgentConfig` y
  `config-alumno`.
