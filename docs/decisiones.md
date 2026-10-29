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
