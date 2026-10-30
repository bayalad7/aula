---
titulo: "Contexto del sistema"
descripcion: "Qué piezas forman Aula en un centro de cómputo y quién se conecta con quién"
audiencia: [profesor, instalador, desarrollador]
pantallas: [setup, console]
secciones_manual: [uso-hito1, configuracion]
enlaces: [canal=configuracion, clave=protocol#101-identidades-y-claves, consola=pantalla:console]
---

# Contexto del sistema

¿Qué hay en un laboratorio con Aula y hacia dónde va cada conexión?

```mermaid
%%{init: {"htmlLabels": false, "flowchart": {"htmlLabels": false}}}%%
flowchart LR
    subgraph prof["Equipo del profesor"]
        consola["Aula — Consola<br/>(aula-consola.exe)"]
        clave["Clave del centro<br/>clave-centro.pem"]
        cert["Certificado TLS<br/>autofirmado"]
    end

    subgraph red["Red local del centro"]
        mdns["mDNS _aula._tcp<br/>UDP 5353 multicast"]
    end

    subgraph sala1["Sala principal: canal 1"]
        a1["Aula — Cliente<br/>PC-01 … PC-40"]
    end
    subgraph sala2["Sala alterna: canal 2"]
        a2["Aula — Cliente<br/>PC-41 … PC-52"]
    end

    canal["Canal fijo 1–16000<br/>config.json del alumno"]

    consola -- "anuncia canal, puerto<br/>y huella del certificado" --> mdns
    mdns -- "solo anuncios<br/>de su canal" --> a1
    mdns -. "ignora el canal 1" .-> a2
    a1 -- "wss:// TCP 47830<br/>el alumno conecta" --> consola
    consola -- "video WebRTC<br/>UDP 47831" --> a1
    clave -. "firma el reto" .-> consola
    cert -. "cifra la conexión" .-> consola
    canal -. "instalado por<br/>el administrador" .-> a1

    classDef externo fill:#dbeafe,stroke:#1d4ed8,color:#111827
    classDef aula fill:#dcfce7,stroke:#15803d,color:#111827
    classDef datos fill:#ffedd5,stroke:#c2410c,color:#111827
    classDef seguridad fill:#fee2e2,stroke:#b91c1c,color:#111827
    class consola,a1,a2 aula
    class mdns externo
    class clave,cert seguridad
    class canal datos
```

## Lo que no es obvio

- **Siempre conecta el alumno al profesor.** La consola solo escucha (TCP 47830 por defecto,
  `port` en `docs/configuracion.md`); nunca abre conexiones hacia los equipos de alumno. Por eso
  el firewall que importa es el del equipo del profesor.
- **El canal no es un aula virtual**: es un número fijo de cada computadora de alumno que solo el
  administrador cambia. La consola arranca en su canal predeterminado y el maestro puede
  cambiarlo; si lo cambia al 2, ve a la sala alterna y la principal lo pierde.
- **mDNS solo sirve para encontrar.** Cualquiera en la LAN puede anunciar cualquier canal; la
  confianza la dan el certificado fijado y la firma con la clave del centro (diagrama 04). Una
  sola clave por centro, no por maestro: la tienen todas las consolas, y los alumnos solo su
  parte pública.
- El video de la transmisión de pantalla (diagrama 14) va por un segundo puerto, UDP
  `ws_port + 1`, directo de la consola a cada alumno; su señalización viaja por el WebSocket.
- Si la red bloquea multicast, el alumno puede usar `direct` (`host:puerto`) en su configuración y
  saltarse mDNS.

## Fuentes en el código

- `apps/teacher/main.go`, `apps/teacher/class.go`, `apps/teacher/machine.go`
- `apps/student/main.go`, `apps/student/config.go`
- `internal/discovery/announce.go`, `internal/discovery/browse.go`
- `internal/transport/server.go`, `internal/transport/client.go`
- `internal/security/sitekey.go`, `internal/security/tls.go`
- `internal/broadcast/pion.go`
- `docs/protocol.md` §1 y §10
