---
titulo: "Tareas frecuentes"
resumen: "Mostrar tu pantalla o la de un alumno, mandar avisos, pausar las pantallas, dar tiempo a un ejercicio, restringir aplicaciones y sitios, apagar equipos, tomar el control de un equipo, tomar una captura de su pantalla y cambiar de canal."
grupo: "Tareas frecuentes"
orden: 60
pantallas: []
---

# Tareas frecuentes

Todas estas acciones están en la barra de abajo de [Alumnos](alumnos#acciones-sobre-la-selección).
Sin selección van a todos los conectados; con selección, solo a los elegidos.

## Mostrar tu pantalla

1. Si es solo para algunos, selecciónalos en Alumnos; sin selección, la ven todos los conectados
   y los que se conecten después.
2. Pulsa **Mostrar mi pantalla**.
3. Elige el monitor y el **Modo**:
   - **En una ventana:** queda encima de sus ventanas. La pueden mover y cambiar de tamaño (para
     copiar lo que haces en su editor), pero no cerrar ni minimizar.
   - **Pantalla completa:** cubre toda su pantalla y no la pueden cerrar ni minimizar. Úsala para
     presentaciones (PowerPoint, Canva…).
4. Pulsa **Empezar**.

Tus alumnos ven tu pantalla con tu nombre arriba. Una franja roja en tu consola te recuerda que
estás transmitiendo, en qué modo y a cuántos; la ventana de Aula no aparece en la transmisión.
Para terminar, pulsa **Detener** en la franja: la transmisión termina en todos y la ventana de
cada alumno vuelve a como estaba, sin tocar sus programas. Por ahora, solo equipos con Windows
pueden recibirla.

Nadie queda bloqueado: si un equipo pierde la conexión con tu consola, o tu consola deja de
responder, su pantalla completa se quita sola en unos segundos. También termina al cerrar la
clase o cambiar de canal. Si ese alumno se reconecta mientras sigues transmitiendo, vuelve a
recibirla en el mismo modo.

{{diagrama:14-transmision-pantalla}}

## Proyectar la pantalla de un alumno

Para mostrar al grupo el trabajo de alguien: su pantalla se ve en grande en los equipos de sus
compañeros, como cuando muestras la tuya.

1. En [Alumnos](alumnos#acciones-sobre-un-alumno) › **Miniaturas**, haz clic en su pantalla para
   verla en grande.
2. Pulsa **Proyectar a la clase**.

No se le pregunta al alumno: tú decides cuándo empieza y cuándo termina, como al tomar el control.
La franja de arriba dice «Proyectando LAB3-PC07 · conectando…» y en segundos «Proyectando
LAB3-PC07 · 3 de 12 viendo», con **Ver** y **Detener**:

- **Ver** abre su pantalla en grande en tu consola: es lo mismo que ven sus compañeros. Cerrar esa
  vista no detiene la proyección.
- **Detener** (en la franja o en su pantalla en grande) la termina en todos los
  equipos.

- **El alumno siempre lo sabe:** su ventana de Aula pasa al frente con la banda «Tu pantalla se
  está mostrando a la clase» antes de que se vea nada, y un aviso fijo arriba de su pantalla dice lo
  mismo, aunque cierre la ventana. No puede cerrar la banda ni detener la proyección: solo tú.
- **Lo privado no se ve:** la ventana de Aula del alumno no aparece en la proyección (un mensaje
  privado o una encuesta suya no llegan a sus compañeros). Se ve su monitor principal.
- **Uno a la vez,** y no junto con tu pantalla: si estás mostrando la tuya, Aula te pregunta si
  quieres terminarla para proyectar la del alumno, y al revés.
- **Termina sola** si el alumno se desconecta, si cambias de canal o si cierras la clase.
- **Sus compañeros** ven «Estás viendo la pantalla de LAB3-PC07». Los equipos con una versión de
  Aula anterior no la reciben; su ficha dice «Ver la pantalla de un compañero: no».

Solo equipos con Windows y una versión de Aula que lo incluya (su ficha dice «Mostrar su pantalla a
la clase: sí»); los alumnos simulados no se pueden proyectar ni la reciben. El video va del equipo
del alumno a tu consola y de ahí a cada compañero, por la red del laboratorio; el equipo del alumno
envía una sola copia.

{{diagrama:21-proyectar-alumno}}

## Mandar un aviso

1. Pulsa **Aviso**.
2. Escribe el mensaje y elige **Información** o **Advertencia** (en amarillo).
3. Pulsa **Enviar aviso**.

En [Código › Últimos envíos](codigo#últimos-envíos) ves quién ya lo vio.

## Pausar las pantallas

Cuando necesitas que te pongan atención («ojos al frente»):

1. Pulsa **Pausar** (o **Ctrl+Mayús+P** para pausar a todos sin abrir el panel).
2. Si quieres, escribe un mensaje corto, como «Miren al pizarrón un momento» (hasta 300
   caracteres).
3. Pulsa **Pausar N pantallas**.

Cada pantalla se cubre, en todos sus monitores, con un aviso que dice **«Tu profesor pausó esta
computadora»**, tu nombre, el canal y tu mensaje. Los alumnos no pierden su trabajo: lo que tenían
abierto sigue ahí. Mientras dure, una franja arriba de la consola dice cuántas pantallas están en
pausa, cuántas no lo han confirmado aún y tu mensaje, con:

- **Cambiar mensaje**: lo cambia en todas las pantallas en pausa, sin quitarla.
- **Reanudar todo**: quita la pausa (también con **Ctrl+Mayús+P**).

Para un solo alumno, abre su pantalla en grande y pulsa **Pausar pantalla** (o **Reanudar
pantalla**). En la lista, su fila lleva la insignia **En pausa**.

Algunas cosas que conviene saber:

- **Si muestras tu pantalla** (o la de un alumno) a quienes están en pausa, la transmisión se ve
  encima de la pausa; al terminar, la pausa vuelve a cubrir todo. Lo mismo pasa con la cuenta
  regresiva de Energía y el aviso de control remoto: el alumno siempre los ve.
- **Si tomas el control** de un alumno en pausa, o lo proyectas a la clase, Aula le quita la pausa
  primero, porque necesitas ver su pantalla real.
- **El alumno que se está mostrando a la clase no se pausa:** su pantalla es la que ven los demás.
- **Si se pierde la conexión** (cierras la consola o se cae la red), la pausa se quita sola en cada
  equipo: nadie se queda bloqueado. Si el alumno vuelve a conectarse, regresa sin pausa.
- **No es un bloqueo total:** Aula no bloquea el teclado. Un alumno podría cambiar de ventana con
  Alt+Tab y escribir sin ver, aunque la pausa vuelve a quedar encima cada medio segundo.
- Por ahora solo se pausan equipos con Windows; los alumnos simulados nunca se pausan.

{{diagrama:08-monitoreo-pantalla}}

## Dar tiempo a un ejercicio

1. Pulsa **Temporizador**.
2. Escribe el nombre («Ejercicio 1») y, si quieres, una **Descripción** con lo que tienen que
   hacer (hasta 1000 caracteres).
3. Elige la duración: 1 a 60 minutos, u **Otro…** para escribir los minutos (hasta 240).
4. Pulsa **Iniciar**.

Los alumnos ven cuánto falta en su barra de arriba, sin que les tape el trabajo. Si escribiste
una descripción, la leen en una tarjeta «Qué hacer» arriba de su lista mientras corre el tiempo. En la franja de
tu consola puedes **Extender** o **Detener**. Más detalles en
[Participación › Temporizador](participacion#temporizador).

## Hacer una pregunta rápida a la clase

1. Pulsa **Encuesta** en la barra de acciones.
2. Escribe la pregunta y elige el tipo: opción múltiple (escribe las opciones), sí o no, o
   respuesta corta.
3. Si quieres saber quién respondió qué, marca **Ver quién respondió qué**; si no, es anónima.
4. Pulsa **Lanzar**.

Las respuestas llegan en vivo a **Participación**, con una barra por opción. Cuando quieras, pulsa
**Cerrar encuesta** y, si es de opción múltiple o de sí o no, **Compartir resultados**: los
alumnos ven solo el total. Más detalles en [Participación › Encuestas](participacion#encuestas).

## Recoger el ejercicio

1. En **Archivos**, abre la pestaña **Recolecciones**.
2. Escribe la actividad («Ejercicio 4») y pulsa **Crear carpeta en los equipos**: cada alumno ve
   dónde guardar su ejercicio.
3. Cuando terminen, pulsa **Recoger**.

Lo de cada alumno llega a tu `Documentos\Aula\recolecciones\<fecha>\<actividad>\`, en una carpeta
por alumno. Solo se lee esa carpeta y el alumno ve antes y después qué se recogió. Más detalles en
[Archivos › Recolecciones](archivos#recolecciones).

## Mandar un mensaje privado a un alumno

Para una pista o una corrección que solo le sirve a uno, sin que lo vea el resto:

1. En [Alumnos](alumnos#acciones-sobre-un-alumno) › **Miniaturas**, haz clic en su pantalla para
   verla en grande.
2. Pulsa **Mensaje privado**: se abre un diálogo. Escribe y elige **Información** o
   **Advertencia**.
3. Pulsa **Enter** (o **Enviar**). **Shift+Enter** pasa a otra línea y **Esc** o **Cancelar**
   cierra el diálogo.

Le aparece en la esquina como **«Mensaje para ti»**, sin quitarle el teclado; ningún otro alumno
lo recibe. Debajo del botón ves «Enviado… · ya lo vio» cuando su equipo lo mostró, y en
[Código › Últimos envíos](codigo#últimos-envíos) aparece como «Mensaje privado a <equipo>», sin
el texto: así no queda a la vista si estás transmitiendo tu pantalla. Funciona también con los
alumnos simulados.

**Mensaje privado** también está en la pantalla en grande de Miniaturas y en la vista del control
remoto, junto a **Tomar captura**. Mientras lo escribes, tu teclado vuelve a la consola: lo que
tecleas no llega a su equipo. Al enviarlo, haz clic en su pantalla para seguir manejándolo; el
control nunca se detiene.

Si el alumno te escribió desde su equipo, **Responder** en [Participación](participacion) hace lo
mismo y además cita su mensaje.

## Bloquear aplicaciones o sitios

1. Pulsa **Restricciones** y elige **Sitios web** o **Aplicaciones**.
2. Elige **Bloquear estos** o **Permitir solo estos**.
3. Escribe uno por línea (o usa una plantilla) y pulsa **Aplicar**.

- **Sitios**: escribe el dominio, como `youtube.com`; incluye sus subdominios.
- **Aplicaciones**: el nombre del programa, como `steam` o `steam.exe`.

Los alumnos ven un aviso mientras haya restricciones, y todo se quita solo al cerrar la clase o
si se pierde la conexión. Solo funcionan en equipos con Windows y Aula instalada como servicio.

> **Lo que no se puede garantizar:** un alumno con permisos de administrador puede detener Aula;
> renombrar un programa evita el bloqueo por nombre; entrar a un sitio por su dirección IP o con
> una VPN evita el filtro.

{{diagrama:15-restricciones}}

## Abrir una página o una app en los equipos

Para que todos abran la misma página («entren a docs.python.org») o el mismo programa:

1. Selecciona a los alumnos, o a nadie para que les llegue a todos los conectados.
2. Pulsa **Abrir** en la barra de abajo.
3. Elige **Página web** y escribe la dirección (con o sin `https://`), o elige **App permitida** y
   escoge una de la lista.
4. Pulsa **Abrir**.

Se abre en el navegador predeterminado de cada alumno, o en su equipo si es una app. Cada alumno
ve un aviso con lo que se abrió y quién lo pidió. En el panel, **Últimas que abriste** dice en
cuántos equipos se abrió y en cuáles no, y por qué. Para un solo alumno, usa **Abrir página o app**
en su [pantalla en grande](alumnos#acciones-sobre-un-alumno): se abre un diálogo solo para él.

- Solo se abren direcciones `http://` o `https://`, y solo las apps de
  [Ajustes › Apps permitidas](ajustes#apps-permitidas). Aula nunca ejecuta comandos.
- Si el filtro web de la clase bloquea esa página, el panel te avisa antes de abrirla: en esos
  equipos el navegador no la encontrará.
- Necesita Aula 0.12 o posterior como servicio, con la sesión del alumno iniciada. A los alumnos
  simulados no les llega.
- Aula no cierra lo que abrió ni sabe si el alumno se quedó en la página.

## Apagar o reiniciar los equipos

1. Pulsa **Energía**.
2. Elige **Reiniciar**, **Apagar** o **Cerrar sesión**.
3. Elige el tiempo para que guarden su trabajo y confirma.

Cada alumno ve un aviso con la cuenta regresiva y puede posponerla 5 minutos, hasta 2 veces. Si
marcas **Forzar ahora**, no hay cuenta y las aplicaciones se cierran sin guardar. Mientras haya
órdenes pendientes, una franja en tu consola permite **Cancelar**.

{{diagrama:16-energia}}

## Ayudar a un alumno tomando el control

1. En [Alumnos](alumnos#acciones-sobre-un-alumno) › **Miniaturas**, haz clic en la pantalla del
   alumno para verla en grande. Es lo más rápido cuando ves que alguien está haciendo otra cosa.
2. Pulsa **Tomar control** arriba.

Se abre su pantalla en grande y en vivo. Haz clic sobre ella: tu mouse y tu teclado manejan su
equipo hasta que pulses **Soltar**. Mientras dura, una franja arriba de tu consola dice
«Controlando LAB3-PC07», con **Ver** y **Soltar**; puedes cerrar la vista para hacer otra cosa en
la consola sin soltar el control.

- **El alumno siempre lo sabe:** su ventana de Aula pasa al frente con una banda roja «Prof. …
  está controlando tu equipo», y un aviso fijo arriba de su pantalla lo dice aunque cierre la
  ventana. No se le pide permiso; tampoco puede cortar el control.
- **Si el alumno mueve su mouse o escribe,** tu control se pausa hasta 2 segundos después: la
  vista dice «El alumno está usando su equipo».
- **Uno a la vez.** Para ayudar a otro, primero suelta al que controlas.
- **Termina solo** si el alumno se desconecta, si pasan 5 minutos sin que uses el mouse ni el
  teclado o si cierras la clase.
- **Teclas que no llegan:** Windows, Alt+Tab y Ctrl+Alt+Supr las atiende tu propio equipo. Para
  que tu teclado vuelva a la consola sin soltar, pulsa **Ctrl+Alt+Fin**.
- **Lo que Windows no deja controlar:** la pantalla de bloqueo, los avisos de «¿Quieres permitir
  que esta aplicación haga cambios?» y las ventanas abiertas como administrador.

Solo se puede en equipos con Windows y una versión de Aula que lo incluya (su ficha dice «Control
remoto: Sí»); los alumnos simulados no se pueden
controlar. La imagen va por la red del laboratorio a unas 8 imágenes por segundo: sirve para
ayudar, no para ver videos.

{{diagrama:17-control-remoto}}

## Tomar una captura de la pantalla de un alumno

Una foto fija de su pantalla, a resolución completa, para revisarla después o dejar evidencia.

1. En [Alumnos](alumnos#acciones-sobre-un-alumno) › **Miniaturas**, haz clic en su pantalla para
   verla en grande.
2. Pulsa **Tomar captura**. En unos segundos ves la **vista previa**; haz clic en la imagen para
   verla a tamaño real.
3. Pulsa **Guardar** para guardarla en tu equipo, o **Descartar** si no la necesitas. Al guardar,
   la consola te dice dónde quedó y te deja **Abrir carpeta**.

También **mientras controlas** a un alumno: el botón **Tomar captura** está arriba, junto a
**Soltar**. Tomarla no suelta el control; tu teclado
deja de llegar a su equipo mientras ves la vista previa, y al cerrarla vuelves a la vista del
control (haz clic en su pantalla para seguir).

**De varios alumnos a la vez:** selecciónalos, abre **Captura** en la barra de abajo y pulsa
**Tomar captura de N alumnos**. Cada una aparece con su miniatura; desmarca las que no quieras y
pulsa **Guardar**. Los equipos que no lo permiten se omiten y se nombran.

- **Se guarda solo si pulsas «Guardar».** Hasta entonces la imagen está únicamente en la memoria
  de la consola; al descartarla, cerrar la clase o cambiar de canal, desaparece.
- **Dónde:** en `Documentos\Aula\capturas\<equipo> (<usuario>)\<fecha>\<hora>.jpg`, una carpeta por
  alumno y por día. La de los alumnos simulados termina en «(simulado)». La carpeta está en
  [Ajustes › Acerca de](ajustes#capturas-de-pantalla). Aula no las borra ni las comparte: son
  tuyas, y conviene borrarlas cuando ya no las necesites.
- **El alumno siempre lo sabe:** un aviso fijo arriba de su pantalla dice «Tu profesor está
  tomando una captura de tu pantalla» antes de tomarla (sale en la imagen), y su ventana de Aula
  le muestra después «Prof. … tomó una captura de tu pantalla (10:42)». Nunca se guarda en
  su equipo.
- **Cuándo no se puede:** con su sesión bloqueada o un aviso de administrador en pantalla
  («¿Quieres permitir que esta aplicación haga cambios?»), y en equipos sin Windows o con una
  versión de Aula anterior (su ficha dice «Tomar una captura de su pantalla: No»).

## Cambiar de canal o de nombre

Toca la tarjeta «Clase abierta · canal» del menú (o ve a [Ajustes](ajustes#clase)), escribe el
canal y confirma. Los alumnos del canal anterior dejan de verte, se quitan las restricciones y se
detiene la transmisión; tu historial de código se conserva.

## Atajos de teclado

Toda la consola se usa con el teclado: **Tab** y **Mayús+Tab** van de un control al siguiente, y
**Enter** o **Espacio** lo activan. Además:

| Atajo | Dónde | Qué hace |
|---|---|---|
| **Ctrl+Alt+Mayús+C** | En cualquier programa | Carga en el editor de Código lo que copiaste. Si otro programa lo ocupa, [Ajustes](ajustes#acerca-de) lo dice. |
| **Ctrl+A** | Alumnos, fuera de un campo de texto | Selecciona a todos los alumnos de la lista. |
| **Ctrl+Mayús+P** | En toda la consola | Pausa las pantallas de todos o, si ya hay pausa, las reanuda (ver [Pausar las pantallas](#pausar-las-pantallas)). |
| **Esc** | Paneles, ventanas y globos de información | Cierra lo que está abierto y vuelve al botón que lo abrió. |
| **Tab** | Dentro del editor de código | Sangra el código. Para salir del editor, pulsa **Ctrl+M** y después **Tab**. |
| **+ − 0** | Sobre un diagrama | Acercan, alejan y vuelven al tamaño original. |
| **↑ ↓** | Pestañas de Ajustes | Cambian de pestaña. |
