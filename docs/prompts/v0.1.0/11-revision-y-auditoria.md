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
~~~~

## Nota
Antes de desplegar a todo el laboratorio en una fase importante, conviene que un segundo par de
ojos —otra persona o una sesión independiente que no escribió el código— repase el informe de
seguridad y privacidad.
