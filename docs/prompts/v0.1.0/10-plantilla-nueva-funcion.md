# 10 · Plantilla reutilizable para cada función nueva

**Qué es:** no es una etapa única, sino una plantilla. Cópiala y rellénala cada vez que agregues una función de las Fases 2–3 (transmitir tu pantalla por WebRTC, recoger código/archivos, bloquear apps, filtrar web, encuestas, apagar/reiniciar, control remoto).

**Cómo usarla:** sustituye `<FUNCIÓN>` y los corchetes, y pégala como prompt.

## Prompt (plantilla)

~~~~markdown
Lee CLAUDE.md, docs/protocol.md, docs/privacidad.md y docs/progreso.md. Vamos a agregar la
función: <FUNCIÓN>. Sigue el flujo del proyecto: protocolo primero, luego interfaz de
plataforma, luego Windows, después la UI de la app que corresponda, y Linux/macOS solo si lo indico.

## 1. Diseño (detente para que lo apruebe)
- Describe los mensajes nuevos del protocolo que necesita <FUNCIÓN> (tipo, dirección, payload,
  respuesta) y agrégalos a docs/protocol.md.
- Di qué `capability` declara el agente y en qué SO estará disponible.
- Si <FUNCIÓN> observa o controla al alumno, describe el indicador/aviso visible y agrégalo a
  docs/privacidad.md.
Espera mi aprobación.

## 2. Implementación (cuando apruebe)
- Define o extiende la interfaz en `internal/platform` con fallback "no soportado" por defecto,
  y archivos `*_windows.go` para la implementación.
- Conecta la lógica en los paquetes `internal/*` que correspondan y la UI en la app (binding +
  evento Wails). No metas lógica de SO fuera de `platform`.
- Respeta la transparencia: nada de funciones ocultas; el alumno siempre puede saber qué pasa
  en su equipo.

## 3. Pruebas y cierre
- Pruebas de la lógica en `internal/*` y de la interfaz de plataforma (con un doble de prueba).
- Prueba manual descrita paso a paso para verificar en VMs.
- `gofmt`, `go vet`, `staticcheck ./...`, `go test ./...`.
- Actualiza docs/progreso.md.

## Restricciones
- Cambios incrementales; no toques paquetes ajenos a esta función.
- Si <FUNCIÓN> requiere permisos especiales (accesibilidad, grabación de pantalla, Polkit/DBus
  en Linux), documenta cómo se conceden en un laboratorio gestionado.
~~~~

## Backlog sugerido (orden recomendado)

**Fase 2**
1. Transmitir la pantalla del profesor a los alumnos (WebRTC con `pion/webrtc`; señalización
   `screen.broadcast.*` por el WebSocket).
2. Recoger el código de cada alumno y recoger archivos (`code.collect`, `file.collect`).
3. Chat/comandos globales y "levantar la mano" / encuestas rápidas (`poll.*`).
4. Agente como **servicio del sistema** en Windows (patrón servicio + ayudante en la sesión del
   usuario para captura y overlay), con su interfaz `ServiceControl` en platform.

**Fase 3**
5. Bloquear aplicaciones y filtrar sitios web (filtro DNS o proxy local en el agente).
6. Apagar / reiniciar equipos (`power.*`).
7. Control remoto de un equipo (permisos de accesibilidad; aviso visible permanente).

**Portabilidad**
8. Implementaciones de Linux (X11 primero) y macOS de cada interfaz de `platform`, más
   empaquetado `.deb`/`.rpm`/AppImage y `.dmg`/`.pkg` con sus perfiles de permisos.

**Funciones propias para programación (tu ventaja sobre LanSchool)**
9. Enviar plantillas de ejercicio a todos.
10. Ejecutar las entregas en un contenedor Docker desechable (sin red, con límites) y devolver
    el resultado de las pruebas a la consola.
11. Ver en vivo, como texto, el avance del alumno en un ejercicio.
