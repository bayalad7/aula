# Notas de interfaz · v0.1.0

Las escribe el anotador (`node scripts/ux/anotar.mjs`): cada una dice dónde (con los nombres de
`docs/ux/MAPA.md`), qué tipo de cambio y qué quieres, con una captura del elemento y de la
pantalla (con el tema «ambos», también en oscuro). Este índice se regenera solo; el detalle está en `NNN.json`. Estado: `pendiente`,
`en curso`, `hecha` o `descartada` (lo actualiza Claude al trabajarlas).

Al repositorio solo suben `NNN.json` y este índice. Las capturas (`*.webp`) se quedan en el equipo
donde se anotó (`.gitignore`): los enlaces de «Capturas» abren ahí. Todas las versiones:
[../LEEME.md](../LEEME.md).

| # | Estado | Dónde | Tipo | Qué quieres | Escena · tema | Capturas |
|---|---|---|---|---|---|---|
| 1 | hecha | Consola › Vista Código › Tarjeta del editor · «Enviar a Todos (4) Seleccionados (0) Emitir Agregar Limpiar… Pegar Escribe o peg» | estilo | El grupo de los botones de "Emitir" hasta "Pegar" muévelos a la parte de la derecha y que queden alineados en la misma fila donde esta "Enviar a" | lab · ambos | [elemento](001-elemento.webp) · [pantalla](001-pantalla.webp) · oscuro: [elemento](001-elemento-oscuro.webp) · [pantalla](001-pantalla-oscuro.webp) |
| 2 | hecha | Consola › Vista Archivos › Enviar archivos · «Enviar a Todos (4) Seleccionados (0) Enviar archivos Elige archivos o una planti» | estilo | El botón de "Enviar archivos" muévelo para que quede alineado en la misma fila donde esta "Enviar a" | lab · ambos | [elemento](002-elemento.webp) · [pantalla](002-pantalla.webp) · oscuro: [elemento](002-elemento-oscuro.webp) · [pantalla](002-pantalla-oscuro.webp) |
| 3 | hecha | Consola › (zona sin nombre) | error | El input de cambio de canal permite ingresar texto, solo debería ser puros números enteros del 1 al 16000 | lab · ambos | [elemento](003-elemento.webp) · [pantalla](003-pantalla.webp) · oscuro: [elemento](003-elemento-oscuro.webp) · [pantalla](003-pantalla-oscuro.webp) |
| 4 | hecha | Consola › Vista Alumnos › Miniaturas · «Esperando imagen… LAB3-PC07Conectado alumno07 Esperando imagen… LAB3-PC12Conecta» | agregar | Necesito una configuración donde pueda cambiar el ajuste del tamaño de las vistas en miniaturas para la grilla, algo como: Pequeño, Mediano, Grande, Extra grande, Mejor ajuste, para que las pantallas se puedan volver mas pequeñas o más grandes. | lab · ambos | [elemento](004-elemento.webp) · [pantalla](004-pantalla.webp) · oscuro: [elemento](004-elemento-oscuro.webp) · [pantalla](004-pantalla-oscuro.webp) |
| 5 | hecha | Consola › Vista Alumnos › Miniaturas › Tarjeta del alumno · «Esperando imagen… LAB3-PC12Conectado alumno12» | estilo | Quiero que la vista miniatura se represente en forma de un icono de monitor para darle un mejor diseño y estilo y que la pantalla pues sea la imagen del alumno | lab · ambos | [elemento](005-elemento.webp) · [pantalla](005-pantalla.webp) · oscuro: [elemento](005-elemento-oscuro.webp) · [pantalla](005-pantalla-oscuro.webp) |
| 6 | hecha | Consola › Vista Alumnos › Barra de acciones · «Sin selección: las acciones van a todos los conectados Mostrar mi pantalla Aviso» | mover | Este texto que controla si no hay alumnos seleccionados o el contador de cuantos hay seleccionados, muevelo en la parte de la cabecera aún lado de donde esta el contador de los conectados o desconectados, y que las leyendas de los textos no sean tan largos "simplifícalos" y agrégales tooltips como mensajes de ayuda si es necesario, esto servirá para la barra de acciones para que tenga más espacio para nuevas acciones | lab · ambos | [elemento](006-elemento.webp) · [pantalla](006-pantalla.webp) · oscuro: [elemento](006-elemento-oscuro.webp) · [pantalla](006-pantalla-oscuro.webp) |
