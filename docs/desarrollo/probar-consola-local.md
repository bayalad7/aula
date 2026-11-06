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
| C-44 | Abre la ficha de un simulado y la de un alumno real con Windows | El simulado no tiene «Tomar control» y en «Qué puede hacer su equipo» dice No en «Control remoto»; el real con Windows y una versión que lo incluya tiene el botón |
| C-45 | En la ficha de un alumno real, «Tomar control»; haz clic en su pantalla y escribe | Se abre su pantalla en grande con «Controlando …» y «Soltar»; arriba de la consola, la franja roja «Controlando …» con «Ver» y «Soltar»; lo que escribes aparece en su equipo |
| C-46 | Con la imagen enfocada pulsa Esc, luego Ctrl+Alt+Fin; cierra la vista con la X | Esc llega al alumno (no cierra la vista); Ctrl+Alt+Fin lleva el foco a «Soltar»; al cerrar, la franja sigue y «Ver» vuelve a abrir la vista |
| C-47 | Con un alumno controlado, toma el control de otro | Aviso «Ya estás controlando otro equipo» con «Ver en el manual»; el primero sigue controlado |
| C-48 | En Miniaturas, haz clic en la pantalla de un alumno real y pulsa «Tomar control» arriba | La misma vista pasa a «Controlando …» con «Soltar»; al soltar, su miniatura sigue actualizándose en la cuadrícula. En un simulado no aparece el botón |
| C-49 | Abre el aula simulada; en la ficha de «Simulado 1», «Tomar captura» | La vista previa dice «Tomando la captura…» y en un segundo muestra su pantalla dibujada, con la insignia «Simulado» y la nota de que se guarda aparte; un clic en la imagen la ve a tamaño real |
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
| C-59 | Abre la ficha de Simulado 1 › «Mensaje privado», escribe y pulsa Enter | «Mensaje privado enviado a Simulado 1»; bajo el botón, «Enviado… · ya lo vio»; en Aula simulada, solo Simulado 1 suma un aviso; Simulado 2 no |
| C-60 | Ve a Código › Últimos envíos | El envío dice «Mensaje privado a Simulado 1», **sin el texto**, con su confirmación |
| C-61 | En la ficha, abre «Mensaje privado» sin escribir y pulsa Enter; luego Esc | «Escribe el mensaje antes de enviarlo.» junto al campo; Esc lo cierra y el foco vuelve a «Mensaje privado» |
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
| A-09 | Con Aula compacta o cerrada, toma el control desde la ficha del alumno en la consola | La ventana vuelve a su tamaño y al frente con la banda roja «… está controlando tu equipo» (sin botón para cerrarla) y la de pantalla vista; el distintivo nativo dice «El profesor está controlando tu equipo» |
| A-10 | Mientras te controlan, mueve tu propio mouse | La banda dice «Su control está en pausa…» y la consola «En pausa»; a los 2 s sin moverte vuelve a «Controlando» |
| A-11 | Pulsa «Soltar» en la consola con el foco en tu editor | La banda y el distintivo se van; en la esquina aparece «… dejó de controlar tu equipo» sin quitarle el foco a tu editor |
| A-12 | Con tu editor al frente, toma una captura desde la ficha del alumno en la consola | Arriba de la pantalla, 5 s, «● Tu profesor está tomando una captura de tu pantalla» (sale también en la imagen de la consola); en la esquina de Aula, «Captura de pantalla · Prof. … tomó una captura de tu pantalla (HH:MM)», sin quitarle el foco a tu editor; el título dice «(1) Aula — …» |
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

### Con alumnos reales (opcional, con `instaladores-locales.md`)

| # | Pasos | Esperado |
|---|---|---|
| C-20 | Con 1 VM de alumno y los simulados abiertos | El real cuenta en «conectados» y los simulados aparte. Energía y Restricciones solo se ofrecen para el real |

## Revisión automática del frontend (arnés web)

Antes de pedir la prueba en la ventana real, corre el arnés de `scripts/ux/` (ver su
`README.md`): recorre 41 pantallas en Chromium headless con datos de demostración, falla si hay
errores de JS o violaciones serias de accesibilidad, y regenera las capturas del manual. No
sustituye estos casos: no ve la ventana nativa ni el backend.

## Si algo falla

- Log de la consola: `%LOCALAPPDATA%\Aula\consola\aula-consola.log`. Las líneas de los simulados
  llevan `simulated=true`.
- Anota el caso, la captura de la ventana (solo de la ventana de Aula) y el commit.
