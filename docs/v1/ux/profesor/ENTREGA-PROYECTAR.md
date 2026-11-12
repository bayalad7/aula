# Entrega: proyectar la pantalla de un alumno (prompt 12.1)

Estado: **para aprobación**. Diseño aprobado el 2026-11-11, con un cambio el mismo día:
- **Ubicación:** «Proyectar a la clase» está en la Ficha del alumno y en la Pantalla grande.
  Mientras dura, la franja global «Proyectando LAB3-PC07 · N de M viendo» tiene «Ver» y «Detener».
- **Sin consentimiento:** lo decide y lo termina el profesor, como el control remoto.
  - Al empezar, la ventana del alumno pasa al frente con la banda «Tu pantalla se está mostrando a
    la clase» y el distintivo nativo, antes del primer cuadro.
  - La banda no tiene botones.
  - El primer diseño pedía permiso y dejaba al alumno retirarse; se quitó por decisión del
    usuario.
- **La consola ve el video en vivo** en la Pantalla grande.
- **Una cosa a la vez:** proyectar a un alumno y mostrar la pantalla del profesor se reemplazan
  entre sí, con confirmación.
- **Transporte:** el video pasa por la consola; los alumnos no se conectan entre sí.

Más documentación:
- Protocolo: `docs/protocol.md` §5.24.
- Privacidad: `docs/privacidad.md`, reglas 75–82.
- Decisión: `docs/decisiones.md`, D-020.
- Diagrama: `docs/v1/diagramas/21-proyectar-alumno.md`.

## Consola (`proyectar/`)

| Captura | Qué muestra |
|---|---|
| `137-ficha-proyectar.webp` | La ficha de LAB3-PC07 con «Proyectar a la clase». La nota dice que el alumno lo verá y que solo tú la detienes. |
| `138-proyectar-conectando.webp` | Recién empezada: la franja «Proyectando LAB3-PC07 · conectando con su pantalla…» con «Ver» y «Detener». |
| `139-proyectando-franja.webp` | La franja «Proyectando LAB3-PC07 · 1 de 2 viendo». En la ficha, el botón dice «Detener». |
| `140-proyectando-ver.webp` | «Ver» abre la Pantalla grande «Proyectando LAB3-PC07» con el video y sus acciones. En el arnés el video dice «Conectando…». |
| `141-proyectar-fallo.webp` | El equipo del alumno no pudo capturar: el aviso dice por qué. |
| `142-proyectar-reemplaza-transmision.webp` | Proyectar a un alumno mientras transmites tu pantalla pregunta «¿Dejar de mostrar tu pantalla?». El «()» de la franja de fondo viene del doble del arnés (no se cargó el monitor), no de esta función. |
| `143-proyectando-oscuro-900x600.webp` | La Pantalla grande de la proyección en modo oscuro, a 900×600. |
| `144-pantalla-grande-proyectar.webp` | La acción en la Pantalla grande de Miniaturas. |

## Alumno (`../alumno/proyectar/`)

| Captura | Qué muestra |
|---|---|
| `84-mostrando.webp` | La banda «Tu pantalla se está mostrando a la clase», sin botones. |
| `85-mostrando-oscuro.webp`, `86-mostrando-angosta.webp` | La banda en modo oscuro y a 480×360. |
| `87-mostrando-compacto.webp` | El modo compacto con la insignia «Mostrando a la clase». |
| `88-viendo-companero.webp` | Lo que ve un compañero: «Estás viendo la pantalla de LAB3-PC07 (canal 1)». |
| `34-todas-las-bandas.webp`, `35-todas-las-bandas-oscuro.webp` | Todas las bandas: la nueva queda después de la de pantalla vista y antes de la de restricciones. |

## Falta probar a mano

- En VM, `docs/prueba-vms.md` §25: para aceptar, PR-A, PR-B y PR-C, con las cifras de CPU y red de
  la fuente anotadas (PR-D).
- Los casos C-89 a C-94 y A-31 a A-34 de `docs/desarrollo/probar-consola-local.md`.
- Lo que el arnés no ve:
  - el video real;
  - el distintivo nativo;
  - que la ventana de Aula quede fuera de la captura;
  - que la ventana pase al frente.
