---
titulo: "Diagnóstico: «Mi alumno no conecta»"
descripcion: "Preguntas en orden para averiguar por qué un equipo de alumno no aparece en la consola"
audiencia: [profesor, instalador]
pantallas: [console, setup]
secciones_manual: [uso-hito1, prueba-vms, configuracion]
enlaces: [config=configuracion, firewall=uso-hito1#7-red-y-firewall, clave=protocol#101-identidades-y-claves]
---

# Diagnóstico: «Mi alumno no conecta»

¿Qué reviso, y en qué orden, cuando un equipo no aparece en la consola?

```mermaid
%%{init: {"htmlLabels": false, "flowchart": {"htmlLabels": false}}}%%
flowchart TD
    inicio["El alumno no aparece en la consola"]
    barra{"¿Qué dice la barra<br/>de Aula — Cliente?"}

    sinconf["«Aula — sin configurar»"]
    config["Reinstalar el agente con su canal<br/>y aula-centro.pub, o revisar<br/>ProgramData Aula alumno config.json"]

    nocons["«No se encontró ninguna<br/>consola del canal X»"]
    canal{"¿La consola está abierta<br/>en ese mismo canal?"}
    cambiar["Cambiar el canal de la consola<br/>(el del alumno es fijo)"]
    perfil{"¿La red de ambos equipos<br/>es Privada o de Dominio?"}
    privada["Marcar la red como Privada<br/>o reinstalar con «redes Públicas»"]
    mdns["mDNS bloqueado en la red:<br/>configurar direct host:puerto"]

    inalc["«Se encontró la consola …<br/>pero no se pudo conectar»"]
    firewall["Firewall del equipo del profesor:<br/>TCP 47830 entrante, misma red"]

    cert["«… presentó un certificado<br/>distinto del que anuncia»"]
    impostor["Otro equipo anuncia con datos<br/>ajenos: se ignora; avisar al<br/>administrador si persiste"]

    clave["«… su clave no es la de este centro»"]
    reclave["Los alumnos tienen otra clave del centro:<br/>reinstalar con el aula-centro.pub<br/>de esta consola"]

    diag["Ejecutar aula-cliente diagnostico:<br/>configuración, consolas vistas por mDNS<br/>e intento de conexión"]
    ok["Conectado: aparece en la consola"]

    inicio --> barra
    barra -- "sin configurar" --> sinconf --> config
    barra -- "buscando" --> nocons --> canal
    canal -- "no" --> cambiar
    canal -- "sí" --> perfil
    perfil -- "no, Pública" --> privada
    perfil -- "sí" --> mdns
    barra -- "inalcanzable" --> inalc --> firewall
    barra -- "certificado" --> cert --> impostor
    barra -- "clave" --> clave --> reclave
    barra -- "no sé / sin ventana" --> diag
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
  diagrama; la etapa 18 genera de aquí el árbol interactivo de la consola.
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
- Aún no existe el comprobador de conexión de la consola (etapa 18); hoy el diagnóstico es del
  lado del alumno.

## Fuentes en el código

- `internal/session/agent.go` (`diagnose` y sus mensajes en `Run` y `session`)
- `apps/student/diagnose.go` (`runDiagnose`)
- `apps/student/app.go` (`onDiagnostic`, `setStatus`), `apps/student/config.go`
- `internal/discovery/multicast.go` (D-015)
- `docs/uso-hito1.md`, `docs/prueba-vms.md` §10, `docs/decisiones.md` D-012 y D-015
