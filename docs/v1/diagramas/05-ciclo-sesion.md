---
titulo: "Ciclo de sesión y heartbeat"
descripcion: "Por qué estados pasa la conexión de un alumno, cuándo se da por caída y cómo vuelve sola"
audiencia: [desarrollador]
pantallas: [console]
secciones_manual: [protocol]
---

# Ciclo de sesión y heartbeat

¿En qué estados está la conexión de un agente, qué la tumba y cómo se recupera?

```mermaid
%%{init: {"htmlLabels": false}}%%
stateDiagram-v2
    state "Buscando consola" as Buscando
    state "Conectando TLS" as Conectando
    state "Autenticando" as Autenticando
    state "Presentándose (hello)" as Hello
    state "Conectado" as Conectado
    state "Esperando (retroceso)" as Espera
    state "Consola no confiable 5 min" as NoConfiable

    [*] --> Buscando
    Buscando --> Conectando: consola del canal descubierta
    Buscando --> Buscando: 10 s sin consola, diagnóstico y sigue
    Conectando --> Autenticando: huella fijada OK
    Conectando --> NoConfiable: huella distinta
    Conectando --> Espera: dirección inalcanzable
    Autenticando --> Hello: firma válida, auth.ok
    Autenticando --> NoConfiable: firma inválida
    Autenticando --> Espera: 10 s sin respuesta o cierre
    Hello --> Conectado: hello enviado, fallos = 0
    Hello --> Espera: no se pudo enviar
    NoConfiable --> Espera
    Conectado --> Espera: cierre o 15 s de silencio

    state Conectado {
        [*] --> Latiendo
        Latiendo --> Latiendo: heartbeat cada 5 s en ambos sentidos
        Latiendo --> Atendiendo: mensaje de la consola
        Atendiendo --> Latiendo: ack o error
    }

    Espera --> Conectando: min(30 s, 1 s × 2^n) ± 20 %
    Espera --> Buscando: cada 3 fallos seguidos, descubrir de nuevo

    classDef externo fill:#dbeafe,stroke:#1d4ed8,color:#111827
    classDef aula fill:#dcfce7,stroke:#15803d,color:#111827
    classDef datos fill:#ffedd5,stroke:#c2410c,color:#111827
    classDef seguridad fill:#fee2e2,stroke:#b91c1c,color:#111827
    class Buscando externo
    class Conectando,Autenticando,NoConfiable seguridad
    class Hello,Conectado,Espera aula
```

## Lo que no es obvio

- **Lado del agente.** El diagrama es la máquina de `session.Agent.Run`. La consola no reconecta:
  solo acepta conexiones y lleva un registro con dos estados por sesión, `conectado` y
  `desconectado`.
- **`n` es el número de fallos consecutivos** y vuelve a 0 solo cuando la sesión se estableció
  (se envió `hello`). Secuencia típica: 1, 2, 4, 8, 16, 30, 30… s, con ±20 % de azar para que
  40 equipos no reconecten en el mismo instante tras reiniciar la consola.
- **Silencio de 15 s** (3 latidos perdidos): cualquier mensaje cuenta como señal de vida, no solo
  `heartbeat`. Lo aplican los dos extremos: `transport` cierra la conexión muda y el registro de
  la consola marca la sesión `desconectado` en su barrido (`Registry.Sweep`).
- **Al reconectar es una sesión nueva**, pero la consola reemplaza la tarjeta vieja del mismo
  equipo en vez de duplicarla, y el agente conserva el código que mostraba. Al cerrarse la
  sesión, el agente detiene miniaturas, transmisión y restricciones; la cuenta regresiva de
  apagado sigue (diagramas 08, 14–16).
- Una consola que falló la autenticación se salta 5 minutos aunque siga anunciándose; si todas
  las conocidas están marcadas, el agente espera con el mismo retroceso.

## Fuentes en el código

- `internal/session/agent.go` (`Run`, `session`, `authenticate`, `untrustedFor`,
  `failuresBeforeRediscover`)
- `internal/session/backoff.go`
- `internal/session/registry.go` (`Connect`, `Touch`, `Sweep`, `Disconnect`)
- `internal/session/console.go`
- `internal/transport/conn.go` (heartbeat y cierre por silencio)
- `docs/protocol.md` §3.4 y §3.5
