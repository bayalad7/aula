# Progreso

Última actualización: 2026-10-23 (etapa 02).

| Etapa | Descripción | Estado |
|---|---|---|
| 02 | Entorno y repositorio: monorepo Go + dos apps Wails, Git, CI | ✅ Completada |
| 03 | Especificación del protocolo (`docs/protocol.md`, paquete `protocol`) | ⏳ Pendiente |
| 04 | Descubrimiento mDNS con canales + servidor/cliente WebSocket | ⏳ Pendiente |
| 05 | Seguridad: TLS y autenticación Ed25519 | ⏳ Pendiente |
| 06 | App estudiante: overlay con Monaco y botón Copiar | ⏳ Pendiente |
| 07 | App profesor: lista, hotkey de captura, emisión | ⏳ Pendiente |
| 08 | Hito 1: distribución de código de punta a punta + instaladores | ⏳ Pendiente |
| 09 | Monitoreo: miniaturas y pantalla en blanco (con aviso visible) | ⏳ Pendiente |
| 10 | Plantilla reutilizable para nuevas funciones | ⏳ Pendiente |
| 11 | Revisión de seguridad y privacidad | ⏳ Pendiente |

## Etapa 02: notas

- Módulo `github.com/bayalad7/aula` (`go 1.26.0`), Wails v2.17.0.
- Paquetes `internal/{protocol,discovery,transport,session,security,platform,codedist}`, cada uno
  con un archivo mínimo y una prueba trivial. `platform` ya define `ErrNotSupported`.
- `apps/teacher` ("Aula — Consola") y `apps/student` ("Aula — Cliente"): Wails v2 con frontend
  HTML/CSS/JS sin bundler. Verificadas con `wails dev` y `wails build` en Windows 10.
- Decisiones en `docs/decisiones.md` (D-001 a D-003).
- CI en `.github/workflows/ci.yml` (Ubuntu, Windows, macOS). No hay job de Node porque el
  frontend todavía no tiene pruebas ni chequeo de tipos.
- Ramas: `main` y `develop` en `origin`; `develop` es la rama por defecto.

## Pendientes conocidos

- Este equipo no tiene `gcc`, así que `go test -race` solo corre en CI. Para correrlo en local,
  instala MinGW-w64 (por ejemplo, `winget install BrechtSanders.WinLibs.POSIX.UCRT`).
- No están instaladas las herramientas opcionales de Wails `nsis` (instalador, etapa 08) y `upx`.
