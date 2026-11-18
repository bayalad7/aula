# Auditoría 2026-11-17 (prompt 11)

Primera revisión completa de Aula, en `develop` en `9917345` (10.28 subida), antes de publicar
`v0.1.0`. No se cambió código: este informe lista lo encontrado y propone el arreglo. **Se corrige
solo con el visto bueno del usuario**, en el orden de la lista priorizada del final.

Clasificación:
- **Crítico:** rompe una garantía de seguridad o de transparencia (alguien ordena sin la clave del
  centro, o se observa al alumno sin aviso).
- **Importante:** incumple una regla escrita de `docs/privacidad.md` o del protocolo, o abre un
  abuso realista en el aula.
- **Menor:** documentación desfasada, fragilidad o deuda sin efecto para el alumno hoy.

## Resumen

- **Ningún hallazgo crítico.** La autenticación, la regla de oro y los avisos de transparencia
  aguantan la revisión.
- **2 importantes:** registros que guardan rutas y nombres de archivo del alumno (reglas 48, 59
  y 65), y suplantación de un equipo por un agente no autenticado, que desde 10.10 y 10.20 afecta
  lo recogido y las calificaciones.
- **8 menores:** cuatro de documentación, dos de robustez, dos de pruebas y una del entorno local.

## Herramientas

| Comprobación | Resultado |
|---|---|
| `gofmt -l .` | Sin salida |
| `go vet ./...` | Sin hallazgos |
| `staticcheck` v0.8.2 | Sin hallazgos |
| `GOOS=linux` y `GOOS=darwin go vet ./internal/...` | Compilan: todo `platform` tiene fallback |
| `go test -race ./...` (local, Windows 10) | **0 data races.** 5 pruebas de `internal/screen` fallan por tiempo con toda la suite en paralelo y pasan solas, con y sin `-race` (M-1) |
| `govulncheck` v1.9.0 (local, Go 1.27.2) | 2 de la biblioteca estándar, del Go local, no del código (M-8) |
| CI de `9917345` (Go 1.26.x, tres plataformas) | Go, `-race`, `govulncheck` y servicio de Windows: verdes. Arnés web: ver «UX en verde» |

### Cobertura de los paquetes críticos

| Paquete | Cobertura | Sin probar |
|---|---|---|
| `protocol` | 92,6 % | `parts.Received`, `ValidMAC`, `PowerAction.Delay` |
| `session` | 82,8 % | `Agent.Probe` (lo prueba `connectcheck`), ramas de `waitConsole` y `upsert` |
| `security` | 76,6 % | Ramas de error: `LoadIdentity` con PEM dañado o de otro tipo, fallas de `writeSecretFile` y `writeMachineSecret` (M-7) |
| `transport` | 86,4 % | — |

## 1. Seguridad

### Lo verificado

- **Nada se procesa antes de `auth.ok`.** El agente (`internal/session/agent.go:365-403`) solo
  acepta un `auth.proof` con su `ref_id`; cualquier otra cosa → `unauthorized` y cierre. La firma
  cubre nonce, canal y la huella del certificado **que vio el agente**: un `cert_fingerprint` falso
  por mDNS o un intermediario no pueden reenviar la prueba. La consola (`console.go:190-239`)
  descarta todo lo que llega antes de `hello`, y lo que inicia un agente exige su capacidad
  declarada (`console.go:259`).
- **Canal local servicio ↔ ayudante:** nombre aleatorio por lanzamiento, ACL protegida (SYSTEM y el
  SID de la sesión), se escucha antes de lanzar y se rechaza todo PID que no sea el del ayudante
  (`agentsvc/supervisor.go:145-213`, `platform/session_windows.go:131`).
- **Clave del centro:** el agente solo tiene la pública; ningún `.pem` ni `.key` versionado; el
  binario del alumno no contiene PEM. ACL de máquina protegida y sin escritura para Usuarios,
  probada (`TestRestrictMachineSecret`). La lectura por Usuarios del equipo del profesor es un
  riesgo aceptado y documentado (`docs/protocol.md` §10.1).
- **Entradas de red acotadas:** `SetReadLimit(1 MiB)` (`transport/conn.go:120`), cada payload pasa
  por `Validate()` al decodificar (`protocol/envelope.go:176`, `:233`), TLS 1.3 mínimo con fijación
  por `VerifyConnection` (`security/tls.go:150-154`), ritmos por agente (§7).
- **Órdenes que ejecutan algo en el alumno:** `open.url` solo http/https con host, sin usuario ni
  espacios ni `\` (`protocol/open.go:42`), y `ShellExecute` directo, sin `cmd`; `open.app` sin
  argumentos, con raíces fijas y lista de herramientas negadas (`openapp/rules.go`).
- **Lo que manda el alumno no ejecuta nada en la consola:** sin `innerHTML` salvo la salida de
  Prism (que escapa); CSP en las dos páginas. Las rutas que manda al recoger se comprueban dos veces
  (`collect/store.go:91`); los nombres de carpeta del alumno se sanean (`snapshot/files.go:84-99`);
  el CSV de calificaciones neutraliza fórmulas (`poll/quiz.go:394`).
- **Concurrencia:** sin data races en la suite completa con `-race`, local y en el CI.

### I-1 · Importante · Los registros guardan rutas y nombres de archivo del alumno

**Dónde.** Los errores de archivo se registran enteros (`"err", err`), y llevan la ruta:
- `internal/snapshot/console.go:323`: `errors.Unwrap(err)` quita el prefijo pero deja el
  `*os.PathError` de `snapshot/files.go:56`, `:70` y `:76`. Un fallo al guardar registra
  `mkdir C:\Users\<profesor>\Documents\Aula\capturas\LAB3-PC07 (ana)\…: Acceso denegado`.
  **La regla 48 lo prohíbe expresamente** («nunca la ruta del archivo, que nombra al alumno»).
- `internal/filepush/receiver.go:256` y `apps/student/helper.go:209`: el error viene de
  `platform/workfolder_windows.go:50` y `:55` (`%q` con la ruta relativa, p. ej.
  `"Ejercicio 4/datos.csv"`) o de `os.WriteFile` (ruta completa con el usuario del alumno). La regla
  59 dice «nunca los nombres de los archivos ni la carpeta».
- `internal/collect/responder.go:193` y `:299`, `apps/student/helper.go:399`: los errores de
  `platform/workfolder_read_windows.go:87` llevan el nombre de una carpeta del alumno. Del lado de
  la consola, `collect/collector.go:421`, `:481` y `:594` registran errores de `collect/store.go:92`
  (`%q` con la ruta que mandó el alumno) y `*os.PathError` con `…\<equipo> (<usuario>)\…`. Regla 65.

**Por qué importa.** Contradice tres reglas escritas de privacidad. Además, el registro del
servicio (`C:\ProgramData\Aula\alumno\logs\aula-agente.log`) es legible por **todos** los usuarios
del equipo a propósito (`empaquetado/windows/aula-alumno.iss:71-87`), así que en un laboratorio
compartido el alumno siguiente puede leer los nombres de archivo del anterior. Solo pasa cuando
falla una escritura o una lectura, pero es justo cuando alguien abre el registro.

**Arreglo propuesto.**
1. Una función en `internal/platform`, `ErrorKind(err) string`, que devuelva solo la clase: el
   centinela (`ErrNoSpace`, `ErrDenied`, `ErrLinkInPath`…), el `syscall.Errno` en número o
   `"other"`. Nunca `err.Error()`.
2. Los `slog` de `snapshot`, `filepush`, `collect` y de `helper.go` (escritura, lectura, apertura de
   carpeta) registran `"err_kind", platform.ErrorKind(err)` en lugar de `"err", err`. El `reason`
   que ya registran se queda.
3. Una prueba por paquete que fuerza un fallo de escritura con una ruta conocida, captura el
   `slog` y comprueba que la ruta no aparece. Lo mismo para la consola (`snapshot`, `collect`).
4. `docs/privacidad.md` regla 5: agregar «ni las rutas de archivos», que hoy solo dicen las reglas
   48, 59 y 65.

### I-2 · Importante · Un agente no autenticado puede hacerse pasar por otro equipo, y eso ya afecta las calificaciones y lo recogido

**Dónde.** La consola no autentica agentes (riesgo aceptado en v0, `docs/protocol.md` §10.3). El
`hello` solo limita la longitud del equipo y del usuario (`protocol/payloads.go:117-123`), y
`session/registry.go:92-119` acepta dos sesiones **conectadas a la vez** con el mismo equipo y
usuario, sin marcarlas. Desde 10.10 y 10.20, esos dos campos deciden:
- la carpeta donde se guarda lo recogido (`collect/store.go:35`), y «recoger otra vez reemplaza la
  copia del profesor de ese alumno» (regla 64);
- la fila del CSV de calificaciones (`poll/quiz.go:378`).

**Por qué importa.** Con un cliente modificado y la clave **pública** del centro (está en la
configuración de cualquier equipo), un alumno puede entrar al canal como `LAB3-PC07 (ana)`,
responder el cuestionario por ella o entregar archivos que pisen su recolección. Cuando se escribió
§10.3, lo peor era ver el código o la transmisión; hoy son calificaciones. No es crítico: no
permite dar órdenes a ningún equipo.

**Arreglo propuesto** (sin cambiar el modelo de seguridad):
1. La consola marca **«Duplicado»** a dos sesiones conectadas que dicen ser el mismo equipo (o la
   misma MAC), con insignia en la Fila del alumno y en la Ficha, y una línea de registro.
2. Mientras haya un duplicado: Participación avisa en el resultado del cuestionario, el CSV agrega
   una columna «Aviso» y `aula-recoleccion.json` guarda la IP de origen y la marca. No se descarta
   nada solo: decide el profesor.
3. `docs/protocol.md` §10.3: actualizar la fila «Agente no autorizado» con estas consecuencias y la
   mitigación.
4. **Autenticar al agente** (una clave por equipo, que la consola conozca) cambia el modelo de
   seguridad: va como propuesta a `docs/ux/PROPUESTAS.md`, no a esta corrección.

## 2. Transparencia y privacidad

### Lo verificado

- **Nada captura sin aviso.** La miniatura muestra el distintivo antes del primer cuadro y no
  captura si falla (`screen/streamer.go:231`); la captura puntual y el control remoto pasan por
  `Announce`, que falla sin distintivo (`streamer.go:120-150`); proyectar muestra el distintivo
  (`broadcast/presenter.go:133`) y excluye las ventanas de Aula antes de capturar
  (`publisher.go:108`). El ayudante que se cae detiene captura, transmisión y proyección
  (`apps/student/service.go:455-463`).
- **Cada `raise` es una acción del profesor sobre el equipo:** restricciones (`app.go:684`), pausa
  (`pause.go:61`), energía (`power.go:45`), proyectar (`present.go:39`), control remoto
  (`remote.go:43`) y transmisión (`window.go:178`). Lo único más es `OnSecondInstanceLaunch`
  (`main.go:183`), cuando el propio alumno vuelve a abrir Aula. Código, avisos, encuestas,
  archivos, recolecciones, captura, temporizador y fin de clase usan `notify`, y lo prueban
  `app_test.go`, `classend_test.go` y los demás.
- **El distintivo dice lo mismo que las reglas**, palabra por palabra: 6
  (`screen/streamer.go:15`), 9 (`screen/freeze.go:15`), 38 (`remote/agent.go:20`), 44
  (`snapshot/agent.go:19`) y 76 (`broadcast/presenter.go:15`). Los títulos de la ventana
  (`textos.es-MX.js:16-25`) coinciden con las reglas 1, 6, 9, 12, 37 y 76.
- **Orden de las bandas** como la regla 1, probado en `scripts/ux/pruebas-alumno.mjs:841`; ninguna
  se cierra (pruebas de bandas y `TestNoBindingTurnsOffTransparency`).
- **Registros** sin código, textos, preguntas, respuestas ni claves: se registran IDs, longitudes y
  conteos (`participation.go:174`, `poll.go:161`…). La excepción son las rutas (I-1) y el nombre de
  la app cerrada (M-3).

### M-2 · Menor · `docs/privacidad.md` no dice lo que hace el código en tres puntos

- **Regla 17:** «sin ayudante, el agente se conecta sin capacidades, salvo `power`». El servicio
  declara también `health` y `health.selftest` (`apps/student/service.go:336`); el comentario de
  `service.go:325` repite lo viejo. La regla 92 sí lo dice.
- **Regla 19:** «falta `remote.*`», pero el control remoto ya tiene sus reglas 36–43 y corre en el
  ayudante, no como SYSTEM. Falta en cambio la salud: el servicio, como SYSTEM, lee el equipo y en
  el autodiagnóstico abre conexiones a los destinos de Ajustes y escribe un archivo temporal
  (regla 93).
- **Reglas 2 y 4:** enumeran las excepciones a «no traer la ventana» como «reglas 12, 20, 26 y
  36» y omiten la pausa (regla 9) y proyectar (regla 75), que sí usan `raise` (y `CLAUDE.md` los
  lista).

**Arreglo:** corregir las tres reglas y el comentario. Sin cambio de código.

### M-3 · Menor · El registro del servicio guarda qué programa bloqueado abrió el alumno

`internal/policy/apps.go:153` y `:156` registran `"app", proc.Name` cada vez que Aula cierra una app
bloqueada. Es actividad del alumno, en un registro que leen todos los usuarios del equipo (I-1), y
ninguna regla lo contempla (la 23 solo habla de dominios). **Arreglo:** registrar la política y
cuántas se cerraron, sin el nombre, como ya hace el filtro web, o declararlo en la regla 22.

## 3. Arquitectura

- **Regla de oro, sin violaciones.** Ninguno de los 25 paquetes de la lista de `CLAUDE.md`
  importa `syscall`, `unsafe`, `os/exec`, `golang.org/x/sys`, captura, portapapeles, `go-winio` ni
  Wails, y ninguno tiene archivos `*_windows.go`, `*_linux.go` o `*_darwin.go`. Del paquete `os`
  usan solo archivos portables y `os.Hostname` (`discovery/announce.go:72`); las variables de
  entorno están solo en pruebas.
- **Toda función del SO tiene fallback.** Los 22 archivos `*_windows.go` de `platform` tienen su
  `!windows`: 18 con pareja directa y 4 cubiertos por otros (`service_other.go`,
  `appwindow_other.go`, `workfolder_other.go`). Compila y pasa `vet` en Linux y macOS.

## 4. Calidad

- **`panic`:** uno solo, en `platform/h264_windows.go:161`, al interpretar GUID constantes al
  cargar el paquete. Equivale al arranque y lo ejercita cualquier prueba del paquete. Correcto.
- **Errores descartados:** 64 `_ =`, casi todos de mejor esfuerzo (`Close`, `Send`, soltar
  recursos). Uno con efecto para el alumno, M-4.
- **Goroutines:** 63 lanzamientos fuera de pruebas. Las de larga vida terminan con su `ctx` o con
  el `Close`/`stop` de su dueño (ventanas nativas, resolvedor DNS, conexión). No hay prueba que lo
  garantice (M-6).

### M-4 · Menor · Si quitar el filtro web falla, el alumno se queda sin red y sin explicación

`internal/policy/manager.go:256-266`: `clearWeb` pone `m.web = nil` (la banda de restricciones
desaparece) **antes** de `teardown()`. Si el desmontaje falla (DNS o NRPT), el resolvedor ya está
cerrado y el DNS sigue apuntando a él: el alumno queda sin navegación hasta que el servicio vuelva a
arrancar (el diario lo reintenta solo entonces). `apps/student/service.go:462` descarta además ese
error. Es un caso raro, pero contradice «nadie queda bloqueado» (regla 24).
**Arreglo:** si `teardown` falla, mantener vivo el resolvedor en modo de paso (sin filtrar),
reintentar cada 30 s y avisar a la consola; registrar el error en `service.go:462`.

### M-1 · Menor · Pruebas de `internal/screen` con plazo fijo de 3 s

`internal/screen/screen_test.go:98` (`sink.next`) espera 3 s. Con la suite completa y `-race` en
este equipo fallan 5 pruebas (`TestStartShowsIndicatorBeforeCapturingAndStopEndsIt`,
`TestStopOnDisconnectAndIdempotent`, `TestNewStartReplacesParameters`,
`TestAnnounceDuringStreamKeepsIndicator`, `TestStreamStartedDuringAnnounceKeepsIndicator`); solas
pasan con y sin `-race`, y el CI pasa. Ya estaba anotado en 10.28. **Arreglo:** plazo de 10 s, o el
de `t.Deadline()`; no cambia lo que se prueba.

### M-6 · Menor · Ninguna prueba detecta goroutines que no terminan

No se usa `goleak` ni se compara `runtime.NumGoroutine()`. **Arreglo:** una comprobación sin
dependencias nuevas en `transport`, `session` y `screen` (contar antes y después, con espera
corta). `goleak` sería una dependencia nueva: decisión aparte.

### M-7 · Menor · Ramas de error de `security` sin prueba

`LoadIdentity` con un PEM dañado o de otro tipo y las fallas de `writeSecretFile` y
`writeMachineSecret` (`security/files.go:33-80`, `identity.go:131`) no se ejercitan. Son las que
deciden qué ve el administrador si la clave está mal. **Arreglo:** pruebas de tabla con PEM vacío,
truncado, RSA y Ed25519 público.

## 5. Definición de terminado (todas las funciones; es la primera auditoría)

| Punto | Estado |
|---|---|
| Manual | Cada función tiene su página o sección en `docs/administracion/` (Laboratorio, Salón, Cuestionarios, Encender, Terminar la clase, Pausar, Proyectar, Abrir, Recolecciones, Temporizador, Mensaje privado, Captura, Control remoto, Encuestas, Etiqueta). `go test ./docs`: verde |
| Diagramas | 23, con SVG validados por `go test ./docs`. Todos los flujos del protocolo aparecen en alguno, salvo `timer.*` y `computer.label`, que su prompt dejaba opcionales |
| Arnés | Escenas en `recorrido.mjs` de todas las funciones; en el alumno, las que se le ven (la salud y la etiqueta en «Qué puede hacer Aula», `recorrido-alumno.mjs:157`) |
| Casos manuales | C-01 a C-156 y A-01 a A-65, **salvo C-20** (M-5) |
| Errores con código y texto | Lo exigen las pruebas de las dos apps (`errores.go`, `errorHelp`, `agentErrorHelp`): verdes |

### M-5 · Menor · El caso C-20 se borró sin nota

`docs/desarrollo/probar-consola-local.md` salta de C-19 a C-21. El commit `cb90f2e` (encuestas)
borró «Con 1 VM de alumno y los simulados abiertos: el real cuenta en conectados y los simulados
aparte; Energía y Restricciones solo se ofrecen para el real», que es la salvaguarda de la regla
34 vista en la ventana real. **Arreglo:** restaurarlo, sumando Pausar, Abrir y Proyectar, que
llegaron después.

## 6. UX en verde y recursos del alumno

- **Arneses:** el CI de `9917345` corre `recorrido.mjs`, `teclado.mjs`, `usuario-nuevo.mjs`,
  `recorrido-alumno.mjs`, `pruebas-alumno.mjs` y la autoprueba del anotador.
  **Todos en verde** (corrida 44196429107): sin errores de JS ni violaciones serias de axe, y las
  pruebas de textos de las dos apps pasan en `go test`.
- **Recursos del alumno** (`go run ./tools/medir-alumno`, frente a
  `docs/ux/historial/alumno-resultados.md`: 0,0–0,1 % de CPU, 187–191 MB):


  | Estado | CPU (% de un núcleo) | RAM media (MB) | Procesos |
  |---|---|---|---|
  | Reposo (1 min) | 0,2 | 197 | 8 |
  | Recibiendo (un código cada 2 s) | 0,0 | 195 | 8 |

  La CPU sigue igual. La RAM sube unos 7 MB (+4 %) y hay un proceso de WebView2 más, tras diez
  funciones nuevas del alumno desde la medición del prompt 19; no es un hallazgo, pero conviene
  medir otra vez después de 10.13. El primer plano no cambió a Aula al llegar código.
  `-ventana`: las seis comprobaciones en verde (minimizada no cambia el primer plano, `WM_CLOSE`
  deja el agente vivo y la ventana oculta, oculta no reaparece). Falta a ojo el icono del área de
  notificación y el modo compacto.

## 7. Entorno local

### M-8 · Menor · El Go de este equipo (1.27.2) tiene dos vulnerabilidades de la biblioteca estándar

`govulncheck` local marca GO-2026-6859 (`net/http`, corregida en 1.27.4) y GO-2026-6809
(`crypto/x509`, en 1.27.3). El CI compila con Go 1.26.x más reciente y sale limpio, y los binarios
publicados salen del CI. **Arreglo:** actualizar el Go local a 1.27.4. Aparte, decidir cuándo el
CI pasa a 1.27.x (1.26 recibe parches hasta que salga 1.28).

## Lista priorizada

1. **I-1** Rutas en los registros: `platform.ErrorKind`, cambiar los `slog` de `snapshot`,
   `filepush`, `collect` y `helper.go`, pruebas de registro y regla 5.
2. **I-2** Duplicados: marca «Duplicado» en la consola, aviso en el CSV y en
   `aula-recoleccion.json`, §10.3 al día y la propuesta de autenticar agentes en `PROPUESTAS.md`.
3. **M-3** Sin el nombre de la app cerrada en el registro (va con I-1).
4. **M-4** Desmontaje del filtro web que falla: resolvedor de paso, reintento y aviso.
5. **M-2** `docs/privacidad.md`, reglas 2, 4, 17 y 19, y el comentario de `service.go:325`.
6. **M-5** Restaurar C-20.
7. **M-1** Plazo de las pruebas de `screen`.
8. **M-7** Pruebas de error de `security`.
9. **M-6** Comprobación de goroutines en `transport`, `session` y `screen`.
10. **M-8** Go local a 1.27.4 (de tu lado) y decidir el Go del CI.

Antes de desplegar en todo el laboratorio, conviene que otra persona, o una sesión que no
escribió el código, repase las secciones 1 y 2 (nota del prompt 11).

## Correcciones (2026-11-17, con el visto bueno del usuario)

| Hallazgo | Resultado |
|---|---|
| **I-1** | Hecho. `platform.ErrorKind(err, known...)` da la clase del error (el centinela, el número del SO u «other»), nunca su texto. Los `slog` de `snapshot`, `filepush`, `collect`, `helper.go`, `files.go`, `collect.go` y la plantilla de la consola registran `err_kind`. Pruebas: `TestErrorKindNeverCarriesThePath`, `TestFailedSaveKeepsThePathOutOfTheLog`, `TestReceiverLogsNoNames` (ahora con un fallo de escritura) y `TestResponderLogsNoPaths`. Regla 5 al día |
| **I-2** | Hecho, con la ubicación y el efecto que eligió el usuario (insignia en la Fila y la Ficha; solo marcar). `Session.DuplicateOf` (mismo equipo y usuario sin mayúsculas, o una MAC en común; solo sesiones conectadas del canal; los simulados van en otro registro), una línea de registro, la insignia «Duplicado» que abre la ficha, la ficha con «Aviso» y «Dirección», un aviso flotante por equipo, la columna «Aviso» del CSV y `address`/`duplicate` en `aula-recoleccion.json`. §10.3 al día, propuesta P-06 (autenticar a los agentes), manual («Dos conexiones dicen ser el mismo equipo»), capturas 230–232 del arnés y C-157 y C-158. Limitación: en Miniaturas las dos conexiones comparten tarjeta (la clave de la interfaz es el nombre del equipo) y la ficha abre la primera, que ya muestra las dos direcciones |
| **M-1** | Hecho: plazo de 10 s en `screen_test.go`. Lo mismo para `TestPusherDeliversToEverySession` (300 ms → 2 s), que falló por carga en la corrida completa |
| **M-2** | Hecho: reglas 2, 4, 17 y 19 y el comentario de `service.go` |
| **M-3** | Hecho: el registro lleva la política, no la app; regla 22 al día |
| **M-4** | Hecho: si `DNS.Clear` falla, el resolvedor sigue vivo sin filtro (el alumno conserva la red), el desmontaje se reintenta cada 30 s y un filtro nuevo lo retoma conservando la configuración de los navegadores del alumno. `service.go` registra el error. Pruebas `TestManagerFailedRemovalKeepsTheNetworkAndRetries` y `TestManagerNewFilterAfterAFailedRemoval`; regla 24 al día |
| **M-5** | Hecho: C-20 restaurado, con Pausar, Abrir, Transmitir, Proyectar y Tomar control |
| **M-6** | Hecho: `TestMain` en `transport`, `session` y `screen` falla si quedan goroutines (comprobado con una sonda que deja una viva) |
| **M-7** | Hecho: `TestLoadIdentityRefusesBadFiles` y `TestWriteSecretFileFailures`; `security` de 76,6 a 80,8 % |
| **M-8** | Hecho: el usuario instaló Go 1.27.4 (`govulncheck` sin hallazgos) y la 10.29 pasó el CI, el release y el mínimo de `go.mod` a Go 1.27 (`docs/prompts/10.29-go-1-27.md`) |
