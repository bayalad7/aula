---
description: Agrega una función nueva a Aula siguiendo la plantilla 10 y la «Definición de terminado» de CLAUDE.md
argument-hint: <descripción o prompt (ej. 10.4)> [para: profesor|alumno|ambos] [destino: todos|selección|un alumno] [so: windows|linux|macos]
---

# /funcionalidad-nueva

Agrega a Aula la función que pide el usuario. Es la plantilla
`docs/prompts/10-plantilla-nueva-funcion.md` convertida en proceso: si cambias una, cambia la
otra.

Lo que pidió el usuario: **$ARGUMENTS**

## 0. Entiende el pedido

Lee `CLAUDE.md` (sobre todo «Definición de terminado»), `docs/protocol.md`,
`docs/privacidad.md`, `docs/progreso.md` y `docs/eventos-ui.md`. Si el pedido nombra un prompt
(«10.4», «control remoto») o una ruta de `docs/prompts/`, léelo: es la especificación, y su
sección **«Encaje con lo implementado»** dice dónde va en cada app, qué componentes usa y qué
diagramas toca. Lo que diga el usuario en este comando manda sobre el prompt.

Lee también los diagramas que nombre ese encaje (`docs/diagramas/`) y, para ver cómo se ven
hoy las apps, las capturas del arnés (`node scripts/ux/recorrido.mjs` y `recorrido-alumno.mjs`).
Copia sus patrones; no inventes otros.

Arma esta ficha con lo que dice el pedido y lo que deduces del código y la documentación.
**Pregunta solo lo que no puedas deducir y cambie el diseño**, en una sola tanda
(AskUserQuestion, máximo 4 preguntas, con tu recomendación primero):

| Parámetro | Qué es | Si no lo dice |
|---|---|---|
| **Nombre** | Corto, en español: «Control remoto» | Dedúcelo de la descripción |
| **Descripción** | Qué logra, contado desde quien la usa | Obligatorio: pregunta si falta |
| **Para quién** | `profesor` (la usa en la consola), `alumno` (la usa en su app) o `ambos` | Dedúcelo: si la dispara el profesor y la ve el alumno, es `ambos` |
| **Destino** | A quién llega: `todos` (el canal), `selección` (los marcados en Alumnos), `un alumno` (desde su ficha) o `ninguno` | Dedúcelo; si dudas, pregunta |
| **Dónde vive en la consola** | Panel de la barra de acciones, ficha del alumno, sección de la vista «Participación» (lo que responden los alumnos) y/o franja global (lo que sigue activo) | El encaje del prompt; si no hay, la «Definición de terminado» |
| **Dónde vive en el alumno** | Tarjeta en la lista, aviso en la esquina, botón o insignia en la tira de identidad, o banda de transparencia | El encaje del prompt |
| **¿Actúa sobre el equipo?** | Sí: usa `raise` (trae la ventana, sale del modo compacto) y lleva banda. No: usa `notify` (nunca quita el foco) | No, salvo que cambie o controle el equipo |
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
- **Ubicación (decisión del usuario, no tuya):** con los nombres de `docs/ux/MAPA.md`,
  propone **2–3 ubicaciones** posibles, cada una con su boceto en texto (ASCII), y di cuál
  recomiendas y por qué. Pregúntalo con AskUserQuestion usando `preview` para los bocetos. Aplica
  las reglas de «Ubicación y vocabulario» de `CLAUDE.md`:
  - función con su propio material de trabajo → módulo propio del menú (una de las opciones
    siempre lo considera);
  - acción sobre un alumno → en la Ficha del alumno **y** en la Pantalla grande;
  - lo que responden los alumnos → sección de «Participación»; lo que sigue activo → Franja global;
  - nada de pantallas a pantalla completa nuevas.
  El «Encaje con lo implementado» del prompt es una sugerencia, no una decisión: si choca con
  estas reglas, dilo y ofrécelo como una de las opciones.
- **Consola:** el boceto de la opción elegida, con el panel, la ficha o el módulo completo.
- **App del alumno:** qué ve y qué aviso recibe, con un boceto en texto. Con `notify` o `raise`
  (y por qué); si observa o controla, su banda y su lugar en el orden fijo; qué botones agrega a
  la tira; qué pasa en el modo compacto y con la ventana cerrada al área de notificación.
- **Modo servicio:** si la ventana o el SO del alumno intervienen (entrada, portapapeles,
  captura, archivos), cómo pasa por el ayudante (`internal/agentsvc`: llamada o evento nuevo).
- **Paquetes:** qué va en `internal/platform` (interfaz + `*_windows.go` + fallback «no
  soportado») y qué en `internal/*`. La regla de oro: nada de APIs del SO fuera de `platform`.
- **Errores:** los códigos nuevos (`errores.go`) y los del protocolo, si hay.
- **Manual y diagramas:** qué página o sección del manual cambia; qué diagramas existentes se
  actualizan y si hace falta uno nuevo (un flujo nuevo, sí), con su `pantallas`.
- **Riesgos y lo que queda fuera.**

Espera la aprobación. Si una parte exige cambiar el modelo de seguridad, no la diseñes: anótala
en `docs/ux/PROPUESTAS.md`.

## 2. Implementación (cuando apruebe)

En pasos pequeños y en este orden:
1. Protocolo y sus pruebas.
2. Plataforma, con un doble de prueba; nunca inyectes entrada real ni toques el escritorio del
   usuario.
3. Lógica en `internal/*`.
4. Bindings y eventos Wails, con su documentación en `docs/eventos-ui.md`.
5. Interfaz de la consola: sistema de diseño de `sistema/`, textos en `textos.es-MX.js`, claro,
   oscuro y 900×600.
6. App del alumno: componentes de `internal/ui` (si falta uno, se agrega ahí y al catálogo de
   `tools/vistaprevia`, no en la app), textos en su `textos.es-MX.js`, Go manda estados, y el
   ayudante del servicio si hace falta.

No toques paquetes ajenos a la función.

## 3. Pruebas y cierre

Recorre la «Definición de terminado» de `CLAUDE.md` punto por punto y di cuál se cumple y cómo.
Como mínimo:
- Pruebas de la lógica, de la plataforma con su doble y de los bindings; cada código de error
  con su texto y su enlace al manual.
- Consola: `scripts/ux/dobles.js` con los bindings nuevos, y una escena y una captura en
  `scripts/ux/recorrido.mjs`.
- Alumno: `scripts/ux/dobles-alumno.js`, una escena en `recorrido-alumno.mjs` y el comportamiento
  en `pruebas-alumno.mjs` (no quita el foco; si tiene banda, que no se cierra). Un binding nuevo
  de la ventana, revisado en `TestNoBindingTurnsOffTransparency`.
- `node scripts/ux/recorrido.mjs --solo=<capturas de la función>`,
  `node scripts/ux/recorrido-alumno.mjs --solo=…` y `node scripts/ux/pruebas-alumno.mjs` sin
  errores de JS ni violaciones serias. El recorrido completo lo corre el CI; en local, solo si el
  cambio toca algo común a muchas pantallas (sistema de diseño, menú, avisos).
- Si cambió un diagrama, `scripts/docs/generar-diagramas` y `go test ./docs`.
- Si cambió una pantalla del manual, `node scripts/ux/capturas-manual.mjs`.
- **Revisión de ubicación:** la función quedó donde se aprobó; las acciones sobre un alumno
  están en la Ficha **y** en la Pantalla grande; las zonas nuevas están en `scripts/ux/zonas.mjs`
  y el mapa regenerado (`node scripts/ux/mapa.mjs`).
- La sección del manual en `docs/administracion/` y el diagrama en `docs/diagramas/`, si
  aplica.
- Los casos manuales en `docs/desarrollo/probar-consola-local.md` (C-xx consola, A-xx alumno),
  y en VM si toca el SO. Lo nativo se prueba a ojo: nunca con clics ni teclas sintéticas en el
  escritorio del usuario.
- `gofmt -l .` sin salida, `go vet ./...`,
  `go run honnef.co/go/tools/cmd/staticcheck@v0.8.2 ./...` y `go test -race ./...`.
- Su línea en «Sin publicar» de `CHANGELOG.md`, en su área y en palabras de quien usa Aula.
- `docs/progreso.md` y, si la entrega tomó decisiones de interfaz que valga conservar, una sola
  página en `docs/ux/historial/` con el análisis, los resultados y las decisiones, **sin
  capturas versionadas** (las del arnés quedan en `scripts/ux/salida*` y en el CI).

Al terminar, muestra un resumen, las capturas y lo que falta probar a mano, y espera la
aprobación. **No hagas commit ni push hasta que el usuario lo pida**; en `develop`, agrupado por
ámbito.
