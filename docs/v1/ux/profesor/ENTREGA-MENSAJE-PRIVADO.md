# Entrega: mensaje privado a un alumno (prompt 10.7)

Estado: **para aprobación**. Diseño aprobado el 2026-11-05 como **mejora del aviso**: `msg.show`
con el campo opcional `personal`, sin `msg.direct`. Regla 53 en `docs/privacidad.md`; protocolo
en `docs/protocol.md` §5.4.

## Consola (`mensaje-privado/`)

| Captura | Qué muestra |
|---|---|
| `81-privado-escribiendo.png` | La ficha: «Mensaje privado» abierto, «Mensaje solo para LAB3-PC07», Información/Advertencia y la ayuda (Enter, Shift+Enter, Esc). |
| `82-privado-enviado.png` | Tras Enter: «Mensaje privado enviado a LAB3-PC07» y, bajo el botón, «Enviado a LAB3-PC07 · ya lo vio». |
| `83-privado-ultimos-envios.png` | Código › Últimos envíos: «Mensaje privado a LAB3-PC07 · 1 de 1», **sin el texto**; el aviso a todos sí lo lleva. |
| `84-privado-oscuro-900x600.png` | Oscuro a 900×600. |
| `85-privado-durante-control.png` | En la vista del control remoto, «Mensaje privado» junto a «Tomar captura»: el campo arriba de su pantalla («Lo que escribes aquí no le llega a su equipo»). El recorrido comprueba que ninguna tecla salió por `SendRemoteInput`. |
| `86-privado-durante-control-enviado.png` | Enviado: el acuse en la vista y el control sigue. |

## App del alumno (`../alumno/mensaje-privado/`)

| Captura | Qué muestra |
|---|---|
| `57-aviso-privado.png` | En la esquina, un «Aviso» a todos y un «Mensaje para ti» con el icono del sobre; no son bandas. |
| `58-aviso-privado-oscuro.png` | Oscuro. |

## Verificado

- `node scripts/ux/recorrido.mjs` (86 capturas) y `node scripts/ux/recorrido-alumno.mjs`
  (58): sin errores de JS ni violaciones serias de axe.
- `node scripts/ux/pruebas-alumno.mjs`: 39 casos, con `aviso-privado` (título e icono propios,
  distinto del aviso a todos, no es banda, no quita el foco).
- `go test -race ./...`, `go vet`, `staticcheck` y `gofmt -l` limpios.

## Cambios visibles fuera de la función

- «Mensaje privado» también en la pantalla en grande y en el control remoto (agregado del usuario
  tras el diseño): el formulario es uno solo (`formPrivado`) para la ficha y la vista.
- «Responder» de Participación ahora va `personal` (sigue titulándose «Respuesta a tu mensaje»).
- Icono nuevo `sobre` y prueba nueva en `internal/ui` de que cada icono es un `<symbol>` del
  sprite y existe para las apps.

## Falta probar a mano

- C-59 a C-62 y A-17 (`docs/desarrollo/probar-consola-local.md`).
- La prueba en VM, `docs/prueba-vms.md` §19 (PM-A, PM-B y PM-C para aceptar, y PM-E durante el
  control): que la otra VM no reciba nada, el foco real del alumno y que lo escrito no llegue a su
  equipo mientras lo controlas.
