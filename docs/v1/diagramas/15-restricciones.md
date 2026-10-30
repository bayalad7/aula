---
titulo: "Restricciones de la clase: apps y filtro web"
descripcion: "Cómo se aplican el bloqueo de aplicaciones y el filtro web, y por qué siempre se pueden deshacer"
audiencia: [profesor, desarrollador, auditor]
pantallas: [restr]
secciones_manual: [privacidad, protocol]
enlaces: [consola=pantalla:restr, diario=configuracion]
---

# Restricciones de la clase: apps y filtro web

¿Qué hace el agente al recibir una política, y qué garantiza que el equipo quede como estaba?

```mermaid
%%{init: {"htmlLabels": false, "flowchart": {"htmlLabels": false}}}%%
flowchart TD
    consola["ClassPolicy en la consola<br/>a todo el canal o a equipos concretos"]
    set["app.block.set · web.filter.set"]
    aviso["Aviso «Restricciones de la clase activas»<br/>antes de aplicar (regla 20)"]
    tipo{"¿Tipo?"}

    apps["Cierra las apps de la lista en la sesión del alumno<br/>y vigila las que se abren (menos de 1 s)<br/>protegidas: el agente, WebView2, sistema, explorer"]

    diario[("politica-activa.json<br/>NRPT y políticas DoH como estaban")]
    resolver["Resolver DNS local de Aula<br/>allow o deny por dominio y subdominios"]
    pasos["1. lee los DNS del equipo<br/>2. arranca el resolver<br/>3. guarda el diario<br/>4. desactiva DoH del navegador<br/>5. regla NRPT al resolver y vacía la caché"]

    ack["app.block.ack · web.filter.ack<br/>{policy_id, active}"]
    error["error internal<br/>y se deshace todo"]

    quitar["clear, cierre de la conexión,<br/>salida del ayudante, servicio detenido"]
    teardown["teardown en orden inverso<br/>borra el diario solo si todo se deshizo"]
    arranque["Al volver a arrancar tras un corte:<br/>Restore antes de conectarse"]

    consola --> set --> aviso --> tipo
    tipo -- "apps" --> apps --> ack
    tipo -- "web" --> pasos
    pasos --> resolver
    pasos --> diario
    pasos -- "todo bien" --> ack
    pasos -- "falla un paso" --> error
    ack --> consola
    quitar --> teardown
    diario --> arranque --> teardown

    classDef externo fill:#dbeafe,stroke:#1d4ed8,color:#111827
    classDef aula fill:#dcfce7,stroke:#15803d,color:#111827
    classDef datos fill:#ffedd5,stroke:#c2410c,color:#111827
    classDef seguridad fill:#fee2e2,stroke:#b91c1c,color:#111827
    class consola,aviso,tipo,apps,resolver,pasos,ack,teardown,arranque aula
    class set,error seguridad
    class diario datos
    class quitar externo
```

## Lo que no es obvio

- **Una política por tipo, de la clase**: un `set` nuevo reemplaza al anterior. La consola lleva
  qué tiene cada alumno y **la reenvía al reconectar**; el agente nunca la conserva entre
  sesiones.
- **El diario se escribe antes de tocar el navegador y el DNS**, así que un corte de luz a mitad
  de clase no deja el equipo filtrado: al arrancar, el agente quita la regla NRPT marcada como de
  Aula y restaura las políticas guardadas antes de conectarse. El bloqueo de apps no cambia nada
  persistente y no necesita diario.
- **El filtro es por nombre** (DNS): no mira rutas ni contenido, no descifra HTTPS y no registra
  qué sitios se consultaron (regla 23). Bloquea siempre los servidores DoH conocidos, que lo
  saltarían.
- **Lo que no se puede garantizar** se le dice al profesor: un administrador local puede detener
  el agente, renombrar un ejecutable evita el bloqueo por nombre y entrar por IP evita el filtro
  (regla 25).
- Solo Windows y solo con el agente como servicio y su ayudante (diagrama 13): sin ellos el
  agente no declara `app.block` ni `web.filter`.

## Fuentes en el código

- `internal/policy/manager.go` (`Handle`, `applyWeb`, `teardown`, `Clear`, `Restore`)
- `internal/policy/apps.go`, `internal/policy/web.go`, `internal/policy/resolver.go`,
  `internal/policy/journal.go`, `internal/policy/class.go`
- `internal/platform/policy.go`, `internal/platform/policy_windows.go`
- `apps/teacher/restrictions.go`, `apps/student/service.go` (`newPolicyManager`, `policyJournal`)
- `docs/protocol.md` §5.13–5.14, `docs/privacidad.md` reglas 20–25, `docs/decisiones.md` D-017
