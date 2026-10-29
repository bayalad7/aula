# 12 · Transmisión de la pantalla del profesor a los alumnos (WebRTC)

**Prerrequisitos:** Hito 1 cerrado (08); conviene tener ya la etapa 09 (miniaturas) para reutilizar la captura.
**Duración estimada:** 3–5 sesiones. Es la función más exigente en red del proyecto.
**Marco:** la transmisión es una acción del profesor y visible para el alumno (overlay con aviso de que está viendo la pantalla del profesor). Nada oculto.

## Prompt

~~~~markdown
Lee CLAUDE.md, docs/protocol.md, docs/privacidad.md y docs/progreso.md. Vamos a implementar la
transmisión en vivo de la pantalla del profesor a todos los alumnos del canal, con WebRTC
(`github.com/pion/webrtc`). Trabaja en dos partes y DETENTE al final de la Parte A.

## Parte A: diseño (detente para que lo apruebe)
1. **Topología.** Analiza y recomienda, para un aula de hasta ~50 alumnos en LAN:
   - Malla (el profesor abre un PeerConnection por alumno) vs. un SFU ligero en el proceso del
     profesor. Compara uso de CPU y subida del profesor. Da una recomendación y un umbral de
     alumnos para cambiar de estrategia.
2. **Captura y códec.** Cómo se alimenta el track de video desde la captura de pantalla
   (reutiliza `internal/platform.ScreenCapture` ampliándolo a cuadros continuos). Resolución y
   fps objetivo (p. ej. 1080p a 10–15 fps para texto de código legible), códec (VP8/H264) y
   control de bitrate. El texto debe leerse nítido; prioriza claridad sobre fluidez.
3. **Señalización.** Como todo corre en LAN, define si usas ICE con candidatos de host sin
   STUN/TURN (lo normal en red local) y documenta el plan B si hay segmentación de red/VLAN.
   Diseña los mensajes de señalización sobre el WebSocket ya existente:
   `screen.broadcast.start` (profesor → alumnos), `screen.broadcast.offer`,
   `screen.broadcast.answer`, `screen.broadcast.ice`, `screen.broadcast.stop`.
   Agrégalos a docs/protocol.md.
4. **Privacidad.** Define el overlay/indicador en el alumno mientras recibe la transmisión y
   agrégalo a docs/privacidad.md.
5. El transmitir la pantalla del profesor deberá poder funcionar teniendo en cuenta los siguientes puntos:
  - Poder seleccionar a ciertos alumnos o a todos
  - Ventana completa que se pueda superponer a todo lo que el alumno tenga abierto de programas
    y no pueda minimizar o cerrar la proyección del profesor: esto sirve para cuando un profesor
    quiere proyectar su presentación de power point, canva, etc el alumno lo vea de la mejor manera
    y no lo pueda cerrar
  - Ventana pequeña que se pueda superponer a todo lo que el alumno tenga abierto de programas,
    pero a diferencia del punto anterior que el alumno pueda hacer que la ventana se pueda volver
    mas grande o mas pequeña "redimensionar (o cambiar el tamaño de la ventana)"
  - Si el profesor tiene más de 1 monitor conectado preguntar cual pantalla quiere presentar a los alumnos
  - Poder detener la presentación de la pantalla compartida del profesor
Espera mi aprobación.

## Parte B: implementación (cuando apruebe)
- Paquete `internal/broadcast` con la lógica WebRTC (sin tocar el SO: la captura entra por la
  interfaz de `platform`). El profesor crea los PeerConnection y publica el track; el alumno lo
  recibe y lo pinta en un `<video>` del frontend Wails.
- Reconexión: si un alumno se cae y vuelve, renegocia solo.
- Control en la consola: botón "Transmitir mi pantalla" / "Detener", con selector de monitor si
  hay varios, y un conteo de cuántos alumnos están recibiendo.
- Límite de recursos: corta o reduce calidad si la CPU del profesor se dispara; documenta el
  comportamiento.

## Pruebas
- Prueba de la señalización (offer/answer/ice) con un doble de PeerConnection.
- Prueba manual en VMs: 2–3 alumnos reciben la pantalla; mide CPU del profesor y ancho de banda.
- Prueba de que "Detener" cierra todos los PeerConnection y quita el overlay en los alumnos.

## Criterios de aceptación
- En VMs, los alumnos ven tu pantalla en vivo con el texto de código legible y el aviso visible.
- Detener la transmisión libera recursos en ambos lados (verifícalo).
- `staticcheck ./...` y `go test ./...` pasan.

Actualiza docs/protocol.md, docs/privacidad.md y docs/progreso.md.
~~~~

## Verificación manual
- [ ] ¿El código proyectado se lee sin pixelarse?
- [ ] Con 3+ alumnos, ¿la CPU y la red de tu laptop aguantan? Anota los números para decidir malla vs. SFU.
