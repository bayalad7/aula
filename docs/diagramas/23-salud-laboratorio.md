---
titulo: "Salud del laboratorio"
descripcion: "Qué informa cada equipo y cuándo, cómo la consola lo convierte en alertas, qué hace «Revisar ahora» y cómo corre el autodiagnóstico"
audiencia: [profesor, instalador, desarrollador]
pantallas: [laboratorio]
secciones_manual: [administracion/laboratorio, administracion/solucion-de-problemas, privacidad, protocol]
---

# Salud del laboratorio

¿Cómo sabe la consola, antes de clase, qué equipos van a dar problemas, y qué hace cada botón de
**Laboratorio**?

```mermaid
%%{init: {"htmlLabels": false}}%%
sequenceDiagram
    autonumber
    actor Prof as Profesor
    box rgb(220,252,231) Consola
        participant UI as Laboratorio y ficha
        participant L as health.Lab
    end
    box rgb(220,252,231) Equipo del alumno
        participant A as Agente o servicio
        participant SO as platform.SystemHealth
    end

    A->>L: hello con health y health.selftest
    L->>A: health.config (dirección de internet y servicios de Ajustes › Salud)
    A->>SO: lee disco, memoria, procesador, encendido, red, batería (solo lectura)
    A-->>L: health.report first (lista cerrada)
    loop cada 60 s ±10 %, o antes si algo cambia mucho (≥ 10 s entre uno y otro)
        A->>SO: lee otra vez
        A-->>L: health.report periodic o change
    end
    loop cada heartbeat (5 s)
        A-->>L: heartbeat con su hora (ts)
        Note over L: reloj = mediana de ts − llegada
    end
    L->>L: umbrales de Ajustes › Salud → alertas
    L-->>UI: aula:lab (tabla, contador del menú, insignia en Alumnos)

    Prof->>UI: Revisar ahora (una vez cada 10 s)
    UI->>A: health.request
    A-->>L: health.report request (ref_id)

    rect rgb(255,237,213)
        Prof->>UI: Autodiagnóstico (todos o la selección)
        UI->>A: health.selftest (test_id)
        A->>SO: escribe, verifica y borra 4 MiB temporales, revisa audio, red, video, teclado y mouse
        A->>A: DNS y TCP a internet y a cada servicio
        A-->>L: health.selftest.result done, busy o unsupported
        Note over L: sin respuesta en 30 s, «No respondió»
    end
```

## Lo que no es obvio

- **Solo informa** (privacidad, regla 93): ni el informe ni el autodiagnóstico cambian nada del
  equipo. El autodiagnóstico escribe solo en la carpeta temporal del agente y borra su archivo
  siempre; nunca toca `Documentos`.
- **Lista cerrada** (regla 92, §5.27.3): un dato nuevo cambia el protocolo y la regla. Un dato
  que el sistema no sabe leer **no viaja**, y la consola dice «No se puede leer en este equipo» sin
  alerta. Fuera de Windows el informe va casi vacío y no hay autodiagnóstico.
- **El reloj no viaja**: la consola lo calcula con los `heartbeat` que ya existían, con la
  mediana de los últimos 5 para que un paquete lento no lo mueva.
- **Como servicio, sin ayudante**: el servicio (SYSTEM) lee el equipo él mismo, también sin
  nadie con sesión iniciada; entonces no hay disco de `Documentos`. El estado S.M.A.R.T. necesita
  permisos de administrador: con Aula como ventana, suele no poder leerse.
- **Los umbrales viven en la consola**: el agente solo sabe cuándo un cambio es «grande» (bandas
  fijas) para mandar el informe antes de tiempo; qué es alerta lo decide Ajustes › Salud.
- **Solo en memoria** (regla 95): los informes se olvidan al cerrar la consola o cambiar de canal.
- Los alumnos simulados inventan sus valores, con algunas alertas, para probar la pantalla.

## Fuentes en el código

- `internal/protocol/health.go`, `internal/platform/health*.go`
- `internal/health/agent.go` (`Monitor`, `SelfTest`, `Notable`), `internal/health/console.go`
  (`Lab`, `Evaluate`, `Thresholds`, `Targets`), `internal/health/simulated.go`
- `internal/session/registry.go` (`RecordClock`)
- `apps/student/health.go`, `apps/teacher/health.go`, `internal/simclass/health.go`
- `docs/protocol.md` §5.27, `docs/privacidad.md` reglas 92–95
