---
titulo: "Transmisión de la pantalla del profesor"
descripcion: "Cómo llega la pantalla del profesor como video a cada alumno y qué pasa si una conexión falla"
audiencia: [profesor, desarrollador]
pantallas: [console]
secciones_manual: [protocol, privacidad]
---

# Transmisión de la pantalla del profesor

¿Cómo se negocia el video WebRTC por la sesión autenticada y cómo se recupera un alumno que
pierde la imagen?

```mermaid
%%{init: {"htmlLabels": false}}%%
sequenceDiagram
    autonumber
    box rgb(220,252,231) Consola
        participant B as Broadcaster y Caster
        participant M as Media (pion, ICE-lite)
    end
    box rgb(220,252,231) Agente
        participant V as broadcast.Viewer
        participant F as Ventana (RTCPeerConnection en WebView2)
    end

    B->>V: screen.broadcast.start {broadcast_id, width, height, fps}
    V->>F: vista maximizada con la franja<br/>«Estás viendo la pantalla de …»
    B->>M: conexión nueva para este agente (attempt 1)
    M-->>B: oferta sendonly, una sola m=video H.264
    B->>V: screen.broadcast.offer {broadcast_id, attempt, sdp}
    V->>V: valida: solo video, sin audio ni datos
    V->>F: oferta
    F-->>V: respuesta recvonly
    V-->>B: screen.broadcast.answer {ref_id, attempt, sdp}
    par trickle ICE, solo candidatos host
        B->>V: screen.broadcast.ice
    and
        V-->>B: screen.broadcast.ice
    end

    rect rgb(219,234,254)
        M->>F: SRTP sobre UDP ws_port+1: H.264, ≤ 15 cuadros/s, ≤ 2,5 Mbit/s
    end

    opt la conexión pasa a failed o queda disconnected más de 5 s
        B->>B: descarta la conexión, espera 1, 2, 4… 30 s
        B->>V: screen.broadcast.offer {attempt + 1}
        V->>F: cierra la anterior, descarta answer e ice viejos
    end

    alt Detener, cambio de canal o fin de la clase
        B->>V: screen.broadcast.stop {broadcast_id}
    else se cierra el WebSocket
        Note over V: el agente cierra WebRTC por sí mismo
    end
    V->>F: quita la vista y la franja
```

## Lo que no es obvio

- **El WebSocket solo lleva la señalización**; el video va directo de la consola a cada alumno por
  WebRTC, dentro de la LAN. La consola codifica una sola vez y comparte la misma pista H.264 con
  todas las conexiones.
- **Sin servidores externos**: no hay STUN ni TURN. La consola es ICE-lite y escucha en un único
  puerto UDP, `ws_port + 1` (47831), que la regla de firewall por programa ya cubre.
- **Recibir no expone al alumno**: el agente rechaza ofertas con audio, canal de datos o más de un
  video, y nunca envía medios propios. Como el SDP (con la huella DTLS) viaja por la sesión
  autenticada, solo acepta video de la consola legítima.
- **La vista va antes que la oferta** (regla 12): si no se puede mostrar, el agente responde
  `error internal` y no procesa la oferta.
- **El reintento no tumba la sesión**: cada intento lleva un `attempt` mayor y el agente descarta
  lo de intentos viejos. Si el agente se reconecta mientras la transmisión sigue, la consola le
  reenvía `start` y `offer`.
- Una transmisión a la vez por consola; no se graba en ningún extremo. Solo Windows declara
  `screen.broadcast`, y solo si el WebView2 tiene `RTCPeerConnection` y H.264.

## Fuentes en el código

- `internal/broadcast/broadcaster.go`, `internal/broadcast/caster.go`,
  `internal/broadcast/pion.go`, `internal/broadcast/sdp.go`, `internal/broadcast/source.go`
- `internal/broadcast/viewer.go`
- `internal/platform/broadcast.go`, `internal/platform/h264_windows.go`
- `apps/teacher/broadcast.go`, `apps/student/broadcast.go`
- `docs/protocol.md` §5.12, `docs/privacidad.md` reglas 12–16, `docs/decisiones.md` D-014
