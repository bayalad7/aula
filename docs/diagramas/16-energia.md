---
titulo: "Energía: apagar, reiniciar, cerrar sesión y encender"
descripcion: "Qué pasa en el equipo del alumno desde que el profesor ordena apagar hasta que se apaga, y cómo la consola lo vuelve a encender"
audiencia: [profesor, alumno, desarrollador]
pantallas: [alumnos]
secciones_manual: [administracion/tareas-frecuentes, privacidad, protocol]
---

# Energía: apagar, reiniciar, cerrar sesión y encender

¿Por qué estados pasa una orden `power.action` en el agente, qué puede hacer el alumno, y cómo
vuelve a encenderse un equipo apagado (Wake-on-LAN, desde la consola)?

```mermaid
%%{init: {"htmlLabels": false}}%%
stateDiagram-v2
    state "Sin orden" as Libre
    state "Cuenta regresiva con aviso" as Cuenta
    state "Ejecutando" as Ejecutando
    state "Falló (el SO la rechazó)" as Fallo
    state "Apagado, sin agente" as Apagado
    state "Encendiendo (la consola espera 3 min)" as Encendiendo
    state "No respondió" as NoRespondio
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

    Ejecutando --> [*]: reboot o logoff, ack executing y llama al SO
    Ejecutando --> Apagado: shutdown, ack executing y llama al SO
    Apagado --> Encendiendo: Encender, paquete mágico UDP 9 al broadcast
    Encendiendo --> Libre: el agente arranca y manda hello con su MAC
    Encendiendo --> NoRespondio: 3 min sin hello
    NoRespondio --> Encendiendo: Encender otra vez
    Ejecutando --> Fallo: error internal con el motivo del SO
    Ejecutando --> Libre: sigue encendido tras 2 min, se quita el aviso
    Fallo --> Libre: el alumno cierra el aviso

    classDef externo fill:#dbeafe,stroke:#1d4ed8,color:#111827
    classDef aula fill:#dcfce7,stroke:#15803d,color:#111827
    classDef datos fill:#ffedd5,stroke:#c2410c,color:#111827
    classDef seguridad fill:#fee2e2,stroke:#b91c1c,color:#111827
    class Libre,Cuenta aula
    class Ejecutando,Fallo seguridad
    class Apagado,Encendiendo,NoRespondio externo
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
- **Encender no es un mensaje del protocolo** (§5.15.2): el equipo apagado no tiene agente. La
  consola recuerda la MAC que cada agente manda en `hello` (`equipos-conocidos.json`, por canal) y
  manda el paquete mágico (6 × `FF` y 16 × la MAC) 3 veces, al broadcast de su tarjeta que está
  en la subred del equipo. Si ninguna lo está, prueba con `255.255.255.255` y avisa «Otra red»: el
  broadcast no cruza routers. No hay acuse: solo se sabe que encendió cuando llega su `hello`.
- **Nunca a simulados:** no tienen MAC y la consola no los recuerda.

## Fuentes en el código

- `internal/power/countdown.go` (`Handle`, `start`, `cancel`, `Postpone`, `execute`, `staleAfter`,
  `DefaultFlushWait`)
- `internal/power/tracker.go` (lado de la consola)
- `internal/power/wake.go` (`MagicPacket`, `AgentMACs`, `RoutesFor`, `PlanWake`),
  `internal/power/machines.go` (`Machines`), `internal/power/waker.go` (`Waker`, `WakeTimeout`)
- `internal/platform/nic.go` (`Interfaces`), `apps/teacher/wake.go`
- `internal/platform/power.go`, `internal/platform/power_windows.go`
- `apps/student/power.go`, `apps/teacher/power.go`
- `docs/protocol.md` §5.15 y §5.15.2, `docs/privacidad.md` reglas 26–31 bis, `docs/decisiones.md` D-018
