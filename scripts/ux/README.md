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
| `node recorrido.mjs [carpeta] [--solo=…]` | Recorre las pantallas y estados (claro, oscuro, 900×600), guarda las capturas (por omisión en `./salida`, ignorada por git) y pasa axe-core (WCAG 2.1 AA). Sale con error si hay errores de JS o violaciones serias o críticas. `--solo=archivos,94-105` corre solo esas capturas (por palabra del nombre o por número): para revisar un cambio en un par de minutos; el completo lo corre el CI. |
| `node teclado.mjs` | Recorre cada vista solo con Tab: el foco nunca se pierde y siempre se ve, no hay trampas (del editor se sale con Ctrl+M y Tab) y Esc cierra paneles y diálogos devolviendo el foco a quien los abrió. Corre en el CI. |
| `node usuario-nuevo.mjs` | Sigue «Primeros pasos» con el aula simulada: cada control que nombra el manual debe existir con esa etiqueta exacta. Cuenta clics y teclas. Corre en el CI. |
| `node recorrido-alumno.mjs [carpeta] [--solo=…]` | La app del alumno (`apps/student`, con `dobles-alumno.js`): cada estado a 900×700 y 480×360, claro y oscuro, con axe-core. Por omisión en `./salida-alumno`. Corre en el CI. |
| `node pruebas-alumno.mjs` | Comportamiento de la app del alumno (27 casos): lo nuevo arriba, «Nuevo», sin saltos al llegar código, ↑/↓ y Ctrl+Mayús+C, el foco no se mueve, `replace`/`clear`, el error al copiar, modo compacto, avisos, estados vacíos, **bandas de transparencia que no se pueden cerrar** y su orden, ciclo de Tab y nada animado en reposo. Corre en el CI. |
| `node capturas-manual.mjs` | Regenera las capturas del manual en `docs/administracion/img/`. Úsalo cuando cambie una pantalla que sale en el manual y revisa el diff de las imágenes antes del commit. |
| `node anotar.mjs [profesor o alumno] [escena]` | **Anotador de interfaz** (guía: `docs/desarrollo/notas-ux.md`). Abre la consola o la app del alumno con datos de demostración en una ventana de Chromium que manejas tú. En la barra (abajo a la izquierda; se arrastra por su asa a cualquier parte y M la pasa arriba o abajo) eliges app, escena, vista y tema; navegas normal y cada clic deja seleccionado lo que tocaste, con su cadena de zonas (eliges el nivel: el elemento o toda la sección); N abre la nota. «Señalar» (A) anota sin que el clic llegue a la app y «Zonas» (Z) muestra el nombre de cada zona al pasar el mouse. Cada nota queda en `docs/ux/notas/` (`NNN.json` y dos capturas WebP; índice en `LEEME.md`) y se trabaja con `/notas-ux`. `--prueba` la ejercita sin ventana (corre en el CI); `--indice` solo regenera `LEEME.md`. |
| `node mapa.mjs` | Genera `docs/ux/MAPA.md` y `docs/ux/mapa/*.webp`: cada pantalla con sus zonas numeradas y su nombre. La fuente de los nombres es `zonas.mjs` (también la usa el anotador): si una zona cambia de lugar o de selector, se cambia ahí y se regenera el mapa. |

## Cómo funciona

- `arnes.mjs` corre `go test -run TestExportManualForUX ./apps/teacher` con `AULA_UX_MANUAL` para
  sacar el manual tal como lo devuelve `Manual()`, sirve `dist/` en `127.0.0.1` (con el sistema de
  diseño de `internal/ui` en `sistema/`, como la consola) y abre cada escena en un contexto
  aislado del navegador (el tema guardado no pasa de una a otra).
- `dobles.js` reemplaza `window.go.main.App` y `window.runtime`. La escena (`window.__SCENE`)
  elige los datos: `vacia`, `datos`, `lab`, `lab-errores`, `lab-restr-activa`,
  `lab-transmitiendo`, `lab-energia-pendiente`, `lab-temporizador`, `lab-temporizador-fin`, `simulada`, `simulada-sola`, `clave-personal`,
  `setup-cargando`, `setup-error`, `atajo-ocupado`. `window.__emit(evento, datos)` simula un evento de Go.
- **Si cambia un binding o un evento** (`apps/teacher/events.go`, `docs/eventos-ui.md`), cambia
  también `dobles.js`; si no, el recorrido prueba una consola que ya no existe.

## Qué no cubre

La ventana nativa (tamaño, atajos globales, portapapeles real, WebView2/WebKit) y el backend Go:
lo primero se prueba a mano con `docs/desarrollo/probar-consola-local.md`, lo segundo con
`go test ./apps/teacher`.
