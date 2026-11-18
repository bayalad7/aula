---
titulo: "Solución de problemas"
resumen: "Qué hacer si un alumno no aparece, si la consola no abre o si algo no funciona."
grupo: "Solución de problemas"
orden: 70
pantallas: [falta]
---

# Solución de problemas

## Un alumno no aparece

Abre [¿Falta un alumno?](pantalla:falta): te hace una pregunta sobre lo que dice la barra de Aula
en el equipo del alumno y te lleva a la causa. Ese asistente sale de este diagrama:

{{diagrama:12-diagnostico-conexion}}

Las causas más comunes:

- **Otro canal**: el equipo del alumno tiene otro número. El del alumno es fijo; cambia el tuyo
  (ver [El alumno está en otro canal](#el-alumno-está-en-otro-canal)).
- **Red Pública**: Windows trata la red como pública y no deja que los equipos se encuentren (ver
  [El firewall o la red Pública](#el-firewall-o-la-red-pública)).
- **Otra clave del centro** (ver [La clave no coincide](#la-clave-no-coincide)).

## Comprobar la conexión

Arriba de [¿Falta un alumno?](pantalla:falta), **Comprobar la conexión** recorre desde tu equipo el
camino que sigue un alumno hasta tu clase y dice en qué paso se atoraría. Tarda unos segundos y no
envía nada a los alumnos, ni a los reales ni a los simulados.

1. **Tu clase se anuncia en la red**: busca el anuncio de tu canal como lo haría un alumno. Si ve
   otra consola en el mismo canal, te avisa: los alumnos de ese canal podrían llegar a cualquiera
   de las dos.
2. **El puerto de la consola responde**: se conecta al puerto TCP de cada dirección de red de tu
   equipo.
3. **Conexión segura y clave del centro**: hace lo mismo que un alumno, comprobar el certificado y
   la firma con la clave del centro, y se desconecta antes de entrar, así que no aparece en la
   lista.
4. **La red y el firewall de este equipo**: revisa si la red es Pública y si el firewall deja
   entrar a Aula Consola. En Linux y macOS este paso dice «No se puede comprobar».

Cada paso que falla dice qué hacer. Desde tu equipo no se ve el del alumno: su canal, su clave, su
red y su firewall. Si todo sale bien y aun así falta, sigue las preguntas de «Vamos paso a paso» o
pide a TI que ejecute `aula-cliente diagnostico` en su equipo.

## El alumno está en otro canal

Cada computadora de alumno tiene su canal fijo, que solo el administrador cambia. Si tu sala usa
otro número, cambia el tuyo: toca la tarjeta «Clase abierta · canal» del menú. Si no sabes qué
canal tiene el alumno, la barra de Aula en su equipo lo dice: «Aula — buscando canal N».

## El firewall o la red Pública

Para que el alumno encuentre la consola:

- la red de Windows de los dos equipos debe ser **Privada** o **de Dominio** (en Configuración ›
  Red e Internet);
- el firewall del equipo del profesor debe permitir Aula Consola (el instalador agrega la regla).

Si la red bloquea la búsqueda automática, el responsable de TI puede configurar en el alumno la
dirección de esta consola. Dale los datos de [Ajustes › Acerca de](ajustes#acerca-de).

## La clave no coincide

Los equipos de los alumnos solo obedecen a consolas con la **clave del centro**. Si el alumno dice
«…su clave no es la de este centro», la consola y el alumno se instalaron con claves distintas:
el administrador debe reinstalar uno de los dos con la clave correcta. Si tu consola muestra la
franja «Este equipo no tiene la clave del centro», pide que la instalen en tu equipo.

## La consola no abre

- **«Otra copia de Aula Consola ya está abierta»**: ciérrala y
  pulsa **Abrir la clase**.
- **«No se pudo leer la clave» o «No se pudo preparar la conexión segura»**: un archivo de Aula
  en este equipo está dañado o sin permisos. Pide al administrador que reinstale Aula Consola.

## No puedo mostrar mi pantalla

- **Falta el codificador de video**: en las ediciones N de Windows, instala el «Media Feature
  Pack».
- **Ya estás transmitiendo**: detén la transmisión actual con **Detener** en la franja roja.
- Si no se pudo ocultar la ventana de Aula de la transmisión, Aula no transmite, para que tus
  alumnos no vean la lista de la clase. Vuelve a intentarlo.

## No se puede proyectar a un alumno

- **El botón no aparece en su pantalla en grande:** su equipo no tiene Windows o tiene una versión de Aula
  anterior (la ficha dice «Mostrar su pantalla a la clase: no»), o es un alumno simulado.
- **«No pudo mostrar su pantalla»:** su sesión estaba bloqueada, su Windows no permite ocultar la
  ventana de Aula de la captura, o le falta el codificador de video (en ediciones N, el «Media
  Feature Pack»). Revisa su equipo y vuelve a intentarlo.
- **«No se pudo empezar a proyectar»:** otro programa usa el puerto de video de Aula (el del
  servidor más uno, 47831 por omisión). Cierra la otra consola o el programa y vuelve a intentarlo.
- **Sus compañeros dicen «Conectando…» sin imagen:** el firewall del equipo del alumno bloquea la
  salida UDP hacia tu consola, o la red no deja pasar el puerto de video. Con
  [Comprobar la conexión](#comprobar-la-conexión) revisa el camino de ese alumno.
- **«Su equipo no contestó»:** Aula no responde en su equipo (o la red se cortó). Revisa que esté
  conectado en Alumnos y vuelve a intentarlo.

## Un equipo no se enciende

Si un equipo se queda en **No respondió** después de **Encender**, casi siempre falta algo en el
propio equipo o en la red. Pide al responsable de TI que revise, en cada equipo del laboratorio:

1. **Wake-on-LAN activado en la BIOS o UEFI.** Suele llamarse «Wake on LAN», «Power On by PCI-E»
   o «Resume by LAN». Si existe «ErP» o «Deep Sleep», debe estar desactivado: corta la corriente
   de la tarjeta de red.
2. **Wake-on-LAN activado en la tarjeta de red de Windows.** En el Administrador de dispositivos,
   en las propiedades de la tarjeta de red: en **Opciones avanzadas**, «Wake on Magic Packet»
   activado, y en **Administración de energía**, «Permitir que este dispositivo reactive el
   equipo».
3. **«Inicio rápido» de Windows desactivado.** En Opciones de energía › Elegir el comportamiento
   de los botones de inicio/apagado. Con Inicio rápido, el equipo no queda apagado del todo y
   muchas tarjetas no despiertan.
4. **Cable de red.** Por Wi-Fi casi nunca se puede encender un equipo apagado.
5. **La misma red que la consola.** La señal de encendido no cruza routers. Si la fila dice
   **Otra red**, la consola está en otra subred o VLAN que el equipo: conéctala a la red del
   laboratorio o pide que el switch deje pasar el broadcast hacia esa VLAN.

Si el botón **Encender** no aparece, el agente del equipo es una versión que no informa su
dirección física: actualízalo, o enciende el equipo a mano una vez después de actualizarlo.

## Un equipo tiene el disco casi lleno

Lo marca [Laboratorio](laboratorio): el disco de Windows o el de Documentos tiene menos de lo que
fijaste en Ajustes › Salud (por omisión, 5 GB o el 10 %). Sin espacio, Windows y los programas
fallan, y el alumno no puede guardar su trabajo ni recibir archivos.

1. En ese equipo, abre **Liberador de espacio en disco** (o Configuración › Sistema ›
   Almacenamiento › Archivos temporales) y borra los temporales y la papelera.
2. Borra lo que ya no se use en ese equipo (descargas viejas, instaladores).
3. Pulsa **Revisar ahora** en Laboratorio: la alerta se va cuando hay espacio.

## El disco de un equipo tiene fallas

El disco avisa por sí mismo (S.M.A.R.T.) que puede dejar de funcionar: sectores dañados, errores
de lectura o, en un disco NVMe, que ya no es confiable o quedó de solo lectura. Un disco así
puede dejar de arrancar de un día para otro.

1. Respalda lo importante de ese equipo.
2. Pide al responsable de TI que cambie el disco antes de la próxima clase.

Si el estado dice «No se puede leer en este equipo», Aula no tiene permiso para preguntarle al
disco: pasa cuando Aula Cliente corre como ventana y no como servicio.

## Un equipo tiene poca memoria o el procesador saturado

El equipo va a ir lento. Casi siempre es un programa que se quedó abierto o una actualización
de Windows que trabaja en segundo plano.

1. En ese equipo, cierra los programas que no se usen.
2. Si sigue igual, espera a que termine la actualización o reinícialo antes de clase.
3. Si pasa siempre, el equipo tiene poca memoria para lo que se usa en tu clase.

## El procesador de un equipo está limitado

Windows baja la velocidad del procesador para que no se caliente de más. Explica por qué un
equipo «lento» falla sin tener nada pesado abierto. Casi siempre es **polvo** en el disipador o
el ventilador, o el equipo está tapado o sin ventilación. Pide que lo limpien. En una laptop
también pasa con el plan de energía en «Ahorro de batería».

## Un equipo espera un reinicio

Windows instaló actualizaciones y necesita reiniciar. Si no lo haces, puede reiniciarse solo en
plena clase. Reinícialo antes de empezar (puedes usar [Apagar o reiniciar los
equipos](tareas-frecuentes#apagar-o-reiniciar-los-equipos)).

## El reloj de un equipo está desfasado

El reloj del equipo se adelanta o se atrasa frente al de tu consola más de lo que fijaste (por
omisión, 2 minutos). Con el reloj mal, los sitios seguros (HTTPS) y los certificados fallan, y
algunos programas no instalan ni inician sesión.

1. En ese equipo, abre Configuración › Hora e idioma y pulsa **Sincronizar ahora**. Revisa que la
   zona horaria sea la correcta.
2. Si vuelve a desfasarse después de apagarlo, la pila de la tarjeta madre puede estar gastada.

Si **todos** los equipos aparecen desfasados por lo mismo, el reloj que está mal es el de tu
consola.

## Un equipo no tiene internet

Lo marca Laboratorio solo si pusiste una dirección en [Ajustes › Salud](ajustes#salud).

- **Sin DNS:** el equipo no encuentra los sitios por su nombre. Revisa su cable o su Wi‑Fi y su
  configuración de red; si les pasa a todos, el servidor DNS del laboratorio no responde.
- **Sin internet:** encuentra el nombre pero no llega. Revisa el proxy y el firewall del
  laboratorio, y que el puerto de la dirección de prueba sea el correcto.

Las restricciones de la clase pueden bloquear sitios a propósito: si filtras la web, elige una
dirección de prueba que el filtro permita.

## El cable de red de un equipo va lento

El cable negoció una velocidad menor a la que fijaste (por omisión, 100 Mb/s). Es el síntoma
clásico de un **cable dañado o mal ponchado**, o de un puerto del switch con problemas. Con
10 Mb/s, ver tu pantalla o recibir archivos se vuelve imposible.

1. Cambia el cable de ese equipo por uno que funcione.
2. Si sigue igual, conéctalo a otro puerto del switch o de la pared.

## Un equipo se conecta por Wi-Fi

- **Wi‑Fi teniendo cable:** el equipo tiene un cable conectado pero sale por Wi‑Fi. Suele pasar
  cuando el cable o el puerto están mal. Revisa el cable o apaga el Wi‑Fi del equipo.
- **Wi‑Fi débil:** la señal es baja; la transmisión de tu pantalla y los archivos irán lentos o
  se cortarán. Acerca el equipo al punto de acceso o conéctalo por cable.

## Un equipo funciona con batería

- **Sin corriente:** la laptop no está cargando. Si les pasa a varias juntas, revisa el
  multicontacto o el interruptor del salón.
- **Batería baja:** se va a apagar pronto. Conéctala a la corriente ya.
- **Batería gastada:** perdió más capacidad de la que fijaste (por omisión, la mitad). Que
  trabaje siempre conectada o pide que cambien la batería.

## En Linux no se ve la pantalla del alumno

Si en la ficha del alumno «Ver su pantalla» y «Captura» salen como no disponibles y su equipo
usa Linux, ese equipo está en una sesión **Wayland**, donde ninguna app puede ver la pantalla de
las demás ni dejar el aviso para el alumno siempre encima. Pide que el alumno cierre sesión y,
en la pantalla de inicio, elija **«Ubuntu en Xorg»** con la rueda junto al botón de entrar. Para
todo el laboratorio, el responsable de TI puede desactivar Wayland en el inicio de sesión
(`WaylandEnable=false` en `/etc/gdm3/custom.conf`). En el equipo, `aula-cliente diagnostico`
dice qué sesión usa y qué falta.

Tampoco funciona en Wayland el atajo del teclado de la consola para traer código: copia con Ctrl+C y pulsa
**Pegar** en la vista Código.

## En Linux la consola no reconoce la clave del centro

Si la consola de un equipo Linux dice que no tiene la clave del centro aunque se creó, el
maestro no está en el grupo `aula`, que es el único que puede leerla. El responsable de TI lo
agrega con `sudo usermod -aG aula <usuario>` y el maestro cierra sesión y vuelve a entrar.

## El portapapeles no funciona

Si «Pegar» dice que no hay texto copiado, copia el código con Ctrl+C y vuelve a
intentarlo. Si dice que no puede leer el portapapeles, pega el código en el editor con Ctrl+V.
Aula envía hasta 256 KiB por envío.

## El equipo del alumno no pudo hacerlo

A veces el equipo de un alumno recibe lo que mandaste pero contesta que no pudo hacerlo. La
consola lo muestra en dos lugares: en **Código**, dentro de «Quién falta» del envío, y en
**Alumnos**, en la columna Notas de ese alumno (por ejemplo, «No pudo: versión distinta»).

| Lo que dice | Qué hacer |
|---|---|
| **No puede hacer eso** | Ese equipo no tiene esa función (por ejemplo, mostrar su pantalla fuera de Windows). Revisa en su ficha qué puede hacer: [Qué puede hacer cada equipo](conceptos#qué-puede-hacer-cada-equipo). |
| **Versión distinta** o **No entendió la orden** | Aula del alumno y Aula Consola son de versiones que no se entienden. Pide al responsable de TI que instale en los dos la misma versión. |
| **Envío demasiado grande** | Manda el código en partes más pequeñas. |
| **No confía en esta consola** | Ve [La clave no coincide](#la-clave-no-coincide). |
| **Falló en el equipo del alumno** | Pulsa **Reenviar a los que faltan**: también reintenta a quien falló. Si se repite, reinicia Aula en ese equipo o el equipo. |
