---
description: Agrega una función nueva a Aula siguiendo la plantilla 10 y la «Definición de terminado» de CLAUDE.md
argument-hint: <descripción o prompt (ej. 10.4)> [para: profesor|alumno|ambos] [destino: todos|selección|un alumno] [so: windows|linux|macos]
---

# /funcionalidad-nueva

Agrega a Aula la función que pide el usuario. Es la plantilla
`docs/v1/prompts/10-plantilla-nueva-funcion.md` convertida en proceso: si cambias una, cambia la
otra.

Lo que pidió el usuario: **$ARGUMENTS**

## 0. Entiende el pedido

Lee `CLAUDE.md` (sobre todo «Definición de terminado»), `docs/protocol.md`,
`docs/privacidad.md`, `docs/progreso.md` y `docs/eventos-ui.md`. Si el pedido nombra un prompt
(«10.4», «control remoto») o una ruta de `docs/v1/prompts/`, léelo: es la especificación, y lo
que diga el usuario en este comando manda sobre él.

Arma esta ficha con lo que dice el pedido y lo que deduces del código y la documentación.
**Pregunta solo lo que no puedas deducir y cambie el diseño**, en una sola tanda
(AskUserQuestion, máximo 4 preguntas, con tu recomendación primero):

| Parámetro | Qué es | Si no lo dice |
|---|---|---|
| **Nombre** | Corto, en español: «Control remoto» | Dedúcelo de la descripción |
| **Descripción** | Qué logra, contado desde quien la usa | Obligatorio: pregunta si falta |
| **Para quién** | `profesor` (la usa en la consola), `alumno` (la usa en su app) o `ambos` | Dedúcelo: si la dispara el profesor y la ve el alumno, es `ambos` |
| **Destino** | A quién llega: `todos` (el canal), `selección` (los marcados en Alumnos), `un alumno` (desde su ficha) o `ninguno` | Dedúcelo; si dudas, pregunta |
| **Observa o controla al alumno** | Ve su pantalla o actividad, o cambia su equipo | Dedúcelo; si es sí, la privacidad es crítica |
| **Alumnos simulados** | Si les llega. Lo que observa o cambia un equipo nunca les llega | No les llega si controla o cambia el equipo; si no, recomienda |
| **Sistemas** | Windows primero; Linux y macOS solo si el usuario lo pide | `windows` |
| **Permisos o servicio** | Si necesita el agente como servicio, accesibilidad, etc. | Dedúcelo de `internal/platform` |
| **Versión** | Función nueva compatible → sube `y` (SemVer); si rompe el protocolo o la configuración → `x` | Dedúcelo y dilo |

Muestra la ficha completa al usuario antes de diseñar.

## 1. Diseño (detente para que lo apruebe)

- **Protocolo:** mensajes nuevos (tipo, dirección, payload, respuesta, límites) y su capacidad,
  y en qué SO se declara. Van en `docs/protocol.md` antes que el código.
- **Privacidad:** si observa o controla, el aviso visible en el equipo del alumno, la regla en
  `docs/privacidad.md` y si pide consentimiento.
- **Consola, dentro de la navegación actual:**
  - si va a alumnos: un panel de la barra de acciones sobre la selección, o la ficha del alumno
    si es para uno;
  - nada de pantallas a pantalla completa nuevas;
  - un boceto en texto (ASCII) del panel o de la ficha.
- **App del alumno:** qué ve y qué aviso recibe.
- **Paquetes:** qué va en `internal/platform` (interfaz + `*_windows.go` + fallback «no
  soportado») y qué en `internal/*`. La regla de oro: nada de APIs del SO fuera de `platform`.
- **Errores:** los códigos nuevos (`errores.go`) y los del protocolo, si hay.
- **Manual y diagramas:** qué página o sección del manual cambia, y si hace falta un diagrama
  (un flujo nuevo, sí).
- **Riesgos y lo que queda fuera.**

Espera la aprobación. Si una parte exige cambiar el modelo de seguridad, no la diseñes: anótala
en `docs/v1/ux/PROPUESTAS.md`.

## 2. Implementación (cuando apruebe)

En pasos pequeños y en este orden:
1. Protocolo y sus pruebas.
2. Plataforma, con un doble de prueba; nunca inyectes entrada real ni toques el escritorio del
   usuario.
3. Lógica en `internal/*`.
4. Bindings y eventos Wails, con su documentación en `docs/eventos-ui.md`.
5. Interfaz: sistema de diseño de `sistema/`, textos en `textos.es-MX.js`, claro, oscuro y
   900×600.
6. App del alumno.

No toques paquetes ajenos a la función.

## 3. Pruebas y cierre

Recorre la «Definición de terminado» de `CLAUDE.md` punto por punto y di cuál se cumple y cómo.
Como mínimo:
- Pruebas de la lógica, de la plataforma con su doble y de los bindings; cada código de error
  con su texto y su enlace al manual.
- `scripts/ux/dobles.js` con los bindings nuevos y una escena y una captura en
  `scripts/ux/recorrido.mjs`.
- `node scripts/ux/recorrido.mjs` sin errores de JS ni violaciones serias.
- Si cambió una pantalla del manual, `node scripts/ux/capturas-manual.mjs`.
- La sección del manual en `docs/v1/administracion/` y el diagrama en `docs/v1/diagramas/`, si
  aplica.
- Los casos manuales en `docs/desarrollo/probar-consola-local.md`, y en VM si toca el SO.
- `gofmt -l .` sin salida, `go vet ./...`,
  `go run honnef.co/go/tools/cmd/staticcheck@v0.8.2 ./...` y `go test -race ./...`.
- `docs/progreso.md` y una nota de entrega en `docs/v1/ux/profesor/` con las capturas.

Al terminar, muestra un resumen, las capturas y lo que falta probar a mano, y espera la
aprobación. **No hagas commit ni push hasta que el usuario lo pida**; en `develop`, agrupado por
ámbito.
