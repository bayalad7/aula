---
titulo: "Autenticación y fijado de certificado"
descripcion: "Cómo el agente comprueba que habla con la consola del profesor y no con un impostor"
audiencia: [desarrollador, auditor]
pantallas: [setup]
secciones_manual: [protocol]
---

# Autenticación y fijado de certificado

¿Qué comprueba el agente antes de obedecer a una consola, y en qué orden?

```mermaid
%%{init: {"htmlLabels": false}}%%
sequenceDiagram
    autonumber
    box rgb(220,252,231) Equipo del alumno
        participant A as Agent
    end
    box rgb(220,252,231) Equipo del profesor
        participant C as Console
    end

    Note over A: Candidata del canal (diagrama 03)<br/>con cert_fingerprint anunciada

    rect rgb(254,226,226)
        A->>C: TLS 1.3, ClientHello (wss://host:47830/ws)
        C-->>A: Certificado autofirmado ECDSA P-256
        A->>A: ClientTLS: SHA-256 del certificado<br/>== cert_fingerprint anunciada
        alt la huella no coincide
            A->>A: cierra sin enviar nada,<br/>consola no confiable 5 min
        end
    end

    rect rgb(254,226,226)
        A->>C: auth.challenge {nonce: 32 bytes aleatorios}
        C->>C: SignProof con la clave del centro:<br/>"aula-auth-v1" ‖ 0 ‖ nonce ‖ 0 ‖ canal ‖ 0 ‖ cert_sha256
        C-->>A: auth.proof {ref_id, signature}
        A->>A: VerifyProof con console_public_key<br/>y la huella que vio el agente
        alt firma inválida u otro mensaje
            A->>C: error unauthorized
            A->>C: cierre 1008
            A->>A: consola no confiable 5 min,<br/>diagnóstico «su clave no es la de este centro»
        end
        A->>C: auth.ok {ref_id}
    end

    A->>C: hello {hostname, username, os, os_version,<br/>agent_version, capabilities}
    C->>C: Registry.Connect, StartHeartbeat
    Note over A,C: Sesión autenticada (diagrama 05).<br/>Todo el handshake debe terminar en 10 s.
```

## Lo que no es obvio

- **El agente se autentica primero y se presenta después.** `hello` (con equipo y usuario) solo
  sale tras `auth.ok`: una consola sin la clave del centro nunca sabe quién es el alumno.
- **Qué liga cada parte de la firma**: la etiqueta `aula-auth-v1` impide reutilizar una firma de
  otro contexto; el `nonce` nuevo por conexión impide repetir una prueba vieja; el `canal` liga la
  prueba a la sala; y `cert_sha256` es la huella del certificado **que vio el agente**: si un
  intermediario reenvía el reto a la consola legítima, esta firma *su* huella, no la del
  intermediario, y la verificación falla.
- **Conexión directa** (`direct`, sin mDNS): no hay huella anunciada que fijar, así que el agente
  acepta el certificado presentado, pero la firma sigue ligada a esa huella; un intermediario con
  su propio certificado no puede producir una firma válida.
- Antes de `auth.ok` el agente solo acepta `auth.proof` y `error`; la consola solo
  `auth.challenge`, `auth.ok` y `error`. Cualquier otra cosa → `error unauthorized` y cierre
  `1008` (nada llega a `OnMessage`).
- **Lo que no cubre v0**: la consola no autentica al agente (cualquier equipo con el agente puede
  unirse al canal; el profesor ve equipo y usuario), y quien robe la clave del centro puede
  suplantar consolas. Ver `docs/protocol.md` §10.3.
- Nunca se registran el nonce, la firma ni las claves.

## Fuentes en el código

- `internal/session/agent.go` (`session`, `authenticate`)
- `internal/session/console.go` (`handshake`)
- `internal/security/tls.go` (`ClientTLS`, `ServerTLS`, `LoadOrCreateCertificate`)
- `internal/security/auth.go` (`SignProof`, `VerifyProof`), `internal/security/identity.go`
- `internal/security/sitekey.go`
- `docs/protocol.md` §3.2 y §10
