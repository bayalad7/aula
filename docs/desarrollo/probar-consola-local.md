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
