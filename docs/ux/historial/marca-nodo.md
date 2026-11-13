# Marca Nodo y sistema de diseño v0.1.0 (2026-11-12)

Rediseño en tres fases, solo de presentación: no cambió HTML funcional (ids, `data-*`, roles), JS,
bindings ni eventos. Referencia vigente: `docs/ux/DESIGN_SYSTEM.md`.

## Análisis

- `internal/ui` ya era un sistema de diseño completo (tokens claro/oscuro con prueba de contraste,
  ~150 componentes, 61 iconos) y las apps no tenían colores sueltos.
- Faltaba la marca: la consola usaba el icono `codigo` como logo y los iconos de app eran
  genéricos.
- Había tres valores y dos tamaños de letra sueltos en `componentes.css`.
- La cuadrícula de miniaturas ya no tenía efectos caros (sin `backdrop-filter`, sin transiciones).
- Hallazgo: la raíz de las apps mide 15 px (`html` toma `--t-md`), no 16; la documentación de la
  escala estaba corrida.

## Decisiones (del usuario)

1. Logo **Nodo** entre cuatro propuestas (Nodo, Foco, Escudo, Trazo).
2. **Plus Jakarta Sans** para los títulos, embebida en el binario (D-021); texto del sistema.
3. Un token **`--t-codigo`** de 14 px para el código que el alumno copia (antes 13.1 px).
4. Página de marca (artifact privado del usuario) con los SVG oficiales y la marca en JSON, para
   presentaciones y la página oficial.

## Resultados

- Tokens nuevos: `--c-marca-tinta`, `--c-marca-nodo`, `--c-sobre-solido`,
  `--c-sobre-solido-texto`, `--c-sombra-barra`, `--t-codigo`, con sus pares de contraste.
- `#marca` en el sprite, componente `.logo` (`.marca` ya eran las casillas de selección).
- `contain: layout paint` en `.alumno`.
- `scripts/ux/marca.mjs`: SVG oficiales (`internal/ui/marca/`), `appicon.png` e `icon.ico` de las
  dos apps desde una sola geometría, sin máscaras.
- Logo en la navegación y el arranque de la consola, y en la tira y el modo compacto del alumno.
  A pedido del usuario, el menú lleva el logotipo centrado con «aula» en minúsculas (dibujado,
  `#marca-horizontal`) en lugar de «Aula» como texto.
- Arnés: escenas 01, 04 y 05 de la consola y 05 y 56 del alumno sin errores ni violaciones;
  `pruebas-alumno.mjs` 61 de 61. Manual y mapa regenerados (cambian por la letra de los títulos).

## Pendiente

Casos manuales C-119, C-120, A-51 y A-52 en `docs/desarrollo/probar-consola-local.md`: iconos de
la barra de tareas y del área de notificación, y la fuente servida por Wails sin red.
