# Entrega: archivos recibidos (prompt 10.9)

Estado: **para aprobación**. La tarjeta va en la lista, como el código, con `notify` (nunca
`raise`), sin banda: no observa ni controla (`docs/privacidad.md` reglas 55–59).

| Captura | Qué muestra |
|---|---|
| `65-archivos-recibidos.png` | Un archivo suelto y una plantilla de 3: dónde quedaron y «Abrir carpeta». |
| `66-archivo-renombrado.png` | «Ya tenías datos.csv: el nuevo quedó como datos (2).csv. El tuyo no cambió.» |
| `67-archivo-fallo.png` | Un envío con un archivo sin guardar: «no se guardó. El disco está lleno.» |
| `68-archivos-oscuro.png`, `69-archivos-angosta.png` | Modo oscuro y 480×360. |
| `70-compacto-archivo-nuevo.png` | En modo compacto, el archivo nuevo suma a la insignia de nuevos. |

Comportamiento probado en `pruebas-alumno.mjs`: `archivo-recibido` (no quita el foco, no es banda,
«Nuevo» y título, «Abrir carpeta» pasa solo el id), `archivo-renombrado-y-fallo` y
`archivo-sin-carpeta`. A mano: A-20 a A-22 y §21 en VM.
