# Cuestionarios con puntaje (10.20): ubicación y decisiones

Estado: **implementado, sin commit** (13/11/2026). Prompt
`docs/prompts/10.20-cuestionario-con-puntaje.md`, con `/funcionalidad-mejora` (mejora de 10.12).
Sin capturas versionadas: las del arnés son `scripts/ux/salida/195-202` (consola) y
`scripts/ux/salida-alumno/98-103` (alumno), y salen en el CI. El mapa tiene «Consola ·
Cuestionarios», «Consola · Participación › Encuestas (cuestionario)» y «Alumno · Cuestionario»; el
manual, `cuestionarios.md` con dos capturas nuevas.

## Análisis

Las encuestas de 10.12 sirven para comprobar si se entendió, pero no para un repaso o un examen
corto: falta la respuesta correcta, el puntaje y que cada alumno sepa cómo le fue. Un cuestionario
es **material propio** del profesor (se prepara antes y se reutiliza), así que por la regla de
CLAUDE.md va en un módulo del menú. Lo que **responden** los alumnos sigue yendo a «Participación».

## Opciones de ubicación presentadas

1. **Módulo «Cuestionarios» del menú** (recomendada, elegida): los guardados, el editor y «Lanzar
   a». Los resultados van en la sección Encuestas de Participación, y la franja es la de la
   encuesta abierta.
2. **Pestaña en el Panel Encuesta:** menos superficie, pero mete material propio en un panel.
3. **Pestaña en Vista Archivos**, junto a Plantillas: mezcla archivos con evaluación.

## Decisiones

- **Protocolo compatible:** `poll.open` con `kind: "quiz"`, `poll.vote` con `answers` y
  `submitted`, mensaje nuevo `poll.quiz.result` y capacidad `poll.quiz` (§5.23.1). Un agente sin
  ella no recibe el cuestionario y la consola lo lista. Las encuestas de 10.12 no cambian.
- **La correcta no sale de la consola antes del cierre**, y solo viaja en el resultado de cada
  alumno (regla 88). Hay una prueba sobre el JSON enviado, también con orden al azar y al reenviar.
- **Siempre con nombre** (regla 87): la tarjeta lo dice antes de la primera pregunta.
- **Calificación todo o nada** (decisión del usuario): suma solo si eligió exactamente las correctas.
- **«Elige todas las que apliquen»** (decisión del usuario): viaja `multi`, nunca cuáles ni cuántas.
- **Con tiempo, se cierra solo** al llegar a cero (decisión del usuario); cerrarlo antes detiene su
  temporizador. Si el profesor extiende o detiene el temporizador, decide él.
- **Al azar por alumno**, siempre igual para el mismo alumno (semilla: encuesta, equipo y usuario),
  para que no cambie al reconectar.
- **Guardar al marcar:** la ventana guarda cada opción que el alumno marca; el agente manda como
  mucho una por segundo, solo la última, así no se pierde la última respuesta si el profesor cierra.
- **Privacidad** (reglas 87–91): se guardan en disco solo los cuestionarios (material del profesor)
  y, con «Exportar calificaciones», el CSV (sin simulados). Los puntajes solo se ven en
  Participación, nunca en Alumnos ni en la Pantalla grande. Los registros no llevan el título ni el
  `id` del archivo, que sale del título.
- **Sin 10.13 ni 10.16:** el puntaje es por equipo y usuario y solo de la sesión; el CSV no lleva
  cuenta, nombre, grado ni grupo, y no se lanza a un grupo. Queda anotado en `docs/progreso.md`.

## Resultados

- `go test -race ./...`, `go vet`, `staticcheck` y `gofmt` limpios.
- Arnés de la consola (escenas 195–202) y del alumno (98–103) sin errores de JS ni violaciones
  serias de axe; `pruebas-alumno.mjs` con los casos nuevos `cuestionario`,
  `cuestionario-entregar-asi` y `cuestionario-resultado`.
- Falta la prueba en VM (`docs/prueba-vms.md` §29) y los casos C-127 a C-131 y A-54 a A-56.
