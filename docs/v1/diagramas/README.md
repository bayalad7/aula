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

| # | Diagrama | Tipo | Para quién |
|---|---|---|---|---|
| 01 | [Contexto del sistema](01-contexto.md) | `flowchart` | profesor, instalador, desarrollador |
| 02 | [Componentes y dependencias](02-componentes.md) | `flowchart` | desarrollador |
| 03 | [Descubrimiento de canal (mDNS)](03-descubrimiento-canal.md) | `sequenceDiagram` | instalador, desarrollador |
| 04 | [Autenticación y fijado de certificado](04-autenticacion-tls.md) | `sequenceDiagram` | desarrollador, auditor |
| 05 | [Ciclo de sesión y heartbeat](05-ciclo-sesion.md) | `stateDiagram-v2` | desarrollador |
| 06 | [Emisión y recepción de código](06-emision-codigo.md) | `sequenceDiagram` | profesor, desarrollador |
| 07 | [Acuses y recolección de código/archivos](07-acuses-recoleccion.md) | `flowchart` | desarrollador |
| 08 | [Monitoreo: miniaturas y pantalla](08-monitoreo-pantalla.md) | `sequenceDiagram` | profesor, desarrollador |
| 09 | [Transparencia: avisos e indicadores al alumno](09-transparencia-avisos.md) | `flowchart` | profesor, alumno, desarrollador |
| 10 | [Aula simulada y simulación de fallas](10-aula-simulada.md) | `flowchart` | profesor, desarrollador |
| 11 | [Modelo de datos local](11-modelo-datos.md) | `erDiagram` | desarrollador, auditor |
| 12 | [Diagnóstico: «Mi alumno no conecta»](12-diagnostico-conexion.md) | `flowchart` | profesor, instalador |

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
| `pantallas` | Las pantallas de la consola donde aparece como ayuda (claves de navegación en el frontend). Puede ir vacía. |
| `secciones_manual` | Los documentos del manual que lo citan, sin `.md` (por ejemplo `administracion/conexion`). |
| `enlaces` | Opcional, solo en `flowchart`: nodos que llevan a otra parte. El destino es un documento del manual (sin `.md`, con ancla opcional) o una pantalla de la consola. |

`go test ./docs` exige el bloque en cada diagrama y revisa cada campo.

Ejemplo de front-matter (diagrama 04):

```yaml
---
titulo: "Autenticación y fijado de certificado"
descripcion: "Cómo el agente comprueba que habla con la consola del profesor y no con un impostor"
audiencia: [desarrollador, auditor]
pantallas: [ajustes, alumnos]
secciones_manual: [administracion/seguridad]
---
```

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
