# docs/ux

Lo que queda del trabajo de interfaz (UX/UI) de la consola y de la app del alumno.

| Ruta | Qué es |
|---|---|
| `DESIGN_SYSTEM.md` | El sistema de diseño: la marca Nodo, la paleta por función en claro y oscuro, la tipografía, los tokens, el catálogo de componentes y las reglas de la cuadrícula en vivo. La fuente de verdad es `internal/ui/tokens.css`. |
| `MAPA.md`, `mapa/` | El mapa de zonas: cada pantalla con sus zonas numeradas y su nombre. Lo genera `node scripts/ux/mapa.mjs` desde `scripts/ux/zonas.mjs`. |
| `notas/` | Las notas de interfaz del anotador (`node scripts/ux/anotar.mjs`), una carpeta por versión (`notas/v0.1.0/`, la de `notas/VERSION`), con su índice; `notas/LEEME.md` las resume. Se trabajan con `/notas-ux`. Al repositorio suben `NNN.json`, los `LEEME.md` y `VERSION`; las capturas (`*.webp`) se quedan en el equipo donde se anotó. |
| `PROPUESTAS.md` | Cambios que tocarían el protocolo o el modelo de seguridad, para decidir aparte. |
| `historial/` | El análisis, el diseño y los resultados de las etapas 18 (consola) y 19 (alumno), con sus decisiones. |

## Qué se quitó (prompt 20, 12/11/2026)

Hasta esa fecha la documentación vivía en `docs/v1/`. Se subió todo a `docs/`:

- `docs/v1/administracion` pasó a `docs/administracion`;
- `docs/v1/diagramas` pasó a `docs/diagramas`;
- `docs/v1/prompts` pasó a `docs/prompts`;
- `docs/v1/ux` pasó a `docs/ux`.

También se eliminaron 392 archivos (19 MiB) de evidencia de entregas terminadas:

- las capturas «antes», «después» y de cada entrega, y los `ENTREGA-*.md`, de
  `docs/v1/ux/profesor/` y `docs/v1/ux/alumno/` (etapas 18, 19 y 10.4 a 12.1);
- el script `scripts/ux/despues.mjs`, que solo regeneraba las capturas «después» de la etapa 18;
- las notas 001 a 019 (sus JSON, sus 76 capturas y su índice): las notas empezaron de cero con
  el versionado, en `notas/v0.1.0/`.

Sus pendientes ya estaban en `docs/progreso.md`, `docs/desarrollo/probar-consola-local.md` y
`docs/prueba-vms.md`. El único que no estaba se agregó a `docs/progreso.md`. Las capturas que
citan los documentos de `historial/` apuntan a ese mismo commit en GitHub.

## Convención desde la limpieza

- Al repositorio solo suben las capturas que enlaza un documento vigente: el manual
  (`docs/administracion/img/`) y el mapa (`mapa/`). Las del arnés quedan en `scripts/ux/salida*`
  y en los artefactos del CI.
- Las notas van con el versionado: una carpeta por versión, la de `notas/VERSION`. Sus capturas
  se quedan en tu equipo (`notas/**/*.webp` está en `.gitignore`).
- Cada entrega deja, si hace falta, una sola página en `historial/` con el análisis, los
  resultados y las decisiones. Nada de `ENTREGA-*` ni carpetas de capturas.

## Cómo recuperar algo

Todo sigue en el historial de git, en el commit `d3152a7`, el último antes de la limpieza (las
prerelease y sus etiquetas se borraron el mismo día: se recupera por commit, no por etiqueta).
Las capturas de las notas 001 a 019 también están ahí:

```
git show d3152a7:docs/v1/ux/profesor/ENTREGA-E3.md
git checkout d3152a7 -- docs/v1/ux/profesor/despues
```

Las rutas `d3152a7:docs/v1/ux/…` que aparecen en `docs/progreso.md` se abren así.
