---
titulo: "Transparencia: avisos e indicadores al alumno"
descripcion: "Qué ve el alumno en su pantalla cada vez que el profesor lo mira, le transmite, lo restringe o apaga su equipo"
audiencia: [profesor, alumno, desarrollador]
pantallas: [console, screens, restr, power]
secciones_manual: [privacidad]
enlaces: [distintivo=privacidad#reglas-vigentes, visto=pantalla:screens, restr=pantalla:restr, energia=pantalla:power]
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
    end

    subgraph avisos["Lo que ve el alumno (antes de actuar)"]
        distintivo["Distintivo fijo y título de la ventana:<br/>«Aula — conectado (canal X - profesor)»<br/>regla 1"]
        frente["La ventana pasa al frente<br/>con el código o el aviso<br/>regla 4"]
        visto["Barra en rojo: «Tu pantalla está<br/>siendo vista por el profesor»<br/>regla 6"]
        franja["Vista maximizada con franja:<br/>«Estás viendo la pantalla de …»<br/>regla 12"]
        restr["«Restricciones de la clase activas…»<br/>y aviso por cada app cerrada<br/>reglas 20–22"]
        energia["Aviso siempre encima con cuenta<br/>y «Posponer 5 min»; con force,<br/>«El profesor … está apagando»<br/>reglas 26–28"]
    end

    fin["Se quita con stop o clear,<br/>al cerrar la vista o la clase,<br/>o al caerse la conexión"]
    sigue["La cuenta de apagado sigue<br/>sin conexión (regla 30)"]

    conexion --> distintivo
    share --> frente
    thumb --> visto
    bcast --> franja
    pol --> restr
    pwr --> energia
    visto --> fin
    franja --> fin
    restr --> fin
    energia --> sigue

    classDef externo fill:#dbeafe,stroke:#1d4ed8,color:#111827
    classDef aula fill:#dcfce7,stroke:#15803d,color:#111827
    classDef datos fill:#ffedd5,stroke:#c2410c,color:#111827
    classDef seguridad fill:#fee2e2,stroke:#b91c1c,color:#111827
    class conexion,share,thumb,bcast,pol,pwr seguridad
    class distintivo,frente,visto,franja,restr,energia aula
    class fin,sigue externo
```

## Lo que no es obvio

- **El aviso va antes que la acción**, y el código lo hace cumplir: `Streamer` no captura si no
  pudo mostrar el distintivo; `Viewer` muestra la vista antes de procesar la oferta WebRTC;
  `policy.Manager` avisa antes de aplicar; `Countdown` llama a `OnChange` con el aviso antes de
  responder a la consola.
- **Toda orden viene de una consola autenticada** (rojo): nada de esto se procesa antes de
  `auth.ok` (diagrama 04).
- **Al perderse la conexión el alumno no queda vigilado ni restringido**: miniaturas,
  transmisión y restricciones terminan solas. La única excepción es la cuenta regresiva de
  apagado, porque la orden ya llegó de la consola legítima; el alumno aún puede posponerla.
- **Con el agente como servicio** todos estos avisos los muestra el ayudante en la sesión del
  alumno; el servicio no tiene ventana ni puede capturar (reglas 17–18, diagrama 13).
- Lo que el profesor recibe a cambio (acuses) dice «se mostró», nunca «se leyó» ni qué sitios
  intentó abrir el alumno (regla 23).
- La pantalla en blanco (reglas 9–10) está pendiente y no aparece.
- Los textos exactos y las reglas completas están en `docs/privacidad.md`; si cambian, se
  revisa este diagrama.

## Fuentes en el código

- `docs/privacidad.md` reglas 1–31
- `apps/student/app.go` (`setStatus`, `onViewed`, `onBoard`, `onNotice`, `setRestrictions`,
  `appClosed`), `apps/student/events.go`, `apps/student/power.go`, `apps/student/broadcast.go`
- `internal/screen/streamer.go`, `internal/broadcast/viewer.go`, `internal/policy/manager.go`,
  `internal/power/countdown.go`
