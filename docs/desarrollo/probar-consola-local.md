# Probar la consola en local, sin laboratorio

Para comprobar en la **ventana real** de Aula Consola (WebView2) lo que está en `develop`, sin
VMs ni equipos de alumno: el **aula simulada** pone alumnos de prueba dentro de la propia
consola. Para probar con alumnos reales, ver `instaladores-locales.md` y `docs/prueba-vms.md`.

## Qué usar en cada caso

| Quiero… | Usa | Necesita |
|---|---|---|
| Ver un componente o el sistema de diseño | `go run ./tools/vistaprevia` → `dev/catalogo.html` | Go |
| Ver el prototipo aprobado | `go run ./tools/vistaprevia` → `dev/prototipo/alumnos.html` | Go |
| Iterar en el frontend con recarga automática | `wails dev` en `apps\teacher` | Go y Wails |
| Probar la consola tal como sale, con alumnos simulados | La consola portable (abajo) | Go y Wails |
| Probar con alumnos reales | `instaladores-locales.md` | Además, Inno Setup y VMs |

Las pruebas automáticas (`go test ./...`) ya cubren los bindings y el aula simulada de punta a
punta. Esta guía cubre lo que solo se ve en la ventana: que se vea bien y que cada botón haga lo
que dice.

## Compilar y abrir la consola

En PowerShell, desde `C:\aula`:

```powershell
Push-Location apps\teacher
wails build -clean -ldflags "-X main.version=0.6.0-dev"
Pop-Location
& apps\teacher\build\bin\aula-consola.exe
```

Antes de abrirla:

- **Cierra cualquier Aula Consola instalada.** Las dos usan el puerto 47830, y la segunda dirá
  «Otra copia de Aula Consola ya está abierta». Eso es justo el caso C-01.
- **Firewall:** Windows puede pedir permiso de red. Para el aula simulada puedes cancelar: los
  simulados se conectan dentro del mismo equipo (`127.0.0.1`).
- **Clave del centro:** si el equipo no la tiene, la consola usa una clave personal y muestra la
  franja amarilla. Es lo esperado (caso C-14).
- **Datos de la consola:** guarda canales recientes y nombre en `%AppData%\Aula\consola`. Para no
  tocar los tuyos, ábrela con otro directorio:
  `$env:AULA_CONSOLE_DIR = "$env:TEMP\aula-prueba"; & apps\teacher\build\bin\aula-consola.exe`.

**Regla del proyecto:** estas pruebas las hace una persona. No se automatizan con clics o
teclas sintéticas sobre el escritorio (ver la memoria del proyecto). Las capturas automáticas se
hacen en el arnés web, no en la ventana de Wails.

## Casos

Anota el commit (`git log --oneline -1`) y el resultado de cada caso. Cada entrega que cambie la
consola agrega aquí sus casos.

### Arranque y clase

| # | Pasos | Esperado |
|---|---|---|
| C-01 | Abre la consola con otra ya abierta | «No se pudo abrir la clase. Otra copia de Aula Consola ya está abierta…», con el detalle técnico plegado en «Ver detalles» |
| C-02 | Ciérrala y pulsa «Abrir la clase» | Se abre en Alumnos, vacío, con «¿Falta un alumno?» y «Probar con alumnos simulados» |

### Aula simulada

| # | Pasos | Esperado |
|---|---|---|
| C-03 | En Alumnos, «Probar con alumnos simulados» | Aparecen «Simulado 1» y «Simulado 2» al final, en el grupo «Alumnos simulados», con la franja morada y «0 conectados · 2 simulados» |
| C-04 | En Aula simulada, agrega uno con Linux y solo «recibir código» | Aparece «Simulado 3» en la tabla y en Alumnos |
| C-05 | Alumnos → Miniaturas | Las pantallas dibujadas de los simulados, con la franja roja «Tu pantalla está siendo vista…» y el aviso en línea arriba. Clic en una: se ve en grande |
| C-06 | «Cerrar el aula simulada» y confirma | Desaparecen los simulados y la franja |

### Código, acuses y reenvío

| # | Pasos | Esperado |
|---|---|---|
| C-07 | Con 2 simulados, pon a «Simulado 2» la falla «No confirma lo que recibe». En Código, pega código y pulsa Emitir | En «Últimos envíos»: «1 de 2» y «2 simulados» |
| C-08 | Abre «Quién falta (1)» | Muestra a Simulado 2, «Sin confirmar» |
| C-09 | Quítale la falla y pulsa «Reenviar a los que faltan» | Pasa a «2 de 2» |
| C-10 | En el historial, «Reemitir» del ejercicio 1 | Pide confirmación con el número de alumnos y luego reemite |
| C-11 | Selecciona 1 alumno en Alumnos, vuelve a Código y elige «El alumno seleccionado» | El envío dice «1 de 1» |

### Canal, nombre y ajustes

| # | Pasos | Esperado |
|---|---|---|
| C-12 | Pulsa la tarjeta «1 · Prof. Ayala», escribe 99999 | Error en línea «Escribe un número del 1 al 16000»; no se cierra |
| C-13 | Elige otro canal y confirma | «Ahora estás en el canal N». **El historial de Código sigue ahí**; el aula simulada se cerró |
| C-14 | Ajustes › Acerca de | Versión, clave del centro (o personal, con su explicación), atajo de captura y datos para quien instala con Copiar |
| C-15 | Ajustes › Apariencia: Claro, Oscuro, Sistema | Toda la consola y el editor cambian; al reabrir la consola, recuerda la elección |
| C-16 | Ajustes › Clase › Cambiar… el nombre | Confirma la consecuencia y luego muestra el nombre nuevo en el menú |

### Ventana y teclado

| # | Pasos | Esperado |
|---|---|---|
| C-17 | Encoge la ventana al mínimo (900×600) | El menú queda en iconos y el canal como «1»; los paneles se abren encima; la barra de acciones queda solo con iconos |
| C-18 | Recorre con Tab desde el inicio | El primer Tab ofrece «Saltar al contenido». En el editor, Ctrl+M y luego Tab sale del editor |
| C-19 | En Alumnos, Ctrl+A | Selecciona a todos; el botón cambia a «Quitar selección» |

### Manual y ayuda

| # | Pasos | Esperado |
|---|---|---|
| C-21 | Ayuda → «Solución de problemas» | La página con migas «Ayuda › Solución de problemas»; las capturas y las notas se ven bien en claro y oscuro |
| C-22 | En Ayuda busca «firewal» y luego «simulado» | Resultados con su sección; clic en uno lleva a esa sección |
| C-23 | En Alumnos y en Código, pulsa «?» | Se abre el panel con la página de esa pantalla; «Abrir en el manual» la abre en Ayuda |
| C-24 | «¿Falta un alumno?»: responde las preguntas hasta una causa, luego «Empezar de nuevo» | Cada respuesta queda anotada; al final, la causa y qué hacer; reiniciar vuelve a la primera pregunta |
| C-25 | En Código, «Pegar del portapapeles» con el portapapeles vacío | El aviso «No hay texto copiado» con «Ver en el manual», que abre «El portapapeles no funciona» |

### Ficha, errores de los alumnos e información

| # | Pasos | Esperado |
|---|---|---|
| C-26 | En Alumnos, con simulados abiertos | La lista tiene la columna Sistema (SO y «Aula x.y.z»); al encoger la ventana o abrir un panel, la columna se esconde |
| C-27 | Pulsa el botón «i» al final de la fila de un simulado | Se abre su ficha: usuario, estado, sistema, versión, canal, hora de conexión y «Qué puede hacer su equipo» con Sí/No |
| C-28 | Ajustes › Acerca de: pulsa la «i» junto a «Clave del centro», luego fuera | Aparece la explicación con «Más en el manual»; un clic fuera o Esc la cierra |
| C-29 | Ajustes › Acerca de | «Certificado de la conexión: Vigente hasta el …» (10 años después de instalar) |
| C-30 | Con un alumno real de otra versión o sin una capacidad, mándale lo que no soporta (opcional) | En su fila «No pudo: …»; en Código, «Quién falta» lo lista con el motivo y «Ver en el manual»; «Reenviar» lo reintenta |
| C-44 | Abre la pantalla en grande (Miniaturas) de un simulado y la de un alumno real con Windows | El simulado no tiene «Tomar control» y en «Qué puede hacer su equipo» dice No en «Control remoto»; el real con Windows y una versión que lo incluya tiene el botón |
| C-45 | En la pantalla en grande de un alumno real, «Tomar control»; haz clic en su pantalla y escribe | Se abre su pantalla en grande con «Controlando …» y «Soltar»; arriba de la consola, la franja roja «Controlando …» con «Ver» y «Soltar»; lo que escribes aparece en su equipo |
| C-46 | Con la imagen enfocada pulsa Esc, luego Ctrl+Alt+Fin; cierra la vista con la X | Esc llega al alumno (no cierra la vista); Ctrl+Alt+Fin lleva el foco a «Soltar»; al cerrar, la franja sigue y «Ver» vuelve a abrir la vista |
| C-47 | Con un alumno controlado, toma el control de otro | Aviso «Ya estás controlando otro equipo» con «Ver en el manual»; el primero sigue controlado |
| C-48 | En Miniaturas, haz clic en la pantalla de un alumno real y pulsa «Tomar control» arriba | La misma vista pasa a «Controlando …» con «Soltar»; al soltar, su miniatura sigue actualizándose en la cuadrícula. En un simulado no aparece el botón |
| C-49 | Abre el aula simulada; en la pantalla en grande de «Simulado 1», «Tomar captura» | La vista previa dice «Tomando la captura…» y en un segundo muestra su pantalla dibujada, con la insignia «Simulado» y la nota de que se guarda aparte; un clic en la imagen la ve a tamaño real |
| C-50 | En esa vista previa pulsa «Guardar»; luego «Abrir carpeta» | Dice «Guardada en …\Documentos\Aula\capturas\Simulado 1 (Alumno de prueba) (simulado)\<fecha>\<hora>.jpg»; el Explorador abre esa carpeta y el archivo se ve igual que la vista previa |
| C-51 | Ajustes › Acerca de, «Capturas de pantalla» | Muestra la ruta de `capturas` y «Abrir carpeta» la abre. En un equipo donde nunca se guardó ninguna, dice «Se crea al guardar la primera captura» y el botón está desactivado |
| C-52 | Selecciona dos simulados, panel «Captura» › «Tomar captura de 2 alumnos»; desmarca uno y pulsa «Guardar 1» | Aparece una miniatura por alumno con «Lista»; se guarda solo la marcada, el panel dice «1 captura guardada en …» con «Abrir carpeta» y la otra sigue en la lista hasta «Descartar todas» |
| C-53 | Toma el control de un alumno real y, en la vista del control, pulsa «Tomar captura»; con la vista previa abierta escribe algo; ciérrala | Lo que escribes no llega al alumno; al cerrar vuelves a «Controlando …» (la franja nunca se fue) y, tras un clic en su pantalla, tu teclado vuelve a manejar su equipo |

### Participación: manos y mensajes (10.6)

Con el aula simulada abierta (C-03): cada simulado tiene «Levantar la mano» y «Escribir» en su fila.

| # | Pasos | Esperado |
|---|---|---|
| C-54 | En Aula simulada, «Levantar la mano» en Simulado 2 y luego en Simulado 1 | Aparece abajo a la derecha «Simulado 2 levantó la mano» con «Ver en Participación» (sin el texto); el menú muestra «Participación 2»; en Alumnos, Notas dice «Mano levantada · 1» en Simulado 2 y «· 2» en Simulado 1 |
| C-55 | Abre «Participación» | «2 manos levantadas» en orden de llegada (Simulado 2 primero, con el 1); cada una con «Responder» y «Bajar la mano»; marcadas «Simulado» |
| C-56 | «Escribir» en Simulado 1; en Participación, «Responder» al mensaje, escribe y pulsa Enter | El mensaje aparece arriba de «Mensajes» como «Nuevo»; al enviar: «Respuesta enviada a Simulado 1», el mensaje pasa a «Respondido» con tu respuesta debajo, el contador baja y en Aula simulada Simulado 1 suma un aviso recibido. Esc en el campo cancela sin enviar |
| C-57 | «Bajar la mano» en Simulado 2; luego «Bajar todas» y confirma | Simulado 2 sale de la fila y Simulado 1 pasa a ser el 1; en Aula simulada el botón de Simulado 2 vuelve a «Levantar la mano». «Bajar todas» deja la fila vacía y el contador sin número |
| C-59 | En la pantalla en grande de Simulado 1 › «Mensaje privado» (se abre su diálogo), escribe y pulsa Enter | El diálogo se cierra; «Mensaje privado enviado a Simulado 1»; debajo de su pantalla, «Enviado… · ya lo vio»; en Aula simulada, solo Simulado 1 suma un aviso; Simulado 2 no |
| C-60 | Ve a Código › Últimos envíos | El envío dice «Mensaje privado a Simulado 1», **sin el texto**, con su confirmación |
| C-61 | En la pantalla en grande, abre «Mensaje privado» sin escribir y pulsa Enter; luego Esc | «Escribe el mensaje antes de enviarlo.» junto al campo; Esc cierra el diálogo y el foco vuelve a «Mensaje privado» |
| C-62 | Toma el control de un alumno real; en su vista pulsa «Mensaje privado», escribe y Enter; luego Esc con el campo abierto | Lo que escribes no aparece en su equipo; le llega «Mensaje para ti»; la franja «Controlando…» nunca se fue. Esc cierra el mensaje, no la vista; tras un clic en su pantalla, tu teclado vuelve a manejarla |
| C-58 | Cierra el aula simulada | Participación queda vacía («Nadie ha levantado la mano…»); a 900×600 y en oscuro se lee completa, sin barras horizontales |

### Temporizador de ejercicio (10.8)

Con el aula simulada abierta (C-03) y dos simulados; pon a Simulado 2 la falla «No confirma lo que recibe».

| # | Pasos | Esperado |
|---|---|---|
| C-63 | Sin selección, «Temporizador» en la barra; escribe «Ejercicio 1», pulsa «1 min» e «Iniciar» | Se cierra el panel con «Temporizador iniciado»; arriba aparece la franja azul «Ejercicio 1 · 1:00 · 1 de 2 lo ven», que baja un segundo a la vez en cualquier vista. En Aula simulada los dos muestran el temporizador |
| C-64 | En la franja, «Extender» › «+5 min» | «Se agregaron 5 min»; la franja salta a unos 6 minutos. Participación › Temporizador dice «Queda …», «Para toda la clase» y «Aún no lo confirma: Simulado 2» |
| C-65 | Con el temporizador corriendo, abre otra vez «Temporizador» | El panel avisa «Reemplaza a «Ejercicio 1» (…)». Inicia uno de 1 min con otro nombre: la franja muestra solo el nuevo |
| C-66 | Espera a que llegue a cero | Aparece un aviso «Ejercicio 1 · terminó»; la franja dice «… · terminó» con «Extender» y «Quitar». «Quitar» la quita y Participación vuelve a «No hay un temporizador en marcha». En el log de la consola dice «timer started/ended/stopped» con `timer_id` y duraciones, nunca el nombre |
| C-67 | Marca solo Simulado 1 y abre «Temporizador»; luego a 900×600 y en oscuro | El destino dice «1 alumno seleccionado»; al iniciar, Participación dice «Para 1 equipo elegido». La franja y la sección se leen completas, sin barras horizontales |

### Enviar archivos y plantillas (10.9)

Con el aula simulada abierta (C-03) y dos simulados; pon a Simulado 2 la falla «No confirma lo que recibe». Ten a mano dos archivos pequeños (un `.csv` y un `.py`).

| # | Pasos | Esperado |
|---|---|---|
| C-68 | En el menú, «Archivos»; «Elegir archivos…» y marca los dos; en «Guardar en» escribe `Ejercicio 4`; «Enviar 2 archivos» | El diálogo es el del sistema; la lista muestra los dos con su tamaño. Toast «Enviando Archivos: … a 2 alumnos». En Envíos de archivos: «1 de 2» con «Quién falta (1)»: Simulado 2 «sin confirmar»; Código › Últimos envíos no lo muestra. En `Documentos` de este equipo no aparece nada nuevo (los simulados no escriben) |
| C-69 | «Guardar como plantilla…», nombre `Ejercicio 4`, «Guardar»; luego «Abrir su carpeta» | «Plantilla «Ejercicio 4» guardada»; la plantilla queda elegida y la lista muestra sus archivos sin la «x». El Explorador abre `Documentos\Aula\plantillas\Ejercicio 4` con los dos archivos |
| C-70 | En el Explorador, agrega un tercer archivo a la plantilla; vuelve a la consola y abre otra vez Código | Al elegir la plantilla dice «3 archivos, …». «Enviar 3 archivos» la manda a los dos simulados |
| C-71 | En «Guardar en» escribe `..\Windows` y envía; luego un nombre `con.txt` (renombra un archivo así) | «Un nombre no se puede usar» con «Ver en el manual», y nada sale: Últimos envíos no cambia |
| C-72 | Archivos › «Archivos enviados» › «Reenviar» en la plantilla; luego en la tabla Plantillas, «Usar» y «Abrir carpeta» | Pide confirmación «¿Reenviar Plantilla Ejercicio 4 (3)?»; al aceptar, un envío nuevo en Últimos envíos y la columna «Veces» dice 2. El log de la consola dice «file push» con `push_id`, archivos y bytes, **nunca** nombres ni carpetas |
| C-73 | A 900×600 y en oscuro, la pestaña Archivos con una plantilla elegida; la ficha de un simulado | Todo se lee sin barras horizontales; la ficha dice «Recibir archivos: Sí» |

### Recolecciones (10.10)

Con el aula simulada abierta (C-03) y dos simulados; emite antes un código Python (C-04) para que tengan qué entregar.

| # | Pasos | Esperado |
|---|---|---|
| C-74 | Archivos › pestaña «Recolecciones»; en «Actividad» escribe `Ejercicio 4`, en «Tipos» `*.py`; «Crear carpeta en los equipos» | Mientras escribes, la nota dice «Cada alumno tendrá la carpeta Documentos\Aula\recolecciones\<hoy>\Ejercicio 4…». Toast «Carpeta «Ejercicio 4» creada en 2 equipos». La tarjeta «Ejercicio 4 · <hoy>» lista a los dos con «Carpeta lista» e insignia «Simulado» |
| C-75 | «Recoger» | Los dos pasan por «Recogiendo…» a «Recogido» con «Simulado 1.py (… B)» y la hora; el resumen dice «2 de 2 entregaron · ronda 1». En `Documentos\Aula\recolecciones\<hoy>\Ejercicio 4\` de este equipo hay `Simulado 1 (Alumno de prueba) (simulado)\Simulado 1.py` con el código emitido y `aula-recoleccion.json` |
| C-76 | En la fila de Simulado 1: «Abrir» y «Guardar como…» (elige el Escritorio); arriba, «Carpeta de recolecciones» | El Explorador abre su carpeta; el diálogo es el del sistema y en el Escritorio queda una copia (otra vez: `… (2)`). «Carpeta de recolecciones» abre `Documentos\Aula\recolecciones`. Ajustes › Acerca de muestra esa misma ruta con «Abrir carpeta» |
| C-77 | Pon a Simulado 2 la falla «No confirma lo que recibe»; «Recoger otra vez» y espera 30 s; luego quítale la falla y «Recoger al que falta» | Simulado 2 queda «No se pudo: no respondió a tiempo» y el botón «Recoger al que falta» aparece; al pulsarlo, solo Simulado 2 pasa a «Recogido» (ronda 3) |
| C-78 | Actividad `Ejercicio: 4` y crea; luego cierra la clase y vuelve a abrirla; a 900×600 y en oscuro | «Ese nombre no sirve para la carpeta» con «Ver en el manual», y nada sale. Tras reabrir, la pestaña está vacía, pero la carpeta del paso C-75 sigue en Documentos. Todo se lee sin barras horizontales. El log de la consola dice «collection opened/round saved» con `collect_id` y cantidades, **nunca** la actividad ni nombres |

### Abrir página o app (10.11)

Con el aula simulada abierta (C-03) y, para C-81 y C-82, un alumno real en otra ventana o VM
(`instaladores-locales.md`). Lo nativo (el navegador que se abre y su foco) se mira a ojo.

| # | Pasos | Esperado |
|---|---|---|
| C-79 | Ajustes › **Apps permitidas** | Dice «Son los ejemplos de Aula…» con la ruta donde se guardará y los 6 ejemplos (Bloc de notas … Scratch 3). Agrega «Consola» con `C:\Windows\System32\cmd.exe` y **Guardar**: sale el error «Una app de la lista no es válida…» y nada se guarda. Quítala, cambia el nombre de Paint y **Guardar**: «Lista de apps guardada»; cierra y vuelve a abrir Ajustes: sigue el cambio y dice «Lista de esta consola». **Restaurar ejemplos** y **Guardar** la deja como al principio |
| C-80 | Solo con simulados: Alumnos › **Abrir** | El panel dice «Ninguno de estos equipos puede abrirlo…» y «Los alumnos simulados no abren páginas ni apps»; «Abrir» está desactivado. La ficha de un simulado no tiene «Abrir página o app» |
| C-81 | Con un alumno real: **Abrir** › Página web `docs.python.org/3/tutorial` › **Abrir** | Toast «Se envió a 1 equipo»; la dirección se vacía y en «Últimas que abriste» aparece `https://docs.python.org/3/tutorial` con «Abierta en 1 de 1». En el alumno se abre su navegador en esa página |
| C-82 | Restricciones › Sitios web › bloquear `youtube.com`; luego **Abrir** › escribe `youtube.com` | Debajo de la dirección aparece «El filtro web de la clase bloquea esta página en <equipo>…». Escribe `file:///C:/Windows` y **Abrir**: el error va junto al campo y no se envía nada |
| C-83 | **Abrir** › App permitida › Thonny (o una que no esté instalada en el alumno) › **Abrir**; luego desde la Pantalla grande del alumno, **Abrir página o app** (se abre su diálogo) | Si no la tiene: «Abierta en 0 de 1 · 1 no pudo» y «No se abrió en <equipo>: esa app no está instalada en su equipo». Desde la ficha y la Pantalla grande el mismo formulario se abre debajo del botón, va solo a ese alumno y debajo dice «Se abrió en <equipo>.» o el motivo. A 900×600 y en oscuro el panel y Ajustes se leen completos |

### Encuestas (10.12)

Con el aula simulada abierta (C-03): los simulados responden solos. Para C-87, un alumno real en
otra ventana o VM (`instaladores-locales.md`).

| # | Pasos | Esperado |
|---|---|---|
| C-84 | Alumnos › **Encuesta** › «¿Qué imprime print(3 // 2)?», opción múltiple con `1.5`, `1`, `2` › **Lanzar** | Toast «Encuesta lanzada» con «Ver resultados». Arriba aparece la franja «Encuesta abierta: «¿Qué imprime…»» con cuántos respondieron. En Participación › **Encuestas**, las barras se llenan una por una (rayadas: son simulados) con «n (pct %) · n simulados». Sin escribir la pregunta o con una sola opción, el error va junto al campo y no se lanza nada |
| C-85 | Con la encuesta abierta, **Encuesta** otra vez | El panel avisa «Ya hay una encuesta abierta («…»). Al lanzar esta, aquella se cierra.»; al lanzar, la anterior pasa a «Anteriores de esta clase» |
| C-86 | **Cerrar encuesta** en la franja | La franja desaparece; en Participación la recién cerrada queda desplegada, con sus barras y **Compartir resultados**. Al compartir: insignia «Resultados compartidos» y el botón desaparece. En una de respuesta corta no hay botón y dice que no se comparten |
| C-87 | Con un alumno real: una de **Sí o no** con **Ver quién respondió qué** marcada; responde en el alumno | En Participación aparece «Nominal», «Quién respondió qué: <equipo> · <usuario>: Sí» y «Aún no responden: …» con los que faltan. En una anónima de respuesta corta solo se ven los textos, sin equipo ni hora |
| C-88 | Cierra el aula simulada con una encuesta abierta; luego cierra la clase | Las respuestas simuladas desaparecen de las barras; al cerrar la clase, la sección vuelve a «Aún no has lanzado una encuesta». En la ficha de un alumno, «Responder encuestas» aparece en «Qué puede hacer su equipo». A 900×600 y en oscuro el panel y la sección se leen completos |

### Proyectar la pantalla de un alumno (12.1)

Con dos alumnos reales (VM o la misma máquina con `-directo`, `instaladores-locales.md`), uno como
fuente y otro como receptor, ambos con Windows. El aula simulada abierta sirve para C-93.

| # | Pasos | Esperado |
|---|---|---|
| C-89 | Pantalla grande de la fuente › **Proyectar a la clase** | No se le pregunta nada al alumno: la franja «Proyectando <equipo> · conectando…» y en segundos «Proyectando <equipo> · 1 de 1 viendo», con **Ver** y **Detener**. En la ficha y la Pantalla grande el botón dice **Detener** y su nota dice que solo tú la detienes |
| C-90 | **Ver** en la franja; cierra la vista | La Pantalla grande «Proyectando <equipo>» con el video en vivo; cerrarla no detiene nada (la franja sigue) |
| C-91 | Con tu pantalla transmitiéndose (**Mostrar mi pantalla**), **Proyectar a la clase** a un alumno; luego, con la proyección activa, **Mostrar mi pantalla** › **Empezar** | Pregunta «¿Dejar de mostrar tu pantalla?» (o «¿Dejar de proyectar a <equipo>?»); con **Volver** no cambia nada; al confirmar, una termina antes de que empiece la otra |
| C-92 | **Detener** en la franja y en la Pantalla grande (una vez cada uno) | La proyección termina en todos sin aviso de error; la franja se va |
| C-93 | Con el aula simulada abierta, abre la pantalla en grande y la ficha de un alumno simulado | En la pantalla en grande no aparece **Proyectar a la clase**; en la ficha, «Qué puede hacer su equipo» dice «Mostrar su pantalla a la clase: no» y «Ver la pantalla de un compañero: no». Al proyectar a un real, los simulados no reciben nada. A 900×600 y en oscuro la franja y la Pantalla grande se leen completas |
| C-94 | Proyectando, desconecta la fuente (cierra su agente o su red); luego vuelve a proyectar y cierra la clase | «<equipo> se desconectó»; la vista del receptor se cierra. Al cerrar la clase, la franja se va y el receptor vuelve a su vista de código |

### Notas de interfaz 014–016 (2026-11-12)

| # | Pasos | Esperado |
|---|---|---|
| C-102 | Código: mira el pie del editor; pasa el mouse (y luego Tab) por cada botón | «Enviar a: Todos (n) · Seleccionados (n)»; una sola fila del mismo alto: **Emitir** (principal), **Agregar**, **Limpiar…** (en rojo), **Pegar**. Cada uno explica qué hace en un tooltip, también con el teclado. A 900×600 y en oscuro no se desacomodan |
| C-103 | Ficha de un alumno › la «i» de «Qué puede hacer su equipo»; luego la «i» del canal en Ajustes y la del instalador en «¿Falta un alumno?» | La explicación sale en un globo flotante **encima** de la «i», con su flecha y «Más en el manual», sin mover nada; cerca del borde de arriba se abre abajo, y nunca se sale por los lados. Esc o un clic afuera lo cierran |
| C-104 | Archivos › Plantillas: la papelera de una plantilla; **Volver**; otra vez y **Eliminar**. Repite con un archivo de otra plantilla abierto en Excel | Pide confirmar («Se borra su carpeta… Lo que ya enviaste no cambia»). Con **Volver** no pasa nada; con **Eliminar** desaparece de la tabla y su carpeta de `Documentos\Aula\plantillas`, y el envío sigue en «Archivos enviados». Con un archivo abierto: «No se pudo eliminar la plantilla» con qué hacer |
| C-105 | Archivos: elige dos archivos sueltos; cambia a **Una plantilla**; luego **Usar** en la tabla; luego **Nueva plantilla…**, ponle nombre y **Guardar plantilla** | «¿Qué envías?» cambia entre la zona «Elegir archivos…» y la lista de la plantilla (sus archivos sin quitar). **Usar** elige «Una plantilla» con esa plantilla. El diálogo empieza con los archivos sueltos elegidos; sin nombre o sin archivos dice qué falta; al guardar aparece en la tabla. Sin plantillas, «Una plantilla» está desactivada |
| C-106 | Panel **Temporizador**: escribe una **Descripción** de varias líneas; **Iniciar** con un alumno real | En el alumno, sin quitarle el foco, una tarjeta «Qué hacer: <nombre>» arriba de su lista, con «Nuevo», el tiempo y el texto con sus saltos de línea; con Aula cerrada solo se marca el icono. Al reconectar no vuelve a decir «Nuevo»; al **Detener**, la tarjeta se va. `aula-agente.log` no tiene el texto |
| C-107 | Panel **Temporizador** › Duración: 30, 45, 60; **Otro…**, escribe 90 e **Iniciar**; abre otra vez, **Otro…** y **Volver a las opciones** | Botones del mismo tamaño en una rejilla; el elegido marcado. «Otro…» muestra Minutos con el foco en el campo; «Volver» lo esconde y deja la última opción. El temporizador dura 90 min. A 900×600 y en oscuro cabe |

### Mostrar mi pantalla: a quién y en qué modo (v0.15)

Con dos alumnos reales con Windows (VM o `-directo`) y el aula simulada abierta. Los A-37 a A-41
se miran en la ventana real del alumno: el arnés no ve la ventana nativa (CLAUDE.md, «Cierre»).

| # | Pasos | Esperado |
|---|---|---|
| C-99 | Sin selección, **Mostrar mi pantalla**; luego selecciona solo un alumno real y uno simulado y ábrelo otra vez | Sin selección: «Para todos los equipos conectados (n).» y «La verán los n alumnos que pueden recibirla…». Con selección: «Para 1 equipo seleccionado. 1 alumno simulado no la recibe: es de prueba.» y «La verá 1 alumno…». **Modo** con «En una ventana» (elegido) y «Pantalla completa», cada uno con icono y su explicación debajo |
| C-100 | Con un alumno seleccionado, **Pantalla completa** › **Empezar**; luego desconecta y reconecta su red; conecta otro alumno | La franja roja «Mostrando tu pantalla (Monitor 1) en pantalla completa a 1 de 1 alumnos». Solo el seleccionado la recibe; al reconectarse la recibe otra vez; el que se conecta después no. El monitor y el modo quedan desactivados en el panel mientras transmites |
| C-101 | **Detener** en la franja | La franja se va; en cada alumno la vista se cierra y su ventana vuelve a como estaba |
| A-37 | Con tu editor al frente y Aula compacta, la consola transmite **En una ventana** | Aula sale del modo compacto y al frente, encima de tu editor, con la franja «Estás viendo la pantalla de <profesor> (canal <n>) · Se queda encima hasta que termine…». Puedes moverla y cambiarle el tamaño; **no** tiene botón de minimizar, la X está gris y Alt+F4 no hace nada. Al hacer clic en tu editor, Aula sigue encima. No hay botón de modo compacto |
| A-38 | La consola **Detener** | La vista se cierra y Aula vuelve a la tira compacta en su esquina, sin quedarse encima. Repite con Aula cerrada al área de notificación antes de empezar: al terminar vuelve a ocultarse; con Aula maximizada: sigue maximizada |
| A-39 | La consola transmite en **Pantalla completa** | Aula cubre todo el monitor (también la barra de tareas), sin bordes ni botones de ventana, con la franja arriba. No sale con Alt+F4, Esc, Windows+D ni Windows+M (si se minimiza, vuelve en menos de un segundo). Ctrl+Alt+Supr sí funciona |
| A-40 | En pantalla completa, desconecta la red del alumno (o cierra la consola a la fuerza) | En unos 15 s la vista se quita sola y la ventana vuelve a como estaba; tus programas siguen abiertos y sin cambios. `aula-agente.log` dice «peer silent» o la desconexión y «broadcast ended why=disconnected» |
| A-41 | Repite A-37 y A-39 con el agente como servicio | Lo mismo: lo hace el ayudante en tu sesión. `aula-cliente.log` no tiene errores «window: hold» |

### Notas de interfaz 001–011 e iconos (2026-11-11)

| # | Pasos | Esperado |
|---|---|---|
| C-95 | Abre la ficha de un alumno con «i» teniendo otros seleccionados | Su fila queda marcada con una línea azul a la izquierda (distinta del fondo de la selección); la selección sigue; la ficha dice «<equipo> · <usuario>» y no tiene acciones |
| C-96 | Recorre Alumnos, Código, Archivos, Participación, los paneles de la barra, Ajustes y los diálogos | Todo botón lleva su icono y todo campo su icono a la izquierda; las opciones son botones agrupados con icono; las casillas de formulario son interruptores. En oscuro y a 900×600 se leen bien |
| C-97 | A 900×600, pasa el mouse por los botones de la barra de acciones | La barra muestra solo iconos, sin desbordarse, y cada uno dice su nombre en un tooltip |
| C-98 | Código: escribe código y mira el pie del editor; Archivos: elige dos archivos y escribe una carpeta | «Enviar a» como botones agrupados; Pegar y Limpiar a la izquierda (Limpiar en rojo), Agregar y Emitir (grande) a la derecha. En Archivos, los elegidos en la zona punteada y «Guardar en» con `Documentos\Aula\` dentro del campo |

### Diagramas

| # | Pasos | Esperado |
|---|---|---|
| C-31 | Ayuda › Cómo funciona Aula | Cada diagrama se ve dibujado, con zoom (−, ajustar, +), «Lo que no es obvio» y «Ver el diagrama (texto)» |
| C-32 | Baja a «Todos los diagramas»: elige «Profesor» y escribe «canal» | Quedan los diagramas para el profesor que hablan del canal, con su conteo |
| C-33 | Abre uno: pulsa + y − con el teclado en el dibujo; cambia el tema en Ajustes | Cambia el tamaño; el dibujo pasa a la paleta oscura sin recargar |
| C-34 | En «¿Falta un alumno?», pulsa «?» | El panel muestra la página y, al final, «Diagramas de esta pantalla» |

### Comprobar la conexión

| # | Pasos | Esperado |
|---|---|---|
| C-35 | «¿Falta un alumno?» › **Comprobar la conexión** con la red del equipo como Privada | Los cuatro pasos salen bien (o con aviso) en unos segundos; no aparece ningún alumno nuevo en Alumnos; no se abre ninguna ventana de PowerShell |
| C-36 | Marca la red del equipo como Pública y comprueba de nuevo | El paso 4 falla: «La red … es Pública y el firewall no deja entrar a los alumnos», con «Qué hacer» |
| C-37 | Con el aula simulada abierta, comprueba | Los alumnos simulados no reciben nada y siguen igual; el resultado no cambia |

### Teclado, formularios y carga

| # | Pasos | Esperado |
|---|---|---|
| C-38 | Con «Restricciones» abierto, pulsa Tab hasta un campo del panel y luego Esc | El panel se cierra y el foco vuelve al botón «Restricciones» |
| C-39 | Haz que la consola arranque con error (otro programa ocupa el puerto 47830); en el arranque escribe otro canal y nombre y pulsa «Abrir la clase» | Sale el error y lo que escribiste sigue ahí; si el problema es el canal o el nombre, ese campo queda marcado y con el foco |
| C-40 | En Código, pulsa «Emitir» dos veces seguidas muy rápido | Sale un solo ejercicio en el historial |
| C-41 | Abre Ayuda nada más arrancar la consola | Se ve «Cargando el manual…» y luego el índice |
| C-42 | Ayuda › Tareas frecuentes › Atajos de teclado (desde el índice de la página) | La página baja hasta la tabla de atajos, aunque haya diagramas arriba |
| C-43 | Con otro programa usando Ctrl+Alt+Mayús+C, abre la consola y ve a Ajustes › Acerca de | «Otro programa ya usa Ctrl+Alt+Shift+C. Usa «Pegar del portapapeles» en Código.» |

### App del alumno (prompt 19)

Con la consola local y el agente apuntando a ella (`aula-cliente -canal 1 -clave <clave> -directo
127.0.0.1:47830`), o con `go run ./tools/medir-alumno -ventana` para la parte automática.

| # | Pasos | Esperado |
|---|---|---|
| A-01 | Con tu editor al frente, emite código desde la consola | Sigues escribiendo en tu editor: Aula no se pone encima ni toma el teclado; si está visible, el código aparece arriba con «Nuevo» |
| A-02 | Minimiza Aula y emite código | Aula se queda minimizada; su botón de la barra de tareas parpadea y el título dice «(1) Aula — …» |
| A-03 | Cierra Aula con la X | El agente sigue: el icono de Aula queda en «Mostrar iconos ocultos» y al pasar el puntero dice «Aula — conectado (canal …)» |
| A-04 | Con Aula cerrada, emite código; luego haz clic en el icono | El icono se marca con un punto azul y la ventana no aparece; al hacer clic, vuelve y el punto se quita. Clic derecho: «Abrir Aula» |
| A-05 | Pulsa el botón de compactar (junto al estado) y emite código | La ventana queda como una tira abajo a la derecha, sin quedar encima de las demás; resalta «1 nuevo»; «Volver al tamaño normal» la regresa |
| A-06 | Sin tocar la ventana, pulsa Ctrl+Mayús+C con Aula al frente y pega en el Bloc de notas | Se pega el último código, con su sangría y sus saltos de línea |
| A-07 | En compacto, manda una orden de energía de 10 min y cancélala en seguida | La ventana vuelve a su tamaño y al frente con la banda roja; al cancelar, la banda se va |
| A-08 | Abre «Miniaturas» en la consola con el alumno en la lista | El distintivo nativo arriba de la pantalla y la banda «Tu pantalla la está viendo…» en Aula; al cerrar, ambos se van |
| A-09 | Con Aula compacta o cerrada, toma el control desde la pantalla en grande del alumno en la consola | La ventana vuelve a su tamaño y al frente con la banda roja «… está controlando tu equipo» (sin botón para cerrarla) y la de pantalla vista; el distintivo nativo dice «El profesor está controlando tu equipo» |
| A-10 | Mientras te controlan, mueve tu propio mouse | La banda dice «Su control está en pausa…» y la consola «En pausa»; a los 2 s sin moverte vuelve a «Controlando» |
| A-11 | Pulsa «Soltar» en la consola con el foco en tu editor | La banda y el distintivo se van; en la esquina aparece «… dejó de controlar tu equipo» sin quitarle el foco a tu editor |
| A-12 | Con tu editor al frente, toma una captura desde la pantalla en grande del alumno en la consola | Arriba de la pantalla, 5 s, «● Tu profesor está tomando una captura de tu pantalla» (sale también en la imagen de la consola); en la esquina de Aula, «Captura de pantalla · Prof. … tomó una captura de tu pantalla (HH:MM)», sin quitarle el foco a tu editor; el título dice «(1) Aula — …» |
| A-13 | Cierra Aula y vuelve a tomar una captura; busca archivos nuevos en el equipo del alumno | La ventana no aparece: el icono del área de notificación se marca y al abrirla está el aviso. En `%TEMP%`, Documentos y `C:\ProgramData\Aula` no hay ninguna imagen nueva, y `aula-agente.log` solo dice «screenshot sent» con partes y tamaños |
| A-14 | En el alumno, pulsa «Levantar la mano» (en la tira) y luego «Agregar un motivo», escribe y Enter | El botón queda pulsado («Bajar la mano») y arriba de la lista: «Tu mano está levantada…» con el motivo; en la consola, la mano en Participación con «Motivo: …». La ventana no cambia de tamaño ni de lugar |
| A-15 | «Mensaje al profesor», escribe y pulsa Enter; responde desde la consola con tu editor al frente | El mensaje queda en «Tus mensajes» como «Enviando…» y en seguida «Recibido»; la respuesta aparece en la esquina como «Respuesta a tu mensaje» con «Tú escribiste: «…»», sin quitarle el foco a tu editor. En compacto, el botón de mensaje devuelve la ventana a su tamaño |
| A-17 | Con dos alumnos y tu editor al frente en uno, manda desde la consola un «Mensaje privado» a ese alumno | En su esquina aparece «Mensaje para ti» con el icono del sobre, sin quitarle el foco; el otro alumno no recibe nada. En `aula-agente.log` y en el log de la consola no aparece el texto |
| A-16 | Con la mano levantada, cierra la consola y vuelve a abrirla; luego «Bajar la mano» desde la consola | Al reconectar, la mano vuelve a la fila de Participación sola; al bajarla, el alumno ve «Tu profesor bajó tu mano.» sin que la ventana salga al frente. `aula-agente.log` y el log de la consola dicen «hand raised»/«student message» con IDs y longitudes, nunca el texto |
| A-18 | Con tu editor al frente, lanza desde la consola un temporizador de 1 min | En la tira de Aula aparece «Ejercicio 1 · 1:00», que baja una vez por segundo sin moverse ni parpadear; en el último minuto (todo el minuto, aquí) cambia a color de advertencia. Tu editor conserva el foco. En modo compacto la insignia también se ve |
| A-19 | Deja que llegue a cero con la ventana abierta; luego con la ventana cerrada al área de notificación | Con la ventana abierta: la insignia dice «Ejercicio 1 · terminó» y en la esquina aparece «Se acabó el tiempo» con «Entendido», sin quitar el foco ni traer la ventana. Con la ventana cerrada: no aparece; solo se marca el icono. «Quitar» en la consola quita la insignia |
| A-20 | Con tu editor al frente, envía desde la consola un archivo `datos.csv` a la carpeta `Ejercicio 4` | En la lista de Aula aparece arriba la tarjeta «Archivo recibido: datos.csv · Guardado en Documentos\Aula\Ejercicio 4» con «Nuevo» y «Abrir carpeta»; tu editor conserva el foco; el título dice «(1) Aula — …». El archivo está en `Documentos\Aula\Ejercicio 4\datos.csv` y su propietario es tu usuario (Propiedades › Seguridad › Opciones avanzadas), no SYSTEM |
| A-21 | Envía otra vez el mismo archivo, sin «Reemplazar»; luego con «Reemplazar si ya lo tienen» | La segunda tarjeta dice «Ya tenías datos.csv: el nuevo quedó como datos (2).csv. El tuyo no cambió.»; la tercera, «datos.csv reemplazó al que tenías con ese nombre.» |
| A-22 | «Abrir carpeta»; luego cierra Aula al área de notificación y envía otro archivo | El Explorador abre `Documentos\Aula\Ejercicio 4` (nada más). Con Aula cerrada, la ventana no aparece: solo se marca el icono, y al abrirla está la tarjeta con «Nuevo». `aula-agente.log` dice «file push saved» con tamaños, nunca nombres |
| A-23 | Con tu editor al frente, crea desde la consola la recolección `Ejercicio 4` con `*.py` | En la lista de Aula aparece arriba «Recolección: Ejercicio 4 · Guarda tu ejercicio en Documentos\Aula\recolecciones\<hoy>\Ejercicio 4 · Se recogerán: *.py» con «Nuevo» y «Abrir carpeta»; el editor conserva el foco. «Abrir carpeta» abre esa carpeta (existe y está vacía) |
| A-24 | Guarda ahí `20231234-Ana.py` y un `notas.txt`, y otro `.py` en `Documentos\Aula` (fuera); desde la consola, «Recoger» | Primero el aviso «El profesor solicitó recoger tu ejercicio en Documentos\Aula\recolecciones\…»; luego «Tu profesor recogió tu ejercicio: 20231234-Ana.py (… KB).». La tarjeta dice «Recogido a las …: 1 archivo». Ni `notas.txt` ni el de afuera llegan a la consola; nada cambia en tu carpeta. Ninguno de los avisos quita el foco ni trae la ventana |
| A-26 | Con tu editor al frente, abre desde la consola la página `docs.python.org/3/tutorial/?q=1` | Se abre tu navegador predeterminado en esa página y queda al frente; Aula **no** sale al frente ni deja el modo compacto. En la esquina de Aula: «Tu profesor abrió una página · <profesor> (canal <n>) abrió docs.python.org/3/tutorial en tu navegador (hh:mm)», sin el `?q=1`; al pasar el mouse se ve la dirección completa. «Entendido» lo cierra. Con Aula cerrada al área de notificación, solo se marca el icono y el aviso está al abrirla |
| A-27 | Abre desde la consola la app Calculadora; luego guarda en la consola una app con la ruta `%LOCALAPPDATA%\Temp\x.exe` (sale error) y otra con `C:\Windows\System32\cmd.exe` editando a mano `apps-permitidas.json` y ábrela | La Calculadora se abre y el aviso dice «… abrió Calculadora en tu equipo». La de `cmd.exe` no se abre: la consola dice «su equipo no permite esa app…» y Aula no muestra aviso. `aula-agente.log` (o `aula-cliente.log`) dice «opened» / «open failed reason=not_allowed», **nunca** la URL ni la ruta |
| A-25 | Cierra Aula al área de notificación y recoge otra vez | La ventana no aparece: solo se marca el icono; al abrirla están los avisos y la ronda nueva en la tarjeta. `aula-agente.log` dice «collection read/collected file acked» con tamaños, nunca nombres |
| A-28 | Con tu editor al frente, lanza desde la consola una encuesta de opción múltiple | Arriba de la lista de Aula aparece la tarjeta «Encuesta» con «Nuevo», «Abierta», la pregunta y «Elige una opción. Anónima: tu profesor ve el total, no tu respuesta.»; el editor conserva el foco y el título dice «(1) Aula — …». No aparece ningún diálogo. En modo compacto, la tira dice «1 nuevo»; con la ventana cerrada solo se marca el icono |
| A-29 | Elige una opción con el teclado (Tab hasta las opciones, flechas, Enter); luego otra y **Cambiar mi respuesta** | «Enviando tu respuesta…» y en seguida «Respuesta enviada: 1. Puedes cambiarla mientras siga abierta.»; en la consola cuenta una sola vez, con la última. Al cerrar la encuesta: «Encuesta cerrada. Respondiste: …» y las opciones quedan desactivadas |
| A-31 | Con tu editor al frente y Aula compacta, proyecta este alumno desde la consola | Sin preguntar nada: Aula vuelve a su tamaño y al frente con la banda «Tu pantalla se está mostrando a la clase» (después de la de pantalla vista, **sin botones**) y el distintivo nativo arriba de la pantalla con el mismo texto, **antes** de que el receptor vea la imagen. El título dice «Aula — tu pantalla se está mostrando a la clase…» |
| A-32 | Durante la proyección: Escape con la banda enfocada, cierra Aula al área de notificación, vuelve a compactarla | La banda no se cierra ni se oculta; con Aula cerrada el distintivo sigue; en compacto se ve «Mostrando a la clase». No hay forma de detenerla desde el alumno. En el receptor, «Estás viendo la pantalla de <equipo> (canal <n>)» con el video legible (texto de un editor a 100 %) |
| A-33 | Con la proyección activa, manda desde la consola un **Mensaje privado** a la fuente y abre en ella una encuesta nominal | En la fuente se ven; en el receptor (y en la Pantalla grande de la consola) **no** aparece la ventana de Aula: solo lo que hay debajo. Mide en el Administrador de tareas de la fuente CPU y red de `aula-cliente` (o del ayudante): una sola subida de ≈ 1–3 Mbit/s, sin importar cuántos receptores haya |
| A-34 | En la consola **Detener** | La banda y el distintivo se van en la fuente (sin quitarle el foco a tu editor) y la vista se cierra en el receptor. `aula-agente.log` dice «presentation started/ended» con `present` y tamaños, **nunca** el SDP ni candidatos. Con el agente como servicio: lo mismo, y en el Administrador de tareas la captura la hace el ayudante (usuario del alumno), no el servicio (SYSTEM) |
| A-35 | «Mensaje al profesor» en la tira; escribe y Enter; ábrelo otra vez y Esc | Se abre un diálogo con el campo y su icono; Enter envía y lo cierra; Esc lo cierra y el foco vuelve al botón. En compacto, la ventana vuelve a su tamaño antes de abrirlo |
| A-36 | El botón ⓘ de la tira, con el agente en modo ventana y luego como servicio | «Qué puede hacer Aula en este equipo» con la grilla: verde ✓ lo que este equipo permite (como su ficha en la consola), gris ✗ lo que no; cada icono dice qué es al pasar el mouse |
| A-30 | La consola comparte los resultados; luego revisa `aula-agente.log` (o `aula-cliente.log`) | En la tarjeta: «Resultados de la clase» con una barra por opción, su número y porcentaje, la tuya marcada «(tu respuesta)» y «Respondieron n de m». El log dice «poll arrived», «poll answered», «poll closed» con `poll` y longitudes, **nunca** la pregunta, las opciones ni tu respuesta |
