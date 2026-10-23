# 16 · Comercialización y licenciamiento

**Prerrequisitos:** Hito 1 cerrado (08). Es una etapa tanto de negocio como de código; conviene correrla cuando ya tengas algo demostrable.
**Duración estimada:** 1 sesión de análisis + 2–3 de implementación.
**Nota:** la Parte A es análisis de modelo de negocio (compara opciones y recomienda); las decisiones de precio y las fiscales/legales las confirmas tú con un contador/asesor. La Parte B implementa el mecanismo técnico de licencias.

## Prompt

~~~~markdown
Lee CLAUDE.md, docs/protocol.md y docs/progreso.md. Vamos a definir e implementar cómo se
comercializa la app. Trabaja en dos partes y DETENTE al final de la Parte A para que yo apruebe
el modelo antes de escribir código.

## Parte A: análisis del modelo de negocio (docs/comercializacion.md, en español)
1. **Contexto de mercado.** Toma en cuenta:
   - Compradores: profesores individuales vs. escuelas/universidades (muchas públicas, con
     presupuesto limitado y compras por licitación o por ciclo escolar).
   - Operación: la app corre en LAN, a veces **sin internet**; la validación de licencia NO puede
     exigir conexión a un servidor para funcionar en clase.
   - Competencia: LanSchool y Netop/Vision (suscripción comercial) y Veyon (gratis, código
     abierto). Explica cómo te posicionas frente a cada uno.
2. **Modelos a comparar** (ventajas, desventajas, encaje con el contexto):
   - Licencia perpetua (pago único) por equipo, por aula o por institución.
   - Suscripción / renta por periodo (mensual, anual o por ciclo escolar).
   - Freemium: núcleo gratuito (p. ej. distribución de código) y funciones de pago (monitoreo,
     transmisión, sandbox).
   - Código abierto + soporte/implementación de pago.
3. **Criterios de decisión:** ingreso recurrente vs. pago único, costo de soporte y
   actualizaciones, fricción de compra institucional, piratería, y cómo se valida offline cada
   modelo.
4. **Cumplimiento de licencias de terceros (clave para poder vender):** revisa las licencias de
   las dependencias (Wails, pion, gorilla/nhooyr, zeroconf, screenshot, clipboard, Monaco, y las
   librerías de sistema como WebKitGTK en Linux). Señala cualquiera copyleft que condicione
   distribuir la app como software cerrado, y qué implica.
5. **Entregable:** una recomendación de modelo y una estructura de precios tentativa, con una
   nota de que los temas fiscales (facturación, SAT) y legales (términos, licencia de uso) se
   confirman con un asesor.
Espera mi aprobación del modelo.

## Parte B: implementación del licenciamiento (cuando yo apruebe)
Diseña para el modelo aprobado; estas reglas aplican a casi todos:
- Paquete `internal/licensing`. La licencia es un **token firmado** (Ed25519, con una clave de
  VENDEDOR distinta de la clave profesor↔agente) que contiene: id de licencia, titular, alcance
  (número de equipos/aulas o institución), tipo (perpetua | suscripción), fecha de emisión y,
  si es suscripción, de expiración, y la versión máxima cubierta.
- **Validación offline:** la app verifica la firma localmente, sin llamar a ningún servidor. Para
  suscripción, compara contra la fecha de expiración del token con un periodo de gracia; la
  renovación se hace entregando un token nuevo (archivo o cadena), no exigiendo conexión en clase.
- **Dónde aplica:** la verificación vive en la app del PROFESOR. Los agentes de los alumnos no
  requieren licencia propia (no pongas fricción en 40 equipos).
- **Periodo de prueba** configurable y mensajes claros de estado (activa, por expirar, expirada,
  inválida). Degradación amable al expirar: no borres datos ni rompas la clase a media sesión.
- **Privacidad:** el licenciamiento NUNCA envía datos de alumnos ni de uso a ningún servidor.
- Herramienta de línea de comandos para que TÚ generes y firmes licencias (`license issue ...`).

## Pruebas
- Licencia válida activa la app; firma inválida o alterada la rechaza.
- Suscripción expirada entra en periodo de gracia y luego en estado expirado, sin pérdida de datos.
- Prueba de que la validación funciona sin red.
- Prueba de que la clave privada de vendedor nunca está en el binario distribuido.

## Criterios de aceptación
- docs/comercializacion.md con el análisis y la recomendación.
- La app valida licencias offline y maneja los estados con claridad.
- `staticcheck ./...` y `go test ./...` pasan.

Actualiza docs/progreso.md y docs/comercializacion.md.
~~~~

## Verificación manual
- [ ] Desconecta de internet y prueba una licencia: ¿valida igual?
- [ ] Altera un byte del token: ¿lo rechaza?
- [ ] ¿La app expirada deja terminar la clase en curso sin romperse?

## Nota de negocio
Para instituciones públicas, una **licencia por aula/laboratorio con actualizaciones por un año**
suele tener menos fricción que la suscripción (encaja con compras por ciclo y sin dependencia de
internet). Guárdalo como hipótesis a validar en la Parte A, no como decisión tomada.
