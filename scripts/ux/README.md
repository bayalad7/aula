# Arnés web de la consola del profesor

Wails dibuja la consola en un WebView nativo que no se puede manejar desde fuera. Por eso el
frontend real (`apps/teacher/frontend/dist`) se prueba aquí en Chromium headless, con un doble de
los bindings de Go (`dobles.js`) y el manual real exportado desde Go. Nada de esto entra en el
binario: `go build` no necesita Node.

## Requisitos

- Node 22 o más nuevo y Go (para exportar el manual).
- `npm ci` en esta carpeta (instala Puppeteer, que baja su propio Chromium, y axe-core).

## Comandos

| Comando | Qué hace |
|---|---|
| `node recorrido.mjs [carpeta]` | Recorre 53 pantallas y estados (claro, oscuro, 900×600), guarda las capturas (por omisión en `./salida`, ignorada por git) y pasa axe-core (WCAG 2.1 AA). Sale con error si hay errores de JS o violaciones serias o críticas. |
| `node teclado.mjs` | Recorre cada vista solo con Tab: el foco nunca se pierde y siempre se ve, no hay trampas (del editor se sale con Ctrl+M y Tab) y Esc cierra paneles y diálogos devolviendo el foco a quien los abrió. Corre en el CI. |
| `node usuario-nuevo.mjs` | Sigue «Primeros pasos» con el aula simulada: cada control que nombra el manual debe existir con esa etiqueta exacta. Cuenta clics y teclas. Corre en el CI. |
| `node despues.mjs [carpeta]` | Las 26 capturas «después» de la etapa 18 (las mismas pantallas que `docs/v1/ux/profesor/antes/`), con axe-core. Por omisión escribe en `docs/v1/ux/profesor/despues/`. |
| `node capturas-manual.mjs` | Regenera las capturas del manual en `docs/v1/administracion/img/`. Úsalo cuando cambie una pantalla que sale en el manual y revisa el diff de las imágenes antes del commit. |

## Cómo funciona

- `arnes.mjs` corre `go test -run TestExportManualForUX ./apps/teacher` con `AULA_UX_MANUAL` para
  sacar el manual tal como lo devuelve `Manual()`, sirve `dist/` en `127.0.0.1` (con el sistema de
  diseño de `internal/ui` en `sistema/`, como la consola) y abre cada escena en un contexto
  aislado del navegador (el tema guardado no pasa de una a otra).
- `dobles.js` reemplaza `window.go.main.App` y `window.runtime`. La escena (`window.__SCENE`)
  elige los datos: `vacia`, `datos`, `lab`, `lab-errores`, `lab-restr-activa`,
  `lab-transmitiendo`, `lab-energia-pendiente`, `simulada`, `simulada-sola`, `clave-personal`,
  `setup-cargando`, `setup-error`, `atajo-ocupado`. `window.__emit(evento, datos)` simula un evento de Go.
- **Si cambia un binding o un evento** (`apps/teacher/events.go`, `docs/eventos-ui.md`), cambia
  también `dobles.js`; si no, el recorrido prueba una consola que ya no existe.

## Qué no cubre

La ventana nativa (tamaño, atajos globales, portapapeles real, WebView2/WebKit) y el backend Go:
lo primero se prueba a mano con `docs/desarrollo/probar-consola-local.md`, lo segundo con
`go test ./apps/teacher`.
