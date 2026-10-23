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
