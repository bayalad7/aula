# 11 · Revisión y auditoría (al cerrar cada fase)

**Qué es:** un prompt para pasar al final de cada hito o fase, antes de instalar en el laboratorio. Ejecútalo con `/clear` para que Claude revise con mirada fresca.

## Prompt

~~~~markdown
Lee CLAUDE.md, docs/protocol.md, docs/privacidad.md y docs/progreso.md. Haz una revisión completa
del estado actual del proyecto Aula. No cambies código todavía: primero entrégame un informe, y
luego arreglamos por prioridad.

## Revisa
1. **Seguridad**
   - ¿El agente sigue aceptando solo a la consola autenticada? Busca rutas donde se procese un
     mensaje antes de `auth.ok`.
   - ¿Hay algún lugar donde se registre en logs código de alumnos, contenido de pantalla o claves?
   - ¿La clave privada está fuera del binario del agente y con permisos restringidos?
   - ¿Los tamaños de mensaje y las entradas de red están acotados? ¿Se validan los payload?
   - Concurrencia: corre `go test -race ./...` y reporta cualquier data race.
2. **Transparencia y privacidad**
   - ¿Toda función que observa o controla al alumno tiene su indicador/aviso visible?
   - ¿Existe algún camino que capture, congele o transmita sin avisar? Si lo hay, es un defecto.
   - ¿docs/privacidad.md refleja lo que hace el código realmente?
3. **Arquitectura**
   - ¿`protocol`, `discovery`, `transport`, `session`, `codedist` siguen libres de llamadas al SO?
     Busca violaciones de la regla de oro.
   - ¿Toda función de SO está detrás de una interfaz de `internal/platform` con su fallback?
4. **Calidad**
   - Resultado de `gofmt -l`, `go vet ./...`, `staticcheck ./...`, `go test ./...`.
   - Manejo de errores: errores ignorados, `panic` fuera del arranque, goroutines sin cancelación
     por `context`.
   - Cobertura de pruebas de los paquetes críticos (security, protocol, session).

## Entrega
- Informe en docs/auditoria-<fecha>.md con hallazgos clasificados: crítico / importante / menor.
- Por hallazgo: archivo, línea, por qué importa y arreglo propuesto.
- Al final, una lista priorizada. Espera mi visto bueno antes de corregir.

## Encaje con lo implementado (no lo cambies sin decirlo)
Aplica la «Definición de terminado» de CLAUDE.md (consola, app del alumno, transparencia, arnés,
manual y diagramas). Para esta etapa en concreto:
- **Arquitectura:** la lista de paquetes sin llamadas al SO es la de la «regla de oro» de
  CLAUDE.md (hoy también `screen`, `broadcast`, `agentsvc`, `policy`, `power`, `simclass`,
  `manual`, `connectcheck` y `ui`), no la de este prompt.
- **Transparencia, con las pruebas que ya existen:**
  - `scripts/ux/pruebas-alumno.mjs`: cada banda aparece, no se cierra ni con Esc, y se va sola;
  - `TestNoBindingTurnsOffTransparency`;
  - la salvaguarda de capacidades de los simulados.

  Revisa además que cada `raise` del alumno sea una acción del profesor sobre el equipo y que
  todo lo demás use `notify`, y que el distintivo nativo y las bandas digan lo mismo que
  `docs/privacidad.md`, regla por regla.
- **Definición de terminado:** por cada función entregada desde la última auditoría:
  - su sección del manual y sus diagramas al día (`go test ./docs`, SVG regenerados);
  - sus escenas en el arnés de cada app;
  - sus casos C-xx/A-xx;
  - sus errores con código y texto.
- **UX en verde:** `recorrido.mjs`, `teclado.mjs`, `usuario-nuevo.mjs`, `recorrido-alumno.mjs` y
  `pruebas-alumno.mjs` sin errores de JS ni violaciones serias; las pruebas de textos de las dos
  apps.
- **Recursos del alumno:** `go run ./tools/medir-alumno` (CPU y RAM frente a
  `docs/ux/historial/alumno-resultados.md`) y `-ventana` (sin quitar el foco).
~~~~

## Nota
Antes de desplegar a todo el laboratorio en una fase importante, conviene que un segundo par de
ojos —otra persona o una sesión independiente que no escribió el código— repase el informe de
seguridad y privacidad.
