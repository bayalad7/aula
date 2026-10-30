---
titulo: "Energía: apagar, reiniciar y cerrar sesión"
descripcion: "Qué pasa en el equipo del alumno desde que el profesor ordena apagar hasta que el equipo se apaga"
audiencia: [profesor, alumno, desarrollador]
pantallas: [power]
secciones_manual: [privacidad, protocol]
---

# Energía: apagar, reiniciar y cerrar sesión

¿Por qué estados pasa una orden `power.action` en el agente, y qué puede hacer el alumno?

```mermaid
%%{init: {"htmlLabels": false}}%%
stateDiagram-v2
    state "Sin orden" as Libre
    state "Cuenta regresiva con aviso" as Cuenta
    state "Ejecutando" as Ejecutando
    state "Falló (el SO la rechazó)" as Fallo
    state sesion <<choice>>

    [*] --> Libre
    Libre --> sesion: power.action
    sesion --> Cuenta: sin force y con sesión, ack countdown
    sesion --> Ejecutando: force, o shutdown/reboot sin sesión
    sesion --> Libre: logoff sin sesión, error unsupported_capability

    Cuenta --> Cuenta: Posponer +5 min (máx. 2), ack postponed
    Cuenta --> Libre: power.cancel, ack cancelled
    Cuenta --> sesion: power.action nueva, ack cancelled de la vieja
    Cuenta --> Ejecutando: llega a 0

    Ejecutando --> [*]: ack executing, espera 2 s y llama al SO
    Ejecutando --> Fallo: error internal con el motivo del SO
    Ejecutando --> Libre: sigue encendido tras 2 min, se quita el aviso
    Fallo --> Libre: el alumno cierra el aviso

    classDef externo fill:#dbeafe,stroke:#1d4ed8,color:#111827
    classDef aula fill:#dcfce7,stroke:#15803d,color:#111827
    classDef datos fill:#ffedd5,stroke:#c2410c,color:#111827
    classDef seguridad fill:#fee2e2,stroke:#b91c1c,color:#111827
    class Libre,Cuenta aula
    class Ejecutando,Fallo seguridad
```

## Lo que no es obvio

- **Una orden a la vez**: una `power.action` nueva reemplaza a la vigente; el agente envía
  `cancelled` de la vieja (con su `ref_id`) y empieza la nueva. `power.cancel` de otra orden, o
  sin orden vigente, se ignora.
- **La cuenta no depende de la red** (regla 30): si se cae la conexión, el aviso sigue, se puede
  posponer y al llegar a 0 se ejecuta. Los `ack` que no salen solo quedan en el log. Tampoco se
  reenvía al reconectar: es una orden puntual.
- **`executing` sale antes de llamar al SO** y el agente espera hasta 2 s para que el acuse
  llegue antes de que se corte la conexión. Todo se registra (acción, `force`, aplazamientos)
  antes de llamar al SO.
- **Sin `force`, Windows deja que las apps pidan guardar** y el alumno puede cancelar en la
  pantalla de Windows; por eso, si el equipo sigue encendido 2 minutos después, el aviso se quita.
- **Quién llama al SO**: sin `force` y con sesión iniciada, el propio agente de la sesión (en modo
  servicio, el ayudante, diagrama 13), porque desde SYSTEM Windows rechaza el apagado con
  usuarios conectados. Con `force`, o sin sesión, el servicio.
- **Sin sesión iniciada** no hay a quién avisar ni trabajo que perder: `shutdown` y `reboot` se
  ejecutan de inmediato y `logoff` no tiene sentido.
- Solo Windows declara `power`.

## Fuentes en el código

- `internal/power/countdown.go` (`Handle`, `start`, `cancel`, `Postpone`, `execute`, `staleAfter`,
  `DefaultFlushWait`)
- `internal/power/tracker.go` (lado de la consola)
- `internal/platform/power.go`, `internal/platform/power_windows.go`
- `apps/student/power.go`, `apps/teacher/power.go`
- `docs/protocol.md` §5.15, `docs/privacidad.md` reglas 26–31, `docs/decisiones.md` D-018
