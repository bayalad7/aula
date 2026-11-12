# Pausar pantallas (10.21): ubicación y decisiones

Estado: **implementado, sin commit** (12/11/2026). Prompt `docs/prompts/10.21-pantalla-en-blanco.md`,
con `/funcionalidad-nueva 10.21`. Sin capturas versionadas: las del arnés son
`scripts/ux/salida/166-172` (consola) y `scripts/ux/salida-alumno/92-94` (alumno), y salen en el CI.
El mapa tiene «Consola · Pausar pantallas» (`docs/ux/mapa/consola-pausa.webp`).

## Análisis

La pantalla en blanco estaba documentada desde la etapa 09 (protocolo §5.9–5.11, reglas 9–10,
D-013) y diseñada en el prototipo `apps/teacher/frontend/dev/prototipo/alumno-pausa.html`, pero
retirada del código. Es una **acción sobre el equipo** (la tapa y toma los clics), así que en el
alumno usa `raise` y lleva banda, y nunca llega a los alumnos simulados. En la consola tiene las
tres formas que piden las reglas de ubicación de `CLAUDE.md`: se aplica a la selección (Barra de
acciones), a un alumno (Pantalla grande) y sigue activa mientras dura (Franja global).

Choques que había que resolver con lo que ya existía:

- **Transmisión y proyección:** el caso típico es «pausar y mostrar algo». Si la pausa quedara
  encima, la transmisión no se vería.
- **Energía y control remoto:** su aviso debe verse siempre (reglas 26 y 36).
- **Control remoto y proyección de un alumno en pausa:** el profesor vería (y mostraría a la clase)
  la pausa, no la pantalla.
- **El texto:** el protocolo y el prototipo decían cosas distintas.
- **«Pide el foco cada medio segundo»** (D-013): pelea con la cuenta regresiva de energía y Windows
  lo impide a medias.

## Opciones de ubicación presentadas

1. **Panel + Pantalla grande + franja** (recomendada): panel «Pausar pantallas» con mensaje
   opcional, «Pausar pantalla» en Acciones sobre el alumno, Franja global con «Cambiar mensaje» y
   «Reanudar todo». Igual que Energía y Temporizador.
2. **Botón directo + franja:** pausa en un clic; el mensaje se agrega después desde la franja.
3. **Dentro del panel Transmitir:** sin botón nuevo en la barra, pero mezcla dos funciones.

## Decisiones (del usuario)

- Ubicación: la opción 1. El mensaje se cambia en su propio diálogo, desde la franja.
- Tomar el control o proyectar a un alumno en pausa **le quita la pausa primero**.
- **La transmisión queda encima de la pausa** (también la de un compañero), y lo mismo la cuenta de
  energía y el aviso del control remoto: la ventana de pausa cede el paso (`PauseOverlay.Yield`)
  y queda justo debajo de la ventana de Aula. El alumno que se proyecta no se pausa.
- Atajo **Ctrl+Mayús+P**: pausa a todos o, si ya hay pausa, reanuda a todos. Se toma en la fase de
  captura para que el editor (Monaco) no abra su paleta de comandos.
- La ventana de pausa pide el foco **solo al aparecer y al recibir un clic** (cambio a D-013).
- Una ventana nativa **por monitor**, con su DPI, en los colores `c-vista-solido` de `tokens.css`
  (los del prototipo), claro u oscuro según el sistema, con la hora al pie para que no parezca
  un cuelgue.

## Resultados

- Arnés de la consola (166–172) y del alumno (92–94) sin errores de JS ni violaciones serias de
  axe. Una violación inicial (`label-content-name-mismatch`): el tooltip del botón «Pausar» no
  contenía su texto visible; ahora empieza con «Pausar pantallas…».
- `pruebas-alumno.mjs`: la banda no se cierra ni se oculta con Esc, se va al reanudar sin quitar el
  foco, saca la ventana del modo compacto y queda en el orden fijo
  (`power, banda-control, banda-pausa, banda-vista, banda-mostrando, restrictions`).
- Lo nativo (la ventana de pausa, dos monitores, DPI distintos, Alt+Tab, Alt+F4, el tema) queda en
  los casos manuales A-42 a A-47 y en `docs/prueba-vms.md` §27.
