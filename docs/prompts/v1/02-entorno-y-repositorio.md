# 02 · Entorno y repositorio

**Prerrequisitos:** Go, Wails v2 (`wails doctor` pasa), Node.js y Git instalados. Carpeta vacía con `CLAUDE.md` (archivo 01) en la raíz.
**Duración estimada:** 1 sesión.

## Prompt

~~~~markdown
Lee CLAUDE.md. Vamos a crear la base del repositorio de Aula en Go + Wails. No implementes
funcionalidad todavía: solo estructura que compile y arranque.

## Tareas
1. Verifica el entorno: `go version`, `wails doctor`, `node -v`, `git`. Dime si falta algo
   para compilar apps Wails en Windows antes de continuar.
2. Inicializa el módulo Go en la raíz (`go mod init <ruta/github/aula>`).
3. Crea la estructura de paquetes de CLAUDE.md:
   - `internal/protocol`, `internal/discovery`, `internal/transport`, `internal/session`,
     `internal/security`, `internal/platform`, `internal/codedist` — cada uno con un archivo
     mínimo y una prueba trivial que pase (`go test ./...`).
   - `apps/teacher` y `apps/student`: cada una un proyecto Wails v2 con su `main.go`, su
     `wails.json` y un frontend mínimo (HTML/CSS/JS sin framework, o Svelte/Vue si lo prefieres;
     elige y justifícalo en una línea).
   - Decide cómo conviven dos apps Wails en un mismo módulo Go. Evalúa dos opciones y recomienda
     una: (a) dos directorios de app independientes bajo `apps/` compartiendo `internal/`;
     (b) un solo binario con build tags `//go:build teacher|student`. Explica la elección en
     docs/decisiones.md.
4. La app profesor abre una ventana con el texto "Aula — Consola"; la app estudiante, "Aula — Cliente".
5. Crea `docs/progreso.md` con una tabla de las etapas 02 a 11 y su estado, y `docs/decisiones.md`.
6. Crea `.gitignore` (Go, Node, build de Wails, `*.key`/`*.pem`), `.golangci.yml` razonable
   y `README.md` en español con cómo compilar y correr cada app (`wails dev`, `wails build`).
7. Inicializa Git con ramas `main` y `develop` (solo esas dos; sin ramas de feature). Conecta el
   remoto `origin` a `https://github.com/bayalad7/aula.git` y sube ambas ramas
   (`git push -u origin main` y `git push -u origin develop`). Deja `develop` como rama por
   defecto para el trabajo diario. Si el remoto ya tiene contenido (por ejemplo un README inicial
   de GitHub), reconcílialo antes de empujar y dime cómo lo resolviste; no fuerces el push.
8. Crea `.github/workflows/ci.yml` siguiendo las convenciones de CI de CLAUDE.md:
   - Dispara en `push` a `main`/`develop` y en `pull_request`. `permissions: contents: read`.
   - Un job con matriz `ubuntu-latest`, `windows-latest`, `macos-latest` (`fail-fast: false`).
   - Acciones ancladas: `actions/checkout@v7`, `actions/setup-go@v7` con `go-version: '1.26.x'`
     y `check-latest: true`.
   - Pasos: `gofmt -l` (debe salir vacío), `go vet ./...`, `staticcheck` vía
     `go run honnef.co/go/tools/cmd/staticcheck@<versión fija> ./...`, y `go test -race ./...`
     con `CGO_ENABLED=1`.
   - Si el frontend de Wails tiene pruebas o chequeo de tipos, agrega un job de Node (`setup-node@v7`).
   - El build del bundle Wails y el empaquetado van en un workflow de release aparte (se crea en
     etapas posteriores, al empujar tags `v*`); no lo incluyas aquí todavía.

## Restricciones
- No agregues dependencias que no se usen aún, más allá de las ya listadas en CLAUDE.md cuando toque.
- No escribas lógica de red ni de SO.

## Criterios de aceptación
- `go build ./...` y `go test ./...` pasan.
- `wails dev` en `apps/teacher` y en `apps/student` abre cada ventana con su título.
- El árbol de paquetes coincide con CLAUDE.md.

Al terminar, muéstrame el árbol del repositorio, los comandos que ejecutaste y actualiza docs/progreso.md.
~~~~

## Verificación manual
- [ ] Ambas ventanas (profesor y estudiante) abren en tu laptop.
- [ ] El primer commit está en `develop` y el workflow corre en GitHub.
- [ ] En `https://github.com/bayalad7/aula` se ven las ramas `main` y `develop` con el código subido.
