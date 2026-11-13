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
