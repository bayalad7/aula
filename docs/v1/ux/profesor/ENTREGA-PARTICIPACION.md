# Entrega: manos y mensajes del alumno (prompt 10.6)

Estado: **para aprobación**. Diseño aprobado el 2026-11-05, con un cambio: la vista se llama
**«Participación»** (no «Clase», que ya es el grupo del menú). Reglas 50–52 en
`docs/privacidad.md`; protocolo en `docs/protocol.md` §5.18; diagrama 18.

## Consola (`participacion/`)

| Captura | Qué muestra |
|---|---|
| `72-participacion.png` | La vista: «2 manos levantadas» en orden (con su número y el motivo), mensajes «Nuevo» y «Respondido» con tu respuesta; «Participación 3» en el menú. |
| `73-participacion-insignias-alumnos.png` | En Alumnos › Notas: «Mano levantada · 1/· 2» y «1 mensaje», que llevan a Participación. |
| `74-participacion-respondiendo.png` | «Responder» abre el campo en la tarjeta: «Solo la ve este alumno. Enter envía; Shift+Enter, otra línea; Esc cancela.» |
| `75-participacion-respondido.png` | Tras Enter: «Respuesta enviada a lab-ubuntu-02»; el mensaje pasa a «Respondido» y el contador baja a 2. |
| `76-participacion-llega-mano.png` | Aviso flotante «lab-ubuntu-02 levantó la mano» con «Ver en Participación» (sin el texto) y la insignia nueva. |
| `77-participacion-vacia.png` | Sin manos ni mensajes: el estado vacío. |
| `78-participacion-oscuro-900x600.png` | Oscuro a 900×600: el menú de iconos con el contador encima. |
| `79-participacion-ficha.png` | La ficha: «Levantar la mano: Sí» y «Escribirte mensajes: Sí». |
| `80-simulada-mano-y-mensaje.png` | Aula simulada: «Levantar la mano/Bajar la mano» y «Escribir» en la fila; la insignia «Mano levantada». |

## App del alumno (`../alumno/participacion/`)

| Captura | Qué muestra |
|---|---|
| `45-mano-levantada.png` | «Bajar la mano» pulsado en la tira y la línea «Tu mano está levantada… Motivo: …» con «Cambiar el motivo». No es una banda. |
| `46-mano-motivo.png` | El campo del motivo (Enter lo envía, Esc cancela). |
| `47-mano-bajada-por-profesor.png` | «Tu profesor bajó tu mano.», sin color de alerta. |
| `48-mensaje-escribiendo.png` | La tarjeta «Mensaje para Prof. Ayala» arriba de la lista, con contador e indicaciones. |
| `49-mensaje-enviado.png` | «Tus mensajes»: «Recibido»; el foco volvió a «Mensaje al profesor». |
| `50-mensaje-vacio.png` | Enter con el campo vacío: «Escribe tu mensaje antes de enviarlo.» |
| `51-respuesta-del-profesor.png` | En la esquina, «Respuesta a tu mensaje» con «Tú escribiste: «…»». |
| `52-sin-conexion-botones.png` | Buscando la clase: los dos botones desactivados, con su motivo en el `title`. |
| `53-consola-sin-soporte.png` | Con una consola anterior: lo dice y desactiva los botones. |
| `54-mano-oscuro.png`, `55-mano-angosta.png` | Oscuro y a 480×360. |
| `56-compacto-mano.png` | En el modo compacto: la mano (pulsada) y el mensaje como botones de icono. |

## Verificado

- `node scripts/ux/recorrido.mjs` (80 capturas) y `node scripts/ux/recorrido-alumno.mjs`
  (56 capturas): sin errores de JS ni violaciones serias de axe.
- `node scripts/ux/pruebas-alumno.mjs`: 38 casos, 8 nuevos (levantar y bajar sin perder el foco
  ni mostrar banda, mano bajada por el profesor, motivo, Enter/Esc/vacío, respuesta con cita, sin
  conexión, compacto).
- `node scripts/ux/capturas-manual.mjs`: `docs/v1/administracion/img/participacion.png`.
- `gofmt -l`, `go vet`, `staticcheck` limpios; `go test -race ./...` en verde salvo dos paquetes
  que esta función no toca (ver abajo).

## Cambios visibles fuera de la función

- Menú: «Participación» en el grupo Clase, con contador (`.nav-lateral__contador`, nuevo en
  `internal/ui`). Iconos nuevos `mano`, `mensaje` y `participacion`.
- Las insignias de Notas pueden ser enlaces (`a.insignia`).
- `toast()` de la consola acepta un enlace a una vista («Ver en Participación»).

## Falta probar a mano

- C-54 a C-58 y A-14 a A-16 (`docs/desarrollo/probar-consola-local.md`).
- La prueba en VM, `docs/prueba-vms.md` §18 (HM-A, HM-B y HM-C para aceptar): el foco real del
  alumno, el modo servicio y la reconexión.
