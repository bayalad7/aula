# Auditoría y limpieza de artefactos de trabajo de la documentación

Actúa como un ingeniero de software sénior especializado en mantenimiento de
repositorios y gestión de documentación técnica. Prioriza conservar el
conocimiento y la trazabilidad, decide con base en evidencia y, ante la duda,
no elimines.

# Objetivo

Audita y limpia los artefactos de diseño y UX que quedaron de versiones ya
terminadas. Conserva el conocimiento y quita solo la evidencia de proceso que
ya no aporta. La meta no es borrar lo más posible, sino dejar un repositorio
más claro sin perder información ni romper nada.

## DATOS DEL PROYECTO

- Rutas a auditar: docs/v1/ux, docs/v1/ux/profesor, docs/v1/ux/alumno
- Versión publicada más reciente y su etiqueta: X.Y.Z
- Versión en curso: v1
- Rutas activas que no se pueden modificar: <ninguna | docs/v1/ux>

Si un dato falta o contradice lo que ves en el repositorio, deténte y pregunta.

## Reglas

- Lee primero CLAUDE.md, las instrucciones del proyecto y las convenciones de
  commits y ramas, y síguelas.
- Fase 1 de solo lectura: no borres, muevas ni edites nada hasta que apruebe.
- No toques las rutas activas ni el trabajo de una versión sin publicar. Si
  dudas si algo sigue vigente, clasifícalo como activo.
- No reescribas el historial de git. No hagas push ni abras PR sin que lo pida.
- Trabaja solo con archivos versionados; reporta aparte los no versionados o
  ignorados, sin tocarlos.
- Basa cada conclusión en evidencia del repositorio, no en el nombre del archivo.

## Fase 1: Auditoría

1. Haz el inventario de cada carpeta: tamaño, número de archivos, tipo de
   contenido y fecha del último commit.
2. Confirma qué versión produjo cada carpeta y si esa versión ya está publicada.
3. Busca las dependencias reales de cada carpeta:
   - enlaces desde Markdown, ADR, README, CHANGELOG y documentación;
   - importaciones, embebidos y lectura de archivos en código y build;
   - pruebas, CI, scripts y pruebas que validen enlaces;
   - instrucciones de CLAUDE.md o memorias que indiquen dónde guardar archivos.
4. Clasifica cada carpeta o archivo en una sola categoría:
   - CONSERVAR: explica decisiones, tiene valor futuro o algo depende de él.
   - CONSERVAR Y CONSOLIDAR: útil, pero mezclado o disperso.
   - ELIMINAR: evidencia de una entrega terminada, sin dependencias y
     recuperable desde una etiqueta (capturas, scripts de un solo uso, informes).
   - ACTIVO: pertenece a la versión en curso; no se toca.
   - DUDOSO: necesita mi decisión.
5. Busca en lo que propones eliminar cualquier pendiente, decisión o hallazgo
   que no exista en otro lugar. Debe trasladarse antes de eliminarlo.

Entrega este informe y detente:

- Resumen: tamaño actual, tamaño liberado, archivos afectados.
- Tabla: ruta | categoría | tamaño | dependencias | motivo.
- Pendientes encontrados y a dónde los llevarías.
- Referencias que habría que actualizar.
- Qué seguirá ocupando espacio en `.git` y cómo recuperar lo eliminado desde
  la etiqueta.
- Riesgos y decisiones que necesitas de mí.

## Fase 2: Ejecución (solo tras mi aprobación)

1. Crea una rama desde la rama de trabajo, salvo que las reglas del proyecto
   digan otra cosa.
2. Traslada primero los pendientes y las decisiones.
3. Actualiza las referencias afectadas.
4. Elimina con `git rm` solo lo aprobado. Si aparece algo que no estaba en el
   informe, detente.
5. Agrega un README breve en la raíz de la carpeta auditada, si no existe: qué
   contiene, qué se quitó y con qué etiqueta recuperarlo.

## Verificación

Ejecuta las pruebas, el lint, el build y la validación de documentación y
enlaces que defina el proyecto. Confirma con `git diff --stat` que solo cambió
lo aprobado. Si algo falla, corrígelo solo si fue causado por esta limpieza; si
no, repórtalo sin arreglarlo. Haz commit solo si te lo pido.

## Prevención

Propón, sin aplicarla, una convención para las siguientes versiones:

- Las capturas y los informes de cada entrega van fuera del repositorio
  (carpeta temporal o artefactos del CI), salvo las que un documento enlace.
- Cada versión deja solo un análisis, sus resultados y sus decisiones.
- Indica qué líneas de CLAUDE.md o memorias habría que cambiar.

## Resultado final

Reporta qué se quitó, cuánto se liberó, qué se trasladó, qué se conservó y por
qué, las pruebas ejecutadas con su resultado y lo que quedó pendiente.
