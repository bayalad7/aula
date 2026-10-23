# 14 · Ejecución segura de entregas en sandbox (Docker)

**Prerrequisitos:** Hito 1 cerrado (08). Idealmente ya existe el canal de recoger código (`code.collect`, de la plantilla 10).
**Duración estimada:** 2–4 sesiones.
**Por qué su propio prompt:** correr código de alumnos es **seguridad crítica**. El aislamiento no es opcional ni negociable, por eso no va por la plantilla genérica.

## Prompt

~~~~markdown
Lee CLAUDE.md, docs/protocol.md y docs/progreso.md. Vamos a ejecutar las entregas de los alumnos
de forma aislada y devolver el resultado de las pruebas a la consola. Trabaja en dos partes y
DETENTE al final de la Parte A.

## Parte A: diseño y modelo de amenaza (detente para que lo apruebe)
- Asume que el código del alumno es NO confiable y potencialmente hostil. Escribe un modelo de
  amenaza breve en docs/seguridad-sandbox.md: qué puede intentar (consumir recursos, acceder a
  red/archivos, escapar del contenedor) y cómo lo contenemos.
- Decide dónde corre el sandbox: en el equipo del profesor o en un servidor aparte. NUNCA con los
  privilegios de la consola. Recomienda y justifica.
- Define el contrato de un "ejercicio": código fuente + conjunto de pruebas + lenguaje + límites,
  y el formato del resultado (pasó/falló por prueba, salida, tiempo).
Espera mi aprobación.

## Parte B: implementación (cuando apruebe)
- Paquete `internal/runner` que ejecuta cada entrega en un contenedor Docker desechable con, como
  mínimo: sin red (`--network none`), sistema de archivos de solo lectura salvo un tmp acotado,
  límites de CPU y memoria, límite de tiempo de pared (timeout con `context`), sin privilegios
  (`--cap-drop ALL`, usuario no root), y borrado del contenedor al terminar.
- Imágenes base mínimas por lenguaje (Python, C++, Java, Go, JS) documentadas y versionadas.
- Nunca ejecutar nada fuera del contenedor; validar y acotar el tamaño del código recibido.
- Integra con la consola: enviar un ejercicio a los alumnos, recoger entregas (reusa
  `code.collect`), correrlas en lote y mostrar una tabla de resultados por alumno.
- Si Docker no está disponible en el equipo, degrada con un mensaje claro; no ejecutes sin sandbox.

## Pruebas
- Prueba con una entrega correcta (todas las pruebas pasan) y una incorrecta.
- Prueba de que un programa que intenta abrir red falla por el aislamiento.
- Prueba de que un bucle infinito se corta por timeout y libera el contenedor.
- Prueba de que superar el límite de memoria no tumba la máquina anfitriona.

## Criterios de aceptación
- Una entrega maliciosa simple (red, bucle infinito, uso excesivo de memoria) queda contenida.
- La tabla de resultados por alumno se muestra en la consola.
- `staticcheck ./...` y `go test ./...` pasan.

Actualiza docs/progreso.md y docs/seguridad-sandbox.md.
~~~~

## Verificación manual
- [ ] Ejecuta una entrega que intente escribir fuera del tmp o abrir internet: debe fallar contenida.
- [ ] Un bucle infinito: ¿se corta por timeout sin dejar contenedores colgados (`docker ps`)?
