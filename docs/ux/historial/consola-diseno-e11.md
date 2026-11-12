# Diseño E11: comprobador de conexión (Fase 6.3)

Estado: **aprobado el 04/11/2026.** Decisiones: en Windows, PowerShell con salida JSON (no depende
del idioma); la comprobación corre solo con el botón; `session.Cause` exportado.

## Qué hace

En la consola, el botón **«Comprobar la conexión»** en «¿Falta un alumno?» recorre el camino
que sigue un alumno para llegar a esta consola. Dice en qué paso fallaría, con un mensaje «qué pasó
+ qué hacer» y su sección del manual. No envía nada a los alumnos ni cambia el protocolo.

| # | Paso | Cómo lo comprueba | Qué no puede saber |
|---|---|---|---|
| 1 | Clase abierta | Hay clase, con su canal y su puerto | — |
| 2 | Anuncio en la red | Un `discovery.Browser` del propio canal encuentra el anuncio de esta consola con su huella, en las direcciones de la LAN | Si el switch o la Wi‑Fi dejan pasar multicast hasta el alumno |
| 3 | Puerto abierto | TCP a cada dirección anunciada que no sea de loopback | Desde el mismo equipo el firewall de Windows no filtra; por eso existe el paso 5 |
| 4 | TLS y clave | Conecta como un agente: TLS fijado a la huella anunciada, reto `auth.challenge` y firma comprobada con la clave pública del centro. Cierra antes de `hello`, así que no aparece ningún alumno | Si el alumno tiene otra clave del centro |
| 5 | Red y firewall | Perfil de cada red (Pública, Privada o Dominio) y si una regla de entrada permite `aula-consola.exe` en ese perfil, mediante una interfaz nueva en `internal/platform` | En Linux y macOS: «no se puede comprobar aquí» |

Al final dice con honestidad lo que solo se ve desde el equipo del alumno: su canal, su clave, su
firewall y su red. Remite a las preguntas de «¿Falta un alumno?» y a `aula-cliente diagnostico`.

## Las mismas causas que el agente

Hoy `session.Agent` escribe sus diagnósticos como texto suelto. Propongo agregar a `session` un
tipo exportado `Cause` (`NoConsole`, `Unreachable`, `CertMismatch`, `WrongKey`, `AuthIncomplete`)
que el agente emite junto con el mismo texto de hoy. Es un cambio pequeño: la barra del alumno
no cambia. El comprobador clasifica sus fallos con esas mismas causas.

**Prueba (Fase 7.3):** se provoca cada falla (sin anuncio, otra huella, otra clave, puerto
cerrado). El comprobador debe reportar la misma `Cause` que un `session.Agent` real ante la misma
falla.

## Dónde va

- **`internal/connectcheck`** (paquete nuevo; se agrega a CLAUDE.md y a la regla de oro, sin
  llamadas al SO):
  - `Checker{Channel, PublicKey, Fingerprint, Port, Browse, Net platform.NetworkInfo}`;
  - `Run(ctx, progress func(Step)) Report`;
  - cada `Step` lleva `{ID, Status: ok|falla|aviso|noComprobable, Cause, Detail}`.
- **`internal/platform`:** `NetworkInfo` con `Profiles()` y `FirewallAllows(program)`. Fallback
  «no soportado» y una implementación para Windows con PowerShell (`Get-NetConnectionProfile`,
  `Get-NetFirewallApplicationFilter` y sus reglas, salida `ConvertTo-Json`).
- **Consola:**
  - binding `CheckConnection()`, con el evento `aula:connectcheck` para el avance;
  - error `aula:noClass` si no hay clase abierta;
  - textos `comprobar.*` en `textos.es-MX.js`.
  - Interfaz: botón y lista de pasos con su estado (componentes de `sistema/`) arriba del árbol
    de «¿Falta un alumno?». Cada falla enlaza su sección del manual y el diagrama 12.
- **Definición de terminado:**
  - `docs/eventos-ui.md` y `dobles.js`, con escenas «todo bien» y «red pública»;
  - dos capturas en `recorrido.mjs`;
  - sección «Comprobar la conexión» en `solucion-de-problemas`;
  - diagrama 12: al implementar se decidió no agregar el nodo. El árbol es el asistente que
    pregunta por la barra del alumno, y un nodo intermedio saldría como paso «hecho». Solo cambia
    «Lo que no es obvio», así que los SVG no cambian;
  - casos C-35 a C-37.
  - Alumnos simulados: el comprobador no les llega. Su puerto de `127.0.0.1` no se anuncia y el
    comprobador lo ignora (prueba).
  - Privacidad: no observa ni controla a nadie; no hace falta regla.
