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

## El portapapeles no funciona

Si «Pegar del portapapeles» dice que no hay texto copiado, copia el código con Ctrl+C y vuelve a
intentarlo. Si dice que no puede leer el portapapeles, pega el código en el editor con Ctrl+V.
Aula envía hasta 256 KiB por envío.
