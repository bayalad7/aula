# Prompts maestros: proyecto "Aula" (Go + Wails)

Sistema multiplataforma de gestión de aula (estilo LanSchool / Veyon) con un módulo diferenciador de distribución de código para clases de programación.

> **Aula** es un nombre provisional. Si lo cambias, reemplázalo en todos los prompts y en `CLAUDE.md`.

## Cómo usar estos prompts

1. Úsalos **en orden**, **uno por sesión** de Claude Code (`/clear` entre cada uno).
2. El archivo `01` no es un prompt: es el contenido de `CLAUDE.md`. Cópialo a la raíz del repositorio antes del prompt `02`. Claude Code lo leerá en cada sesión.
3. Copia solo el bloque dentro de la sección **Prompt** de cada archivo.
4. Antes de pasar al siguiente, verifica los **criterios de aceptación**. Si algo falla, corrígelo en la misma sesión.
5. Al terminar cada sesión, haz commit en `develop`. **Solo se usan dos ramas, `develop` y `main`; sin ramas de feature.** `main` solo recibe releases probados, etiquetados `vX.Y.Z` (SemVer).
6. Si una sesión se alarga, pide a Claude que actualice `docs/progreso.md` y continúa en una sesión nueva con: *"Lee docs/progreso.md y continúa donde quedamos."*

## Mapa de módulos (tu documento) → etapas

| Tu módulo | Etapas que lo construyen |
|---|---|
| A — Red, descubrimiento mDNS con canales, WebSocket core | 03, 04, 05 |
| C — Captura y distribución de código (diferenciador) | 06, 07 |
| B — Control y monitoreo en tiempo real | 09 |
| D — Instalación y despliegue | 02 (base), 08 (Hito 1), y cada etapa aporta su empaquetado |

> **Nota de orden:** aunque tu documento lista B antes que C, el plan construye **primero C** (distribución de código). Así terminas antes lo que más usarás en clase y tienes algo instalable y útil en el Hito 1, dejando el monitoreo para la Fase 2.

## Orden y alcance

| # | Archivo | Resultado | Fase |
|---|---|---|---|
| 01 | `01-CLAUDE.md` | Reglas del proyecto para Claude Code | Base |
| 02 | `02-entorno-y-repositorio.md` | Monorepo Go + dos apps Wails compilando, Git, CI | Base |
| 03 | `03-especificacion-protocolo.md` | `docs/protocol.md` y paquete `protocol` | Módulo A |
| 04 | `04-descubrimiento-mdns-y-servidor.md` | mDNS con canales + servidor/cliente WebSocket | Módulo A |
| 05 | `05-seguridad-tls-y-autenticacion.md` | Canal TLS y agentes que solo obedecen a la consola | Módulo A |
| 06 | `06-cliente-alumno-codigo.md` | App estudiante: overlay con Monaco y botón Copiar | Módulo C |
| 07 | `07-consola-profesor.md` | App profesor: lista, hotkey de captura, emisión | Módulo C |
| 08 | `08-hito1-integracion.md` | Distribución de código de punta a punta + instaladores | **Hito 1 ✔** |
| 09 | `09-monitoreo-miniaturas-y-freeze.md` | Miniaturas y pantalla en blanco (con aviso visible) | Módulo B |
| 10 | `10-plantilla-nueva-funcion.md` | Plantilla reutilizable por función | Fases 2–3 |
| 11 | `11-revision-y-auditoria.md` | Revisión de seguridad/privacidad al cerrar cada fase | Siempre |
| 12 | `12-broadcast-pantalla-webrtc.md` | Transmitir tu pantalla a los alumnos (WebRTC) | Fase 2 |
| 13 | `13-portabilidad-linux-macos-y-empaquetado.md` | Linux/macOS + empaquetado multiplataforma | Fase 2 |
| 14 | `14-sandbox-ejecucion-entregas.md` | Correr entregas del alumno en sandbox Docker | Fase 3 |
| 15 | `15-flota-despliegue-actualizacion-diagnostico.md` | Despliegue masivo, auto-actualización y diagnóstico | Fase 3 |
| 16 | `16-comercializacion-y-licenciamiento.md` | Modelo de negocio (licencia vs. renta) + licenciamiento | Negocio |

> Las etapas 12 a 16 son ampliaciones posteriores al Hito 1; no tienen un orden estricto entre sí.
> Usa la plantilla `10` para las funciones del backlog que no tengan prompt propio.

## Antes de empezar

- **Go** (versión estable reciente) y **Wails v2** (`wails doctor` debe pasar).
- En Windows: WebView2 (ya viene en Win 10/11) y las Build Tools de C para cgo.
- Node.js LTS (Wails lo usa para el frontend).
- Git y cuenta de GitHub.
- 2–3 máquinas virtuales o equipos del laboratorio para pruebas en red.
- Autorización del área de TI de la Universidad para instalar el agente en el laboratorio, y aviso formal a los alumnos sobre el monitoreo.
