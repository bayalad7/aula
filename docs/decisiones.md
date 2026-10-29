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

## D-012: Redes Públicas solo como opción explícita del instalador

**Fecha:** 2026-10-29 · **Etapa:** 08 (prueba real de la v0.2.0-rc.1)

**Contexto.** En la prueba real los dos equipos estaban en una red con perfil **Público**. Las
reglas de firewall de los instaladores solo cubren Privado y Dominio, así que Windows descartaba
el mDNS y el alumno decía "No se encontró ninguna consola del canal 1". Hay laboratorios donde
TI no deja cambiar el perfil de la red.

**Decisión.** Los dos instaladores ofrecen **"Permitir también en redes Públicas"**, desmarcada
por defecto (silencioso: `/MERGETASKS="redpublica"`). Al marcarla, la regla de firewall de Aula
incluye el perfil Público.
- No se activa por defecto: el perfil Público es el de redes no confiables (cafeterías, hoteles).
  Ahí cualquiera vería la consola anunciada y podría conectarse a ella como si fuera un alumno y
  recibir el código emitido. Tampoco conviene que las laptops de los alumnos respondan a mDNS
  fuera de la escuela. La autenticación sigue impidiendo que una consola ajena controle a los
  alumnos.
- Lo recomendado sigue siendo marcar la red del laboratorio como Privada (o Dominio), a mano o
  por GPO.
- Inno Setup recuerda la opción al actualizar; para quitarla, `/MERGETASKS="!redpublica"`.

## D-013: Monitoreo de pantalla en Windows: miniaturas por WebSocket y freeze nativo

**Fecha:** 2026-10-29 · **Etapa:** 09 (Módulo B)

**Contexto.** El profesor necesita ver las pantallas de los alumnos en miniatura, ampliar una y
poner las pantallas en blanco. La regla de privacidad exige que el alumno vea siempre que su
pantalla se está viendo.

**Decisiones** (elegidas por el usuario donde había alternativas):
- **Transporte:** cuadros JPEG dentro de la misma conexión WebSocket/TLS (`screen.thumb.frame`),
  como máximo 2 por segundo. Una miniatura de 320 px pesa unos 10–25 KiB; con 20 alumnos a 1
  cuadro/s son menos de 0,5 MiB/s. WebRTC (`pion/webrtc`) queda para la transmisión de la
  pantalla del profesor, donde sí hace falta video continuo.
- **Qué se captura:** solo el **monitor principal**, como LanSchool (miniaturas con proporción
  normal).
- **Vista en detalle:** el mismo mensaje `screen.thumb.start` con `max_width` 1280 a un solo
  agente; no hay un segundo flujo.
- **Indicador:** un **distintivo nativo** de Win32, siempre encima y que no recibe clics, en la
  parte superior del monitor principal, más el cambio de la barra de la ventana de Aula. Se ve
  aunque la ventana esté minimizada.
- **Pantalla en blanco:** **ventana nativa siempre encima en todos los monitores**, que vuelve a
  quedar encima y pide el foco cada medio segundo. **No intercepta el teclado:** el alumno podría
  hacer Alt+Tab y escribir a ciegas en otra ventana; es un límite aceptado. Bloquear también el
  teclado queda fuera de Aula (por ejemplo, con directivas de Windows que administre TI). Se
  descartó la ventana de Wails: cubre un solo monitor y se puede minimizar.
- **Seguridad del alumno:** al cerrarse la conexión el agente quita la pantalla en blanco y el
  indicador por sí mismo.
- **Paquete nuevo `internal/screen`** (sin APIs del SO): la lógica del agente (transmisión,
  indicador, freeze) y de la consola (qué equipos transmiten, último cuadro, estado de freeze),
  probada con dobles de las interfaces de `platform`.
- **Actualización (2026-10-29):** la pantalla en blanco se **retiró del código** y queda
  documentada como pendiente (`docs/protocol.md` §5.9–5.11). Esta etapa entrega solo las
  miniaturas, la vista en detalle y el indicador.
- Linux y macOS devuelven "no soportado" y no declaran las capacidades `screen.thumb` ni
  `screen.freeze`. Versión **0.3.0** (función nueva compatible).

## D-014: Transmisión de la pantalla del profesor: un codificador repartido a N conexiones

**Fecha:** 2026-10-29 · **Estado:** aprobada por el usuario (códec con Media Foundation, el alumno
puede minimizar, la consola siempre excluida de la captura, UDP 47831) e implementada en v0.4.

**Contexto.** El profesor transmite un monitor suyo a hasta ~40 alumnos de la LAN, y el código
en pantalla debe leerse nítido. Video por WebRTC (`pion/webrtc`), señalización
`screen.broadcast.*` por el WebSocket (`docs/protocol.md` §5.12).

**Malla o SFU en el proceso del profesor.** Un SFU ahorra subida porque el emisor sube una vez y
el SFU reparte; si el SFU vive **en el mismo proceso** que el emisor, la subida de la máquina
del profesor sigue siendo N × tasa. La comparación real es entre:

| | Malla clásica (un codificador por alumno) | **Un codificador, N conexiones** (lo que es un SFU dentro del proceso) |
|---|---|---|
| CPU de codificación | N × (1080p15 en software ≈ 20–40 % de un núcleo) → inviable desde ~4 alumnos | 1 × codificación, constante. Con codificador por hardware, casi nula |
| CPU por alumno | toda la codificación | solo cifrar SRTP y enviar (~250 paquetes/s a 2,5 Mbit/s): fracción de % por alumno |
| Subida del profesor | N × tasa | N × tasa (igual) |
| Adaptación por alumno | sí | no: una sola tasa (aceptable en LAN conmutada) |
| Cuadro clave que pide un alumno | solo a él | a todos → se limita a 1/s y hay uno periódico cada 5 s |

En pion es natural: un solo `TrackLocalStaticSample` agregado a N `PeerConnection`; cada
`WriteSample` empaqueta una vez y cifra por receptor.

Estimación de subida con tope de 2,5 Mbit/s (código casi estático ≈ 0,2–0,5 Mbit/s; al
desplazar o cambiar de ventana se llega al tope):

| Alumnos | Promedio | Pico |
|---|---|---|
| 10 | 2–5 Mbit/s | 25 Mbit/s |
| 20 | 4–10 Mbit/s | 50 Mbit/s |
| 40 | 8–20 Mbit/s | 100 Mbit/s |

CPU estimada del profesor antes de implementar: captura GDI de 1080p a 15 cuadros/s ≈ 15–20 % de
un núcleo, conversión a NV12 ≈ 5 %, codificación ≈ 0–40 % (hardware o software) y reparto a 40
alumnos ≈ 5 %. Total: menos de un núcleo. **La medición real** está en "Resultado" al final.

**Umbral para cambiar de estrategia.** El reparto deja de bastar cuando la **subida pico**
(N × tope) supera el **50 % del enlace del profesor**, o cuando su CPU medida pasa del 50 %:

- Enlace de 1 Gbit/s: hasta ~200 alumnos a 2,5 Mbit/s. Para 40 sobra.
- Enlace de 100 Mbit/s: hasta 20 alumnos a 2,5 Mbit/s. Con más, se baja el tope a
  `50 Mbit/s / N` (40 alumnos → 1,25 Mbit/s; nunca menos de 0,8 Mbit/s, donde el texto estático
  sigue nítido y solo el movimiento se degrada).
- Más allá (o si la medición lo pide): un SFU **en otra máquina** del laboratorio o un árbol de
  reenvío; fuera del alcance de esta etapa. Simulcast no hace falta en una LAN conmutada.

El tope se configura en `C:\ProgramData\Aula\consola\config.json` (`broadcast_max_kbps`,
predeterminado 2500).

**Calidad para leer código.**
- **Resolución nativa** del monitor hasta 1920 px de ancho; si es mayor, se reduce a 1920
  conservando la proporción. Escalar 1080p a 1280 px emborrona una fuente monoespaciada de 14 px,
  así que no se hace por defecto.
- **15 cuadros/s** como máximo: suficiente para escribir y desplazar; la pantalla quieta cuesta
  casi nada (cuadros P mínimos).
- Cuadro clave cada 5 s y ante PLI/FIR, limitado a 1/s para todos.
- En el alumno, el video se ajusta a la ventana con un botón "Tamaño real" (1:1, con
  desplazamiento) para monitores más chicos que el del profesor. Se recomienda al profesor
  aumentar la fuente del editor durante la transmisión.

**Códec: H.264 Constrained Baseline con Media Foundation.** pion no trae codificadores. Opciones:
- **H.264 con el codificador de Media Foundation de Windows** (MFT de hardware Intel/NVIDIA/AMD si
  existe, si no el de software de Microsoft), llamado por COM sin cgo, dentro de
  `internal/platform` (`*_windows.go`). Mantiene los binarios sin cgo. WebView2 lo decodifica, y
  H.264 CB es obligatorio en WebRTC. **Riesgo:** las ediciones Windows "N" sin el Media Feature
  Pack no tienen H.264 (ni para codificar ni en WebView2). El agente lo detecta y no declara la
  capacidad; la consola deshabilita el botón si no hay codificador.
- VP8 con libvpx: decodifica en cualquier WebView2, pero exige **cgo** en la consola publicada
  (contra la regla de CLAUDE.md y con libvpx en el CI). Queda como alternativa.

**Red.**
- Sin STUN ni TURN; solo candidatos `host`.
- La consola es **ICE-lite** y usa un único puerto UDP (`ws_port + 1`, 47831) con `UDPMux`:
  - la regla de firewall del instalador de la consola es por programa y ya lo cubre (no hizo
    falta una regla nueva);
  - el alumno siempre inicia las comprobaciones, así que su firewall no ve UDP entrante no
    solicitado. Importa porque el RTCPeerConnection del alumno corre en `msedgewebview2.exe`, no
    en `aula-cliente.exe`.
- WebView2 oculta las IP locales con candidatos `.local`. Con ICE-lite no importa: la consola
  aprende la dirección del alumno de sus comprobaciones (candidato peer-reflexive).

**Dónde vive cada cosa.**
- `internal/platform`: `Displays()` (monitores), captura continua de un monitor y
  `VideoEncoder` (Media Foundation), con fallback "no soportado".
- `internal/broadcast` (sin APIs del SO):
  - en la consola, el emisor: pion, el track compartido, una conexión por sesión, reintentos y
    conteo de receptores;
  - en el agente, la máquina de estados de la señalización: validar el SDP, intentos, mostrar
    el aviso antes de responder.
  - Se prueba con un doble de `PeerConnection`.
- Alumno: el `RTCPeerConnection` corre en el JavaScript del frontend Wails y pinta el `<video>`.
  Go solo valida y reenvía la señalización. El agente declara `screen.broadcast` solo si el
  frontend confirma `RTCPeerConnection` y decodificador H.264 (sondeo al arrancar, antes de
  `hello`, con un máximo de 5 s).

Versión: función nueva compatible → sube `y`.

**Resultado de la implementación (2026-10-29).**
- El codificador es el **MFT de software síncrono** de Windows (`H264 Encoder MFT`), llamado por
  COM desde un hilo del SO dedicado (MTA). Entrega Constrained Baseline nivel 4.0 (SPS
  `profile_idc=66`, `constraint_set1`); se antepone SPS/PPS a cada IDR que no los traiga. Los
  MFT de **hardware** (AMD/Intel/NVIDIA) son asíncronos y quedan pendientes: bajarían la CPU.
- Medido en el equipo de desarrollo (AMD Ryzen 5 3450U, 4 núcleos y 8 hilos, 1080p, un
  receptor WebView2 real):
  - conversión RGBA → NV12 en Go, repartida en hilos: 3,7 ms por cuadro;
  - codificación de software: ~9 ms por cuadro;
  - con la pantalla cambiando en **cada** cuadro, ~130 % de un núcleo: más que lo estimado.
- Por eso **un cuadro idéntico al anterior no se convierte ni se codifica**: se vuelve a enviar
  una vez por segundo, o antes si alguien pide un cuadro clave. Con el patrón de prueba (cursor
  que cambia 5 veces por segundo) la consola bajó a **25–45 % de un núcleo** y el video a
  100–320 kbit/s por alumno. Una clase de programación es casi siempre un editor quieto.
- La enumeración de monitores de `kbinani/screenshot` falla `checkptr` con `-race` (su callback
  hace aritmética de punteros); se enumera con `EnumDisplayMonitors` + `GetMonitorInfoW` propios
  y solo se usa `CaptureRect` de la biblioteca.

## D-015: Anuncio mDNS por cada interfaz (Windows con Hyper-V) y logs de las apps de ventana

**Fecha:** 2026-10-29 · **Origen:** prueba de la v0.3.0-rc.1 con dos laptops en una red Wi-Fi: el
alumno se quedó en "buscando la clase del canal 1". Corregido en v0.3.0-rc.2.

**Causa (confirmada el 2026-10-29 con las dos laptops de la prueba).**
- `grandcat/zeroconf` elige la interfaz de salida de cada paquete multicast con
  `ipv4.ControlMessage.IfIndex`. En Windows, `golang.org/x/net/ipv4` **ignora** ese campo
  (`ctlOpts` vacío): todo sale por la interfaz multicast predeterminada del sistema, la de
  métrica más baja.
- El equipo del profesor tiene ahora **Hyper-V** (para las VMs de prueba). Su adaptador
  `vEthernet (Default Switch)` tiene métrica 15, menor que la Ethernet o el Wi-Fi. El anuncio y
  las respuestas de la consola salían solo hacia el switch virtual, y los alumnos de la red real
  nunca la veían.
- En la prueba de la v0.2 todavía no había Hyper-V. Con los binarios publicados de la v0.3, en
  un solo equipo, el alumno sí se conecta: el paquete le llega por ese mismo switch.

**Decisión.**
- La consola, además de `zeroconf`, envía su anuncio (PTR + SRV + TXT + A/AAAA, con el mismo
  nombre de instancia, host y direcciones) **por cada interfaz**, eligiéndola con
  `IP_MULTICAST_IF`, que Windows sí respeta. Lo repite cada 5 s y manda la despedida (TTL 0) al
  cerrar (`internal/discovery/multicast.go`).
- Los alumnos ya instalados no cambian: el cliente de `zeroconf` acepta cualquier respuesta que
  traiga esos registros juntos. Una prueba lo verifica con el `Browser` real y sin el servidor
  de `zeroconf`.
- Se descartó cambiar de biblioteca mDNS: es un cambio mayor para una corrección de patch.

**Logs vacíos.** Las dos apps escribían con `io.MultiWriter(os.Stderr, archivo)`. En una app de
ventana stderr no es válido, `MultiWriter` se detiene en el primer error y **el archivo nunca
recibía nada**: `aula-cliente.log` y `aula-consola.log` estaban vacíos en las instalaciones
reales. Ahora el archivo va primero.

## D-016: El agente del alumno como servicio de Windows, con un ayudante en la sesión

**Fecha:** 2026-10-29 · **Estado:** aprobada e implementada (Windows; Linux y macOS después).

**Contexto.** El agente era un proceso de usuario que arrancaba con la clave `Run`: el alumno
podía cerrarlo, no existía sin sesión iniciada y nada podía correr con privilegios. Las funciones
futuras (`power.*`, `app.block`, `web.filter`, `remote.*`) los necesitan.

**Decisión.**
- **Servicio `AulaAgente`** (LocalSystem, arranque automático, reinicio ante fallos a los 5, 10 y
  30 s, también si sale con error). Mantiene la conexión con la consola: la lógica de siempre
  (`App` con `session.Agent`, `codedist`, `screen.Streamer`, `broadcast.Viewer`) corre sin ventana.
- **Ayudante** `aula-cliente.exe --ayudante <canal>` en la **sesión de consola activa**, con el
  token del usuario (`WTSQueryUserToken` + `CreateProcessAsUser`, sin elevar). Es la ventana de
  siempre: distintivo, código, avisos, transmisión, portapapeles, captura y distintivo nativo.
- **Canal local:** named pipe con nombre aleatorio por lanzamiento y DACL solo para SYSTEM y el
  usuario de la sesión. El servicio acepta únicamente al proceso que lanzó (PID del cliente). Por
  él viajan JSON por líneas: eventos de la ventana (`emit`, `raise`, `maximize`), los bindings de
  la ventana hacia el agente y los pedidos al ayudante (portapapeles, captura, distintivo). No es
  el protocolo con la consola: no sale del equipo.
- **Puente sin reescribir `App`:** en el servicio, `platform.Clipboard`, `ScreenCapture` y
  `ScreenIndicator` son *proxies* hacia el ayudante (`internal/agentsvc`), y `internal/screen` e
  `internal/broadcast` no cambian.
- **Sin protocolo nuevo.** El `hello` lleva el usuario de la sesión y las capacidades que el
  ayudante puede cumplir. Sin ayudante, ninguna. Cuando el ayudante llega o se va, el agente se
  reconecta con un `hello` nuevo (el mismo mecanismo que un cambio de configuración).
- **Sesiones:** aviso `SERVICE_CONTROL_SESSIONCHANGE` más revisión cada 2 s. Al cerrar sesión el
  ayudante termina; al cambiar de usuario se reubica; si muere se relanza con retroceso (1, 2, 4…
  30 s); un ayudante que duró 30 s olvida sus fallos. Las sesiones RDP se ignoran.
- **Modo sin servicio** (desarrollo, portables, Linux, macOS): igual que antes. Si el servicio
  corre, abrir `aula-cliente.exe` solo trae al frente la ventana del ayudante.
- **Named pipes con `github.com/Microsoft/go-winio`** (ya venía como dependencia indirecta de
  Wails): E/S solapada, DACL por SDDL y handle para `GetNamedPipeClientProcessId`.
- **Instalador:** quita la clave `Run` (dos agentes se pelearían la conexión), detiene el servicio
  y las ventanas antes de copiar, instala e inicia el servicio, y lo desinstala al desinstalar.
- **Logs:** el servicio escribe en `C:\ProgramData\Aula\alumno\logs\aula-agente.log` (SYSTEM no
  tiene perfil útil); el ayudante, en `%LOCALAPPDATA%\Aula\alumno\aula-cliente.log`.
- **Firewall:** sin reglas nuevas. "Aula Cliente (mDNS)" es por programa y cubre al servicio, que
  es el mismo ejecutable; la conexión con la consola y la transmisión son salientes.
- **Pruebas:** la lógica (supervisor, canal, *proxies*, `hello` según el ayudante) con dobles y una
  consola real. El ciclo de un servicio real solo en el job de CI `servicio`, con otro nombre
  (`AulaAgentePrueba`) y un paso que siempre lo borra.
