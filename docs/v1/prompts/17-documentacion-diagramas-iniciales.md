# 17 · Documentación: generar los diagramas iniciales

**Prerrequisitos:** etapa 03 (protocolo) aprobada, para que los diagramas de descubrimiento,
autenticación, sesión y código sean fieles. Puede correrse temprano para los diagramas «base» y
revisitarse cuando lleguen las funciones de Fase 2/3 (cada una añade su diagrama).
**Depende de:** `docs/v1/diagramas/README.md` (el índice, colores, front-matter y pipeline ya definidos).

## Prompt

~~~~markdown
Lee CLAUDE.md, docs/protocol.md, docs/v1/diagramas/README.md y docs/progreso.md, y el código que ya
exista de los paquetes internal/* y las apps. Vamos a escribir los diagramas iniciales de
docs/v1/diagramas/ siguiendo EXACTAMENTE el README (numeración, colores, front-matter y pipeline).

## Reglas (del README, no las reinventes)
- Un diagrama Mermaid por archivo `NN-nombre.md`, con su front-matter YAML válido (titulo,
  descripcion, audiencia ∈ {profesor, alumno, instalador, desarrollador, auditor}, pantallas,
  secciones_manual, enlaces opcional).
- Usa las clases de color del README: `externo` (azul), `aula` (verde), `datos` (naranja),
  `seguridad` (rojo). Define esas clases en cada diagrama para que se vean igual.
- Cada diagrama cierra con: una explicación en prosa de lo que no es obvio, y una sección
  «Fuentes en el código» que cita los archivos/paquetes en que se basa (si ese código cambia, se
  revisa el diagrama).
- Mermaid con htmlLabels desactivado (sin foreignObject), pensando en que luego se pre-renderiza a
  SVG seguro.

## Diagramas a crear (los «base» y de Módulos A/C primero)
Crea, en este orden, con contenido fiel al protocolo y al código actual:
1. 01-contexto.md (flowchart): consola del profesor, app del alumno, red local, canal.
2. 02-componentes.md (flowchart): paquetes internal/* y apps, con la regla de oro visible
   (protocol/transport/… sin SO; todo lo de SO tras internal/platform).
3. 03-descubrimiento-canal.md (sequenceDiagram): anuncio mDNS con canal → el alumno filtra y elige.
4. 04-autenticacion-tls.md (sequenceDiagram): TLS + fijado de huella + reto/firma Ed25519.
5. 05-ciclo-sesion.md (stateDiagram-v2): hello → auth → conectado/heartbeat → reconexión.
6. 06-emision-codigo.md (sequenceDiagram): code.share → render en el alumno → code.ack.
7. 11-modelo-datos.md (erDiagram): historial de snippets, acuses, config, llaves (lo que persista).
Si el tiempo alcanza y la función existe, agrega 07, 08, 09, 10 y 12 según el README; si no, déjalos
listados como pendientes en docs/progreso.md.

Si piensas que hace falta algún diagrama importante agregalo y documentalo.

## Validador
- Agrega (o ajusta) la prueba `go test ./docs` que exige front-matter en cada diagrama y valida cada
  campo contra el esquema del README, y que falla si un `{{diagrama:...}}` del manual referencia un
  id inexistente. No implementes aún el render a SVG (eso es la etapa 17); aquí basta el Markdown y
  su validación.

## Cierre
- Verifica que cada diagrama se vea bien en GitHub (Mermaid nativo).
- `go test ./docs` pasa. Actualiza docs/progreso.md con qué diagramas quedaron y cuáles faltan.

## Restricciones
- No inventes componentes que no existan en el código; si algo aún no está, dilo en la prosa del
  diagrama en vez de dibujar algo falso.
- Cambios incrementales; este trabajo es solo documentación.
~~~~

## Verificación manual
- [ ] Abre 2–3 diagramas en GitHub: ¿se renderizan y se entienden?
- [ ] ¿Cada diagrama cita las fuentes de código correctas?
- [ ] `go test ./docs` falla si borras un front-matter (comprueba que el validador sirve).
