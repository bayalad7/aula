# Sistema de diseño de Aula (v0.1.0)

Este documento describe el sistema de diseño de la consola del profesor y de la app del alumno:
la marca (el logo **Nodo**), los colores por función en claro y oscuro, la tipografía, los tokens
y las reglas para usarlos. Lo escribe la fase 1 del rediseño. Las fases 2 y 3 lo aplican sin tocar
la lógica de las apps.

**La fuente de verdad es el código:** `internal/ui/tokens.css` y `internal/ui/componentes.css`.
Si este documento y `tokens.css` no coinciden, manda `tokens.css` y este documento se corrige.

---

## 1. Lo que ya existe (auditoría, 2026-11-12)

| Pieza | Dónde | Qué tiene |
|---|---|---|
| Tokens | `internal/ui/tokens.css` (222 líneas) | 55 colores con claro y oscuro, 3 familias tipográficas, escala de 6 tamaños, 7 espacios, 4 radios, 2 sombras, medidas de la estructura |
| Componentes | `internal/ui/componentes.css` (893 líneas) | ~150 clases: botones, campos, insignias, tarjetas, alumno (monitor), tablas, toast, franjas, diálogos, pasos, código, navegación, panel lateral, ajustes, app del alumno |
| Iconos | `internal/ui/iconos.svg` + `iconos.js` | Sprite de 61 iconos de línea (24 × 24); `data-icono` en botones y campos |
| Video | `internal/ui/receptor-video.js` | Receptor WebRTC compartido (no es presentación) |
| Entrega | `internal/ui/ui.go` | `go:embed`; cada frontend Wails lo recibe bajo `sistema/` (`ui.Assets`). Una sola copia |
| Pruebas | `contrast_test.go`, `icons_test.go`, `ui_test.go` | WCAG AA de cada par texto/fondo en los dos temas, oscuro automático = oscuro manual, todo color claro tiene su oscuro; iconos del sprite |
| Consola | `apps/teacher/frontend/dist/style.css` (186 líneas) | Solo composición propia (arranque, código, manual, ficha); sin colores sueltos |
| Alumno | `apps/student/frontend/dist/style.css` (94 líneas) | Solo composición (tira, bandas, modo compacto); sin colores sueltos |
| Iconos de app | `apps/*/build/appicon.png`, `build/windows/icon.ico` | Icono genérico (en la fase 2 pasó a Nodo); la bandeja del alumno se escala desde `appicon.png` (`apps/student/trayicon.go`) con un punto `#1d4ed8` |

Frontend sin framework ni bundler (D-002): HTML, CSS y JS planos, servidos por Wails (WebView2 en
Windows). No hay estilos en línea (`style="…"`: 0 en los dos `index.html`).

### Hallazgos

1. **La marca no existe en la interfaz.** La navegación y el arranque de la consola usan el
   icono `codigo` como logo (`index.html`, `.nav-lateral__marca`, `.arranque__marca`). Los iconos
   de app son genéricos.
2. **Tres valores sueltos en `componentes.css`:** el botón sobre `.franja--transmitiendo`
   (`#ffffff`, `#7f1212`) y la sombra de la barra de acciones (`rgb(17 24 39 / 0.06)`), que no
   cambia con el tema.
3. **Dos tamaños fuera de la escala:** el contador de la navegación compacta (`0.65rem`) y el
   código de la app del alumno (`0.875rem`).
4. **Rendimiento de la cuadrícula:** bien. `.alumno` no tiene `transition`, ni
   `backdrop-filter`, ni sombras animadas; la selección es un `box-shadow` fijo. Los únicos
   `backdrop` son el velo de los diálogos (`::backdrop` con color plano, sin desenfoque).
5. **Base sólida que se conserva:** nombres de tokens en español (`--c-`, `--t-`, `--e-`,
   `--r-`), tema por `prefers-color-scheme` o `data-theme`, contraste probado. El rediseño
   **no renombra tokens**: decenas de reglas y el arnés dependen de ellos.

---

## 2. Marca: Nodo

Cuatro pantallas alrededor de un punto central: la consola del profesor que une los equipos de la
clase. Elegido entre cuatro propuestas el 2026-11-12.

### Construcción (rejilla de 32 × 32)

| Elemento | Medida |
|---|---|
| Pantallas | 4 cuadrados de 12 × 12, radio 3, separados 4 (x/y = 2 y 18) |
| Recorte | Círculo de radio 7.5 en el centro (16, 16) que muerde la esquina interior de cada pantalla |
| Nodo | Círculo de radio 4.5 en el centro, color de acento |
| Área de respeto | 4 unidades (1/8 del lado) alrededor del isotipo |
| Logotipo horizontal | Isotipo a 40 px + «aula» en minúsculas a 10 px del isotipo, en `viewBox 0 0 116 40` |
| Letras «aula» | Dibujadas, no una fuente: trazo de 4.4, ojos de la «a» y curva de la «u» de radio 6 (como el radio 7.5 del recorte), altura x de 12.8 a 29.2, «l» desde 4, separación de 3.6 |

La guía visual con los SVG oficiales para copiar, la paleta y los componentes de muestra está en
la página de marca: <https://claude.ai/artifact/UWEnHMdoZsa73aqLW8iYbq> (privada; sus datos en
JSON están en el bloque `#aula-marca` de la página).

La relación radio/lado del cuadrado (3/12 = 1/4) es la misma de las tarjetas del sistema
(`--r-lg` 10 px sobre bloques de ~40 px de alto): el logo y la interfaz comparten esquinas.

### Colores del logo

| Tema | Pantallas (`--c-marca-tinta`) | Nodo (`--c-marca-nodo`) |
|---|---|---|
| Claro | `#1a1f2b` (= `--c-texto`) | `#1d4ed8` (= `--c-acento`) |
| Oscuro | `#e7eaf0` (= `--c-texto`) | `#6b8cff` (más claro que el acento de botones para que el punto se lea sobre `#12151c`: 5.9:1) |
| Una tinta | Todo en `currentColor` (sobre fotos, franjas sólidas, impresión) | — |

### Tamaños mínimos

- Isotipo: **16 px** (bandeja del sistema). A 16 px el recorte se pierde en algunos píxeles; se
  acepta, las cuatro pantallas y el punto se siguen leyendo.
- Logotipo horizontal: **24 px de alto**. Más chico, solo el isotipo.

### No hacer

- Cambiar el color del nodo por un color de estado (verde, rojo): el nodo es la marca, no un
  indicador de conexión.
- Rotar el isotipo, añadir sombras, degradados o contornos.
- Escribir «Aula» con mayúscula en el logotipo (en el texto corrido, sí: «Aula»).
- Usar el isotipo como icono de una acción. Los iconos de acción son los del sprite.

### Archivos

Todo sale de una sola geometría en `scripts/ux/marca.mjs` (`node scripts/ux/marca.mjs`). Las
pantallas son trazados con la esquina mordida ya dibujada, sin máscaras, para que cualquier
rasterizador (iconos, instaladores, presentaciones) las dibuje igual.

- `internal/ui/iconos.svg#marca`: el isotipo para las apps, con `class="logo"` (`.logo`,
  `.logo--sm`, `.logo--lg`): pantallas en `currentColor` (`--c-marca-tinta`) y punto en
  `--c-marca-nodo`. `.marca` es otra cosa: las casillas de selección de alumnos.
- `internal/ui/iconos.svg#marca-horizontal`: el logotipo (isotipo y «aula» dibujado, en
  minúsculas) con `class="logotipo"`; lo usa el menú de la consola, centrado.
- `internal/ui/marca/`: `aula-isotipo.svg`, `aula-horizontal.svg` y sus variantes `-oscuro` y
  `-una-tinta`, más `aula-icono-app.svg` (el isotipo oscuro sobre una placa `#12151c` de radio
  7/32, con 3 unidades de margen).
- `apps/*/build/appicon.png` (1024 px) y `apps/*/build/windows/icon.ico` (16, 20, 24, 32, 40,
  48, 64 y 256 px) desde `aula-icono-app.svg`. El icono de la bandeja del alumno sale de
  `appicon.png` (`apps/student/trayicon.go`); su punto de «llegó algo» queda en la esquina,
  fuera del nodo central.

---

## 3. Color

Los colores se nombran por su **función**, nunca por su tono. Cada estado tiene tres tokens:
texto (`--c-x`), fondo suave (`--c-x-fondo`) y sólido para puntos y botones (`--c-x-solido`).
Los valores son los vigentes de `tokens.css`; **nuevo** marca lo que agrega la fase 2.

### Superficies y texto

| Token | Claro | Oscuro | Uso |
|---|---|---|---|
| `--c-fondo` | `#f4f5f7` | `#12151c` | Fondo de la ventana |
| `--c-superficie` | `#ffffff` | `#1a1f29` | Tarjetas, paneles, diálogos, tabla |
| `--c-superficie-2` | `#eef0f4` | `#232a37` | Bloques dentro de una tarjeta, código, pantalla sin imagen |
| `--c-texto` | `#1a1f2b` | `#e7eaf0` | Texto principal |
| `--c-texto-suave` | `#4f5869` | `#a7b0bf` | Ayudas, metadatos, etiquetas |
| `--c-borde` | `#d5dae2` | `#313949` | Separadores, bordes de tarjeta |
| `--c-borde-fuerte` | `#858fa0` | `#6f7a8c` | Bordes de controles (3:1) |
| `--c-velo` | negro 45 % | negro 60 % | Fondo detrás de un diálogo |

Los grises tiran a azul pizarra (no son grises neutros): conversan con el azul del nodo.

### Marca y acción

| Token | Claro | Oscuro | Uso |
|---|---|---|---|
| `--c-marca-tinta` **nuevo** | `#1a1f2b` | `#e7eaf0` | Pantallas del logo |
| `--c-marca-nodo` **nuevo** | `#1d4ed8` | `#6b8cff` | Punto del logo |
| `--c-acento` | `#1d4ed8` | `#3a66dd` | Botón primario, selección, interruptor encendido |
| `--c-acento-hover` | `#1e40af` | `#2f5fd8` | Botón primario al pasar el ratón |
| `--c-acento-texto` | `#ffffff` | `#ffffff` | Texto sobre el acento |
| `--c-acento-suave` | `#e6edff` | `#1d2a4d` | Elegido en un segmentado, fila seleccionada |
| `--c-acento-sobre-suave` | `#1e3a8a` | `#b9ceff` | Texto sobre el acento suave |
| `--c-foco` | `#1d4ed8` | `#8db4ff` | Anillo de foco del teclado |
| `--c-enlace` | `#1d4ed8` | `#8db4ff` | Enlaces |

**Una sola acción primaria por zona.** El azul es la marca y la acción: no se usa para decorar.

### Estados

| Estado | Texto claro / oscuro | Fondo claro / oscuro | Sólido claro / oscuro | Uso |
|---|---|---|---|---|
| Conectado | `#146c34` / `#6fd697` | `#e3f4e8` / `#12301e` | `#1c8a44` / `#3fae6a` | Alumno en línea, acuse, éxito |
| Desconectado | `#4f5869` / `#a7b0bf` | `#eceef2` / `#262d39` | `#8a93a3` / `#6f7a8c` | Sin conexión, sin dato |
| Advertencia | `#8a4600` / `#f3bc6a` | `#fff1d6` / `#3a2a10` | `#c26a00` / `#d58a1c` | Pendiente, energía en cuenta regresiva |
| Error | `#b42318` / `#ff9b91` | `#fde8e6` / `#3d1715` | `#c8322a` / `#c8322a` | Fallo, acción destructiva |
| Simulado | `#6230c9` / `#c5b0ff` | `#efe8ff` / `#2a1f47` | `#7c4ae0` / `#8d63ee` | Alumnos simulados: siempre distinguibles de los reales |
| Info | `#1d4ed8` / `#8db4ff` | `#e6edff` / `#1d2a4d` | — | Avisos neutros |

Un estado nunca depende solo del color: siempre va con su icono (`conectado`, `desconectado`,
`advertencia`, `error`, `simulado`, `info`) o su texto.

### Transparencia y video (app del alumno y miniaturas)

| Token | Claro | Oscuro | Uso |
|---|---|---|---|
| `--c-vista-solido` / `-texto` | `#0b5d6b` / `#ffffff` | `#7fd1db` / `#06292f` | Bandas «El profesor ve tu pantalla»: informa, no alarma; distinto del rojo y del azul |
| `--c-video-fondo` / `-texto` | `#000000` / `#e7eaf0` | igual | Transmisión del profesor |
| `--c-monitor-marco` | `#1f2937` | `#0b0e14` | Marco del monitor de cada miniatura |
| `--c-monitor-pie` | `#9ca3af` | `#4b5563` | Pie del monitor |
| `--c-sobre-solido` **nuevo** | `#ffffff` | `#ffffff` | Botón dentro de una franja sólida (reemplaza el `#ffffff` suelto) |
| `--c-sobre-solido-texto` **nuevo** | `#7f1212` | `#7f1212` | Texto de ese botón (reemplaza el `#7f1212` suelto) |
| `--c-sombra-barra` **nuevo** | negro 6 % | negro 35 % | Sombra de la barra de acciones (reemplaza el `rgb()` suelto) |

### Sintaxis del código (sobre `--c-superficie-2`)

| Token | Claro | Oscuro |
|---|---|---|
| `--c-sintaxis-comentario` | `#57606f` | `#9aa4b5` |
| `--c-sintaxis-clave` | `#6d28d9` | `#c9adff` |
| `--c-sintaxis-cadena` | `#146c34` | `#7ed69c` |
| `--c-sintaxis-numero` | `#9a4a00` | `#f3bc6a` |
| `--c-sintaxis-funcion` | `#1d4ed8` | `#8db4ff` |
| `--c-sintaxis-tipo` | `#0b6375` | `#6fd0e0` |
| `--c-sintaxis-operador` | `#374151` | `#d0d5de` |

### Reglas de contraste (las exige `contrast_test.go`)

- Texto: **4.5:1** sobre su fondo, en los dos temas.
- Bordes de controles, anillo de foco y marcas de estado: **3:1**.
- Todo token `--c-*` del claro existe en el oscuro, y el oscuro automático es igual al manual.
- Un token nuevo entra en la lista de pares de la prueba en el mismo cambio.

---

## 4. Tipografía

**Títulos en Plus Jakarta Sans, texto con la fuente del sistema** (decisión del 2026-11-12,
D-021). Aula corre en laboratorios sin internet, así que Plus Jakarta Sans va **dentro del
binario**: `internal/ui/fuentes/plus-jakarta-sans.woff2` (variable 200–800, subconjunto latino,
27 KB, licencia OFL en `fuentes/OFL.txt`), declarada con `@font-face` en `tokens.css` y servida
bajo `sistema/fuentes/`. Nada se descarga. El texto sigue con Segoe UI, que es la que mejor dibuja
Windows a tamaños pequeños. El logotipo no depende de ninguna fuente: «aula» está dibujado.

| Token | Pila | Uso |
|---|---|---|
| `--f-titulo` | **Plus Jakarta Sans**, Segoe UI Variable Display, Segoe UI, system-ui, -apple-system, Ubuntu | Títulos `h1`–`h4` |
| `--f-texto` | Segoe UI Variable Text, Segoe UI, system-ui, -apple-system, Ubuntu, Cantarell | Interfaz y texto |
| `--f-mono` | Cascadia Mono, Consolas, SF Mono, DejaVu Sans Mono | Código, canal, huellas, cifras técnicas |

Para presentaciones y la página oficial: Plus Jakarta Sans (títulos), Inter (texto) y
JetBrains Mono (código), de Google Fonts.

### Escala

La raíz no mide 16 px: `html` toma `--t-md` (0.9375 × 16 = **15 px**), así que en las apps
**1 rem = 15 px**. Los px de la tabla son los que se ven.

| Token | rem | px | Uso |
|---|---|---|---|
| `--t-xs` | 0.75 | 11.3 | Insignias, contadores |
| `--t-sm` | 0.8125 | 12.2 | Ayudas, metadatos, pie de miniatura |
| `--t-codigo` | 0.9333 | 14 | El código que el alumno lee y copia (`.codigo-recibido__codigo`) |
| `--t-md` | 0.9375 | 14.1 | Texto base, botones, campos |
| `--t-lg` | 1.0625 | 15.9 | Títulos de panel y de tarjeta (`h2`), marca en la navegación |
| `--t-xl` | 1.375 | 20.6 | Título de vista (`h1`) |
| `--t-2xl` | 1.75 | 26.3 | Arranque, pantallas vacías, asistentes |

- Pesos: `--peso-normal` 400, `--peso-medio` 600 (títulos `h2`/`h3`, nombres), `--peso-fuerte`
  700 (`h1`, cifras clave). No hay más pesos.
- Interlineado: `--lh` 1.45 en texto; 1.25 en títulos; 1.6 en el manual.
- Cifras que cambian (temporizador, conteos, porcentajes): `font-variant-numeric: tabular-nums`.
- Mayúsculas solo en etiquetas cortas de grupo, con interletra 0.06 em.
- Ya no hay tamaños sueltos: el contador de la navegación compacta usa `--t-xs` (antes 9.8 px) y
  el código del alumno `--t-codigo` (antes 13.1 px).

---

## 5. Espacio, forma, profundidad y movimiento

### Espaciado (cuartos de rem)

Con la raíz de 15 px, el paso es de 3.75 px (casi 4).

| Token | rem | px | Uso típico |
|---|---|---|---|
| `--e-1` | 0.25 | 3.75 | Icono ↔ texto, insignias |
| `--e-2` | 0.5 | 7.5 | Dentro de un control, entre botones |
| `--e-3` | 0.75 | 11.25 | Cabecera de tarjeta, filas |
| `--e-4` | 1 | 15 | Cuerpo de tarjeta, margen de la vista |
| `--e-5` | 1.5 | 22.5 | Entre secciones |
| `--e-6` | 2 | 30 | Arranque, estados vacíos |
| `--e-7` | 3 | 45 | Pantallas vacías grandes |

Los hermanos se separan con `gap` en flex o grid, no con márgenes.

### Radios

| Token | px | Uso |
|---|---|---|
| `--r-sm` | 4 | Insignias, casillas |
| `--r-md` | 6 | Botones, campos, pantalla de la miniatura |
| `--r-lg` | 10 | Tarjetas, paneles, diálogos, tarjeta de alumno |
| `--r-pill` | 999 | Interruptor, puntos de estado, contador |

### Sombras

| Token | Uso |
|---|---|
| `--sombra-1` | Tarjetas en reposo (casi plana) |
| `--sombra-2` | Lo que flota: diálogos, toast, globo de información, menús |

La jerarquía se marca con bordes y superficies; la sombra solo dice «esto flota».

### Medidas de la estructura

`--ancho-nav` 13.5 rem, `--ancho-nav-compacta` 3.75 rem, `--ancho-panel` 25 rem,
`--alto-control` 2.25 rem (36 px: botones y campos), `--ancho-miniatura` (lo elige el profesor).
La consola funciona a **900 × 600**.

### Movimiento

`--transicion` 140 ms ease-out, solo en color, borde y la perilla del interruptor. Con
`prefers-reduced-motion: reduce` vale 0 ms. Nada se anima al aparecer, salvo el giro de «trabajando».

---

## 6. Componentes (catálogo)

| Familia | Clases | Notas |
|---|---|---|
| Acción | `.btn` (`--primario`, `--fantasma`, `--peligro`, `--peligro-texto`, `--icono`, `--sm`, `--lg`, `--sobre-color`), `.grupo-botones`, `.segmentado`, `.rejilla-opciones` | Todo botón lleva icono (`data-icono`); deshabilitado sin opacidad; `aria-busy` muestra el giro |
| Formularios | `.campo`, `.entrada` (`--area`, `--prefijo`), `.control`, `.opciones-seg` / `.opcion-seg`, `.interruptor`, `.opciones-tarjeta`, `.buscador`, `.zona-archivos` | Todo campo lleva icono; una casilla es un interruptor; los radios son opciones agrupadas |
| Estado | `.insignia--*`, `.estado-conexion`, `.capacidades`, `.paso--*`, `.cargador`, `.esqueleto`, `.vacio` | Color + icono + texto |
| Contenedores | `.tarjeta`, `.panel-lateral`, `.dialogo` (`--form`, `--peligro`), `.modal-ajustes`, `.detalles` | |
| Avisos | `.toast--*`, `.aviso-en-linea--*`, `.franja--*`, `.bandas` | Franjas globales arriba del contenido; las bandas del alumno no se cierran |
| Datos | `.tabla`, `.tabla-adaptable`, `.barras` / `.barra-opcion`, `.bloque-codigo`, `.visor-diagrama` | La tabla pasa a tarjetas en ventana angosta |
| Alumno (miniatura) | `.alumno`, `.alumno__monitor`, `.alumno__pantalla`, `.alumno__pie`, `.cuadricula-alumnos` | Ver reglas de rendimiento |
| Estructura | `.app`, `.nav-lateral`, `.principal`, `.vista`, `.barra-acciones`, `.migas`, `.indice-manual` | |
| App del alumno | `.codigo-recibido`, `.codigos`, `.modo-compacto`, `.pantalla-pausa` | |

---

## 7. Rendimiento de la cuadrícula en vivo

La consola pinta decenas de miniaturas que se reemplazan cada pocos segundos. Dentro de
`.alumno` y sus hijos:

- **Prohibido:** `backdrop-filter`, `filter` (incluido `drop-shadow`), `mix-blend-mode`,
  sombras grandes o animadas, `transition` o `animation` en `box-shadow`, `transform` o tamaño,
  degradados encima de la imagen, `will-change` permanente.
- **Permitido:** bordes, fondos planos, un `box-shadow` fijo para la selección, pseudoelementos
  estáticos (el pie del monitor), insignias planas.
- Hover y selección cambian solo `border-color` o `box-shadow` fijos, sin transición.
- La imagen va en un `<img>` con `object-fit: cover` dentro de una caja con `aspect-ratio`: cambiar
  el `src` no mueve la rejilla.
- Fase 2 agrega `contain: layout paint` a `.alumno` para que repintar una miniatura no recalcule
  a sus vecinas. Es solo CSS; no toca el JS que actualiza las imágenes.
- El velo de los diálogos sigue siendo color plano, sin desenfoque.

---

## 8. Reglas de uso

1. Solo tokens: ningún color, tamaño de letra, radio o sombra suelto en `componentes.css` ni en
   el `style.css` de una app. Un valor nuevo es un token nuevo en `tokens.css`, con su oscuro y su
   par en `contrast_test.go`.
2. Lo común va en `internal/ui`; el `style.css` de cada app solo compone su página.
3. Los nombres de tokens y clases no se cambian: el rediseño cambia valores y agrega, no renombra.
4. Claro, oscuro y 900 × 600 en toda pantalla de la consola.
5. Iconos del sprite, de línea, a 1.25 rem por defecto (`.icono`); el isotipo de la marca no es
   un icono de acción.
6. Foco visible con `--c-foco` en todo lo que se alcanza con el teclado.
7. El rediseño es solo presentación: no cambia HTML funcional (ids, `data-*`, roles), JS,
   bindings ni eventos.

---

## 9. Fases del rediseño

**Fase 1 — auditoría y este documento:** hecha (2026-11-12).

**Fase 2 — tokens y primitivos** (`internal/ui`): hecha (2026-11-12).

- Tokens nuevos en los tres bloques de tema: `--c-marca-tinta`, `--c-marca-nodo`,
  `--c-sobre-solido`, `--c-sobre-solido-texto`, `--c-sombra-barra` y `--t-codigo`; sus pares en
  `contrast_test.go`.
- Plus Jakarta Sans embebida para `--f-titulo`; `TestCSSURLsAreEmbedded` comprueba que todo
  `url()` del CSS va dentro del binario, con su licencia.
- `#marca` en el sprite y el componente `.logo`.
- Los tres valores y los dos tamaños sueltos, a tokens.
- `contain: layout paint` en `.alumno`.
- SVG de la marca e iconos de app de las dos apps (`scripts/ux/marca.mjs`).

**Fase 3 — vistas:** hecha (2026-11-12); detalle en `historial/marca-nodo.md`.

- Consola: el logotipo «aula» (`.logotipo`, en minúsculas y dibujado) centrado en el menú, que
  en la ventana angosta pasa a solo el isotipo; el isotipo (`.logo--lg`) en el arranque. Ambos
  en lugar del icono `codigo`.
- Alumno: el logo (`.logo--sm`) en la tira de identidad y en el modo compacto; el icono nuevo
  en la bandeja (sale de `appicon.png`).
- Las vistas ya usaban solo tokens y componentes: no hubo que cambiar su composición.
- Capturas del manual y del mapa regeneradas; casos C-119, C-120, A-51 y A-52.
