# Cambios

Los cambios de Aula que nota quien la usa o la instala. Versionado semántico (`x.y.z`, ver
`CLAUDE.md`). Para el detalle técnico, el historial de git y `docs/progreso.md`.

Cada función o cambio visible agrega su línea en «Sin publicar» al terminarse. Al publicar, esa
sección pasa a llamarse con el número y la fecha de la versión (`docs/desarrollo/publicar.md`).

## Sin publicar (v0.1.0, la primera versión de desarrollo)

Aula para el centro de cómputo: el profesor ve y guía a su grupo desde una consola, y cada alumno
recibe el código formateado para copiarlo con un clic. Es transparente: el alumno siempre ve qué
está viendo o haciendo el profesor en su equipo. Las versiones de prueba anteriores (`-rc`) se
retiraron; esta es la primera.

### Clase y conexión

- **Canales del 1 al 16000**, como en un centro de cómputo: cada equipo de alumno tiene su canal
  fijo y la consola abre el suyo; varias salas conviven en la misma red.
- Los alumnos encuentran solos a su profesor en la red local y se conectan de forma segura (TLS),
  solo a una consola con la **clave del centro**.
- **¿Falta un alumno?**: un asistente paso a paso y **Comprobar la conexión**, que dice dónde se
  atoraría un alumno (anuncio, puerto, conexión segura, red y firewall).
- **Aula simulada:** alumnos de prueba dentro de la consola para probarla sin laboratorio, con
  fallas a propósito. Siempre marcados y contados aparte; nunca reciben nada que cambie un equipo.

### Código

- Escribir o pegar código y **Emitir** (reemplaza lo que ven), **Agregar** (debajo de lo que ven),
  **Limpiar…** y **Pegar** desde el portapapeles o con un atajo desde cualquier programa.
- A todos o a los seleccionados. Cada envío dice quién lo tiene y quién falta, con **Reenviar a
  los que faltan**, y queda en el historial de la clase para abrirlo o reemitirlo.
- En el alumno, el código llega con colores y un botón **Copiar**, sin quitarle el teclado.

### Ver y guiar la clase

- **Miniaturas** de las pantallas y una **Pantalla grande** de un alumno, con su aviso visible en
  el equipo del alumno.
- **Mostrar mi pantalla:** a todos o a los seleccionados, en una ventana encima de las suyas o en
  **pantalla completa** para presentaciones; se libera sola si se pierde la conexión.
- **Proyectar a la clase** la pantalla de un alumno.
- **Captura** de la pantalla de un alumno, que solo se guarda si el profesor pulsa «Guardar».
- **Control remoto** de un alumno, con aviso antes y pausa si el alumno usa su equipo.

### Participación

- **Manos levantadas y mensajes** de los alumnos, en la vista «Participación».
- **Avisos** a la clase y **mensajes privados** a un alumno.
- **Temporizador** de ejercicio, con descripción de lo que hay que hacer y duraciones de 1 a 60
  minutos (u otra); extender y detener.
- **Encuestas en vivo:** opción múltiple, sí o no, o respuesta corta; anónimas o nominales, con
  resultados para compartir.

### Archivos

- **Enviar archivos** sueltos o una **plantilla** a la carpeta `Documentos\Aula` de cada alumno,
  con quién los tiene y quién falta.
- **Plantillas** de ejercicio: crear, abrir su carpeta y eliminar.
- **Recolecciones:** recoger el trabajo de cada alumno de una carpeta acordada.
- **Abrir** una página o una app permitida en el equipo del alumno.

### Control del equipo (Windows)

- **Restricciones** de la clase: bloquear apps y filtrar sitios web, siempre reversibles.
- **Energía:** apagar, reiniciar o cerrar sesión con cuenta regresiva que el alumno ve y puede
  posponer.
- **Pausar pantallas** («ojos al frente»): cubre las pantallas de todos, de los seleccionados o de
  un alumno, en todos sus monitores, con un aviso que dice quién las pausó y un mensaje opcional;
  **Reanudar todo** desde la franja o con **Ctrl+Mayús+P**. Tu transmisión, la cuenta de energía y
  el control remoto se ven encima, y la pausa se quita sola si se pierde la conexión.

### App del alumno

- No interrumpe: lo que llega se ve sin quitar el foco; el icono del área de notificación y el
  modo compacto.
- **Transparencia:** bandas fijas y sin botón de cerrar cuando el profesor ve, controla, pausa,
  restringe o transmite; el alumno sabe siempre quién y qué.
- **Qué puede hacer Aula en este equipo**, levantar la mano y escribir al profesor desde la tira.

### Consola

- Navegación por módulos (Alumnos, Código, Archivos, Participación), acciones sobre la selección
  desde la barra de acciones y **Ficha del alumno** con lo que puede hacer su equipo.
- **Manual y diagramas** dentro de la consola, con búsqueda y ayuda de cada pantalla.
- Modo claro y oscuro, teclado completo y contraste WCAG AA; funciona a 900×600.

### Para quien instala

- Instaladores para Windows; el agente del alumno se instala como **servicio** y muestra su
  ventana en la sesión del alumno.
- Las versiones van firmadas (`checksums.txt` con la llave de releases del repositorio).
- La consola no carga nada de Internet; compilar no necesita Node.

### Todavía no

- La pantalla en blanco de la clase, la asistencia y bitácora, Linux y macOS en las funciones que
  tocan el sistema, y la auto-actualización.
