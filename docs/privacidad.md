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
   pantalla en pausa (reglas 9-10); pantalla vista (regla 6); tu pantalla mostrada a la clase
   (reglas 75-82); restricciones. La transmisión lleva la suya sobre el video. **Ninguna tiene botón para cerrarse ni se oculta con el teclado**: cada una desaparece
   sola cuando el profesor termina. Lo prueban `scripts/ux/pruebas-alumno.mjs` y, del lado de Go,
   `TestNoBindingTurnsOffTransparency` (ninguna función de la ventana las desactiva).
2. **Al cerrar la ventana, el agente no se detiene: sigue en segundo plano, a la vista.** Cerrar
   la ventana la oculta y deja el icono de Aula en el área de notificación de Windows («Mostrar
   iconos ocultos»), con el estado al pasar el puntero y «Abrir Aula». Mientras está oculta, el
   código y los avisos que llegan solo marcan el icono; la ventana no reaparece sola, salvo por
   una acción del profesor sobre el equipo (energía, restricciones, su transmisión o el control remoto, reglas 12, 20,
   26 y 36). El distintivo de pantalla vista (regla 6) no depende de la ventana: se ve igual. Donde no
   hay área de notificación (Linux y macOS por ahora), cerrar minimiza. **Excepción:** mientras el
   profesor transmite su pantalla en una ventana o en pantalla completa, cerrar no hace nada hasta
   que la transmisión termine (regla 12).
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

Reglas para `screen.thumb.*` (`docs/protocol.md` §5.6–5.8) y para la pantalla en blanco
(«Pausar pantallas», `screen.freeze`, §5.9–5.11: reglas 9 a 11, aprobadas el 2026-11-12 con el
prompt 10.21).

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
9. **La pantalla en blanco explica por qué y quién.** No pide consentimiento: es una acción del
   profesor sobre un equipo del centro, como la energía (regla 29). Al llegar `screen.freeze`, una
   ventana nativa cubre **cada monitor** con:

   > **Tu profesor pausó esta computadora**
   > *«<mensaje del profesor, si lo hay>»*
   > <profesor> · canal <X>. Tu trabajo sigue abierto y nada se perdió: tu equipo vuelve cuando
   > el profesor lo reanude.
   > Si se pierde la conexión con la clase, la pausa se quita sola.
   > *<la hora, que avanza: no parece un cuelgue>*

   Tapa todos los monitores, recibe los clics y vuelve a quedar encima cada medio segundo; toma el
   foco al aparecer y cuando el alumno le hace clic. Aula **no intercepta ni registra el
   teclado** (D-013): no es un bloqueo inviolable, y cerrar Aula la quita (la consola lo ve como
   desconexión). Además, la ventana de Aula pasa al frente y sale del modo compacto (`raise`) con
   la banda **«<profesor> (canal <X>) pausó esta computadora»**, sin botón para cerrarse, entre
   la del control remoto y la de pantalla vista; en el modo compacto, la insignia «En pausa». La
   barra de la ventana dice **"Aula — <profesor> pausó esta computadora (canal <X>)"**.

   **Lo que el alumno debe ver va encima de la pausa:** la transmisión del profesor o de un
   compañero (regla 12), la cuenta regresiva de energía (regla 26) y el aviso del control remoto
   (regla 36). Mientras duran, la pausa queda debajo de la ventana de Aula y vuelve a cubrir todo
   al terminar. La consola quita la pausa de un alumno antes de tomar su control o de mostrar su
   pantalla a la clase, y no pausa al alumno que se está mostrando.
10. **Nadie queda bloqueado si se pierde la clase.** Si la conexión con la consola se cierra
    (la consola se cierra, se cae la red), el agente quita la pantalla en blanco y el
    distintivo de inmediato. Como servicio, el ayudante también la quita si pierde el canal local
    con el servicio, y sus ventanas se van con él si se cierra. Al volver a conectarse, el alumno
    entra sin pausa: la consola no la reenvía sola.
11. **Registros.** No se registran las imágenes ni el mensaje de la pantalla en blanco; sí el
    tipo de mensaje, `seq`, tamaños y dimensiones, y que hubo una pausa (con la longitud del
    mensaje, nunca su texto). Ningún extremo guarda una imagen de la pausa.

## Transmisión de la pantalla del profesor (v0.4, Windows)

Reglas para `screen.broadcast.*` (`docs/protocol.md` §5.12), aprobadas el 2026-10-29.

12. **El alumno sabe que está recibiendo, y de quién.** Al llegar `screen.broadcast.start`, y
    **antes** de aceptar la conexión de video, la ventana de Aula se trae al frente (sale del
    área de notificación y del modo compacto) y cambia a la vista de transmisión, con una franja
    fija arriba del video:

    > **Estás viendo la pantalla de <profesor> (canal <X>)** (con su icono)

    La barra de la ventana y la barra de tareas cambian a:

    > **Aula — viendo la pantalla de <profesor> (canal <X>)**

    Mientras se conecta o reconecta, la vista dice "Conectando con la pantalla del profesor…" en
    lugar del video. El botón **Tamaño real** muestra el video píxel a píxel, con barras de
    desplazamiento, si la pantalla del alumno es más chica que la del profesor. La franja no se
    puede ocultar desde Aula. El profesor elige en el panel cómo se muestra (v0.15,
    `docs/protocol.md` §5.12.1), y la franja lo dice:

    - **En una ventana:** queda encima de las demás ventanas. El alumno la puede mover y cambiarle
      el tamaño, pero no minimizarla ni cerrarla.
    - **En pantalla completa:** cubre el monitor del alumno, encima de todo, sin bordes, y el
      alumno no la puede minimizar ni cerrar. Es para presentaciones (PowerPoint, Canva…). Es la
      decisión del profesor.

    En los dos modos, cerrar la ventana (la X, Alt+F4) no hace nada mientras dura la
    transmisión: es la **única excepción a la regla 2**. Tampoco se encoge al modo compacto. Lo que
    sí sigue funcionando es lo del sistema: Ctrl+Alt+Supr, cambiar de usuario y cerrar sesión. Aula
    no lo bloquea.

    **Nadie queda bloqueado si se pierde la clase (como la regla 10).** El modo dura solo lo que
    dura la transmisión en esta conexión: se libera solo con «Detener», al cerrar la clase o
    cambiar de canal, al cortarse la conexión y cuando la consola deja de responder (los 15 s sin
    mensajes que marcan la desconexión, `docs/protocol.md` §3.4). Si no fuera así, el alumno no
    sabría por qué sigue bloqueado y podría perder el trabajo que tiene abierto. Al liberarse, la
    ventana de Aula **vuelve a como estaba** (su tamaño y lugar, el modo compacto u oculta en el
    área de notificación), sin cerrar ni tocar sus programas.

    Una consola anterior a v0.15 no manda el modo: la ventana se maximiza y el alumno la puede
    minimizar o redimensionar (no es un bloqueo: bloquear es la pantalla en blanco, reglas 9–10).
    El título de la barra de tareas sigue indicando la transmisión.
13. **Recibir no expone al alumno.** La conexión es solo de recepción: el agente rechaza ofertas
    con audio, canal de datos o envío de video, no usa cámara, micrófono ni captura de pantalla
    para esto, y no envía nada propio por WebRTC. Por eso **no** se muestra el distintivo "Tu
    pantalla está siendo vista" (regla 6), que significa otra cosa; si el profesor abre además
    las miniaturas, ese distintivo aparece como siempre.
14. **Termina con la clase.** La vista y su franja desaparecen con `screen.broadcast.stop`, al
    cerrarse la conexión con la consola (también cuando la consola deja de responder) o al cerrar
    el agente. Al terminar, la ventana vuelve a la vista de código y a como estaba antes de la
    transmisión (regla 12).
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

31 bis. **Encender (Wake-on-LAN, sin publicar, prompt 10.19).** El agente manda en `hello` la
    dirección física (MAC) de la tarjeta de red que lo lleva a la consola. La MAC identifica al
    **equipo**, no al alumno, y solo se guarda en la consola (`equipos-conocidos.json`, por
    canal): nombre del equipo, MAC, última IP y cuándo se vio; nunca el usuario ni lo que hizo.
    El equipo no guarda nada nuevo. «Olvidar» en la fila de un apagado borra su registro. El
    agente nunca manda UDP; la consola solo manda paquetes mágicos al broadcast de su red. No hay
    aviso en el equipo del alumno porque estaba apagado: al arrancar, el agente se conecta como
    siempre. Los alumnos simulados no tienen MAC y nunca se recuerdan.

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
    «alumno simulado», y se guarda aparte (regla 46). Tampoco cambia un equipo levantar la mano
    o escribir al profesor (reglas 50–52): un simulado lo hace solo cuando el profesor lo pulsa
    en su fila, y llega a Participación marcado «Simulado».
    Tampoco abrir una página o una app (regla 70): no les llega. Ni pausar su pantalla (regla 9):
    un simulado no declara `screen.freeze` y la consola no se lo envía.
    Recibir archivos tampoco: el simulado los acusa sin guardarlos (regla 59). Ni que se recoja
    su trabajo: entrega de memoria el último código que recibió, y la consola lo guarda aparte, en
    `… (simulado)` (regla 64).
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

## Manos y mensajes del alumno (v0.8, todos los sistemas)

El alumno levanta la mano (con un motivo breve opcional) o escribe un mensaje corto al profesor,
que lo ve en la vista «Participación» de la consola y le responde (`docs/protocol.md` §5.18). Lo
inicia siempre el alumno y no observa ni controla su equipo: no lleva banda ni aviso fijo, y
nada de esto trae su ventana al frente ni le quita el foco (prompt 19). Lo que llega del profesor
(la respuesta, «Tu profesor bajó tu mano») usa `notify`, como cualquier aviso.

50. **El contenido no se registra.** Los registros de la consola y del agente llevan `hand_id`,
    `message_id`, quién y la longitud; **nunca** el texto del mensaje, el motivo de la mano ni la
    respuesta del profesor. El aviso flotante de la consola dice quién levantó la mano o
    escribió, no qué escribió.
51. **Solo en memoria, solo durante la clase.** La consola guarda la fila de manos y los mensajes
    en memoria y los borra al cerrar la clase o cambiar de canal; lo de los alumnos simulados se
    borra al cerrar el aula simulada. Nadie los escribe en disco. Exportarlos será parte de la
    asistencia y bitácora (prompt 10.13), con su propia regla.
52. **Solo lo ve el profesor.** La mano y el mensaje de un alumno van solo a la consola; los demás
    alumnos nunca los ven. La respuesta va solo a ese alumno (`msg.show` con `reply_to`), y el
    acuse dice «llegó a la consola», no «el profesor lo leyó».

## Mensaje privado a un alumno (v0.8)

El profesor escribe a **un** alumno desde su ficha («Mensaje privado»), o le responde desde
Participación. Es un `msg.show` con `personal` (`docs/protocol.md` §5.4): no observa ni controla
el equipo, así que no lleva banda; la ventana lo muestra en la esquina como «Mensaje para ti»,
con `notify`, sin quitar el foco.

53. **Solo lo recibe su destinatario, y su texto no queda a la vista.** La consola lo manda a una
    sola sesión y nunca lo difunde; un agente anterior, que no conoce `personal`, lo muestra como
    un aviso normal, pero igual solo a él. Los registros llevan el equipo, el `id` y la longitud,
    **nunca** el texto (regla 5). En «Últimos envíos» la consola lo etiqueta «Mensaje privado a
    <equipo>», sin el texto, para que no quede a la vista si el profesor transmite su pantalla.

## Temporizador de ejercicio (v0.9, todos los sistemas)

El profesor da un tiempo con nombre a la clase o a una selección («Ejercicio 1 · 10 min») y cada
alumno ve cuánto falta en una insignia de su tira de identidad (`docs/protocol.md` §5.19). No
observa ni controla el equipo: no lleva banda ni aviso fijo, no tapa el trabajo del alumno y nunca
trae su ventana al frente ni le quita el foco. El fin usa `notify`: un aviso suave en la esquina y,
con la ventana cerrada, solo la marca en el icono del área de notificación.

54. **El temporizador solo informa.** Al llegar a cero no se bloquea, no se recoge ni se cambia
    nada en el equipo: el alumno sigue trabajando. Por eso también lo reciben los alumnos
    simulados (regla 34 no aplica). Los registros llevan `timer_id`, la duración y lo que falta,
    **nunca** el nombre que le puso el profesor (`label`) ni su descripción (`description`, lo
    que hay que hacer), que son texto suyo. La consola los guarda en memoria y los olvida al
    cerrar la clase o cambiar de canal. La descripción llega a una tarjeta de la lista con
    «Nuevo», sin quitar el foco ni traer la ventana, y se va con el temporizador.

## Archivos que envía el profesor (v0.10, Windows)

El profesor envía archivos o una plantilla de ejercicio a la clase o a una selección
(`docs/protocol.md` §5.20) y cada equipo los guarda en la **carpeta de trabajo** del alumno. Es
lo único que Aula escribe en el equipo del alumno. No observa ni controla: no lleva banda, no
trae la ventana al frente y no quita el foco; la tarjeta usa `notify`. Con la ventana cerrada,
solo se marca el icono del área de notificación.

55. **Solo en la carpeta de trabajo, y como el alumno.** La carpeta es `Documentos\Aula` del
    usuario con la sesión iniciada. Fuera de ella no se escribe nunca.
    - **Rutas.** La consola y el agente comprueban cada nombre y carpeta: nada de `..`, rutas
      absolutas, unidades, `:`, nombres reservados de Windows ni caracteres invisibles. El
      agente nunca corrige una ruta: la rechaza. El ayudante lo comprueba otra vez y la carpeta
      de trabajo no sigue enlaces ni *junctions*.
    - **Identidad.** En modo servicio escribe el ayudante, con el token del alumno: el archivo es
      suyo, nunca de SYSTEM, y un enlace no le da a Aula más permisos que los que ya tiene el
      alumno.
    - **Sin sesión.** Sin sesión iniciada no se declara `file.push` y no se escribe nada.
56. **El alumno ve qué llegó y dónde quedó.** Cada envío es una tarjeta en su lista, como el
    código: «Archivo recibido: datos.csv · Guardado en Documentos\Aula\Ejercicio 4», con
    «Abrir carpeta» y la insignia «Nuevo». La tarjeta también dice lo que no se pudo guardar y por
    qué (disco lleno, nombre no válido…). «Abrir carpeta» solo abre la carpeta de ese envío: la
    ventana no puede pedir otra ruta.
57. **Nunca se pisa el trabajo del alumno sin decirlo.** Si ya tiene un archivo con ese nombre, el
    nuevo se guarda como `nombre (2)` y el suyo no cambia. Solo con «Reemplazar si ya lo tienen»,
    que el profesor marca a propósito, se reemplaza, y entonces la tarjeta lo dice: «reemplazó al
    que tenías con ese nombre».
58. **Lo recibido no se abre ni se ejecuta, y no da acceso al equipo.** Aula guarda el archivo y
    nada más: el alumno decide si lo abre. Enviar archivos no le permite a la consola leer, listar
    ni borrar nada del alumno. Leer es la recolección (reglas 60–65), y solo en su carpeta.
59. **Registros y simulados.** Los registros llevan `push_id`, `file_id`, partes, tamaños y el
    resultado, **nunca** el contenido, los nombres de los archivos, la carpeta ni el nombre de la
    plantilla. Los alumnos simulados acusan los archivos sin escribir nada: su carpeta es una
    lista de nombres en memoria (regla 35). Las plantillas son carpetas en el equipo del
    profesor (`Documentos\Aula\plantillas`) y solo él las cambia.

## Recoger el trabajo de los alumnos (v0.11, Windows)

Reglas para `file.collect.*` (`docs/protocol.md` §5.21), aprobadas el 2026-11-06. El profesor
abre una recolección (fecha y actividad), cada equipo crea su carpeta y, cuando el profesor pulsa
«Recoger», la consola recibe lo que el alumno guardó ahí. Es la única función de Aula que **lee**
archivos del alumno, y por eso se limita a una carpeta acordada y el alumno sabe, antes y
después, qué se recoge. No controla el equipo: no lleva banda, no trae la ventana al frente ni
quita el foco (`notify`). Con la ventana cerrada, solo se marca el icono del área de
notificación.

60. **Una sola carpeta, acordada.** Solo se lee la carpeta de la actividad dentro de la carpeta
    de trabajo, `Documentos\Aula\recolecciones\<fecha>\<actividad>\`, y lo que hay dentro hasta 4
    niveles. **Nunca** otra carpeta ni el resto del disco.
    - **La consola no manda rutas.** Manda la fecha y el nombre de la actividad; el agente arma
      la carpeta bajo `recolecciones\` y rechaza un nombre que no cumpla las reglas de ruta
      (regla 55). No sigue enlaces ni *junctions*: los omite y lo dice.
    - **Como el alumno.** En modo servicio lee el ayudante, con el token del alumno: Aula no puede
      leer nada que el alumno no pueda leer. Sin sesión iniciada no se declara `file.collect`.
61. **El alumno lo sabe antes.** Al abrir la recolección, la ventana agrega una tarjeta con
    «Nuevo»:

    > **Recolección: Ejercicio 4** · Guarda tu ejercicio en
    > `Documentos\Aula\recolecciones\2026-11-06\Ejercicio 4\` · Se recogerán: `*.py`

    con «Abrir carpeta». Al pedir los archivos, y **antes** de leer nada, un aviso en la esquina:

    > El profesor solicitó recoger tu ejercicio en
    > `Documentos\Aula\recolecciones\2026-11-06\Ejercicio 4\`.
62. **El alumno ve qué se recogió y cuándo.** Al terminar, un aviso en la esquina:

    > Tu profesor recogió tu ejercicio: `20231234-Ana.py` (3 KB).

    (con varios archivos, «recogió 3 archivos (12 KB)»), y la tarjeta de la recolección anota,
    por ronda, la hora y cada archivo con su tamaño, y lo que **no** se recogió y por qué
    (demasiado grande, abierto en otro programa, solo en la nube). La lista queda a la vista
    durante toda la sesión. Si la carpeta estaba vacía, también se le dice: «No había archivos que
    recoger».
63. **Solo lee.** Recoger no borra, no mueve, no cambia ni ejecuta nada del alumno: su carpeta
    queda igual. Tampoco da acceso a nada más: la consola no puede listar otras carpetas ni pedir
    un archivo por ruta.
64. **Dónde lo guarda el profesor.** Al llegar, en su carpeta Documentos:
    `Documentos\Aula\recolecciones\<fecha>\<actividad>\<equipo> (<usuario>)\`, con un
    `aula-recoleccion.json` que dice de qué ronda y hora es cada archivo y su huella. Los de un
    simulado van en `… (simulado)`. La carpeta está en Ajustes › Acerca de. Recoger otra vez
    reemplaza la copia del profesor de ese alumno, nunca el original. Aula no los sube ni los
    comparte; el manual recomienda borrarlos cuando ya no se necesiten.
65. **Registros y simulados.** Los registros llevan `collect_id`, la ronda, `file_id`, partes,
    tamaños, cuántos archivos y el resultado; **nunca** el contenido, los nombres de los archivos,
    la actividad ni las carpetas. Los alumnos simulados no leen ningún disco: devuelven el último
    código que recibieron.

## Abrir una página o una app (v0.12, Windows)

Reglas para `open.*` (`docs/protocol.md` §5.22), aprobadas el 2026-11-09. El profesor abre una
página web en el navegador del alumno o una app de la lista de apps permitidas de la consola.
Es una **acción sobre el equipo**, pero puntual: no lleva banda, porque no queda nada activo.

66. **El alumno ve qué se abrió y quién lo pidió.** En cuanto se abre, un aviso en la esquina:

    > **Prof. Ruiz (canal 1) abrió `docs.python.org/3/tutorial` en tu navegador.**

    o «… abrió **Thonny**». De la URL se muestra el dominio y la ruta, sin la consulta
    (`?…`), recortada a 80 caracteres; el texto completo está en el aviso al pasar el mouse. El
    aviso se queda hasta que el alumno pulsa «Entendido». Con la ventana cerrada, el aviso espera
    a la próxima vez que la abra y se marca el icono del área de notificación. Si no se pudo
    abrir, no se abrió nada y el alumno no ve aviso.
67. **El foco lo toma lo que se abre, no Aula.** Es la excepción a «una acción sobre el equipo
    trae la ventana al frente» (`raise`): el navegador o la app ya pasan al frente por sí
    mismos, y traer Aula encima se los taparía. Por eso la ventana usa `notify`: no sale del
    modo compacto ni roba el foco a lo que se acaba de abrir.
68. **Nunca un comando.** La consola no puede ejecutar nada arbitrario: `open.url` solo abre
    direcciones `http`/`https` validadas (sin usuario ni contraseña), y `open.app` solo lanza,
    **sin argumentos**, un `.exe` ya instalado en `Program Files`, `Program Files (x86)`,
    `Windows` o `%LOCALAPPDATA%\Programs`, nunca en Documentos, Escritorio, Descargas, la carpeta
    temporal ni la carpeta de trabajo de Aula, y nunca un intérprete o herramienta del sistema
    (`cmd`, `powershell`, `wscript`, `mshta`, `rundll32`…). Esas reglas están compiladas en el
    agente: ninguna consola las cambia. Así, aunque alguien tuviera la clave del centro, solo
    podría abrir lo que el alumno ya podría abrir con un doble clic, y nunca algo que haya
    enviado con «Enviar archivos».
69. **Como el alumno, y sin vigilar.** Se abre en la sesión del alumno con sus permisos (en
    modo servicio, el ayudante; nunca como SYSTEM). Aula no sabe si el alumno se quedó en la
    página, no cierra lo que abrió y no mira el historial del navegador. El filtro web sigue
    mandando: un sitio bloqueado no carga, y la consola se lo advierte al profesor antes de
    enviar. Sin sesión iniciada no se declara `open.*`.
70. **Registros y simulados.** Los registros llevan `open_id`, el tipo, el resultado y el motivo,
    **nunca** la URL, el nombre de la app ni su ruta. Los alumnos simulados no declaran `open.*`
    (regla 34): no les llega.

## Encuestas en vivo (v0.13, todos los sistemas)

El profesor hace una pregunta rápida a la clase o a una selección (opción múltiple, sí o no o
respuesta corta) y ve las respuestas llegar en la vista «Participación» (`docs/protocol.md`
§5.23). No observa ni controla el equipo: no lleva banda ni aviso fijo, y responder es decisión
del alumno.

71. **Llega sin interrumpir.** La encuesta es una tarjeta arriba de la lista de la ventana, con
    «Nuevo» y `notify`: nunca un diálogo, nunca `raise`, nunca quita el foco. Con la ventana
    cerrada solo se marca el icono del área de notificación; en el modo compacto cuenta como
    «sin ver». El alumno puede no responder, y puede cambiar su respuesta hasta que se cierre.
72. **Anónima o nominal, y el alumno lo sabe antes.** Por omisión la encuesta es **anónima**: la
    consola solo muestra cuántos eligieron cada opción y cuántos respondieron; en una de respuesta
    corta, los textos sin nombre ni hora, en orden alfabético. El profesor puede marcar «Ver quién
    respondió qué» en cada encuesta (para saber a quién ayudar); entonces es **nominal** y ve la
    respuesta de cada alumno y quién no ha respondido. La tarjeta del alumno dice cuál de las dos
    es **antes** de que responda («Anónima: tu profesor ve el total, no tu respuesta» o «Tu
    profesor verá tu respuesta con tu nombre»). En una anónima la consola guarda en memoria a qué
    equipo corresponde cada voto, solo para contar uno por alumno y aceptar el reenvío tras una
    desconexión; ninguna pantalla lo muestra y nada lo escribe.
73. **Los compañeros solo ven el total, y solo si el profesor lo comparte.** Ningún alumno ve la
    respuesta de otro. «Compartir resultados» manda a quienes recibieron la encuesta **solo los
    totales** de una de opción múltiple o de sí o no, ya cerrada, aunque haya sido nominal; las
    respuestas cortas nunca se comparten. Los votos de los alumnos simulados se cuentan aparte y
    no entran en ese total (regla 32).
74. **Solo en memoria, sin texto en los registros.** La consola guarda las encuestas de la clase
    (como mucho 20) en memoria y las borra al cerrar la clase o cambiar de canal; lo de los
    alumnos simulados se borra al cerrar el aula simulada. Los registros de la consola y del
    agente llevan `poll_id`, el tipo, si es nominal, los estados y los totales; **nunca** la
    pregunta, las opciones ni el texto de una respuesta, y en una anónima tampoco qué eligió cada
    equipo. Exportar resultados será parte de la asistencia y bitácora (prompt 10.13), con su
    propia regla.

## Proyectar la pantalla de un alumno a la clase (v0.14, Windows)

Reglas para `student.present.*` (`docs/protocol.md` §5.24), aprobadas el 2026-11-11 (prompt
12.1). El profesor elige a un alumno y su pantalla se ve en grande en los equipos de sus
compañeros y en la consola. Expone a un alumno ante sus compañeros, no solo ante el profesor.
**Sin consentimiento** (decisión del usuario, 2026-11-11): como en el control remoto (regla 36),
el profesor tiene autoridad sobre los equipos del centro y decide cuándo empieza y cuándo
termina. A cambio, el aviso va **antes** del primer cuadro y no se puede ocultar.

75. **Sin consentimiento, con aviso inmediato.** No se le pregunta al alumno. Al llegar la orden, y
    **antes** de capturar nada, la ventana de Aula pasa al frente (`raise`: es una acción del
    profesor sobre el equipo), sale del modo compacto y muestra su banda (regla 76); el distintivo
    nativo aparece al mismo tiempo. Si el aviso no se puede mostrar, el agente **no publica** y la
    consola lo dice.
76. **Banda permanente y aviso fijo.** Mientras dura, la ventana muestra la banda **«Tu pantalla
    se está mostrando a la clase»**, en el orden de la regla 1 (después de la de pantalla vista),
    **sin ningún botón** y sin ocultarse con el teclado; en el modo compacto, la insignia
    «Mostrando a la clase». El distintivo nativo de la regla 6 (siempre encima, aunque la ventana
    esté cerrada o minimizada) dice lo mismo. El título de la ventana y de la barra de tareas
    cambia a «Aula — tu pantalla se está mostrando a la clase (canal X - profesor)».
77. **Solo el profesor la termina.** El alumno no puede cortarla ni negarse (como la regla 39 del
    control remoto): no hay botón ni binding de la ventana que la detenga
    (`TestNoBindingEndsAPresentation`). Termina con «Detener» en la consola o por las causas de
    la regla 81; al terminar, la banda y el distintivo desaparecen solos.
78. **Lo privado no se proyecta.** Las ventanas del agente (la de Aula y su distintivo) **se
    excluyen de la captura** (`WDA_EXCLUDEFROMCAPTURE`; en Windows anteriores a 10 2004 se ven
    negras): un mensaje privado (regla 53), una encuesta nominal o la tarjeta de un archivo nunca
    llegan a los compañeros. Si Windows no permite excluirlas, el agente **no publica**. No se
    capturan audio, cámara ni micrófono; solo el **monitor principal**.
79. **Uno a la vez, y nunca junto con la pantalla del profesor.** Una consola proyecta a un solo
    alumno; empezar con otro termina el primero en todos los equipos antes de preguntarle al
    segundo. Mostrar la pantalla del profesor (reglas 12–16) y proyectar a un alumno usan la misma
    vista de los compañeros: empezar una termina la otra, y la consola lo confirma con el profesor.
80. **Los compañeros saben de quién es.** Reciben la vista de la regla 12 con la franja **«Estás
    viendo la pantalla de LAB3-PC07 (canal X)»**: el nombre del equipo, no el del usuario. Un
    agente de una versión anterior, que diría «la pantalla del profesor», no la recibe. Recibir no
    los expone (regla 13): su conexión es solo de recepción.
81. **Termina en todos.** Con «Detener» en la consola, un fallo de la captura, al desconectarse el
    alumno (no se reanuda sola), al cambiar de canal o al cerrar la clase. Entonces la banda, el
    distintivo y la vista de los compañeros desaparecen. Si el agente pierde la conexión con la
    consola, deja de capturar y quita la banda por su cuenta. La consola muestra mientras dura la
    franja «Proyectando LAB3-PC07 · N de M viendo», con «Ver» y «Detener».
82. **Sin grabación, nunca a simulados.** Ni el agente, ni la consola, ni los compañeros graban el
    video ni escriben cuadros en disco; la consola reenvía los paquetes sin decodificarlos ni
    guardarlos. Los registros llevan `present_id`, intentos, estados, el equipo fuente y tamaños;
    nunca el SDP, los candidatos ni ningún cuadro. Los alumnos simulados no declaran
    `screen.present` ni `screen.present.view` (regla 34): ni se proyectan ni la reciben.

**Permisos del sistema operativo.** En Windows la captura no pide permiso; como servicio, el
agente no captura: lo hace su ayudante, en la sesión del alumno y con sus derechos (regla 17).
En Linux con Wayland la captura pasa por el portal de escritorio, que pide al alumno aprobar en
cada sesión; en macOS, el permiso «Grabación de pantalla». Linux y macOS no lo declaran todavía.

## Terminar la clase (sin publicar, todos los sistemas)

Al acabar la hora, el profesor pulsa «Terminar la clase» (bajo el Selector de clase del menú de
la consola): se detiene lo que sigue activo y la ventana de cada alumno queda como recién abierta
(`docs/protocol.md` §5.25). No observa ni controla: solo deshace.

83. **El alumno sabe qué se quitó y cuándo.** La ventana muestra una tarjeta en la lista, sin
    quitar el foco (`notify`, nunca `raise`):

    > **La clase terminó** · 10:52
    > Tu profesor terminó la clase: ya no hay restricciones, se quitó el temporizador y se
    > vaciaron el código y los avisos. Tus archivos en Documentos\Aula siguen ahí.

    Dice solo lo que había (restricciones, pausa, control, transmisión, temporizador, encuestas,
    código, avisos, archivos, recolecciones, la mano y los mensajes sin enviar). Cada banda de
    transparencia desaparece con lo que la causaba, como siempre; la tarjeta no es una banda y el
    alumno la cierra con «Entendido» (o se va sola cuando llega código nuevo).
84. **Nunca borra ni lee archivos.** Terminar la clase no borra, mueve, abre ni lee nada del
    alumno: lo que está en `Documentos\Aula` (archivos recibidos y recolecciones) se queda; solo
    se quitan las tarjetas de la ventana. En la consola tampoco se borra nada en disco (capturas
    guardadas, recolecciones, plantillas); se vacía lo que estaba en memoria. Borrar archivos al
    terminar queda como propuesta (`docs/ux/PROPUESTAS.md`). Si el profesor marca «Recoger el
    trabajo antes de terminar», aplica el aviso de la regla 61.
85. **La energía no se cancela sola.** Un apagado o reinicio programado sigue su cuenta regresiva
    (reglas 26–31): el profesor pudo programarlo a propósito. Solo se cancela si lo marca en el
    diálogo («Cancelar el apagado programado»), apagado por omisión.
86. **Registros y simulados.** Los registros llevan `end_id`, el estado, el motivo, el equipo y
    cuántas cosas se quitaron; nunca el código, los avisos ni los nombres de los archivos. Los
    alumnos simulados declaran `class.end` y limpian su ventana simulada: no cambia ningún equipo
    (regla 34 no aplica). Un equipo desconectado al terminar recibe el fin de clase al volver.

## Pendiente para etapas futuras

- Linux y macOS (incluida la recepción de la transmisión, las restricciones, el control remoto
  y la captura de pantalla): sus textos de aviso se agregarán aquí antes de implementarlos, con
  las mismas reglas 6–49.
