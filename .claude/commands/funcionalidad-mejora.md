---
description: Cambia o corrige una función ya implementada de Aula sin romper lo que ya existe
argument-hint: <función o pantalla> <qué cambiar> [tipo: corrección|mejora|cambio] [para: profesor|alumno|ambos]
---

# /funcionalidad-mejora

Cambia una función que **ya existe** en Aula: corrige un error, mejora su uso o cambia su
comportamiento. Para algo que no existe, usa `/funcionalidad-nueva`.

Lo que pidió el usuario: **$ARGUMENTS**

## 0. Entiende el pedido

Lee `CLAUDE.md` (sobre todo «Definición de terminado») y `docs/progreso.md`. Si el pedido
nombra un prompt de `docs/v1/prompts/` («12.2») o su ruta, léelo: es la especificación (hoy,
deseado, diseño y pruebas), con su «Encaje con lo implementado» si lo trae. Lo que diga el
usuario en este comando manda sobre el prompt.

Arma esta ficha con lo que dice el pedido y lo que deduces del código. **Pregunta solo lo que no
puedas deducir y cambie lo que harás**, en una sola tanda (AskUserQuestion, máximo 4 preguntas,
con tu recomendación primero):

| Parámetro | Qué es | Si no lo dice |
|---|---|---|
| **Función** | La que cambia: pantalla («Código», «la ficha»), acción («Energía») o paquete | Búscala en el código y el manual; si hay dos candidatas, pregunta |
| **Hoy** | Cómo se comporta ahora | Compruébalo en el código o con el arnés (`scripts/ux`); no lo supongas |
| **Deseado** | Cómo debe comportarse | Obligatorio: pregunta si falta |
| **Tipo** | `corrección` (no hace lo que debía), `mejora` (UX, textos, rendimiento sin cambiar qué hace) o `cambio` (hace otra cosa) | Dedúcelo |
| **Para quién** | `profesor`, `alumno` o `ambos` | Dedúcelo de dónde vive la función |
| **¿Cambia lo que ve o recibe el alumno?** | Texto, aviso, banda, foco, ventana | Dedúcelo; si sí, revisa las reglas de la app del alumno en «Definición de terminado» |
| **Cómo reproducirlo** | Pasos, captura o mensaje de error (en una corrección) | Pide los pasos si no puedes reproducirlo |
| **Alcance** | ¿Toca el protocolo, la configuración guardada, la privacidad o lo que ve el alumno? | Dedúcelo |
| **Versión** | `corrección` sin cambio externo → `z`; mejora o cambio compatible → `y`; rompe el protocolo o la configuración → `x` | Dedúcelo y dilo |

Muestra la ficha al usuario.

## 1. Encuentra todo lo que depende de la función

Antes de cambiar nada, haz el inventario:
- el código (`internal/*`, `apps/*`, bindings) y sus pruebas;
- los textos (`textos.es-MX.js`) y los errores (`errores.go`, `agenterrors.go`);
- el manual (`docs/v1/administracion/`, busca la función y sus capturas);
- los diagramas (`docs/v1/diagramas/`, sus `pantallas` y «Fuentes en el código»);
- `docs/protocol.md`, `docs/eventos-ui.md` y `docs/privacidad.md`;
- el arnés de la consola (`scripts/ux/dobles.js`, `recorrido.mjs`) y el del alumno
  (`dobles-alumno.js`, `recorrido-alumno.mjs`, `pruebas-alumno.mjs`);
- la app del alumno (`apps/student`, su `textos.es-MX.js`, `window.go` si toca la ventana) y el
  ayudante del servicio (`internal/agentsvc`) si la función pasa por él;
- el prompt de la función en `docs/v1/prompts/` (su «Encaje con lo implementado») y su nota de
  entrega en `docs/v1/ux/`;
- los casos de `docs/desarrollo/probar-consola-local.md` (C-xx y A-xx).

Lo que diga algo distinto después del cambio también se actualiza.

## 2. Diseño (solo si hace falta aprobarlo)

Detente y espera la aprobación si el cambio:
- toca el protocolo o la configuración guardada;
- cambia lo que ve o recibe el alumno, o la privacidad;
- mueve algo de lugar en la consola;
- es de tipo `cambio`.

Muestra el antes y el después: un boceto en texto o una captura del arnés. Si exige cambiar el
modelo de seguridad, no lo hagas: anótalo en `docs/v1/ux/PROPUESTAS.md`.

Una `corrección` o una `mejora` pequeña y local no necesita esta pausa: dilo y sigue.

## 3. Implementación

- **Corrección:** primero una prueba que falle y reproduzca el error, luego el arreglo, y
  confirma que la prueba ya pasa.
- **Mejora o cambio:** ajusta las pruebas que describen el comportamiento anterior; nunca las
  borres para que pase.
- Cambios pequeños y solo en lo necesario; no refactorices lo que no pide la tarea.
- Las reglas de siempre:
  - regla de oro (nada de APIs del SO fuera de `platform`);
  - textos en `textos.es-MX.js` (de cada app), sin `innerHTML`; en el alumno, Go manda estados;
  - errores con código;
  - los alumnos simulados nunca reciben lo que controla un equipo;
  - en el alumno, lo que llega no quita el foco (`notify`), y una banda de transparencia nunca
    se puede cerrar.

## 4. Pruebas y cierre

- Recorre la «Definición de terminado» de `CLAUDE.md` en lo que el cambio toque, y actualiza
  todo lo del inventario del paso 1 que haya quedado distinto.
- `node scripts/ux/recorrido.mjs` sin errores de JS ni violaciones serias; si tocó al alumno,
  también `recorrido-alumno.mjs` y `pruebas-alumno.mjs`. Si cambió una pantalla del manual,
  `node scripts/ux/capturas-manual.mjs`; si cambió un diagrama, `scripts/docs/generar-diagramas`.
- Ajusta o agrega los casos manuales de `docs/desarrollo/probar-consola-local.md`.
- `gofmt -l .` sin salida, `go vet ./...`,
  `go run honnef.co/go/tools/cmd/staticcheck@v0.8.2 ./...` y `go test -race ./...`.
- Actualiza `docs/progreso.md`.

Al terminar, muestra el antes y el después (con capturas si cambió algo visible), lo que falta
probar a mano y la versión que sugiere el cambio. Espera la aprobación. **No hagas commit ni
push hasta que el usuario lo pida**; en `develop`, con `correccion` o `caracteristica` según el
tipo.
