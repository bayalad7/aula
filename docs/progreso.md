# Progreso

Última actualización: 2026-10-28 (etapa 06).

| Etapa | Descripción | Estado |
|---|---|---|
| 02 | Entorno y repositorio: monorepo Go + dos apps Wails, Git, CI | ✅ Completada |
| 03 | Especificación del protocolo (`docs/protocol.md`, paquete `protocol`) | ✅ Completada |
| 04 | Descubrimiento mDNS con canales + servidor/cliente WebSocket | ✅ Completada (falta prueba en VMs) |
| 05 | Seguridad: TLS y autenticación Ed25519 | ✅ Completada |
| 06 | App estudiante: overlay con Prism (D-008) y botón Copiar | ✅ Completada (falta prueba en VM) |
| 07 | App profesor: lista, hotkey de captura, emisión | ⏳ Pendiente |
| 08 | Hito 1: distribución de código de punta a punta + instaladores | ⏳ Pendiente |
| 09 | Monitoreo: miniaturas y pantalla en blanco (con aviso visible) | ⏳ Pendiente |
| 10 | Plantilla reutilizable para nuevas funciones | ⏳ Pendiente |
| 11 | Revisión de seguridad y privacidad | ⏳ Pendiente |

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
