---
titulo: "Transparencia: avisos e indicadores al alumno"
descripcion: "Qué ve el alumno en su pantalla cada vez que el profesor lo mira, le toma una captura, le transmite, lo restringe o apaga su equipo"
audiencia: [profesor, alumno, desarrollador]
pantallas: [alumnos, ayuda]
secciones_manual: [v1/administracion/como-funciona, privacidad]
enlaces: [distintivo=privacidad#reglas-vigentes, visto=pantalla:alumnos, restr=pantalla:alumnos, energia=pantalla:alumnos]
---

# Transparencia: avisos e indicadores al alumno

¿Qué aviso aparece en el equipo del alumno por cada acción del profesor, y cuándo desaparece?

```mermaid
%%{init: {"htmlLabels": false, "flowchart": {"htmlLabels": false}}}%%
flowchart LR
    subgraph ordenes["Orden de la consola autenticada"]
        conexion["sesión establecida"]
        share["code.share · msg.show"]
        thumb["screen.thumb.start"]
        bcast["screen.broadcast.start"]
        pol["app.block.set · web.filter.set"]
        pwr["power.action"]
        rmt["remote.start"]
        snap["screen.snapshot.request"]
    end

    subgraph avisos["Lo que ve el alumno (antes de actuar)"]
        distintivo["Distintivo fijo y título de la ventana:<br/>«Aula — conectado (canal X - profesor)»<br/>regla 1"]
        frente["La ventana pasa al frente<br/>con el código o el aviso<br/>regla 4"]
        visto["Barra en rojo: «Tu pantalla está<br/>siendo vista por el profesor»<br/>regla 6"]
        franja["Vista maximizada con franja:<br/>«Estás viendo la pantalla de …»<br/>regla 12"]
        restr["«Restricciones de la clase activas…»<br/>y aviso por cada app cerrada<br/>reglas 20–22"]
        energia["Aviso siempre encima con cuenta<br/>y «Posponer 5 min»; con force,<br/>«El profesor … está apagando»<br/>reglas 26–28"]
        control["La ventana pasa al frente con la banda<br/>«… está controlando tu equipo» y el<br/>aviso fijo dice que lo controlan<br/>reglas 36–41"]
        captura["Aviso fijo 5 s «Tu profesor está tomando<br/>una captura…» y después, sin quitar<br/>el foco, «Captura de pantalla» en la esquina<br/>regla 44"]
    end

    fin["Se quita con stop o clear,<br/>al cerrar la vista o la clase,<br/>o al caerse la conexión"]
    puntual["Hecho puntual: el alumno lo cierra<br/>con «Entendido»; la imagen solo se<br/>guarda si el profesor pulsa «Guardar»<br/>reglas 45–47"]
    sigue["La cuenta de apagado sigue<br/>sin conexión (regla 30)"]

    conexion --> distintivo
    share --> frente
    thumb --> visto
    bcast --> franja
    pol --> restr
    pwr --> energia
    rmt --> control
    snap --> captura
    captura --> puntual
    visto --> fin
    franja --> fin
    restr --> fin
    control --> fin
    energia --> sigue

    classDef externo fill:#dbeafe,stroke:#1d4ed8,color:#111827
    classDef aula fill:#dcfce7,stroke:#15803d,color:#111827
    classDef datos fill:#ffedd5,stroke:#c2410c,color:#111827
    classDef seguridad fill:#fee2e2,stroke:#b91c1c,color:#111827
    class conexion,share,thumb,bcast,pol,pwr,rmt,snap seguridad
    class distintivo,frente,visto,franja,restr,energia,control,captura aula
    class fin,sigue,puntual externo
```

## Lo que no es obvio

- **El aviso va antes que la acción**, y el código lo hace cumplir: `Streamer` no captura si no
  pudo mostrar el distintivo; `Viewer` muestra la vista antes de procesar la oferta WebRTC;
  `policy.Manager` avisa antes de aplicar; `Countdown` llama a `OnChange` con el aviso antes de
  responder a la consola; `remote.Agent` muestra la banda y el aviso fijo antes de aceptar entrada;
  `snapshot.Taker` muestra el aviso fijo antes de capturar.
- **La captura es el único aviso que no es una banda**: no sigue activa, así que va como aviso en
  la esquina (`notify`, nunca `raise`). Si el profesor ya ve o controla la pantalla, el aviso fijo
  conserva su texto; el de la esquina aparece igual.
- **Toda orden viene de una consola autenticada** (rojo): nada de esto se procesa antes de
  `auth.ok` (diagrama 04).
- **Al perderse la conexión el alumno no queda vigilado ni restringido**: miniaturas,
  transmisión, control remoto y restricciones terminan solas. La única excepción es la cuenta regresiva de
  apagado, porque la orden ya llegó de la consola legítima; el alumno aún puede posponerla.
- **Con el agente como servicio** todos estos avisos los muestra el ayudante en la sesión del
  alumno; el servicio no tiene ventana ni puede capturar (reglas 17–18, diagrama 13).
- Lo que el profesor recibe a cambio (acuses) dice «se mostró», nunca «se leyó» ni qué sitios
  intentó abrir el alumno (regla 23).
- La pantalla en blanco (reglas 9–10) está pendiente y no aparece.
- Los textos exactos y las reglas completas están en `docs/privacidad.md`; si cambian, se
  revisa este diagrama.

## Fuentes en el código

- `docs/privacidad.md` reglas 1–31 y 36–49
- `apps/student/app.go` (`setStatus`, `onViewed`, `onBoard`, `onNotice`, `setRestrictions`,
  `appClosed`), `apps/student/events.go`, `apps/student/power.go`, `apps/student/broadcast.go`
- `internal/screen/streamer.go`, `internal/broadcast/viewer.go`, `internal/policy/manager.go`,
  `internal/power/countdown.go`, `internal/remote/agent.go`, `apps/student/remote.go`,
  `internal/snapshot/agent.go`, `apps/student/snapshot.go`
