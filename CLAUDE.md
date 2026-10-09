# Aula: sistema de gestión de aula multiplataforma

## Qué es
Software para que un maestro de programación monitoree y controle los equipos de su
laboratorio (como LanSchool) y además comparta código que los alumnos pueden copiar como
texto, recoja su código y les envíe ejercicios.

Dos programas:
- **Consola del maestro** (`apps/console`): app de escritorio Tauri (Rust + HTML/CSS/JS).
- **Agente del alumno** (`crates/agent`): binario Rust que corre en cada equipo del alumno.

## Arquitectura (no cambiar sin actualizar este archivo)
- Monorepo con workspace de Cargo.
  - `crates/protocol`: tipos de mensajes y serialización. Sin I/O, sin dependencias de SO.
  - `crates/core`: red, descubrimiento, TLS, autenticación, sesión. Sin código de SO.
  - `crates/platform`: traits de funciones del SO + implementaciones `windows`, `macos`, `linux`
    detrás de `#[cfg(target_os = ...)]`.
  - `crates/agent`: une `core` + `platform` en el programa del alumno.
  - `apps/console`: consola Tauri; usa `core` y `protocol`.
- **Regla de oro:** `protocol` y `core` NUNCA llaman APIs del sistema operativo. Todo lo que
  toca pantalla, teclado, procesos o servicios pasa por un trait de `platform`.
- La consola anuncia su presencia por UDP broadcast. **Los agentes se conectan a la consola**
  (nunca al revés), por WebSocket sobre TLS.
- Los agentes solo aceptan órdenes de una consola autenticada con la clave Ed25519 cuya
  clave pública viene en su configuración.

## Stack
- Rust estable (edición 2021), `tokio`, `serde`, `serde_json`, `tokio-tungstenite`, `rustls`,
  `ed25519-dalek`, `tracing`, `thiserror` (librerías) y `anyhow` (binarios).
- Consola: Tauri 2, frontend en HTML/CSS/JS sin framework (o con uno ligero solo si se justifica).
- Resaltado de código en el frontend: highlight.js.

## Convenciones
- Código, nombres y comentarios técnicos en inglés; textos de interfaz y documentación en español.
- Errores con `thiserror` en librerías; nada de `unwrap()` fuera de pruebas.
- Logs con `tracing`; nunca registrar contenido de pantallas, código de alumnos ni claves.
- Cada mensaje nuevo del protocolo se documenta primero en `docs/protocol.md`.
- Cambios pequeños e incrementales; no refactorizar módulos ajenos a la tarea.
- Antes de terminar una tarea: `cargo fmt`, `cargo clippy -- -D warnings`, `cargo test`.
- Ramas: `main` estable, `develop` integración, `feat/<nombre>` por tarea.

## Plataformas
Prioridad: Windows 10/11 → Linux (X11) → macOS. Toda función nueva define primero su trait en
`platform` y una implementación `unsupported` que devuelve un error claro.

## Privacidad
Los alumnos deben ser informados del monitoreo. El agente muestra un indicador visible
cuando su pantalla está siendo vista. No se guardan capturas en disco salvo petición explícita del maestro.

## Progreso
El estado actual está en `docs/progreso.md`. Actualízalo al final de cada sesión.