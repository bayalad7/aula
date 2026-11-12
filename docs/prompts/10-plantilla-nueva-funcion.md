# 10 · Plantilla reutilizable para cada función nueva

**Qué es:** no es una etapa única, sino una plantilla. Cópiala y rellénala cada vez que agregues una función de las Fases 2–3 (transmitir tu pantalla por WebRTC, recoger código/archivos, bloquear apps, filtrar web, encuestas, apagar/reiniciar, control remoto).

**Cómo usarla:** sustituye `<FUNCIÓN>` y los corchetes, y pégala como prompt. O, más corto, en
Claude Code: `/funcionalidad-nueva <descripción o número de prompt>` (esta plantilla hecha
proceso, en `.claude/commands/funcionalidad-nueva.md`); para cambiar algo que ya existe,
`/funcionalidad-mejora`. Si cambias esta plantilla, cambia también el comando. Guía con ejemplos:
`docs/desarrollo/comandos-claude.md`.

## Prompt (plantilla)

~~~~markdown
Lee CLAUDE.md, docs/protocol.md, docs/privacidad.md y docs/progreso.md. Vamos a agregar la
función: <FUNCIÓN>. Sigue el flujo del proyecto: protocolo primero, luego interfaz de
plataforma, luego Windows, después la UI de la app que corresponda, y Linux/macOS solo si lo indico.

## 1. Diseño (detente para que lo apruebe)
- Describe los mensajes nuevos del protocolo que necesita <FUNCIÓN> (tipo, dirección, payload,
  respuesta) y agrégalos a docs/protocol.md.
- Di qué `capability` declara el agente y en qué SO estará disponible.
- Si <FUNCIÓN> observa o controla al alumno, describe el indicador/aviso visible y agrégalo a
  docs/privacidad.md.
- Ubicación: 2–3 opciones con su boceto en texto y los nombres de docs/ux/MAPA.md, con tu
  recomendación; yo elijo. Sigue «Ubicación y vocabulario» de CLAUDE.md (material propio →
  módulo del menú; acción sobre un alumno → Ficha y Pantalla grande).
- Dónde vive en cada app, con un boceto en texto (sigue la «Definición de terminado» de CLAUDE.md
  y el «Encaje con lo implementado» del prompt, que es una sugerencia):
  - consola: panel de la barra de acciones, ficha del alumno, sección de la vista «Participación» o
    franja global;
  - alumno: tarjeta en la lista, aviso en la esquina, botón o insignia en la tira, o banda; con
    `notify` (no quita el foco) salvo que actúe sobre el equipo (`raise`, con banda).
- Qué diagramas de docs/diagramas/ se actualizan y si hace falta uno nuevo.
Espera mi aprobación.

## 2. Implementación (cuando apruebe)
- Define o extiende la interfaz en `internal/platform` con fallback "no soportado" por defecto,
  y archivos `*_windows.go` para la implementación.
- Conecta la lógica en los paquetes `internal/*` que correspondan y la UI en la app (binding +
  evento Wails). No metas lógica de SO fuera de `platform`.
- En la consola, dentro de la navegación actual: lo que se aplica a alumnos va sobre la
  selección de Alumnos (panel de la barra de acciones) o en la ficha del alumno; lo que
  responden los alumnos, en la vista «Participación»; con el sistema de diseño de `sistema/` y los
  textos en `textos.es-MX.js`.
- En la app del alumno: componentes de `internal/ui`, textos en su `textos.es-MX.js` y Go que
  manda estados; si pasa por el SO del alumno en modo servicio, por el ayudante
  (`internal/agentsvc`).
- Decide si llega a los alumnos simulados; si controla o cambia el equipo, nunca.
- Respeta la transparencia: nada de funciones ocultas; el alumno siempre puede saber qué pasa
  en su equipo.

## 3. Pruebas y cierre
- Pruebas de la lógica en `internal/*` y de la interfaz de plataforma (con un doble de prueba).
- Prueba manual descrita paso a paso para verificar en VMs, y sus casos en
  docs/desarrollo/probar-consola-local.md.
- Cumple la «Definición de terminado» de CLAUDE.md: errores con código y texto, sección del
  manual, diagrama si cambia un flujo (con sus SVG regenerados), `scripts/ux/dobles.js` y una
  captura en `scripts/ux/recorrido.mjs`; en el alumno, `dobles-alumno.js`,
  `recorrido-alumno.mjs` y `pruebas-alumno.mjs` (no quita el foco; su banda no se cierra).
- `gofmt`, `go vet`, `staticcheck ./...`, `go test -race ./...`, `node scripts/ux/recorrido.mjs`,
  `node scripts/ux/recorrido-alumno.mjs` y `node scripts/ux/pruebas-alumno.mjs`.
- Actualiza docs/eventos-ui.md y docs/progreso.md.

## Restricciones
- Cambios incrementales; no toques paquetes ajenos a esta función.
- Si <FUNCIÓN> requiere permisos especiales (accesibilidad, grabación de pantalla, Polkit/DBus
  en Linux), documenta cómo se conceden en un laboratorio gestionado.
~~~~

## Backlog sugerido (orden recomendado)

Estado: hechos el 1 (prompt 12), el 4 (10.1), el 5 (10.2) y el 6 (10.3). Lo que
falta tiene su prompt en `docs/prompts/10.4` a `10.13`, `12.1`, `12.2`, `13` y `14`.

**Fase 2**
1. Transmitir la pantalla del profesor a los alumnos (WebRTC con `pion/webrtc`; señalización
   `screen.broadcast.*` por el WebSocket).
2. Recoger el código de cada alumno y recoger archivos (`code.collect`, `file.collect`).
3. Chat/comandos globales y "levantar la mano" / encuestas rápidas (`poll.*`).
4. Agente como **servicio del sistema** en Windows (patrón servicio + ayudante en la sesión del
   usuario para captura y overlay), con su interfaz `ServiceControl` en platform.

**Fase 3**
5. Bloquear aplicaciones y filtrar sitios web (filtro DNS o proxy local en el agente).
6. Apagar / reiniciar equipos (`power.*`).
7. Control remoto de un equipo (permisos de accesibilidad; aviso visible permanente).

**Portabilidad**
8. Implementaciones de Linux (X11 primero) y macOS de cada interfaz de `platform`, más
   empaquetado `.deb`/`.rpm`/AppImage y `.dmg`/`.pkg` con sus perfiles de permisos.

**Funciones propias para programación (tu ventaja sobre LanSchool)**
9. Enviar plantillas de ejercicio a todos.
10. Ejecutar las entregas en un contenedor Docker desechable (sin red, con límites) y devolver
    el resultado de las pruebas a la consola.
11. Ver en vivo, como texto, el avance del alumno en un ejercicio.
