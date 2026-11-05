---
titulo: "Tareas frecuentes"
resumen: "Mostrar tu pantalla, mandar avisos, restringir aplicaciones y sitios, apagar equipos, tomar el control de un equipo, tomar una captura de su pantalla y cambiar de canal."
grupo: "Tareas frecuentes"
orden: 60
pantallas: []
---

# Tareas frecuentes

Todas estas acciones están en la barra de abajo de [Alumnos](alumnos#acciones-sobre-la-selección).
Sin selección van a todos los conectados; con selección, solo a los elegidos.

## Mostrar tu pantalla

1. Pulsa **Mostrar mi pantalla**.
2. Elige el monitor y pulsa **Empezar**.

Tus alumnos ven tu pantalla en grande, con tu nombre arriba. Una franja roja en tu consola te
recuerda que estás transmitiendo; la ventana de Aula no aparece en la transmisión. Para terminar,
pulsa **Detener** en la franja. Por ahora, solo equipos con Windows pueden recibirla.

{{diagrama:14-transmision-pantalla}}

## Mandar un aviso

1. Pulsa **Aviso**.
2. Escribe el mensaje y elige **Información** o **Advertencia** (en amarillo).
3. Pulsa **Enviar aviso**.

En [Código › Últimos envíos](codigo#últimos-envíos) ves quién ya lo vio.

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

## Apagar o reiniciar los equipos

1. Pulsa **Energía**.
2. Elige **Reiniciar**, **Apagar** o **Cerrar sesión**.
3. Elige el tiempo para que guarden su trabajo y confirma.

Cada alumno ve un aviso con la cuenta regresiva y puede posponerla 5 minutos, hasta 2 veces. Si
marcas **Forzar ahora**, no hay cuenta y las aplicaciones se cierran sin guardar. Mientras haya
órdenes pendientes, una franja en tu consola permite **Cancelar**.

{{diagrama:16-energia}}

## Ayudar a un alumno tomando el control

1. En [Alumnos](alumnos#la-ficha-de-un-alumno), abre la **ficha** del alumno (botón de información
   al final de su fila).
2. Pulsa **Tomar control**.

También desde **Miniaturas**: haz clic en la pantalla de un alumno para verla en grande y pulsa
**Tomar control** arriba. Es lo más rápido cuando ves que alguien está haciendo otra cosa.

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

1. En [Alumnos](alumnos#la-ficha-de-un-alumno), abre la **ficha** del alumno.
2. Pulsa **Tomar captura**. En unos segundos ves la **vista previa**; haz clic en la imagen para
   verla a tamaño real.
3. Pulsa **Guardar** para guardarla en tu equipo, o **Descartar** si no la necesitas. Al guardar,
   la consola te dice dónde quedó y te deja **Abrir carpeta**.

También desde la vista en grande de **Miniaturas** y **mientras controlas** a un alumno: el botón
**Tomar captura** está arriba, junto a **Soltar**. Tomarla no suelta el control; tu teclado
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
| **Esc** | Paneles, ventanas y globos de información | Cierra lo que está abierto y vuelve al botón que lo abrió. |
| **Tab** | Dentro del editor de código | Sangra el código. Para salir del editor, pulsa **Ctrl+M** y después **Tab**. |
| **+ − 0** | Sobre un diagrama | Acercan, alejan y vuelven al tamaño original. |
| **↑ ↓** | Pestañas de Ajustes | Cambian de pestaña. |
