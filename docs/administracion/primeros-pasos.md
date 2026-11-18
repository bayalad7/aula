---
titulo: "Primeros pasos"
resumen: "Abre tu clase, envía tu primer código y mira quién lo recibió, en 5 minutos."
grupo: "Primeros pasos"
orden: 10
pantallas: []
---

# Primeros pasos

Con Aula Consola compartes código con tus alumnos: ellos lo ven en su pantalla y lo copian con
un clic. Esta página te lleva de la consola recién instalada a tu primer código recibido.

## Abrir tu primera clase

Al abrir Aula Consola, tu clase se abre sola en el **canal** de tu sala (el que eligió el
administrador al instalar). Lo ves arriba a la izquierda, en la tarjeta «Clase abierta · canal».

Si no se pudo abrir, la consola te dice por qué y qué hacer. Lo más común es que ya haya otra
copia de Aula Consola abierta: ciérrala y pulsa **Abrir la clase**. Más en
[Solución de problemas](solucion-de-problemas#la-consola-no-abre).

> **¿Qué es un canal?** El número de tu sala. Cada computadora de alumno tiene el suyo y tú ves a
> los alumnos de ese número. Más en [Conceptos](conceptos#el-canal).

## Ver a tus alumnos

La pantalla **Alumnos** es la de inicio. Los equipos del laboratorio aparecen solos cuando se
encienden, con su estado: **Conectado** o **Desconectado**.

Si no aparece nadie y no tienes laboratorio a mano, pulsa **Probar con alumnos simulados**: la
consola abre dos alumnos de prueba para que veas todo funcionando. Más en
[Probar sin laboratorio](probar-sin-laboratorio).

## Enviar tu primer código

1. Ve a **Código** en el menú.
2. Escribe o pega tu código en el editor. También puedes copiarlo en otro programa y pulsar
   **Pegar**.
3. Revisa el lenguaje y, si quieres, ponle un título.
4. Pulsa **Emitir** (reemplaza lo que ven).

Los alumnos lo ven al instante, con colores y un botón para copiarlo.

## Ver quién lo recibió

A la derecha de Código, en **Últimos envíos**, cada envío dice cuántos alumnos ya lo tienen en
pantalla, por ejemplo «7 de 8». Si falta alguien, abre **Quién falta** y, si hace falta, pulsa
**Reenviar a los que faltan**. Más en [Código](codigo#últimos-envíos).

![La pantalla Código, con el editor y los últimos envíos](img/codigo.webp)

## Al terminar la hora

Pulsa **Terminar la clase**, debajo de la tarjeta «Clase abierta · canal»: detiene lo que dejaste
activo y limpia la ventana de cada alumno, sin borrar archivos. Más en
[Tareas frecuentes](tareas-frecuentes#terminar-la-clase).

## Instalar en Linux

Esta parte es para quien instala los equipos (el responsable de TI). Aula funciona en Ubuntu
24.04 y en distribuciones parecidas (Debian 12, Fedora 41 o más nuevas). En Linux todavía no
están todas las funciones de Windows; la lista completa está en la tabla de compatibilidad del
repositorio (`docs/compatibilidad.md`).

**En el equipo del profesor**, instala `aula-consola` (el `.deb` en Ubuntu, el `.rpm` en
Fedora) y crea o importa la clave del centro. Luego agrega al grupo `aula` a cada maestro que
use ese equipo; debe cerrar sesión y volver a entrar:

```
sudo apt install ./aula-consola_X.Y.Z_amd64.deb
sudo aula-consola centro crear
sudo aula-consola centro publica -o aula-centro.pub
sudo usermod -aG aula maestra
```

Si el maestro no puede instalar nada, la consola también viene como AppImage: se marca como
ejecutable y se abre con doble clic.

**En cada equipo de alumno**, instala `aula-cliente` y configura su canal con la clave pública
del centro. Arranca solo con cada sesión:

```
sudo apt install ./aula-cliente_X.Y.Z_amd64.deb
sudo aula-cliente configurar -canal 1 -clave aula-centro.pub
```

**Elige la sesión X11.** Para ver la pantalla de los alumnos, cada equipo debe entrar en
**«Ubuntu en Xorg»** (la rueda de la pantalla de inicio de sesión). En la sesión normal
(Wayland), el alumno recibe código, avisos, encuestas y archivos, pero su pantalla no se puede
ver. Más en [Solución de problemas](solucion-de-problemas#en-linux-no-se-ve-la-pantalla-del-alumno).
