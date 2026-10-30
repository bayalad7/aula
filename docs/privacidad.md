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

## Transmisión de la pantalla del profesor (v0.4, Windows)

Reglas para `screen.broadcast.*` (`docs/protocol.md` §5.12), aprobadas el 2026-10-29.

12. **El alumno sabe que está recibiendo, y de quién.** Al llegar `screen.broadcast.start`, y
    **antes** de aceptar la conexión de video, la ventana de Aula se trae al frente, se maximiza
    y cambia a la vista de transmisión, con una franja fija arriba del video:

    > **▶ Estás viendo la pantalla de <profesor> (canal <X>)**

    La barra de la ventana y la barra de tareas cambian a:

    > **Aula — viendo la pantalla de <profesor> (canal <X>)**

    Mientras se conecta o reconecta, la vista dice "Conectando con la pantalla del profesor…" en
    lugar del video. El botón **Tamaño real** muestra el video píxel a píxel, con barras de
    desplazamiento, si la pantalla del alumno es más chica que la del profesor. La franja no se
    puede ocultar desde Aula. El alumno puede minimizar o
    redimensionar la ventana (no es un bloqueo: bloquear es la pantalla en blanco, reglas 9–10);
    el título de la barra de tareas sigue indicando la transmisión.
13. **Recibir no expone al alumno.** La conexión es solo de recepción: el agente rechaza ofertas
    con audio, canal de datos o envío de video, no usa cámara, micrófono ni captura de pantalla
    para esto, y no envía nada propio por WebRTC. Por eso **no** se muestra el distintivo "Tu
    pantalla está siendo vista" (regla 6), que significa otra cosa; si el profesor abre además
    las miniaturas, ese distintivo aparece como siempre.
14. **Termina con la clase.** La vista y su franja desaparecen con `screen.broadcast.stop`, al
    cerrarse la conexión con la consola o al cerrar el agente. Al terminar, la ventana vuelve a
    la vista de código.
15. **Lo que el profesor transmite, lo sabe el profesor.** Mientras transmite, la consola muestra
    una franja roja "● Transmitiendo tu pantalla (monitor N) a M alumnos" con el botón
    "Detener", y no transmite nada sin que el profesor pulse "Transmitir mi pantalla". La
    ventana de la consola **se excluye de la captura** (`WDA_EXCLUDEFROMCAPTURE`; en Windows
    anteriores a 10 2004 se ve negra): la lista de alumnos, sus usuarios y sus miniaturas nunca
    llegan a la clase aunque estén en el monitor transmitido. Si Windows no permite excluirla,
    la consola **no transmite** y lo explica. Se recomienda al profesor silenciar las notificaciones del sistema durante la
    transmisión.
16. **Sin grabación.** Ni la consola ni el agente graban el video ni escriben cuadros en disco.
    Los logs registran identificadores, intentos, estados y tamaños; no el SDP ni los candidatos.

## El agente como servicio de Windows (D-016)

17. **Correr como servicio no oculta nada.** El servicio no tiene ventana: el distintivo de
    identidad, el código, los avisos, el distintivo "Tu pantalla está siendo vista" y la vista de
    transmisión (reglas 1, 4, 6 y 12) los muestra el **ayudante**, un `aula-cliente.exe` que el
    servicio lanza en la sesión del alumno con sus permisos normales. El servicio **no puede
    capturar la pantalla ni mostrar nada por sí mismo**: la captura solo existe en el ayudante,
    que muestra el distintivo antes. Sin ayudante (nadie inició sesión, o se acaba de cerrar), el
    agente se conecta **sin capacidades**, salvo `power` (regla 29): la consola solo ve que el
    equipo está encendido y no puede pedirle pantalla ni código.
18. **El alumno ve el agente.** El servicio se llama "Aula — agente del aula" en
    `services.msc`, con una descripción clara, y el ayudante aparece como `aula-cliente.exe` en el
    Administrador de tareas. No hay nombres engañosos ni procesos escondidos. Si el alumno cierra
    el ayudante, el servicio lo vuelve a abrir en segundos y con él el distintivo.
19. **Lo que cambia con los privilegios.** Antes, el alumno podía cerrar el agente; ahora solo un
    administrador puede detener el servicio. Correr como SYSTEM no habilita por sí mismo ninguna
    función nueva: cada una que use esos privilegios se documenta aquí, con su aviso visible,
    antes de implementarse. Ya lo están `app.block` y `web.filter` (reglas 20–25) y `power.*`
    (reglas 26–31); falta `remote.*`.

## Bloqueo de aplicaciones y filtro web (v0.4, Windows)

Reglas para `app.block.*` y `web.filter.*` (`docs/protocol.md` §5.13–5.14), aprobadas el
2026-10-29.

20. **El alumno sabe que hay restricciones, y de qué tipo.** Al llegar una política, y **antes**
    de aplicarla, la ventana de Aula pasa al frente con un aviso fijo debajo del distintivo:

    > **🔒 Restricciones de la clase activas: aplicaciones bloqueadas · sitios web filtrados**

    Solo aparece el tipo que esté activo. Con el modo de lista blanca dice "solo se permiten
    algunas aplicaciones" o "solo se permiten algunos sitios". La barra de la ventana agrega
    "· restricciones activas".
21. **La lista se puede consultar, sin abrumar.** Un botón "Ver detalles" muestra las apps y los
    dominios. Si son más de 15, muestra los 15 primeros y "y N más".
22. **Cerrar una app no es silencioso.** Cuando Aula cierra una app bloqueada, el alumno ve
    "Aula cerró «steam.exe»: está bloqueada durante la clase". El aviso se agrupa: uno por app
    cada 30 s.
23. **No se vigila la navegación.** El filtro decide por nombre de dominio y no guarda ni envía a
    la consola qué sitios intentó abrir el alumno, ni cuántos. La consola solo sabe si la
    política está aplicada (`active`). Los logs registran altas y bajas de políticas, nunca
    dominios consultados.
24. **Todo se restablece solo.** Las restricciones se quitan con "Quitar restricciones", al
    perder la conexión con la consola, al cerrar la clase, al cerrar sesión y al detener el
    servicio. Si el agente se cae o el equipo se apaga a mitad de clase, el agente las quita al
    volver a arrancar, antes de conectarse (D-017). No hay filtrado permanente ni oculto, y no
    quedan entradas en `hosts`, DNS ni proxy.
25. **Lo que no se puede garantizar** se le dice al profesor en la consola:
    - un alumno con permisos de administrador puede detener el agente;
    - renombrar un ejecutable evita el bloqueo por nombre;
    - entrar a un sitio por su dirección IP evita el filtro por nombre;
    - Ctrl+Alt+Supr no se bloquea.

## Apagar, reiniciar y cerrar sesión (v0.5, Windows)

Reglas para `power.*` (`docs/protocol.md` §5.15), aprobadas el 2026-10-29. Una orden de apagado
puede hacer perder trabajo no guardado: por eso, salvo que el profesor la fuerce, el alumno
siempre tiene tiempo de guardar.

26. **Aviso antes de apagar, siempre visible.** Sin `force`, al llegar la orden la ventana de Aula
    pasa al frente, queda siempre encima y muestra un aviso fijo con la cuenta en vivo:

    > **⏻ El profesor <nombre> (canal <X>) va a reiniciar este equipo en 0:30. Guarda tu
    > trabajo.**

    ("apagar este equipo" o "cerrar tu sesión" según la orden), con el botón **"Posponer 5 min
    (te quedan 2)"**. La cuenta dura 30 s si el profesor no elige otra (10 s a 10 min). La barra
    de la ventana agrega "· reinicio pendiente" (o "apagado" o "cierre de sesión"), también sin
    conexión.
27. **Posponer.** Cada aplazamiento suma 5 minutos a lo que falta, como máximo 2 veces por orden.
    La consola ve cada aplazamiento. Agotados, el botón se deshabilita con "Ya no se puede
    posponer". Aula no cierra nada durante la cuenta.
28. **Forzar es explícito y se advierte.** Con `force` no hay cuenta: el alumno ve "El profesor
    <nombre> está apagando este equipo" mientras Windows lo apaga, y las aplicaciones se cierran
    **sin guardar**. La consola lo advierte en rojo antes de confirmar. Sin `force`, Windows
    deja que las apps pidan guardar ("Esta aplicación impide…").
29. **Solo la consola autenticada.** Ninguna orden `power.*` se procesa antes de `auth.ok`. Sin
    sesión iniciada (el servicio sin ayudante, regla 17), el agente declara solo `power`:
    apagar o reiniciar es inmediato, porque no hay trabajo que perder, y queda en el log del
    servicio. Cerrar sesión no aplica.
30. **La cuenta no depende de la red.** Si se pierde la conexión con la consola, la cuenta sigue
    visible, se puede posponer y al final se ejecuta. El profesor puede anularla con "Cancelar"
    mientras esté conectado; el aviso desaparece de inmediato.
31. **Registros.** Se registran la acción, `action_id`, `force`, los aplazamientos, el
    resultado y el equipo, **antes** de ejecutar. Nunca qué aplicaciones estaban abiertas ni su
    contenido.

Permisos del SO (detalle en `docs/protocol.md` §5.15.1): apagar y reiniciar usan
`SeShutdownPrivilege`, que tienen SYSTEM y, salvo una GPO que lo quite, los usuarios de un equipo
de escritorio; cerrar la sesión de otro usuario (con `force`, desde el servicio) requiere SYSTEM.

## Pendiente para etapas futuras

- Guardar una captura por petición explícita del profesor, y Linux y macOS (incluida la
  recepción de la transmisión y las restricciones): sus textos de aviso se agregarán aquí antes
  de implementarlos, con las mismas reglas 6–31.
