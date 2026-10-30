# 06 · Módulo C — App estudiante: overlay de código con Monaco y botón Copiar

**Prerrequisitos:** etapa 05 completa.
**Duración estimada:** 1–2 sesiones.
**Meta:** la función diferenciadora. Al terminar, el alumno recibe el código que emites, formateado, y lo copia con un clic sin errores de tipeo.

## Prompt

~~~~markdown
Lee CLAUDE.md, docs/protocol.md y docs/progreso.md. Dale a la app estudiante la recepción y
visualización de código. El agente ya se conecta y autentica.

## Alcance
1. Al recibir `code.share`, muestra una ventana/overlay emergente de Wails con:
   - Título del snippet y lenguaje.
   - El código en **Monaco Editor** en modo solo lectura con resaltado según `lang`
     (si Monaco resulta pesado para el overlay, usa Prism.js y anótalo en docs/decisiones.md).
   - Botón "Copiar al portapapeles".
   - `mode`: "replace" sustituye el contenido; "append" agrega un bloque nuevo (lista de snippets).
2. Al recibir `msg.show`, muestra el texto como aviso (info/warning) y responde `msg.ack`.
3. `code.clear` limpia el overlay.

## Portapapeles (importante)
- Copia usando el portapapeles nativo desde el lado Go (`golang.design/x/clipboard`) expuesto a
  la UI por un binding de Wails, no la API del navegador: así funciona en conexión local sin HTTPS.
- Prueba que el texto copiado conserva saltos de línea y sangría (clave para Python).

## Interacción con Wails
- El backend Go recibe el mensaje y emite un evento Wails (`runtime.EventsEmit`) al frontend,
  que renderiza el overlay. Define el contrato de eventos en un archivo y documéntalo.

## Privacidad / identidad
- El agente muestra siempre un distintivo visible: "Aula — conectado a la clase de <profesor>
  (canal <X>)". Nada de ejecución oculta.

## Pruebas
- Integración: la consola envía `code.share` y el registro confirma el `code.ack` del agente.
- Prueba del portapapeles con un fragmento multilínea con sangría.

## Criterios de aceptación
- En una VM, emites código desde la consola y el alumno lo copia y pega intacto en su editor.
- `staticcheck ./...` y `go test ./...` pasan.

Actualiza docs/progreso.md.
~~~~

## Verificación manual
- [ ] Copia un fragmento de Python con sangría: ¿se pega sin romperse?
- [ ] ¿El distintivo de identidad es visible para el alumno?
