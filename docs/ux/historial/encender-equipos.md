# Encender equipos (10.19): ubicación y decisiones

Estado: **implementado, sin commit** (12/11/2026). Prompt
`docs/prompts/10.19-encender-equipos-wake-on-lan.md`, con `/funcionalidad-nueva 10.19`. Sin capturas
versionadas: las del arnés son `scripts/ux/salida/189-194` y salen en el CI. El mapa tiene la zona
«Mostrar apagados» en «Consola · Alumnos (lista)» (`docs/ux/mapa/consola-alumnos.webp`).

## Análisis

Un equipo apagado no tiene agente, así que encenderlo no es un mensaje del protocolo: la consola
recuerda la MAC que cada agente manda en `hello` y manda el paquete mágico por la red. En la
interfaz, eso trae dos cosas que no existían:

- **Equipos que no son sesiones.** Hasta ahora la lista solo mostraba sesiones: conectadas y
  desconectadas desde que abrió la consola. Un equipo apagado desde ayer no estaba en ningún lado.
- **Una acción sobre un equipo sin pantalla.** La regla «acción sobre un alumno → Pantalla grande»
  no aplica: no hay pantalla que ver. Y la Ficha solo informa.

10.15 (mapa del aula) todavía no existe, así que el «asiento vacío» que proponía el prompt no
estaba disponible.

## Opciones de ubicación presentadas

1. **Panel Energía + Fila del alumno** (recomendada, **elegida**): «Encender» como cuarta opción
   del Panel Energía, para los apagados de la selección o de todo el canal, y un botón «Encender»
   en la Fila del alumno de cada equipo sin conexión. Junta todo lo de energía en un lugar y deja
   el atajo para uno.
2. **Solo la Fila y la cabecera:** botón en cada fila y «Encender todos los apagados» junto al
   filtro. Saca una acción de energía del panel donde están las demás.
3. **Solo el Panel Energía:** la fila solo muestra el estado. Encender uno obliga a marcarlo y
   abrir el panel.

Para mostrar los apagados: un **interruptor «Mostrar apagados (n)»** en la cabecera (elegido),
oculto por defecto y que se recuerda, o un grupo siempre visible al final. Se eligió el
interruptor para no llenar la lista durante la clase.

## Decisiones

- **La Fila del alumno es la excepción a la Pantalla grande**, solo para equipos sin conexión.
  Los desconectados de esta sesión de la consola muestran «Encender» en Notas; los conocidos sin
  sesión van en el grupo «Apagados (n)», con «Visto hoy a las…» y, en lugar del botón de ficha,
  **Olvidar** (que borra su MAC).
- **«Encendiendo… m:ss» y «No respondió» reemplazan el estado** en la columna Estado; «Otra red»
  y «¿Por qué?» (al manual) van en Notas. Sin franja global: encender no sigue activo, termina en
  3 minutos.
- **En el panel**, Encender oculta el tiempo y «Forzar» y la línea «Para n equipos» (que cuenta
  a los conectados), y dice cuántos sin conexión le tocan y cuántos no se pueden encender por
  tener un agente anterior. El botón es primario, no de peligro, y **no pide confirmación**:
  encender no le hace nada a un equipo que ya está encendido.
- **Icono nuevo `encender`** (un rayo) en `internal/ui/iconos.svg`. `energia` sigue siendo apagar.
- **Ficha › Qué puede hacer su equipo** suma «Encender desde la consola» (se conoce su MAC).
- **El relevo** por un agente de otra subred no entró (P-04 en `docs/ux/PROPUESTAS.md`).

## Resultados

Escenas 189–194 sin errores de JS ni violaciones serias de axe, en claro, en oscuro y a 900×600.
Lo nativo (que un equipo arranque de verdad) queda para el laboratorio: casos C-121 a C-126 y
A-53 de `docs/desarrollo/probar-consola-local.md`.
