---
titulo: "Ajustes"
resumen: "Tu nombre, el canal, el tema y los datos de esta consola."
grupo: "Guía por pantalla"
orden: 50
pantallas: []
---

# Ajustes

Ábrelos con **Ajustes**, abajo en el menú.

## Clase

- **Tu nombre**: lo ven los alumnos arriba de su ventana de Aula.
- **Canal**: el número de tu sala. También lo cambias tocando la tarjeta «Clase abierta · canal»
  del menú.
- **Canal al abrir la consola**: el que fijó el administrador al instalar, o el último que
  usaste.

Al cambiar el canal o el nombre, la consola te dice antes qué pasa: los alumnos se reconectan,
se quitan las restricciones y se detiene la transmisión de tu pantalla. **Tu historial de código
se conserva.**

## Apariencia

Elige **Sistema** (sigue a Windows), **Claro** u **Oscuro**. La consola recuerda tu elección.

## Apps permitidas

Las apps que puedes abrir en los equipos con **Abrir página o app** (ver
[Tareas frecuentes](tareas-frecuentes#abrir-una-página-o-una-app-en-los-equipos)). Cada una tiene un
**nombre**, que es lo que ve el alumno, y la **ruta del .exe**. La ruta puede empezar con
`%ProgramFiles%`, `%ProgramFiles(x86)%`, `%LOCALAPPDATA%` o `%SystemRoot%`, y cada equipo la
completa con sus propias carpetas.

Al principio la lista trae ejemplos: Bloc de notas, Calculadora, Paint, Visual Studio Code, Thonny y
Scratch 3. Cámbialos, agrega los programas de tu laboratorio con **Agregar app** y pulsa
**Guardar**. Con **Restaurar ejemplos** vuelves a los de Aula.

- **Dónde se guarda.** Si existe `C:\ProgramData\Aula\centro\apps-permitidas.json`, esa es la
  lista **del centro**: la comparten todas las consolas del equipo y solo un administrador puede
  cambiarla. Si no, se guarda en la carpeta de esta consola. Para compartir la tuya con todo el
  centro, un administrador copia ese archivo a `C:\ProgramData\Aula\centro\`. Los equipos de los
  alumnos no necesitan nada: no hay que instalar la lista en ellos.
- **Lo que nunca se abre, aunque esté en la lista.** Cada equipo de alumno revisa la ruta con sus
  propias reglas, que vienen dentro de Aula:
  - solo abre un `.exe` ya instalado en `Program Files`, `Program Files (x86)`, `Windows` o
    `%LOCALAPPDATA%\Programs`;
  - nunca abre nada en Documentos, Escritorio, Descargas ni la carpeta temporal, ni `cmd`,
    PowerShell u otras herramientas del sistema;
  - siempre la abre sin argumentos.

  Si no cumple, en tu consola sale «su equipo no permite esa app».
- Si un programa está instalado en otra carpeta en algunos equipos, en esos sale «esa app no está
  instalada en su equipo».

## Acerca de

- **Versión** de Aula Consola.
- **Clave del centro**: la prueba de que esta consola es de tu centro. Más en
  [Conceptos](conceptos#la-clave-del-centro).
- **Certificado de la conexión**: lo que cifra la conexión con los alumnos y hasta qué fecha es
  vigente. Los alumnos lo reconocen por su huella, así que la fecha es solo informativa.
- **Atajo para capturar código**: carga en el editor lo que copiaste, desde cualquier programa.
- **Capturas de pantalla**: la carpeta donde quedan las capturas que guardas, con **Abrir carpeta**
  (ver abajo).
- **Datos para quien instala**: canal, equipo, puertos y clave, con un botón para copiarlos. Dáselos
  al responsable de TI si un alumno no aparece.

## Capturas de pantalla

Las capturas de la pantalla de un alumno que guardas (ver
[Tareas frecuentes](tareas-frecuentes#tomar-una-captura-de-la-pantalla-de-un-alumno)) quedan en tu
carpeta **Documentos**, en `Aula\capturas`, con una carpeta por alumno (`LAB3-PC07 (alumno07)`) y
dentro una por día (`2026-11-05`); cada archivo se llama por la hora en que se tomó
(`10-42-06.jpg`). **Ajustes › Acerca de** muestra la ruta y la abre con **Abrir carpeta**.

- La carpeta se crea al guardar la primera captura: antes, el botón está desactivado.
- Si Documentos está en OneDrive o en una unidad de red, las capturas van ahí.
- Están en tu carpeta personal: los alumnos y los otros usuarios del equipo no las ven (un
  administrador de Windows sí podría). Aula no las borra ni las sube a ningún lado: bórralas tú
  cuando ya no las necesites.
- Si al guardar sale «No se pudo guardar la captura», revisa que Documentos exista y tenga
  espacio; la captura sigue en la vista previa para que lo intentes de nuevo.
