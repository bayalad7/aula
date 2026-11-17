# Salud del laboratorio (10.25): ubicación y decisiones

Estado: **implementado, sin commit** (17/11/2026). Prompt `docs/prompts/10.25-salud-del-laboratorio.md`,
con `/funcionalidad-nueva 10.25`. Sin capturas versionadas: las del arnés son
`scripts/ux/salida/221-229` (consola) y `scripts/ux/salida-alumno/90-91` (alumno), y salen en el CI.
El mapa tiene «Consola · Laboratorio», «Consola · Ficha del alumno (salud)» y «Consola · Ajustes ›
Salud»; el manual, `docs/administracion/img/laboratorio.webp`.

## Análisis

Hoy un equipo con problemas se descubre cuando un alumno dice «a mí no me jala». El profesor o el
técnico necesita, **antes de clase**, una vista de todo el laboratorio con lo que va a fallar y qué
hacer, y la misma información de un equipo cuando mira a un alumno.

- **Tiene material propio** (los informes de cada equipo y los resultados del autodiagnóstico): por
  la regla de «Ubicación y vocabulario» de `CLAUDE.md`, le toca un módulo del menú.
- **La Ficha del alumno solo informa:** ahí va «Salud del equipo», sin botones.
- **No es algo que «sigue activo»** durante la clase: no es Franja global.
- **No observa al alumno ni cambia el equipo:** sin banda ni aviso en la app del alumno; solo su
  línea en «Qué puede hacer Aula en este equipo».

## Opciones de ubicación presentadas

1. **Módulo «Laboratorio»** (recomendada, elegida): vista propia con la tabla, **Revisar ahora** y
   **Autodiagnóstico**, más una insignia en la Fila del alumno y en el salón.
2. **Insignia y Cabecera de Alumnos:** «3 equipos con alertas» en la cabecera, que filtra la lista,
   y los dos botones en la Barra de acciones.
3. **Franja global:** «3 equipos con alertas · Ver». Descartada en la recomendación: la franja es
   para lo que sigue activo, no para estados.

En las tres: «Salud del equipo» en la Ficha y los umbrales en Ajustes › Salud.

## Decisiones

- **Grupo «Pruebas» del menú, antes de Aula simulada** (pregunta del usuario): es lo que se revisa
  antes o fuera de clase, como el aula simulada; con **contador** de equipos con alertas para que
  se note sin abrirlo, como el de Participación.
- **Alcance:** el informe periódico, «Revisar ahora» y el autodiagnóstico automático (parte A).
  El checklist interactivo (parte B) toma el foco en el equipo del alumno y toca micrófono y
  cámara: quedó como propuesta P-05.
- **Destinos en Ajustes de la consola** (no en la configuración del agente): la dirección de
  internet y hasta 8 servicios viajan en `health.config`; la consola ya tiene esa confianza.
- **Ritmo:** cada 60 s ±10 % y antes ante un cambio notable (bandas fijas en el agente, ≥ 10 s);
  los umbrales viven solo en la consola.
- **El reloj no viaja:** se calcula con el `ts` de los `heartbeat` (mediana de 5).
- **Sin WMI ni dependencias nuevas:** todo con `x/sys/windows`. El «throttling» se lee como el
  límite de frecuencia que impone Windows (`MhzLimit`), no como temperatura en grados.
- **Tabla compacta:** en la fila solo el nombre corto de cada alerta y valores cortos («82 GB
  libres», «50 % libre»); el texto con qué hacer y todos los datos, al abrir la fila. Con la
  ventana angosta (contenedor < 58 rem) se ocultan Memoria, Procesador, Reloj, Red y Batería: las
  alertas ya dicen lo importante. Batería solo aparece si algún equipo tiene.

## Resultados

- Recorrido de la consola `--solo=221-229` y del alumno `--solo=90-91`: sin errores de JS ni
  violaciones serias de axe; claro, oscuro y 900×600.
- Primera versión de la tabla se desbordaba a 1280 px y a 900×600 (textos largos y la celda de
  alertas como `flex`); se corrigió con los textos cortos, un contenedor para las insignias y las
  columnas extra ocultas en angosto.
- Lectura real en el equipo de desarrollo (solo lectura): disco, S.M.A.R.T., memoria, límite de
  frecuencia, encendido, reinicio pendiente, batería con desgaste, dispositivos y la red Wi‑Fi con
  su señal.
