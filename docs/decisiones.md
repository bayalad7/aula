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
