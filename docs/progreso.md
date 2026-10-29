# Progreso

Última actualización: 2026-10-29 (transmisión de la pantalla del profesor con WebRTC).

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
| 09 | Monitoreo: miniaturas y pantalla en blanco (con aviso visible) | 🚧 En curso (miniaturas; pantalla en blanco documentada y pendiente) |
| 09.2 | Transmisión de la pantalla del profesor (WebRTC, Windows) | ✅ Implementada (falta prueba en VMs, `docs/prueba-vms.md` §12) |
| 09.3 | Agente del alumno como servicio de Windows (servicio + ayudante) | ✅ Implementado y probado en CI (falta prueba en VM, `docs/prueba-vms.md` §13) |
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
