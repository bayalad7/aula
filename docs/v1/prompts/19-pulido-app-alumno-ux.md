# 18 · Pulido de la app del alumno: overlay de código, avisos y transparencia

**Prerrequisitos:** etapa 06 (cliente de código del alumno) hecha; idealmente la etapa 17 ya corrida, para reutilizar su sistema de diseño. Si 17 no está, esta etapa establece los tokens compartidos.
**Duración estimada:** varias sesiones, por entregas pequeñas.
**Tema central:** la app del alumno es lo que ven 40 personas a la vez. Debe ser clara, poco intrusiva y, sobre todo, **transparente**: el alumno siempre puede ver qué observa o controla el profesor.

## Prompt

~~~~markdown
Lee CLAUDE.md, docs/protocol.md, docs/privacidad.md, docs/progreso.md y TODO el código de la app
del alumno (apps/student: frontend embebido, bindings Go) y los paquetes que consume
(internal/transport, internal/session, internal/codedist y, si existen, internal/broadcast y los
de monitoreo/control de Fase 2/3). Revisa el sistema de diseño de la etapa 17 si ya existe.

# Objetivo
Pulir la interfaz de la app del alumno para los primeros usuarios reales, sin agregar funciones de
aula nuevas. Tres metas:
1. Mejorar la UX/UI de todo lo que el alumno ve (overlay de código, avisos, estado de conexión).
2. Hacer impecables y permanentes los indicadores de transparencia (pantalla vista, congelada,
   transmisión, control remoto).
3. Que la app sea poco intrusiva y ligera: no estorbar mientras el alumno trabaja y consumir pocos
   recursos en equipos modestos.

# Para quién diseñamos
El alumno: puede no ser muy técnico, a veces usa solo teclado, y está concentrado programando.
La app no debe distraerlo salvo cuando el profesor realmente le envía algo.
Meta medible: cuando el profesor emite código, el alumno lo ve y lo copia intacto en menos de 5
segundos, sin perder lo que estaba escribiendo en su editor.

# Restricciones (no las rompas)
- Arquitectura Wails: frontend embebido con go:embed, sin CDNs externos, funcionando sin internet.
- Reutiliza el sistema de diseño de la etapa 17 (tokens, componentes, modo oscuro). Si no existe
  aún, define aquí los tokens compartidos en un lugar que la consola pueda reutilizar después; no
  dupliques estilos entre las dos apps.
- No cambies el protocolo ni el modelo de seguridad. Esta etapa es de interfaz.
- Transparencia (no negociable): todo indicador de que el profesor observa o controla el equipo es
  SIEMPRE visible y NO se puede ocultar ni desactivar desde la app del alumno. Nada de ejecución
  oculta.
- Español de México, claro y sin jerga. Textos externalizados en un solo archivo para i18n futura.
- Huella mínima: mide y reporta CPU y RAM en reposo y mientras recibe; apunta a que sea ligera en
  equipos modestos.

# FASE 1 — Análisis (no escribas código del frontend en esta fase)
1. Recorre la app del alumno en todos sus estados y captura cada uno (docs/ux/v1/alumno/antes):
   sin conexión / buscando clase, conectado en reposo, recibiendo un snippet, con varios snippets,
   con un aviso (info y warning), con la pantalla siendo vista, con pantalla en blanco (freeze),
   recibiendo la transmisión del profesor, y siendo controlado remotamente (si esas funciones
   existen).
2. Recorre y anota la fricción de estos momentos:
   a) el equipo arranca → la app se conecta sola al canal → el alumno entiende a qué canal está
      conectado
   b) el profesor emite código → aparece el overlay → el alumno copia y pega en su editor
   c) el profesor manda un aviso → el alumno lo ve sin que le tape el trabajo
   d) el profesor ve/congela/transmite/controla → el alumno entiende qué está pasando
3. Evaluación heurística (Nielsen) y accesibilidad WCAG 2.1 AA (contraste, foco visible, teclado,
   ARIA) sobre el frontend del alumno.
4. Revisa los textos: ¿el distintivo de identidad dice a qué clase y profesor está conectado? ¿los
   avisos de monitoreo se entienden sin tecnicismos? ¿los estados de error (perdió conexión,
   reconectando) son claros?
5. Punto clave a evaluar: ¿el overlay de código roba el foco del teclado y le interrumpe lo que
   escribe? Documenta el comportamiento actual y propón cómo notificar sin arrebatar el foco.
6. Entrega docs/ux/v1/alumno/ANALISIS-ALUMNO.md con hallazgos priorizados (cada uno con captura, heurística,
   impacto y propuesta), flujos actuales vs propuestos, y la lista de cambios en entregas pequeñas.
   Espera mi aprobación antes de seguir.

# FASE 2 — Sistema de diseño (reutilizar)
- Toma los tokens y componentes de la etapa 17. Si no existe, créalos en un paquete/estilo
  compartido que luego la consola reutilice; no dupliques.
- Componentes propios del alumno: overlay de código (con bloque de código y botón Copiar), tarjeta
  de aviso (toast), barra/insignia de estado de conexión, banda de transparencia permanente
  (vista/congelado/transmisión/control) y pantalla de "sin clase".
- Respeta contraste AA y modo oscuro.

# FASE 3 — Mejoras de UX/UI
Evalúa e implementa lo que corresponda:
1. **Overlay de código:**
   - Legible (Monaco o Prism según la etapa 06), con lenguaje y título visibles.
   - Botón "Copiar" robusto (portapapeles nativo desde Go) y atajo de teclado para copiar; conserva
     saltos de línea y sangría.
   - Varios snippets (modo append) en una lista navegable; "replace" sustituye; "clear" limpia.
   - Redimensionable y que se pueda minimizar a una insignia pequeña; al llegar un snippet nuevo,
     avisa sin arrebatar el foco del editor del alumno (p. ej. resalta la insignia en vez de saltar
     al frente de forma agresiva). Esto es criterio de aceptación, no opcional.
2. **Avisos (msg.show):** toast legible, info y warning con color + icono + texto, que no tape el
   área de trabajo y se pueda cerrar; responde msg.ack.
3. **Estado de conexión:** indicador no intrusivo (barra pequeña o icono) que diga "Conectado al
   canal <X> · Profesor <profesor>" o "Buscando clase…/Reconectando…", sin molestar en reposo.
4. **Banda de transparencia permanente:** mientras el profesor ve, congela, transmite o controla,
   una banda clara e imposible de ocultar explica qué ocurre en lenguaje sencillo ("Tu pantalla la
   está viendo el profesor", "Pantalla en pausa por el profesor", "El profesor está controlando tu
   equipo"). Al terminar, desaparece sola.
5. **Pantalla en blanco (freeze):** overlay a pantalla completa que explica por qué y que no parece
   un cuelgue del equipo.
6. **Pantalla de "sin conexión":** cuando no hay profesor en el canal, un estado tranquilo que no
   alarma y que indica que la app está lista y esperando.

# FASE 4 — Accesibilidad, rendimiento e intrusión
1. Accesibilidad: navegación por teclado, foco visible, contraste AA, etiquetas/ARIA, textos
   alternativos en iconos de estado.
2. Rendimiento: mide CPU y RAM en reposo y recibiendo; evita animaciones costosas; que la app sea
   ligera en equipos modestos del laboratorio. Reporta las cifras.
3. No intrusión: confirma que la app no roba el foco del teclado salvo cuando el alumno elige abrir
   el overlay, y que nada tapa permanentemente su trabajo salvo las acciones legítimas del profesor
   (freeze, control), que sí deben ser evidentes.

# FASE 5 — Verificación
1. Frontend del alumno probado en un arnés web local con axe-core (sin violaciones críticas ni
   serias) y navegación completa por teclado. NOTA: como en la etapa 17, Wails usa un WebView
   nativo, así que Playwright no maneja la ventana nativa; prueba el frontend por el arnés, los
   bindings con `go test`, y deja lo nativo a chequeo visual.
2. Prueba (automatizada donde se pueda) de que los indicadores de transparencia NO se pueden
   ocultar ni desactivar desde la app del alumno, y de que aparecen siempre que la función
   correspondiente está activa.
3. Prueba de que un snippet nuevo no arrebata el foco del editor del alumno.
4. Prueba del portapapeles con un fragmento multilínea con sangría.
5. Capturas "después" en docs/ux/v1/alumno/despues/ y un informe docs/ux/v1/alumno/RESULTADOS-ALUMNO.md
   (antes/después por estado, hallazgos resueltos, cifras de CPU/RAM).
6. `go build` sin Node, `go test -race ./...` en los tres sistemas, binario autocontenido y tamaño
   agregado reportado. Actualiza docs/progreso.md y docs/privacidad.md si cambió algún texto de
   aviso.

# Forma de trabajo
- Por fases y entregas pequeñas; al terminar cada una, muéstrame capturas y un resumen, y espera mi
  aprobación.
- Orden sugerido tras aprobar el análisis: sistema de diseño (o reutilizarlo) → overlay de código →
  avisos y estado de conexión → banda de transparencia y freeze → accesibilidad/rendimiento →
  verificación.
- Si una mejora exige cambiar el protocolo o el modelo de seguridad, no la implementes: anótala
  como propuesta en docs/ux/PROPUESTAS.md.
~~~~

## Relación con otras etapas
- **Comparte sistema de diseño con la etapa 17.** Lo ideal es correr 17 primero (o esta y luego
  reutilizar en 17); lo que no debe pasar es tener dos sistemas de estilo distintos entre consola y
  app del alumno.
- **Refuerza la privacidad de las etapas 09, 12 y del control remoto:** aquí los indicadores de
  transparencia dejan de ser un detalle de cada función y se vuelven un componente formal, probado
  y no ocultable.
