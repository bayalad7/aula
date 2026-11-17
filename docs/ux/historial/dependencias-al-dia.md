# Dependencias al día (10.28): análisis, resultados y decisiones

Estado: **implementado, sin commit** (17/11/2026). Prompt `docs/prompts/10.28-dependencias-al-dia.md`,
con `/funcionalidad-mejora`. Es mantenimiento: no cambia pantallas, protocolo, textos ni el manual,
así que no hay capturas.

## Análisis: ¿Wails v3?

La pregunta fue si convenía pasar de Wails v2.17.0 a v3.1.0 y qué hacer con los paquetes atrasados.

- **Cuánto usa Aula a Wails:** 7 archivos Go lo importan (unas 30 funciones de `runtime.*`, más
  `SingleInstanceLock` y los ganchos de arranque y cierre); 121 métodos enlazados en la consola y
  19 en el alumno. Cada frontend entra por un solo punto (`window.go.main.App` y `window.runtime`
  en su `main.js`), y el arnés (`dobles.js`, `dobles-alumno.js`) imita esa forma.
- **Qué pide v3:** es un port, no un cambio de número: `application.New` con *services* en lugar
  de `Bind`, la ventana como objeto, bindings generados y un build con Taskfile en lugar de
  `wails.json`. Cambian el frontend, los dobles del arnés, las pruebas de bindings, el release y
  la decisión D-001.
- **Qué pasa si no se migra:** a corto plazo, nada. v2 sigue con mantenimiento (v2.17.1 salió el
  1/11/2026 con una corrección de seguridad), y en Windows el motor web es WebView2, que se
  actualiza con el sistema. Las apps solo cargan su contenido embebido.

**Decisión:** Wails se queda en v2. La migración a v3 será otra etapa, con su entrada en
`docs/decisiones.md`, empezando por una prueba acotada que porte solo la app del alumno.

## Antes (17/11/2026)

- `go list -m -u all`: 71 módulos con versión nueva. De ellos, 7 directos, todos menores o de
  parche.
- `govulncheck` v1.9.0 con Go 1.27.2 (el local):
  - 2 vulnerabilidades **llamadas**, ambas de la biblioteca estándar: GO-2026-6859 (`net/http`,
    caída del servidor HTTP/1 por una carrera en `MaxBytesReader`; corregida en 1.27.4 y
    1.26.11) y GO-2026-6809 (`crypto/x509`, desconfiar de certificados intermedios
    comprometidos; corregida en 1.27.3 y 1.26.10);
  - 17 en módulos que el código **no llama**: 7 de `x/net` (servidor y transporte HTTP/2,
    `html`), 1 de `x/text` (`precis`), 4 de `x/crypto` (`ssh`, `openpgp`) y 5 de `x/image`
    (VP8L, TIFF, WebP).

## Qué se hizo

- `go get -u ./...` y `go mod tidy`. Directas: Wails v2.17.0 → v2.17.1, `x/net` v0.57.0 →
  v0.63.0, `x/text` v0.40.0 → v0.44.0, `x/image` v0.41.0 → v0.48.0, `x/sys` v0.49.0 → v0.50.0,
  `pion/ice/v4` v4.4.8 → v4.4.9 y `pion/transport/v5` v5.1.4 → v5.1.5. Indirectas: 18, entre
  ellas `x/crypto` v0.54.0 → v0.59.0, `pion/dtls/v3` v3.1.16, `pion/sctp` v1.12.1 y
  `go-webview2` v1.0.23.
- Revisión del `go.mod`: la directiva sigue en `go 1.26.0`, ninguna ruta `/vN` nueva, no entra
  ni sale ninguna directa, sin `replace`. **No hubo que dejar ninguna dependencia atrás.**
- La CLI de Wails v2.17.1 en `release.yml` y en el mensaje de `scripts/compilar.ps1`.
- `govulncheck` v1.9.0 como paso del CI, en las tres plataformas, y en las listas de
  verificaciones (CLAUDE.md, README y los comandos `/funcionalidad-*`).

### Desviación del prompt

El prompt pedía `govulncheck` solo en `ubuntu-latest`. Quedó en la matriz de las tres
plataformas porque el análisis sigue los build tags: en Linux no revisa los `*_windows.go`
(captura H.264, servicio, bandeja, salud), y Windows es la plataforma prioritaria.

### Lo que traen las notas de versión

- **Wails v2.17.1:** GHSA-r5j2-ghh9-2jqw, solo en Linux: un iframe ya no puede usar el puente IPC
  privilegiado de WebKitGTK. Aula no incrusta iframes de otros orígenes, pero el binario de Linux
  queda cubierto. Además actualiza `echo` y `goldmark`.
- **`x/net`:** las correcciones de HTTP/2 y `html` de la lista de arriba. `govulncheck` no
  encontró llamadas de Aula a esas funciones, y con la versión nueva ya no están en el binario.
- **pion:** ICE-lite con prioridades estables y comprobación de MTU (`ice`), contextos en lugar
  de goroutines en `packetconn` (`transport`), datos tempranos en DTLS 1.2 y entrega antes de
  `close_notify` (`dtls`), orden de escritura antes del reinicio de stream (`sctp`). Sin cambios
  de API.
- **`x/sys`** y **`x/text`**: sin cambios que toquen a Aula fuera de la corrección de `precis`.

## Después

- `go list -m -u all`: quedan 42 módulos con versión nueva. **Ninguno se compila en Aula:** solo
  están en el grafo de módulos (la CLI de Wails, `go-git`, librerías de terminal) y no aparecen en
  `go.mod`. `go get -u ./...` solo sube lo que importan los paquetes de Aula, que es lo que se
  quería.
- `govulncheck` v1.9.0:
  - con Go 1.26.11 (el que toma el CI con `1.26.x` y `check-latest`): **«No vulnerabilities
    found.»**;
  - con Go 1.27.2 (el local): siguen las 2 de la biblioteca estándar y **0 en módulos**. Se
    corrigen instalando Go 1.27.4 en el equipo de desarrollo; no dependen del repositorio.
- `gofmt -l` sin salida, `go vet`, `staticcheck` v0.8.2 y `wails doctor` (*SUCCESS*, WebView2
  157) en verde. `scripts\compilar.ps1 -Solo consola` y `-Solo alumno` compilan con la CLI
  v2.17.1. El humo de los dos arneses (escenas 1–3) pasa.
- `go test -race ./...`: 29 paquetes pasan. En este equipo fallan por tiempo pruebas de
  `internal/screen` (esperan una miniatura en 3 s y, con `-race` y el CPU al 93 % por otras
  apps, codificarla tarda más). **No es una regresión:** con el `go.mod` original fallan más (4
  por corrida) y sin `-race` pasan. Queda para el CI, que corre en máquinas sin carga.

## Pendiente

- Casos manuales C-154 a C-156 y A-65 (`docs/desarrollo/probar-consola-local.md`): consola y
  alumno compilados, descubrimiento y TLS, snippet, transmisión, diálogos e instancia única.
- Que el CI pase en las tres plataformas con el paso nuevo de `govulncheck`.
- Actualizar Go en el equipo de desarrollo a 1.27.4 o superior.
- Si las pruebas de `internal/screen` también fallan en el CI, darles más margen es otra
  corrección, fuera de esta etapa.
