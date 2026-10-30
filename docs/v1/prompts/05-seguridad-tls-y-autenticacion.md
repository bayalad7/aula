# 05 · Módulo A — Seguridad: TLS y autenticación

**Prerrequisitos:** etapa 04 completa.
**Duración estimada:** 1–2 sesiones.
**Por qué importa:** sin esto, cualquiera en la LAN podría hacerse pasar por el profesor o leer el tráfico. La meta: que un agente solo obedezca a **tu** consola.

## Prompt

~~~~markdown
Lee CLAUDE.md, docs/protocol.md y docs/progreso.md. Agrega cifrado y autenticación a la
conexión del Módulo A.

## Modelo de confianza
- Cada instalación de profesor tiene una pareja de claves Ed25519 que identifica a la consola.
- La clave PÚBLICA del profesor va en la configuración del agente al instalarlo.
- El agente solo acepta una consola cuyo reto firme correctamente con esa clave.
- Documenta este modelo en una sección nueva de docs/protocol.md antes de programar.

## Tareas
1. **TLS** en el WebSocket (`crypto/tls`).
   - En v0, la consola genera un certificado autofirmado al primer arranque y lo persiste.
   - El TXT record mDNS lleva la huella del certificado; el agente la fija (certificate
     pinning): verifica que el certificado presentado coincide con la huella anunciada.
2. **Autenticación** del ciclo de sesión:
   - Implementa `auth.challenge` / `auth.proof` / `auth.ok` del protocolo.
   - El agente genera un nonce; la consola lo firma con su clave privada; el agente verifica
     la firma contra la pública configurada. Si no coincide, cierra la conexión y lo registra.
   - Ningún mensaje que no sea del handshake se procesa antes de `auth.ok`.
3. **Gestión de claves** (`internal/security` + comando)
   - Subcomando `teacher keygen` que crea la pareja y muestra la clave pública en un formato
     fácil de copiar al configurar agentes.
   - La clave privada se guarda solo en el equipo del profesor con permisos restringidos; nunca
     en logs ni dentro del binario del agente.

## Pruebas
- Agente con la clave pública correcta se autentica; con otra clave, es rechazado.
- Consola con certificado cuya huella no coincide: rechazada por el agente.
- Prueba de que la clave privada nunca aparece en la salida de logs.
- Prueba de que un mensaje `code.share` recibido antes de `auth.ok` se rechaza.

## Criterios de aceptación
- El tráfico va por TLS; descríbeme cómo lo comprobaste.
- `staticcheck ./...` y `go test ./...` pasan.

Actualiza docs/progreso.md y la sección de seguridad de docs/protocol.md.
~~~~

## Verificación manual
- [ ] Configura un agente con una clave pública equivocada: debe rechazar la conexión.
- [ ] La clave privada solo existe en tu laptop, con permisos restringidos.
