# Progreso

Última actualización: 2026-10-23 (etapa 03).

| Etapa | Descripción | Estado |
|---|---|---|
| 02 | Entorno y repositorio: monorepo Go + dos apps Wails, Git, CI | ✅ Completada |
| 03 | Especificación del protocolo (`docs/protocol.md`, paquete `protocol`) | ✅ Completada |
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

## Entorno de desarrollo (equipo del profesor, Windows 10)

- gcc 16.2.0 (WinLibs POSIX UCRT, instalado con
  `winget install BrechtSanders.WinLibs.POSIX.UCRT`) para cgo y `go test -race`.
- winget lo instala bajo `C:\Users\Benjamín Ayala\...`. `ld` no soporta el espacio de esa ruta
  (falla con `cannot find C:/Users/Benjam?n` al buscar `default-manifest.o`), así que se
  configuró:
  - junction `C:\mingw64` apuntando a
    `...\WinGet\Packages\BrechtSanders.WinLibs.POSIX.UCRT_Microsoft.Winget.Source_8wekyb3d8bbwe\mingw64`;
  - `go env -w CC=C:\mingw64\bin\gcc.exe`;
  - variable de entorno de usuario `GCC_EXEC_PREFIX=C:/mingw64/lib/gcc/`. Sin ella, gcc resuelve
    su ruta real (con espacio) aunque se invoque por el junction o por el nombre corto 8.3.
- Las terminales y VS Code abiertos antes del cambio deben reiniciarse para ver
  `GCC_EXEC_PREFIX`.
- Si `winget upgrade` cambia la carpeta del paquete, hay que recrear el junction:
  `New-Item -ItemType Junction -Path C:\mingw64 -Target <nueva ruta>\mingw64`.
- Con gcc en el PATH, `CGO_ENABLED` vale 1 por defecto en local. Los binarios publicados se
  compilan sin cgo en CI.
- Verificado el 2026-10-23: `go test -race ./...` pasa en local.

## Pendientes conocidos

- No están instaladas las herramientas opcionales de Wails `nsis` (instalador, etapa 08) y `upx`.

## Etapa 03: notas

- Especificación aprobada en `docs/protocol.md` (protocolo v1). Cambios respecto al borrador:
  la autenticación va antes de `hello` (el agente no se identifica ante una consola sin la clave
  correcta) y `code.share` tiene un campo opcional `exercise`.
- La firma de `auth.proof` cubre etiqueta, nonce, canal y huella del certificado TLS (§3.2).
- `internal/protocol`: `Envelope`, `Encode`/`Decode` con validación de versión y límites,
  payloads tipados de los 11 mensajes v0, `Capability`, `HelloInfo`, `NewID` (UUID v4) y
  `ErrorCodeFor`. Solo usa la biblioteca estándar. Decisión de diseño en D-004.
- Pruebas: ida y vuelta de cada tipo, ejemplos en `internal/protocol/testdata/{valid,invalid}`
  (los inválidos se nombran con el código de error esperado), versión desconocida, mensaje y
  contenido demasiado grandes, JSON inválido y un fuzz de `Decode` (`FuzzDecode`).
- Pendiente para etapas siguientes: la consola no autentica al agente en v0 (limitación
  documentada en §3.2).
