# 17 · Pulido de la consola: UX/UI, manual integrado, diagramas y aula simulada

**Prerrequisitos:** Hito 1 cerrado (08); mejor aún con algunas funciones de Fase 2 ya hechas, para que haya UX que pulir. Requiere que existan diagramas en `docs/diagramas/` (si no, esta etapa crea los básicos a partir de `docs/protocol.md`).
**Duración estimada:** varias sesiones; es una fase por entregas pequeñas, no una sola pasada.
**Origen:** adaptación del prompt de pulido del panel de PRISMA v1.1 a la consola de Aula (Wails de escritorio, no panel web).

## Prompt

~~~~markdown
Lee CLAUDE.md, docs/protocol.md, docs/privacidad.md, docs/progreso.md, docs/compatibilidad.md y
TODO el código de la app del profesor (apps/teacher: frontend embebido, bindings Go) y los
paquetes que consume (internal/discovery, internal/transport, internal/session, internal/codedist
y, si existen, internal/broadcast y los de Fase 2/3). Revisa también los ejemplos de agentes
simulados (examples/dos_alumnos u otros).

# Objetivo
Preparar la consola del profesor para los primeros usuarios reales, sin agregar funciones nuevas
de aula. Cuatro metas:
1. Mejorar la UX/UI de la consola.
2. Incluir un manual de usuario dentro de la propia consola.
3. Mostrar dentro de la consola los diagramas de arquitectura y protocolo de docs/diagramas/.
4. Permitir probar el sistema de principio a fin SIN un laboratorio físico: un modo de "aula
   simulada" con alumnos simulados.
NO se agregan funciones nuevas de aula (las de Fase 2/3 que no existan se quedan para sus prompts).

# Para quién diseñamos
1. Profesor no muy técnico: quiere "crear su aula y mandar código" y entender qué pasa cuando algo
   no conecta.
2. Área de TI / instalador: configura la consola y genera los agentes; necesita hacerlo rápido.
3. (Alumno: solo como receptor; su overlay comparte el sistema de diseño, pero no es el foco.)
Meta medible: un profesor no técnico, sin ayuda, pasa de "recién instalado" a "canal creado +
primer código emitido + un alumno simulado recibiéndolo" en menos de 5 minutos.

# Restricciones (no las rompas)
- Mantén la arquitectura de Wails: frontend (HTML/CSS/JS) embebido con go:embed, sin CDNs externos,
  funcionando SIN internet (manual, diagramas y aula simulada incluidos). Si propones una librería
  pequeña, justifícala y embébela.
- No cambies el protocolo, el modelo de confianza ni la regla de oro de CLAUDE.md. La ÚNICA
  excepción es el modo de aula simulada (alumnos simulados dentro del proceso del profesor o como
  binarios de agente locales), que debe estar siempre claramente marcado y aislado de los alumnos
  reales.
- Respeta la transparencia y la privacidad: los alumnos simulados nunca se confunden con equipos
  reales en la lista, en los logs ni en las recolecciones. Todo SVG y Markdown renderizado se
  sanitiza.
- docs/ es la ÚNICA fuente del manual y de los diagramas: la consola nunca tiene copias editadas a
  mano.
- `go build` sigue funcionando sin Node; los pasos que usen Node (generar SVG de diagramas,
  pruebas de frontend) son previos al build y corren también en CI.
- Todo en español de México, claro y sin jerga. Los textos van externalizados en un solo archivo
  para facilitar i18n futura.
- Debe verse bien al tamaño de ventana de escritorio típico y adaptarse si la ventana se encoge.

# FASE 1 — Análisis (no escribas código de la consola en esta fase)
1. Recorre la consola real con datos de demostración (un canal, 2 alumnos simulados, historial de
   snippets, un trabajo/acuse fallido). Captura cada pantalla y estado (vacío, con datos, error,
   cargando). Guarda las capturas en docs/ux/antes/.
2. Recorre estos flujos clave, cuenta pasos y clics, y anota cada fricción:
   a) primer arranque → crear perfil/claves (teacher keygen) → crear o elegir canal
   b) generar la configuración de un agente (con clave pública y canal) para instalarlo
   c) emitir código a todos → ver acuses → reemitir un snippet del historial
   d) ver la lista de alumnos y entender quién está conectado y quién no
   e) entender por qué un alumno no aparece / no conecta y qué hacer
   f) (si existen) ver miniaturas, transmitir pantalla, recoger código
3. Evaluación heurística (10 heurísticas de Nielsen) y revisión de accesibilidad WCAG 2.1 AA
   (contraste, foco visible, navegación por teclado, etiquetas, ARIA) sobre la capa de frontend.
4. Revisa los textos: mensajes de error (¿dicen qué pasó y qué hacer?), etiquetas técnicas sin
   explicar (canal, huella de certificado, clave pública, capacidades, heartbeat), confirmaciones
   de acciones destructivas y estados vacíos.
5. Inventario de docs/diagramas/: por diagrama, tipo Mermaid, de qué trata, para qué público sirve
   (profesor, TI, integrador) y en qué pantallas aportaría valor. Señala los desactualizados. Si
   casi no hay diagramas, lista los mínimos a crear (arquitectura general, ciclo de sesión y
   autenticación, descubrimiento por canal, diagnóstico de conexión).
6. Revisa qué hay para probar sin laboratorio (ejemplos de agentes simulados, el registro de
   sesiones) y qué se reutiliza para el modo de aula simulada de la Fase 6.
7. Entrega docs/ux/ANALISIS-CONSOLA.md con: hallazgos priorizados (crítico/alto/medio/bajo) cada
   uno con captura, heurística, impacto y propuesta; flujos actuales vs propuestos; arquitectura de
   navegación (flowchart Mermaid) incluyendo Ayuda y Aula simulada; el inventario de diagramas; la
   evaluación de cómo mostrar diagramas (Fase 5) con tu recomendación; el plan de reutilización del
   punto 6; y la lista de cambios en entregas pequeñas.
   Espera mi aprobación antes de seguir.

# FASE 2 — Sistema de diseño ligero
Sistema de diseño mínimo y consistente para la consola (y reutilizable en el overlay del alumno):
- Tokens CSS: colores (incluidos estados: conectado, desconectado, advertencia, error, simulado),
  tipografía del sistema, espaciado, radios, sombras, con contraste AA. Modo oscuro respetando
  prefers-color-scheme con selector manual.
- Componentes: botones (primario, secundario, peligro), campos con ayuda y error en línea,
  tarjetas de alumno, tablas que se vuelven tarjetas en ventana angosta, insignias de estado
  (incluida "Simulado"), avisos (toast), diálogos de confirmación, estados vacíos con acción
  sugerida, indicadores de carga, pasos de asistente, bloque de código con "Copiar", visor de
  diagramas (Fase 5).
- Iconos SVG embebidos en un solo sprite.
- Pantalla interna de catálogo de componentes, solo en modo desarrollo.
Muéstrame un prototipo de 3 pantallas clave con el sistema aplicado antes de rediseñar el resto.

# FASE 3 — Mejoras de UX/UI (aplica lo aprobado en la Fase 1)
Evalúa e implementa lo que corresponda:
1. Asistente de primer uso (se puede saltar y retomar): crear perfil/claves → crear canal →
   generar la configuración del primer agente (con botón copiar) → "¿Ya tienes alumnos?" (todavía
   no: crear un aula simulada con 2 alumnos de la Fase 6) → emitir el primer código y verlo llegar.
   Al final, una lista de verificación persistente en Inicio ("3 de 5 pasos").
2. Inicio: estado de un vistazo (canal actual, alumnos conectados/desconectados, reales vs
   simulados, vencimiento del certificado) con acciones directas desde cada tarjeta.
3. Lista de alumnos: estado con color + icono + texto (nunca solo color), separando claramente los
   simulados; detalle por alumno con sus capacidades.
4. Panel de código: editor con lenguaje y título, emitir/agregar/limpiar, historial reemitible,
   conteo de acuses recibidos.
5. Mensajes: reescribe TODOS los errores y avisos con la fórmula "qué pasó + por qué + qué hacer".
   Mapea cada código de error del protocolo a un mensaje amable y a la sección del manual.
6. Formularios: validación en línea, sin perder datos al fallar, confirmación de acciones
   destructivas indicando qué se pierde.
7. Navegación: Inicio · Alumnos · Código · (Pantalla/Transmitir/Recoger si existen) · Aula simulada
   · Recursos · Ajustes · Ayuda. Ruta actual visible; atajos de teclado básicos.
8. Rendimiento percibido: estados de carga, sin saltos de diseño.

# FASE 4 — Manual de usuario dentro de la consola
1. Sección "Ayuda", disponible sin internet:
   - Manual en Markdown en docs/administracion/ (fuente única), embebido con go:embed y renderizado
     a HTML sanitizado (librería Go pequeña justificada, o preprocesado en el build).
   - Contenido: Primeros pasos · Conceptos (canal, agente, clave pública, huella de certificado,
     capacidades, heartbeat) sin jerga · Una guía por pantalla · Tareas frecuentes · Probar sin
     laboratorio · Solución de problemas (un alumno no aparece, no conecta, el canal equivocado, el
     firewall, la clave no coincide) · Preguntas frecuentes · Glosario · Cómo funciona Aula
     (diagramas, Fase 5).
   - Búsqueda local dentro del manual (índice generado en el build), que incluya títulos y textos
     de los diagramas.
   - Capturas generadas automáticamente (ver nota de la Fase 7 sobre Wails).
2. Ayuda contextual: un botón "?" en cada pantalla abre un panel con la sección del manual de esa
   pantalla y sus diagramas, sin perder lo que el profesor estaba haciendo. Iconos de información
   junto a los campos técnicos.
3. Árbol interactivo "Mi alumno no conecta": preguntas sí/no que llevan a la solución, generado a
   partir del diagrama de diagnóstico de docs/diagramas/ (fuente única; si el formato no lo
   permite, propón una convención en el Markdown). Debe atar con el comprobador de conexión de la
   Fase 6 y con el diagnóstico de la etapa 15.
4. Los mensajes de error de la consola enlazan a la sección exacta del manual.
5. Una prueba falla si una pantalla no tiene su sección de ayuda, o si un código de error no tiene
   mensaje amable con enlace al manual.

# FASE 5 — Diagramas de docs/diagramas/ dentro de la consola
Objetivo: cualquier diagrama de docs/diagramas/*.md se ve en la consola, sin internet, legible, y
siempre sincronizado con docs/.
1. Evalúa y recomienda (con tamaños medidos) cómo renderizarlos. Mi preferencia inicial es:
   a) Pre-render en el build con @mermaid-js/mermaid-cli (mmdc) a SVG (variante clara y oscura),
      embebidos; consola sin JS de Mermaid. Alternativas: (b) Mermaid en el navegador embebido solo
      en Ayuda; (c) híbrido. Criterios: tamaño del binario, sin internet, seguridad/CSP, fidelidad,
      y que `go build` no dependa de Node.
2. Front-matter YAML en cada docs/diagramas/NN-nombre.md (titulo, descripcion, audiencia,
   pantallas, secciones_manual), validado con un esquema; la prueba falla si falta o es inválido.
3. Pipeline (scripts/docs/generar-diagramas.{sh,ps1} + CI): por diagrama, SVG claro/oscuro, texto
   alternativo y un manifiesto JSON (id, metadatos, hash del bloque mermaid, rutas). Mermaid con
   htmlLabels desactivado y tema acorde a los tokens. Sanitiza los SVG (sin script, on*,
   foreignObject ni enlaces externos). Si el hash no coincide con el manifiesto, la prueba y el CI
   fallan con "Diagramas desactualizados: ejecuta scripts/docs/generar-diagramas".
4. Visor de diagramas (componente del sistema de diseño): se adapta al ancho, zoom y "ver en
   grande" dentro de la consola, cambia claro/oscuro con el tema, muestra título/descripción/público
   y la explicación del .md, accesible por teclado, y si falta un SVG muestra el código Mermaid en
   vez de fallar.
5. Dónde aparecen: galería "Cómo funciona Aula" con filtro por público y búsqueda; dentro del
   manual con una sintaxis de inclusión {{diagrama:NN-nombre}} (una prueba falla si el id no
   existe); y en la ayuda contextual "?" de cada pantalla según el front-matter.

# FASE 6 — Aula simulada (probar sin laboratorio físico)
Objetivo: que el profesor pruebe Aula de principio a fin SIN equipos de alumno reales.
1. Modo de aula simulada: levanta N alumnos simulados que se conectan al canal y se comportan como
   agentes reales ante code.share, msg.show, heartbeat y (si existen) screen.thumb y code.collect.
   Reutiliza los ejemplos de agentes simulados. Pueden correr dentro del proceso del profesor o
   como binarios de agente locales; decide y justifícalo.
   - Cada alumno simulado lleva SIEMPRE la insignia "Simulado" en la lista, en el registro y en
     cualquier recolección; nunca se mezcla ni se cuenta como real.
   - Simulación de fallas para probar: "desconecta", "lento" (retraso configurable) y "no acusa",
     usando solo estados que el sistema ya sabe manejar.
   - El asistente de primer uso ofrece crear un aula simulada para ver el primer código llegar.
2. Pantalla "Aula simulada": crear/eliminar alumnos simulados, elegir su SO declarado y sus
   capacidades, disparar las fallas, y ver en vivo cómo reciben lo que emites.
3. Comprobador de conexión ("¿Por qué un alumno no conecta?"): simula el camino que sigue un agente
   —ve el anuncio mDNS del canal → alcanza el puerto WebSocket → la huella del certificado coincide
   → la clave pública configurada es la correcta → el firewall no bloquea— y muestra en qué paso
   fallaría, con el mensaje amable y enlace al manual y al diagrama de diagnóstico. Ata con la
   etapa 15.
4. Privacidad y seguridad: el modo simulado no abre rutas nuevas del protocolo ni afecta a alumnos
   reales; una prueba verifica que los alumnos simulados no aparecen como reales en logs ni en
   recolecciones.

# FASE 7 — Verificación
1. Pruebas de la capa de frontend: sírvela en un arnés web local y pásale axe-core (sin violaciones
   críticas ni serias) y pruebas de interacción; navegación completa por teclado. NOTA: Wails
   renderiza en un WebView nativo, no en una URL navegable, así que Playwright no puede manejar la
   ventana nativa como en un panel web; prueba el frontend por ese arnés, los bindings Go con
   `go test`, y deja chequeos visuales/manuales para la ventana nativa. Si decides generar capturas
   automáticas, hazlo contra el arnés web, no contra la ventana de Wails.
2. Pruebas de diagramas: todos los docs/diagramas/*.md con front-matter válido, SVG claro/oscuro
   vigentes (hash), SVG sanitizados (prueba que busque script, on*, foreignObject y URLs externas),
   y cada {{diagrama:...}} existente.
3. Pruebas de la aula simulada: un alumno simulado recibe y acusa como uno real; las fallas
   simuladas se reportan con los estados esperados; los simulados no persisten ni se cuentan como
   reales; el comprobador de conexión coincide con el diagnóstico real.
4. Capturas "después" en docs/ux/despues/ y un informe docs/ux/RESULTADOS.md: antes/después por
   pantalla, pasos y clics por flujo (antes vs después), hallazgos resueltos y pendientes.
5. Prueba de "usuario nuevo": sigue el manual desde una instalación limpia y SIN alumnos reales
   (solo aula simulada) y reporta cualquier paso confuso.
6. Actualiza CHANGELOG.md, docs/administracion/, docs/progreso.md y agrega el diagrama de la aula
   simulada. Verifica que el binario siga autocontenido, reporta el tamaño agregado, y confirma que
   `go build` funciona sin Node y que `go test -race ./...` pasa en los tres sistemas.

# Forma de trabajo
- Por fases y entregas pequeñas; al terminar cada una, muéstrame capturas y un resumen, y espera mi
  aprobación.
- Orden sugerido tras aprobar el análisis: sistema de diseño → aula simulada (Fase 6.1, porque el
  asistente y las pruebas la usan) → mejoras de UX/UI → manual → diagramas → verificación.
- Si una mejora exige cambiar el protocolo o el modelo de seguridad (fuera de la excepción de la
  aula simulada), no la implementes: anótala como propuesta en docs/ux/PROPUESTAS.md.
~~~~

## Notas de adaptación (por qué difiere del de PRISMA)
- **Panel web → consola de escritorio.** Aula no tiene un panel HTTP con sesión, CSRF ni filtrado
  por IP privada; es una app Wails local. Por eso se quitaron esos puntos de seguridad del panel y
  se conserva el modelo de confianza del proyecto (claves Ed25519, TLS).
- **Impresora virtual → aula simulada.** El análogo de "probar sin hardware" es levantar alumnos
  simulados; reutiliza los ejemplos de agentes simulados que ya piden las etapas 04 y 08.
- **Playwright no maneja la ventana de Wails** como manejaba el panel web de PRISMA: la Fase 7 lo
  dice y propone probar el frontend en un arnés web + los bindings con `go test`.
