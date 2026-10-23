# 07 · Módulo C — App profesor: lista de alumnos, hotkey de captura y emisión

**Prerrequisitos:** etapa 06 completa.
**Duración estimada:** 1–2 sesiones.

## Prompt

~~~~markdown
Lee CLAUDE.md, docs/protocol.md y docs/progreso.md. Construye la interfaz del profesor sobre el
core del Módulo A.

## Pantallas (frontend Wails)
1. **Selección de canal/aula** al iniciar (crear o elegir: `AULA_101`, `CURSO_PYTHON_A`…).
2. **Lista de alumnos conectados** al canal
   - Tarjeta por alumno: equipo, usuario, SO, estado (conectado/desconectado) y capacidades.
   - Se actualiza en vivo vía eventos Wails cuando un agente entra o cae.
3. **Panel de código**
   - Editor Monaco con selector de lenguaje y campo de título.
   - "Emitir a todos" (`code.share` replace), "Agregar bloque" (append), "Limpiar pantallas" (`code.clear`).
   - Historial de snippets de la sesión para reemitir uno anterior con un clic ("Ejercicio 1", "2"…).
4. **Mensajes**: enviar un aviso (`msg.show`) a todos, nivel info/warning.

## Captura rápida de código (diferenciador)
- Implementa un **hotkey global** (vía `internal/platform`, interfaz con impl. de Windows ahora
  y fallback en los demás) que tome el contenido actual del portapapeles y lo cargue en el panel
  de emisión, para capturar código mientras presentas en PowerPoint o el editor.
- Botón flotante equivalente al hotkey.
- Permite ajustar lenguaje antes de emitir; no emitas automáticamente sin confirmación.

## Detalles
- Acciones deshabilitadas si el alumno no soporta la capacidad (lee `capabilities` del `hello`).
- Muestra confirmación visual con el conteo de `code.ack` / `msg.ack` recibidos.
- El estado (alumnos, historial) vive en el backend Go; el frontend solo lo refleja.

## Pruebas
- Prueba del binding que emite un snippet: verifica que llama al transport/registry correctamente.
- Prueba del historial: emitir, agregar y reemitir un snippet anterior.
- Prueba del hotkey detrás de su interfaz (con un doble de prueba del portapapeles).

## Criterios de aceptación
- Con dos agentes en VMs, emites código y ves los dos acuses de recibo.
- El hotkey captura el portapapeles y lo deja listo para emitir.
- `staticcheck ./...` y `go test ./...` pasan.

Actualiza docs/progreso.md.
~~~~
