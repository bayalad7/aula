# Propuestas que cambian el protocolo o el modelo de seguridad

Los prompts de UX (18, consola del profesor; 19, app del alumno) no permiten cambiar el protocolo
ni el modelo de confianza. Lo que una mejora necesitaría de esos lados se anota aquí y no se
implementa. Cada
propuesta, si se aprueba, va primero a `docs/protocol.md` (y es un cambio de versión según
SemVer).

# Consola del profesor (prompt 18)

## P-01 · Nombre del estudiante en la lista

- **Pide:** Fase 3, punto 2: columnas «Nombre de usuario» y «Nombre del estudiante» por separado.
- **Hoy:** `hello` trae `hostname` y `username` (la cuenta del sistema). No hay nombre de persona:
  la consola muestra la cuenta como nombre.
- **Necesitaría:** un campo nuevo en `hello` (por ejemplo, `display_name`, opcional) que el agente
  saque del nombre completo de la cuenta del sistema o de su configuración. Es compatible hacia
  atrás si es opcional (versión menor).
- **Privacidad:** es un dato personal más que viaja por la red y se ve en la consola; habría que
  agregarlo a `docs/privacidad.md`.

## P-02 · Último sitio web visitado

- **Pide:** Fase 3, punto 2: columna «Último sitio web».
- **Hoy:** el agente no informa navegación. El filtro web (D-017) resuelve DNS en el equipo, pero
  no reporta lo que el alumno visita.
- **Necesitaría:** un mensaje nuevo del agente (por ejemplo, `web.activity`) con el dominio
  actual, y su capacidad.
- **Privacidad:** es monitoreo de actividad. Solo sería aceptable con aviso visible en el equipo
  del alumno mientras se informa (regla de transparencia de CLAUDE.md), sin guardarlo en disco y
  con una regla nueva en `docs/privacidad.md`. Recomendación: decidirlo junto con la política del
  centro antes de diseñarlo.

## P-03 · Borrar los archivos del alumno al terminar la clase

- **Pide:** prompt 10.14: que «Terminar la clase» deje también vacía la carpeta `Documentos\Aula`
  del alumno (archivos recibidos y recolecciones), para que el siguiente grupo no los vea.
- **Hoy:** `class.end` (§5.25) solo quita las tarjetas de la ventana; los archivos se quedan
  (`docs/privacidad.md` regla 84). Decisión del 2026-11-12: no borrar en esta versión.
- **Necesitaría:** un campo opcional en `class.end` (por ejemplo, `clear_work_folder`) y una
  regla del agente que solo borre lo que Aula escribió (con su diario, como las restricciones),
  nunca lo que el alumno creó. Es compatible hacia atrás si es opcional (versión menor).
- **Privacidad y riesgo:** borrar trabajo del alumno es irreversible: pediría confirmación del
  profesor, un aviso en la tarjeta «La clase terminó» y quizá que el administrador lo habilite
  por centro. Recomendación: decidirlo con 10.13 (cuentas por alumno), que separa el trabajo de
  cada grupo.

## P-04 · Encender equipos de otra subred por relevo

- **Pide:** prompt 10.19: que la consola pueda encender equipos aunque esté en otra subred o VLAN
  que el laboratorio, donde su paquete mágico no llega (el broadcast no cruza routers).
- **Hoy:** la consola manda el paquete por su propia red (§5.15.2) y avisa «Otra red» cuando no
  está en la del equipo. Decisión del 2026-11-12: el relevo no entra en esta versión.
- **Necesitaría:** un mensaje `power.wake` (P → A, `{macs: [...]}`) con la capacidad `power.wake`:
  un agente ya encendido de esa subred manda los paquetes por su red. Límites: hasta 32 MAC por
  mensaje y 64 por minuto, solo paquetes mágicos (102 bytes al puerto 9 del broadcast de su
  propia tarjeta), nunca UDP arbitrario. Compatible si es opcional (versión menor).
- **Seguridad y riesgo:** el agente pasaría a mandar tráfico de red por orden de la consola. Es
  poco, pero amplía lo que una consola con la clave del centro puede hacer en la red del alumno.
  También hace falta que quede un equipo encendido en esa subred. Recomendación: esperar a que un
  laboratorio real lo necesite; mientras, que sistemas deje pasar el broadcast dirigido hacia la
  VLAN del laboratorio.

## P-05 · Checklist interactivo del autodiagnóstico

- **Pide:** prompt 10.25, parte B: para lo que el software no puede validar solo (que la bocina
  suene, que el micrófono capte, que la cámara vea, que todas las teclas respondan), una pantalla
  guiada en el equipo del alumno que el técnico recorre marcando «Funciona / No funciona», o
  integrarla en `aula-cliente diagnostico`.
- **Hoy:** el autodiagnóstico automático (§5.27.5, parte A) está hecho; la parte B quedó fuera de
  la entrega del 2026-11-17 por decisión del usuario.
- **Necesitaría:** un mensaje `health.checklist` (P → A) con la capacidad `health.checklist`, una
  pantalla nueva en la app del alumno que **sí toma el foco** (`raise`: el técnico la está usando
  en ese equipo) con su texto de privacidad, y `health.checklist.result` (A → P) con una lista
  cerrada de pruebas y su resultado. La bocina y el micrófono tocarían el SO (reproducir un tono,
  medir el nivel), así que van por `platform` y, como servicio, por el ayudante.
- **Privacidad y riesgo:** el micrófono y la cámara son sensibles: nunca se graba ni viaja audio o
  imagen, solo «captó sonido sí/no»; la pantalla diría siempre qué se prueba y se cierra con un
  clic. Recomendación: empezar por `aula-cliente diagnostico --interactivo` (sin red, sin consola)
  y llevarlo a la consola solo si los técnicos lo piden.

## Notas (no son cambios de protocolo)

- **Columna «Canal»** (Fase 3, punto 2): no se agrega a la lista porque todos los alumnos que ve la
  consola están, por diseño, en su mismo canal. El canal de cada alumno está en su ficha.
- **Vencimiento del certificado** (Fase 3, punto 1): se muestra en Ajustes › Acerca de, sin aviso
  en el inicio, porque hoy no tiene efecto: los alumnos fijan la huella del certificado y no
  revisan sus fechas (`security.ClientTLS`). Dura 10 años. Si algún día se revisan las fechas,
  hará falta un aviso con tiempo y una forma de renovarlo sin reinstalar a los alumnos.
- **«Recoger» y «Recursos»** (Fase 3, punto 6): son funciones nuevas de aula (`code.collect`,
  `file.collect` están reservados en el protocolo); quedan para una etapa propia.

# App del alumno (prompt 19)

Todavía sin propuestas.
