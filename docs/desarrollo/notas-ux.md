# Notas de interfaz: cómo pedir cambios de UX/UI

Para pedir un cambio en la interfaz no hace falta describir dónde está algo: lo **señalas**. El
anotador abre la consola o la app del alumno con datos de demostración; haces clic en lo que
quieres cambiar y escribes qué quieres. Cada nota se guarda con el nombre de la zona y dos
capturas, y Claude la trabaja con `/notas-ux`.

| Pieza | Dónde | Para qué |
|---|---|---|
| Anotador | `node scripts/ux/anotar.mjs` | Señalar y escribir las notas |
| Mapa de la interfaz | [`docs/v1/ux/MAPA.md`](../v1/ux/MAPA.md) | Los nombres de cada zona («Ficha del alumno», «Barra de acciones»…) |
| Notas | `docs/v1/ux/notas/` (índice en `LEEME.md`) | Lo que pediste, con su estado |
| Comando | `/notas-ux` en Claude Code | Que Claude las revise, te proponga el cambio y lo aplique |

## 1. Abrir el anotador

Requisitos: Node 22 o más nuevo y Go, y una vez `npm ci` en `scripts/ux` (los mismos del arnés web).

```powershell
cd scripts\ux               # desde la raíz del repositorio (p. ej., cd C:\aula\scripts\ux)
node anotar.mjs              # la consola del profesor
node anotar.mjs alumno       # la app del alumno
node anotar.mjs profesor lab-archivos   # la consola, en una escena concreta
```

Tarda unos segundos (exporta el manual) y abre una ventana de Chromium. Es una ventana normal: la
manejas tú, con tu mouse y tu teclado. No toca la consola real ni a ningún alumno: todo son datos de
demostración.

## 2. La barra del anotador

Abajo a la izquierda:

| Control | Qué hace |
|---|---|
| **Consola / Alumno** | Cambia de app |
| **Escena** | Los datos de demostración: «Laboratorio: 5 equipos reales», «Controlando un alumno», «Envío de archivos con un fallo», «Mano levantada»… Elige la que muestre lo que quieres comentar |
| **Vista** | (Consola) Alumnos, Código, Archivos, Participación, Aula simulada, Ayuda… |
| **Ambos / Claro / Oscuro** | El tema. Con **Ambos** (el que viene puesto) la ventana se ve en claro y cada nota se guarda con capturas en claro y en oscuro, para cambios que valen en los dos temas. Con Claro u Oscuro, solo en ese tema |
| **Anotar selección (N)** | Abre la nota de lo último que tocaste |
| **Señalar (A)** | Señalar sin que el clic llegue a la app (para un botón que no quieres pulsar) |
| **Zonas (Z)** | Muestra el nombre de la zona al pasar el mouse, sin bloquear los clics |
| **Nota de la pantalla** | Una nota sobre toda la pantalla, sin seleccionar nada |
| **Notas** | Las notas guardadas, con «Borrar» |
| **Ocultar (H)** | Esconde la barra si tapa algo |
| **Selección** (segunda línea) | Lo que tocaste, como cadena de zonas: «Consola › Vista Alumnos › Ficha del alumno › Qué puede hacer su equipo › «Control remoto: no»». Cada nivel es un botón |

La app funciona normal mientras anotas: puedes abrir la ficha de un alumno, un panel o la Pantalla
grande. El tamaño de la ventana también cuenta: si algo se ve mal a 900×600, achica la ventana
antes de anotar.

## 3. Hacer una nota

La forma rápida, navegando:

1. **Usa la app normal y haz clic en lo que quieres comentar** (un botón, una tarjeta, una
   sección). El clic funciona como siempre y, además, queda seleccionado: un recuadro punteado azul
   lo marca y la barra muestra su cadena de zonas.
2. **Elige el nivel**, si la nota no es del elemento sino de toda la sección: pulsa el nivel en la
   barra («Ficha del alumno» en vez de «Tomar control»). Por omisión es lo que tocaste.
3. **Pulsa N** (o «Anotar selección»). Se abre el formulario con la misma cadena en «Sobre:», por
   si quieres cambiar el nivel ahí mismo.
4. Elige el **tipo** (Mover de lugar · Cambiar texto · Estilo / diseño · Agregar algo · Quitar ·
   Algo falla · Idea / duda), escribe **qué quieres** y guarda con **Ctrl+Enter**. **Esc** cancela.

Así anotas varias cosas seguidas sin cambiar de modo: clic, N, escribir, Ctrl+Enter; clic, N…

Si el elemento hace algo al pulsarlo que no quieres (cerrar un panel, enviar, apagar), usa
**Señalar (A)**: el cursor cambia a una cruz, el clic no llega a la app y abre la nota directo.
Con **Zonas (Z)** ves el nombre de cada zona al pasar el mouse antes de tocar nada.

Se guarda con una captura de lo elegido (si es un elemento suelto, con la zona que lo contiene) y
otra de la pantalla completa, con lo elegido resaltado.

### Cómo escribir una buena nota

- **Una idea por nota.** Si son tres cambios, son tres notas: así se aprueban o descartan por
  separado.
- **Di qué quieres, no solo qué está mal.** «Esto debería ser una grilla de iconos verde/gris con
  tooltip» es mejor que «no me gusta».
- **Si es «mover», di a dónde**, con los nombres del mapa: «que también salga en la Pantalla
  grande, junto a Tomar control».
- **Si tienes una referencia** (otra app, un ejemplo), menciónala en el texto.
- No hace falta decir en qué archivo está: la nota guarda la zona, el texto visible del elemento y
  su clave de textos.

Ejemplo de nota: *tipo* «Estilo / diseño» sobre «Qué puede hacer su equipo» — «Que sea una grilla
de 5 columnas, un icono por acción, verde con ✓ si puede y gris con ✗ si no; el nombre en un
tooltip.»

## 4. Que Claude las trabaje

Cuando termines (cierra la ventana del anotador), en Claude Code:

```
/notas-ux              # todas las pendientes
/notas-ux 3 5-8        # solo esas
```

Claude:

1. Lee las notas y **mira sus capturas**.
2. Las agrupa (si dos piden lo mismo, lo dice) y te muestra por cada grupo el **antes** y un boceto
   del **después**, con cuánto toca (solo estilo, o también Go, manual o pruebas).
3. Si una nota choca con una regla del proyecto (privacidad, «no quita el foco», las bandas del
   alumno, dónde va cada cosa), te lo dice y propone otra forma.
4. **Espera tu aprobación**, aplica lo aprobado y te muestra la captura de después.
5. Marca cada nota como `hecha` o `descartada` (con el motivo).

Si una nota es en realidad una función nueva o mover una función de módulo, Claude te lo dice y la
trabaja con `/funcionalidad-mejora` o `/funcionalidad-nueva`.

## 5. Estado de las notas

`docs/v1/ux/notas/LEEME.md` es el índice: número, estado, dónde, tipo, qué pediste, escena y
enlaces a las capturas. Los estados:

| Estado | Significa |
|---|---|
| `pendiente` | Recién escrita |
| `en curso` | Claude la propuso y espera tu aprobación, o la está aplicando |
| `hecha` | Aplicada y revisada |
| `descartada` | No se hizo; el índice dice por qué |

Las notas se versionan en git con el cambio que las resolvió. Si quieres borrar una antes de que la
vea Claude, usa «Notas › Borrar» en el anotador. Tras editar un JSON a mano,
`node anotar.mjs --indice` regenera el índice.

## El mapa de la interfaz

[`docs/v1/ux/MAPA.md`](../v1/ux/MAPA.md) muestra cada pantalla con sus zonas numeradas y su nombre.
Úsalo para hablar de la interfaz también sin el anotador («en la Barra de acciones, después de
Captura…»). Los nombres salen de `scripts/ux/zonas.mjs`; cuando una función agrega o mueve una
zona, se actualiza ahí y se regenera el mapa con `node scripts/ux/mapa.mjs`.

## ¿Cuánto tarda revisar un cambio?

El recorrido completo de la consola (109 capturas con revisión de accesibilidad) tarda unos 12–15
minutos y **lo corre el CI en cada push**: no hace falta esperarlo en tu equipo. Para revisar un
cambio basta con las capturas de esa pantalla:

```powershell
node recorrido.mjs --solo=archivos          # las que tienen «archivos» en el nombre
node recorrido.mjs --solo=94-105,108        # por número
node recorrido-alumno.mjs --solo=archivo    # lo mismo en la app del alumno
```

Así, un cambio de texto, icono o estilo se revisa en uno o dos minutos. El recorrido completo se
corre en local solo cuando el cambio toca algo común a muchas pantallas (el sistema de diseño, el
menú, los avisos).

## Si algo no funciona

| Problema | Qué hacer |
|---|---|
| «Cannot find package 'puppeteer'» | `npm ci` en `scripts/ux` |
| Tarda mucho en abrir | La primera vez exporta el manual con Go (unos 30 s) |
| Abrí `http://127.0.0.1:<puerto>/profesor/?escena=lab&tema=ambos` (o `…/alumno/?escena=conectado&tema=ambos`) en mi navegador y sale en blanco o con un aviso; el puerto cambia cada vez que se abre el anotador | Esa dirección es interna: la app solo funciona en la ventana de Chromium que abre `anotar.mjs`, que le agrega los datos de demostración y la barra. Búscala en la barra de tareas |
| No se abre ninguna ventana | Mira los mensajes de la terminal: si dice «Anotador abierto», la ventana existe (puede estar detrás de otras); si hay un error, `npm ci` en `scripts/ux` y vuelve a intentar |
| La etiqueta dice «(zona sin nombre)» | Esa parte no está en el mapa todavía; anota igual y dilo en el texto |
| Lo que quieres comentar no sale en ninguna escena | Usa «Nota de la pantalla» y descríbelo; Claude puede agregar la escena |
