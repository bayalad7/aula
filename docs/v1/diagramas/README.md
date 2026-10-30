# Diagramas

Un diagrama Mermaid por archivo, numerado. GitHub los muestra sin plugins. Cada uno dice qué
pregunta responde y para quién es, explica lo que no es obvio y cita los archivos de código en que
se basa: si ese código cambia, hay que revisar el diagrama.

**Colores** (los mismos en todos):

| Color | Clase | Qué representa |
|---|---|---|
| azul | `externo` | Red local, socket del SO, archivos del sistema o clientes externos. |
| verde | `aula` | Paquetes y componentes de Aula (consola del profesor y app del alumno). |
| naranja | `datos` | Historial de snippets, bases de datos locales, almacenamiento de acuses y logs. |
| rojo | `seguridad` | Claves Ed25519, certificados TLS, huellas digitales y autenticación de canal. |

En Mermaid, cada `flowchart` y `stateDiagram-v2` define estas cuatro clases, copiadas tal cual:

```text
classDef externo fill:#dbeafe,stroke:#1d4ed8,color:#111827
classDef aula fill:#dcfce7,stroke:#15803d,color:#111827
classDef datos fill:#ffedd5,stroke:#c2410c,color:#111827
classDef seguridad fill:#fee2e2,stroke:#b91c1c,color:#111827
```

`sequenceDiagram` no admite `classDef`: agrupa con `box` y `rect` usando los mismos rellenos
(`rgb(219,234,254)` azul, `rgb(220,252,231)` verde, `rgb(255,237,213)` naranja,
`rgb(254,226,226)` rojo). `erDiagram` va sin color, porque no todas las versiones de Mermaid
admiten clases en él. En `sequenceDiagram` no se usa `;` dentro de los mensajes: Mermaid lo toma
como fin de sentencia.

Cada bloque empieza con `%%{init: {"htmlLabels": false, "flowchart": {"htmlLabels": false}}}%%`
(en los que no son `flowchart` basta `"htmlLabels": false`), para que el SVG pre-renderizado no
lleve `foreignObject`.

| # | Diagrama | Tipo | Para quién |
|---|---|---|---|
| 01 | [Contexto del sistema](01-contexto.md) | `flowchart` | profesor, instalador, desarrollador |
| 02 | [Componentes y dependencias](02-componentes.md) | `flowchart` | desarrollador |
| 03 | [Descubrimiento de canal (mDNS)](03-descubrimiento-canal.md) | `sequenceDiagram` | instalador, desarrollador |
| 04 | [Autenticación y fijado de certificado](04-autenticacion-tls.md) | `sequenceDiagram` | desarrollador, auditor |
| 05 | [Ciclo de sesión y heartbeat](05-ciclo-sesion.md) | `stateDiagram-v2` | desarrollador |
| 06 | [Emisión y recepción de código](06-emision-codigo.md) | `sequenceDiagram` | profesor, desarrollador |
| 07 | [Acuses y recolección de código/archivos](07-acuses-recoleccion.md) | `flowchart` | desarrollador |
| 08 | [Monitoreo: miniaturas y pantalla](08-monitoreo-pantalla.md) | `sequenceDiagram` | profesor, desarrollador |
| 09 | [Transparencia: avisos e indicadores al alumno](09-transparencia-avisos.md) | `flowchart` | profesor, alumno, desarrollador |
| 10 | Aula simulada y simulación de fallas (pendiente) | `flowchart` | profesor, desarrollador |
| 11 | [Modelo de datos local](11-modelo-datos.md) | `erDiagram` | desarrollador, auditor |
| 12 | [Diagnóstico: «Mi alumno no conecta»](12-diagnostico-conexion.md) | `flowchart` | profesor, instalador |
| 13 | [El agente como servicio de Windows](13-agente-servicio.md) | `flowchart` | instalador, desarrollador, auditor |
| 14 | [Transmisión de la pantalla del profesor](14-transmision-pantalla.md) | `sequenceDiagram` | profesor, desarrollador |
| 15 | [Restricciones de la clase: apps y filtro web](15-restricciones.md) | `flowchart` | profesor, desarrollador, auditor |
| 16 | [Energía: apagar, reiniciar y cerrar sesión](16-energia.md) | `stateDiagram-v2` | profesor, alumno, desarrollador |

El 10 queda pendiente hasta que exista el modo de aula simulada (etapa 18); una fila sin enlace
debe decir «(pendiente)».

---

## Metadatos

Cada diagrama empieza con un bloque YAML (front-matter) que la consola del profesor usa para su
galería y para la ayuda contextual de cada pantalla; GitHub lo muestra como una tabla. Las listas
van en una línea (`[a, b]`):

| Campo | Qué es |
|---|---|
| `titulo` | El título, igual al encabezado del diagrama. |
| `descripcion` | Una frase sin tecnicismos: qué muestra el diagrama. |
| `audiencia` | Para quién es: `profesor`, `alumno`, `instalador`, `desarrollador`, `auditor`. |
| `pantallas` | Las pantallas de la consola donde aparece como ayuda: el `id` de su `<section>` en `apps/teacher/frontend/dist/index.html` (hoy `setup`, `console`, `screens`, `restr`, `power`). Puede ir vacía. |
| `secciones_manual` | Los documentos de `docs/` que lo citan, sin `.md` (por ejemplo `privacidad` o, cuando exista el manual, `administracion/conexion`). Al menos uno, y deben existir. |
| `enlaces` | Opcional, solo en `flowchart`: nodos que llevan a otra parte, como `nodo=destino`. El destino es un documento de `docs/` (sin `.md`, con `#ancla` opcional, el ancla de GitHub de un encabezado) o `pantalla:<id>`. |

`go test ./docs` exige el bloque en cada diagrama y revisa cada campo contra lo real: que la
pantalla exista en la consola, que el documento y el ancla existan, que el título coincida con el
encabezado `#` y con esta tabla, y que las rutas de «Fuentes en el código» existan. También
comprueba las clases de color, el `init` sin `htmlLabels`, las secciones «Lo que no es obvio» y
«Fuentes en el código», y que todo `{{diagrama:NN-nombre}}` de `docs/` (salvo `docs/v1/prompts/`)
apunte a un diagrama existente.

Ejemplo de front-matter (diagrama 04):

```yaml
---
titulo: "Autenticación y fijado de certificado"
descripcion: "Cómo el agente comprueba que habla con la consola del profesor y no con un impostor"
audiencia: [desarrollador, auditor]
pantallas: [setup]
secciones_manual: [protocol]
---
```

Con enlaces (solo `flowchart`):

```yaml
enlaces: [canal=configuracion, clave=protocol#101-identidades-y-claves, consola=pantalla:console]
```

Cada diagrama tiene, después del bloque, las secciones **«Lo que no es obvio»** (prosa) y
**«Fuentes en el código»** (rutas entre comillas invertidas). Si ese código cambia, se revisa el
diagrama.

---

## La consola los dibuja de antemano

La consola del profesor muestra los diagramas como imágenes SVG embebidas, sin necesidad de
conexión a Internet y sin incluir la librería JavaScript de Mermaid en el binario. Si cambias un
bloque `mermaid` o su front-matter, vuelve a generarlos (requiere Node.js 18+ para el paso de
compilación previa):

```sh
scripts/docs/generar-diagramas.sh        # o .\scripts\docs\generar-diagramas.ps1 en Windows
```

El pipeline genera una variante clara y otra oscura de cada SVG, un texto alternativo a partir de
`titulo` + `descripcion`, y un manifiesto con el hash de cada bloque `mermaid`. Si el hash de un
diagrama no coincide con el del manifiesto, `go test ./docs` y el CI fallan con el mensaje
«Diagramas desactualizados: ejecuta scripts/docs/generar-diagramas». Así el diagrama que ve la
consola nunca queda desincronizado de este `docs/`.
