# Privacidad y transparencia

Aula es una herramienta de aula **transparente**: el alumno siempre sabe que su equipo está
conectado a un canal y a cuál. No hay modo oculto ni evasión de detección.

## Reglas vigentes

1. **El agente siempre se identifica en pantalla.** El título de la ventana, que también se ve
   en la barra de tareas y en el icono del área de notificación (regla 2), dice:
   - Conectado: **"Aula — conectado (canal <X> - <profesor>)"**
   - Buscando: "Aula — buscando canal <X>"
   - Sin conexión: "Aula — sin conexión (canal <X>); reintentando"
   - Sin configurar: "Aula — sin configurar"

   Arriba de la ventana, fija, la tira de identidad dice lo mismo para el alumno: "Conectado al
   canal <X> · <profesor>", "Buscando la clase del canal <X>…", "Se perdió la conexión con
   <profesor>. Reconectando…" o "Aula aún no está lista en este equipo. Avisa a tu profesor."; el
   motivo técnico va plegado en «Detalles para TI». Lo nuevo sin ver se cuenta al frente del
   título: "(2) Aula — conectado…". En el modo compacto, la tira resume canal y profesor.

   **Bandas de transparencia (prompt 19).** Lo que el profesor hace con este equipo se ve en
   bandas fijas arriba de la ventana, en este orden: energía; control remoto (reglas 36-41);
   pantalla vista, o en pausa (cuando exista, reglas 9-10); restricciones. La transmisión lleva la suya sobre el
   video. **Ninguna tiene botón para cerrarse ni se oculta con el teclado**: cada una desaparece
   sola cuando el profesor termina. Lo prueban `scripts/ux/pruebas-alumno.mjs` y, del lado de Go,
   `TestNoBindingTurnsOffTransparency` (ninguna función de la ventana las desactiva).
2. **Al cerrar la ventana, el agente no se detiene: sigue en segundo plano, a la vista.** Cerrar
   la ventana la oculta y deja el icono de Aula en el área de notificación de Windows («Mostrar
   iconos ocultos»), con el estado al pasar el puntero y «Abrir Aula». Mientras está oculta, el
   código y los avisos que llegan solo marcan el icono; la ventana no reaparece sola, salvo por
   una acción del profesor sobre el equipo (energía, restricciones, su transmisión o el control remoto, reglas 12, 20,
   26 y 36). El distintivo de pantalla vista (regla 6) no depende de la ventana: se ve igual. Donde no
   hay área de notificación (Linux y macOS por ahora), cerrar minimiza.
3. **El agente se presenta con su nombre real** (equipo y usuario del sistema) y solo después de
   comprobar que la consola es la autorizada (`docs/protocol.md` §3.1). Sabe quién lo ve.
4. **Lo que recibe el alumno se ve, sin quitarle el teclado.** El código y los avisos aparecen en
   la ventana de Aula, pero nunca le quitan el foco ni la mueven (prompt 19). Si está visible, se
   actualizan en su lugar; si está minimizada, el botón de la barra de tareas parpadea y el título
   dice cuántos hay nuevos; si está cerrada al área de notificación, el icono se marca. Solo las
   acciones del profesor sobre el equipo traen la ventana al frente (reglas 12, 20, 26 y 36). Las
   confirmaciones (`code.ack`, `msg.ack`) solo dicen "se mostró", no "se leyó" ni "se copió".
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

   y la ventana muestra la banda de transparencia «Tu pantalla la está viendo <profesor> (canal
   <X>)», también en el modo compacto («Pantalla vista»).

   El distintivo no roba clics ni el foco, no se puede ocultar desde Aula y desaparece solo
   cuando la transmisión termina (`screen.thumb.stop`, desconexión o cierre del agente). Es
   visible también en las miniaturas: el profesor ve lo mismo que sabe el alumno.
7. **Sin capturas en disco, salvo que el profesor pulse «Guardar».** El agente nunca escribe
   capturas en disco. La consola conserva en memoria el último cuadro de cada equipo mientras la
   vista de pantallas está abierta y lo descarta al cerrarla. **La única excepción** es la captura
   de pantalla de un alumno (reglas 44–49): la consola la escribe en disco solo cuando el profesor
   pulsa «Guardar», nunca sola, nunca en el equipo del alumno y nunca en los registros.
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

    > **Estás viendo la pantalla de <profesor> (canal <X>)** (con su icono)

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

    > **Restricciones de la clase activas: aplicaciones bloqueadas · sitios web filtrados** (con
    > el icono de un candado)

    Solo aparece el tipo que esté activo. Con el modo de lista blanca dice "solo se permiten
    algunas aplicaciones" o "solo se permiten algunos sitios". La barra de la ventana agrega
    "· restricciones activas".
21. **La lista se puede consultar, sin abrumar.** Un botón "Ver detalles" muestra las apps y los
    dominios. Si son más de 15, muestra los 15 primeros y "y N más".
22. **Cerrar una app no es silencioso.** Cuando Aula cierra una app bloqueada, el alumno ve en la
    banda de restricciones "Aula cerró «steam.exe»: está bloqueada durante la clase". El aviso
    se agrupa (uno por app cada 30 s; varias apps en una sola línea) y se queda hasta que el
    alumno pulsa «Entendido» o se quitan las restricciones: si no estaba mirando, sabe por qué se
    cerró su app.
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

    > **<profesor> (canal <X>) va a reiniciar este equipo en 0:30. Guarda tu trabajo.** (con el
    > icono de energía)

    ("apagar este equipo" o "cerrar tu sesión" según la orden), con el botón **"Posponer 5
    minutos"** y "(2 veces más)". Es la primera de las bandas de transparencia. La cuenta dura 30 s si el profesor no elige otra (10 s a 10 min). La barra
    de la ventana agrega "· reinicio pendiente" (o "apagado" o "cierre de sesión"), también sin
    conexión.
27. **Posponer.** Cada aplazamiento suma 5 minutos a lo que falta, como máximo 2 veces por orden.
    La consola ve cada aplazamiento. Agotados, el botón se deshabilita con "Ya no se puede
    posponer". Aula no cierra nada durante la cuenta.
28. **Forzar es explícito y se advierte.** Con `force` no hay cuenta: el alumno ve "<profesor>
    (canal <X>) está apagando este equipo" mientras Windows lo apaga, y las aplicaciones se cierran
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

## Aula simulada (consola del profesor)

Alumnos de prueba que la consola crea dentro de sí misma para probar Aula sin laboratorio
(`internal/simclass`).

32. **Un simulado nunca pasa por real.** Lleva siempre la insignia «Simulado» y su propio conteo,
    aparte de los conectados. Sus acuses se cuentan aparte (`simSent`, `simAcked`), y cada línea
    de log que lo menciona lleva `simulated=true`. Ser simulado depende del puerto por el que
    llegó, nunca de su nombre: un equipo real llamado «Simulado 1» sigue siendo real.
33. **No toca a los alumnos reales.** Los simulados se conectan por un puerto de `127.0.0.1` que
    no se anuncia por mDNS; ningún equipo del laboratorio lo alcanza. Hablan el protocolo real,
    sin mensajes nuevos.
34. **No reciben lo que tiene efecto en un equipo.** Ni energía, ni restricciones, ni la
    transmisión, ni el control remoto: un simulado no puede declararlo, y la consola no se lo
    envía. Así nadie cree que funcionó en un equipo real cuando solo fue una prueba. La captura
    de pantalla sí les llega, porque no cambia ningún equipo: es su pantalla dibujada, que dice
    «alumno simulado», y se guarda aparte (regla 46).
35. **No guarda nada.** Los simulados viven en memoria y desaparecen al cerrar el aula simulada,
    al cambiar de canal o de nombre y al cerrar la clase. Su «pantalla» se dibuja en memoria y,
    como la de un equipo real, muestra «Tu pantalla está siendo vista por el profesor» mientras
    se ve.

## Control remoto (v0.6, Windows)

Reglas para `remote.*` (`docs/protocol.md` §5.16), aprobadas el 2026-11-05. Es la función más
intrusiva de Aula: el profesor ve la pantalla de un alumno y maneja su teclado y su mouse para
ayudarlo. Por eso el alumno lo ve siempre, desde antes del primer evento hasta el final.

36. **Sin consentimiento, con aviso inmediato.** El control no le pide permiso al alumno: es una
    acción del profesor sobre un equipo del centro de cómputo, con la misma autoridad que apagarlo
    o restringirlo, y solo la acepta el agente de una consola autenticada con la clave del centro
    (regla 29, `docs/protocol.md` §10). Al llegar `remote.start`, y **antes** de aceptar el primer
    evento, la ventana de Aula pasa al frente, sale del modo compacto y muestra la banda de la
    regla 37. Se decidió no pedir consentimiento para que el profesor pueda ayudar a un alumno que
    no sabe responder o no está frente al equipo; la transparencia no depende de esa respuesta.
37. **Banda permanente.** Mientras dura, la ventana muestra, después de la banda de energía y
    antes de la de pantalla vista:

    > **<profesor> (canal <X>) está controlando tu equipo: el mouse y el teclado se mueven solos
    > mientras te ayuda.** (con el icono de control)

    En pausa (regla 40) dice además «Su control está en pausa mientras usas tu mouse o tu
    teclado». La banda **no tiene botón para cerrarse** ni se oculta con el teclado; en el modo
    compacto se ve la insignia «Controlado». La barra de la ventana dice **"Aula — <profesor>
    controla este equipo (canal <X>)"**.
38. **Aviso fijo aunque la ventana esté cerrada.** El distintivo nativo de la regla 6 (siempre
    encima, en la parte superior del monitor principal, sin tomar clics ni el foco) dice
    **"● El profesor está controlando tu equipo"** mientras dure el control. Como el profesor ve
    la pantalla, la banda «Tu pantalla la está viendo…» también aparece.
39. **El alumno no corta el control**, por la misma autoridad de la regla 36: no hay botón ni
    atajo para terminarlo desde Aula. Lo termina el profesor («Soltar»), el fin de la clase, la
    desconexión o la inactividad (regla 41).
40. **El alumno tiene prioridad sobre el mouse y el teclado.** Si mueve su mouse o pulsa una
    tecla, la entrada del profesor se descarta (pausa) hasta 2 s después de su último movimiento,
    y la consola ve «El alumno está usando su equipo». Para saberlo, Aula **no instala hooks de
    teclado ni registra teclas**: compara la hora de la última entrada del sistema y la posición
    del puntero con lo que inyectó. Sabe *que* el alumno usó su equipo, nunca *qué* tecleó.
41. **Uno a la vez y fin evidente.** Una consola controla un solo equipo a la vez. Al soltar, al
    cerrar la clase, al perderse la conexión (no se reanuda al reconectar) o tras 5 minutos sin
    entrada del profesor, el agente suelta toda tecla y botón que presionó, deja de inyectar,
    quita la banda y el aviso fijo, y la ventana avisa en una esquina, sin quitar el foco:
    «<profesor> dejó de controlar tu equipo. El mouse y el teclado son solo tuyos otra vez.»
42. **Nunca a simulados, sin grabación.** Los alumnos simulados no declaran `remote` (regla 34).
    Las imágenes del control viven solo en memoria, como las miniaturas (regla 7).
43. **Registros.** Se registran el inicio, el fin y su motivo, las pausas, el equipo, el
    `control_id` y cuántos eventos se inyectaron o descartaron. **Nunca** qué teclas se enviaron,
    las coordenadas, ni la entrada del alumno.

Permisos del SO (detalle en `docs/protocol.md` §5.16.1): en Windows, `SendInput` desde la sesión
del alumno (en modo servicio, el ayudante, sin elevar), sin privilegios especiales; Windows no deja
controlar la pantalla de bloqueo, el aviso de UAC ni las ventanas de administrador. En macOS hará
falta el permiso de **Accesibilidad** y el de **Grabación de pantalla**; en Linux, X11 (XTest);
en **Wayland** la inyección de entrada puede no ser posible sin que el alumno la apruebe en cada
sesión (portal RemoteDesktop). Linux y macOS no declaran `remote` hasta que se documente aquí su
aviso.

## Captura de pantalla de un alumno (v0.7, Windows)

Reglas para `screen.snapshot.*` (`docs/protocol.md` §5.17), aprobadas el 2026-11-05. El profesor
toma **una** foto fija de la pantalla de un alumno, a resolución completa, para revisarla o dejar
evidencia. Es la única función de Aula que guarda una imagen del alumno en disco (regla 7), y por
eso el alumno lo sabe en el momento y el profesor decide, una por una, qué se guarda.

44. **El alumno sabe que se le tomó una captura, en el momento.** Al llegar la petición, y
    **antes** de capturar, el distintivo nativo de la regla 6 (siempre encima, sin tomar clics ni
    el foco) dice durante 5 segundos:

    > **● Tu profesor está tomando una captura de tu pantalla**

    Por eso el distintivo sale en la propia imagen: el profesor ve lo mismo que sabe el alumno. Si
    el distintivo no se puede mostrar, no se captura. Si el profesor ya está viendo la pantalla
    (miniaturas, regla 6) o la controla (reglas 37–38), el distintivo ya está a la vista y **no
    cambia de texto**: quitar el aviso de control, aunque sea un momento, debilitaría la regla 38.

    Después de capturar, la ventana de Aula muestra un aviso en la esquina, de nivel «Importante»:

    > **Captura de pantalla** · <profesor> (canal <X>) tomó una captura de tu pantalla (10:42).

    No es una banda (es un hecho puntual, no algo que sigue activo): se cierra con «Entendido» o
    Esc. Cuenta como «sin ver» en el título y, con la ventana cerrada, marca el icono del área de
    notificación. **No actúa sobre el equipo**, así que no trae la ventana al frente ni le quita el
    foco al alumno (regla 4). Durante el control remoto la imagen sale con la banda de control,
    que es lo que el alumno estaba viendo.
45. **Guardar es explícito.** La imagen llega a la memoria de la consola y se muestra en una vista
    previa. Solo el botón **«Guardar»** la escribe en disco; «Descartar», cerrar la vista previa,
    cerrar la clase o cambiar de canal la borran de la memoria. No hay guardado automático,
    periódico ni oculto. En lote, el profesor ve una miniatura de cada una y elige cuáles guardar.
46. **Dónde se guarda, y que el profesor lo vea.** En la carpeta Documentos del profesor:
    `Documentos\Aula\capturas\<equipo> (<usuario>)\<AAAA-MM-DD>\<HH-MM-SS>.jpg`, una carpeta por
    alumno y por día dentro de `capturas`, que agrupa todas. Las de un alumno simulado van en
    `… (simulado)`, para que una prueba nunca se mezcle con evidencia real. Al guardar, la consola
    muestra la ruta con «Abrir carpeta», y la carpeta está siempre en Ajustes › Acerca de.
47. **Nunca en el equipo del alumno.** El agente captura en memoria, envía la imagen en partes y
    la libera; no escribe archivos temporales. Si la conexión se cierra a mitad, la imagen se
    descarta en los dos extremos y no se reanuda.
48. **Registros.** Se registran la petición, el `snapshot_id`, las partes, el tamaño, las
    dimensiones, el resultado y que se guardó. **Nunca** la imagen ni la ruta del archivo (que
    nombra al alumno).
49. **Las capturas son del profesor, y las borra él.** Aula no las sube, no las comparte y no las
    borra: quedan en la carpeta personal del profesor. El manual recomienda borrarlas cuando ya
    no se necesiten.

Permisos del SO (detalle en `docs/protocol.md` §5.17.1): en Windows, la misma captura de las
miniaturas, desde la sesión del alumno (en modo servicio, el ayudante, sin elevar), sin
privilegios especiales; la pantalla de bloqueo y el aviso de UAC no se pueden capturar. En macOS
hará falta el permiso de **Grabación de pantalla**; en Linux, X11 lo permite sin permiso, y en
**Wayland** la captura pasa por el portal `Screenshot`, que pide la aprobación del alumno en cada
captura o sesión. Linux y macOS no declaran `screen.snapshot` hasta que se documente aquí su
aviso.

## Pendiente para etapas futuras

- Linux y macOS (incluida la recepción de la transmisión, las restricciones, el control remoto
  y la captura de pantalla): sus textos de aviso se agregarán aquí antes de implementarlos, con
  las mismas reglas 6–49.
