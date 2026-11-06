# Progreso

Última actualización: 2026-11-05 (10.7 mensaje privado a un alumno implementado, falta la prueba
en VM; 10.6 manos y mensajes del alumno implementada, con la vista
«Participación»; falta la prueba en VM; 10.5 captura de pantalla de un alumno implementada, falta la
prueba en VM; 10.4 control remoto implementado, falta la prueba en VM; etapa 19 cerrada).

La columna «Etapa» es el número del prompt en `docs/v1/prompts/`. Entre paréntesis, el número con
el que se trabajó antes; las notas de abajo lo conservan.

| Etapa | Descripción | Estado |
|---|---|---|
| 02 | Entorno y repositorio: monorepo Go + dos apps Wails, Git, CI | ✅ Completada |
| 03 | Especificación del protocolo (`docs/protocol.md`, paquete `protocol`) | ✅ Completada |
| 04 | Descubrimiento mDNS con canales + servidor/cliente WebSocket | ✅ Completada (falta prueba en VMs) |
| 05 | Seguridad: TLS y autenticación Ed25519 | ✅ Completada |
| 06 | App estudiante: overlay con Prism (D-008) y botón Copiar | ✅ Completada (falta prueba en VM) |
| 07 | App profesor: lista, hotkey de captura, emisión | ✅ Completada (falta prueba en VMs) |
| 08 | Hito 1: distribución de código de punta a punta + instaladores | ✅ **Hito 1 completado** (falta validación en laboratorio) |
| 08.1 | v0.2.0: canales numéricos tipo LanSchool, clave del centro, diagnóstico del agente | ✅ Completada (falta prueba real, `docs/prueba-vms.md` §10) |
| 09 | Monitoreo: miniaturas y pantalla en blanco (con aviso visible) | 🚧 Miniaturas hechas. La pantalla en blanco está documentada (protocolo §5.9, reglas 9-10) y diseñada (prototipo `alumno-pausa.html`); falta implementarla |
| 10 | Plantilla reutilizable para nuevas funciones | ✅ Completada (al día con las etapas 18 y 19) |
| 10.1 (09.3) | Agente del alumno como servicio de Windows (servicio + ayudante) | ✅ Implementado y probado en CI (falta prueba en VM, `docs/prueba-vms.md` §13) |
| 10.2 | Restricciones de la clase: bloqueo de apps y filtro web (Windows) | ✅ Implementado y probado en CI con administrador (falta prueba en VM, `docs/prueba-vms.md` §14) |
| 10.3 | Energía: apagar, reiniciar y cerrar sesión con cuenta regresiva (Windows) | ✅ Implementado; apagado con cuenta y posponer probado en el laboratorio (faltan los demás casos de `docs/prueba-vms.md` §15) |
| 10.4 | Control remoto del alumno | ✅ Implementado y probado en CI con dobles (falta la prueba en VM, `docs/prueba-vms.md` §16, y los casos C-44 a C-48 y A-09 a A-11) |
| 10.5 | Captura de pantalla de un alumno | ✅ Implementado y probado en CI con dobles (falta la prueba en VM, `docs/prueba-vms.md` §17, y los casos C-49 a C-53 y A-12 y A-13) |
| 10.6 | Mensajería del alumno y «levantar la mano» | ✅ Implementado y probado en CI con dobles y alumnos simulados; creó la vista «Participación» (falta la prueba en VM, `docs/prueba-vms.md` §18, y los casos C-54 a C-58 y A-14 a A-16) |
| 10.7 | Mensaje directo a un alumno | ✅ Implementado como mejora del aviso (`msg.show` con `personal`) y probado en CI (falta la prueba en VM, `docs/prueba-vms.md` §19, y los casos C-59 a C-62 y A-17) |
| 10.8 | Temporizador compartido | ⏳ Pendiente |
| 10.9 | Enviar archivos y plantillas de ejercicio | ⏳ Pendiente |
| 10.10 | Recoger código y archivos | ⏳ Pendiente |
| 10.11 | Enviar una URL o abrir una app | ⏳ Pendiente |
| 10.12 | Encuestas en vivo | ⏳ Pendiente |
| 10.13 | Asistencia y bitácora | ⏳ Pendiente |
| 11 | Revisión de seguridad y privacidad | ⏳ Pendiente (conviene antes de publicar `v0.4.0-rc.1`) |
| 12 (09.2) | Transmisión de la pantalla del profesor (WebRTC, Windows) | ✅ Implementada y probada en el centro de cómputo (4 equipos) |
| 12.1 | Proyectar la pantalla de un alumno a la clase | ⏳ Pendiente |
| 12.2 | Mejora de la transmisión: a una selección; modos ventana y pantalla completa | ⏳ Pendiente (`/funcionalidad-mejora 12.2`) |
| 13 | Portabilidad Linux/macOS y empaquetado | ⏳ Pendiente |
| 14 | Sandbox para ejecutar entregas | ⏳ Pendiente (después de 10.10) |
| 15 | Flota: despliegue, auto-actualización y diagnóstico | 🚧 El diagnóstico está casi todo hecho (`aula-cliente diagnostico`, comprobador de conexión, ficha del alumno); faltan la auto-actualización, la guía de despliegue y marcar a los desactualizados |
| 16 | Comercialización y licenciamiento | ⏳ Pendiente |
| 17 | Documentación: diagramas iniciales y validador `go test ./docs` | ✅ 16 diagramas, con el 10 del aula simulada |
| 18 | Pulido de la consola: UX/UI, manual, diagramas y aula simulada | ✅ Cerrada el 04/11/2026 (`docs/v1/ux/profesor/RESULTADOS.md`). Falta de tu lado: la prueba en una VM limpia y los casos C-01 a C-43 de `docs/desarrollo/probar-consola-local.md` en la ventana real |
| 19 | Pulido de la app del alumno: UX/UI, transparencia y no intrusión | ✅ Cerrada el 05/11/2026 (`docs/v1/ux/alumno/RESULTADOS.md`). Falta de tu lado: los casos A-01 a A-08 de `docs/desarrollo/probar-consola-local.md` en la ventana real |
| 20 | Auditoría y limpieza de los artefactos de `docs/v1/ux/` | ⏳ Pendiente (después de publicar el release) |

**Siguiente:**
1. Probar en la ventana real los casos C-01 a C-62 y A-01 a A-17, el control remoto en VM
   (`docs/prueba-vms.md` §16: RC-A, RC-B y RC-C para aceptar), la captura (§17: SN-A, SN-B y
   SN-C), las manos y mensajes (§18: HM-A, HM-B y HM-C) y el mensaje privado (§19: PM-A, PM-B y
   PM-C).
2. La revisión 11.
3. ~~Publicar `v0.6.0-rc.1`~~: publicado el 05/11/2026 (etapas 10.2, 10.3, 18, 19 y 10.4, instaladores
   de Windows y checksums firmados). Las 0.x son borradores: al integrar todas las funciones se
   borran y empieza la versión 1 oficial. Probar sus instaladores en las VMs (§14 a §16).
4. Cualquier función de 10.8 a 10.13 con `/funcionalidad-nueva <número>`. Las que recogen
   respuestas (10.8, 10.10, 10.12, 10.13) agregan su sección a la vista «Participación».

## Etapa 02: notas

- Módulo `github.com/bayalad7/aula` (`go 1.26.0`), Wails v2.17.0.
- Paquetes `internal/{protocol,discovery,transport,session,security,platform,codedist}`, cada uno
  con un archivo mínimo y una prueba trivial. `platform` ya define `ErrNotSupported`.
- `apps/teacher` ("Aula — Consola") y `apps/student` ("Aula — Cliente"): Wails v2 con frontend
  HTML/CSS/JS sin bundler. Verificadas con `wails dev` y `wails build` en Windows 10.
- Decisiones en `docs/decisiones.md` (D-001 a D-003).
- CI en `.github/workflows/ci.yml` (Ubuntu, Windows, macOS). No hay job de Node porque el
  frontend todavía no tiene pruebas ni chequeo de tipos.
- Ramas: `main` y `develop` en `origin`; `develop` es la rama por defecto.

## Entorno de desarrollo (equipo del profesor, Windows 10)

- CLI de Wails v2 (obligatoria para `wails dev` y `wails build`). Guía oficial:
  <https://wails.io/docs/gettingstarted/installation>. Ejecuta
  `go install github.com/wailsapp/wails/v2/cmd/wails@latest` para instalar la CLI de Wails.
  El binario queda en `%USERPROFILE%\go\bin`, que debe estar en el PATH. Comprueba con
  `wails doctor` (debe terminar en *SUCCESS*). Versión instalada: v2.17.0.
- gcc 16.2.0 (WinLibs POSIX UCRT, instalado con
  `winget install BrechtSanders.WinLibs.POSIX.UCRT`) para cgo y `go test -race`.
- winget lo instala bajo `C:\Users\Benjamín Ayala\...`. `ld` no soporta el espacio de esa ruta
  (falla con `cannot find C:/Users/Benjam?n` al buscar `default-manifest.o`), así que se
  configuró:
  - junction `C:\mingw64` apuntando a
    `...\WinGet\Packages\BrechtSanders.WinLibs.POSIX.UCRT_Microsoft.Winget.Source_8wekyb3d8bbwe\mingw64`;
  - `go env -w CC=C:\mingw64\bin\gcc.exe`;
  - variable de entorno de usuario `GCC_EXEC_PREFIX=C:/mingw64/lib/gcc/`. Sin ella, gcc resuelve
    su ruta real (con espacio) aunque se invoque por el junction o por el nombre corto 8.3.
- Las terminales y VS Code abiertos antes del cambio deben reiniciarse para ver
  `GCC_EXEC_PREFIX`.
- Si `winget upgrade` cambia la carpeta del paquete, hay que recrear el junction:
  `New-Item -ItemType Junction -Path C:\mingw64 -Target <nueva ruta>\mingw64`.
- Con gcc en el PATH, `CGO_ENABLED` vale 1 por defecto en local. Los binarios publicados se
  compilan sin cgo en CI.
- Verificado el 2026-10-23: `go test -race ./...` pasa en local.

## Pendientes conocidos

- No están instaladas las herramientas opcionales de Wails `nsis` (instalador, etapa 08) y `upx`.

## Etapa 03: notas

- Especificación aprobada en `docs/protocol.md` (protocolo v1). Cambios respecto al borrador:
  la autenticación va antes de `hello` (el agente no se identifica ante una consola sin la clave
  correcta) y `code.share` tiene un campo opcional `exercise`.
- La firma de `auth.proof` cubre etiqueta, nonce, canal y huella del certificado TLS (§3.2).
- `internal/protocol`: `Envelope`, `Encode`/`Decode` con validación de versión y límites,
  payloads tipados de los 11 mensajes v0, `Capability`, `HelloInfo`, `NewID` (UUID v4) y
  `ErrorCodeFor`. Solo usa la biblioteca estándar. Decisión de diseño en D-004.
- Pruebas: ida y vuelta de cada tipo, ejemplos en `internal/protocol/testdata/{valid,invalid}`
  (los inválidos se nombran con el código de error esperado), versión desconocida, mensaje y
  contenido demasiado grandes, JSON inválido y un fuzz de `Decode` (`FuzzDecode`).
- Pendiente para etapas siguientes: la consola no autentica al agente en v0 (limitación
  documentada en §3.2).

## Etapa 04: notas

- `internal/discovery`: `Announcer` publica `_aula._tcp` con los TXT del protocolo
  (`cert_fingerprint` = `PlaceholderFingerprint` hasta la etapa 05). `Browser` filtra por
  canal y `proto`, reinicia la búsqueda cada 30 s porque zeroconf deja de preguntar tras la
  primera respuesta, y ordena las direcciones de equipos con varias interfaces (D-006).
- `internal/transport` (`github.com/coder/websocket`, D-005): `Server` con canal de eventos
  (`connect`, `message`, `disconnect`) y `Client` (`Dial`, `Incoming`). Por conexión hay una
  goroutine lectora y una escritora con cola. Aplica el límite de 1 MiB (cierre 1009),
  responde `error` a mensajes inválidos (cierre 1008 por versión), cierra conexiones mudas a los
  15 s y envía `heartbeat` cada 5 s. El `error` sale siempre antes del frame de cierre.
- `internal/session`: `Registry` (por canal, concurrente, marca desconectada la sesión sin
  señal en 15 s y reemplaza la sesión vieja del mismo equipo al reconectar). `Console` exige
  `hello` como primer mensaje (10 s) y enruta por tipo. `Agent` busca, conecta, envía `hello`
  y `heartbeat`, y reconecta con retroceso 1, 2, 4…30 s ±20 %; tras 3 fallos vuelve a
  descubrir. `StaticSource` permite conectar sin mDNS.
- **Temporal hasta la etapa 05:** sin TLS (`ws://`) y sin autenticación; `hello` es el primer
  mensaje. En la etapa 05 se antepone `auth.challenge` → `auth.proof` → `auth.ok` (§3.1).
- Pruebas: loopback con servidor y dos agentes (aparecen, uno se va), agente "congelado"
  marcado desconectado tras el timeout, primer mensaje distinto de `hello`, timeout de
  handshake y reconexión cuando la consola vuelve. Además, filtrado de otro canal (lógica) y
  mDNS real con dos consolas de canales distintos. Esta última se salta si no hay multicast;
  con `AULA_REQUIRE_MDNS=1` falla. En este equipo pasa con multicast real.
- Ejemplo: `go run ./examples/dos_alumnos` (profesor + 2 alumnos por mDNS; el alumno 2 se
  apaga a los 10 s). En VMs: `-rol profesor` en una y `-rol alumno` en otras (o
  `-rol alumno -directo IP:47830` si la red bloquea multicast). Verificado en un solo equipo.
- Pendiente de la etapa: probar en VMs reales que un alumno aparece y desaparece del registro
  (pasos en `docs/prueba-vms.md` §4, casos 04-A a 04-G).
  Windows pide permiso de firewall la primera vez (TCP 47830 entrante y UDP 5353 de mDNS).
- Las apps Wails todavía no usan estos paquetes; se cablean en las etapas 06 y 07.

## Etapa 05: notas

- Modelo de confianza documentado en `docs/protocol.md` §10 (antes de programar). Decisión de
  diseño en D-007.
- `internal/security`: identidad Ed25519 (`CreateIdentity`/`LoadIdentity`, formato
  `aula-ed25519:<Base64>`, huella corta), firma y verificación de `auth.proof` ligadas a nonce,
  canal y huella del certificado, certificado TLS autofirmado persistente y `ClientTLS` con
  fijación de huella.
- `internal/platform`: `RestrictToOwner` (DACL protegida en Windows, `0600`/`0700` en Unix) y
  `AttachParentConsole` para los subcomandos de la app de ventana.
- `transport`: `Options.TLS`; el servidor envuelve el listener y el cliente usa `wss://`.
- `session`: `Console` hace el handshake completo (`auth.challenge` → `auth.proof` → `auth.ok`
  → `hello`) y rechaza con `unauthorized` + 1008 lo que llegue antes. `Agent` exige `Channel` y
  `ConsolePublicKey`, fija la huella anunciada, verifica la firma, registra el rechazo y marca la
  consola como no confiable durante 5 min.
- `aula-consola keygen [-dir] [-force]` y `aula-consola pubkey`: crean la clave o muestran la
  pública. `keygen` no sobrescribe sin `-force`.
- Ejemplo `dos_alumnos`: el profesor usa la clave de `keygen` (o la crea si falta) y la imprime;
  el alumno necesita `-clave aula-ed25519:…`. `-rol todo` usa una clave temporal.
- **Cómo se comprobó que el tráfico va por TLS:** `TestTrafficIsEncrypted` graba los bytes crudos
  de la conexión en ambos sentidos. Con `wss://` empiezan con un registro de handshake TLS
  (`16 03`) y no contienen el texto enviado, ni `msg.show`, ni `GET /ws`. En el control con
  `ws://` el texto sí aparece, así que la prueba detectaría texto plano. Además, el servidor
  registra `tls=true` y el cliente solo acepta TLS 1.3.
- Pruebas de la etapa: clave correcta se autentica; otra clave es rechazada; huella distinta es
  rechazada; `code.share` antes de `auth.ok` es rechazado y no llega a `OnMessage`; la consola
  rechaza mensajes antes de la autenticación; conexión directa sin huella funciona; un relay con
  su propio certificado es detectado por la firma; la clave privada no aparece en los logs (ni al
  registrar la identidad a propósito) ni en la salida de `keygen`.
- Verificado a mano: el `.exe` de `wails build` imprime con `keygen`, e `icacls` muestra una sola
  entrada (el usuario, control total) en el archivo de la clave.
- Pendiente: la configuración persistente del agente (dónde se guarda la clave pública al
  instalar) llega con la app del alumno (etapa 06) y los instaladores (etapa 08).

## Etapa 06: notas

- `apps/student`: el agente real dentro de la ventana Wails. Configuración en
  `%AppData%\Aula\alumno\config.json` (`channel`, `console_public_key`, `direct` opcional) o con
  `-canal`, `-clave`, `-directo`; sin configuración muestra "Aula — sin configurar" y la
  explicación.
- Al llegar `code.share` la ventana se trae al frente y muestra título, ejercicio, lenguaje y el
  código resaltado con Prism.js (no Monaco: D-008), con el botón "Copiar al portapapeles".
  `replace` sustituye, `append` agrega (máx. 20, se descartan los más antiguos; un reenvío con el
  mismo `snippet_id` se actualiza en su lugar). `code.clear` limpia. `msg.show` aparece como aviso
  info/advertencia con "Entendido". Todos responden su `ack`.
- Distintivo siempre visible: "Aula — conectado a la clase de <profesor> (canal <X>)", también en
  el título de la ventana. Reglas en `docs/privacidad.md` (nuevo).
- Contrato de eventos en `apps/student/events.go` y `docs/eventos-ui.md`: eventos `aula:status`,
  `aula:board`, `aula:notices` y métodos `View`, `Copy`, `DismissNotice`.
- `internal/codedist`: `Board` y `Receiver` (lógica sin SO ni red). `internal/platform`:
  `Clipboard` con `golang.design/x/clipboard` v0.11 (CRLF en Windows).
- `session.Console`: `Send` (comprueba la capacidad del agente) y `Broadcast`; el registro guarda
  el último `code.ack` de cada sesión (`LastCodeAck`).
- Ejemplo `dos_alumnos -rol profesor`: comandos `codigo`, `agregar`, `limpiar`, `aviso`, `alerta`
  para emitir mientras llega la consola Wails (etapa 07). El registro muestra "código xxxx ✓"
  cuando el alumno confirma.
- Pruebas: integración consola → agente con `code.ack` registrado y `code.clear`; `Send` rechaza
  sin capacidad; portapapeles real con Python con sangría, tabuladores, línea vacía y espacios
  finales (se salta sin portapapeles; `AULA_REQUIRE_CLIPBOARD=1` lo exige); app del alumno con
  portapapeles falso (copia exacta, eventos, distintivo, avisos, configuración); contrato
  `events.js` ↔ Go; la interfaz solo usa `innerHTML` con Prism.
- Verificado en este equipo con `aula-cliente.exe` real: el profesor envió un `.py`, la ventana
  lo mostró resaltado con el aviso. Un clic en el botón copió el archivo idéntico (10 líneas,
  sangría intacta, CRLF), y el registro del profesor mostró el `code.ack`. El alumno reconectó
  solo al reiniciar la consola.
- Corregido: la regla `dist/` del `.gitignore` (etapa 05) ignoraba también `frontend/dist`; ahora
  es `/dist/`.
- Pendiente de la etapa: en una VM, emitir desde la consola y pegar en el editor del alumno
  (pasos en `docs/prueba-vms.md` §5, casos 06-A a 06-J).

## Etapa 07: notas

- `apps/teacher` (consola Wails):
  - **Selección de aula:** canales recientes, accesos rápidos y nombre visible. La primera clase
    crea la clave y el certificado.
  - **Tarjetas de alumno en vivo:** equipo, usuario, SO, estado, capacidades y último código
    confirmado.
  - **Panel de código con Monaco:** título, lenguaje, "Emitir a todos", "Agregar bloque",
    "Limpiar pantallas".
  - **Historial numerado** ("Ejercicio N") con "Reemitir" (doble clic lo carga en el editor) y
    **avisos** info/advertencia.
  - **Confirmaciones:** "enviado a N · confirmados M/N ✓", con quién no soporta la capacidad.
  - **Botones deshabilitados** si ningún alumno conectado declara la capacidad.
- **Captura rápida:** atajo global `Ctrl+Alt+Shift+C` (Windows) y botón flotante equivalente.
  Cargan el portapapeles en el editor y proponen el lenguaje; nunca emiten solos.
- Nuevo en `internal`: `platform.Hotkeys` (impl. Windows, fallback "no soportado"),
  `codedist.History` y `codedist.Deliveries`. `session.Console.Broadcast` devuelve los IDs de sobre
  y `Console.OnChange` avisa los cambios del registro. Decisión en D-009; contrato en
  `docs/eventos-ui.md`.
- `AULA_CONSOLE_DIR` cambia el directorio de la clave y la configuración de la consola (pruebas).
- Pruebas:
  - binding de emisión con aula doble (payload correcto, acuses, sin capacidad);
  - historial (emitir, agregar, reemitir con el mismo número e ID);
  - atajo y botón con portapapeles doble (carga sin emitir, portapapeles vacío);
  - registro real del atajo (conflicto y liberación);
  - `TestLiveClassTwoAgentsAck`: consola real con TLS y dos agentes, 2/2 `code.ack` y 2/2
    `msg.ack`;
  - contrato `events.js` ↔ Go.
- Verificado a mano en este equipo con la app real (capturas de su ventana):
  - la selección de aula funciona;
  - Monaco colorea Python;
  - el atajo real cargó el portapapeles y detectó Python;
  - "Emitir a todos" creó el Ejercicio 1 y el alumno real lo confirmó.

  El segundo alumno, de consola (`dos_alumnos`), no respondía `code.ack`. Ya lo hace, pero falta
  verlo en pantalla.
- Pendiente de la etapa: en VMs, dos alumnos confirman (2/2) y el atajo captura desde otra
  aplicación (pasos en `docs/prueba-vms.md` §6, casos 07-A a 07-J).

## Etapa 08 (Hito 1): notas

- **Configuración** (`docs/configuracion.md`):
  - agente: `session.AgentConfig` (`channel`, `console_public_key`, `direct` opcional);
  - precedencia: argumentos > `C:\ProgramData\Aula\alumno\config.json` (máquina, protegida) >
    `%AppData%` (usuario);
  - consola: `%AppData%\Aula\consola\consola.json` (`recent_channels`, `console_name`, `port`).
- **Configuración de alumnos generada por la consola:** botón **Configuración de alumnos** o
  `aula-consola config-alumno -canal … [-directo …]`.
- **Instaladores Inno Setup** (`empaquetado/windows/`):
  - agente por máquina, con arranque al iniciar sesión (HKLM `Run`);
  - toma `aula-alumno.json` de junto al setup o de `/CONFIG=`;
  - protege la configuración con `icacls`, crea la regla de firewall y admite
    `/VERYSILENT /SUPPRESSMSGBOXES /NORESTART`;
  - consola con instalador (accesos directos y regla de firewall) y portable.
  - Compilan en local con Inno Setup 6.7.3.
- **Versiones:**
  - `aula-consola version` y `aula-cliente -version` (`-ldflags -X main.version`);
  - metadatos del `.exe` en `wails.json`;
  - instancia única en ambas apps.
- **Workflow de release** (`.github/workflows/release.yml`, solo Windows):
  - tag `v*` → pruebas → apps → comprobación de versión → Authenticode opcional → instaladores
    → `checksums.txt` firmado con Ed25519 y verificado con `empaquetado/llave-releases.pub` →
    `gh release create` (prerelease si hay guion);
  - secretos en `docs/desarrollo/publicar.md`; `RELEASE_SIGNING_KEY` ya está creado (la privada
    no existe en disco);
  - `.deb`/`.rpm`/`.pkg` quedan para la etapa 13.
- **Robustez:**
  - `TestTwelveAgentsAndConsoleRestart`: 12 agentes reales, 12/12 acuses, reinicio de la consola
    y vuelven solos;
  - `examples/muchos_alumnos -n N`: probado aquí con 15 agentes, emisión, reinicio del profesor
    (0/15 → 15/15, 15 reconexiones) y segunda emisión.
- **Guías:** `docs/uso-hito1.md` (uso en clase e instalación) y `docs/prueba-vms.md` §10 (casos
  08-A a 08-K con los instaladores). Decisión D-010.
- **Release verificado:** el tag `v0.1.0-rc.1` publicó la prerelease con los dos instaladores,
  los dos portables, `checksums.txt` y `checksums.txt.sig`. Descargada desde GitHub: la firma
  verifica con la llave embebida (también con
  `go run github.com/bayalad7/aula/tools/relsign@v0.1.0-rc.1 verify`), los 4 SHA-256 coinciden,
  los binarios reportan `0.1.0-rc.1` y un `checksums.txt` alterado se rechaza.
- **Pendiente:** validación en laboratorio o VMs con 2–3 equipos (08-C, 08-F y 08-G) y la
  prueba en VMs de las etapas 04, 06 y 07.
- **Prueba real (29/10/2026):** consola instalada con `Aula-Consola-Setup-0.1.0-rc.1.exe` en el
  equipo del profesor y agente con `Aula-Alumno-Setup-0.1.0-rc.1.exe` en otra laptop. El alumno
  aparece (agente `0.1.0-rc.1`) y el Ejercicio 1 se confirma 1/1. **Defecto encontrado:** hubo que
  reiniciar la laptop para que el agente leyera `C:\ProgramData\Aula\alumno\config.json`, porque
  el agente solo leía la configuración al arrancar y, si ya estaba abierto (o la instancia única
  traía al frente uno viejo), seguía sin configurar. **Corregido:** el agente relee la
  configuración cada 3 s y se reconecta solo si cambia (`TestConfigWrittenWhileRunningIsPickedUp`,
  con consola real). Sale en la próxima versión (`v0.1.0-rc.2`).

## v0.2.0: canales tipo LanSchool y clave del centro (D-011)

- **Motivo:** en la prueba real el modelo por maestro (cada consola con su clave y un
  `aula-alumno.json` para sus alumnos) no era práctico. El usuario espera el de LanSchool: cada
  computadora de alumno con su canal fijo y la consola con un canal predeterminado que el maestro
  puede cambiar.
- **Canales:** números del 1 al 16000 (`protocol.ValidChannel`, `ParseChannel`). Cambio
  incompatible: por eso es 0.2.0.
- **Clave del centro:** una sola Ed25519 por centro, en `C:\ProgramData\Aula\centro\clave-centro.pem`
  (Administradores/SYSTEM control total, Usuarios lectura; `platform.RestrictMachineSecret`).
  `aula-consola centro crear|importar|exportar|publica`. Sin ella, la consola usa la clave
  personal y lo advierte en la barra superior.
- **Consola:** arranca sola en su canal predeterminado (`C:\ProgramData\Aula\consola\config.json`,
  `default_channel` y `port`; si no existe, el último canal o el 1). Selector **Canal … Cambiar**
  y nombre editable en la barra superior. Se quitaron "Configuración de alumnos", "Copiar clave",
  "Cambiar de aula" y el subcomando `config-alumno`.
- **Instaladores:** la consola pide canal y crea o importa la clave del centro (deja
  `aula-centro.pub` y `aula-centro.key`); el alumno pide su canal y toma la clave de
  `aula-centro.pub` junto al setup o de `/CLAVECENTRO=`. Cambiar de sala: reinstalar con
  `/CANAL=N` (el agente lo toma en segundos).
- **Diagnóstico** (el "buscando la clase" de la rc.2 no decía por qué):
  - el agente muestra el motivo bajo la barra: ninguna consola del canal, clave que no es la del
    centro, certificado distinto o dirección inalcanzable;
  - logs a archivo con rotación de 1 MiB: `%LOCALAPPDATA%\Aula\alumno\aula-cliente.log` y
    `…\consola\aula-consola.log`;
  - `aula-cliente diagnostico`: configuración usada, consolas vistas por mDNS e intento de
    conexión.
  - La causa exacta de la rc.2 no se confirmó (no había logs). Las candidatas son la red en
    perfil Público o un firewall que bloquea mDNS, o una clave o canal que no coinciden con la
    consola abierta. La v0.2.0 la dirá en pantalla.
- **Pruebas nuevas:** dos salas con la misma clave del centro (`TestTwoRoomsShareTheSiteKey`),
  motivos del diagnóstico (`TestDiagnostic*`), arranque en el canal predeterminado, comandos
  `centro`, ACL de máquina y log con rotación.
- **Prueba real de la rc.1 (29/10/2026):** instalaciones correctas; la consola generó la clave del
  centro y el alumno tomó `aula-centro.pub`, pero dijo "No se encontró ninguna consola del canal
  1": los dos equipos estaban en una red **Pública**, que las reglas de firewall no cubren. El
  diagnóstico nuevo cumplió su propósito. **Agregado (D-012):** casilla "Permitir también en
  redes Públicas" en los dos instaladores (`/MERGETASKS="redpublica"`), desmarcada por defecto;
  lo recomendado sigue siendo marcar la red como Privada. Caso 08-Q en `docs/prueba-vms.md`.
- **Pendiente:** prueba real con los instaladores de `v0.2.0-rc.2` (`docs/prueba-vms.md` §10,
  casos 08-A a 08-Q).

## Etapa 09 (Módulo B): notas — en curso

- **Hecho (sin commitear):**
  - documentación: `docs/protocol.md` §5.6–5.11, reglas 6–11 de `docs/privacidad.md`, D-013 y
    `internal/screen` en CLAUDE.md;
  - `internal/protocol`: `screen.thumb.start/frame/stop` con validación, fixtures y pruebas;
  - `internal/platform`: interfaces `ScreenCapture` y `ScreenIndicator`, `EncodeFrame`
    (escalado y JPEG), captura del monitor principal en Windows (`kbinani/screenshot`), el
    distintivo nativo "Tu pantalla está siendo vista por el profesor" y el fallback "no
    soportado" en Linux y macOS.
- **Pantalla en blanco (`screen.freeze`):** retirada del código a pedido del usuario; queda
  documentada como pendiente (protocolo §5.9–5.11, privacidad reglas 9–10, D-013).
- **Hecho después (sin commitear):**
  - `internal/screen`: `Streamer` (agente: distintivo antes del primer cuadro, `start`
    reemplaza parámetros, `stop` o desconexión terminan todo, error de captura reportado una
    vez, baja la calidad si el cuadro excede 640 KiB) y `Wall` (consola: quién transmite,
    miniatura o detalle, descarta cuadros viejos, `Close` detiene todo). Pruebas con dobles.
  - Agente: declara `screen.thumb` solo si hay captura e indicador; la barra cambia a "Aula — tu
    pantalla está siendo vista por …" (en rojo); al desconectarse se detiene la captura.
  - Consola: botón **Ver pantallas** (cuadrícula en vivo; clic → vista en detalle; Esc o Cerrar
    detienen las transmisiones), `OpenScreens`/`CloseScreens`/`ViewScreen`, evento `aula:frame`,
    `canThumb` en las tarjetas; cambiar de canal o cerrar la clase detiene todo.
- **Falta:**
  - la prueba automática con 20 agentes simulados (no se pudo completar); mientras tanto, la
    medición se hace en VMs y se proyecta a 20 (`docs/prueba-vms.md` §11.3);
  - correr los casos 09-A a 09-L y la medición de `docs/prueba-vms.md` §11;
  - prueba real en VMs Windows (miniaturas, distintivo visible, detener la vista detiene la
    captura) y los commits.

## Etapa 09.2: transmisión de la pantalla del profesor — notas

- **Diseño aprobado** (2026-10-29), con las recomendaciones:
  - H.264 con Media Foundation, sin cgo;
  - el alumno puede minimizar la vista;
  - la consola siempre excluida de la captura;
  - UDP 47831.

  Documentado en:
  - `docs/protocol.md` §5.12 (`screen.broadcast.start/offer/answer/ice/stop`), con su tabla en
    §5, límites en §7, logs en §9 y amenaza en §10.3; también se corrigió la fila rota de
    `screen.broadcast` en §4;
  - `docs/privacidad.md`, reglas 12–16;
  - `docs/decisiones.md`, D-014: un codificador repartido a N conexiones; un SFU dentro del
    proceso no ahorra subida. Umbral: subida pico > 50 % del enlace o CPU > 50 %.
- **Protocolo:** `internal/protocol/broadcast.go` (validación, límites de SDP y de candidatos),
  `CapScreenBroadcast` y su mapa de capacidades, fixtures válidos e inválidos y pruebas.
- **`internal/platform`:**
  - interfaces `DisplayCapture` (monitores y captura continua) y `VideoEncoder`;
  - `RGBAToNV12`, `ExcludeFromCapture`;
  - en Windows: monitores con `EnumDisplayMonitors` y `GetMonitorInfoW` propios (los de
    `kbinani/screenshot` fallan `checkptr` con `-race`), captura con `CaptureRect`, exclusión con
    `SetWindowDisplayAffinity` y el codificador H.264 de Media Foundation por COM sin cgo;
  - fallback "no soportado" en Linux, macOS y Windows de 32 bits.
- **`internal/broadcast`** (sin APIs del SO):
  - `Caster`: señalización por sesión con intentos, reintentos con retroceso, expiración del
    intento, gracia de desconexión y conteo de receptores;
  - `Source`: captura y codificación a 15 cuadros/s; un cuadro idéntico no se codifica, se
    reenvía una vez por segundo; cuadros clave a pedido, como máximo uno por segundo;
  - `Media`: pion, ICE-lite, `UDPMux` en un puerto y un track H.264 compartido;
  - `Broadcaster`: une las tres piezas;
  - `Viewer` (agente): valida que la oferta sea solo video de recepción y muestra la vista
    antes de entregarla.
- **Consola:**
  - selector de monitor y **Transmitir mi pantalla / Detener transmisión** en la barra
    superior;
  - franja roja "Transmitiendo tu pantalla (monitor N) a M de K alumnos" con **Detener**;
  - etiqueta "ve tu pantalla" en las tarjetas de los alumnos;
  - la ventana se excluye de la captura antes de transmitir (si no puede, no transmite);
  - cambiar de canal o cerrar la clase detienen la transmisión;
  - `broadcast_max_kbps` en la configuración de máquina.
- **Alumno:**
  - el frontend informa con `ReportMedia` si tiene WebRTC y H.264; el agente espera ese aviso
    hasta 5 s antes de conectarse y solo entonces declara `screen.broadcast`;
  - la vista de transmisión trae la ventana al frente y la maximiza, con la franja azul fija,
    el `<video>`, **Tamaño real** y la barra "Aula — viendo la pantalla de …";
  - el RTCPeerConnection vive en el frontend y Go valida y reenvía la señalización;
  - al perder la conexión la vista se cierra sola.
- **Pruebas automáticas:**
  - señalización con un doble de PeerConnection (`caster_test.go`): orden start → offer → ice,
    respuesta a la oferta vigente, respuestas viejas ignoradas, candidatos `.local` filtrados,
    reintento por falla, desconexión, error del agente y expiración; un alumno que sale y otro
    que entra; stop a todos al detener;
  - `Viewer`: aviso antes de la oferta, rechazo de audio y canal de datos, intentos, límite de
    candidatos;
  - **de punta a punta con pion real en ambos lados** por loopback: dos receptores, uno cae y
    vuelve como sesión nueva y recibe video otra vez, detener y volver a empezar;
  - salto de cuadros idénticos, codificador real de Media Foundation (IDR con SPS/PPS, cuadro
    clave forzado), NV12, exclusión de la captura (leyendo la afinidad, sin capturar la
    pantalla);
  - bindings de ambas apps y contrato de eventos.
- **Verificado en este equipo con el WebView2 real** (`aula-cliente.exe` compilado y
  `examples/transmision` con el patrón sintético de código, sin transmitir el escritorio):
  - ICE-lite y DTLS conectan (receptores 1/1) y el H.264 de Media Foundation se decodifica;
  - la captura de la ventana del alumno muestra el código legible bajo la franja azul;
  - al cerrar la consola la vista desaparece sola, y al volver el alumno se reconecta y recibe
    la nueva transmisión sin intervención;
  - la ventana se maximiza al empezar y se restaura al terminar. Una vez, con la transmisión
    llegando ~2 s después de abrir la app, quedó maximizada; no se reprodujo.
- **Medido** (Ryzen 5 3450U, 1080p, un receptor):
  - NV12: 3,7 ms por cuadro; codificación de software: ~9 ms por cuadro;
  - con cada cuadro distinto, ~130 % de un núcleo; con el salto de cuadros idénticos y el
    cursor del patrón cambiando 5 veces por segundo, 25–45 % de un núcleo;
  - video de 100–320 kbit/s.
- **Falta:**
  - prueba en VMs con 2–3 alumnos y la medición de `docs/prueba-vms.md` §12 (CPU y subida del
    profesor con la consola real);
  - probar la interfaz de la consola con clics (selector, Transmitir, Detener, franja), que solo
    se verificó con pruebas de los bindings;
  - codificadores H.264 por hardware (MFT asíncronos), que bajarían la CPU;
  - Linux y macOS (cuando se indique);
  - los commits.

## v0.3.0-rc.2: el alumno no encontraba la consola (D-015)

- **Síntoma** (29/10/2026), con dos laptops en la red Wi-Fi de casa: tras actualizar el alumno a
  la v0.3.0-rc.1 (redes Públicas permitidas en las dos instalaciones), se quedó en "buscando la
  clase del canal 1". Con la v0.2 funcionaba.
- **Descartado:** entre v0.2.0-rc.2 y v0.3.0-rc.1 no cambiaron `discovery`, `session`,
  `transport` ni los instaladores. Con los binarios publicados, consola y alumno en este equipo
  se conectan por mDNS en 1 s. La regla de firewall de la consola incluye el perfil Público.
- **Causa (confirmada):** Hyper-V en el equipo del profesor. `zeroconf` no puede elegir la interfaz
  de salida en Windows y el anuncio salía solo por `vEthernet (Default Switch)`, que tiene la
  métrica más baja (15).
- **Corregido en v0.3.0-rc.2:**
  - anuncio mDNS por cada interfaz con `IP_MULTICAST_IF`; los alumnos instalados no necesitan
    actualizarse, basta la consola nueva;
  - los logs de las apps de ventana estaban vacíos (stderr inválido en `io.MultiWriter`).
- **Confirmado** con las dos laptops de la prueba (Wi-Fi de casa, consola en la que tiene
  Hyper-V): el alumno v0.3.0-rc.1 sin cambios se conectó con la consola corregida, o subiendo la
  métrica de `vEthernet (Default Switch)`.

## Etapa 09.3: el agente del alumno como servicio de Windows (D-016) — notas

- **Diseño aprobado** (2026-10-29):
  - sin mensajes de protocolo nuevos;
  - el mismo conjunto de capacidades, declaradas solo si el ayudante de la sesión puede
    cumplirlas;
  - el servicio mantiene la conexión con la consola y el ayudante en la sesión del alumno muestra
    todo;
  - reglas de privacidad 17–19.
- **`internal/platform`:**
  - `ServiceControl`: instalar o actualizar, desinstalar, iniciar, detener, estado (este último sin
    administrador);
  - `RunAsService` con avisos de cambio de sesión;
  - `SessionManager`: sesión de consola activa y lanzamiento con el token del usuario;
  - canal local `ListenLocal`, `DialLocal` y `PeerPID` (named pipe con DACL, go-winio);
  - fallback "no soportado" fuera de Windows;
  - log del servicio en `ProgramData` (`OpenLogFileIn`, `AgentServiceLogDir`).
- **`internal/agentsvc`** (sin APIs del SO):
  - `Supervisor`: lanza, relanza con retroceso y reubica el ayudante, y solo acepta al proceso que
    lanzó;
  - `Link`: JSON por líneas, llamadas con respuesta y eventos, hasta 4 MiB por línea;
  - *proxies* de portapapeles, captura y distintivo.
- **`apps/student`:**
  - subcomandos `servicio instalar|desinstalar|iniciar|detener|estado|ejecutar [-nombre]`;
  - modo servicio: `App` sin ventana, `hello` con el usuario de la sesión y capacidades según el
    ayudante, y reconexión cuando llega o se va;
  - modo ayudante `--ayudante <canal>`: la ventana de siempre con los bindings reenviados;
  - si el servicio corre, abrir el programa solo trae la ventana al frente.
- **Instalador:** quita la clave `Run`, detiene el servicio y las ventanas antes de copiar,
  instala e inicia el servicio, lo desinstala al desinstalar y crea `logs\`. Compila con Inno
  Setup 6; no se ejecutó en este equipo para no instalar un servicio en él.
- **CI:** job `servicio` (windows-latest) con `TestServiceLifecycle` (etiqueta `servicetest`):
  - instala `AulaAgentePrueba`;
  - verifica arranque automático y reinicio ante fallo en la configuración;
  - lo inicia, **mata su proceso y espera a que el administrador de servicios lo levante con otro
    PID**, lo detiene y lo desinstala;
  - un paso `if: always()` lo borra con `sc.exe` aunque la prueba falle.
- **Pruebas locales:**
  - supervisor con dobles: sin sesión no lanza; logon; PID ajeno rechazado; relanzar tras morir;
    cambio de usuario; cierre de sesión; canal cerrado; fallo al lanzar;
  - `Link` (llamadas concurrentes, errores, 900 KiB) y *proxies*;
  - agente-servicio contra una **consola real**: sin capacidades sin ayudante, reconexión con
    usuario y capacidades cuando el ayudante avisa, "Copiar" llega al portapapeles del ayudante, y
    vuelta a sin capacidades cuando se va;
  - el pipe real con DACL y PID del cliente, y el estado de servicios sin administrador.
- **Verificado en este equipo, sin administrador:** con un programa temporal haciendo de servicio,
  el `aula-cliente.exe --ayudante` real (Wails y WebView2):
  - se conectó por el pipe con su PID verificado;
  - informó sus capacidades reales (captura, portapapeles, WebRTC con H.264);
  - mostró el distintivo y el código pedidos por el canal (vista reenviada y título por evento);
  - terminó solo al cerrarse el canal.

  No se tocó el portapapeles ni se capturó la pantalla.
- **CI en GitHub (29/10/2026):** el job `servicio` pasó en `windows-latest`: instaló
  `AulaAgentePrueba`, se mató su proceso (PID 620) y el administrador de servicios lo levantó solo
  (PID 2260), y luego se detuvo y desinstaló; el paso final confirmó que no quedó instalado.
- **Falta:**
  - la prueba en VM de `docs/prueba-vms.md` §13: arranque con el equipo sin sesión, ayudante en
    la sesión estándar, relanzar, cambio de usuario;
  - Linux (systemd) y macOS (launchd).

## Prueba en el centro de cómputo de la escuela con 0.4.0-dev (29/10/2026)

- **Entorno:** red local del centro de cómputo; la laptop del profesor como consola; instaladores
  locales `0.4.0-dev` (`docs/desarrollo/instaladores-locales.md`, commit `c422f35`).
- **Resultado** (reportado por el usuario):
  - el alumno se instaló bien en **4 equipos**, con el agente como servicio;
  - la vista de pantallas mostró las **miniaturas** y la **vista en detalle**;
  - la **transmisión de la pantalla del profesor** se vio correctamente en los 4.
- **Queda por probar del agente como servicio** (`docs/prueba-vms.md` §13):
  - reiniciar sin iniciar sesión (SV-B);
  - cerrar la ventana a la fuerza (SV-E);
  - cambiar de usuario (SV-H);
  - que un alumno estándar no pueda detenerlo (SV-F).
- **Queda medir** CPU y subida del profesor al transmitir (§12.3).

## Etapa 10.2: bloqueo de apps y filtro web (D-017) — notas

- **Diseño aprobado** (2026-10-29):
  - `app.block.set/clear/ack` y `web.filter.set/clear/ack`;
  - capacidades `app.block` y `web.filter`, solo con el agente como servicio y su ayudante;
  - filtro DNS local con NRPT, no proxy;
  - cierre de procesos por sondeo;
  - diario para deshacer siempre;
  - reglas de privacidad 20–25.
- **`internal/protocol`:** los 6 mensajes con validación (nombres de proceso, dominios en
  punycode, duplicados, límites 100 y 500), ejemplos válidos e inválidos y `AppKey`/`DomainKey`.
- **`internal/platform`:**
  - `ProcessControl`: Toolhelp, filtro por sesión y ruta, terminar;
  - `DNSRedirect`: NRPT con cmdlets y marcada `aula-filtro-web`, vaciar caché, DNS y sufijos de
    los adaptadores;
  - `BrowserDoH`: leer, apagar y restaurar exacto las directivas de Chrome, Edge y Firefox,
    incluidas las claves creadas;
  - fallback "no soportado".
- **`internal/policy`** (nuevo):
  - `AppPolicy` y `AppEnforcer`: protegidos y aviso agrupado;
  - `WebPolicy`: subdominios; siempre bloquea el canario y los DoH conocidos; con lista blanca
    siempre deja pasar lo que Windows necesita;
  - `Resolver` (`miekg/dns`, UDP y TCP, no registra nombres);
  - `Journal`;
  - `Manager`: aviso antes de aplicar, deshace todo si un paso falla, `Clear`, `Restore`;
  - `ClassPolicy` en la consola: destinatarios por nombre de equipo, reenvío al reconectar,
    `clear` a quien deja de estar marcado y acuses.
- **Alumno (servicio):**
  - `Restore` antes de conectarse;
  - se limpia al desconectarse, cuando se va el ayudante y al detenerse el servicio;
  - `servicio desinstalar` también restaura;
  - franja "🔒 Restricciones de la clase activas…" con **Ver detalles**, aviso "Aula cerró «…»"
    y barra "· restricciones activas".
- **Consola:**
  - botón **Restricciones**: apps y sitios, modo, plantillas, todo el canal o alumnos marcados
    (casilla en la tarjeta);
  - estado "activa en M de N", **Quitar** y **Quitar todas las restricciones**;
  - texto de límites (regla 25) y "🔒 apps / 🔒 sitios" en las tarjetas;
  - cerrar la clase envía los `clear`.
- **Pruebas locales:**
  - políticas y protegidos;
  - aplicador (cierra, avisa una vez cada 30 s, sin política ni sesión no cierra);
  - resolvedor real en loopback contra un DNS falso (bloquea, reenvía por UDP y TCP, el
    canario, sin DNS de origen → SERVFAIL);
  - `Manager` con dobles:
    - orden aviso → diario → directivas → NRPT;
    - reemplazo sin tocar el equipo;
    - quitar deja el "registro" **idéntico**;
    - un fallo al aplicar no deja nada a medias;
    - **caída**: un `Manager` nuevo restaura desde el diario, y sin diario borra reglas
      huérfanas;
  - `ClassPolicy`;
  - agente-servicio contra consola real (sin ayudante no declara `app.block`, aviso y cierre,
    limpieza cuando se va el ayudante);
  - bindings de la consola;
  - Windows sin administrador: listar y cerrar un proceso hijo real, leer DNS y directivas,
    vaciar la caché.
- **Requiere administrador (job de CI `servicio`):**
  - NRPT: aplicar, borrar, sin residuos;
  - directivas DoH restauradas exactas, con una directiva previa de la escuela que sobrevive;
  - **punta a punta por el resolvedor del sistema**: con el filtro, `youtube.com` no resuelve y
    `github.com` sí; al quitarlo, todo como antes.
- **CI en GitHub (29/10/2026):** pasaron las tres pruebas con administrador:
  - regla NRPT sin residuos;
  - directivas DoH restauradas exactas;
  - de punta a punta por el resolvedor del sistema: `youtube.com` bloqueado, `github.com` resuelve
    y ninguna regla de Aula al quitarlo.

  En el camino se corrigieron tres cosas de las pruebas, no del filtro:
  - una carrera en la prueba del resolvedor;
  - la lectura de reglas desde Go (necesita `Out-String`);
  - correr las pruebas de administrador de un paquete a la vez.

  Detalles en D-017.
- **Falta:**
  - la prueba en VM de `docs/prueba-vms.md` §14, en especial RB-H (equipo idéntico), RB-J (caída)
    y RB-K (apagado);
  - probar la interfaz de la consola con clics;
  - Linux y macOS.

## Etapa 10.3: energía (apagar, reiniciar, cerrar sesión, D-018) — notas

- **Diseño aprobado** (2026-10-29):
  - `power.action`, `power.cancel` y `power.ack` (`docs/protocol.md` §5.15, con permisos del SO
    en §5.15.1);
  - capacidad `power`;
  - posponer 5 min como máximo 2 veces;
  - la cuenta sigue sin conexión;
  - sin sesión iniciada el servicio declara solo `power` (apagar y reiniciar inmediatos; cerrar
    sesión → `unsupported_capability`);
  - reglas de privacidad 26–31 y ajuste de las reglas 17 y 19.
- **`internal/protocol`:** los tres mensajes con validación (`delay_s` 10–600 o 0 = 30 s,
  estados, límites), fixtures válidos e inválidos y capacidad requerida.
- **`internal/platform`:** interfaz `Power` (`Shutdown`, `Reboot`, `Logoff`) y `CurrentSession`.
  En Windows: `InitiateShutdownW` sin gracia tras habilitar `SeShutdownPrivilege`;
  `WTSLogoffSession` con `force` y `ExitWindowsEx(EWX_LOGOFF)` en la propia sesión sin `force`.
  Fallback "no soportado" en Linux y macOS. Las pruebas solo comprueban que el privilegio se
  resuelve y la sesión se lee; **nunca llaman a las acciones**.
- **`internal/power`** (nuevo, sin APIs del SO):
  - `Countdown` (agente): el aviso antes del `ack`; posponer; cancelar; una orden reemplaza a la
    anterior; `executing` registrado y enviado antes de llamar al SO, con hasta 2 s para que salga;
    sobrevive a la desconexión; tras ejecutar sin que el equipo se apague (el alumno canceló en
    Windows), el aviso se quita a los 2 min;
  - `Tracker` (consola): última orden por sesión, su estado y su vencimiento.
- **Alumno:**
  - modo ventana y servicio declaran `power` si hay `platform.Power`;
  - franja roja fija con la cuenta en vivo, **Posponer 5 min (te quedan N)**, ventana al frente y
    siempre encima, y barra "· reinicio pendiente";
  - en modo servicio, el cierre de sesión sin forzar lo hace el ayudante (*proxy*
    `agentsvc.Logoff`), y el evento `ontop` llega a su ventana.
- **Consola:**
  - botón **Energía**: Apagar, Reiniciar o Cerrar sesión; todo el canal o los marcados; cuenta de
    30 s a 5 min; **Forzar (sin cuenta regresiva)**;
  - confirmación explícita con `<dialog>`: "Volver" tiene el foco, Esc cancela, y con forzar
    advierte en rojo que se pierde lo no guardado;
  - estado en las tarjetas ("⏻ reinicio en 0:25 · pospuesto (1/2)") y franja de pendientes con
    **Cancelar**.
- **Pruebas automáticas (todas con dobles; ninguna apaga el equipo):**
  - `internal/power` con reloj falso: se ejecuta una sola vez y solo al vencer; `delay_s`;
    posponer dos veces y el tercero falla; `force` sin cuenta y con el `ack` antes del SO (orden
    comprobado); cancelar e ignorar otro `action_id`; reemplazo; sin sesión; desconexión; fallo
    del SO;
  - `apps/student`:
    - una consola que envía `power.action` **antes de `auth.ok`** recibe `unauthorized` y el doble
      no se llama;
    - con consola real: `countdown` → `postponed` → `executing` y una sola llamada al doble;
    - como servicio: sin ayudante solo `power` y reinicio inmediato; con ayudante, posponer por el
      binding y cierre de sesión por el ayudante;
  - `apps/teacher`: destinatarios, sin capacidad, marcados, forzado, entradas inválidas, acuses,
    errores y cancelar solo lo pendiente; contrato de eventos y bindings.
- `gofmt`, `go vet` (Windows, Linux y macOS), `staticcheck` y `go test -race ./...` sin errores.
  En una de dos corridas completas, `internal/screen` e `internal/broadcast` (que esta etapa no
  toca) fallaron por tiempos bajo carga; solos y en la segunda corrida completa pasan.
- **Prueba en el laboratorio (29/10/2026, reportada por el usuario):** apagar 1 equipo sin forzar.
  La cuenta regresiva y el posponer funcionaron, pero al final el alumno mostró "No se pudo apagar
  este equipo". **Causa:** con el agente como servicio, `InitiateShutdownW` sin forzar se llamaba
  desde SYSTEM, y Windows lo rechaza con `ERROR_SHUTDOWN_USERS_LOGGED_ON` mientras el alumno tiene
  sesión iniciada. Las pruebas usan un doble y no podían verlo. **Corregido:**
  - sin forzar y con sesión iniciada, apagar, reiniciar y cerrar sesión los ejecuta el ayudante
    en la sesión del alumno (`agentsvc.Power`, `Countdown.InSession`);
  - con forzar, o sin sesión, sigue haciéndolo el servicio;
  - el `error` a la consola incluye ahora el motivo del SO;
  - pruebas nuevas: qué camino toma cada acción, y el caso del laboratorio de punta a punta con
    el servicio y un ayudante doble.

  **Confirmado en el laboratorio** (29/10/2026, reportado por el usuario): con el agente
  corregido, el apagado sin forzar de 1 equipo se ejecutó al terminar la cuenta.
- **Falta:**
  - la prueba en VM de `docs/prueba-vms.md` §15 (PW-A a PW-I; aceptación con PW-A, PW-B y PW-D);
  - probar la interfaz de las dos apps con clics;
  - Linux y macOS (cuando se indique).

## Etapa 17: diagramas iniciales — notas

- **Hechos** en `docs/v1/diagramas/`, cada uno con front-matter, bloque Mermaid sin
  `htmlLabels`, «Lo que no es obvio» y «Fuentes en el código»:
  - 01 contexto, 02 componentes (dependencias reales de `go list`), 03 descubrimiento por canal,
    04 autenticación y fijado de certificado, 05 ciclo de sesión, 06 emisión de código,
    07 acuses (la recolección `code.collect`/`file.collect` sigue reservada y solo se menciona),
    08 miniaturas (pantalla en blanco pendiente, solo se menciona), 09 avisos al alumno,
    11 modelo de datos (qué persiste y qué solo vive en memoria), 12 «Mi alumno no conecta»
    (con los mensajes reales de `Agent.diagnose`);
  - **nuevos, fuera del índice original:** 13 agente como servicio, 14 transmisión WebRTC,
    15 restricciones (apps y filtro web, con su diario), 16 energía (máquina de estados).
- **Pendiente:** 10 «Aula simulada y simulación de fallas», porque el modo simulado no existe
  todavía (etapa 18); en el README figura como «(pendiente)», sin enlace.
- **README:** se corrigió la tabla (la fila separadora tenía 5 columnas), se agregaron 13–16, las
  cuatro `classDef` exactas, los colores `rgb(...)` para `box`/`rect` de `sequenceDiagram`
  (no admite `classDef`), `erDiagram` sin color, la forma de `enlaces` (`nodo=destino`) y un
  ejemplo de front-matter con valores válidos.
- **Validador** (`docs/doc.go`, `docs/diagramas_test.go`, solo biblioteca estándar):
  - parser estricto del subconjunto YAML del README;
  - campos contra lo real: `pantallas` = `id` de `<section>` de
    `apps/teacher/frontend/dist/index.html`; `secciones_manual` y destinos de `enlaces` =
    documentos de `docs/` existentes, con ancla de GitHub si la hay;
  - título = encabezado `#` = fila del README (con el mismo tipo y la misma audiencia);
  - un solo bloque `mermaid` con `init` sin `htmlLabels`, las cuatro clases en
    `flowchart`/`stateDiagram-v2`, colores de la paleta en `sequenceDiagram` y sin `;` en sus
    mensajes (GitHub los corta);
  - que existan las rutas citadas en «Fuentes en el código»;
  - que todo `{{diagrama:NN-nombre}}` de `docs/` (salvo `docs/v1/prompts/` y el código) exista;
  - `TestValidateRejects` prueba con 25 diagramas rotos que el validador falla. A mano: borrar el
    front-matter del 04 hace fallar `go test ./docs`.
- **Verificado:** los 15 bloques renderizan con `@mermaid-js/mermaid-cli` 11 (instalado solo en
  un directorio temporal, no se versiona) y ningún SVG lleva `foreignObject`. Falta verlos en
  GitHub tras empujar a `develop`.
- **No implementado (etapa del render):** SVG claro/oscuro, `scripts/docs/generar-diagramas.*`,
  manifiesto y chequeo de hash.
- **Para la etapa 18:** al crear la navegación y el manual en `docs/administracion/`, actualizar
  `pantallas` y `secciones_manual` (la prueba fallará hasta hacerlo si se renombran las
  secciones), agregar el diagrama 10 y generar el árbol interactivo a partir del 12.

## Etapa 18: pulido de la consola — notas

- **Fase 1, análisis** (`docs/v1/ux/profesor/ANALISIS-CONSOLA.md`, commit `b27c408`):
  - 26 capturas del frontend real en Chromium headless, con un doble de los bindings;
  - revisión WCAG con axe-core;
  - hallazgos priorizados;
  - decisiones del usuario: Alumnos como inicio, acciones sobre la selección en paneles
    laterales, conservar el historial al cambiar de nombre o canal con confirmación,
    «Reenviar a los que faltan» y Aula simulada en la navegación principal.
- **Fase 2, sistema de diseño** (`docs/v1/ux/profesor/FASE2-SISTEMA-DISENO.md`):
  - tokens, componentes e iconos en `apps/teacher/frontend/dist/sistema/` (todavía no los usa
    la consola);
  - catálogo y prototipo de 3 pantallas en `apps/teacher/frontend/dev/`, fuera del binario,
    que se ven con `go run ./tools/vistaprevia`;
  - prueba de contraste AA `TestTokensContrast`;
  - 12 capturas sin violaciones de axe.
- **Fase 2 aprobada** (commits `63b337b`, `97912f7` y `b194ade`), con las decisiones de la
  ronda 2 en `FASE2-SISTEMA-DISENO.md`.
- **Aula simulada, Fase 6.1** (`docs/v1/ux/profesor/ENTREGA-AULA-SIMULADA.md`):
  - `internal/simclass`: alumnos simulados en proceso, con el protocolo real por un puerto de
    `127.0.0.1` sin anuncio, y sus fallas (lento, no acusa, desconectado);
  - la consola los separa por el registro al que llegan: se cuentan aparte y nunca reciben
    energía, restricciones ni transmisión;
  - bindings `Simulation*` y evento `aula:simulation`;
  - botón mínimo en la consola actual;
  - reglas de privacidad 32–35 y diagrama 10.
- **Aula simulada aprobada** (commits `085e9f9`, `28b617d`, `87cc9fa` y `51f4b11`).
- **E3, la consola con la navegación nueva** (`docs/v1/ux/profesor/ENTREGA-E3.md`):
  - frontend reescrito sobre el sistema de diseño: Alumnos con selección y paneles, Código con
    quién falta y reenviar, Aula simulada, Ayuda, «¿Falta un alumno?» y Ajustes;
  - textos en `textos.es-MX.js` (probado);
  - backend: historial conservado al cambiar de canal o de nombre, envío a la selección,
    `Resend` e `InstallerInfo`;
  - 27 capturas sin errores de JS ni violaciones de axe;
  - falta la prueba en la ventana real.
- **E3 aprobada** (commits `1bc0e2c`, `67830d5` y `1baed73`) y guía
  `docs/desarrollo/probar-consola-local.md` (`98b51a7`).
- **E2, errores con código** (commits `cd429c6` y `e45e0e5`): cada error de los métodos lleva
  un código estable (`apps/teacher/errores.go`, hoy 35 códigos) y la interfaz muestra su texto de
  `textos.es-MX.js`, con el detalle técnico plegado. Lo prueban `TestErrorCodesHaveTexts`,
  `TestBindingsReturnCodedErrors` (21 fallos típicos) y `TestPortInUseIsCoded`.
- **Pendiente de E2:** los códigos de error del protocolo que mandan los alumnos (hoy solo
  afectan a la energía, que muestra «no se pudo»).
- **E7, el manual en la consola** (`docs/v1/ux/profesor/ENTREGA-E7.md`, commits `1c99c1d`,
  `f0b7260` y `8e5cc21`):
  - Markdown en `docs/v1/administracion/` (única fuente), embebido por el paquete `docs`;
  - `internal/manual` lo lee sin HTML, lo valida, lo busca y saca el asistente «¿Falta un
    alumno?» del diagrama 12;
  - Ayuda con índice, búsqueda y enlaces; «?» por pantalla; cada error enlaza a su sección;
  - pruebas: una página por pantalla, enlaces y secciones válidos, el árbol del diagrama 12 y
    el enlace de cada código de error;
  - falta la prueba en la ventana real (C-21 a C-25).
- **Pendientes de las Fases 3 y 4** (`docs/v1/ux/profesor/ENTREGA-FASES-3-4.md`,
  aprobados):
  - los errores del protocolo que mandan los alumnos llegan a la consola: «No pudo: …» en el
    alumno y en «Quién falta», con texto y enlace al manual, y «Reenviar» los reintenta;
  - ficha de cada alumno, columna Sistema, «i» de información en los campos técnicos y
    vencimiento del certificado en Ajustes;
  - el arnés web pasa al repo (`scripts/ux/`): recorrido con axe-core y capturas del manual;
  - propuestas que cambiarían el protocolo en `docs/v1/ux/PROPUESTAS.md`;
  - falta la prueba en la ventana real (C-26 a C-30).
- **Salvaguardas para las funciones nuevas** (commits `a4e42c6`, `c6bb241` y `32881a8`):
  - «Definición de terminado» en `CLAUDE.md`;
  - pruebas en `apps/teacher/guardrails_test.go`: el doble del arnés tiene todos los bindings,
    cada capacidad implementada sale en la ficha del alumno, y los simulados solo declaran
    capacidades inofensivas;
  - job `ux` del CI con el arnés (sin probar todavía en GitHub);
  - la plantilla 10 y los prompts 10.4, 10.5, 10.7 y 12.1 apuntan a la consola nueva (ficha del
    alumno, selección y paneles).
  - comandos `/funcionalidad-nueva` y `/funcionalidad-mejora` en `.claude/commands/`, con su
    guía y ejemplos en `docs/desarrollo/comandos-claude.md`.
- **Siguiente:** el plan de cierre de abajo.

## Etapa 18: plan de cierre

Lo que falta para cerrar el prompt 18 (`docs/v1/prompts/18-pulido-consola-ux-manual-simulador.md`):
las Fases 5, 6.3 y 7, y el paquete `internal/ui`, que `CLAUDE.md` ya describe pero que todavía no
existe. Cada paso es una entrega: capturas del arnés, resumen, aprobación del usuario y commit en
`develop`. Se sigue en orden. Para empezar una sesión nueva: «Lee docs/progreso.md y continúa con
el plan de cierre de la etapa 18».

Estado al escribir este plan (04/11/2026):
- `develop` va 27 commits por delante de `origin/develop` (último push: `5af6107`), así que el job
  `ux` del CI nunca ha corrido en GitHub.
- No existen `internal/ui/`, `CHANGELOG.md`, `scripts/docs/` ni `docs/v1/ux/profesor/despues/`.
- La evaluación de los diagramas ya está hecha y aprobada (`ANALISIS-CONSOLA.md` §9): SVG
  pre-renderizados claro y oscuro con mermaid-cli y, si falta uno, el texto Mermaid. Pesan unos
  270 KB comprimidos, frente a un ejecutable de 19,6 MB. La paleta oscura de los diagramas ya está
  aprobada.
- Versiones probadas en el arnés: mermaid-cli 11.17.1, Puppeteer 25.16.0 y axe-core 4.14.0.

### Paso 0 — Antes de empezar (usuario)

- ✅ Push hecho (`eda8bc2`); el primer run del CI pasó completo, incluido el job `ux` (run
  41896892640). El job `ux` tardó 25 min, casi todo en `npm ci`.
- Hacer push de `develop` y revisar en GitHub el primer run del job `ux`. El riesgo conocido es el
  sandbox de Chromium en Ubuntu 24.04; `ci.yml` ya tiene el `sysctl` que suele hacer falta.
- Probar en la ventana real los casos C-01 a C-30 de `docs/desarrollo/probar-consola-local.md`.
  Se puede ir haciendo en paralelo con los pasos siguientes.

### Paso 1 — E12: el paquete `internal/ui` — ✅ hecho y aprobado

Hecho el 04/11/2026:
- `internal/ui` embebe `tokens.css`, `componentes.css` e `iconos.svg`; `ui.Assets(app)` los
  monta bajo `sistema/` sobre el frontend de cada app, y oculta cualquier `sistema/` propio de la
  app. `TestTokensContrast` vive en `internal/ui/contrast_test.go`.
- Las dos apps sirven `frontend()` (`apps/*/assets.go`). La del alumno ya recibe `sistema/`,
  aunque su HTML todavía no lo usa (prompt 19).
- `tools/vistaprevia` sirve `dist/sistema/` desde `internal/ui` en disco, y el arnés
  (`scripts/ux/arnes.mjs`) también.
- Pruebas: `TestAssetsMountsTheDesignSystem` (`internal/ui`) y
  `TestFrontendServesTheSharedDesignSystem` (`apps/teacher`), que falla si una app vuelve a
  guardar su copia.
- El recorrido pasa sin errores (41 capturas) y las capturas del manual salieron idénticas byte a
  byte.

Plan original:


- **Qué:**
  - mover `apps/teacher/frontend/dist/sistema/` (`tokens.css`, `componentes.css`, `iconos.svg`)
    a `internal/ui/`, con `go:embed`;
  - mover también `apps/teacher/sistema_test.go` (`TestTokensContrast`);
  - la consola sirve un solo sistema de archivos: `frontend/dist` más `internal/ui` bajo
    `sistema/` (`assetserver.Options.Assets` en `apps/teacher/main.go`). Las rutas `sistema/…` del
    HTML no cambian;
  - la app del alumno solo queda preparada; su rediseño es del prompt 19.
- **También cambian:** `scripts/ux/arnes.mjs` (servir `sistema/` desde `internal/ui`),
  `tools/vistaprevia` y el catálogo de `apps/teacher/frontend/dev/`.
- **Prueba:** la consola sirve `sistema/tokens.css` desde `internal/ui`, y no queda ninguna copia
  de `sistema/` en `apps/`.
- **Terminado:** el recorrido y las capturas del manual salen iguales, y `go build` funciona sin
  Node.

### Paso 2 — E8: los diagramas dibujados en la consola (Fase 5) — ✅ hecho y aprobado (`5d8f59a`)

Hecho el 04/11/2026 (`docs/v1/ux/profesor/ENTREGA-E8.md`):
- `scripts/docs/generar-diagramas.{mjs,sh,ps1}`: SVG claro y oscuro de los 16 diagramas en
  `docs/v1/diagramas/generados/`, versionados, con su manifiesto. Usa Segoe UI (se niega sin ella)
  y semilla fija, así que regenerar da los mismos bytes.
- `TestDiagramsAreDrawn` y `TestDrawingsAreSanitized` (`docs/dibujos_test.go`).
- `internal/manual`: SVG como `data:` solo si el hash coincide, explicación, pantallas y
  `{{diagramas}}`.
- Consola: visor con zoom y «Ver en grande», galería con filtro y búsqueda, y diagramas en el «?».
- El binario crece 1,79 MB (25,26 → 27,05 MB).
- Recorrido: 47 capturas sin errores.

Plan original:


- **Pipeline** `scripts/docs/generar-diagramas.{sh,ps1}`, con su propio `package.json` y
  mermaid-cli 11.17.1 fijo. Por cada diagrama:
  - un SVG claro y uno oscuro, con el tema según los tokens, en `docs/v1/diagramas/generados/`;
  - su texto alternativo;
  - una entrada en `manifiesto.json`: id, metadatos, hash del bloque Mermaid y rutas;
  - limpieza con lista blanca: sin `script`, `on*`, `foreignObject` ni URLs externas.
- **Prueba Go** (`docs/`):
  - cada diagrama tiene sus dos SVG;
  - el hash coincide con su bloque Mermaid; si no, falla con «Diagramas desactualizados: ejecuta
    scripts/docs/generar-diagramas»;
  - los SVG están limpios.

  Compara el hash del texto Mermaid, no el SVG, así que da igual en los tres sistemas operativos.
- **`internal/manual`:** `Diagram` agrega los SVG y la prosa «Lo que no es obvio», y `ManualView`
  los manda a la interfaz.
- **Visor** (componente del sistema de diseño, en `internal/ui`):
  - se ajusta al ancho, con zoom y «ver en grande» en el diálogo `#diagrama`;
  - cambia de SVG con el tema;
  - muestra título, descripción, público y la explicación;
  - se usa con el teclado;
  - si falta el SVG, muestra el texto Mermaid, como hoy.
- **Dónde aparecen:**
  - la galería «Cómo funciona Aula», con filtro por público y búsqueda (la búsqueda ya incluye
    los diagramas);
  - las inclusiones `{{diagrama:…}}`;
  - el «?» de cada vista, con los diagramas cuyo front-matter `pantallas` la nombra.
- **También:**
  - `scripts/ux/dobles.js` con los SVG;
  - en `recorrido.mjs`, el visor, la galería y el tema oscuro;
  - un paso en el CI;
  - medir cuánto crece el binario.

### Paso 3 — E11: comprobador de conexión (Fase 6.3) — ✅ hecho (espera aprobación)

Diseño aprobado el 04/11/2026 (`docs/v1/ux/profesor/DISENO-E11.md`); hecho el mismo día
(`docs/v1/ux/profesor/ENTREGA-E11.md`):
- `internal/connectcheck`: anuncio, puerto, conexión segura con `Agent.Probe` (cierra antes de
  `hello`) y red y firewall. `Outcomes` lista los resultados de cada paso y la prueba de textos
  exige uno por cada.
- `internal/session`: `Cause`, que el agente emite con su diagnóstico (`OnCause`), y `Probe`, que
  comparte con el agente el mismo `connect`.
- `internal/platform`: `NetworkInspector`. En Windows usa PowerShell con `HNetCfg.FwPolicy2`
  (las cmdlets `NetSecurity` piden administrador) y salida JSON, sin abrir ventana.
- Consola: binding `CheckConnection`, evento `aula:connectcheck` y error `checkRunning`; tarjeta
  «Comprobar la conexión» arriba de «Vamos paso a paso».
- Prueba 7.3 (`TestCheckerAgreesWithTheAgent`): sin anuncio, otra huella, otra clave y puerto
  cerrado dan la misma causa que un agente real. Se comprobó que detecta una mutación.
- Probado en la red real de este equipo: la red «Wucol-2 2» es Pública y el paso 4 lo señala.
- El árbol del diagrama 12 no cambió: el comprobador va antes que sus preguntas; solo cambió «Lo
  que no es obvio».
- Recorrido: 50 capturas sin errores.

Plan original:

- **Qué:** recorre desde la consola el camino de un alumno y dice en qué paso fallaría:
  1. el anuncio mDNS del canal se ve en la red (un `discovery.Browser` del propio canal);
  2. el puerto TCP responde en cada dirección de la LAN (no en la de loopback);
  3. TLS con `security.ClientTLS`, fijado a la huella anunciada;
  4. el reto y la firma con la clave pública del centro, como lo haría un agente;
  5. el perfil de red (Pública o Privada) y la regla del firewall, por una interfaz nueva en
     `internal/platform` (Windows; en el resto, «no se puede comprobar aquí»).
- **Límites, dichos con honestidad:** lo que solo se ve desde el equipo del alumno (el firewall
  de entrada, un canal distinto) se explica como tal y remite a «¿Falta un alumno?» y a
  `aula-cliente diagnostico`.
- **Dónde va:**
  - la lógica en `internal/*` (paquete nuevo, que se agrega a `CLAUDE.md`, o dentro de
    `session`);
  - la interfaz en «¿Falta un alumno?»: pasos con mensaje claro, sección del manual y enlace al
    diagrama 12;
  - cada fallo usa las mismas causas que `session.Agent.diagnose`.
- **Prueba (Fase 7.3):** con fallas inducidas (sin anuncio, otra huella, otra clave), el
  comprobador reporta la misma causa que el diagnóstico real del agente.
- **Terminado:** cumple la definición de terminado de `CLAUDE.md` (errores con código, manual,
  `dobles.js` y recorrido).

### Paso 4 — Lo que queda de las Fases 3, 4 y 6 — ✅ hecho (espera aprobación)

Hecho el 04/11/2026 (`docs/v1/ux/profesor/ENTREGA-P4.md`):
- Atajos: tabla en Tareas frecuentes. Al cerrar un panel lateral, el foco vuelve al botón que lo
  abrió (antes se perdía).
- Formularios: ayudantes `errorEnCampo`, `limpiarCampo` y `ocupado` en `main.js`.
  - El arranque ya no borra lo escrito cuando «Abrir la clase» falla, y marca el campo culpable.
  - Canal, nombre y aviso muestran junto al campo el error que devuelve Go.
  - Ningún botón de envío se puede pulsar dos veces mientras espera; «Emitir» mandaba dos
    ejercicios.
- Carga: «Cargando el manual…». El salto a una sección del manual espera a que se decodifiquen
  los diagramas de arriba, que la empujaban.
- Primer uso (6.1) y Fase 4.3: sin cambios, como estaba previsto.
- Recorrido: 52 capturas sin errores.

Plan original:
- **Atajos de teclado (3.6):** su lista en el manual. Revisar Esc en paneles y diálogos y Ctrl+A,
  y agregar solo los que falten.
- **Formularios (3.5):** validación en línea y no perder lo escrito al fallar, en canal, nombre,
  aviso, restricciones, energía y aula simulada.
- **Rendimiento percibido (3.7):** estados de carga al abrir la clase, al cargar el manual y al
  enviar.
- **Primer uso (6.1):** el estado vacío ya ofrece el aula simulada; la prueba del usuario nuevo
  dirá si alcanza.
- **Fase 4.3:** el enlace con el diagnóstico de la etapa 15 queda anotado para esa etapa.

### Paso 5 — E10: verificación y cierre (Fase 7) — ✅ hecho (espera aprobación)

Hecho el 04/11/2026 (`docs/v1/ux/profesor/RESULTADOS.md`):
- `scripts/ux/teclado.mjs` (en el CI): Tab por las 5 vistas, foco visible, sin trampas, y Esc en
  paneles y diálogos con el foco de vuelta. Comprobé que falla si se quita el contorno de foco.
- `scripts/ux/despues.mjs`: las 26 capturas de `despues/`, con los mismos estados que `antes/`.
  Destaparon dos fallos que se corrigieron:
  - la miniatura «Esperando imagen…» tenía un nombre accesible sin su texto visible (WCAG 2.5.3);
    ahora está también en el recorrido;
  - el aviso del atajo ocupado lo armaba Go y nombraba un botón «Capturar» que ya no existe;
    ahora Go manda `HotkeyStatus.Reason` y el texto vive en `textos.es-MX.js`.
- `scripts/ux/usuario-nuevo.mjs` (en el CI): «Primeros pasos» con 4 clics y una escritura, y
  cada control que nombra el manual existe con esa etiqueta.
- `RESULTADOS.md`: 23 de los 24 hallazgos de §4 resueltos y A2 parcial (Alt+1…5); del flujo a
  faltan el filtro y el orden de Alumnos. axe en 0.
- `CHANGELOG.md` nuevo, con la etapa 18 en «Sin publicar».
- Binario: 23,07 → 25,86 MB (+12 %) frente a `5af6107`; sin Node para compilar; sin URLs
  externas.
- Release sugerido: **`v0.4.0-rc.1`**. Sube `y` porque hay funciones nuevas compatibles, sin
  cambios de protocolo ni de configuración. La última etiqueta es `v0.3.0-rc.2` y la 0.3.0 nunca
  salió como estable.

Pendiente de tu lado:
- la prueba del usuario nuevo en una VM limpia, con el instalador local y solo el aula simulada;
- los casos C-01 a C-43 en la ventana real.

Plan original:


- **Teclado:** `scripts/ux/teclado.mjs` recorre cada vista con Tab, comprueba el foco visible, que
  Esc cierra y que no hay trampas de foco. Va al CI.
- **Capturas «después»** en `docs/v1/ux/profesor/despues/`: las mismas pantallas que `antes/`.
- **`docs/v1/ux/profesor/RESULTADOS.md`:**
  - antes y después por pantalla;
  - pasos y clics de los flujos a–d, antes frente a después;
  - cada hallazgo de `ANALISIS-CONSOLA.md` §4, resuelto o pendiente;
  - resultados de axe.
- **Usuario nuevo (meta: menos de 5 minutos):**
  - Claude sigue «Primeros pasos» en el arnés y cuenta los pasos;
  - el usuario lo hace en una VM o instalación limpia, con el instalador local
    (`docs/desarrollo/instaladores-locales.md`) y solo el aula simulada, y anota lo que confunda.
- **`CHANGELOG.md`** nuevo, con la etapa 18 bajo «Sin publicar».
- **Binario:**
  - su tamaño frente al commit anterior a la etapa 18;
  - `go build` sin Node;
  - `frontend/dist` sin URLs externas;
  - `go test -race` verde en los tres sistemas operativos (CI).
- **Cierre:** etapa 18 en ✅ en la tabla y la versión sugerida para el release (sube `y`:
  funciones nuevas compatibles; la última etiqueta es `v0.3.0-rc.2`).

### Después del cierre

- Prompt 19 (app del alumno), que ya usará `internal/ui`.
- Prompt 20 (auditoría y limpieza de `docs/v1/ux/`), después de publicar el release.
- Las propuestas de `docs/v1/ux/PROPUESTAS.md` se deciden aparte.

### Problemas conocidos

- En el equipo de desarrollo, `go test -race ./...` con todos los paquetes en paralelo a veces
  falla en `internal/screen` («nothing sent», espera de 3 s) y en `internal/transport`
  (`TestOversizedMessageClosesWith1009`). Solos pasan siempre, y con `-p 4` pasa la suite
  completa: es la carga de la máquina, no el código. En CI pasan.

### Decisiones (tomadas el 04/11/2026)

- **E8:** los SVG generados se versionan en `docs/v1/diagramas/generados/`, para que `go build`
  no necesite Node.
- **E11:** la lógica va en un paquete nuevo, `internal/connectcheck` (se agrega a `CLAUDE.md`).
- **E10:** Claude hace la prueba del usuario nuevo en el arnés; el usuario la repite en una VM
  limpia con el instalador local, cronometra y anota lo que confunda.
- **Paso 0:** Claude hace el push de `develop` y revisa el CI.

## Etapa 19: pulido de la app del alumno — notas

Prompt `docs/v1/prompts/19-pulido-app-alumno-ux.md`, en 7 entregas con aprobación y commit cada una:

| Entrega | Qué | Commit |
|---|---|---|
| A | Análisis: arnés del alumno (`dobles-alumno.js`, `recorrido-alumno.mjs`), `tools/medir-alumno`, `docs/v1/ux/alumno/ANALISIS.md` | `6611a49` |
| B | Sistema de diseño (`internal/ui`), `textos.es-MX.js`, Go manda estados y no frases | `ccfc067` |
| C | Lista de código: lo nuevo arriba, sin saltos, Ctrl+Mayús+C, `pruebas-alumno.mjs` | `2626f30` |
| D | Avisar sin foco, modo compacto, cerrar al área de notificación (`platform.AppWindow`, `platform.Tray`) | `42cabe4` |
| E | Avisos en una esquina, estados vacíos, `Status.Cause` en palabras llanas | `1381683` |
| F | Bandas de transparencia permanentes; pausa y control diseñados | `a6b1175` |
| G | Teclado, rendimiento, `despues/` y `RESULTADOS.md` | (este) |

Decisiones (04/11/2026):
- código y avisos sin quitar el foco; energía, restricciones y transmisión sí traen la ventana;
- pausa y control solo diseñados;
- modo compacto en la ventana, y cerrar al área de notificación;
- `docs/v1/ux/alumno/`, con las propuestas en el mismo `PROPUESTAS.md`, separadas por app.

En la entrega D, «mostrar sin activar» se interpretó así: una ventana minimizada se queda
minimizada y parpadea, porque restaurarla la pondría encima del editor. Es una línea en
`window.go` (`notify`) si se prefiere lo contrario.

Cifras: CPU 0,0–0,1 % de un núcleo; RAM 187–191 MB (+9 MB, casi toda de WebView2); ejecutable
14,30 → 14,56 MB.

Pendiente de tu lado: los casos A-01 a A-08 en la ventana real (icono, modo compacto, parpadeo).

## Prompts y comandos al día con las etapas 18 y 19 (05/11/2026)

Para que cada función pendiente respete lo implementado:
- **`CLAUDE.md`, «Definición de terminado»:**
  - reglas de la app del alumno: sistema de diseño, textos, `notify` frente a `raise`, bandas
    que no se cierran, arnés del alumno;
  - la vista «Clase» de la consola para lo que responden los alumnos, y las franjas globales
    para lo que sigue activo;
  - los casos A-xx.
- **`.claude/commands/funcionalidad-nueva.md` y `funcionalidad-mejora.md`:** la ficha pregunta
  dónde vive en cada app y si actúa sobre el equipo; leen el «Encaje» del prompt; prueban con el
  arnés del alumno. La plantilla 10 se actualizó igual.
- **Prompts 10.4 a 10.13, 11, 12.1 y 13 a 16:** cada uno trae «Encaje con lo implementado»
  dentro del bloque del prompt: dónde va en consola y alumno, qué componentes, qué diagramas
  actualiza, simulados y privacidad.
- **Correcciones:**
  - prerrequisitos de 10.4 (el servicio es 10.1);
  - el troceo de 10.9 (es el de 10.10);
  - el título del 12.1;
  - el estado del 15 (el diagnóstico ya está casi todo).
- **12** marcado como hecho; su punto 5 pendiente pasa a **`12.2-transmision-seleccion-y-modos.md`**,
  una mejora (`/funcionalidad-mejora 12.2`).
- Decisiones (05/11/2026):
  - una vista «Clase» nueva en el menú;
  - en el alumno, acciones en la tira y tarjetas en la lista;
  - el 12 como mejora;
  - una base común más el encaje por prompt.


## Etapa 10.7: mensaje privado a un alumno — notas

- **Diseño aprobado** (2026-11-05) como **mejora del aviso**, no como función nueva: ya existían
  `msg.show`, su acuse, `SendNoticeTo` con destinos y `ReplyToStudent` a un solo alumno. Faltaba
  que el alumno distinguiera lo que solo le llegó a él. Sin `msg.direct`: duplicaría el acuse, la
  capacidad y el conteo de envíos.
- **Protocolo** (`docs/protocol.md` §5.4): `msg.show` gana `personal` (opcional); la consola solo
  lo pone en un envío a **una** sesión. Un agente anterior lo ignora. Capacidad `msg.show`.
- **Consola:** «Mensaje privado» en la ficha (campo con Enter/Shift+Enter/Esc, Información o
  Advertencia) y su acuse debajo («Enviado… · ya lo vio»), de `aula:deliveries`; binding
  `SendPrivateNotice`, error nuevo `noticeUnsupported`. También en la pantalla en grande y en la
  vista del control remoto, junto a «Tomar captura» (agregado del usuario): al abrirlo se sueltan
  las teclas y lo escrito no le llega al alumno; Esc cierra el mensaje y no la vista. En «Últimos envíos» la etiqueta es
  «Mensaje privado a <equipo>», sin el texto. «Responder» de Participación también va `personal`.
- **Alumno:** el aviso se titula «Mensaje para ti» con el icono nuevo `sobre`; `notify`, nunca
  `raise`; sin bindings nuevos (`codedist.Notice.Personal`).
- **Simulados:** lo reciben y lo cuentan aparte (`private`).
- **Guarda nueva:** `internal/ui` comprueba que cada icono es un `<symbol>` del sprite y no texto
  dentro del comentario de uso (pasó dos veces al agregar iconos), y que todo icono usado por las
  apps existe.
- **Documentación:** regla 53 de `docs/privacidad.md`; manual (Tareas frecuentes › «Mandar un
  mensaje privado a un alumno» y la ficha en Alumnos); diagrama 06 con su SVG;
  `docs/eventos-ui.md`; casos C-59 a C-62 y A-17; `docs/prueba-vms.md` §19 (con PM-E durante el control).
- **Pruebas:** enrutamiento (solo al elegido, nada difundido, acuse contado, sin texto en el log
  ni en la etiqueta, errores); `personal` en el protocolo y en `codedist`; simulados; arnés
  (capturas 81–86 de la consola, con la comprobación de que nada sale por `SendRemoteInput`, 57–58 del alumno y el caso `aviso-privado`).
- **Falta de tu lado:** la prueba en VM (§19) y los casos manuales.

## Etapa 10.6: manos y mensajes del alumno — notas

- **Diseño aprobado** (2026-11-05) con un cambio del usuario: la vista se llama **«Participación»**
  y no «Clase», porque «Clase» ya es el grupo del menú. Se renombró en `CLAUDE.md` (Definición de
  terminado), en los prompts 00, 10, 10.6, 10.8, 10.10, 10.12, 10.13 y 14 y en el comando
  `/funcionalidad-nueva`. Decisiones: «pedir ayuda» es `hand.raise` con un motivo opcional (sin
  `help.request` aparte); la respuesta es `msg.show` con `reply_to`; sección de privacidad corta
  (reglas 50–52); paquete `internal/participation` (el nombre `classroom` chocaba con la interfaz
  `classroom` de la consola).
- **Protocolo** (`docs/protocol.md` §5.18): `hand.raise` (`hand_id`, `note` ≤ 140), `hand.lower`
  en los dos sentidos (`by` `student` o `teacher`), `msg.student` (`message_id`, `text` 1–500) y
  `msg.student.ack`; `msg.show` gana `reply_to` opcional. Capacidades `hand` y `msg.student`,
  que **inicia el agente**: `protocol.AgentCapability` y `session.Console` responden
  `unsupported_capability` a quien no las declaró. `session.Agent.OnConnect` da la conexión al
  agente para mandar lo que inicia él. Sin cambio de versión del protocolo (todo es agregado).
- **`internal/participation`** (nuevo, sin APIs del SO): `Board` de la consola (fila por orden de
  llegada, una mano por alumno que conserva su lugar al repetirse, recuerda la mano que bajó el
  profesor para no volver a ponerla al reconectar, mensajes deduplicados por `message_id`, ritmo
  por alumno —ráfaga 5 y 1 cada 3 s; manos 3 y 1 por s—, 20 mensajes por alumno, estados
  `new`/`replied`/`done`, `Reset` y `ForgetSimulated`) y `Desk` del agente (su mano, los mensajes
  sin acuse —máximo 5—, el ritmo propio, el reenvío con los mismos IDs al reconectar y la consola
  anterior que responde `unknown_type`).
- **Consola:** la vista **«Participación»** en el menú (grupo Clase) con el contador de pendientes
  (`.nav-lateral__contador`, nuevo en `internal/ui`) y la sección «Manos y mensajes» (fila con su
  número, mensajes con su estado, «Responder» en la tarjeta con Enter/Shift+Enter/Esc y «Bajar
  también su mano», «Bajar la mano», «Bajar todas» con confirmación, «Marcar como atendido»);
  insignias «Mano levantada · N» y «N mensajes» en Notas que llevan a la vista; aviso flotante al
  llegar (quién, nunca qué); «Levantar la mano» y «Escribirte mensajes» en la ficha. Bindings
  `Participation`, `LowerHand`, `LowerAllHands`, `ReplyToStudent`, `MarkMessageDone`,
  `SimulationHand`, `SimulationMessage`; evento `aula:participation`; errores `handGone`,
  `messageGone`, `studentGone` y `simTooSoon`. Iconos `mano`, `mensaje` y `participacion`.
- **Alumno:** «Levantar la mano» (`aria-pressed`) y «Mensaje al profesor» en la tira y en el modo
  compacto; la línea de estado de la mano (con «Agregar un motivo») y la tarjeta para escribir
  arriba de la lista; «Tus mensajes» con «Enviando…/Recibido/No se envió»; la respuesta como
  «Respuesta a tu mensaje» con la cita. `notify`, nunca `raise`; sin banda. Bindings `RaiseHand`,
  `LowerHand`, `SendTeacherMessage` (en modo servicio, por `CallBinding`); evento
  `aula:participation`; errores `aula:<código>` con `mano.error.*`. Declara `hand` y `msg.student`
  en modo ventana y, como servicio, con el ayudante.
- **Simulados:** declaran `hand` y `msg.student` y, en su fila de Aula simulada, «Levantar la
  mano» y «Escribir» (preguntas de ejemplo); lo que mandan se borra al cerrar el aula simulada.
- **Documentación:** `docs/privacidad.md` reglas 50–52 y regla 34; manual `participacion.md`
  (`pantallas: [participacion]`) con su captura; diagrama nuevo 18 y 06 y 10 actualizados, con sus
  SVG; `docs/eventos-ui.md`; casos C-54 a C-58 y A-14 a A-16; `docs/prueba-vms.md` §18;
  `CLAUDE.md` lista `internal/participation`.
- **Pruebas:** protocolo (validación, `testdata`, capacidades del agente); `participation` (orden,
  levantar y bajar, la mano que bajó el profesor, deduplicación, ritmo, tope, reenvío, rechazos);
  `session` (la capacidad de lo que inicia el agente y `OnConnect`); la consola (fila, insignias,
  acuse, respuesta con `reply_to` a un solo alumno, ritmo, cierre de clase, **el texto nunca en el
  log**); el alumno contra una consola real (mano, mensaje, acuse, respuesta, mano bajada por el
  profesor, sin `raise`, sin texto en el log, reenvío al reconectar); simulados de punta a punta.
- **Arnés:** escena `lab-participacion` y capturas 72–80 en `recorrido.mjs`; escenas `mano`,
  `mano-bajada`, `mensajes` y `sin-soporte` y capturas 45–56 en `recorrido-alumno.mjs`; casos de
  mano, motivo, mensaje (Enter/Esc/vacío), respuesta, sin conexión y compacto en
  `pruebas-alumno.mjs`.
- **Falta de tu lado:** la prueba en VM (§18) y los casos manuales.

## Etapa 10.5: captura de pantalla de un alumno — notas

- **Diseño aprobado** (2026-11-05) con un agregado del usuario: «Tomar captura» también en la vista
  del control remoto, sin terminarlo ni pausarlo. Decisiones aprobadas: distintivo nativo de 5 s
  antes de capturar (sin cambiar el de pantalla vista o control), `Documentos\Aula\capturas`,
  JPEG a calidad 90 y resolución nativa, captura también a los simulados (guardada aparte) y el
  límite del canal local del ayudante a 24 MiB.
- **Protocolo** (`docs/protocol.md` §5.17, permisos en §5.17.1): `screen.snapshot.request`
  (`snapshot_id`, `quality` 50–95, `max_width` 0 = nativa) y `screen.snapshot.response` en partes
  de 512 KiB (hasta 32, 16 MiB) con `taken_at`, equipo, usuario, tamaño y `sha256` en la parte 1;
  capacidad `screen.snapshot`, aparte de `screen.thumb` para no mandarla a agentes 0.6. Troceo
  común en `protocol.SplitParts` y `protocol.Reassembly` (lo reutilizará 10.10).
- **`internal/snapshot`** (nuevo, sin APIs del SO): `Taker` del agente (aviso antes de capturar,
  una a la vez, baja la calidad hasta que cabe, partes, aviso a la ventana, nada en disco ni en el
  log) y `Store` de la consola (ensambla, comprueba tamaño, huella y JPEG, 30 s de límite, máximo
  64 en memoria, `Save` en `capturas\<equipo> (<usuario>)\<fecha>\<hora>.jpg` sin sobrescribir;
  la ruta no va al log).
- **`internal/screen`:** `Streamer.Announce`: el distintivo con otro texto por un rato, salvo que
  ya haya una transmisión o un control (entonces no cambia).
- **`internal/platform`:** `DocumentsDir` (`FOLDERID_Documents`) y `OpenFolder` (Explorador), con
  fallback. `ScreenCapture` no cambió. **`internal/agentsvc`:** `maxFrame` de 4 a 24 MiB.
- **Alumno:** declara `screen.snapshot` con la captura (en servicio, si el ayudante captura). El
  aviso es un `Notice` con `kind: "snapshot"` y sin texto: la ventana arma «Captura de pantalla ·
  … tomó una captura de tu pantalla (HH:MM)» (icono nuevo `captura`). `notify`, nunca `raise`.
  Sin bindings nuevos.
- **Consola:** «Tomar captura» en la ficha y en la pantalla grande (también en modo control: suelta
  lo presionado antes de pedirla); vista previa `#captura` (tamaño real con un clic, Guardar,
  Descartar, ruta y «Abrir carpeta»); panel «Captura» en la barra de acciones con miniaturas y
  casillas; carpeta en Ajustes › Acerca de; «Tomar una captura de su pantalla» en «Qué puede
  hacer su equipo»; errores `snapshotUnsupported`, `snapshotTooMany`, `snapshotGone`,
  `snapshotSaveFailed` y `snapshotFolder`; evento `aula:snapshots`; iconos `captura`, `guardar` y
  `carpeta` y `.aviso-en-linea--exito` en `internal/ui`.
- **Simulados:** declaran `screen.snapshot` (su pantalla dibujada dice «alumno simulado») y cuentan
  las capturas; la guía de capacidades inofensivas lo permite a propósito (regla 34).
- **Documentación:** `docs/privacidad.md` regla 7 reescrita y reglas 44–49; manual (Tareas
  frecuentes › «Tomar una captura de la pantalla de un alumno», la ficha y la barra en Alumnos,
  Ajustes › «Capturas de pantalla»); diagramas 08, 09 y 17 con sus SVG; `docs/eventos-ui.md`;
  casos C-49 a C-53, A-12 y A-13 y `docs/prueba-vms.md` §17; `CLAUDE.md` lista
  `internal/snapshot`.
- **Pruebas:** troceo y validación en `protocol`; `Taker` y `Store` con un doble de
  `ScreenCapture` (orden aviso → captura → aviso a la ventana → partes, una a la vez, calidad,
  errores, partes ajenas, corrupta, tiempo límite, guardado organizado y sin sobrescribir, nada en
  disco sin «Guardar», **nada en disco ni en el log del agente**); `Announce`; la app del alumno
  (`notify` y no `raise`); simulados; los bindings de la consola, el lote y la captura durante el
  control (no manda `remote.stop` y la entrada sigue).
- **Arnés:** escenas `lab-captura-falla` y las capturas 62–71 en `recorrido.mjs`; escenas
  `captura` y `captura-vista` y capturas 41–44 en `recorrido-alumno.mjs`; caso `aviso-captura` en
  `pruebas-alumno.mjs`.
- **Falta de tu lado:** la prueba en VM (§17) y los casos manuales.

## Etapa 10.4: control remoto (D-019) — notas

- **Diseño aprobado** (2026-11-05), con dos cambios del usuario sobre la propuesta: **sin
  consentimiento** del alumno (el profesor tiene autoridad sobre los equipos) y **sin botón para
  que el alumno corte** el control. Se mantuvieron la pausa cuando el alumno usa su equipo, el
  video por la vista en grande (no WebRTC) y que cerrar la vista no suelte.
- **Protocolo** (`docs/protocol.md` §5.16, permisos del SO en §5.16.1): `remote.start`,
  `remote.input`, `remote.stop` y `remote.ack`; capacidad `remote`. Las imágenes son
  `screen.thumb.frame` con el `ref_id` del `remote.start` (hasta 10 por segundo). `remote.input`
  lleva hasta 32 eventos (`move`, `button`, `wheel`, `key` con `KeyboardEvent.code` de una lista
  cerrada), como máximo 60 por segundo.
- **`internal/platform`:** `InputInjector` (`Inject`, `LocalInput`) y la tabla de scancodes. En
  Windows, `SendInput` con `MOUSEEVENTF_ABSOLUTE` y `KEYEVENTF_SCANCODE` (Pausa, Bloq Num e Impr
  Pant por tecla virtual), marcado con `dwExtraInfo`; la entrada del alumno se detecta con
  `GetLastInputInfo` y `GetCursorPos`, sin hooks. Fallback «no soportado» en Linux y macOS. Las
  pruebas comprueban el tamaño de `INPUT` y la traducción de eventos; **nunca llaman a
  `SendInput`**. Una prueba del atajo global ya declaraba `procSendInput`: ahora usa la del paquete.
- **`internal/agentsvc`:** llamadas `input.inject` e `input.local`, *proxy* `Input` y
  `Ready.Input`.
- **`internal/screen`:** `Streamer.StartControl`, `RetuneControl` y `StopControl`: el control toma
  prestada la captura con su propio ritmo y aviso; las miniaturas que pida la consola mientras
  tanto se retoman al terminar.
- **`internal/remote`** (nuevo, sin APIs del SO): `Agent` (aviso antes de aceptar entrada, un
  control a la vez, `seq`, ritmo, pausa y reanudación, inactividad de 5 min, 30 s de rechazos →
  `error`, ayudante perdido → `session`, soltar todo al terminar) y `Desk` (uno a la vez, lotes,
  movimientos juntados, 60 por segundo sin perder teclas, video según la vista).
- **Alumno:** declara `remote` con captura e inyección (modo ventana; en servicio, si el ayudante
  dice `input`). `Status.control` (`active`, `paused`); banda `#banda-control` después de la de
  energía, título, insignia «Controlado» en compacto, `raise` al empezar y aviso de fin en la
  esquina sin quitar el foco. El ayudante recuerda lo presionado y lo suelta si el servicio se va.
  Sin bindings nuevos.
- **Consola:** «Tomar control» en la ficha y en la pantalla grande de Miniaturas (agregado a pedido:
  ahí es donde el profesor ve si el alumno está en la actividad), solo reales, conectados y con
  `remote`; la vista es
  `#pantalla-grande` en modo control (mouse, rueda y teclado; Ctrl+Alt+Fin devuelve el teclado;
  Esc va al alumno); franja global «Controlando … · Ver · Soltar»; errores `alreadyControlling`,
  `remoteUnsupported` y `remoteGone`; «Control remoto» en «Qué puede hacer su equipo».
- **Documentación:** `docs/privacidad.md` reglas 36–43 (y las reglas 1, 2 y 4); D-019; manual
  (Tareas frecuentes › «Ayudar a un alumno tomando el control» y la ficha en Alumnos); diagrama
  nuevo 17 y los 09 y 13 actualizados, con sus SVG; `docs/eventos-ui.md`; casos C-44 a C-47,
  A-09 a A-11 y `docs/prueba-vms.md` §16; `CLAUDE.md` lista `internal/remote`.
- **Arnés:** escenas `lab-control` y `lab-control-pausa` en `dobles.js` y capturas 54–59 en
  `recorrido.mjs`; escenas `control` y `control-pausa` en `dobles-alumno.js`, capturas 36–40 en
  `recorrido-alumno.mjs` y los casos `banda-control`, `compacto-control` y el orden de las bandas
  en `pruebas-alumno.mjs`.
- **Falta de tu lado:** la prueba en VM (§16) y los casos manuales. La latencia real del JPEG
  (objetivo: menos de medio segundo) solo se puede medir ahí.
