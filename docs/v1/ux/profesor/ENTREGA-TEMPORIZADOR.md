# Entrega: temporizador de ejercicio (prompt 10.8)

Estado: **para aprobación**. Diseño aprobado el 2026-11-05: `timer.start`, `timer.extend`,
`timer.stop` y `timer.ack` (`docs/protocol.md` §5.19), capacidad `timer`, paquete
`internal/classtimer`, regla 54 en `docs/privacidad.md`. Sin diagrama nuevo.

## Consola (`temporizador/`)

| Captura | Qué muestra |
|---|---|
| `87-panel-temporizador.png` | El panel de la barra de acciones con dos alumnos elegidos: nombre, 1/5/10/15/20 min o los minutos a mano, y la nota «no tapa su trabajo». |
| `88-temporizador-franja.png` | Tras «Iniciar»: la franja azul «Ejercicio 1 · 5:00 · … lo ven» con «Extender» y «Detener». |
| `89-temporizador-extender.png` | «Extender» desplegado: +1, +5 y +10 min en la misma franja. |
| `90-temporizador-termino.png` | Terminado: «Ejercicio 1 · terminó» con «Extender» y «Quitar». |
| `91-participacion-temporizador.png` | Participación › Temporizador: cuánto queda, para quién, quién aún no confirma y quién no puede verlo. |
| `92-temporizador-oscuro-900x600.png` | Oscuro a 900×600, con el panel abierto. |
| `93-temporizador-reemplaza.png` | El panel avisa que un temporizador nuevo reemplaza al que corre. |

## App del alumno (`../alumno/temporizador/`)

| Captura | Qué muestra |
|---|---|
| `59-temporizador.png` | La insignia «Ejercicio 1 · 4:32» en la tira, junto a la mano; el código sin tapar. |
| `60-temporizador-ultimo-minuto.png` | El último minuto en color de advertencia (sin parpadeo). |
| `61-temporizador-termino.png` | «Ejercicio 1 · terminó» y el aviso «Se acabó el tiempo» en la esquina. |
| `62-temporizador-oscuro.png` | Oscuro. |
| `63-temporizador-angosta.png` | A 480×360. |
| `64-compacto-temporizador.png` | En el modo compacto. |

## Verificado

- `node scripts/ux/recorrido.mjs` (93 capturas) y `node scripts/ux/recorrido-alumno.mjs`: sin
  errores de JS ni violaciones serias de axe (se corrigieron dos al revisar: etiquetas de «+1 min»
  distintas del texto visible y la franja fuera de un landmark).
- `node scripts/ux/pruebas-alumno.mjs`, con `temporizador` (en la tira, no es banda,
  `role="timer"`, cambia una vez por segundo sin animaciones, color en el último minuto, el fin en
  la esquina sin quitar el foco, se quita al detenerlo) y `temporizador-compacto`.
- `go test -race ./...`, `go vet`, `staticcheck` y `gofmt -l` limpios.

## Cambios visibles fuera de la función

- Icono nuevo `reloj`, `.franja--temporizador` y `.cuenta` (cifras de ancho fijo) en
  `internal/ui`.
- La ficha del alumno suma «Ver el temporizador» en «Qué puede hacer su equipo».

## Falta probar a mano

- C-63 a C-67 y A-18 y A-19 (`docs/desarrollo/probar-consola-local.md`).
- La prueba en VM, `docs/prueba-vms.md` §20 (TM-A, TM-B y TM-C para aceptar): dos alumnos con
  relojes desfasados, extender y detener, que no quite el foco, y la reconexión a media cuenta.
