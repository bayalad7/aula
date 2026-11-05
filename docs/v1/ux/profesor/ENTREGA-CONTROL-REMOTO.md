# Entrega: control remoto (prompt 10.4)

Estado: **para aprobación**. Diseño aprobado el 2026-11-05: sin consentimiento del alumno, sin
botón para que el alumno corte, pausa cuando el alumno usa su equipo, video por la vista en
grande y cerrar la vista no suelta. Detalle técnico en D-019; reglas 36–43 en
`docs/privacidad.md`.

## Consola (`control-remoto/`)

| Captura | Qué muestra |
|---|---|
| `54-ficha-tomar-control.png` | «Tomar control» en la ficha, con su nota, y «Control remoto: Sí» en «Qué puede hacer su equipo». |
| `55-control-vista.png` | `#pantalla-grande` en modo control: «Controlando LAB3-PC07», estado, «Soltar» y la nota de Ctrl+Alt+Fin; arriba, la franja global. |
| `56-control-franja.png` | La vista cerrada: la franja roja «Controlando LAB3-PC07 · Ver · Soltar» sigue en todas las vistas. |
| `57-control-pausa.png` | El alumno usa su equipo: estado en pausa en la vista y en la franja. |
| `58-control-oscuro-900x600.png` | Modo oscuro a 900×600. |
| `59-control-terminado.png` | Al soltar: aviso «Terminó el control de LAB3-PC07» y la ficha vuelve a «Tomar control». |
| `60-miniatura-grande-tomar-control.png` | Miniaturas, pantalla en grande: «Tomar control» arriba (agregado a pedido: ahí se ve si el alumno está en la actividad). |
| `61-miniatura-grande-controlando.png` | Tras pulsarlo, la misma vista pasa a modo control. |

## App del alumno (`../alumno/control-remoto/`)

| Captura | Qué muestra |
|---|---|
| `36-control.png` | Banda roja «Prof. Ayala (canal 1) está controlando tu equipo…» después de energía y antes de «pantalla vista», sin botón para cerrarla. |
| `37-control-pausa.png` | La misma banda en pausa: «Su control está en pausa mientras usas tu mouse o tu teclado». |
| `38-control-terminado.png` | Aviso en la esquina «… dejó de controlar tu equipo», sin quitar el foco. |
| `39-control-oscuro.png` | Modo oscuro. |
| `40-compacto-controlado.png` | Modo compacto con la insignia «Controlado». |
| `34-todas-las-bandas.png` | Todas las bandas en su orden fijo: energía, control, pantalla vista, restricciones. |

## Verificado

- `node scripts/ux/recorrido.mjs` (61 capturas) y `node scripts/ux/recorrido-alumno.mjs`
  (40 capturas): sin errores de JS ni violaciones serias de axe.
- `node scripts/ux/pruebas-alumno.mjs`: 29 casos, incluidos `banda-control` (aparece, no se
  cierra ni con Esc, se va al soltar, el aviso de fin no quita el foco), `compacto-control` y el
  orden de las bandas.

## Falta probar a mano

C-44 a C-48 y A-09 a A-11 (`docs/desarrollo/probar-consola-local.md`) y `docs/prueba-vms.md` §16
(RC-A, RC-B y RC-C para aceptar): mouse y teclado reales, el aviso fijo nativo, la latencia y que
al soltar nada quede presionado.
