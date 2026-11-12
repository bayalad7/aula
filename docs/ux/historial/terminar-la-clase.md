# Terminar la clase (10.14): ubicación y decisiones

Estado: **implementado, sin commit** (12/11/2026). Prompt `docs/prompts/10.14-terminar-la-clase.md`,
con `/funcionalidad-nueva 10.14`. Sin capturas versionadas: las del arnés son
`scripts/ux/salida/184-188` (consola) y `scripts/ux/salida-alumno/95-97` (alumno), y salen en el CI.
El mapa tiene «Consola · Terminar la clase» y «Alumno · La clase terminó».

## Análisis

En un laboratorio compartido, el profesor que olvida detener algo (una restricción, una pausa, el
temporizador) se lo deja al siguiente grupo. Cada función ya tenía su parada; faltaba una sola
acción que las llamara en orden y limpiara la ventana del alumno, que ninguna parada tocaba (el
código, los avisos, las tarjetas de archivos y encuestas, la mano).

- **No es una acción sobre la selección ni sobre un alumno:** es sobre la clase entera. Por eso no
  va en la Barra de acciones ni en la Pantalla grande.
- **No es algo que «sigue activo»:** la Franja global solo aparece si hay algo activo, y la ventana
  del alumno puede necesitar limpieza aunque no haya nada activo.
- **No tiene material propio:** no es un módulo del menú.

## Opciones de ubicación presentadas

1. **Bajo el Selector de clase del Menú** (recomendada, elegida): siempre visible, junto a lo que
   identifica la clase; en el menú compacto queda el icono.
2. **Franja global:** al final de las franjas, solo con algo activo.
3. **Cabecera de Alumnos:** junto a la Selección, solo en esa vista.

En las tres, el diálogo es propio («Terminar la clase»), no un panel de la barra.

## Decisiones

- **Mensaje nuevo `class.end`** con acuse y capacidad (§5.25), en vez de solo los mensajes uno por
  uno: limpia la ventana de una vez y en orden, y da un resultado por alumno. Un agente anterior
  recibe `code.clear` y queda «limpio en parte».
- **Orden:** recoger (opcional) → lo que actúa sobre el equipo → lo que informa → energía (solo si
  se marca) → `class.end` → la consola «sin clase». Si un paso falla, los demás siguen.
- **La energía pendiente se conserva** por omisión (regla 85).
- **No se borran archivos** en esta versión (regla 84); borrar quedó como propuesta P-03.
- **Historial de código y «Últimos envíos» se vacían** (decisión del usuario): el siguiente grupo
  empieza de cero. Antes sobrevivían al cambio de canal (C2) y lo siguen haciendo.
- **Quien faltaba recibe `class.end` al reconectar**, siempre, antes de lo que la consola reenvía:
  mientras estuvo fuera no recibió nada nuevo, así que no hace falta condicionarlo a que el
  profesor no haya mandado nada (se simplificó el diseño aprobado).
- **Sin 10.13:** «Exportar la asistencia», «olvidar la identidad» y «sin materia» se agregan como
  un paso más cuando exista 10.13.
- **En el alumno:** una tarjeta arriba del Estado vacío, con «Nuevo», la hora y lo que se quitó;
  `notify`, nunca `raise`; se cierra con «Entendido» o al llegar código nuevo. No es una banda.

## Resultados

- Capturas 184–188 y 95–97 sin errores de JS ni violaciones serias de axe (la tabla con desplazamiento
  del resultado necesitó foco y nombre de región).
- `pruebas-alumno.mjs` «fin-de-clase»: la tarjeta no quita el foco, no es banda, cuenta como «sin ver»
  y se cierra con «Entendido».
