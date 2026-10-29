# Aula

Gestión de aula multiplataforma para laboratorios de programación. El profesor distribuye
código que los alumnos reciben formateado y copian con un clic, y monitorea y controla los
equipos de forma transparente. Está hecho en Go + [Wails v2](https://wails.io).

- **Consola del profesor:** `apps/teacher`
- **Cliente del alumno:** `apps/student`
- **Código compartido:** `internal/*` (ver `CLAUDE.md` y `docs/decisiones.md`)

## Requisitos

- Go 1.26 o más reciente.
- Wails CLI v2: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
  (`wails doctor` debe terminar en *SUCCESS*).
- Node.js LTS. Wails lo pide, aunque el frontend actual no necesita `npm install`.
- Windows: WebView2 (incluido en Windows 10/11).
- Linux: `libgtk-3-dev` y `libwebkit2gtk-4.1-dev`; compila con `-tags webkit2_41`.
- macOS: Xcode Command Line Tools.
- Para `go test -race` en Windows: gcc de MinGW-w64 (p. ej. `winget install
  BrechtSanders.WinLibs.POSIX.UCRT`). Si tu carpeta de usuario tiene espacios, `ld` falla; ver
  "Entorno de desarrollo" en `docs/progreso.md` para la solución.

## Desarrollo

Cada app se corre desde su propio directorio:

```sh
cd apps/teacher   # o apps/student
wails dev         # abre la ventana con recarga en vivo
```

## Compilar

```sh
cd apps/teacher && wails build   # -> apps/teacher/build/bin/aula-consola(.exe)
cd apps/student && wails build   # -> apps/student/build/bin/aula-cliente(.exe)
```

En Linux: `wails build -tags webkit2_41`.

## Usar la consola del profesor

1. Abre `aula-consola`, elige el canal del aula (p. ej. `AULA_101`) y el nombre que verán los
   alumnos, y pulsa **Abrir clase**. La primera vez se crea la clave de la consola; con
   **Copiar clave** la llevas a la configuración de los alumnos.
2. Las tarjetas de la izquierda muestran a los alumnos conectados en vivo.
3. Escribe o pega código en el editor y pulsa **Emitir a todos** (reemplaza) o **Agregar
   bloque**. Abajo verás "confirmados N/N ✓" a medida que cada alumno lo muestra. El historial
   de la derecha permite **Reemitir** un ejercicio anterior.
4. **Captura rápida:** mientras presentas, copia código (`Ctrl+C`) y pulsa
   **Ctrl+Alt+Shift+C** (o el botón **Capturar portapapeles**). El código queda en el editor
   para que revises el lenguaje y lo emitas; nunca se envía solo.

## Configurar el cliente del alumno

El cliente necesita el canal y la clave pública de la consola. Puedes pasarlos como argumentos
(`aula-cliente -canal AULA_101 -clave aula-ed25519:… [-directo IP:47830]`) o guardarlos en
`%AppData%\Aula\alumno\config.json`:

```json
{ "channel": "AULA_101", "console_public_key": "aula-ed25519:..." }
```

Con `wails dev`: `wails dev -appargs "-canal AULA_101 -clave aula-ed25519:..."`. Sin
configuración, la ventana muestra "Aula — sin configurar" y cómo arreglarlo. El contrato entre
el backend y la interfaz está en `docs/eventos-ui.md`.

## Clave de la consola

Cada consola tiene una clave Ed25519 que la identifica ante los alumnos (ver
`docs/protocol.md` §10):

```sh
aula-consola keygen     # crea la clave (una vez) y muestra la clave pública aula-ed25519:…
aula-consola pubkey     # vuelve a mostrar la clave pública
```

La clave privada queda en `%AppData%\Aula\consola\` con acceso solo para tu usuario. La
pública es la que se configura en cada alumno. `keygen -force` crea otra clave, y entonces hay
que reconfigurar todos los alumnos.

## Ejemplo: descubrimiento, TLS y registro en vivo

```sh
go run ./examples/dos_alumnos                     # profesor + 2 alumnos simulados en un equipo
go run ./examples/dos_alumnos -rol profesor       # en el equipo del profesor (imprime la clave)
go run ./examples/dos_alumnos -rol alumno -clave aula-ed25519:...                            # mDNS
go run ./examples/dos_alumnos -rol alumno -clave aula-ed25519:... -directo 192.168.1.10:47830 # sin mDNS
```

Opciones: `-canal AULA_101`, `-puerto 47830`, `-nombre "Consola de ejemplo"`, `-v` (logs).
En Windows, permite el acceso en el firewall la primera vez. Pasos para VMs en
`docs/prueba-vms.md`.

## Verificaciones antes de cada commit

```sh
gofmt -l .                     # no debe imprimir nada
go vet ./...
go run honnef.co/go/tools/cmd/staticcheck@v0.8.2 ./...
go test -race ./...            # requiere cgo (gcc)
```

## Ramas y versiones

Solo hay dos ramas: `develop`, para el trabajo diario, y `main`, para releases estables
etiquetados `vX.Y.Z` (SemVer).

## Estado

Ver [`docs/progreso.md`](docs/progreso.md).
