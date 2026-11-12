# 09 · Módulo B — Monitoreo: miniaturas y pantalla en blanco

**Prerrequisitos:** Hito 1 cerrado (etapa 08).
**Duración estimada:** 2–4 sesiones.
**Marco:** esto convierte a Aula en gestión de aula al estilo Veyon. El monitoreo es **transparente y con aviso al alumno**, nunca vigilancia oculta.

## Prompt

~~~~markdown
Lee CLAUDE.md, docs/protocol.md, docs/privacidad.md y docs/progreso.md. Empezamos el Módulo B en
Windows: miniaturas del escritorio del alumno y pantalla en blanco (freeze). Define primero las
interfaces en `internal/platform` y luego la implementación de Windows; deja fallback
"no soportado" para Linux y macOS por ahora.

## Principio de transparencia (no negociable)
- Mientras la pantalla de un alumno se ve o transmite, su equipo muestra un indicador visible y
  claro ("Tu pantalla está siendo vista por el profesor").
- El agente se identifica siempre; sin modo oculto ni evasión de detección.
- No se guardan capturas en disco salvo petición explícita del profesor.
- Documenta estas reglas y los textos de aviso en docs/privacidad.md y que el código las cumpla.

## Interfaces en platform
1. `ScreenCapture`: captura a tamaño miniatura (ancho ~320 px) y, aparte, a tamaño mayor para
   ver un equipo en detalle. Implementación Windows con `github.com/kbinani/screenshot`; comprime a JPEG.
2. `ScreenOverlay`: muestra/oculta una superposición a pantalla completa que bloquea la
   interacción del alumno, con un texto que explica el porqué.

## Protocolo (documentar en docs/protocol.md antes de implementar)
- `screen.thumb.start` (profesor → alumno: intervalo, calidad), `screen.thumb.frame`
  (alumno → profesor: JPEG), `screen.thumb.stop`. Máximo 1–2 cuadros/seg para no saturar la red.
- `screen.freeze` / `screen.unfreeze` (profesor → alumno), con `screen.freeze.ack`.

## Consola del profesor
- Cuadrícula de miniaturas (una por alumno), actualizándose en vivo; clic abre la vista grande.
- La transmisión de miniaturas corre solo mientras el profesor tiene la vista abierta
  (`screen.thumb.start` al abrir, `stop` al cerrar).
- Botones "Pantalla en blanco" / "Reanudar" por alumno y para todo el grupo.

## Pruebas
- `screen.thumb.start` inicia el envío; `stop` lo detiene (verificable).
- `screen.freeze` activa el overlay y `unfreeze` lo quita.
- Rendimiento: mide CPU y ancho de banda con 20 agentes simulados.

## Criterios de aceptación
- En VMs Windows ves las miniaturas y el aviso visible en el equipo del alumno.
- El freeze bloquea y explica; reanudar lo libera.
- Detener la vista detiene la captura.
- `staticcheck ./...` y `go test ./...` pasan.

Actualiza docs/progreso.md y docs/privacidad.md.
~~~~

## Verificación manual
- [ ] ¿El aviso al alumno aparece siempre que ves o congelas su pantalla?
- [ ] ¿La CPU del equipo del alumno se mantiene razonable?

> **Siguiente (streaming de tu pantalla a los alumnos):** cuando lo abordes, usa WebRTC
> (`pion/webrtc`) y define en el protocolo los mensajes de señalización (`screen.broadcast.*`).
> Es la función más exigente en red; trátala con la plantilla del archivo 10.
