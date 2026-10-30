---
titulo: "Componentes y dependencias"
descripcion: "Cómo se reparten las dos aplicaciones en paquetes y por qué solo uno toca el sistema operativo"
audiencia: [desarrollador]
pantallas: []
secciones_manual: [decisiones]
---

# Componentes y dependencias

¿Qué paquete hace qué, de quién depende y dónde está la frontera con el sistema operativo?

```mermaid
%%{init: {"htmlLabels": false, "flowchart": {"htmlLabels": false}}}%%
flowchart TB
    teacher["apps/teacher<br/>consola Wails"]
    student["apps/student<br/>agente Wails, servicio y ayudante"]

    subgraph sinso["Sin llamadas al SO"]
        session["session<br/>registro, handshake, reconexión"]
        transport["transport<br/>WebSocket coder/websocket"]
        discovery["discovery<br/>mDNS zeroconf"]
        protocol["protocol<br/>envoltura, payloads, límites"]
        codedist["codedist<br/>historial, acuses, tablero"]
        screen["screen<br/>miniaturas"]
        broadcast["broadcast<br/>WebRTC pion"]
        policy["policy<br/>apps, filtro DNS, diario"]
        power["power<br/>cuenta regresiva"]
        agentsvc["agentsvc<br/>supervisor y canal local"]
    end

    security["security<br/>Ed25519, TLS, huella"]

    subgraph so["Frontera con el SO"]
        platform["platform<br/>interfaces Go"]
        win["*_windows.go<br/>captura, portapapeles, servicio,<br/>named pipe, NRPT, apagado"]
        other["*_other.go<br/>no soportado"]
    end

    sistema["Windows · Linux · macOS"]

    teacher --> session & codedist & screen & broadcast & policy & power & platform
    student --> session & codedist & screen & broadcast & policy & power & agentsvc & platform
    session --> transport & discovery & security & protocol
    transport --> protocol
    discovery --> protocol
    codedist --> protocol
    screen --> session & platform
    broadcast --> session & platform
    policy --> session & platform
    power --> platform & protocol
    agentsvc --> session & platform
    security --> platform & protocol
    platform --> win & other
    win --> sistema

    classDef externo fill:#dbeafe,stroke:#1d4ed8,color:#111827
    classDef aula fill:#dcfce7,stroke:#15803d,color:#111827
    classDef datos fill:#ffedd5,stroke:#c2410c,color:#111827
    classDef seguridad fill:#fee2e2,stroke:#b91c1c,color:#111827
    class teacher,student,session,transport,discovery,protocol,codedist,screen,broadcast,policy,power,agentsvc,platform,win,other aula
    class security seguridad
    class sistema externo
```

## Lo que no es obvio

- **La regla de oro**: `protocol`, `discovery`, `transport`, `session`, `codedist`, `screen`,
  `broadcast`, `agentsvc`, `policy` y `power` no llaman APIs del SO. Lo que toca pantalla,
  portapapeles, teclado, servicios, DNS o energía pasa por una interfaz de `internal/platform`,
  con una implementación por SO (`*_windows.go`) y un fallback `*_other.go` que devuelve
  `ErrNotSupported`. Por eso esos paquetes se prueban con dobles en los tres SO del CI.
- `discovery` y `transport` sí usan red (sockets UDP y TCP de la biblioteca estándar y sus
  librerías), que es portable; no es "SO" en el sentido de la regla.
- `security` usa `platform` solo para proteger archivos con la ACL correcta
  (`RestrictToOwner`, `RestrictMachineSecret`).
- `agentsvc` no importa `go-winio`: el named pipe entre servicio y ayudante lo abre
  `platform.ListenLocal`; `agentsvc` solo ve un `net.Conn`.
- Las apps solo consumen `internal/*`; no hay dependencias entre `apps/teacher` y
  `apps/student`. Las flechas son los `import` reales (`go list`), simplificadas: se omite
  `protocol` cuando ya llega por otro paquete.

## Fuentes en el código

- `internal/*/doc.go` de cada paquete
- `internal/platform/platform.go` (`ErrNotSupported`) y sus pares `*_windows.go` / `*_other.go`
- `apps/teacher/main.go`, `apps/student/main.go`
- `go.mod`
- CLAUDE.md, sección «Arquitectura»
