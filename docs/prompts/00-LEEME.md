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
| 09 | `09-monitoreo-miniaturas-y-freeze.md` | Miniaturas (la pantalla en blanco pasó a 10.21) | Módulo B |
| 10 | `10-plantilla-nueva-funcion.md` | Plantilla reutilizable por función | Fases 2–3 |
| 11 | `11-revision-y-auditoria.md` | Revisión de seguridad/privacidad al cerrar cada fase | Siempre |
| 12 | `12-broadcast-pantalla-webrtc.md` | Transmitir tu pantalla a los alumnos (WebRTC) ✅ | Fase 2 |
| 12.1 | `12.1-proyectar-pantalla-de-un-alumno.md` | Proyectar la pantalla de un alumno a la clase | Fase 2 |
| 12.2 | `12.2-transmision-seleccion-y-modos.md` | Mejora: transmitir a una selección; modos ventana y pantalla completa | Fase 2 |
| 13 | `13-portabilidad-linux-macos-y-empaquetado.md` | Linux/macOS + empaquetado multiplataforma | Fase 2 |
| 14 | `14-sandbox-ejecucion-entregas.md` | Correr entregas del alumno en sandbox Docker | Fase 3 |
| 15 | `15-flota-despliegue-actualizacion-diagnostico.md` | Despliegue masivo, auto-actualización y diagnóstico | Fase 3 |
| 16 | `16-comercializacion-y-licenciamiento.md` | Modelo de negocio (licencia vs. renta) + licenciamiento | Negocio |
| 17 | `17-documentacion-diagramas-iniciales.md` | Escribir los diagramas de `docs/diagramas/` (ver su README) | Docs |
| 18 | `18-pulido-consola-ux-manual-simulador.md` | Pulido de la consola: UX/UI, manual, diagramas y aula simulada | Pulido |
| 19 | `19-pulido-app-alumno-ux.md` | Pulido de la app del alumno: overlay de código, avisos y transparencia ✅ | Pulido |
| 20 | `20-auditoria-limpieza-artefactos.md` | Auditoría y limpieza de los artefactos de `docs/v1/ux/` (hecha el 12/11/2026; `docs/v1` ya no existe) | Docs |

Las funciones del backlog con prompt propio son `10.1` a `10.21` (hechas: 10.1 a 10.12; ver
`docs/progreso.md`). Las de 10.13 en adelante (redactadas el 12/11/2026):

| # | Archivo | Resultado | Para |
|---|---|---|---|
| 10.13 | `10.13-asistencia-y-bitacora.md` | Identidad del alumno (cuenta, nombre, grado, grupo), materias con su lista, asistencia y bitácora | `v0.1.0` |
| 10.14 | `10.14-terminar-la-clase.md` | Un botón que deja el laboratorio limpio para el siguiente grupo | `v0.1.0` |
| 10.15 | `10.15-nombres-y-mapa-del-aula.md` | Nombre de cada equipo y vista «Como en el salón» | `v0.1.0` |
| 10.16 | `10.16-grupos-guardados.md` | Selecciones con nombre y equipos de trabajo al azar | `v0.1.0` si hay tiempo |
| 10.17 | `10.17-entregar-codigo-desde-el-overlay.md` | El alumno entrega su código (`code.collect`) | `v0.1.0` |
| 10.18 | `10.18-perfiles-de-clase.md` | «Iniciar clase» con un perfil guardado por materia | Después |
| 10.19 | `10.19-encender-equipos-wake-on-lan.md` | Encender los equipos con Wake-on-LAN | Después |
| 10.20 | `10.20-cuestionario-con-puntaje.md` | Mejora de 10.12: cuestionario con respuesta correcta y puntaje (`/funcionalidad-mejora`) | Después |
| 10.21 | `10.21-pantalla-en-blanco.md` | Pantalla en blanco («Pausar pantallas»), la parte pendiente de 09 | `v0.1.0` |

Orden sugerido para `v0.1.0`: 10.21 → 10.13 → 10.14 → 10.15 → 10.17 → 15 (auto-actualización)
→ 11 (revisión) → pruebas en VM.

> Las etapas 12 a 19 son ampliaciones posteriores al Hito 1; no tienen un orden estricto entre sí.
> La `17` puede correrse temprano (tras el protocolo, etapa 03) para los diagramas base.
> Las etapas 18 y 19 comparten sistema de diseño: córrelas juntas o una tras otra, no por separado.
> Usa la plantilla `10` para las funciones del backlog que no tengan prompt propio.
> **Desde las etapas 18 y 19**, cada prompt de función (10.4 en adelante, 11 a 16) trae una
> sección **«Encaje con lo implementado»** dentro del bloque del prompt: dónde va en la consola
> (barra de acciones, ficha, vista «Participación», franja global) y en la app del alumno (tarjeta, aviso,
> tira, banda), qué diagramas actualiza y si llega a los simulados. Las reglas generales están en
> la «Definición de terminado» de `CLAUDE.md`; el encaje, en cada prompt. Si cambias de idea en
> una de esas decisiones, dilo al pedir la función: lo que digas manda sobre el prompt.
> En Claude Code, `/funcionalidad-nueva` y `/funcionalidad-mejora` hacen ese proceso por ti; ver
> `docs/desarrollo/comandos-claude.md`.

## Antes de empezar

- **Go** (versión estable reciente) y **Wails v2** (`wails doctor` debe pasar).
- En Windows: WebView2 (ya viene en Win 10/11) y las Build Tools de C para cgo.
- Node.js LTS (Wails lo usa para el frontend).
- Git y cuenta de GitHub.
- 2–3 máquinas virtuales o equipos del laboratorio para pruebas en red.
- Autorización del área de TI de la Universidad para instalar el agente en el laboratorio, y aviso formal a los alumnos sobre el monitoreo.
