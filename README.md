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

## Instalar (Windows, modelo tipo LanSchool)

Descarga de *Releases* `Aula-Consola-Setup-X.Y.Z.exe` (profesores) y `Aula-Alumno-Setup-X.Y.Z.exe`
(alumnos).

1. **Primera consola:** el instalador pide el **canal predeterminado** de la sala (p. ej. `1`) y
   crea la **clave del centro**, una sola para todo el centro de cómputo. Deja `aula-centro.pub`
   (para los alumnos) y `aula-centro.key` (secreta, para importar en otras consolas).
2. **Alumnos:** pon `aula-centro.pub` junto al instalador. Cada computadora se instala con el
   canal fijo de su sala, que solo un administrador cambia. Para muchos equipos:
   `Aula-Alumno-Setup-X.Y.Z.exe /VERYSILENT /SUPPRESSMSGBOXES /NORESTART /CANAL=1 /CLAVECENTRO=ruta\aula-centro.pub`.

- Guía de uso completa: `docs/uso-hito1.md`.
- Configuración y firewall: `docs/configuracion.md`.
- Cómo se publica una versión (tags `vX.Y.Z`, firma de `checksums.txt`): `docs/desarrollo/publicar.md`.

## Usar la consola del profesor

1. Abre `aula-consola`: arranca directo en su canal predeterminado. En la barra superior puedes
   cambiar de canal (**Canal … Cambiar**) y el nombre que verán los alumnos.
2. Las tarjetas de la izquierda muestran a los alumnos conectados en vivo.
3. Escribe o pega código en el editor y pulsa **Emitir a todos** (reemplaza) o **Agregar
   bloque**. Abajo verás "confirmados N/N ✓" a medida que cada alumno lo muestra. El historial
   de la derecha permite **Reemitir** un ejercicio anterior.
4. **Captura rápida:** mientras presentas, copia código (`Ctrl+C`) y pulsa
   **Ctrl+Alt+Shift+C** (o el botón **Capturar portapapeles**). El código queda en el editor
   para que revises el lenguaje y lo emitas; nunca se envía solo.

## Configurar el cliente del alumno

El cliente necesita su canal (número del 1 al 16000) y la clave pública del centro. Puedes pasarlos como argumentos
(`aula-cliente -canal 1 -clave aula-ed25519:… [-directo IP:47830]`) o guardarlos en
`%AppData%\Aula\alumno\config.json`:

```json
{ "channel": "1", "console_public_key": "aula-ed25519:..." }
```

Con `wails dev`: `wails dev -appargs "-canal 1 -clave aula-ed25519:..."`. Sin
configuración, la ventana muestra "Aula — sin configurar" y cómo arreglarlo; si no conecta,
muestra el motivo, y `aula-cliente diagnostico` da el detalle. El contrato entre
el backend y la interfaz está en `docs/eventos-ui.md`.

## Clave del centro

Las consolas firman con la **clave del centro** (Ed25519) y los alumnos solo obedecen a esa
clave (ver `docs/protocol.md` §10). Como administrador:

```sh
aula-consola centro crear                       # una vez por centro
aula-consola centro publica -o aula-centro.pub  # para instalar los alumnos
aula-consola centro exportar aula-centro.key    # secreta, para otras consolas
aula-consola centro importar aula-centro.key
aula-consola pubkey                             # qué clave usa esta consola
```

La privada queda en `C:\ProgramData\Aula\centro\` (lectura para los usuarios del equipo, cambios
solo para administradores). Sin clave del centro, la consola usa una clave personal
(`aula-consola keygen`, en `%AppData%\Aula\consola\`) y avisa que los alumnos del centro no la
reconocerán.

## Ejemplo: descubrimiento, TLS y registro en vivo

```sh
go run ./examples/dos_alumnos                     # profesor + 2 alumnos simulados en un equipo
go run ./examples/dos_alumnos -rol profesor       # en el equipo del profesor (imprime la clave)
go run ./examples/dos_alumnos -rol alumno -clave aula-ed25519:...                            # mDNS
go run ./examples/dos_alumnos -rol alumno -clave aula-ed25519:... -directo 192.168.1.10:47830 # sin mDNS
```

Opciones: `-canal 1`, `-puerto 47830`, `-nombre "Consola de ejemplo"`, `-v` (logs).
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
