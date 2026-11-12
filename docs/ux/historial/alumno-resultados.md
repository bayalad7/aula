# Resultados del prompt 19: la app del alumno antes y después

Cierre de la Fase 5. Compara la app de `ANALISIS.md` (capturas en `antes/`, del 04/11/2026) con la
de hoy (capturas en `despues/`, del 05/11/2026). Las dos series salen de
`scripts/ux/recorrido-alumno.mjs` sobre el frontend real (`apps/student/frontend/dist`) en el
arnés web; las cifras y el comportamiento de la ventana, de la ventana real con
`tools/medir-alumno`.

## Resumen

- **Los 19 hallazgos de `ANALISIS.md` §5 quedan resueltos:** 3 críticos, 5 altos, 7 medios y 4
  bajos.
- **Criterio de aceptación: el código llega sin quitarle el foco al alumno.**
  - Medido en la ventana real: VS Code conserva el primer plano.
  - La ventana de Aula ya no se pone encima del editor; antes se ponía «siempre encima» 1,5 s.
  - Minimizada, se queda minimizada; cerrada, no reaparece.
  - Copiar el último código: **Ctrl+Mayús+C** sin tocar el mouse, o un clic en «Copiar».
- **Transparencia:**
  - bandas permanentes con orden fijo, sin botón de cerrar ni Esc;
  - 7 pruebas del arnés y 2 de Go lo comprueban (Fase 5.2).
- **axe-core:** 0 violaciones de cualquier nivel en las 35 capturas. Antes había 2 por captura
  (sin `h1` y un rol no permitido).
- **Teclado:** el ciclo completo de Tab vuelve al principio, sin trampas y con el foco siempre
  visible, en cuatro escenas (códigos, bandas, avisos y modo compacto).
- **Recursos** (ventana real, Windows 10): CPU 0,0–0,1 % de un núcleo; RAM 187–191 MB (+9 MB). El
  ejecutable crece 0,26 MB.
- **Sin cambios al protocolo ni al modelo de seguridad.** No hubo propuestas para
  `docs/ux/PROPUESTAS.md`.

## Antes y después, por estado

| # | Estado | Antes | Después | Qué cambió |
|---|---|---|---|---|
| 01 | Sin configurar | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/antes/01-sin-configurar.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/despues/01-sin-configurar.png) | «Aula aún no está lista. Avisa a tu profesor»; lo de TI plegado |
| 02 | Buscando | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/antes/02-buscando-clase.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/despues/02-buscando-clase.png) | «Esperando a tu profesor», tranquilo |
| 03 | Sin consola | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/antes/03-sin-conexion-diagnostico.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/despues/03-sin-conexion-diagnostico.png) | La causa en palabras llanas (`session.Cause`) |
| 04 | Reconectando | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/antes/04-reconectando.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/despues/04-reconectando.png) | «Reconectando… no tienes que hacer nada» |
| 05 | Conectado | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/antes/05-conectado-reposo.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/despues/05-conectado-reposo.png) | Tira discreta «Conectado al canal 1 · Prof. Ayala» y el botón compacto |
| 06 | Un código | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/antes/06-un-codigo.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/despues/06-un-codigo.png) | Lenguaje legible, hora y «Copiar» con icono |
| 07 | Varios códigos | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/antes/07-varios-codigos.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/despues/07-varios-codigos.png) | Lo nuevo arriba, sin saltos; selección y atajo |
| 08 | Copiado | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/antes/08-codigo-copiado.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/despues/08-codigo-copiado.png) | «Copiado», anunciado al lector de pantalla |
| 09 | No se pudo copiar | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/antes/09-copia-falla.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/despues/09-copia-falla.png) | El motivo junto al código, no en un `title` |
| 10 | Tras `clear` | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/antes/10-tras-limpiar.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/despues/10-tras-limpiar.png) | Estado vacío «Listo» |
| 11–13 | Avisos | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/antes/13-varios-avisos.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/despues/13-varios-avisos.png) | En una esquina, con icono y nivel; no empujan el código |
| 14 | Pantalla vista | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/antes/14-pantalla-vista.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/despues/14-pantalla-vista.png) | Banda propia con icono, no el rojo de error |
| 15 | Transmisión | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/antes/15-transmision.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/despues/15-transmision.png) | La misma banda del sistema |
| 16–18 | Restricciones | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/antes/18-app-cerrada.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/despues/18-app-cerrada.png) | Icono del sprite; «Aula cerró…» se queda en la banda |
| 19–21 | Energía | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/antes/19-energia-cuenta.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/despues/19-energia-cuenta.png) | Sin ⏻ roto; «Prof. Ayala (canal 1) va a reiniciar…» |
| 22–23 | Oscuro | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/antes/22-oscuro-varios-codigos.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/despues/22-oscuro-varios-codigos.png) | Tokens del sistema y sintaxis con contraste AA |
| 24–25 | 480×360 | [antes](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/antes/24-ventana-minima-480x360.png) | [después](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/despues/24-ventana-minima-480x360.png) | «Copiar» no baja de línea; un solo aviso a la vista |
| 26–35 | Nuevos | — | [código nuevo](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/despues/26-codigo-nuevo-arriba.png), [compacto](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/despues/29-compacto-nuevos-y-vista.png), [avisos](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/despues/30-muchos-avisos.png), [otra clave](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/despues/33-otra-clave-detalles.png), [todas las bandas](https://github.com/bayalad7/aula/blob/d3152a7/docs/v1/ux/alumno/despues/34-todas-las-bandas.png) | Estados que antes no existían |

## Flujos clave (§3 de `ANALISIS.md`)

| Flujo | Antes | Después |
|---|---|---|
| a) Arranca y se conecta | «Aula — buscando…» y un diagnóstico técnico | «Esperando a tu profesor» y la causa en palabras llanas; título y barra de tareas con canal y profesor |
| b) Llega código y se copia | La ventana sale y se pone encima 1,5 s; la lista salta al final | Sin moverse ni quitar el foco: «Nuevo», contador en el título, parpadeo si está minimizada; **Ctrl+Mayús+C** copia lo que acaba de llegar |
| c) Llega un aviso | Trae la ventana y empuja el código; solo color | Esquina, icono y nivel; no trae la ventana; Esc lo cierra |
| d) El profesor ve, transmite, restringe o apaga | Cuatro estilos, rojo para «se ve», ⏻ roto, «Aula cerró» se esfuma | Bandas con orden fijo, sin cerrarse; energía, restricciones y transmisión sí traen la ventana |

## Hallazgos de `ANALISIS.md` §5

| | Hallazgo | Estado | Entrega |
|---|---|---|---|
| C1 | Lo que llega interrumpe | ✅ | D |
| C2 | Cerrar termina el agente o la ventana vuelve | ✅ | D |
| C3 | Llegar código mueve lo que se lee | ✅ | C |
| A1 | Transparencia sin sistema | ✅ | B, F |
| A2 | Avisos que empujan y solo por color | ✅ | E |
| A3 | Textos para TI | ✅ | B, E |
| A4 | Distintivo distinto de la regla 1 | ✅ | B, E (regla 1 alineada) |
| A5 | Sin sistema de diseño ni textos en un archivo | ✅ | B |
| M1 | ⏻ y 🔒 | ✅ | B |
| M2 | «Aula cerró» se esfuma | ✅ | F |
| M3 | Copiar sin atajo; error sin motivo | ✅ | C |
| M4 | Un solo estado vacío | ✅ | E |
| M5 | Cabecera del código | ✅ | C |
| M6 | Textos de energía | ✅ | B, F |
| M7 | Sin modo compacto | ✅ | D |
| B1 | axe (`h1`, rol) | ✅ | B, D |
| B2 | Avisos `assertive` | ✅ | B, E |
| B3 | Animación sin «reducir movimiento» | ✅ | C (sin `scrollIntoView`), G (prueba: nada anima en reposo) |
| B4 | `favicon.ico` en el arnés | ✅ | B |

## Accesibilidad (WCAG 2.1 AA)

| Criterio | Antes | Después | Cómo se comprueba |
|---|---|---|---|
| 1.4.1 Uso del color | ❌ | ✅ | Avisos y bandas con icono y texto |
| 1.4.3 Contraste | ✅ | ✅ | `internal/ui/contrast_test.go` (también la sintaxis y las bandas) y axe |
| 1.4.10 Reajuste | ⚠️ | ✅ | Capturas a 480×360 |
| 2.1.1 Teclado | ⚠️ | ✅ | ↑/↓, Ctrl+Mayús+C, Esc; ciclo de Tab en `pruebas-alumno.mjs` |
| 2.1.2 Sin trampas | ✅ | ✅ | Ciclo de Tab |
| 2.4.6 Encabezados | ⚠️ | ✅ | axe |
| 2.4.7 Foco visible | ✅ | ✅ | Ciclo de Tab |
| 2.5.3 Etiqueta en el nombre | — | ✅ | axe («Copiar»/«Copiado») |
| 3.3.1 Errores | ⚠️ | ✅ | El motivo de «No se pudo copiar» se ve |
| 4.1.2 Nombre y rol | ⚠️ | ✅ | axe |
| 4.1.3 Mensajes de estado | ⚠️ | ✅ | Avisos `polite`; «Copiado» anunciado |

## Recursos (ventana real, este equipo, Windows 10)

| | CPU (% de un núcleo) | RAM privada (agente + WebView2) | Procesos |
|---|---|---|---|
| Antes, en reposo | 0,3 | 182 MB | 7 |
| Antes, recibiendo (1 código cada 2 s) | 0,1 | 179 MB | 8 |
| **Después, en reposo** | **0,0** | **191 MB** | 7 |
| **Después, recibiendo** | **0,1** | **187 MB** | 7 |

- La RAM es casi toda de WebView2. Los 9 MB de más son del sistema de diseño (tokens,
  componentes, el sprite) y de la lista por partes.
- En reposo no corre ninguna animación (lo comprueba `pruebas-alumno.mjs`). Con «reducir
  movimiento», las transiciones duran 0.
- **Ejecutable `aula-cliente`** (Windows, `-s -w`): 14,30 MB antes del prompt 19 (`8f75614`) →
  14,56 MB (+0,26 MB, +1,8 %). Compila sin Node; el frontend no carga nada de Internet.

## Pendiente

- **A ojo, en la ventana real** (lo nativo que el arnés no ve; sin clics sintéticos en tu
  escritorio): casos A-01 a A-08 de `docs/desarrollo/probar-consola-local.md`.
  - el icono del área de notificación y su punto;
  - el modo compacto;
  - el parpadeo de la barra de tareas.
- **Pantalla en pausa y control remoto:** diseñados (prototipos), sin conectar; se implementan
  con sus etapas.
- **Linux y macOS:** sin área de notificación ni control nativo de la ventana. Cerrar minimiza y
  el modo compacto usa solo Wails.
