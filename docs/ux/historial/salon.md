# Etiquetas de los equipos y salón (10.15): ubicación y decisiones

Estado: **implementado, sin commit** (13/11/2026). Prompt
`docs/prompts/10.15-nombres-y-mapa-del-aula.md`, trabajado con `/funcionalidad-nueva 10.15`. No se
versionan capturas: las del arnés son `scripts/ux/salida/203-213` (consola) y
`scripts/ux/salida-alumno/90-91` (alumno), y el CI las sube. El mapa suma «Consola · Salón» y
«Consola · Alumnos (salón)» (`docs/ux/mapa/`), y el manual la página `salon.md` con su captura.

## Análisis

Con 30 a 40 equipos, «LAB3-PC07» no dice dónde está la máquina. El prompt pedía dos cosas: una
etiqueta por equipo, que se viera en toda la consola, y un plano del salón que ordenara la lista y
las miniaturas. Eso traía tres preguntas de interfaz:

- **El plano es material propio**: se arma una vez, se guarda y se copia a otras consolas. Según la
  regla de «Ubicación y vocabulario», eso pide un módulo propio del menú.
- **Ver la clase como está sentada** es otra forma de ver a los alumnos, no una pantalla nueva:
  cabe junto a Lista y Miniaturas.
- **Cambiar la etiqueta de un equipo** es una acción sobre un alumno. La Ficha solo informa
  (nota de interfaz 5), así que va en la Pantalla grande con su propio diálogo.

El prompt decía que la función no tocaba el protocolo, pero pedía que el alumno viera su etiqueta.
La etiqueta vive en la consola, así que hacía falta un mensaje: `computer.label` (§5.26),
compatible.

## Opciones de ubicación presentadas

1. **Módulo «Salón» + Ajustes › Salón** (recomendada, **elegida**): el editor en su módulo, después
   de Alumnos; en Ajustes, el resumen con Exportar e Importar, que son tareas del administrador.
2. **Modo «Acomodar» dentro de Alumnos › Salón**: sin módulo nuevo. Mezcla editar el plano con dar
   la clase y rompe la regla del material propio.
3. **Solo Ajustes › Salón**: todo lo configurado junto, pero un plano de 12 × 12 no cabe en el modal
   a 900×600.

Para la etiqueta: **Pantalla grande con diálogo** (elegida) o solo el editor del plano. Para el
alumno: **hostname y etiqueta** con el mensaje nuevo (elegida) o solo el hostname sin tocar el
protocolo. Para un equipo reinstalado con otro nombre y la misma MAC: **conservar su lugar solo,
con aviso** (elegida), preguntar al profesor o tratarlo como nuevo.

## Decisiones

- **Una sola función del nombre visible.** `nombreEquipo(x)` en `main.js` devuelve la etiqueta o el
  hostname, y `hostSecundario(x)` el hostname cuando hay etiqueta. Los ~70 lugares de la consola
  que mostraban `s.hostname` pasan por ella, y las listas por `listaEquipos`. La identidad
  (`clave`, `data-equipo`, lo que va a Go) sigue siendo el hostname.
- **La clave del equipo es el hostname sin mayúsculas, con las MAC de respaldo.** El agente no tiene
  identidad propia, y un ID nuevo tampoco sobreviviría a una reinstalación (el `MachineGuid` se
  repite en imágenes clonadas). Solo se renombra si la MAC es de un único equipo del salón y ese
  equipo no está conectado.
- **La vista Salón reusa la tarjeta de Miniaturas**, con su imagen, su selección y su clic a la
  Pantalla grande, colocada con `grid-row`/`grid-column`. Las cabeceras de fila y columna son
  botones que seleccionan a los conectados de esa fila o columna (para la Barra de acciones y
  10.16). Un asiento es más angosto que una miniatura (`ANCHO_ASIENTO`), así un salón de 8 columnas
  cabe en «Pequeño» a 1280 px; a 900×600 se desplaza dentro de su recuadro con la cabecera de las
  filas fija. En un asiento, el nombre va en su propia línea para que la etiqueta no se corte.
- **El editor es una rejilla de botones con foco itinerante** (no `role=grid`): Tab entra una vez,
  las flechas recorren y Enter toma y deja. El nombre accesible de cada botón es su texto visible,
  y la posición y el estado van en un `solo-lector` (axe `label-content-name-mismatch`).
- **El frente del salón** (pizarrón) se dibuja arriba en el editor y en la vista, para que «fila 1»
  tenga sentido.
- **Los simulados ocupan los primeros lugares libres**, solo en memoria; un equipo real les gana el
  lugar.
- **Lo que queda fuera:** la etiqueta en el CSV de calificaciones (tocaría `internal/poll`; el CSV
  conserva el hostname, que es estable), la selección por zona libre (10.16) y el nombre del alumno
  (10.13).

## Resultados

- `go test ./internal/roomplan ./apps/teacher ./apps/student ./internal/protocol`: versión,
  validación, lugares repetidos, equipo desconocido, importar y exportar, renombrado por MAC,
  simulados nunca en el archivo, archivo dañado o de una versión nueva, y la etiqueta solo a quien
  declara `computer.label`.
- Arnés: escenas 203–213 sin errores de JS ni violaciones serias de axe (en claro, oscuro y a
  900×600, con 30 simulados acomodados); `pruebas-alumno.mjs` 65/65 con «Este equipo».
