# Entrega: encuestas en vivo (prompt 10.12)

Estado: **para aprobación**. Diseño aprobado el 2026-11-11:
- **Ubicación:** el panel «Encuesta» de la barra de acciones, para todos o para la selección.
  Mientras hay una encuesta abierta, una franja global con «Ver resultados» y «Cerrar encuesta».
  La sección «Encuestas» está en Participación.
- **Anonimato:** anónima por omisión. La casilla «Ver quién respondió qué» la hace nominal, y el
  alumno lo lee en su tarjeta antes de responder.
- **Voto:** uno por alumno y cuenta el último. Se puede cambiar hasta el cierre.

Más documentación:
- Protocolo: `docs/protocol.md` §5.23.
- Privacidad: `docs/privacidad.md`, reglas 71–74.
- Diagrama: `docs/v1/diagramas/20-encuestas.md`.

## Consola (`encuestas/`)

| Captura | Qué muestra |
|---|---|
| `127-panel-encuesta.webp` | El panel con dos alumnos seleccionados: la pregunta, «Opción múltiple» y tres opciones, cada una con su «✕». |
| `128-panel-encuesta-sino-nominal.webp` | Sin selección, «Sí o no» con su nota y «Ver quién respondió qué» marcada. Arriba, el aviso de que un equipo con un Aula anterior no la verá. |
| `129-panel-encuesta-sin-opciones.webp` | «Lanzar» sin opciones: el error va junto al campo y no se lanza nada. |
| `130-encuesta-franja.webp` | Después de «Lanzar»: la franja «Encuesta abierta: «…»», con el aviso «Encuesta lanzada» y su «Ver resultados». |
| `131-participacion-encuesta.webp` | La sección con la encuesta abierta y anónima. Cada barra lleva su texto, número y porcentaje, y la parte simulada va rayada y contada aparte. Debajo, quién no puede verla y las anteriores. |
| `132-participacion-encuesta-nominal.webp` | Una de respuesta corta nominal: quién respondió qué y quién aún no responde. |
| `133-encuesta-cerrada-compartir.webp` | Recién cerrada: queda desplegada en «Anteriores», con sus barras y «Compartir resultados». |
| `134-panel-encuesta-reemplaza.webp` | Con una encuesta abierta, el panel avisa que la nueva la cerrará. |
| `135-participacion-encuesta-oscuro-900x600.webp`, `136-panel-encuesta-oscuro-900x600.webp` | Modo oscuro a 900×600. |

## Alumno (`../alumno/encuestas/`)

| Captura | Qué muestra |
|---|---|
| `78-encuesta.webp` | Arriba de la lista, una encuesta abierta: la pregunta, «Elige una opción. Anónima: …», los radios y «Enviar». Debajo, una de sí o no cerrada, con los resultados de la clase y su respuesta marcada «(tu respuesta)». |
| `79-encuesta-respondida.webp` | Después de «Enviar»: «Respuesta enviada: 1. Puedes cambiarla mientras siga abierta.», y el botón dice «Cambiar mi respuesta». |
| `80-encuesta-texto.webp` | Una de respuesta corta nominal: «Tu profesor verá tu respuesta con tu nombre.» y el contador de 140. |
| `81-encuesta-oscuro.webp`, `82-encuesta-angosta.webp` | Modo oscuro y la ventana mínima (480×360). |
| `83-compacto-encuesta-nueva.webp` | En modo compacto, la encuesta que llega cuenta como «1 nuevo». |

## Lo que se prueba a mano

- **Ventana cerrada:** con la ventana del alumno cerrada al área de notificación, solo se marca el
  icono.
- **Foco:** el editor del alumno conserva el foco cuando llega una encuesta.
- **Modo servicio:** en modo servicio, «Enviar» pasa por el ayudante.

Son los casos A-28 a A-30 y §24 de `docs/prueba-vms.md`.
