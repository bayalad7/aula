---
titulo: "Modelo de datos local"
descripcion: "Qué guarda Aula en el disco de cada equipo, qué solo vive en memoria y cómo se relacionan claves y configuración"
audiencia: [desarrollador, auditor]
pantallas: [setup]
secciones_manual: [configuracion, privacidad]
---

# Modelo de datos local

¿Qué escribe Aula en disco en el equipo del profesor y en el del alumno, y qué se pierde al
cerrar?

```mermaid
%%{init: {"htmlLabels": false}}%%
erDiagram
    CLAVE_CENTRO ||--|| CENTRO_PUB : "se exporta la parte pública"
    CENTRO_PUB ||--o{ CONFIG_ALUMNO : "el instalador copia console_public_key"
    CLAVE_CENTRO |o--o| IDENTIDAD_PERSONAL : "sin clave del centro se usa la personal"
    CONFIG_CONSOLA_MAQUINA ||--o| AJUSTES_CONSOLA : "canal predeterminado, si no el último"
    CERTIFICADO_TLS ||--o{ CONFIG_ALUMNO : "huella anunciada por mDNS, no se guarda"
    CONFIG_ALUMNO ||--o| POLITICA_ACTIVA : "solo con filtro web aplicado"
    HISTORIAL_MEMORIA ||--o{ ENVIOS_MEMORIA : "cada emisión crea un envío"
    ENVIOS_MEMORIA }o--o{ TABLERO_MEMORIA : "code.ack confirma lo mostrado"

    CLAVE_CENTRO {
        pem ed25519_pkcs8 "ProgramData Aula centro clave-centro.pem"
        acl admins_system_total "Usuarios del equipo, solo lectura"
    }
    CENTRO_PUB {
        string public_key "aula-centro.pub: aula-ed25519 Base64"
    }
    IDENTIDAD_PERSONAL {
        pem ed25519_pkcs8 "AppData Aula consola identidad-ed25519.pem"
        acl solo_el_usuario "RestrictToOwner"
    }
    CERTIFICADO_TLS {
        pem tls_cert "AppData Aula consola tls-cert.pem, público"
        pem tls_key "tls-key.pem ECDSA P-256, solo el usuario"
    }
    AJUSTES_CONSOLA {
        list recent_channels "hasta 8"
        string console_name
        int port "0 = 47830"
    }
    CONFIG_CONSOLA_MAQUINA {
        int default_channel "ProgramData Aula consola config.json"
        int port
        int broadcast_max_kbps
    }
    CONFIG_ALUMNO {
        string channel "ProgramData Aula alumno config.json"
        string console_public_key
        string direct "opcional host:puerto"
    }
    POLITICA_ACTIVA {
        bool nrpt "ProgramData Aula alumno politica-activa.json"
        list doh "políticas de navegador como estaban"
        datetime since
    }
    HISTORIAL_MEMORIA {
        string snippetId "solo memoria de la consola, máx. 100"
        int number
        string title
        string lang
        string content
        int times
    }
    ENVIOS_MEMORIA {
        string id "solo memoria de la consola, 20 recientes"
        string kind "code, clear o notice"
        int sent
        int acked
        int failed
        int noCapability
    }
    TABLERO_MEMORIA {
        string id "solo memoria del agente, máx. 20"
        string exercise
        string title
        string content
        datetime receivedAt
    }
```

## Lo que no es obvio

- **Lo único secreto en disco es la clave privada** (la del centro o la personal) y la del
  certificado TLS. Se escriben con un archivo temporal ya protegido que luego se renombra, para
  que en ningún instante otro usuario pueda leerlas. Nunca se registran ni viajan por la red, y
  nunca se instalan en equipos de alumno.
- **Riesgo aceptado**: los usuarios del equipo del profesor pueden *leer* la clave del centro,
  para que cualquier maestro dé clase ahí (`docs/protocol.md` §10.1).
- **El historial de código, los acuses y el tablero del alumno no se guardan**: viven en memoria
  y se pierden al cerrar la app. Tampoco se escriben capturas de pantalla, video ni cuadros
  (`docs/privacidad.md` reglas 7 y 16).
- **El certificado y la clave son independientes**: regenerar el certificado no obliga a
  reconfigurar alumnos (la huella viaja en el anuncio); cambiar la clave del centro sí.
- **Precedencia de la configuración del alumno**: argumentos `-canal`/`-clave`/`-directo` >
  `ProgramData\Aula\alumno\config.json` (instalador, protegido) >
  `AppData\Aula\alumno\config.json` (usuario). El agente la relee cada 3 s.
- **`politica-activa.json` existe solo mientras hay filtro web** y se escribe *antes* de tocar el
  equipo, para poder deshacerlo tras un corte de luz (diagrama 15). El bloqueo de apps no deja
  nada en disco.
- **Logs** (no dibujados): `LOCALAPPDATA\Aula\<app>\aula-*.log` con rotación de 1 MiB, y
  `ProgramData\Aula\alumno\logs` con el agente como servicio. Nunca contienen código, textos,
  imágenes, nonces, firmas ni claves.
- `erDiagram` no admite las clases de color en todas las versiones de Mermaid, por eso este
  diagrama va sin colores.

## Fuentes en el código

- `internal/security/sitekey.go`, `internal/security/identity.go`, `internal/security/files.go`,
  `internal/security/tls.go`
- `internal/platform/paths.go`, `internal/platform/machinesecret.go`,
  `internal/platform/fileperm.go`, `internal/platform/logfile.go`
- `apps/teacher/settings.go`, `apps/teacher/machine.go`, `apps/teacher/cli.go`
- `apps/student/config.go`, `internal/session/agentconfig.go`
- `internal/policy/journal.go`, `internal/platform/policy.go` (`SavedSetting`)
- `internal/codedist/history.go`, `internal/codedist/deliveries.go`, `internal/codedist/board.go`
- `docs/configuracion.md`
