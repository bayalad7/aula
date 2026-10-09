# Aula

Sistema de gestión de aula para maestros de programación. Permite monitorear y controlar
los equipos de un laboratorio, compartir código con los alumnos, recoger su código y
enviarles ejercicios.

## Componentes

- **Consola del maestro** (`apps/console`): aplicación de escritorio con Tauri 2.
- **Agente del alumno** (`crates/agent`): binario que corre en cada equipo del alumno.

## Requisitos

- [Rust](https://rustup.rs/) (edición 2021, estable)
- [Node.js](https://nodejs.org/) ≥ 18
- En Windows: Visual Studio Build Tools con C++ y WebView2

## Compilar

### Workspace completo (librerías + agente)

```bash
cargo build --workspace
```

### Ejecutar pruebas

```bash
cargo test --workspace
```

### Consola del maestro (Tauri)

```bash
cd apps/console
npm install
npm run tauri:dev
```

### Agente del alumno

```bash
cargo run -p aula-agent
```

## Verificación de estilo

```bash
cargo fmt --check
cargo clippy -- -D warnings
```

## Estructura

```
Aula/
├── apps/
│   └── console/          # Consola Tauri 2
│       ├── src/          # Frontend HTML/CSS/JS
│       └── src-tauri/    # Backend Rust
├── crates/
│   ├── protocol/         # Tipos de mensajes (sin I/O)
│   ├── core/             # Red, TLS, sesión (sin código de SO)
│   ├── platform/         # Traits de SO + implementaciones por plataforma
│   └── agent/            # Binario del alumno
└── docs/
    └── progreso.md       # Estado del proyecto
```
