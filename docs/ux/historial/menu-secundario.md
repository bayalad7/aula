# Utilidades, capas, 60-30-10 y movimiento (2026-11-17)

Prompt 10.27, decisión D-022. Solo presentación: no cambió HTML funcional (ids, `data-*`, roles),
JS, bindings ni eventos. Referencia vigente: `docs/ux/DESIGN_SYSTEM.md` (§3 Secundario, §5
Movimiento, §6 Utilidades y capas, §8 reglas 8 y 9).

## Análisis

- La pregunta fue si convenía **Tailwind CSS**. No: exige un paso de build (D-002), choca con las
  clases semánticas que no se renombran y con las pruebas que dependen de ellas, y sus valores
  arbitrarios escapan a `contrast_test.go`. Lo que mejora el diseño son los valores y la
  composición, que ya se cambian en un solo lugar (`internal/ui`).
- Ya había utilidades informales en el `style.css` de la consola (`.pila`, `.pila-mini`, `.fila`,
  `.crece`, `.lista-limpia`, `.ancho-completo`) y de texto en `componentes.css`. Hallazgo: la app
  del alumno usaba `.pila` en dos diálogos sin que la regla le llegara.
- El sistema ya usaba `@container`, `:has()` y `color-mix()`; faltaba `@layer`.
- **60-30-10:** la consola tenía los neutros (60) y el azul de acción (10), pero no el 30: el Menú
  era blanco como el contenido. El logo Nodo ya tiene la jerarquía correcta (88 % tinta, 12 %
  nodo); medido sobre su geometría, el icono de app sobre una placa reparte 64 / 32 / 4.
- Diálogos y paneles aparecían de golpe; el token `--transicion` solo se usaba en colores.

## Decisiones (del usuario)

1. Sin Tailwind; utilidades propias, cascada en capas (D-022).
2. **Opción A** de tres para el logo: el isotipo no cambia; el 30 % va al Menú y a la placa del
   icono de app (las otras eran dos tintas en diagonal y un anillo de señal).
3. El Menú en azul pizarra profundo en claro y casi negro en oscuro (más hondo que el fondo).
4. Movimiento **sutil** (frente a «marcado» y «solo fundido»).
5. El prompt se numeró 10.27 (10.17 ya era «Entregar código»).

## Resultados

- `internal/ui/utilidades.css`: las utilidades que ya existían, con el mismo comportamiento, más
  `.fila--centro`, `.empujar`, `.estirar`, `.gap-0`…`.gap-6`, `.rejilla-auto`, `.truncar`, `.num`
  y `.contenedor`. Pruebas `TestUtilitiesOnlyArrange` y `TestCSSIsLayered`.
- `@layer tokens, componentes, utilidades, vista;` en las cinco hojas.
- `--c-secundario` y su familia (9 tokens × 3 temas, 12 pares de contraste nuevos). El Menú
  redefine dentro de sí los tokens base, así que sus reglas no se reescribieron.
- Icono de app regenerado (`scripts/ux/marca.mjs`): placa `#17213a`, pantallas `#e8ecf5`, nodo
  `#7b9bff`.
- Movimiento de diálogos (también al cerrar), velo, panel lateral y avisos, con tokens `--mov-*`
  que «reducir movimiento» pone en 0. Medido en el arnés: el diálogo pasa de opacidad 0 y escala
  0.98 a 1 en unos 80–180 ms; con «reducir movimiento» aparece al instante.

### Comparación de estilos calculados (capas)

Entre capas no cuenta la especificidad, así que antes de meter las hojas en capas se grabó el
estilo calculado (50 propiedades y la caja) de cada elemento visible en las 221 escenas de la
consola y las 108 del alumno, y se comparó después:

| Diferencia | Causa | Qué se hizo |
|---|---|---|
| Colores del Menú (≈ 7 300 elementos·escena) | El cambio buscado | Nada |
| Encabezados de cantidades y acciones centrados (191 + 2) y celdas de acción `right → end` (934) | `.tabla thead th` y `.col-accion` de `style.css` ya le ganaban a reglas más específicas de `componentes.css` | `style.css` deja esas columnas a `componentes.css`; vuelto a comparar: sin diferencias |
| Diálogos del alumno con hueco de 15 px en vez de 11 px (5) | `.pila` ahora le llega al alumno | Se deja: igual que los diálogos de la consola |
| Margen del diálogo Ajustes (1) | Cómo se informa el margen `auto` | Captura idéntica al píxel |
| Un icono y 29 cajas de 1–2 px | Ruido de tiempo (giro de carga, cuenta regresiva) | No se repite al volver a grabar |

## Pendiente

Casos manuales en la ventana real (WebView2): C-144 a C-146 y A-61.
