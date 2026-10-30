# 15 · Despliegue en flota, auto-actualización y diagnóstico

**Prerrequisitos:** Hito 1 cerrado (08). Mejor aún tras portar a los SO del laboratorio (13).
**Duración estimada:** 2–4 sesiones.
**Por qué su propio prompt:** es lo que más pesa al operar 40 equipos y es transversal (no es "una función"). Bien resuelto, te ahorra horas cada semestre.

## Prompt

~~~~markdown
Lee CLAUDE.md, docs/protocol.md y docs/progreso.md. Vamos a facilitar instalar, actualizar y
diagnosticar la flota de agentes del laboratorio. Son tres bloques; puedes entregarlos en sesiones
separadas, pero diséñalos juntos primero y DETENTE para que lo apruebe.

## Diseño (detente)
- Esquema de versión: el `hello` ya reporta versión del agente; define cómo la consola muestra
  qué versiones hay en la flota y marca las desactualizadas.
- Canal de actualización: consume los releases que ya produce `.github/workflows/release.yml`
  (etapas 08 y 13): artefactos + `checksums.txt` + `checksums.txt.sig` firmados con la llave
  Ed25519 **dedicada de releases** (distinta de la identidad profesor↔agente), cuya pública se
  versiona en `empaquetado/llave-releases.pub` y va embebida en el binario del agente.
Espera mi aprobación.

## Bloque 1: despliegue masivo
- Documenta instalación silenciosa del agente en Windows (parámetros del instalador para GPO) y en
  Linux (`.deb`/`.rpm` por el gestor de paquetes o por script); provisión de la configuración
  (canal + clave pública) sin editar equipo por equipo.
- Entrega una guía en docs/despliegue.md para el área de TI.

## Bloque 2: auto-actualización
- El agente consulta si hay una versión nueva (desde la consola o una ruta de red/archivo),
  descarga el artefacto y su `checksums.txt`/`checksums.txt.sig`, **verifica la firma** contra la
  llave pública de releases embebida y comprueba el sha256 del binario, y solo entonces se
  reinstala/reinicia de forma segura. Si algo no verifica, no aplica nada y lo registra.
- Nunca aplicar un binario sin firma válida. Documenta el flujo y el rollback.

## Bloque 3: diagnóstico
- Panel en la consola de "salud" por alumno: versión, latencia del heartbeat, capacidades,
  último error.
- Un modo de diagnóstico en el agente que verifique lo típico que falla: ¿ve anuncios mDNS?,
  ¿resuelve y alcanza el puerto WebSocket?, ¿la huella del certificado coincide?, ¿la clave
  pública configurada es la correcta?, ¿el firewall local bloquea? Que muestre un reporte legible
  para que tú o TI resuelvan "por qué el equipo X no conecta".

## Reglas
- Toda acción remota (actualizar, reiniciar servicio) solo desde la consola autenticada.
- Nada de rutas que apliquen binarios o comandos sin firma verificada.

## Pruebas
- Prueba de que un release con firma inválida es rechazado por el agente.
- Prueba del diagnóstico detectando un caso provocado (puerto bloqueado, canal equivocado).

## Criterios de aceptación
- Instalación silenciosa documentada y probada en al menos un equipo.
- Una actualización firmada se aplica sola; una no firmada se rechaza.
- El panel de salud muestra versiones y señala quién está desactualizado o con problemas.
- `staticcheck ./...` y `go test ./...` pasan.

Actualiza docs/progreso.md, docs/despliegue.md y docs/protocol.md si agregas mensajes.
~~~~

## Verificación manual
- [ ] Simula un release nuevo: ¿los agentes se actualizan solos y rechazan uno sin firma?
- [ ] Desconfigura el canal de un agente: ¿el diagnóstico lo detecta y lo explica?
