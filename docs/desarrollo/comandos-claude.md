# Comandos de Claude Code para Aula

Dos comandos del proyecto, en `.claude/commands/`, que convierten la plantilla 10 y la
«Definición de terminado» de `CLAUDE.md` en un proceso. Con ellos, una función nueva o un cambio
respeta lo que ya existe sin repetir la lista de reglas en cada prompt.

| Comando | Para qué |
|---|---|
| `/funcionalidad-nueva` | Algo que Aula todavía no hace |
| `/funcionalidad-mejora` | Corregir, mejorar o cambiar algo que ya existe |

¿No sabes cuál? Si hoy hay un botón, una pantalla o un mensaje del protocolo para eso, es
`/funcionalidad-mejora`.

## Antes de llamarlos

- **Una función por sesión.** Usa `/clear` entre una y otra; el comando lee lo que necesita
  (`CLAUDE.md`, `docs/progreso.md`, el protocolo…).
- **Escribe como se lo contarías a otra persona.** Los parámetros entre corchetes son
  opcionales: lo que no digas, Claude lo deduce del código, y solo te pregunta, en una sola tanda,
  lo que cambia el diseño.
- **Si cambias un comando, cambia también la plantilla 10** (y al revés).

## `/funcionalidad-nueva`

```
/funcionalidad-nueva <descripción o prompt> [para: …] [destino: …] [so: …]
```

| Parámetro | Valores | Si no lo dices |
|---|---|---|
| Descripción o prompt | Texto libre, o el número de un prompt de `docs/v1/prompts/` (`10.4`) | Obligatorio |
| `para:` | `profesor`, `alumno`, `ambos`: qué app cambia | Lo deduce |
| `destino:` | `todos` (el canal), `selección` (los marcados en Alumnos), `un alumno` (su ficha), `ninguno` | Lo deduce o te pregunta |
| `so:` | `windows`, `linux`, `macos` (separados por coma) | `windows` |

Además decide, y te lo muestra antes de diseñar:
- si la función observa o controla al alumno (entonces lleva aviso visible y regla en
  `docs/privacidad.md`);
- si les llega a los alumnos simulados (nunca, si controla o cambia el equipo);
- qué permisos necesita;
- qué versión sugiere.

### Ejemplos

Desde un prompt del backlog. El prompt es la especificación; lo que agregues manda sobre él:

```
/funcionalidad-nueva 10.4
/funcionalidad-nueva 10.8 temporizador compartido, pero que el profesor pueda pausarlo
/funcionalidad-nueva docs/v1/prompts/10.12-encuestas-en-vivo.md solo preguntas de opción múltiple
```

Con una descripción libre:

```
/funcionalidad-nueva Que el profesor congele la pantalla de los alumnos para que pongan atención
/funcionalidad-nueva Botón para que el alumno pida ayuda y el profesor vea la lista en orden de llegada
```

Con los parámetros explícitos (útil cuando quieres decidirlo tú):

```
/funcionalidad-nueva Congelar pantalla para: ambos destino: selección so: windows
/funcionalidad-nueva Enviar un enlace que se abra en el navegador del alumno para: ambos destino: todos
/funcionalidad-nueva Tomar una captura de la pantalla de un alumno destino: un alumno
/funcionalidad-nueva Modo oscuro en la app del alumno para: alumno destino: ninguno
/funcionalidad-nueva Apagar equipos también en Linux so: linux
```

### Qué pasa después

1. **Ficha de la función:** Claude te muestra los parámetros, los que dijiste y los que dedujo,
   y te pregunta lo que falte.
2. **Diseño:** protocolo, privacidad, un boceto de la consola, errores, manual y diagramas. **Se
   detiene hasta que lo apruebes.**
3. **Implementación:** protocolo → plataforma → `internal` → bindings → consola → app del alumno.
4. **Cierre:**
   - repasa la «Definición de terminado» punto por punto;
   - corre las pruebas y el arnés web;
   - te muestra capturas y lo que falta probar a mano (casos en `probar-consola-local.md`).
5. **Commit:** solo cuando lo pidas («Sí» o `/commit`).

## `/funcionalidad-mejora`

```
/funcionalidad-mejora <función o pantalla> <qué cambiar> [tipo: …] [para: …]
```

| Parámetro | Valores | Si no lo dices |
|---|---|---|
| Función y qué cambiar | Texto libre: qué pantalla o acción, y cómo debe quedar | Obligatorio |
| `tipo:` | `corrección` (no hace lo que debía), `mejora` (UX, textos o rendimiento, sin cambiar qué hace), `cambio` (hace otra cosa) | Lo deduce |
| `para:` | `profesor`, `alumno`, `ambos` | Lo deduce |

En una corrección ayuda dar **los pasos para reproducirla** y **el mensaje que viste**: Claude
empieza por una prueba que falle con eso.

### Ejemplos

Correcciones:

```
/funcionalidad-mejora Código: al reemitir un ejercicio el contador queda en 0 de 2 aunque los alumnos lo reciben tipo: corrección
/funcionalidad-mejora La ficha del alumno no se actualiza cuando se desconecta; pasos: abrir la ficha, apagar la VM, esperar 20 s
/funcionalidad-mejora Ajustes › Cambiar canal acepta "0" y luego falla con «aula:invalidChannel»
```

Mejoras de uso (no cambian qué hace):

```
/funcionalidad-mejora Código: que «Reenviar a los que faltan» pida confirmación tipo: mejora
/funcionalidad-mejora Alumnos: ordenar la lista por nombre de equipo y dejar los desconectados al final
/funcionalidad-mejora El texto del error portInUse no se entiende; que diga qué hacer en palabras simples
/funcionalidad-mejora La app del alumno: el botón Copiar es muy chico para:alumno
```

Cambios de comportamiento (se detiene para que apruebes el diseño):

```
/funcionalidad-mejora Energía: que la cuenta regresiva por omisión sea de 2 minutos en vez de 1 tipo: cambio
/funcionalidad-mejora Restricciones: que se mantengan al cambiar de canal en vez de quitarse tipo: cambio
/funcionalidad-mejora Aula simulada: permitir hasta 60 alumnos simulados
```

### Qué pasa después

1. **Ficha del cambio:** función, hoy (lo comprueba en el código o en el arnés, no lo supone),
   deseado, tipo, alcance y versión (`z`, `y` o `x`).
2. **Inventario:** todo lo que depende de esa función (código, pruebas, textos, errores, manual,
   diagramas, protocolo, arnés y casos manuales), para actualizar lo que quede distinto.
3. **Diseño, solo si hace falta:**
   - se detiene si toca el protocolo, la configuración, la privacidad o lo que ve el alumno, si
     mueve algo de lugar o si es un `cambio`;
   - una corrección o una mejora pequeña sigue directo y te lo dice.
4. **Implementación:** una corrección empieza por una prueba que falle. Las pruebas del
   comportamiento anterior se ajustan, nunca se borran.
5. **Cierre:** antes y después con capturas, pruebas y arnés, lo que falta probar a mano y la
   versión sugerida. Commit solo cuando lo pidas.

## Si algo se desvía

- **Si quieres cambiar el diseño antes de aprobarlo,** responde con lo que cambiarías. No hace
  falta volver a llamar el comando.
- **Si algo exige cambiar el protocolo o la seguridad fuera de lo permitido,** Claude no lo
  implementa: lo anota en `docs/v1/ux/PROPUESTAS.md` para que lo decidas aparte.
- **Si la sesión se alarga,** pide «actualiza `docs/progreso.md`» y sigue en una sesión nueva con
  «Lee docs/progreso.md y continúa donde quedamos».
