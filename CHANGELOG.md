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
- **Duplicado:** si dos conexiones dicen ser el mismo equipo (un equipo clonado o alguien
  haciéndose pasar por otro), las dos filas lo marcan, la ficha dice desde qué dirección se conecta
  cada una y la consola avisa una vez. Las calificaciones (columna «Aviso») y lo recogido lo
  registran; nada se descarta. La ficha muestra también la dirección de cada alumno.

### Código

- Escribir o pegar código y **Emitir** (reemplaza lo que ven), **Agregar** (debajo de lo que ven),
  **Limpiar…** y **Pegar** desde el portapapeles o con un atajo desde cualquier programa.
- A todos o a los seleccionados. Cada envío dice quién lo tiene y quién falta, con **Reenviar a
  los que faltan**, y queda en el historial de la clase para abrirlo o reemitirlo.
- En el alumno, el código llega con colores y un botón **Copiar**, sin quitarle el teclado.

### Ver y guiar la clase

- **Miniaturas** de las pantallas, cada una como un monitor y en el tamaño que elijas (o ajustadas
  para ver a todos), y una **Pantalla grande** de un alumno, con su aviso visible en el equipo del
  alumno. Arriba de la lista se ve a quién van las acciones: «Todos (n)» o «n seleccionados».
- **Mostrar mi pantalla:** a todos o a los seleccionados, en una ventana encima de las suyas o en
  **pantalla completa** para presentaciones; se libera sola si se pierde la conexión.
- **Proyectar a la clase** la pantalla de un alumno.
- **Captura** de la pantalla de un alumno, que solo se guarda si el profesor pulsa «Guardar».
- **Control remoto** de un alumno, con aviso antes y pausa si el alumno usa su equipo.
- **Salón:** cada equipo con una etiqueta que se entiende («Fila 2 · PC07») en lugar de su nombre en
  toda la consola, y el plano del salón (hasta 12 × 12, con escritorio y pasillos) para acomodarlos
  arrastrando o con el teclado. En Alumnos, la vista **Salón** muestra cada pantalla en su lugar y
  selecciona una fila o una columna con un clic. Se exporta e importa para copiarlo a las demás
  consolas del centro, y un equipo reinstalado con otro nombre conserva su lugar.

### Participación

- **Manos levantadas y mensajes** de los alumnos, en la vista «Participación». Los mensajes se
  ven como un chat: una conversación por alumno con lo que te escribió y lo que le respondiste (o le
  mandaste en privado), en una bandeja con **Por atender**, **Atendidas** y **Todas**, búsqueda por
  equipo o usuario y manejo con el teclado. Un mensaje que llega no mueve la lista ni borra lo que
  escribes.
- **Avisos** a la clase y **mensajes privados** a un alumno.
- **Temporizador** de ejercicio, con descripción de lo que hay que hacer y duraciones de 1 a 60
  minutos (u otra); extender y detener.
- **Encuestas en vivo:** opción múltiple, sí o no, o respuesta corta; anónimas o nominales, con
  resultados para compartir.
- **Cuestionarios con puntaje** (nuevo módulo «Cuestionarios»): varias preguntas con su respuesta
  correcta y sus puntos, guardados para reutilizarlos; tiempo opcional que lo cierra solo y orden
  al azar. Ves cuántos acertaron cada pregunta y el puntaje de cada alumno, compartes a cada uno
  **solo su** resultado y exportas las calificaciones a un CSV para Excel. El alumno responde
  pregunta por pregunta y entrega; la respuesta correcta no llega a su equipo antes del cierre.

### Archivos

- **Enviar archivos** sueltos o una **plantilla** a la carpeta `Documentos\Aula` de cada alumno,
  con quién los tiene y quién falta.
- **Plantillas** de ejercicio: crear, abrir su carpeta y eliminar.
- **Recolecciones:** recoger el trabajo de cada alumno de una carpeta acordada.
- **Abrir** una página o una app permitida en el equipo del alumno.

### Antes de clase

- **Laboratorio** (menú, grupo Pruebas): la salud de cada equipo antes de que lleguen los
  alumnos. Avisa con qué hacer cuando un equipo tiene el disco casi lleno o con fallas, poca
  memoria, el procesador saturado o frenado por calor, un reinicio pendiente, el reloj desfasado,
  sin internet, el cable de red lento, Wi‑Fi débil o teniendo cable, o la batería baja o sin
  corriente. **Revisar ahora** pide el estado al momento y el **Autodiagnóstico** prueba el disco,
  la red, los servicios de tu clase, el audio, el video, el teclado y el mouse. El menú cuenta los
  equipos con alertas, la lista de Alumnos los marca y la ficha muestra **Salud del equipo**. Los
  umbrales, la dirección para probar internet y los servicios se ajustan en **Ajustes › Salud**.
  Solo informa: nunca cambia nada del equipo, y nunca ve lo que hace el alumno.

### Al terminar la hora

- **Terminar la clase**, bajo la tarjeta de la clase en el menú: detiene en un paso lo que quedó
  activo (transmisión, proyección, control, pausa, restricciones, temporizador, encuesta) y deja la
  ventana de cada alumno como recién abierta, con la tarjeta «La clase terminó». Opcionalmente
  recoge el trabajo antes y cancela un apagado programado (por omisión se conserva). Muestra el
  resultado por alumno con «Reintentar a los que faltan»; quien estaba desconectado lo recibe al
  volver. No borra ningún archivo.

### Control del equipo (Windows)

- **Restricciones** de la clase: bloquear apps y filtrar sitios web, siempre reversibles. Si
  Windows no deja quitar el filtro a la primera, Aula lo reintenta cada 30 s y mientras tanto los
  sitios cargan: el alumno nunca se queda sin red.
- **Energía:** apagar, reiniciar o cerrar sesión con cuenta regresiva que el alumno ve y puede
  posponer.
- **Encender los equipos** desde la consola (Wake-on-LAN): **Energía › Encender** para todos los
  apagados o los seleccionados, o **Encender** en la fila de uno; la fila dice «Encendiendo…» hasta
  que el equipo se conecta, o «No respondió» con lo que hay que revisar. **Mostrar apagados** lista
  los equipos que la consola ya conoce aunque no estén conectados. Necesita Wake-on-LAN activado
  en cada equipo, cable de red y la consola en la misma red.
- **Pausar pantallas** («ojos al frente»): cubre las pantallas de todos, de los seleccionados o de
  un alumno, en todos sus monitores, con un aviso que dice quién las pausó y un mensaje opcional;
  **Reanudar todo** desde la franja o con **Ctrl+Mayús+P**. Tu transmisión, la cuenta de energía y
  el control remoto se ven encima, y la pausa se quita sola si se pierde la conexión.

### App del alumno

- No interrumpe: lo que llega se ve sin quitar el foco; el icono del área de notificación y el
  modo compacto.
- **Transparencia:** bandas fijas y sin botón de cerrar cuando el profesor ve, controla, pausa,
  restringe o transmite; el alumno sabe siempre quién y qué.
- **Qué puede hacer Aula en este equipo** desde la tira de arriba; **Levantar la mano**, **Mensaje
  al profesor** y el **tema** (del sistema, claro u oscuro, que se recuerda) en una barra de
  acciones abajo, como la de la consola. El motivo de la mano se escribe en su propio diálogo.
  **Tus mensajes** muestra la conversación con el profesor: lo que escribiste y sus respuestas, que
  se ocultan mientras tu pantalla se muestra a la clase.
  También dice el nombre del equipo y la etiqueta que le puso el profesor, para decírsela al soporte.
  Y que el equipo informa su estado (disco, memoria, red, batería) y se prueba cuando el profesor lo
  pide, nunca sus apps, sitios ni archivos.
- **Registros más privados:** cuando algo falla al guardar, recoger o enviar archivos, el registro
  dice qué tipo de error fue («acceso denegado», «disco lleno»), nunca la ruta ni el nombre del
  archivo; tampoco qué programa bloqueado se cerró.

### Consola

- Navegación por módulos (Alumnos, Código, Archivos, Participación), acciones sobre la selección
  desde la barra de acciones y **Ficha del alumno** con lo que puede hacer su equipo.
- **Manual y diagramas** dentro de la consola, con búsqueda y ayuda de cada pantalla.
- Modo claro y oscuro, teclado completo y contraste WCAG AA; funciona a 900×600.
- **Identidad nueva, Nodo:** el logo de Aula (cuatro pantallas unidas por un punto) en el icono de
  las dos apps y de la bandeja, y títulos en Plus Jakarta Sans, incluida en Aula para que se vea
  igual sin internet. El código que recibe el alumno se ve un poco más grande (14 px).
- **Menú en azul pizarra:** el menú de la consola se distingue del contenido (azul marino profundo
  en modo claro, casi negro en oscuro), y el icono de las dos apps toma ese mismo color de placa.
- Los diálogos, los paneles y los avisos aparecen con una transición breve en vez de golpe, en la
  consola y en la app del alumno; con «reducir movimiento» de Windows aparecen al instante.
- **Alumnos:** el tamaño de las miniaturas con su icono (y «Ajustar» en vez de «Ajustar a la
  pantalla»), **Apagados** como un botón que se resalta al activarlo, y las ayudas de la cabecera
  ya no se salen de la ventana. **Código** pasa a una columna cuando abres la ayuda a un lado.
- **Tablas más claras:** cada fila numerada («#»), el encabezado fijo al desplazarse y cada dato
  alineado según su tipo (texto a la izquierda, horas al centro, cantidades a la derecha). Una
  lista larga de alumnos ya se desplaza y la barra de acciones se queda abajo.
- **Alumnos:** la cabecera en una sola línea; la barra de acciones con botones iguales (icono arriba
  y nombre abajo) y **Encender** para los equipos marcados; una columna **Acciones** con iconos
  (ficha, Encender, Olvidar) y una casilla para marcar todos los apagados.
- El editor de cuestionarios se abre en su propio diálogo, con la columna **Correcta** separada de
  quitar la opción; **Salón** pone «Sin acomodar» y «Equipo elegido» arriba y el plano a todo lo
  ancho; **Participación** se ve en dos columnas.

### Para quien instala

- Instaladores para Windows; el agente del alumno se instala como **servicio** y muestra su
  ventana en la sesión del alumno.
- Las versiones van firmadas (`checksums.txt` con la llave de releases del repositorio).
- La consola no carga nada de Internet; compilar no necesita Node.
- **`aula-cliente diagnostico`** muestra también la salud del equipo y corre su autodiagnóstico.
- Para probar sin publicar, un solo comando (`scripts\compilar.ps1`) deja las apps y los
  instaladores en `dist\local`, compilados igual que una versión publicada.
- Componentes de red, cifrado, imagen y ventana al día con sus correcciones de seguridad (entre
  ellas, varias del servidor HTTP/2 y de WebKit en Linux); cada cambio del código se revisa contra
  la base de vulnerabilidades conocidas de Go antes de integrarse.
- Las apps se compilan con **Go 1.27**, con su último parche de seguridad; para compilarlas en
  tu equipo hace falta Go 1.27 o más reciente.

### Linux

- **Aula en Linux** (Ubuntu 24.04 y parecidas): paquetes `.deb` y `.rpm` de la consola y del
  agente, y la consola también como **AppImage**, sin instalar. El agente arranca con cada sesión
  y se configura con `sudo aula-cliente configurar -canal 1 -clave aula-centro.pub`.
- El agente de Linux queda como **servicio** (`aula-agente`): arranca con el equipo, también sin
  nadie conectado, y abre la ventana de Aula en la sesión del alumno, como en Windows.
- En Linux funcionan el código y **Copiar**, avisos, mensajes, manos, temporizador, encuestas y
  cuestionarios, enviar y recoger archivos (en `~/Documentos/Aula`), terminar la clase y el
  atajo para traer código a la consola.
- **Ver la pantalla del alumno**, la captura, **tomar el control** y **pausar pantallas**, con
  los mismos avisos que en Windows, en la sesión **«Ubuntu en Xorg»**. En la sesión Wayland, Aula
  no ve ni controla la pantalla y lo dice: la ficha lo muestra como no disponible y
  `aula-cliente diagnostico` explica qué hacer.
- **Apagar, reiniciar y cerrar sesión**, **encender equipos** y la **salud del laboratorio** con
  su autodiagnóstico también en Linux (sin la revisión S.M.A.R.T. del disco). Cerrar sesión sin
  «forzar» deja que los programas pidan guardar en GNOME, Cinnamon y MATE, como en Windows.
- **Abrir una página o una app** también en Linux. En Ajustes › Apps permitidas, cada app suma
  su nombre en Linux (por ejemplo `org.thonny.Thonny.desktop`); una misma lista sirve a un
  laboratorio con Windows y Linux.
- En Linux, cerrar la ventana del alumno la deja en el **área de notificación**, con su icono y
  el menú «Abrir Aula», y el icono se marca cuando llega algo, como en Windows. Donde el
  escritorio no muestra esos iconos, cerrar sigue minimizando.
- **Mostrar tu pantalla** desde una consola Linux y **proyectar** la de un alumno Linux a la
  clase (sesión «Ubuntu en Xorg», con openh264). En Linux la ventana de Aula también se ve en lo
  que se transmite, y la franja lo dice. Un alumno Linux todavía no puede ver transmisiones.
- **Bloquear apps y sitios** también en Linux, con los mismos avisos: nunca se cierra el
  escritorio y reiniciar el equipo siempre quita el filtro de sitios.
- **Comprobar la conexión** lee el firewall de una consola Linux (`firewalld`; con `ufw`, sin
  permisos de administrador lo deja sin juzgar).
- La clave del centro de una consola Linux la leen los maestros del grupo `aula`; el paquete
  abre el firewall (`ufw` o `firewalld`) si está activo. Lo que falta en Linux está en la tabla
  de compatibilidad (`docs/compatibilidad.md`).

### Todavía no

- La asistencia y bitácora, la auto-actualización, macOS y, en Linux, ver las transmisiones en
  el equipo del alumno.
