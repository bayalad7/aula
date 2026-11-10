# Mapa de la interfaz

Los nombres de cada zona de las dos apps, para hablar de la interfaz sin ambigüedad: «en la
**Ficha del alumno**, en **Qué puede hacer su equipo**…». Son los mismos que muestra el anotador
(`node scripts/ux/anotar.mjs`) al pasar el mouse. La fuente es `scripts/ux/zonas.mjs`; este
archivo y sus imágenes los genera `node scripts/ux/mapa.mjs` (no se editan a mano).


## Consola del profesor

### Consola · Alumnos (lista)

![Consola · Alumnos (lista)](mapa/consola-alumnos.webp)

| # | Zona | Qué es |
|---|---|---|
| 1 | **Menú** | Columna izquierda: la clase abierta, las vistas y Ayuda y Ajustes. |
| 2 | **Selector de clase** | Canal y nombre de la clase abierta; abre «Cambiar de canal». |
| 3 | **Cabecera de la vista** | Título de la vista, su resumen y el «?» de ayuda contextual. |
| 4 | **Selector Lista | Miniaturas** | Cambia entre la tabla y las miniaturas de pantalla. |
| 5 | **Lista de alumnos** | Tabla con un alumno por fila. |
| 6 | **Fila del alumno** | Un alumno: casilla, equipo, estado, insignias y el botón de su ficha. |
| 7 | **Barra de acciones** | Abajo: lo que se aplica a la selección (aviso, transmitir, restricciones, captura, temporizador, energía…). |

### Consola · Alumnos (miniaturas)

![Consola · Alumnos (miniaturas)](mapa/consola-miniaturas.webp)

| # | Zona | Qué es |
|---|---|---|
| 1 | **Miniaturas** | Cuadrícula con la pantalla de cada alumno. |
| 2 | **Tarjeta del alumno** | La miniatura de un alumno con su nombre; un clic abre la Pantalla grande. |
| 3 | **Barra de acciones** | Abajo: lo que se aplica a la selección (aviso, transmitir, restricciones, captura, temporizador, energía…). |

### Consola · Ficha del alumno

![Consola · Ficha del alumno](mapa/consola-ficha.webp)

| # | Zona | Qué es |
|---|---|---|
| 1 | **Ficha del alumno** | Panel derecho con los datos de un alumno y sus acciones. |
| 2 | **Acciones sobre el alumno** | Tomar control, Mensaje privado, Tomar captura, Abrir página o app: las mismas que en la Pantalla grande (ACCIONES_ALUMNO en main.js). |
| 3 | **Qué puede hacer su equipo** | Lista de capacidades del equipo del alumno. |

### Consola · Pantalla grande

![Consola · Pantalla grande](mapa/consola-pantalla-grande.webp)

| # | Zona | Qué es |
|---|---|---|
| 1 | **Pantalla grande** | La pantalla de un alumno en grande; también la vista del control remoto. |
| 2 | **Acciones sobre el alumno (Pantalla grande)** | Las mismas acciones que en la Ficha del alumno. |

### Consola · Franja global y un panel de la barra

![Consola · Franja global y un panel de la barra](mapa/consola-franja-panel.webp)

| # | Zona | Qué es |
|---|---|---|
| 1 | **Franja global** | Lo que sigue activo durante la clase (transmitir, control, energía, temporizador, aula simulada), con su botón para detenerlo. |
| 2 | **Panel Aviso** | Mandar un aviso a la selección. |
| 3 | **Barra de acciones** | Abajo: lo que se aplica a la selección (aviso, transmitir, restricciones, captura, temporizador, energía…). |

### Consola · Abrir página o app

![Consola · Abrir página o app](mapa/consola-abrir.webp)

| # | Zona | Qué es |
|---|---|---|
| 1 | **Panel Abrir página o app** | Abrir una página en el navegador o una app permitida en los equipos de la selección, y las últimas que abriste. |
| 2 | **Barra de acciones** | Abajo: lo que se aplica a la selección (aviso, transmitir, restricciones, captura, temporizador, energía…). |

### Consola · Ajustes › Apps permitidas

![Consola · Ajustes › Apps permitidas](mapa/consola-ajustes-apps.webp)

| # | Zona | Qué es |
|---|---|---|
| 1 | **Ajustes** | Diálogo con Clase, Apariencia, Apps permitidas y Acerca de. |
| 2 | **Apps permitidas** | Las apps que se pueden abrir en los equipos: nombre y ruta del .exe, con ejemplos. |

### Consola · Código

![Consola · Código](mapa/consola-codigo.webp)

| # | Zona | Qué es |
|---|---|---|
| 1 | **Tarjeta del editor** | Título, lenguaje, editor, «Enviar a» y los botones de emitir. |
| 2 | **Editor** | El editor de código (Monaco). |
| 3 | **Enviar a** | Todos los conectados o la selección. |
| 4 | **Últimos envíos** | Cada envío con quién lo tiene y quién falta. |
| 5 | **Historial de la clase** | Lo emitido en la clase, para reabrir o reemitir. |

### Consola · Archivos

![Consola · Archivos](mapa/consola-archivos.webp)

| # | Zona | Qué es |
|---|---|---|
| 1 | **Menú** | Columna izquierda: la clase abierta, las vistas y Ayuda y Ajustes. |
| 2 | **Enviar archivos** | Plantilla o archivos elegidos, «Guardar en», «Reemplazar», «Enviar a» y Enviar. |
| 3 | **Envíos de archivos** | Cada envío de archivos con quién lo tiene y quién falta. |
| 4 | **Plantillas** | Las plantillas de Documentos\Aula\plantillas con Usar y Abrir carpeta. |
| 5 | **Archivos enviados** | Los envíos de la clase, para reenviarlos. |

### Consola · Archivos › Recolecciones

![Consola · Archivos › Recolecciones](mapa/consola-recolecciones.webp)

| # | Zona | Qué es |
|---|---|---|
| 1 | **Selector Enviar | Recolecciones** | Cambia entre enviar archivos y las recolecciones. |
| 2 | **Nueva recolección** | Actividad, tipos de archivo y «Crear carpeta en los equipos». |
| 3 | **Tarjeta de recolección** | Una actividad: Recoger, Recoger a los que faltan, Abrir carpeta y la tabla por alumno. |
| 4 | **Fila de la recolección** | Un alumno: estado, qué llegó, cuándo, Abrir, Guardar como… y Recoger otra vez. |

### Consola · Participación

![Consola · Participación](mapa/consola-participacion.webp)

| # | Zona | Qué es |
|---|---|---|
| 1 | **Cabecera de la vista** | Título de la vista, su resumen y el «?» de ayuda contextual. |
| 2 | **Manos levantadas** | La fila de manos, por orden de llegada. |
| 3 | **Mensajes de los alumnos** | La bandeja de mensajes con Responder. |
| 4 | **Sección Temporizador** | Quién ve el temporizador y quién falta. |


## App del alumno

### Alumno · Tira de identidad y bandas

![Alumno · Tira de identidad y bandas](mapa/alumno-bandas.webp)

| # | Zona | Qué es |
|---|---|---|
| 1 | **Tira de identidad** | Arriba: a qué clase está conectado, el temporizador y los botones del alumno. |
| 2 | **Botones de la tira** | Levantar la mano, Mensaje al profesor y Modo compacto. |
| 3 | **Bandas de transparencia** | Lo que el profesor hace con el equipo (pantalla vista, control, restricciones, energía); no se cierran. |
| 4 | **Estado vacío** | Lo que se ve cuando no hay código ni archivos. |

### Alumno · Código recibido

![Alumno · Código recibido](mapa/alumno-codigo.webp)

| # | Zona | Qué es |
|---|---|---|
| 1 | **Tira de identidad** | Arriba: a qué clase está conectado, el temporizador y los botones del alumno. |
| 2 | **Lista de código** | El código que mandó el profesor, lo más nuevo arriba. |
| 3 | **Tarjeta de código** | Un fragmento con su ejercicio, título, lenguaje y Copiar. |

### Alumno · Archivos y mensajes

![Alumno · Archivos y mensajes](mapa/alumno-archivos.webp)

| # | Zona | Qué es |
|---|---|---|
| 1 | **Tira de identidad** | Arriba: a qué clase está conectado, el temporizador y los botones del alumno. |
| 2 | **Mensaje al profesor** | La tarjeta para escribirle al profesor. |
| 3 | **Archivos recibidos** | Una tarjeta por envío de archivos. |
| 4 | **Tarjeta de archivo** | Qué llegó, dónde quedó y Abrir carpeta. |

### Alumno · Recolección

![Alumno · Recolección](mapa/alumno-recolecciones.webp)

| # | Zona | Qué es |
|---|---|---|
| 1 | **Tira de identidad** | Arriba: a qué clase está conectado, el temporizador y los botones del alumno. |
| 2 | **Tarjeta de recolección** | Dónde guardar el ejercicio, qué se recogió y cuándo, y Abrir carpeta. |
| 3 | **Avisos en la esquina** | Avisos del profesor y de Aula, abajo a la derecha. |

### Alumno · Mano levantada y avisos

![Alumno · Mano levantada y avisos](mapa/alumno-avisos.webp)

| # | Zona | Qué es |
|---|---|---|
| 1 | **Tira de identidad** | Arriba: a qué clase está conectado, el temporizador y los botones del alumno. |
| 2 | **Estado de la mano** | «Tu mano está levantada» y el motivo. |
| 3 | **Avisos en la esquina** | Avisos del profesor y de Aula, abajo a la derecha. |

### Alumno · Modo compacto

![Alumno · Modo compacto](mapa/alumno-compacto.webp)

| # | Zona | Qué es |
|---|---|---|
| 1 | **Modo compacto** | La ventana encogida a una tira. |


## Todas las zonas

También las que no salen en una imagen (diálogos y paneles). El `id` es el que guardan las notas.

### Consola

| id | Zona | Qué es |
|---|---|---|
| `menu` | Menú | Columna izquierda: la clase abierta, las vistas y Ayuda y Ajustes. |
| `menu-clase` | Menú › Selector de clase | Canal y nombre de la clase abierta; abre «Cambiar de canal». |
| `franja` | Franja global | Lo que sigue activo durante la clase (transmitir, control, energía, temporizador, aula simulada), con su botón para detenerlo. |
| `cabecera` | Cabecera de la vista | Título de la vista, su resumen y el «?» de ayuda contextual. |
| `avisos` | Avisos (toasts) | Mensajes breves abajo a la derecha tras una acción. |
| `alumnos` | Vista Alumnos | Los alumnos del canal, en lista o miniaturas. |
| `alumnos-modo` | Vista Alumnos › Selector Lista | Miniaturas | Cambia entre la tabla y las miniaturas de pantalla. |
| `alumnos-lista` | Vista Alumnos › Lista de alumnos | Tabla con un alumno por fila. |
| `fila-alumno` | Vista Alumnos › Lista de alumnos › Fila del alumno | Un alumno: casilla, equipo, estado, insignias y el botón de su ficha. |
| `alumnos-miniaturas` | Vista Alumnos › Miniaturas | Cuadrícula con la pantalla de cada alumno. |
| `tarjeta-alumno` | Vista Alumnos › Miniaturas › Tarjeta del alumno | La miniatura de un alumno con su nombre; un clic abre la Pantalla grande. |
| `barra-acciones` | Vista Alumnos › Barra de acciones | Abajo: lo que se aplica a la selección (aviso, transmitir, restricciones, captura, temporizador, energía…). |
| `ficha` | Ficha del alumno | Panel derecho con los datos de un alumno y sus acciones. |
| `ficha-acciones` | Ficha del alumno › Acciones sobre el alumno | Tomar control, Mensaje privado, Tomar captura, Abrir página o app: las mismas que en la Pantalla grande (ACCIONES_ALUMNO en main.js). |
| `ficha-puede` | Ficha del alumno › Qué puede hacer su equipo | Lista de capacidades del equipo del alumno. |
| `pantalla-grande` | Pantalla grande | La pantalla de un alumno en grande; también la vista del control remoto. |
| `pg-acciones` | Pantalla grande › Acciones sobre el alumno (Pantalla grande) | Las mismas acciones que en la Ficha del alumno. |
| `panel-aviso` | Panel Aviso | Mandar un aviso a la selección. |
| `panel-pantalla` | Panel Transmitir | Transmitir tu pantalla a la clase. |
| `panel-restricciones` | Panel Restricciones | Bloquear aplicaciones y filtrar sitios. |
| `panel-captura` | Panel Captura | Tomar capturas de varios alumnos. |
| `panel-temporizador` | Panel Temporizador | Lanzar un temporizador de ejercicio. |
| `panel-energia` | Panel Energía | Apagar, reiniciar o cerrar sesión. |
| `panel-abrir` | Panel Abrir página o app | Abrir una página en el navegador o una app permitida en los equipos de la selección, y las últimas que abriste. |
| `captura` | Vista previa de captura | La captura de un alumno, con Guardar y Descartar. |
| `codigo` | Vista Código | Preparar y enviar código a los alumnos. |
| `codigo-editor` | Vista Código › Tarjeta del editor | Título, lenguaje, editor, «Enviar a» y los botones de emitir. |
| `editor` | Vista Código › Tarjeta del editor › Editor | El editor de código (Monaco). |
| `enviar-a` | Vista Código › Tarjeta del editor › Enviar a | Todos los conectados o la selección. |
| `envios` | Vista Código › Últimos envíos | Cada envío con quién lo tiene y quién falta. |
| `historial` | Vista Código › Historial de la clase | Lo emitido en la clase, para reabrir o reemitir. |
| `archivos` | Vista Archivos | Enviar archivos y plantillas a los alumnos, y recoger su trabajo. |
| `archivos-pestanas` | Vista Archivos › Selector Enviar | Recolecciones | Cambia entre enviar archivos y las recolecciones. |
| `archivos-enviar` | Vista Archivos › Enviar archivos | Plantilla o archivos elegidos, «Guardar en», «Reemplazar», «Enviar a» y Enviar. |
| `archivos-envios` | Vista Archivos › Envíos de archivos | Cada envío de archivos con quién lo tiene y quién falta. |
| `plantillas` | Vista Archivos › Plantillas | Las plantillas de Documentos\Aula\plantillas con Usar y Abrir carpeta. |
| `archivos-historial` | Vista Archivos › Archivos enviados | Los envíos de la clase, para reenviarlos. |
| `recolecciones` | Vista Archivos › Recolecciones | Crear la carpeta de una actividad en los equipos y recoger lo que guardó cada alumno. |
| `reco-nueva` | Vista Archivos › Recolecciones › Nueva recolección | Actividad, tipos de archivo y «Crear carpeta en los equipos». |
| `recoleccion` | Vista Archivos › Recolecciones › Tarjeta de recolección | Una actividad: Recoger, Recoger a los que faltan, Abrir carpeta y la tabla por alumno. |
| `reco-fila` | Vista Archivos › Recolecciones › Tarjeta de recolección › Fila de la recolección | Un alumno: estado, qué llegó, cuándo, Abrir, Guardar como… y Recoger otra vez. |
| `participacion` | Vista Participación | Lo que responden los alumnos en vivo. |
| `manos` | Vista Participación › Manos levantadas | La fila de manos, por orden de llegada. |
| `mensajes` | Vista Participación › Mensajes de los alumnos | La bandeja de mensajes con Responder. |
| `part-temporizador` | Vista Participación › Sección Temporizador | Quién ve el temporizador y quién falta. |
| `simulada` | Vista Aula simulada | Alumnos de prueba y sus fallas. |
| `ayuda` | Vista Ayuda (manual) | El manual, con búsqueda e índice. |
| `panel-ayuda` | Panel de ayuda contextual | La página del manual de la vista actual, al costado. |
| `falta` | Vista ¿Falta un alumno? | Comprobar la conexión y el asistente paso a paso. |
| `ajustes` | Ajustes | Diálogo con Clase, Apariencia, Apps permitidas y Acerca de. |
| `ajustes-apps` | Ajustes › Apps permitidas | Las apps que se pueden abrir en los equipos: nombre y ruta del .exe, con ejemplos. |
| `confirmar` | Diálogo de confirmación | «¿Seguro?» antes de una acción con consecuencias. |
| `arranque` | Pantalla de arranque | Abrir la clase: canal y nombre. |

### Alumno

| id | Zona | Qué es |
|---|---|---|
| `tira` | Tira de identidad | Arriba: a qué clase está conectado, el temporizador y los botones del alumno. |
| `tira-botones` | Tira de identidad › Botones de la tira | Levantar la mano, Mensaje al profesor y Modo compacto. |
| `tira-temporizador` | Tira de identidad › Insignia del temporizador | Cuánto falta del ejercicio. |
| `bandas` | Bandas de transparencia | Lo que el profesor hace con el equipo (pantalla vista, control, restricciones, energía); no se cierran. |
| `mano-estado` | Estado de la mano | «Tu mano está levantada» y el motivo. |
| `redactar` | Mensaje al profesor | La tarjeta para escribirle al profesor. |
| `enviados` | Tus mensajes | Los mensajes enviados y si llegaron. |
| `vacio` | Estado vacío | Lo que se ve cuando no hay código ni archivos. |
| `recolecciones` | Recolecciones | Una tarjeta por recolección del profesor. |
| `tarjeta-recoleccion` | Recolecciones › Tarjeta de recolección | Dónde guardar el ejercicio, qué se recogió y cuándo, y Abrir carpeta. |
| `archivos` | Archivos recibidos | Una tarjeta por envío de archivos. |
| `tarjeta-archivo` | Archivos recibidos › Tarjeta de archivo | Qué llegó, dónde quedó y Abrir carpeta. |
| `codigos` | Lista de código | El código que mandó el profesor, lo más nuevo arriba. |
| `tarjeta-codigo` | Lista de código › Tarjeta de código | Un fragmento con su ejercicio, título, lenguaje y Copiar. |
| `avisos` | Avisos en la esquina | Avisos del profesor y de Aula, abajo a la derecha. |
| `compacto` | Modo compacto | La ventana encogida a una tira. |
| `transmision` | Vista de la transmisión | La pantalla del profesor ocupando la ventana. |
