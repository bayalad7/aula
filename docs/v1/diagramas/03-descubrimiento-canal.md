---
titulo: "Descubrimiento de canal (mDNS)"
descripcion: "Cómo un equipo de alumno encuentra la consola de su sala entre varias de la misma red"
audiencia: [instalador, desarrollador]
pantallas: [setup, console]
secciones_manual: [configuracion, prueba-vms]
---

# Descubrimiento de canal (mDNS)

¿Cómo encuentra el alumno la consola de **su** canal, y qué hace con las demás?

```mermaid
%%{init: {"htmlLabels": false}}%%
sequenceDiagram
    autonumber
    box rgb(220,252,231) Equipo del profesor
        participant C as Consola (Announcer)
    end
    box rgb(219,234,254) Red local
        participant M as mDNS 224.0.0.251:5353
    end
    box rgb(220,252,231) Equipo del alumno (canal 1)
        participant B as Browser
        participant A as Agent
    end

    C->>M: Registro _aula._tcp (zeroconf)<br/>TXT proto=1, channel=1, console_name,<br/>ws_port=47830, cert_fingerprint=sha256:…
    loop cada 5 s, por cada interfaz (D-015)
        C->>M: Mismo anuncio con IP_MULTICAST_IF
    end
    Note over C,M: Otra consola anuncia channel=2 en la misma red

    loop consulta nueva cada 30 s
        B->>M: Pregunta PTR _aula._tcp.local.
        M-->>B: Respuestas de todas las consolas
        B->>B: parseEntry: descarta proto ≠ 1<br/>y channel ≠ 1 (comparación exacta)
        B->>B: orderAddrs: misma subred primero,<br/>IPv4 antes que IPv6, sin IPv6 link-local
        B-->>A: Console{Instance, Name, Addrs, Port, CertFingerprint}<br/>(una vez, y otra si cambian sus direcciones)
    end

    alt ninguna consola del canal en 10 s
        A->>A: diagnóstico «No se encontró ninguna consola del canal 1…»<br/>y sigue buscando
    else una o varias del canal
        A->>A: pick: la siguiente que no esté marcada<br/>como no confiable (5 min)
        Note over A: Conexión y autenticación: diagrama 04
    end
```

## Lo que no es obvio

- **El filtro por canal es local y exacto.** La consola anuncia su canal en los TXT; cada alumno
  descarta los que no coinciden con el suyo. Varias salas conviven en la misma red sin verse.
- **mDNS no es confiable**: cualquiera puede anunciar el canal 1 con cualquier huella. El
  descubrimiento solo produce *candidatos*; la confianza la decide la firma (diagrama 04). Una
  consola que falla la firma queda marcada como no confiable 5 minutos y el agente prueba la
  siguiente.
- **Por qué se reanuncia por interfaz** (D-015): en Windows zeroconf no elige la interfaz de
  salida, y en un equipo con Hyper-V el anuncio salía por el conmutador virtual. La consola
  envía además el anuncio en cada interfaz con `IP_MULTICAST_IF`.
- **Por qué se reinicia la consulta cada 30 s**: zeroconf deja de preguntar tras la primera
  respuesta; sin reiniciar, una consola abierta después nunca se vería.
- Tras 3 fallos seguidos contra la misma consola, el agente vuelve a descubrir (pudo cambiar de
  IP o de puerto). Con `direct` en la configuración no hay mDNS: `StaticSource` entrega una
  dirección fija.
- La red debe ser Privada o de Dominio: en perfil Público las reglas de firewall no cubren mDNS
  (D-012).

## Fuentes en el código

- `internal/discovery/announce.go`, `internal/discovery/multicast.go`
- `internal/discovery/browse.go`, `internal/discovery/console.go`
- `internal/session/agent.go` (`Run`, `pick`, `waitConsole`, `StaticSource`)
- `internal/protocol/channel.go`
- `docs/protocol.md` §1.1 y §1.3
