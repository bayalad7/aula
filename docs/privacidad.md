# Privacidad y transparencia

Aula es una herramienta de aula **transparente**: el alumno siempre sabe que su equipo está
conectado a un canal y a cuál. No hay modo oculto ni evasión de detección.

## Reglas vigentes

1. **El agente siempre se identifica en pantalla.** La ventana del cliente muestra, fijo arriba
   y también en el título de la ventana y en la barra de tareas:
   - Conectado: **"Aula — conectado (canal <X> - <profesor>)"**
   - Buscando: "Aula — buscando canal <X>"
   - Sin conexión: "Aula — sin conexión (canal <X>); reintentando"
   - Sin configurar: "Aula — sin configurar" (con la explicación)
2. **Al cerrar la ventana, el agente no se detiene y debe quedar ejecutandose en un proceso en segundo plano.**
3. **El agente se presenta con su nombre real** (equipo y usuario del sistema) y solo después de
   comprobar que la consola es la autorizada (`docs/protocol.md` §3.1). Sabe quién lo ve.
4. **Lo que recibe el alumno se ve.** El código y los avisos aparecen en pantalla y la ventana
   se trae al frente. Las confirmaciones (`code.ack`, `msg.ack`) solo dicen "se mostró", no
   "se leyó" ni "se copió".
5. **Registros.** No se registran el código, los textos de los avisos, las claves, los nonces ni
   las firmas. Sí el tipo de mensaje, los identificadores, el equipo y los tamaños.

## Monitoreo de pantalla (etapa 09, Windows)

Reglas para `screen.thumb.*` (`docs/protocol.md` §5.6–5.8) y, cuando se implemente, para la
pantalla en blanco (`screen.freeze`, §5.9–5.11, **pendiente**: reglas 9 y 10).

6. **Si la pantalla se ve, el alumno lo sabe.** Desde que llega `screen.thumb.start`, y
   **antes** de capturar el primer cuadro, el equipo muestra un **distintivo fijo, siempre
   encima de las demás ventanas**, en la parte superior del monitor principal:

   > **● Tu pantalla está siendo vista por el profesor**

   Además, la barra de la ventana de Aula cambia a:

   > **Aula — tu pantalla está siendo vista por <profesor> (canal <X>)**

   El distintivo no roba clics ni el foco, no se puede ocultar desde Aula y desaparece solo
   cuando la transmisión termina (`screen.thumb.stop`, desconexión o cierre del agente). Es
   visible también en las miniaturas: el profesor ve lo mismo que sabe el alumno.
7. **Sin capturas en disco.** Ni el agente ni la consola escriben capturas en disco. La consola
   conserva en memoria el último cuadro de cada equipo mientras la vista de pantallas está
   abierta y lo descarta al cerrarla. Guardar una captura será, cuando exista, una acción
   explícita del profesor, y se documentará aquí antes de implementarla.
8. **Solo se transmite mientras el profesor mira.** La consola envía `screen.thumb.start` al
   abrir la vista de pantallas y `screen.thumb.stop` al cerrarla. Como máximo 2 cuadros por
   segundo y solo del monitor principal.
9. **(Pendiente.) La pantalla en blanco explica por qué y quién.** Cubre todos los monitores con:

   > **Tu profesor pausó esta computadora**
   > *<mensaje del profesor, si lo hay>*
   > Aula — clase de <profesor> (canal <X>). Se reanudará cuando el profesor lo indique.

   Tapa todos los monitores y recibe los clics. Aula **no intercepta ni registra el teclado**.
10. **(Pendiente.) Nadie queda bloqueado si se pierde la clase.** Si la conexión con la consola se cierra
    (la consola se cierra, se cae la red), el agente quita la pantalla en blanco y el
    distintivo de inmediato.
11. **Registros.** No se registran las imágenes ni el mensaje de la pantalla en blanco; sí el
    tipo de mensaje, `seq`, tamaños y dimensiones.

## Pendiente para etapas futuras

- Transmisión de la pantalla del profesor (`screen.broadcast.*`), guardar una captura por
  petición explícita del profesor, y Linux y macOS: sus textos de aviso se agregarán aquí antes
  de implementarlos, con las mismas reglas 6–11.
