---
description: Trabaja las notas de interfaz que el usuario dejó con el anotador (docs/v1/ux/notas)
argument-hint: [números de nota, p. ej. 3 5-8] [todas]
---

# /notas-ux

El usuario señaló cosas de la interfaz con el anotador (`node scripts/ux/anotar.mjs`) y escribió
qué quiere. Cada nota está en `docs/v1/ux/notas/NNN.json` con dos capturas
(`NNN-elemento.webp`, `NNN-pantalla.webp`) y el índice en `docs/v1/ux/notas/LEEME.md`.

Lo que pidió: **$ARGUMENTS** (sin argumentos: todas las `pendiente`).

## 1. Lee y entiende

- Lee `LEEME.md`, los JSON de las notas pedidas y **mira sus capturas** (Read sobre los .webp):
  `NNN-elemento` y `NNN-pantalla`, y si `tema` es `ambos`, también `NNN-elemento-oscuro` y
  `NNN-pantalla-oscuro`. Un cambio de una nota `ambos` se revisa en los dos temas.
- Cada nota trae: `app`, `escena` y `ruta` (cómo reproducirla en el arnés), `zona` (nombres de
  `docs/v1/ux/MAPA.md` / `scripts/ux/zonas.mjs`), `elemento` (texto visible, clave de
  `textos.es-MX.js` en `claveTexto` y un selector), `tipo` (mover, texto, estilo, agregar, quitar,
  error, idea) y `texto`.
- Ubica el código: la clave de texto lleva a `textos.es-MX.js` y de ahí a `index.html`/`main.js`;
  la zona, a su selector en `zonas.mjs`.

## 2. Agrupa y propone (detente para que apruebe)

- Junta las notas que tocan lo mismo y dilo («3 y 7 son el mismo cambio»).
- Por cada grupo: qué entendiste, el **antes** (su captura) y el **después** en un boceto en
  texto con los nombres del mapa, y cuánto toca (solo estilo y textos, o también Go, protocolo,
  manual o pruebas).
- Si una nota choca con una regla de `CLAUDE.md` (privacidad, «no quita el foco», bandas,
  «Ubicación y vocabulario»), dilo y ofrece una alternativa; no la apliques a ciegas.
- Una nota ambigua: pregunta (AskUserQuestion, con tu recomendación primero), no adivines.
- Si un grupo es una función nueva o mueve una función a otro módulo, dilo: se trabaja con
  `/funcionalidad-mejora` (o `/funcionalidad-nueva`) y su «Definición de terminado».
- Pon esas notas en `en curso` (campo `estado` del JSON) y espera la aprobación.

## 3. Aplica lo aprobado

- Solo componentes, tokens e iconos de `internal/ui`; textos en `textos.es-MX.js`; claro, oscuro
  y 900×600 (alumno: 480×360).
- Si una zona cambió de lugar o de selector, actualiza `scripts/ux/zonas.mjs`.
- Usa Edit/Write para textos con barras invertidas (`Documentos\Aula`), no `sed`/`perl`.

## 4. Verifica y cierra

- Solo lo que toca el cambio: la prueba de textos (`go test ./apps/teacher` o `./apps/student`)
  y las capturas de esas pantallas en el arnés (`node scripts/ux/recorrido.mjs --solo=…`). El
  recorrido completo lo corre el CI; en local, solo si el cambio es amplio.
- Muestra cada nota con su captura de **después** (el arnés, en la misma escena) junto a la de
  antes.
- Marca las notas `hecha` (o `descartada`, con el motivo en un campo `respuesta`) y regenera el
  índice con `node scripts/ux/anotar.mjs --indice`.
- Si cambió el mapa, `node scripts/ux/mapa.mjs`; si cambió una pantalla del manual,
  `node scripts/ux/capturas-manual.mjs`.
- No hagas commit ni push hasta que el usuario lo pida (`documentacion(ux)` para las notas;
  el ámbito de la app para el cambio).
