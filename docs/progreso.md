# Progreso de Aula

| Etapa | Descripción                                    | Estado       |
|-------|------------------------------------------------|--------------|
| 01    | Estructura del repositorio y scaffold          | ✅ Completada |
| 02    | Protocolo: tipos de mensajes básicos           | ✅ Completada |
| 03    | TLS y autenticación Ed25519                    | ✅ Completada |
| 04    | Descubrimiento UDP y conexión WebSocket        | ✅ Completada |
| 05    | Sesión: registro de agentes y heartbeat        | ✅ Completada |
| 06    | Platform: captura de pantalla                  | ⬜ Pendiente  |
| 07    | Consola: vista de equipos y monitoreo          | ⬜ Pendiente  |
| 08    | Compartir y recoger código                     | ⬜ Pendiente  |
| 09    | Envío de ejercicios                            | ⬜ Pendiente  |
| 10    | Control remoto (bloquear pantalla/teclado)     | ⬜ Pendiente  |
| 11    | Empaquetado e instaladores                     | ⬜ Pendiente  |

---

## Notas de la etapa 03 — TLS y autenticación Ed25519

### Archivos nuevos
- `crates/core/src/tls.rs` — Generación de certificado autofirmado (ECDSA P-256),
  persistencia en PEM, huella SHA-256, configuración de `rustls::ServerConfig` y
  `rustls::ClientConfig` con fijado de certificado (certificate pinning).
- `crates/core/src/auth.rs` — Generación de pareja de claves Ed25519, persistencia,
  generación de nonce, firma y verificación.
- `crates/core/examples/keygen.rs` — Utilidad `console keygen` que genera las claves
  Ed25519 y el certificado TLS.

### Archivos modificados
- `Cargo.toml` (workspace) — Agregadas dependencias: `tokio-rustls`, `rcgen`,
  `rustls-pemfile`, `base64`, `rand`, `sha2`. Activada feature `ring` en `rustls`.
- `crates/core/Cargo.toml` — Nuevas dependencias del workspace.
- `crates/core/src/lib.rs` — Exporta módulos `tls` y `auth`.
- `crates/core/src/error.rs` — Variantes `Tls`, `Auth`, `AuthFailed`.
- `crates/core/src/server.rs` — Acepta TLS antes de WebSocket. Ejecuta flujo
  auth.challenge → auth.proof → auth.ok. `ServerConfig` requiere `TlsIdentity` y
  `SigningKey`.
- `crates/core/src/client.rs` — Conecta con TLS y fijado de huella. Verifica
  auth.challenge + auth.proof con clave pública Ed25519. `ClientConfig` requiere
  `VerifyingKey`. `run()` recibe `expected_fingerprint`.
- `crates/core/examples/dos_agentes.rs` — Actualizado para TLS + auth.
- `crates/core/tests/integration.rs` — 6 pruebas: conexión TLS+auth exitosa,
  clave pública incorrecta rechazada, huella TLS incorrecta rechazada,
  filtrado por class_id, descubrimiento completo, clave privada no aparece en logs.
- `docs/protocol.md` — Corregido diagrama de flujo auth. Nueva sección 8 (Seguridad):
  modelo de confianza, TLS, autenticación Ed25519, gestión de claves.

### Pruebas (55 en total)
- `cargo test --workspace` — 55 pruebas pasan.
- `cargo clippy --workspace -- -D warnings` — Sin advertencias.
