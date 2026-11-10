# Entrega: abrir una página o una app (prompt 10.11)

Estado: **para aprobación**. Diseño aprobado el 2026-11-09:
- **Ubicación:** el panel «Abrir» de la barra de acciones, con la misma acción en la Ficha del alumno
  y en la Pantalla grande.
- **Lista de apps:** solo en la consola (Ajustes › Apps permitidas, con ejemplos), y el agente aplica
  reglas fijas, así que no hay nada que instalar en los equipos.

Más documentación:
- Protocolo: `docs/protocol.md` §5.22.
- Privacidad: `docs/privacidad.md`, reglas 66–70.
- Diagramas: 09 y 13.

## Consola (`abrir/`)

| Captura | Qué muestra |
|---|---|
| `117-panel-abrir-pagina.webp` | Panel con dos seleccionados, «Página web», y el aviso de que el filtro web de la clase bloquea `youtube.com` en un equipo. Abajo, «Últimas que abriste» con una app que no estaba instalada en un equipo. |
| `118-panel-abrir-app.webp` | «App permitida» con la lista de Ajustes y «Editar la lista». |
| `119-panel-abrir-enviado.webp` | Después de «Abrir»: el aviso «Se envió a 2 equipos», la dirección vacía y «Abierta en 2 de 2». |
| `120-panel-abrir-url-invalida.webp` | `file:///C:/Windows`: el error va junto al campo y no se envía nada. |
| `121-ficha-abrir.webp` | La ficha con «Abrir página o app» y su nota, y el formulario abierto debajo. |
| `122-pantalla-grande-abrir.webp` | La misma acción en la Pantalla grande. |
| `123-ajustes-apps-permitidas.webp` | Ajustes › Apps permitidas con los ejemplos y dónde se guardará. |
| `124-ajustes-apps-error.webp` | Una app con `cmd.exe`: «Una app de la lista no es válida» y no se guarda. |
| `125-panel-abrir-oscuro-900x600.webp`, `126-ajustes-apps-oscuro-900x600.webp` | Modo oscuro a 900×600. |

## Alumno (`../alumno/abrir/`)

| Captura | Qué muestra |
|---|---|
| `76-abierto.webp` | Dos avisos en la esquina, «Tu profesor abrió una página» y «Tu profesor abrió una app», que dicen qué se abrió, quién y a qué hora. La ventana no sale al frente: el foco lo tiene lo que se abrió. |
| `77-abierto-oscuro.webp` | Lo mismo en modo oscuro. |

## Lo que falta probar a mano

- Los casos C-79 a C-83, A-26 y A-27 de `docs/desarrollo/probar-consola-local.md`.
- `docs/prueba-vms.md` §23: para aceptar, OA-A (una página en 2 alumnos con su aviso) y OA-B (los
  rechazos).
- Lo que el arnés no ve, a ojo:
  - que el navegador o la app se abren al frente en la sesión del alumno, y como el alumno;
  - que Aula no sale del modo compacto.
