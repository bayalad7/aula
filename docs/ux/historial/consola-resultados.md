# Resultados de la etapa 18: la consola antes y después

Cierre de la Fase 7 del prompt 18. Compara la consola de `ANALISIS-CONSOLA.md` (capturas en
`antes/`, del 30/10/2026) con la de hoy (capturas en `despues/`, del 04/11/2026). Las capturas
«después» las genera `scripts/ux/despues.mjs` con los mismos estados que `antes/`; las dos series
salen del arnés web (Chromium headless), no de la ventana Wails.

## Resumen

- De los **24 hallazgos** de `ANALISIS-CONSOLA.md` §4 (4 críticos, 7 altos, 8 medios, 5 bajos),
  23 quedan resueltos y uno parcial: a A2 le faltan los atajos Alt+1…5 entre módulos. Del flujo
  a quedan pendientes el filtro y el orden de la lista de Alumnos.
- **axe-core:** 0 violaciones de cualquier nivel en las 53 capturas del recorrido y en las 26 de
  «después». Antes, 6 criterios WCAG fallaban y 4 tenían avisos.
- **Teclado:** `teclado.mjs` recorre las 5 vistas con Tab (de 17 a 27 controles cada una). El foco
  nunca se pierde y siempre se ve, no hay trampas, y Esc cierra paneles y diálogos devolviendo el
  foco. Corre en el CI.
- **Usuario nuevo:** «Primeros pasos» se cumple con **4 clics y una escritura**, y cada control que
  nombra el manual existe con esa etiqueta (`usuario-nuevo.mjs`, en el CI). Falta tu prueba en una
  VM limpia.
- **Binario de la consola:** 23,07 MB → 25,86 MB (+2,79 MB, +12 %). Compila sin Node y no carga
  nada de Internet.

## Antes y después, por pantalla

| # | Estado | Antes | Después | Qué cambió |
|---|---|---|---|---|
| 01 | Arranque: abriendo | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/antes/01-arranque-abriendo-canal.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/despues/01-arranque-abriendo-canal.png) | Formulario bloqueado mientras abre (M4) |
| 02 | Arranque: puerto ocupado | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/antes/02-arranque-error-puerto-ocupado.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/despues/02-arranque-error-puerto-ocupado.png) | «Qué pasó + qué hacer», detalle plegado (A3) |
| 03 | Sin alumnos | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/antes/03-consola-vacia-sin-alumnos.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/despues/03-consola-vacia-sin-alumnos.png) | Estado vacío con «¿Falta un alumno?» y «Probar con alumnos simulados» (C3) |
| 04 | Dos simulados con historial | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/antes/04-consola-2-simulados-con-historial.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/despues/04-consola-2-simulados-con-historial.png) | Tabla densa con estado, código en pantalla y contadores (flujo a, A5) |
| 05 | Editor con código | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/antes/05-consola-editor-con-codigo.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/despues/05-consola-editor-con-codigo.png) | Módulo Código propio con «Enviar a» (A7) |
| 06 | Confirmar «Limpiar pantallas» | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/antes/06-confirmar-limpiar-texto.txt) (texto) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/despues/06-confirmar-limpiar.png) | Diálogo propio con número y consecuencia (M1) |
| 07 | Aviso de error | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/antes/07-consola-toast-error.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/despues/07-consola-toast-error.png) | Se queda hasta cerrarlo, con enlace al manual (A3) |
| 08 | Captura cargada | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/antes/08-consola-captura-cargada.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/despues/08-consola-captura-cargada.png) | Va al borrador de Código; nunca emite sola |
| 09 | Sin clave del centro | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/antes/09-consola-clave-personal.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/despues/09-consola-clave-personal.png) | Franja con explicación, sin comando de terminal (C3) |
| 10 | Atajo ocupado | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/antes/10-consola-atajo-ocupado.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/despues/10-consola-atajo-ocupado.png) | En Ajustes › Acerca de, con la alternativa |
| 11 | Laboratorio | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/antes/11-laboratorio-tarjetas.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/despues/11-laboratorio-tarjetas.png) | Capacidades solo como ausencia, en lenguaje llano (A5) |
| 12 | Alumnos marcados | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/antes/12-laboratorio-marcar-alumnos.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/despues/12-laboratorio-marcar-alumnos.png) | Selección con barra de acciones fija (C1) |
| 13 | Miniaturas | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/antes/13-pantallas-cuadricula.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/despues/13-pantallas-cuadricula.png) | Modo de Alumnos, no una capa (C1, M5) |
| 14 | Pantalla en grande | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/antes/14-pantallas-detalle.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/despues/14-pantallas-detalle.png) | Diálogo; las franjas siguen visibles (M5) |
| 15 | Miniaturas solo con simulados | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/antes/15-pantallas-vacia-simulados.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/despues/15-pantallas-vacia-simulados.png) | Los simulados muestran pantalla de prueba (C4) |
| 16 | Transmitiendo | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/antes/16-transmitiendo-pantalla.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/despues/16-transmitiendo-pantalla.png) | Franja roja y «Detener» siempre visibles |
| 17 | Restricciones sin reglas | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/antes/17-restricciones-vacia.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/despues/17-restricciones-vacia.png) | Panel lateral con plantillas (C1) |
| 18 | Restricciones activas | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/antes/18-restricciones-activas.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/despues/18-restricciones-activas.png) | Quién las tiene, en la lista |
| 19 | Nadie puede recibirlas | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/antes/19-restricciones-simulados-sin-soporte.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/despues/19-restricciones-simulados-sin-soporte.png) | «Aplicar» deshabilitado con su motivo (M2) |
| 20 | Energía | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/antes/20-energia.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/despues/20-energia.png) | Panel lateral sobre la selección (C1) |
| 21 | Energía forzada | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/antes/21-energia-confirmar-forzado.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/despues/21-energia-confirmar-forzado.png) | Mismo diálogo de confirmación que el resto (M1) |
| 22 | Energía pendiente | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/antes/22-energia-pendiente-tarjetas.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/despues/22-energia-pendiente-tarjetas.png) | Una franja con «Cancelar» y el estado en la lista (B5, M3) |
| 23 | 900×600 | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/antes/23-ventana-minima-900x600.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/despues/23-ventana-minima-900x600.png) | Menú compacto, nada se tapa (A1) |
| 24 | 900×600, laboratorio | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/antes/24-ventana-minima-laboratorio.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/despues/24-ventana-minima-laboratorio.png) | Sin botón flotante (A1, B3) |
| 25 | Oscuro | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/antes/25-modo-oscuro.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/despues/25-modo-oscuro.png) | Tokens con contraste AA y selector de tema (M6) |
| 26 | Oscuro, energía | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/antes/26-modo-oscuro-energia.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/profesor/despues/26-modo-oscuro-energia.png) | Iconos del sprite, sin ⏻ roto (M3, M6) |

## Flujos clave (§3)

Clics desde la pantalla principal con la clase abierta; N = alumnos.

| Flujo | Antes | Después |
|---|---|---|
| a) Ver quién está y quién no | 0 clics, pero 3–4 alumnos por pantalla y desconectados sin contraste | 0 clics: tabla densa, contadores «2 conectados · 1 desconectado · 2 simulados». **Pendiente:** filtro «solo desconectados» y orden por estado |
| b) Por qué un alumno no aparece | Sin camino en la consola | 1 clic («¿Falta un alumno?») + 1 («Comprobar la conexión»): revisa el anuncio, el puerto, la conexión segura y la red y el firewall, y hace las preguntas del diagrama 12 |
| c) Miniaturas | 1 clic; la capa tapaba todo | 1 clic (Lista ⇄ Miniaturas), con la misma selección y barra de acciones |
| c) Transmitir | 2 clics | 2 clics («Mostrar mi pantalla» → «Empezar») |
| c) Recoger código | No existe | Sigue fuera de alcance (prompt 18) |
| d) Emitir | 1 clic | 1 clic, y «Enviar a: todos / seleccionados» |
| d) Quién falta y reenviar | Imposible | 2 clics («Quién falta» → «Reenviar a los que faltan») |
| d) Reemitir | 1 clic, sin confirmar | 2 clics: confirma cuántos alumnos verán el reemplazo (A6) |
| Restricciones a N seleccionados (C1) | 6 + N clics y cambio de contexto | N + 2 clics: seleccionar, «Restricciones», «Aplicar» |
| Cambiar nombre o canal (C2) | 1 clic sin aviso; borraba el historial | 2 clics, con diálogo que dice qué pasa; el historial se conserva |

## Hallazgos de `ANALISIS-CONSOLA.md` §4

| | Hallazgo | Estado | Dónde se resolvió |
|---|---|---|---|
| C1 | Acciones en capas que tapan a los alumnos | ✅ | Selección + barra de acciones + paneles laterales (E3) |
| C2 | Guardar nombre o canal reinicia la clase sin aviso | ✅ | Diálogo con consecuencia; el historial es de la consola (E3) |
| C3 | Sin ayuda cuando un alumno no aparece | ✅ | «¿Falta un alumno?» del diagrama 12, datos para quien instala (E7) y el comprobador de conexión (E11) |
| C4 | No se puede probar sin laboratorio | ✅ | Aula simulada, marcada y contada aparte |
| A1 | 900×600 rompe el diseño | ✅ | Menú compacto; capturas a 900×600 en el recorrido |
| A2 | Trampa de teclado y orden de foco | ◐ | «Saltar al contenido», nota de Ctrl+M en Código y en el manual, canales recientes con foco, `teclado.mjs` en el CI. **Pendiente:** Alt+1…5 entre módulos (no se agregaron atajos nuevos en el Paso 4) |
| A3 | Errores técnicos y efímeros | ✅ | Códigos estables, «qué pasó + qué hacer», detalle plegado y enlace al manual (E2); errores junto al campo (Paso 4) |
| A4 | Los acuses no dicen quién falta | ✅ | «Quién falta» y «Reenviar a los que faltan» |
| A5 | Jerga técnica en primer plano | ✅ | Ficha del alumno; capacidades solo como ausencia |
| A6 | Reemitir sin confirmar; cargar escondido | ✅ | «Abrir en el editor» y «Reemitir» con confirmación |
| A7 | La jerarquía no refleja la tarea | ✅ | Navegación del §7; una acción primaria por vista |
| M1 | Confirmaciones inconsistentes | ✅ | Un solo diálogo de confirmación |
| M2 | Acciones habilitadas que no pueden funcionar | ✅ | Botones deshabilitados con su motivo |
| M3 | ⏻ como cuadro vacío | ✅ | Sprite de iconos SVG |
| M4 | Sin estados de carga | ✅ | Botones ocupados que no se repiten, «Cargando el manual…» (Paso 4) |
| M5 | Las miniaturas no permiten actuar | ✅ | Modo de la vista Alumnos |
| M6 | Modo oscuro incompleto | ✅ | Tokens AA (prueba de contraste en `internal/ui`), selector de tema, Monaco cambia de tema |
| M7 | Estructura semántica | ✅ | `h1` por vista, *landmarks*, etiquetas reales; axe en 0 |
| M8 | Textos dispersos | ✅ | `textos.es-MX.js` con prueba de claves que faltan o sobran |
| B1 | Canal como `number` con flechas | ✅ | Campo de texto numérico con explicación |
| B2 | Plurales | ✅ | Claves `.uno` / `.varios` |
| B3 | Botón flotante que tapa | ✅ | «Pegar del portapapeles» en Código |
| B4 | «Confirmaciones» mezclada y cortada | ✅ | «Últimos envíos» con desglose |
| B5 | Franja de energía duplicada | ✅ | Una franja de estado con «Cancelar» |

## Accesibilidad (WCAG 2.1 AA, §5)

| Criterio | Antes | Después | Cómo se comprueba |
|---|---|---|---|
| 1.4.3 Contraste | ❌ | ✅ | `internal/ui/contrast_test.go` y axe |
| 1.4.10 Reajuste | ❌ | ✅ | Capturas a 900×600 |
| 1.4.1 Uso del color | ⚠️ | ✅ | Estados con icono + texto |
| 2.1.1 Teclado | ⚠️ | ✅ | `teclado.mjs` |
| 2.1.2 Sin trampas | ❌ | ✅ | `teclado.mjs`: del editor se sale con Ctrl+M y Tab, indicado en pantalla |
| 2.4.1 Saltar bloques | ❌ | ✅ | «Saltar al contenido» |
| 2.4.3 Orden de foco | ⚠️ | ✅ | Diálogos modales; el foco vuelve al cerrar paneles (Paso 4) |
| 2.4.6 Encabezados y etiquetas | ❌ | ✅ | axe |
| 2.4.7 Foco visible | ✅ | ✅ | `teclado.mjs` falla si un foco no se ve (comprobado quitando el contorno) |
| 2.5.3 Etiqueta en el nombre | — | ✅ | axe. Se corrigió en esta entrega: la miniatura «Esperando imagen…» |
| 3.3.1 / 3.3.3 Errores | ⚠️ | ✅ | Error junto al campo, con foco y sin perder lo escrito |
| 4.1.3 Mensajes de estado | ✅ | ✅ | `role=status` / `aria-live` |

## Usuario nuevo

**En el arnés** (`node scripts/ux/usuario-nuevo.mjs`), siguiendo «Primeros pasos» al pie de la
letra:

| # | Paso | Acciones |
|---|---|---|
| 1 | La clase ya está abierta («Clase abierta · canal») | 0 |
| 2 | Abrir el aula simulada («Probar con alumnos simulados») | 1 |
| 3 | Ver a «Simulado 1» y «Simulado 2» | 0 |
| 4 | «Código», escribir, «Emitir (reemplaza lo que ven)» | 2 clics + escribir |
| 5 | «Últimos envíos» → «Quién falta» («Reenviar a los que faltan» a la vista) | 1 |

Total: 4 clics y una escritura. La meta de menos de 5 minutos depende de leer y entender, no de
clics: eso solo lo dice una persona. **Pendiente:** tu prueba en una VM limpia con el instalador
local (`docs/desarrollo/instaladores-locales.md`), solo con el aula simulada, anotando lo que
confunda.

## Binario y build

| | Antes de la etapa 18 (`5af6107`) | Después (`develop`) |
|---|---|---|
| `aula-consola` (Windows, `-s -w`) | 23,07 MB | 25,86 MB (+2,79 MB, +12 %) |

- De dónde sale el aumento: 1,78 MB de SVG de los diagramas, unos 0,4 MB del manual en Markdown,
  0,17 MB del frontend y del sistema de diseño, y el resto es código Go nuevo (manual, aula
  simulada, comprobador). Se podría recortar quitando de los SVG el CSS que Mermaid repite.
- `go build` no necesita Node: los SVG están en el repositorio.
- `frontend/dist` no carga nada de Internet. Las únicas URL son el espacio de nombres SVG, un
  texto de ayuda («sin https://») y comentarios de licencia de Monaco.
- `go test -race` corre en Windows, Linux y macOS en el CI.
