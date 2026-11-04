---
titulo: "Diagnóstico: «Mi alumno no conecta»"
descripcion: "Preguntas en orden para averiguar por qué un equipo de alumno no aparece en la consola"
audiencia: [profesor, instalador]
pantallas: [falta]
secciones_manual: [v1/administracion/solucion-de-problemas, uso-hito1, prueba-vms, configuracion]
enlaces: [config=configuracion, firewall=uso-hito1#7-red-y-firewall, clave=protocol#101-identidades-y-claves]
---

# Diagnóstico: «Mi alumno no conecta»

¿Qué reviso, y en qué orden, cuando un equipo no aparece en la consola?

```mermaid
%%{init: {"htmlLabels": false, "flowchart": {"htmlLabels": false}}}%%
flowchart TD
    inicio["Un alumno no aparece en la consola"]
    barra{"¿Qué dice la barra de arriba<br/>de Aula en el equipo del alumno?"}

    sinconf["Aula no está configurada en ese equipo"]
    config["Pide al responsable de TI que instale<br/>Aula en ese equipo con su canal<br/>y la clave pública del centro"]

    nocons["El alumno no encuentra esta consola"]
    canal{"¿Tu clase está abierta en el<br/>mismo canal que dice su barra?"}
    cambiar["Cambia tu canal al del alumno:<br/>el del alumno es fijo"]
    perfil{"¿La red de Windows de los dos<br/>equipos es Privada o de Dominio?"}
    privada["Marca la red como Privada en<br/>Configuración › Red e Internet"]
    mdns["La red bloquea la búsqueda automática:<br/>pide a TI que configure en el alumno<br/>la dirección de esta consola"]

    inalc["El alumno encuentra la consola<br/>pero no llega a ella"]
    firewall["Revisa que el firewall de este equipo<br/>permita Aula Consola y que los dos<br/>equipos estén en la misma red"]

    cert["Otro equipo se hace pasar por una consola"]
    impostor["El alumno lo ignora, y está bien.<br/>Si pasa seguido, avisa a TI"]

    clave["El alumno tiene otra clave del centro"]
    reclave["Pide a TI que reinstale el alumno<br/>con la clave pública de este centro<br/>o la clave del centro en tu equipo"]

    diag["Pide a TI que ejecute<br/>aula-cliente diagnostico<br/>en el equipo del alumno"]
    ok["Listo: el alumno aparece en la consola"]

    inicio --> barra
    barra -- "«Sin configurar»" --> sinconf --> config
    barra -- "«Buscando…» o «No se encontró ninguna consola»" --> nocons --> canal
    canal -- "No" --> cambiar
    canal -- "Sí" --> perfil
    perfil -- "No, es Pública" --> privada
    perfil -- "Sí" --> mdns
    barra -- "«…no se pudo conectar»" --> inalc --> firewall
    barra -- "«…presentó un certificado distinto»" --> cert --> impostor
    barra -- "«…su clave no es la de este centro»" --> clave --> reclave
    barra -- "No veo la ventana de Aula" --> diag
    config & cambiar & privada & mdns & firewall & reclave --> ok

    classDef externo fill:#dbeafe,stroke:#1d4ed8,color:#111827
    classDef aula fill:#dcfce7,stroke:#15803d,color:#111827
    classDef datos fill:#ffedd5,stroke:#c2410c,color:#111827
    classDef seguridad fill:#fee2e2,stroke:#b91c1c,color:#111827
    class inicio,barra,canal,perfil,diag,ok aula
    class nocons,inalc,privada,mdns,firewall externo
    class sinconf,config,cambiar datos
    class cert,impostor,clave,reclave seguridad
```

## Lo que no es obvio

- **Las preguntas salen de mensajes reales**: cada rama corresponde a un texto que el agente
  muestra bajo su barra (`Agent.OnDiagnostic`). Si un mensaje cambia en el código, cambia este
  diagrama; **la consola genera de aquí el asistente «¿Falta un alumno?»** (`internal/manual.ParseTree`):
  los nodos `{…}` son preguntas y las etiquetas de sus flechas, las respuestas. Por eso las
  etiquetas están escritas para el profesor.
- **El canal del alumno no se toca en clase**: es fijo por computadora. Si no coincide, se cambia
  el de la consola; reubicar un equipo de sala es reinstalar con `/CANAL=N`.
- **La red Pública es la causa más común** (prueba real del 29/10/2026, D-012): las reglas de
  firewall de los instaladores cubren Privada y Dominio, salvo que se marque «Permitir también
  en redes Públicas».
- **Certificado distinto o clave ajena no se «arreglan» en el alumno**: el agente hace bien en
  ignorar esa consola. Lo primero es comprobar si es la consola correcta.
- **Un equipo de profesor con Hyper-V** ya no es causa (D-015: anuncio por cada interfaz), pero
  con una consola anterior a la 0.3.0-rc.2 sí lo era.
- `aula-cliente diagnostico` hace lo mismo que el agente durante unos 20 s y lo imprime, para
  distinguir un problema de red de uno de clave o canal. Los logs están en
  `LOCALAPPDATA\Aula\alumno\aula-cliente.log`.
- **Antes de preguntar, la consola puede comprobarse sola**: «Comprobar la conexión», arriba de
  este asistente (`internal/connectcheck`), recorre desde el equipo del profesor el anuncio, el
  puerto, la conexión segura con la clave del centro y la red y el firewall. Usa las mismas causas
  que el agente (`session.Cause`; una prueba lo compara con un agente real), pero no ve el equipo
  del alumno: su canal, su clave y su firewall siguen siendo estas preguntas.

## Fuentes en el código

- `internal/session/agent.go` (`diagnose` y sus mensajes en `Run` y `connect`),
  `internal/session/cause.go` (`Cause`)
- `internal/connectcheck` (el comprobador de la consola), `apps/teacher/connectcheck.go`
- `apps/student/diagnose.go` (`runDiagnose`)
- `apps/student/app.go` (`onDiagnostic`, `setStatus`), `apps/student/config.go`
- `internal/discovery/multicast.go` (D-015)
- `docs/uso-hito1.md`, `docs/prueba-vms.md` §10, `docs/decisiones.md` D-012 y D-015
- `internal/manual/tree.go` (`ParseTree`: este diagrama → asistente de la consola),
  `apps/teacher/frontend/dist/main.js` (`pintarArbol`)
