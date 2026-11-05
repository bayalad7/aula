# Cambios

Los cambios de Aula que nota quien la usa o la instala. Versionado semántico (`x.y.z`, ver
`CLAUDE.md`); las versiones de prueba llevan `-rc.N`. Para el detalle técnico, el historial de git
y `docs/progreso.md`.

## Sin publicar

Etapa 18: la consola del profesor rediseñada, con su manual, sus diagramas y el aula simulada.
Etapa 19: la app del alumno pulida, sobre el mismo sistema de diseño. Sin cambios al protocolo ni
a la configuración: los alumnos instalados siguen funcionando igual.

### App del alumno (etapa 19)

- **No interrumpe:** el código y los avisos llegan sin quitarle el teclado al alumno ni poner la
  ventana encima de su editor. Minimizada, parpadea en la barra de tareas; el título dice cuántos
  hay nuevos.
- **Cerrar ya no la quita:** la ventana se oculta y Aula sigue con su icono en el área de
  notificación (antes, cerrar terminaba el agente, o la ventana reaparecía sola en modo
  servicio).
- **Modo compacto:** una tira pequeña en una esquina con el estado y lo nuevo.
- **Código:** lo más nuevo arriba, sin saltos; **Ctrl+Mayús+C** copia el seleccionado; ↑/↓
  cambian de código; nombre del lenguaje y hora de llegada.
- **Avisos** en una esquina, con icono y nivel, sin empujar el código.
- **Transparencia:** bandas permanentes con orden fijo (energía, pantalla vista, restricciones,
  transmisión) que no se pueden cerrar; «Aula cerró…» se queda hasta leerlo.
- **Textos para el alumno:** estado tranquilo («Esperando a tu profesor») y por qué no conecta,
  en palabras llanas; lo técnico, plegado en «Detalles para TI».
- Modo oscuro y accesibilidad (WCAG AA, teclado) con el sistema de diseño de la consola.

### Nuevo

- **Navegación nueva:** Alumnos, Código y Aula simulada en un menú lateral, y Ayuda y Ajustes
  abajo. Las acciones (aviso, pantalla, restricciones y energía) se aplican sobre la selección de
  alumnos desde la barra de acciones.
- **Ficha del alumno:** su sistema, su versión, qué puede hacer su equipo y el último error que
  mandó, con su motivo.
- **Aula simulada:** alumnos de prueba dentro de la consola para probarla sin laboratorio, con
  fallas a propósito (lento, desconectado, no confirma). Siempre marcados y contados aparte; nunca
  reciben energía, restricciones ni la transmisión.
- **Envíos:** cada envío dice quién lo tiene y quién falta, y se le puede reenviar a quien falta.
- **Manual dentro de la consola:** búsqueda, ayuda de la pantalla actual con «?», «¿Falta un
  alumno?» paso a paso, y enlace al manual desde cada error.
- **Diagramas dibujados** en el manual, en claro y oscuro, con zoom y «Ver en grande», y una
  galería con filtro por público.
- **Comprobar la conexión:** desde «¿Falta un alumno?», revisa el anuncio, el puerto, la conexión
  segura con la clave del centro y la red y el firewall del equipo del profesor, y dice dónde se
  atoraría un alumno.
- **Atajos de teclado** en el manual.

### Mejorado

- **Sistema de diseño** propio (`internal/ui`), con modo claro y oscuro y contraste WCAG AA
  comprobado. La consola funciona a 900×600.
- **Errores** con la fórmula «qué pasó + qué hacer», junto al campo cuando es de un formulario,
  sin perder lo escrito.
- **Teclado y lectores de pantalla:** todo se usa con el teclado. El foco siempre se ve y vuelve
  al botón que abrió un panel. Se comprueba con axe-core y con un recorrido solo con Tab.

### Corregido

- Dos clics rápidos en «Emitir» mandaban dos ejercicios.
- Si abrir la clase fallaba, la pantalla de arranque borraba el canal y el nombre escritos.
- Un enlace a una sección del manual quedaba antes de ella cuando había diagramas arriba.
- El diagrama 03 mostraba un color como texto.

### Para quien instala

- El ejecutable de la consola pasa de 23,07 MB a 25,86 MB (+12 %), sobre todo por los diagramas
  dibujados (1,78 MB).
- Compilar sigue sin necesitar Node: los diagramas dibujados se guardan en el repositorio.
- La consola no carga nada de Internet.

## 0.3.0-rc.2 y anteriores

Versiones de prueba anteriores a este archivo; ver sus notas en GitHub Releases.
