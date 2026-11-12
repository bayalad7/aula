# Análisis de UX/UI de la app del alumno (prompt 19, Fase 1)

Estado: **para aprobación**. Fecha: 04/11/2026. Sin cambios de código en la app: solo el arnés y
la herramienta de medición.

## 1. Resumen

La app cumple lo básico: el código se ve coloreado, se copia exacto desde Go y cada acción del
profesor tiene su aviso. Los problemas de fondo son tres:

1. **Interrumpe al llegar algo, y no se deja apartar.**
   - Cada código o aviso trae la ventana al frente: la muestra y la pone «siempre encima» 1,5 s.
   - Cerrar la ventana **termina el agente**, o en modo servicio hace que se reabra. Eso
     contradice la regla 2 de privacidad.
   - No hay forma de dejarla pequeña.
2. **Los indicadores de transparencia no forman un sistema.** Cada uno tiene su color, su
   lugar y su forma:
   - pantalla vista: barra roja;
   - transmisión: banda azul;
   - restricciones: banda amarilla con un emoji;
   - energía: banda roja con un símbolo que no se ve.
   - El rojo de «tu pantalla se ve» es el mismo que el de los errores y el apagado.
3. **Lo que se lee está pensado para TI, no para el alumno.** Diagnósticos con «consola», «Red
   Pública», «firewall» y rutas de archivos. Un distintivo que repite «Aula —» y pega estado,
   restricciones y energía en una sola línea. Avisos que solo se distinguen por el color.

Con lo medido, la app es **ligera en CPU** (0,1–0,3 % de un núcleo). En RAM pesa **unos 180 MB**,
que son casi todos de WebView2.

## 2. Método

- **Arnés web** (`scripts/ux/recorrido-alumno.mjs`, nuevo): el frontend real de
  `apps/student/frontend/dist` en Chromium headless.
  - Los bindings los simula `scripts/ux/dobles-alumno.js`; `apps/student/guardrails_test.go`
    exige que cubra cada binding.
  - Pasa axe-core (WCAG 2.1 AA y buenas prácticas).
  - Ventana de 900×700 (la de `main.go`) y su mínimo, 480×360.
- **Ventana real** (`tools/medir-alumno`, nuevo):
  - una consola de prueba en `127.0.0.1` con clave temporal;
  - el agente compilado con `-directo`;
  - medición de CPU y memoria privada del agente y de sus procesos de WebView2, y del primer
    plano al llegar un código, sin tocar teclado ni mouse.
- **Lectura del código:**
  - `apps/student/{app,main,helper,power,broadcast}.go`;
  - `frontend/dist/{index.html,main.js,style.css}`;
  - `internal/codedist`, `internal/screen` e `internal/platform/window_windows.go` (el distintivo
    nativo).

### Capturas (`antes/`)

| # | Estado | # | Estado |
|---|---|---|---|
| 01 | Sin configurar | 14 | Pantalla vista por el profesor |
| 02 | Buscando la clase | 15 | Viendo la pantalla del profesor |
| 03 | Sin consola, con diagnóstico | 16 | Restricciones activas |
| 04 | Reconectando | 17 | Restricciones: detalles |
| 05 | Conectado, sin código | 18 | Aula cerró una app bloqueada |
| 06 | Un código | 19 | Energía: cuenta regresiva |
| 07 | Varios códigos (`append`) | 20 | Energía: forzada, en curso |
| 08 | Código copiado | 21 | Energía: falló |
| 09 | No se pudo copiar | 22 | Oscuro, varios códigos |
| 10 | Tras `code.clear` | 23 | Oscuro, aviso y restricciones |
| 11 | Aviso info | 24 | 480×360, varios códigos |
| 12 | Aviso warning | 25 | 480×360, aviso |
| 13 | Tres avisos | | |

**No existen** la pantalla en blanco (`screen.freeze`, protocolo §5.9, pendiente) ni el control
remoto (prompt 10.4). No se pueden capturar. Se diseñan en la entrega F sin conectarlos.

El **distintivo nativo** «Tu pantalla está siendo vista por el profesor»
(`platform.ScreenIndicator`) es una ventana de Windows fuera del WebView y no sale en el arnés.
Funciona: no roba clics ni foco y queda encima de todo. Se revisa en la ventana real.

### Medición (ventana real, Windows 10, este equipo)

| Estado | CPU media (% de un núcleo) | RAM privada media | RAM máxima | Procesos |
|---|---|---|---|---|
| Reposo (60 s) | 0,3 % | 182 MB | 183 MB | 7 |
| Recibiendo (un código cada 2 s, 30 códigos) | 0,1 % | 179 MB | 181 MB | 8 |

La RAM es casi toda de WebView2: 7–8 procesos `msedgewebview2.exe` cuelgan del agente. La CPU no
es un problema. La meta para RAM es no subir; las animaciones nuevas no deben despertar la CPU en
reposo.

## 3. Flujos clave: hoy y propuesta

### a) El equipo arranca → se conecta solo → el alumno sabe a qué canal

| | Hoy | Propuesta |
|---|---|---|
| Qué ve | «Aula — buscando la clase del canal 1» (02). Si tarda, un párrafo técnico: «No se encontró ninguna consola… Red Privada o de Dominio… firewall» (03). Sin configurar: «no hay configuración: falta config.json en C:\ProgramData\…» (01). | Una tira discreta: «Buscando la clase del canal 1…». Si tarda: «Aula está lista y espera a tu profesor. Si la clase ya empezó, avísale.», con «Detalles para TI» plegado. Sin configurar: «Aula aún no está instalada del todo en este equipo. Avisa a tu profesor.», con el detalle plegado. |
| Conectado | «Aula — conectado a la clase de Prof. Ayala (canal 1)» (05), y el área vacía repite «Cuando el profesor comparta código…» en todos los estados, incluso sin configurar o sin conexión. | Tira: «Conectado al canal 1 · Profesor Ayala». Estado vacío propio de cada caso («Listo. Cuando tu profesor comparta código, aparecerá aquí.»). |

### b) El profesor emite código → aparece → el alumno copia y pega

| | Hoy | Propuesta |
|---|---|---|
| Al llegar | `onBoard` → `raise()`: la ventana sale de minimizada y se pone «siempre encima» 1,5 s. En la medición, con VS Code en primer plano, **Windows no le dio el foco del teclado a Aula**, pero la ventana se dibuja encima del editor (comprobación visual pendiente en la ventana real). El frontend re-dibuja la lista entera y hace `scrollIntoView` suave al último: si el alumno estaba leyendo otro, salta (07, 24). | Mostrar **sin activar** y hacer parpadear la barra de tareas; marcar «Nuevo» y contar en el título y el icono. Si el alumno la cerró a la bandeja, solo cambia el icono. Lo nuevo va arriba, sin mover lo que el alumno está leyendo. |
| Copiar | Un clic en «Copiar al portapapeles» (Go copia el texto exacto, con sangría y saltos). «Copiado ✓» 2 s. Si falla, «No se pudo copiar» y el motivo solo en un `title` (09). | Igual de directo, más **Ctrl+Mayús+C** copia el fragmento seleccionado y ↑/↓ cambia de fragmento. El error se explica junto al botón. |
| Varios | `append` los pone al final; `replace` deja uno; `clear` vuelve al vacío (10). | Lista navegable, lo nuevo arriba, hora de llegada; `clear` con un vacío amable. |

### c) El profesor manda un aviso → el alumno lo ve sin que le tape el trabajo

| | Hoy | Propuesta |
|---|---|---|
| Qué pasa | `onNotice` → `raise()`. El aviso entra en el flujo de la página, **arriba del código, y lo empuja hacia abajo** (13). Se acumulan hasta 10. Info y warning solo se distinguen por el color de fondo (WCAG 1.4.1). `aria-live="assertive"` interrumpe al lector de pantalla. | Toast en una esquina, sobre el contenido pero sin desplazarlo, con color + icono + texto por nivel, «Entendido» y Esc. Los avisos se apilan con un máximo visible y «N avisos más». Sin traer la ventana al frente. |

### d) El profesor ve, transmite, restringe o apaga → el alumno entiende qué pasa

| | Hoy | Propuesta |
|---|---|---|
| Ver la pantalla | Distintivo nativo fijo encima de todo, y la barra de la ventana en **rojo** (el color de error) con el texto largo (14). | El distintivo nativo se queda igual. En la ventana, la banda de transparencia «Tu pantalla la está viendo el profesor Ayala» con su icono, en un color propio (no el de error) y sin poder cerrarse. |
| Transmitir | Banda azul «▶ Estás viendo la pantalla de…» y la ventana maximizada; negro con «Conectando…» (15). Bien resuelto. | Misma banda del sistema; se conserva «Tamaño real». |
| Restricciones | Banda amarilla con 🔒 (emoji) y «Ver detalles»; la barra agrega «· restricciones activas» (16, 17). Aula cerró una app: aviso que **se esfuma a los 8 s** (18). | La misma banda del sistema, con icono del sprite. «Aula cerró Steam» queda en la banda hasta que el alumno lo lee. |
| Energía | Banda roja con **⏻, que se ve como un cuadro vacío** (19, 20); «El profesor Prof. Ayala» repite el título. «Posponer 5 min (te quedan 2)». | Banda de energía con icono del sprite y la cuenta grande: «Tu profesor va a reiniciar este equipo en 1:26. Guarda tu trabajo.» y «Posponer 5 minutos (2 veces más)». |
| Pantalla en pausa / control | No existen. | Componentes listos (entrega F) para cuando se implementen. |

## 4. El foco (punto 5 del prompt)

- **Hoy** (`app.go`: `onBoard` y `onNotice`; `main.go`: `raiseWindow`):
  - `WindowUnminimise` + `WindowShow` + `WindowSetAlwaysOnTop(true)`, y a los 1,5 s
    `WindowSetAlwaysOnTop(false)`;
  - el comentario del código dice que «siempre encima» sirve para saltarse la protección de foco
    de Windows.
- **Medido:**
  - con VS Code en primer plano, el primer código no le quitó el foco del teclado (el primer plano
    siguió siendo `Code.exe`);
  - Windows limita `SetForegroundWindow` desde procesos en segundo plano, pero esa protección no
    es una garantía (depende de la versión, de la última entrada y de la configuración);
  - la ventana sí se desminimiza y se dibuja encima: **tapa el editor aunque no tenga el foco**.
- **Propuesta** (decidida el 04/11/2026):
  - código y avisos **nunca** usan `raise()`: `ShowWindow(SW_SHOWNOACTIVATE)` sin «siempre
    encima» y `FlashWindowEx` en la barra de tareas, más la marca «Nuevo»;
  - si el alumno cerró la ventana a la bandeja, no se abre: solo cambia el icono;
  - energía y restricciones **sí** la traen al frente (reglas 20 y 26: son acciones del profesor
    que el alumno debe ver antes de que ocurran);
  - se reescribe la regla 4 de privacidad: «el código y los avisos se muestran sin quitarle el
    teclado al alumno».

## 5. Hallazgos priorizados

Heurísticas de Nielsen:
- **H1** visibilidad del estado;
- **H2** lenguaje del usuario;
- **H3** control y libertad;
- **H4** consistencia;
- **H5** prevención de errores;
- **H6** reconocer antes que recordar;
- **H7** flexibilidad;
- **H8** diseño minimalista;
- **H9** reconocer y corregir errores;
- **H10** ayuda.

### Críticos

**C1. Lo que llega del profesor interrumpe al alumno.** Capturas 06, 07 y 11 · H3, H8.

- **Qué pasa:** código y avisos desminimizan la ventana y la ponen encima del editor 1,5 s; el
  foco del teclado depende de la protección de Windows, no de Aula.
- **Por qué importa:** rompe el criterio de aceptación del prompt («sin perder lo que estaba
  escribiendo»).
- **Propuesta:** §4 y entrega D.

**C2. Cerrar la ventana termina el agente, o la ventana vuelve.** `main.go` y `helper.go` · H3,
transparencia.

- **Agente independiente:** no hay `HideWindowOnClose`: cerrar mata el proceso, y el alumno sale
  de la clase sin saberlo.
- **Como servicio:** el ayudante termina y el servicio lo relanza, así que la ventana reaparece.
- **Por qué importa:** contradice la regla 2 («al cerrar, sigue en segundo plano»).
- **Propuesta:** cerrar oculta la ventana. Aula sigue en el área de notificación con su icono, un
  texto («Aula — conectado al canal 1 · Profesor Ayala») y el menú «Abrir Aula». Entrega D.

**C3. Llegar un código mueve lo que el alumno estaba leyendo.** Capturas 07 y 24 · H3, H1.

- **Qué pasa:** cada `aula:board` re-dibuja toda la lista y hace `scrollIntoView` suave al último
  fragmento. Lo nuevo va **abajo** y el anterior queda cortado arriba (en la 07 no se ve la
  cabecera del ejercicio 1).
- **Propuesta:** re-dibujo incremental, lo nuevo arriba con «Nuevo» y sin mover el desplazamiento
  del alumno. Entrega C.

### Altos

**A1. Los indicadores de transparencia no son un sistema.** Capturas 14, 15, 16 y 19 · H4, H1.

- **Qué pasa:**
  - cuatro estilos distintos;
  - el rojo de «tu pantalla se ve» es el de los errores y el apagado: asusta y no informa;
  - el texto se mezcla en una sola línea («… (canal 1) · restricciones activas · reinicio
    pendiente»).
- **Propuesta:** una **banda de transparencia** con variantes (vista, transmisión,
  restricciones, energía, pausa, control). Cada una lleva su icono y su texto llano, sin botón de
  cerrar; se apilan en un orden fijo y desaparecen solas. Entrega F.

**A2. Los avisos empujan el código y solo se distinguen por color.** Captura 13 · H8, WCAG
1.4.1.

- **Propuesta:** toast en una esquina con icono y nivel en texto (entrega E).

**A3. Textos para TI en la cara del alumno.** Capturas 01, 03 y 04 · H2, H9.

- **Qué pasa:**
  - «No se encontró ninguna consola…», «Red Privada o de Dominio», «firewall», «config.json en
    C:\ProgramData\…»;
  - el alumno no puede hacer nada con eso: lo útil es «avisa a tu profesor».
- **Propuesta:** un mensaje llano para el alumno y el detalle plegado «Detalles para TI»
  (entrega E).

**A4. El distintivo no coincide con la regla 1 y repite «Aula —».** Todas las capturas · H4.

- **Qué pasa:** la regla dice «Aula — conectado (canal X - profesor)»; el código dice «Aula —
  conectado a la clase de X (canal N)».
- **Propuesta:** el estado en la tira, con «Conectado al canal 1 · Profesor Ayala». El título de
  la ventana y la barra de tareas conservan «Aula — …», como pide la regla, y se alinea la regla
  al texto final. Entrega E.

**A5. No usa el sistema de diseño ni tiene sus textos en un archivo.**

- **Qué pasa:**
  - `style.css` tiene su propia paleta y colores sueltos (`#b91c1c`, `#ffffff`, `#000000`);
  - los textos están repartidos entre `index.html`, `main.js` y Go (`statusFor`, `setStatus`,
    `broadcastView`, `powerPending`);
  - no hay prueba de textos.
- **Propuesta:** entrega B (tokens de `internal/ui`, `textos.es-MX.js`, prueba de textos; Go solo
  manda estados).

### Medios

- **M1. El símbolo ⏻ se ve como un cuadro vacío** (19, 20), y 🔒 depende de la fuente de emoji.
  H4. → Iconos del sprite.
- **M2. «Aula cerró Steam» se esfuma a los 8 s** (18). Si el alumno no estaba mirando, no sabe
  por qué se cerró su app. H1. → Se queda en la banda de restricciones hasta que el alumno lo lea.
- **M3. Copiar sin atajo; el error no se explica** (08, 09).
  - «No se pudo copiar» lleva el motivo solo en un `title`, que el teclado y los lectores no ven.
  - El botón «Copiar al portapapeles» es largo y a 480×360 baja a otra línea (24).
  - H7, H9. → «Copiar» con Ctrl+Mayús+C y el motivo junto al botón.
- **M4. Un solo estado vacío para todo:** «Cuando el profesor comparta código…» sale también sin
  configurar, buscando o sin conexión (01–04, 19). H1. → Un estado vacío propio por situación.
- **M5. Cabecera del código poco útil.**
  - «Código del profesor» cuando no hay título, y el lenguaje en minúsculas técnicas
    («javascript»).
  - No se sabe cuál es nuevo ni a qué hora llegó.
  - H6. → Nombre legible del lenguaje («JavaScript»), hora y «Nuevo».
- **M6. Energía con textos torpes** (19–21): «El profesor Prof. Ayala», «(te quedan 2)» y «No se
  pudo cerrar tu sesión.» sin qué hacer. H2. → Textos del §3 d.
- **M7. Sin modo compacto.** El alumno solo puede minimizar a la barra de tareas o cerrar (que hoy
  mata el agente). H3, H7. → Modo compacto (entrega D).

### Bajos

- **B1. axe:**
  - sin `h1` (moderado, 24 de 25 capturas);
  - `role="status"` en un `<header>` no permitido (menor, 25 de 25).
  - El bloque de código, cuando se desplaza a lo ancho, no se alcanza con el teclado.
- **B2. `aria-live="assertive"`** en los avisos: interrumpe al lector de pantalla. Debe ser
  `polite`, salvo la energía.
- **B3. Animación** `scroll-behavior: smooth` sin respetar `prefers-reduced-motion`.
- **B4. El arnés pide `favicon.ico`:** el `index.html` del alumno no declara icono (la consola
  usa `data:,`). No afecta a la app, pero ensucia el recorrido con un 404.

## 6. Accesibilidad (WCAG 2.1 AA)

| Criterio | Estado | Dónde | Hallazgo |
|---|---|---|---|
| 1.4.1 Uso del color | ❌ | Avisos info/warning solo por fondo | A2 |
| 1.4.3 Contraste | ✅ | axe sin `color-contrast` en claro ni oscuro | — |
| 1.4.10 Reajuste | ⚠️ | A 480×360 el botón Copiar baja de línea; el código se desplaza a lo ancho | M3 |
| 2.1.1 Teclado | ⚠️ | Botones alcanzables; no hay forma de cambiar de fragmento ni copiar sin el ratón, y el código desplazable no toma foco | M3, B1 |
| 2.4.6 Encabezados | ⚠️ | Sin `h1` | B1 |
| 2.4.7 Foco visible | ✅ | `button:focus-visible` con contorno de 2 px | — |
| 3.3.1 Errores | ⚠️ | «No se pudo copiar» sin motivo visible | M3 |
| 4.1.2 Nombre y rol | ⚠️ | `role="status"` en `<header>` | B1 |
| 4.1.3 Mensajes de estado | ⚠️ | Avisos `assertive` | B2 |

## 7. Textos (revisión del punto 4)

| Hoy | Propuesta |
|---|---|
| «Aula — conectado a la clase de Prof. Ayala (canal 1)» | Tira: «Conectado al canal 1 · Profesor Ayala». Título: «Aula — conectado (canal 1 - Prof. Ayala)» |
| «Aula — buscando la clase del canal 1» | «Buscando la clase del canal 1…» |
| «Aula — sin conexión con la clase de… ; reintentando» | «Se perdió la conexión con la clase. Reconectando…» |
| «No se encontró ninguna consola del canal 1. Revisa que la consola… firewall» | «Aula está lista y espera a tu profesor. Si la clase ya empezó, avísale.» + «Detalles para TI» |
| «no hay configuración: falta config.json…» | «Aula aún no está lista en este equipo. Avisa a tu profesor.» + «Detalles para TI» |
| «Aula — tu pantalla está siendo vista por Prof. Ayala (canal 1)» | Banda: «Tu pantalla la está viendo el profesor Ayala» |
| «▶ Estás viendo la pantalla de Prof. Ayala (canal 1)» | Banda: «Estás viendo la pantalla del profesor Ayala» |
| «🔒 Restricciones de la clase activas: aplicaciones bloqueadas · sitios web filtrados» | Banda: «Tu profesor limitó apps y sitios durante la clase» · «Ver cuáles» |
| «⏻ El profesor Prof. Ayala (canal 1) va a reiniciar este equipo en 1:26. Guarda tu trabajo.» | Banda: «Tu profesor va a reiniciar este equipo en 1:26. Guarda tu trabajo.» |
| «Código del profesor» | «Código sin título» |
| «Copiar al portapapeles» / «Copiado ✓» | «Copiar» / «Copiado» (con icono) |

El texto del distintivo nativo, «Tu pantalla está siendo vista por el profesor», lo fija la
regla 6 y no cambia.

## 8. Entregas propuestas

En el orden del plan aprobado; cada una con capturas y tu aprobación.

| Entrega | Qué | Resuelve |
|---|---|---|
| **B** | Sistema de diseño: tokens de `internal/ui` (los de Prism, también), componentes del alumno, `textos.es-MX.js` y su prueba; Go manda estados y no frases | A5, M1, B1, B4 |
| **C** | Código: lista navegable, lo nuevo arriba, sin saltos, Ctrl+Mayús+C, error visible | C3, M3, M5 |
| **D** | Ventana: avisar sin foco, modo compacto, cerrar a la bandeja con icono (`platform.Window`, `platform.Tray`) | C1, C2, M7 |
| **E** | Avisos en toast con icono, tira de estado, estados vacíos y textos para el alumno; regla 1 alineada | A2, A3, A4, M4, B2 |
| **F** | Banda de transparencia permanente; pantalla en pausa y control diseñados sin conectar | A1, M2, M6 |
| **G** | Accesibilidad, rendimiento (sin animación en reposo, `prefers-reduced-motion`), pruebas de transparencia, de foco y del portapapeles, `despues/` y `RESULTADOS.md` | B3, y la verificación |

Sin cambios de protocolo hasta ahora. Si alguno aparece, va a `docs/ux/PROPUESTAS.md` § «App
del alumno».
