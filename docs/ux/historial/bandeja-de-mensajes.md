# Bandeja de mensajes tipo chat (10.26): ubicación y decisiones

Estado: **implementado, sin commit** (14/11/2026). Prompt
`docs/prompts/10.26-bandeja-de-mensajes-tipo-chat.md`, trabajado con `/funcionalidad-mejora`
como mejora de 10.6 (manos y mensajes) y 10.7 (mensaje privado). Nace de la nota de interfaz 015.
No se versionan capturas: las del arnés son `scripts/ux/salida/72-79` y `214-220` (consola) y
`scripts/ux/salida-alumno/104-105` (alumno); el CI las sube. El mapa suma «Consola · Participación ›
Conversación» (`docs/ux/mapa/`) y el manual la captura `participacion-bandeja.webp`.

## Análisis

Antes, cada mensaje era una tarjeta suelta, del más nuevo al más viejo y mezclando alumnos. Cada
mensaje guardaba **una sola** respuesta (la segunda borraba la primera), y no se guardaba lo que el
profesor respondía a una mano ni lo que mandaba con «Mensaje privado». Con 30 alumnos escribiendo
no se podía seguir un hilo. El alumno veía sus últimos 3 mensajes y las respuestas solo como avisos
en la esquina.

## Opciones de ubicación presentadas

Dentro de Participación › Manos y mensajes, en la columna izquierda (nota 015):

1. **Bandeja → conversación con «Volver»** (recomendada, **elegida**): la conversación ocupa el
   lugar de la bandeja y Esc regresa a la misma fila. Funciona igual a 1366 px (la columna mide
   unos 550 px) y a 900×600 (una sola columna).
2. **Lado a lado** dentro de la sección: unos 270 px por panel, demasiado apretado para un chat.
3. **Panel lateral flotante**: tapa Encuestas y Temporizador.

## Decisiones

- **Qué entra en la conversación:** todo lo que es solo para ese alumno: sus mensajes, las
  respuestas, lo que se le respondió a la mano y los «Mensaje privado» de la Pantalla grande.
- **Historial:** hasta 50 líneas por conversación, en memoria. Al pasar de 50 se descarta lo más
  viejo que no espera respuesta, y nunca un mensaje que espera si hay otra línea que descartar. Se
  vacía al terminar o cerrar la clase y al cambiar de canal; no se guarda entre clases (eso sería
  una regla de privacidad nueva).
- **Por atender, sin leer y el menú:** una conversación está «Por atender» si tiene algún mensaje
  sin responder ni marcar. La insignia de la fila cuenta esos mensajes, los mismos que ya contaban
  el menú y la Lista de alumnos, así que esos números no cambian. Abrir una conversación no la
  marca.
- **Responder** manda `reply_to` con el último mensaje que espera, y ese mensaje y los anteriores
  que esperaban quedan respondidos. Sin pendientes, sale como mensaje privado.
- **Protocolo:** sin cambios. El agente arma su hilo con los `msg.show` `personal` y su `reply_to`.
- **Alumno:** «Tus mensajes» es el hilo, con las últimas 6 líneas (siempre con `notify`). Mientras
  su pantalla se proyecta a la clase, oculta el texto del profesor (regla 53): sin eso, una
  respuesta privada quedaba a la vista de todos.
- **Foco:** con el foco en la bandeja, un mensaje que llega actualiza las filas sin reordenarlas y
  avisa «N conversaciones nuevas esperan» con **Mostrar**. En la conversación, el cuadro de
  respuesta no se vuelve a pintar: no se pierde ni el foco ni lo escrito.
- **Componente nuevo:** `.hilo` y `.burbuja` en `internal/ui/componentes.css`, usados por las dos
  apps; no necesitó tokens nuevos (los pares de contraste ya existían).

## Resultados

- Go: `participation.Board` guarda una conversación por alumno (`Conversation`, `Entry`, `Said`,
  `DoneAll`); `participation.Desk` guarda las respuestas (`Reply`, `Heard`). Binding nuevo
  `MarkConversationDone`. Pruebas: el orden de la bandeja, sin leer y atendidas, el límite, que una
  copia ya entregada no cambia, terminar la clase vacía todo y que el texto nunca llega al log.
- Arnés: escenas con 30 conversaciones en claro, oscuro y 900×600; el teclado; un mensaje que
  llega con el foco en la lista y otro mientras se escribe; Esc regresa a la fila. Alumno: el hilo,
  que no quita el foco, y lo oculto al proyectar.
- Durante el arnés se encontró y corrigió un error: `visibles.map(claveDe)` le pasaba el índice a
  `claveDe` como identificador de mensaje, y la lista quedaba con una sola fila al llegar un
  mensaje con el foco dentro.

## Pendiente

- Casos C-139 a C-143 y A-59 a A-60 de `docs/desarrollo/probar-consola-local.md`.
- El aviso de la esquina del alumno («Respuesta a tu mensaje», «Mensaje para ti») sigue mostrando
  el texto aunque su pantalla se proyecte. Ya era así antes de este cambio; queda anotado para
  decidirlo aparte.
