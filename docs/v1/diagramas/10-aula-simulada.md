---
titulo: "Aula simulada y simulación de fallas"
descripcion: "Cómo prueba el profesor Aula sin laboratorio con alumnos de prueba que nunca se confunden con equipos reales"
audiencia: [profesor, desarrollador]
pantallas: [simulada]
secciones_manual: [v1/administracion/como-funciona, v1/administracion/probar-sin-laboratorio, privacidad, eventos-ui]
enlaces: [reglas=privacidad#aula-simulada-consola-del-profesor]
---

# Aula simulada y simulación de fallas

¿De dónde salen los alumnos simulados, por dónde se conectan y qué los separa de los reales?

```mermaid
%%{init: {"htmlLabels": false, "flowchart": {"htmlLabels": false}}}%%
flowchart LR
    subgraph equipo["Equipo del profesor: un solo proceso, aula-consola.exe"]
        app["App de la consola<br/>SimulationStart, Add,<br/>Fault, Hand, Message, Stop"]
        subgraph sims["internal/simclass"]
            s1["Simulado 1<br/>session.Agent real"]
            s2["Simulado 2<br/>falla: lento, no acusa<br/>o desconectado"]
        end
        simsrv["Consola de simulados<br/>127.0.0.1:puerto libre<br/>sin anuncio mDNS"]
        simreg[("Registro de simulados")]
        realsrv["Consola real<br/>TCP 47830, anunciada"]
        realreg[("Registro real")]
        ui["Interfaz: insignia «Simulado»,<br/>conteo y acuses aparte"]
    end
    reales["Equipos de alumno reales"]
    nunca["Energía, restricciones<br/>y transmisión: nunca"]

    app --> sims
    s1 & s2 -- "TLS, firma Ed25519, hello,<br/>code.ack, msg.ack, miniaturas,<br/>hand.raise, msg.student" --> simsrv
    simsrv --> simreg
    reales -- "wss://" --> realsrv
    realsrv --> realreg
    realreg -- "simulated: false" --> ui
    simreg -- "simulated: true" --> ui
    nunca -. "solo al registro real" .-> realreg
    reglas["Reglas 32 a 35<br/>docs/privacidad.md"]
    reglas -.- ui

    classDef externo fill:#dbeafe,stroke:#1d4ed8,color:#111827
    classDef aula fill:#dcfce7,stroke:#15803d,color:#111827
    classDef datos fill:#ffedd5,stroke:#c2410c,color:#111827
    classDef seguridad fill:#fee2e2,stroke:#b91c1c,color:#111827
    class app,s1,s2,simsrv,realsrv,ui aula
    class simreg,realreg datos
    class reales externo
    class nunca,reglas seguridad
```

## Lo que no es obvio

- **Son agentes de verdad.** Cada simulado es un `session.Agent` como el de un equipo de alumno:
  verifica el certificado y la firma con la clave de la consola, se presenta con `hello`, late
  cada 5 s y responde con `codedist.Receiver` y `screen.Streamer`. Si algo funciona con ellos,
  funciona por el mismo camino con un equipo real.
- **Se distinguen por dónde llegaron, no por cómo se llaman.** Llegan a un segundo servidor de la
  consola, en `127.0.0.1` y en un puerto libre, con su propio registro. Un equipo real llamado
  «Simulado 1» llega al registro real y sigue siendo real.
- **No se anuncian.** El puerto de los simulados no viaja por mDNS y solo escucha en el propio
  equipo: ningún alumno del laboratorio puede llegar a él. No hay mensajes nuevos en el
  protocolo.
- **Solo reciben lo que no tiene efecto en un equipo:** código, avisos, miniaturas y capturas.
  Solo pueden declarar `code.view`, `msg.show`, `screen.thumb`, `screen.snapshot`, `hand` y
  `msg.student`, y la consola calcula energía, restricciones y transmisión solo sobre el registro
  real.
- **Levantan la mano y escriben cuando el profesor se lo pide** («Levantar la mano» y «Escribir»
  en su fila): usan el mismo `participation.Desk` que un alumno real, y lo que mandan llega a
  Participación marcado como simulado y se borra al cerrar el aula simulada (diagrama 18).
- **Las fallas usan estados que la consola ya conoce:**
  - «lento» contesta tarde (de 0,5 a 10 s, menos que los 15 s de silencio que tumban una sesión);
  - «no acusa» muestra el código pero no manda `code.ack`, y el envío queda incompleto;
  - «desconectado» cierra la sesión hasta volver a «ninguna».
- **Nada persiste.** El aula simulada se cierra con «Cerrar el aula simulada», al cambiar de canal
  o de nombre y al cerrar la clase. La pantalla de un simulado se dibuja en memoria, con la
  franja «Tu pantalla está siendo vista por el profesor» mientras se ve.
- En la consola, la vista **Aula simulada** crea y quita alumnos (SO y capacidades), dispara las
  fallas y muestra qué tiene cada uno en pantalla. En **Alumnos** aparecen al final, en su propio
  grupo. El comprobador de conexión del prompt 18 (Fase 6.3) sigue pendiente.

## Fuentes en el código

- `internal/simclass/class.go`, `internal/simclass/screen.go`
- `apps/teacher/class.go` (`simulationHost`, `StartSimulation`, `simulatedMessages`)
- `apps/teacher/simulation.go`, `apps/teacher/restrictions.go` (`studentsIn`, `simSessionsOf`)
- `apps/teacher/frontend/dist/main.js` (vista Aula simulada)
- `internal/codedist/deliveries.go` (`AddSimulated`)
- `apps/teacher/simulation_test.go`, `internal/simclass/class_test.go`
- `docs/privacidad.md` reglas 32–35
