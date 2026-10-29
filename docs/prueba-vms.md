# Prueba en VMs: etapas 04, 06, 07, 09, transmisión de la pantalla y agente como servicio

Guía paso a paso para cerrar las tres etapas que siguen marcadas "falta prueba en VMs" en
`docs/progreso.md`:

| Etapa | Qué tiene que pasar en VMs reales | Sección |
|---|---|---|
| **04** Descubrimiento y registro | Levantas el profesor y un alumno **aparece** en el registro. Al cerrarlo (o si su equipo se cuelga) **desaparece**. | §4 |
| **06** Código en el alumno | Emites código desde la consola y el alumno lo **copia y pega intacto** en su editor. | §5 |
| **07** Consola del profesor | Con **dos alumnos** en VMs, emites código desde la consola real y ves **los dos acuses** (2/2). El **atajo** captura el portapapeles y lo deja listo para emitir. | §6 |

Hazlas en orden: la 04 prueba la red, la 06 la app del alumno y la 07 usa ambas. Cada caso
tiene un identificador (04-A, 06-C, 07-F…). Al final hay una tabla para anotar los resultados
(§9). Calcula unos **75 minutos** la primera vez (45 para la 04 y la 06, 30 para la 07).

---

## 1. Qué necesitas

| Dónde | Qué |
|---|---|
| Tu equipo de desarrollo (`C:\aula`) | Go y la CLI de Wails, para compilar (ya los tienes). |
| Hipervisor | VirtualBox, Hyper-V o VMware. |
| **VM del profesor** | Windows 10/11. Para la etapa 07 necesita **WebView2** (la consola es una ventana Wails; ver §2.4). |
| **VMs del alumno** | Windows 10/11 con **WebView2**. Para las etapas 04 y 06 basta **una**; para la 07 hacen falta **dos** (el criterio es ver 2/2 acuses). La segunda puede ser un clon de la primera (cambia su nombre de equipo para distinguir las tarjetas). |
| Opcional en el alumno | Python (`winget install Python.Python.3.12`) para ejecutar lo que pegues (06-D). Sin Python, la comprobación se hace a ojo en el Bloc de notas. |

No instalas Go ni el repositorio en las VMs: copias unos pocos archivos ya compilados.

> **Sin segunda VM de alumno**, la etapa 07 también se puede cerrar con un alumno real
> (`aula-cliente.exe`) y otro de consola (`dos_alumnos.exe -rol alumno`) en la **misma** VM:
> los dos confirman el código. Anótalo en los resultados.

---

## 2. Preparación (una sola vez)

### 2.1 Compilar en tu equipo

En PowerShell, desde `C:\aula`:

```powershell
New-Item -ItemType Directory -Force dist | Out-Null

# Profesor de ejemplo con comandos para emitir código (y alumno de consola para la etapa 04)
$env:CGO_ENABLED = '0'; go build -o dist\dos_alumnos.exe ./examples/dos_alumnos; Remove-Item Env:CGO_ENABLED

# App real del alumno (ventana Wails)
Push-Location apps\student; wails build; Pop-Location
Copy-Item apps\student\build\bin\aula-cliente.exe dist\

# App real de la consola del profesor (ventana Wails, etapa 07)
Push-Location apps\teacher; wails build; Pop-Location
Copy-Item apps\teacher\build\bin\aula-consola.exe dist\

# Código de muestra para enviar
Copy-Item examples\snippets\*.py dist\
```

En `dist\` quedan:

| Archivo | Va a |
|---|---|
| `dos_alumnos.exe` | VM del profesor **y** VMs de alumno (etapas 04 y 06) |
| `aula-cliente.exe` | VMs de alumno (etapas 06 y 07) |
| `aula-consola.exe` | VM del profesor (etapa 07) |
| `prueba_sangria.py`, `saludo.py` | VM del profesor (es lo que se envía) |

`dist/` está en `.gitignore`: los binarios no se suben al repositorio.

### 2.2 Copiar a las VMs

Usa una carpeta compartida del hipervisor, un USB virtual o arrastrar y soltar. Ponlo todo en
una carpeta **sin espacios en la ruta**, por ejemplo `C:\aula-prueba\`. El comando `codigo` del
profesor separa el archivo del título en el primer espacio.

> Si Windows muestra "Windows protegió tu PC" (SmartScreen) al abrir un `.exe`, pulsa
> **Más información → Ejecutar de todas formas**. Los binarios de prueba no están firmados; la
> firma llega con los instaladores (etapa 08).

### 2.3 Red de las VMs

Todas las VMs tienen que estar **en la misma red local**, para que mDNS (multicast) funcione:

| Hipervisor | Configuración del adaptador |
|---|---|
| VirtualBox | **Adaptador puente** (bridged), o todas en la misma **Red interna**. En **NAT** las VMs no se ven entre sí. |
| Hyper-V | Todas en el mismo **conmutador externo** (o el mismo conmutador interno). |
| VMware | **Bridged**, o todas en el mismo **segmento LAN**. |

En cada VM, anota su IP con `ipconfig` (línea "Dirección IPv4").

Marca la red como **Privada** en cada VM Windows. Con red "Pública" el firewall bloquea más de
lo necesario. En PowerShell como administrador:

```powershell
Get-NetConnectionProfile                      # mira NetworkCategory e InterfaceIndex
Set-NetConnectionProfile -InterfaceIndex <n> -NetworkCategory Private
```

### 2.4 WebView2 en las VMs con apps de ventana

`aula-cliente.exe` (alumnos) y `aula-consola.exe` (profesor, etapa 07) dibujan su ventana con
WebView2. Para comprobar si está instalado, abre **Configuración → Aplicaciones** y busca
"Microsoft Edge WebView2 Runtime". Si falta, al abrir la app aparece un aviso para descargarlo.
Acéptalo, o instálalo desde <https://developer.microsoft.com/microsoft-edge/webview2/>
("Evergreen Bootstrapper").

### 2.5 Firewall de la VM del profesor

La primera vez que ejecutes el profesor, Windows pregunta si permites el acceso: marca **Redes
privadas** y pulsa **Permitir acceso**. Si no apareció el aviso, abre los puertos a mano
(PowerShell como administrador):

```powershell
New-NetFirewallRule -DisplayName "Aula consola (TCP 47830)" -Direction Inbound -Protocol TCP -LocalPort 47830 -Action Allow
New-NetFirewallRule -DisplayName "Aula mDNS (UDP 5353)"     -Direction Inbound -Protocol UDP -LocalPort 5353  -Action Allow
```

El aviso de Windows es **por programa**: `aula-consola.exe` (etapa 07) vuelve a preguntar la
primera vez aunque ya hayas permitido `dos_alumnos.exe`. Las reglas por puerto de arriba sirven
para los dos.

Las VMs de alumno no necesitan reglas: solo hacen conexiones salientes.

> `ping` entre VMs Windows suele fallar aunque la red esté bien, porque el firewall bloquea ICMP
> por defecto. Para comprobar la conectividad usa la prueba de §3.2.

---

## 3. Arrancar el profesor de ejemplo (etapas 04 y 06)

> En la etapa 07 el profesor es la app real `aula-consola.exe` (§6.1). Usa **la misma clave**
> que `dos_alumnos.exe` (las dos la guardan en `%AppData%\Aula\consola\`), así que los alumnos
> ya configurados siguen sirviendo.

### 3.1 En la VM del profesor

Abre PowerShell en `C:\aula-prueba\` y ejecuta:

```powershell
.\dos_alumnos.exe -rol profesor
```

Debes ver:

```
No había clave: se creó una nueva en C:\Users\...\AppData\Roaming\Aula\consola     ← solo la 1.ª vez
Consola "Consola de ejemplo" en el canal 1, puerto 47830 (TLS). Ctrl+C para salir.
Clave pública para los alumnos: aula-ed25519:I5rPjgu2TFt5s5eDGz7sJst4FfG9v+bZIOwFmYuFvf0=
Huella: 7E65-1323-287C-FE59
Escribe "ayuda" para ver cómo enviar código a los alumnos.
[10:00:00] Registro 1: 0 equipo(s)
```

**Copia la clave pública completa** (desde `aula-ed25519:` hasta el `=` final), por ejemplo en
un `clave.txt` dentro de la carpeta compartida. No es secreta: es la parte pública, la que los
alumnos usan para reconocer a esta consola. La clave **privada** se queda en la VM del profesor
y nunca se muestra.

Deja esta ventana abierta durante toda la prueba: ahí ves el registro y escribes los comandos.

### 3.2 Comprobar la red desde el alumno

En la VM del alumno, con el profesor ya corriendo:

```powershell
Test-NetConnection <IP del profesor> -Port 47830
```

Debe decir `TcpTestSucceeded : True`. Si dice `False`, revisa §2.3 y §2.5 antes de seguir.

---

## 4. Etapa 04: el alumno aparece y desaparece

Aquí el alumno es `dos_alumnos.exe -rol alumno`, un alumno de consola sin ventana: así se ve el
protocolo sin la interfaz. En la VM del alumno, abre PowerShell en `C:\aula-prueba\`. Para no
pegar la clave cada vez, guárdala en una variable:

```powershell
$clave = "aula-ed25519:...la clave del profesor..."
```

### 04-A Aparece en el registro (con mDNS)

```powershell
.\dos_alumnos.exe -rol alumno -clave $clave
```

| Dónde | Esperado (en menos de 5 s) |
|---|---|
| Alumno | `PC-ALUMNO: buscando consola en 1` y luego `PC-ALUMNO: conectado (Consola de ejemplo)` |
| Profesor | `Registro 1: 1 equipo(s)` con una línea `PC-ALUMNO  <usuario>  windows  conectado  <IP>:<puerto>` |

El nombre del equipo y el usuario son los reales de la VM del alumno.

### 04-B Desaparece al cerrarlo

En el alumno, pulsa `Ctrl+C`.

- **Profesor:** en menos de 1 s, la línea del equipo pasa a `desconectado`. No se borra: queda
  para que el profesor sepa que estuvo.
- Vuelve a lanzar el alumno (04-A). Esperado: **una sola** línea para ese equipo, otra vez
  `conectado`. La sesión vieja se reemplaza; no se duplica.

### 04-C Equipo colgado: timeout de 15 s

Con el alumno conectado, **pausa la VM del alumno** desde el hipervisor (VirtualBox: Máquina →
Pausar; Hyper-V: Pausar; VMware: Suspender), sin cerrar el programa.

- **Profesor:** la línea **sigue** `conectado` unos segundos y pasa a `desconectado` a los
  **15 s aprox.** (entre 15 y 18 s). Cronométralo.
- **Reanuda la VM.** En el alumno aparece `desconectado` y luego `conectado` (a los pocos
  segundos, nunca más de 30 s). En el profesor el equipo vuelve a `conectado`.

### 04-D La consola se reinicia

Con el alumno conectado, en el profesor pulsa `Ctrl+C` y vuelve a lanzar
`.\dos_alumnos.exe -rol profesor`.

- **Profesor:** imprime **la misma clave pública** que antes (la clave persiste).
- **Alumno:** pasa a `desconectado` y luego a `conectado` solo, sin tocar nada (en menos de 30 s).

### 04-E Otro canal no se ve

Cierra el alumno y lánzalo con otro canal:

```powershell
.\dos_alumnos.exe -rol alumno -clave $clave -canal 2
```

- **Alumno:** se queda en `buscando consola en 2` y **nunca** se conecta.
- **Profesor:** no aparece ningún equipo nuevo.

Con dos VMs de alumno puedes probar a la vez una en `1` (aparece) y otra en `2`
(no aparece).

### 04-F Clave equivocada (seguridad)

Lanza el alumno con una clave inventada, con el mismo formato y longitud pero otros caracteres:

```powershell
.\dos_alumnos.exe -rol alumno -clave aula-ed25519:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA= -v
```

- **Alumno:** nunca dice `conectado`. Con `-v` aparece
  `console rejected … reason="auth.proof signature does not verify with the configured key"`.
- **Profesor:** no aparece en el registro. En sus logs sale
  `agent reported an error during handshake … code=unauthorized`.

Esto comprueba que un alumno solo obedece a la consola cuya clave tiene configurada.

### 04-G Conexión directa (sin mDNS)

Solo es obligatorio si 04-A falló porque la red bloquea multicast; si no, es opcional.

```powershell
.\dos_alumnos.exe -rol alumno -clave $clave -directo <IP del profesor>:47830
```

Esperado: igual que 04-A, pero el alumno dice `conectado (<IP>:47830)` en lugar del nombre de la
consola. La conexión sigue cifrada y autenticada.

> Si 04-A no funciona pero 04-G sí, la aplicación está bien y el problema es la red de las VMs
> (modo NAT, conmutador distinto, multicast bloqueado). Anótalo en los resultados.

---

## 5. Etapa 06: emitir código y pegarlo intacto

Aquí el alumno es la **app real** `aula-cliente.exe`. Cierra cualquier `dos_alumnos.exe -rol
alumno` que siga abierto en la VM del alumno. El profesor sigue siendo `dos_alumnos.exe -rol
profesor` (§3).

### 06-A Sin configurar

Haz doble clic en `aula-cliente.exe`, sin argumentos.

- **Esperado:** se abre la ventana y arriba dice **"Aula — sin configurar"** (punto rojo), con
  una línea que explica que falta el canal o la clave. No se conecta a nada.
- Ciérrala.

### 06-B Se conecta e identifica la clase

En PowerShell, en `C:\aula-prueba\` de la VM del alumno:

```powershell
.\aula-cliente.exe -canal 1 -clave $clave
```

Si mDNS no funcionó en 04-A, agrega `-directo <IP del profesor>:47830`.

| Dónde | Esperado |
|---|---|
| Alumno (ventana) | Barra superior con punto verde: **"Aula — conectado a la clase de Consola de ejemplo (canal 1)"**. El mismo texto aparece en el título de la ventana y en la barra de tareas. En el centro: "Cuando el profesor comparta código, aparecerá aquí listo para copiar." |
| Profesor | El equipo aparece `conectado` en el registro. |

> Para no escribir los argumentos cada vez, crea `%AppData%\Aula\alumno\config.json` en la VM
> del alumno (pega la ruta en la barra del Explorador):
>
> ```json
> { "channel": "1", "console_public_key": "aula-ed25519:..." }
> ```
>
> Con ese archivo basta con doble clic en `aula-cliente.exe`.

### 06-C Recibir código (la ventana pasa al frente)

1. En el alumno, **minimiza** la ventana de Aula y abre otra cosa encima (el Bloc de notas).
2. En el profesor escribe (con la ruta real de tu carpeta):

   ```
   codigo C:\aula-prueba\prueba_sangria.py Ejercicio 1 - Sangria
   ```

| Dónde | Esperado |
|---|---|
| Profesor | `code.share enviado a 1 equipo(s)` y, al momento, el registro muestra `código xxxxxxxx ✓` junto al equipo. Ese ✓ es el `code.ack`: el alumno confirmó que lo **mostró**. |
| Alumno | La ventana de Aula **se restaura y pasa al frente**. Aparece una tarjeta titulada "Ejercicio 1 - Sangria", con `python` a la derecha, el botón azul **Copiar al portapapeles** y el código con colores (palabras clave, cadenas, comentarios). |

Revisa a ojo en la tarjeta que la sangría se vea escalonada (4, 8, 12 y 16 espacios) y que los
caracteres de la línea 2 (`ñ á é ü < > & "comillas"`) se vean tal cual.

### 06-D Copiar y pegar intacto (criterio de aceptación)

1. En la ventana de Aula pulsa **Copiar al portapapeles**. El botón se pone verde y dice
   **"Copiado ✓"** durante 2 s.
2. Pega (`Ctrl+V`) en **al menos dos** editores de la VM del alumno:

| Editor | Qué comprobar |
|---|---|
| **Bloc de notas** | 32 líneas, cada una en su renglón (ninguna pegada a la anterior), sangría escalonada, acentos y `ñ` correctos. |
| **VS Code** o **IDLE** (si están) | Lo mismo. En VS Code, *Ver → Representar espacio en blanco* muestra los puntos de cada espacio: deben ser 4, 8, 12 o 16, sin tabuladores. |

3. Si la VM tiene Python, guarda lo pegado como `C:\aula-prueba\pegado.py` (en el Bloc de notas,
   *Codificación: UTF-8*) y ejecútalo:

   ```powershell
   python C:\aula-prueba\pegado.py
   ```

   Salida esperada, exacta:

   ```
   pares: [2, 4, 6, 8, 10]
   impares: [1, 3, 5, 7, 9]
   total: 7
   Si ves estas 4 líneas, la sangría llegó intacta.
   ```

   Un `IndentationError` o `TabError` significaría que la copia alteró la sangría: anótalo como
   fallo, con una captura de lo pegado.

### 06-E Agregar un segundo bloque

En el profesor:

```
agregar C:\aula-prueba\saludo.py Ejercicio 2 - Saludo
```

- **Alumno:** aparece una **segunda** tarjeta debajo de la primera, que sigue ahí. La ventana se
  desplaza hasta la nueva.
- **Copiar** en la tarjeta 2 copia **solo** `saludo.py` (3 líneas). Pégalo en el Bloc de notas y
  compruébalo.
- **Profesor:** el registro muestra el `código xxxxxxxx ✓` del nuevo bloque.

### 06-F Reemplazar

En el profesor:

```
codigo C:\aula-prueba\saludo.py Solo este
```

- **Alumno:** queda **una sola** tarjeta ("Solo este"); las anteriores desaparecen.

### 06-G Limpiar

En el profesor: `limpiar`

- **Alumno:** desaparecen todas las tarjetas y vuelve el texto "Cuando el profesor comparta
  código…".
- **Profesor:** el registro muestra `pantalla limpia ✓`.

### 06-H Avisos

En el profesor:

```
aviso Abran el archivo del ejercicio 2
alerta Guarden su trabajo: en 5 minutos cerramos.
```

- **Alumno:** la ventana pasa al frente y aparecen dos avisos arriba, bajo la barra de
  identidad. El primero va en azul (informativo) y el segundo en amarillo (advertencia), cada uno
  con un botón **Entendido**.
- **Profesor:** `msg.show enviado a 1 equipo(s)` dos veces.
- Pulsa **Entendido** en uno: desaparece solo ese.

### 06-I Cerrar la ventana no deja nada oculto (privacidad)

Cierra la ventana de Aula con la **X**.

- **Profesor:** el equipo pasa a `desconectado` al instante.
- **Alumno:** abre el **Administrador de tareas** (`Ctrl+Shift+Esc`): **no** debe quedar ningún
  proceso `aula-cliente.exe`. El agente no sigue corriendo en segundo plano.

### 06-J Clave equivocada en la app real

```powershell
.\aula-cliente.exe -canal 1 -clave aula-ed25519:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=
```

- **Alumno:** la barra dice **"Aula — buscando la clase del canal 1"** y nunca pasa a
  conectado. En unos segundos aparece debajo el motivo: **"Se encontró la consola «…», pero su
  clave no es la de este centro"**.
- **Profesor:** no aparece en el registro; si le envías `codigo …`, dice `enviado a 0 equipo(s)`.

---

## 6. Etapa 07: la consola del profesor

Aquí el profesor es la **app real** `aula-consola.exe` y los alumnos son `aula-cliente.exe` en
**dos** VMs, ya configurados como en 06-B (canal `1` y la clave del profesor).

Antes de empezar, en la VM del profesor **cierra `dos_alumnos.exe`** (`Ctrl+C`). Las dos usan
el puerto 47830 y no pueden correr a la vez. En las VMs de alumno, cierra cualquier
`dos_alumnos.exe -rol alumno`.

### 6.1 Abrir la consola

**07-A La consola arranca en su canal.** En la VM del profesor, haz doble clic en
`aula-consola.exe`. Si Windows pregunta por el firewall, permite **Redes privadas** (§2.5).

La consola **no pide canal**: arranca directo en el predeterminado. Sin instalador no hay
configuración de máquina, así que usa el último canal abierto o, la primera vez, el `1`.

| Esperado |
|---|
| Barra superior: **Canal [ 1 ] Cambiar**, **Tu nombre** (tu nombre de Windows) con **Guardar**, y la huella **"Clave personal: XXXX-XXXX-XXXX-XXXX"**. |
| Debajo, una franja amarilla: este equipo no tiene la clave del centro. Con los portables es lo **esperado**: la consola usa la clave personal del usuario, la misma que imprimía `dos_alumnos.exe` en §3.1. Si la huella es otra (por ejemplo, porque abriste la consola con otro usuario de Windows), ejecuta `.\aula-consola.exe pubkey` y actualiza la clave de los alumnos (06-B). |
| Izquierda: "Alumnos 0 conectados" y "Esperando alumnos en este canal…". Centro: editor vacío y los botones de emisión **deshabilitados**, con el motivo "No hay alumnos conectados." Abajo a la derecha: el botón azul **Capturar portapapeles · Ctrl+Alt+Shift+C**. |

Cambia el nombre a, por ejemplo, `Prof. Ayala — Programación I` y pulsa **Guardar**.

Si encima del botón azul aparece un recuadro amarillo, el atajo global no se pudo registrar
(normalmente porque otra aplicación ya usa la combinación). Anótalo; el botón funciona igual.

**07-B Alumnos en vivo.** Abre `aula-cliente.exe` en las **dos** VMs de alumno.

| Esperado |
|---|
| En la consola aparece **una tarjeta por alumno**, en menos de 5 s, con punto verde, nombre del equipo, "conectado", usuario, `windows · … · agente 0.1.0` y las etiquetas **código** y **avisos**. El contador dice "2 conectados". |
| **Limpiar pantallas** se habilita (ya hay a quién enviarlo). **Emitir a todos** y **Agregar bloque** siguen deshabilitados mientras el editor esté vacío; el motivo cambia a "Escribe o captura código para emitir." **Enviar aviso** se habilita en cuanto escribas un texto. |

Ahora **cierra la ventana** de uno de los alumnos: su tarjeta se atenúa, el punto se pone rojo y
dice **desconectado** (en menos de 1 s); el contador baja a 1. Vuelve a abrirlo: la tarjeta pasa
a **conectado** sin duplicarse.

### 6.2 Captura rápida con el atajo (criterio de aceptación)

**07-F Atajo global.** Prueba el atajo **mientras otra aplicación está al frente**, como cuando
presentas.

1. En la VM del profesor, abre `C:\aula-prueba\prueba_sangria.py` con el **Bloc de notas**
   (clic derecho → *Abrir con* → *Bloc de notas*). También sirve PowerPoint o VS Code.
2. Selecciona todo (`Ctrl+A`) y copia (`Ctrl+C`).
3. **Sin cambiar de ventana**, con el Bloc de notas al frente, pulsa **Ctrl+Alt+Shift+C**
   (usa el **Ctrl izquierdo**; ver la nota del hipervisor más abajo).
4. Cambia a la ventana de la consola.

| Esperado |
|---|
| El editor de la consola tiene **el contenido completo** de `prueba_sangria.py` (32 líneas), con la sangría marcada con puntos grises. |
| El selector de lenguaje dice **Python** (lo propuso la consola; puedes cambiarlo). |
| Abajo apareció el aviso verde "Capturado del portapapeles (32 líneas, python). Revisa el lenguaje y pulsa Emitir." (dura 5 s). |
| **No se emitió nada:** las ventanas de los alumnos no cambiaron. Capturar nunca envía; hay que pulsar un botón de emisión. |

> **El hipervisor puede quedarse el atajo.** VirtualBox usa el **Ctrl derecho** como "tecla
> anfitriona": pulsa el atajo con el **izquierdo**. VMware libera el ratón con `Ctrl+Alt` si la
> VM no tiene el foco del teclado: haz clic dentro de la VM antes. En Hyper-V, haz clic dentro
> de la ventana de la VM para que reciba el teclado. Si ni así llega, anota el hipervisor y usa
> el botón (07-G); el atajo se puede verificar luego en un equipo físico.

**07-G Botón flotante.**

1. En el Bloc de notas, abre `C:\aula-prueba\saludo.py`, selecciona todo y copia.
2. En la consola, pulsa **Capturar portapapeles** (abajo a la derecha).

| Esperado |
|---|
| El editor tiene las 3 líneas de `saludo.py`, lenguaje **Python**, y el mismo aviso verde. Nada se emite. |
| Copia una selección vacía (o un espacio) y vuelve a pulsar el botón: aviso **rojo** "el portapapeles está vacío: copia el código primero (Ctrl+C)". El editor no cambia. |

### 6.3 Emitir y ver los dos acuses (criterio de aceptación)

**07-C Emitir a todos.**

1. Vuelve a cargar `prueba_sangria.py` en el editor (repite 07-F, o pega con `Ctrl+V` dentro del
   editor).
2. En **Título** escribe `Sangría`. Deja el lenguaje en **Python**.
3. Pulsa **Emitir a todos**.

| Dónde | Esperado (en 1–2 s) |
|---|---|
| Consola, bajo los botones | "Ejercicio 1 · Sangría — enviado a 2 · confirmados **2/2 ✓**", en **verde**. Primero puede verse "0/2" o "1/2" un instante. |
| Consola, tarjetas | Cada alumno muestra "✓ código recibido HH:MM:SS". |
| Consola, historial (derecha) | "Ejercicio 1 · Sangría", "python · 32 líneas", con el botón **Reemitir**. En **Confirmaciones**, la misma línea 2/2. |
| Alumnos | La ventana pasa al frente con la tarjeta "Ejercicio 1" + "Sangría". Pulsa **Copiar al portapapeles** en uno y pega en el Bloc de notas: el código queda idéntico (como en 06-D). |

Si ves **"1/2"** y no sube, el alumno que falta no está confirmando: mira si su tarjeta dice
"✓ código recibido" y si en su ventana apareció el código. Anota cuál falló.

**07-D Agregar bloque.**

1. Carga `saludo.py` en el editor (07-G) y escribe el título `Saludo`.
2. Pulsa **Agregar bloque**.

| Esperado |
|---|
| Consola: "Ejercicio 2 · Saludo — enviado a 2 · confirmados 2/2 ✓". El historial muestra **Ejercicio 2** arriba y **Ejercicio 1** debajo. |
| Alumnos: **dos** tarjetas (Ejercicio 1 y Ejercicio 2), la nueva debajo. |

**07-E Reemitir.** En el historial, pulsa **Reemitir** en **Ejercicio 1**.

| Esperado |
|---|
| Consola: "Ejercicio 1 · Sangría (reemitido) — enviado a 2 · confirmados 2/2 ✓". En el historial, Ejercicio 1 dice "**emitido 2 veces**"; no aparece un Ejercicio 3. |
| Alumnos: vuelven a ver **solo** el Ejercicio 1 (reemitir reemplaza la pantalla). |

Haz **doble clic** en una entrada del historial: su código, título y lenguaje se cargan de nuevo
en el editor, sin emitir.

### 6.4 Limpiar, avisos y casos límite

**07-H Limpiar y avisos.**

1. Pulsa **Limpiar pantallas** y confirma.
2. En **Aviso a todos** escribe `Guarden su trabajo: en 5 minutos cerramos.`, elige
   **Advertencia** y pulsa **Enviar aviso**.

| Esperado |
|---|
| Tras limpiar: "Limpiar pantallas — enviado a 2 · confirmados 2/2 ✓"; las tarjetas dicen "✓ pantalla limpia"; los alumnos ya no tienen código. |
| Tras el aviso: "Aviso: Guarden su trabajo… — confirmados 2/2 ✓"; en los alumnos aparece el aviso **amarillo** con **Entendido**. El campo de aviso se vacía. |

**07-I Sin alumnos.** Cierra las ventanas de los dos alumnos.

| Esperado |
|---|
| Las dos tarjetas pasan a "desconectado" y el contador a 0. **Emitir**, **Agregar**, **Limpiar** y **Enviar aviso** se deshabilitan con el motivo "No hay alumnos conectados." |

Vuelve a abrir los dos alumnos antes del siguiente caso.

**07-J Cambiar de canal.**

1. En la barra superior escribe `2` en **Canal** y pulsa **Cambiar**.
2. Escribe `0` y pulsa **Cambiar**; después `16001`.

| Esperado |
|---|
| Con `2`: los alumnos (configurados en el canal `1`) desaparecen de la lista y muestran "Aula — sin conexión…" y luego "buscando la clase del canal 1", con el motivo "No se encontró ninguna consola del canal 1…". El historial empieza vacío. |
| Con `0` y `16001`: "el canal debe ser un número del 1 al 16000" y la clase sigue en el canal 2. |
| Vuelve a `1`: los alumnos reconectan solos en menos de 30 s. Cierra y abre la consola: arranca en el `1` (último canal abierto). |

---

## 7. Comandos de referencia

**Profesor** (`dos_alumnos.exe -rol profesor`, escritos en su ventana mientras corre):

| Comando | Efecto en el alumno |
|---|---|
| `codigo <archivo> [título]` | Muestra el archivo y reemplaza lo que hubiera. |
| `agregar <archivo> [título]` | Lo agrega debajo de lo que ya hay (máximo 20 bloques). |
| `limpiar` | Borra todos los bloques. |
| `aviso <texto>` | Aviso informativo (azul). |
| `alerta <texto>` | Aviso de advertencia (amarillo). |
| `ayuda` | Lista los comandos. |

El lenguaje se deduce de la extensión: `.py`, `.go`, `.js`, `.ts`, `.java`, `.c`, `.cpp`, `.cs`,
`.sql`, `.html`, `.css`, `.sh`, `.json`, `.rs`, `.kt`. Cualquier otra se muestra sin colores.

**Opciones** de `dos_alumnos.exe`: `-canal 1`, `-puerto 47830`,
`-nombre "Consola de ejemplo"`, `-v` (logs detallados). Las de `aula-cliente.exe` son `-canal`,
`-clave`, `-directo IP:puerto` y `-config archivo.json`.

**Consola real** (`aula-consola.exe`, etapa 07): se usa con la ventana y no tiene opciones de
arranque; arranca en su canal predeterminado y escucha en el puerto 47830 (los dos se cambian en
`C:\ProgramData\Aula\consola\config.json`). Desde una terminal acepta: `pubkey` (muestra la
clave que usa: la del centro o la personal), `keygen` (crea la personal si no existe),
`centro crear|importar|exportar|publica` (clave del centro, como administrador; ver
`docs/configuracion.md`) y `version`. El atajo de captura es **Ctrl+Alt+Shift+C**.

**Agente real** (`aula-cliente.exe`): además de las opciones de arriba, `aula-cliente.exe
diagnostico` imprime la configuración usada, las consolas del canal que ve y un intento de
conexión.

---

## 8. Si algo falla

| Síntoma | Qué probar |
|---|---|
| `Test-NetConnection` da `False` | Firewall del profesor (§2.5), red "Pública" (§2.3) o VMs en redes distintas (§2.3). |
| El alumno se queda en "buscando" pero `Test-NetConnection` da `True` | mDNS bloqueado por el modo de red. Usa `-directo <IP del profesor>:47830` (04-G) y anótalo. |
| "console rejected … signature does not verify" | La clave no es la de esa consola. Cópiala de nuevo de la salida del profesor (§3.1); si regeneraste la clave del profesor, actualízala en los alumnos. |
| `aula-cliente.exe` no abre ventana o pide WebView2 | Instala WebView2 (§2.4). |
| SmartScreen bloquea el `.exe` | *Más información → Ejecutar de todas formas* (§2.2). |
| "Copiar" dice **No se pudo copiar** | Pasa el ratón sobre el botón para ver el motivo. En una sesión remota (RDP), activa el portapapeles compartido de la conexión. |
| El código no llega pero el alumno está `conectado` | Revisa la ruta del comando `codigo` (sin espacios) y que el profesor diga `enviado a 1 equipo(s)`. |
| Se pegan las líneas juntas en un editor | Anota qué editor y su versión: es justo lo que esta prueba debe detectar. |
| Puerto ocupado | `-puerto 47900` en el profesor (y en `-directo` si lo usas). En la etapa 07, "el puerto 47830 está ocupado (¿hay otra consola abierta?)" al abrir la clase significa que `dos_alumnos.exe` u otra `aula-consola.exe` sigue abierta: ciérrala. |
| La consola dice "confirmados 1/2" y no sube | Uno de los alumnos no confirma: mira qué tarjeta no tiene "✓ código recibido" y si ese alumno vio el código. Si es `dos_alumnos.exe -rol alumno`, usa una versión compilada después del 28/10/2026 (antes no confirmaba). |
| El atajo `Ctrl+Alt+Shift+C` no hace nada | Mira el recuadro amarillo sobre el botón Capturar (otra aplicación usa la combinación). En una VM, el hipervisor puede quedarse las teclas (nota de 07-F): usa el Ctrl izquierdo o el botón. |
| Los botones de emisión están grises | Lee el motivo junto a ellos: no hay alumnos conectados, ninguno recibe código o el editor está vacío. |
| Los alumnos no se conectan a la consola real pero sí a `dos_alumnos` | La ventana del alumno dice el motivo bajo "buscando…". Si es "su clave no es la de este centro", compara la huella de la barra superior (07-A) con la clave de los alumnos (`aula-consola pubkey`). |

Para ver logs detallados, agrega `-v` en `dos_alumnos.exe`. `aula-cliente.exe` es una app de
ventana: su log queda en `%LOCALAPPDATA%\Aula\alumno\aula-cliente.log`, y
`.\aula-cliente.exe diagnostico` imprime la configuración, las consolas que ve y un intento de
conexión.

---

## 9. Registro de resultados

Copia esta tabla en un issue o al final de `docs/progreso.md` y complétala:

| Caso | Resultado (✅/❌) | Notas (tiempos, editor, capturas) |
|---|---|---|
| 04-A Aparece (mDNS) | | |
| 04-B Desaparece al cerrar | | |
| 04-C Timeout 15 s y reconexión | | segundos medidos: |
| 04-D Reinicio de la consola | | |
| 04-E Otro canal no se ve | | |
| 04-F Clave equivocada rechazada | | |
| 04-G Directo sin mDNS | | ¿hizo falta? |
| 06-A Sin configurar | | |
| 06-B Conecta e identifica la clase | | |
| 06-C Recibe código, ventana al frente, ✓ en el profesor | | |
| 06-D Copiar y pegar intacto | | editores: / salida de Python: |
| 06-E Agregar | | |
| 06-F Reemplazar | | |
| 06-G Limpiar | | |
| 06-H Avisos | | |
| 06-I Cerrar no deja procesos | | |
| 06-J Clave equivocada (app) | | |
| 07-A Abrir el aula | | ¿misma huella que `dos_alumnos`? |
| 07-B Alumnos en vivo (entran y salen) | | |
| 07-C Emitir: confirmados 2/2 ✓ | | dos VMs o alumno real + de consola: |
| 07-D Agregar bloque | | |
| 07-E Reemitir | | |
| 07-F Atajo global desde otra app | | hipervisor / app al frente: |
| 07-G Botón flotante y portapapeles vacío | | |
| 07-H Limpiar y avisos (2/2) | | |
| 07-I Sin alumnos: botones deshabilitados | | |
| 07-J Cambiar de canal | | |

Datos del entorno: hipervisor y modo de red, versión de Windows de cada VM, si se usó
`-directo`.

**Para dar por cerradas las etapas** en `docs/progreso.md`:

- **04:** basta con 04-A (o 04-G si la red no permite mDNS), 04-B y 04-C en ✅. Quita
  "(falta prueba en VMs)" de la fila 04.
- **06:** basta con 06-B, 06-C y 06-D en ✅. Quita "(falta prueba en VM)" de la fila 06.
- **07:** basta con 07-C (2/2 con dos alumnos) y 07-F (atajo con otra aplicación al frente) en
  ✅. Quita "(falta prueba en VMs)" de la fila 07. Si el atajo solo falló por el hipervisor y el
  botón (07-G) funcionó, anótalo: el atajo se confirma después en un equipo físico.

Cualquier ❌ va a "Pendientes conocidos" de `docs/progreso.md` con el caso y las notas.


---

## 10. Con los instaladores: dos salas (v0.2.0)

Ahora todo se instala como lo hará el centro de cómputo, sin copiar `.exe` sueltos, y se simulan
**dos salas** en la misma red: la VM del profesor y la VM de alumno 1 son la sala principal
(**canal 1**); la VM de alumno 2 es la sala alterna (**canal 2**). Usa los artefactos de una
versión publicada (*Releases*: `Aula-Consola-Setup-*.exe`, `Aula-Alumno-Setup-*.exe`) o
compílalos en tu equipo con `docs/desarrollo/instaladores-locales.md`. Guía de uso completa:
`docs/uso-hito1.md`.

Antes de empezar, en cada VM desinstala lo de las pruebas anteriores y borra
`%AppData%\Aula\alumno\config.json` de los alumnos: la configuración de máquina gana, pero así no
hay dudas.

| Caso | Pasos | Esperado |
|---|---|---|
| **08-A** Primera consola | En la VM del profesor ejecuta `Aula-Consola-Setup-*.exe`: canal predeterminado `1`, **Crear una clave nueva**, carpeta de salida = la carpeta compartida. Abre **Aula Consola** desde el menú Inicio. | En la carpeta compartida quedan `aula-centro.pub` y `aula-centro.key`. La consola abre **directo en el canal 1**, sin pantalla de selección, y la barra dice **"Clave del centro: XXXX-…"** (sin franja amarilla). No aparece aviso del firewall (`Get-NetFirewallRule -DisplayName "Aula Consola"`). |
| **08-B** Clave del centro protegida | PowerShell en la VM del profesor: `icacls C:\ProgramData\Aula\centro\clave-centro.pem` y, en `C:\Program Files\Aula\Consola`, `.\aula-consola.exe pubkey`. | Administradores y SYSTEM con `(F)`, Usuarios con `(R)` o `(RX)`. `pubkey` imprime lo mismo que `aula-centro.pub`. |
| **08-C** Alumno con asistente (sala 1) | Copia `Aula-Alumno-Setup-*.exe` a la carpeta compartida (junto a `aula-centro.pub`) y ejecútalo en la VM de alumno 1. | La página **"Canal y clave del centro"** trae canal `1` y la clave ya llena. Al terminar, Aula Cliente dice "conectado a la clase de … (canal 1)" y aparece en la consola. |
| **08-D** Alumno silencioso (sala 2) | En la VM de alumno 2, PowerShell **como administrador**: `.\Aula-Alumno-Setup-X.Y.Z.exe /VERYSILENT /SUPPRESSMSGBOXES /NORESTART /CANAL=2 /CLAVECENTRO=\\<carpeta>\aula-centro.pub`. Cierra sesión y vuelve a entrar. | Sin ventanas durante la instalación. Al iniciar sesión, Aula Cliente se abre solo y dice "buscando la clase del canal 2" con el motivo "No se encontró ninguna consola del canal 2…". **No** aparece en la consola (canal 1). |
| **08-E** El maestro cambia de canal | En la consola, **Canal** `2` → **Cambiar**. Luego vuelve a `1`. | En el 2 aparece solo el alumno 2 y en el 1 solo el alumno 1; cada uno reconecta solo en menos de 30 s. |
| **08-F** Configuración protegida | En una VM de alumno, con un usuario **sin** permisos de administrador, intenta editar `C:\ProgramData\Aula\alumno\config.json` con el Bloc de notas. | Windows no deja guardar ("Acceso denegado"). Abrirlo sí se puede. |
| **08-G** Mover un equipo de sala | En la VM de alumno 2, como administrador: `.\Aula-Alumno-Setup-X.Y.Z.exe /VERYSILENT /SUPPRESSMSGBOXES /NORESTART /CANAL=1` (sin `/CLAVECENTRO`). **No** reinicies. | En menos de 10 s el alumno 2 pasa al canal 1 y aparece en la consola junto al alumno 1. `config.json` conserva la clave. |
| **08-H** De punta a punta (**aceptación**) | Emite `prueba_sangria.py` (§6.3) a los alumnos. En cada uno, copia y pega en el Bloc de notas o IDLE. | "confirmados N/N ✓" y el código pegado idéntico (como en 06-D). |
| **08-I** Reiniciar la consola (**aceptación**) | Cierra Aula Consola y ábrela de nuevo. Repite reiniciando la VM del profesor. | Arranca sola en el canal 1. Los alumnos pasan a "sin conexión… reintentando" y vuelven a "conectado" solos, en menos de 30 s, sin tocar nada en ellos. |
| **08-J** Consola ajena (seguridad) | Cierra la consola instalada. En una VM sin la clave del centro, abre la consola **portable** `aula-consola.exe`; arranca en el canal 1. | La portable muestra la franja amarilla "Clave personal". Los alumnos **no** se conectan a ella y muestran "Se encontró la consola «…», pero su clave no es la de este centro". |
| **08-K** Segunda consola | En otra VM, instala la consola con canal predeterminado `2` e **Importar la clave del centro** (`aula-centro.key`). | Abre en el canal 2 con la **misma huella** que la primera consola, y el alumno de la sala 2 se conecta a ella. |
| **08-L** Diagnóstico | En un alumno: `& "C:\Program Files\Aula\Cliente\aula-cliente.exe" diagnostico`. Repite con la red marcada como **Pública** (como administrador, `Set-NetConnectionProfile -InterfaceAlias <red> -NetworkCategory Public`; después vuelve a `Private`). | Con red Privada: imprime la configuración y su ruta, la consola del canal y el intento de conexión correcto. Con Pública: no ve ninguna consola, y la ventana muestra el mismo motivo. |
| **08-M** Una sola instancia | En un alumno, abre Aula Cliente otra vez desde el menú Inicio. | No se abre una segunda ventana: la existente pasa al frente. La consola sigue mostrando una sola tarjeta. |
| **08-N** Actualizar | Instala de nuevo el mismo setup (o uno más nuevo) **sin** `aula-centro.pub` junto a él y sin `/CANAL`. | El agente se cierra, se reemplaza y sigue con su canal y su clave. |
| **08-O** Desinstalar | *Configuración → Aplicaciones → Aula Cliente → Desinstalar*. | Se quitan el programa, el arranque automático y la regla de firewall (`Get-NetFirewallRule -DisplayName "Aula Cliente (mDNS)"` ya no la encuentra). |
| **08-P** Carga (opcional) | En una VM de alumno: `go run ./examples/muchos_alumnos -n 12 -config C:\ProgramData\Aula\alumno\config.json` (o su `.exe`). Emite código y reinicia la consola. | "conectados 12/12", la consola cuenta 12 (más los alumnos reales), confirmados N/N y, tras el reinicio, "reconexiones 12". |
| **08-Q** Red Pública | Marca la red de los dos equipos como **Pública**. Luego reinstala la consola y el alumno marcando **"Permitir también en redes Públicas"** (o `/MERGETASKS="redpublica"` en silencioso). | Antes de reinstalar: el alumno dice "No se encontró ninguna consola del canal 1". Después se conecta solo. `Get-NetFirewallRule -DisplayName "Aula*" \| Select-Object DisplayName, Profile` muestra `Any` (o los tres perfiles). |

Anota los resultados en la tabla de §9 (agrega las filas 08-A a 08-Q). **Para aceptar la v0.2.0**
bastan 08-A, 08-C, 08-D, 08-E, 08-G, 08-H, 08-I y 08-J en ✅.

---

## 11. Etapa 09: miniaturas de pantalla (Windows)

Comprueba que el profesor ve las pantallas en miniatura, que el alumno **siempre** sabe que su
pantalla se está viendo y que cerrar la vista detiene la captura (`docs/privacidad.md`, reglas
6–8). La pantalla en blanco (`screen.freeze`) está documentada pero **no implementada**: no hay
casos para ella.

### 11.1 Preparación

La etapa 09 todavía no tiene versión publicada: compila las dos apps como en §2.1
(`wails build` en `apps\student` y `apps\teacher`).

- **Si las VMs ya tienen instalada la v0.2** (§10), reemplaza solo los ejecutables. La
  configuración (canal y clave del centro) se conserva. En cada VM, en PowerShell **como
  administrador**:

  ```powershell
  # Alumno: cierra Aula Cliente (icono en la barra de tareas) y reemplaza el .exe
  Stop-Process -Name aula-cliente -ErrorAction SilentlyContinue
  Copy-Item C:\aula-prueba\aula-cliente.exe "C:\Program Files\Aula\Cliente\aula-cliente.exe" -Force
  # Profesor: lo mismo con la consola
  Stop-Process -Name aula-consola -ErrorAction SilentlyContinue
  Copy-Item C:\aula-prueba\aula-consola.exe "C:\Program Files\Aula\Consola\aula-consola.exe" -Force
  ```

  Vuelve a abrir Aula Cliente desde el menú Inicio (o cierra sesión y entra de nuevo).
- **Sin instaladores** (portables, como en §5 y §6): abre `aula-consola.exe` en la VM del
  profesor y `aula-cliente.exe -canal 1 -clave aula-ed25519:…` en cada VM de alumno.

Necesitas la VM del profesor y **2 o 3 VMs de alumno con Windows**, todas en el canal 1 y
conectadas (punto verde en la consola). En la tarjeta de cada alumno debe aparecer la etiqueta
**pantalla**. Si no aparece, ese agente es de una versión anterior: reemplaza su `.exe`.

### 11.2 Casos

| Caso | Pasos | Esperado |
|---|---|---|
| **09-A** Sin vista no hay captura | Con la consola abierta y la vista de pantallas **cerrada**, mira la pantalla de cada alumno. | No hay franja roja arriba. La barra de la ventana de Aula dice "conectado a la clase de …" (sin rojo). |
| **09-B** Abrir la vista y aviso visible (**aceptación**) | En la consola pulsa **Ver pantallas** (arriba a la derecha). | En la consola: una tarjeta por alumno ("Pantallas 2 de 2"); en menos de 2 s cada una muestra la imagen de su escritorio. En **cada alumno**, antes de que aparezca su imagen: una **franja roja fija arriba al centro**, "● Tu pantalla está siendo vista por el profesor", y la barra de la ventana de Aula pasa a rojo con "Aula — tu pantalla está siendo vista por …". |
| **09-C** El aviso no se puede esconder | En un alumno, minimiza la ventana de Aula; abre el Bloc de notas maximizado; intenta hacer clic sobre la franja. | La franja sigue visible encima de todo. El clic pasa a la ventana de abajo (la franja no roba clics ni el foco). |
| **09-D** Miniatura en vivo | En un alumno, abre el Bloc de notas y escribe algo. | La miniatura de ese alumno cambia en 1–2 s. La franja roja también se ve en la miniatura. |
| **09-E** Vista en detalle | Clic en la miniatura de un alumno. Espera 3 s y pulsa **Volver a la cuadrícula**. | Se abre la pantalla de ese alumno en grande; tras el primer cuadro grande el texto del Bloc de notas se lee nítido. Al volver, la cuadrícula sigue actualizándose. |
| **09-F** Cerrar la vista detiene la captura (**aceptación**) | Pulsa **Cerrar** (o Esc dos veces si estabas en el detalle). | En cada alumno, en 1–2 s **desaparece la franja** y la barra vuelve a "conectado a la clase de …". En el Monitor de recursos del alumno (§11.3), `aula-cliente.exe` deja de enviar datos. |
| **09-G** Alumno nuevo con la vista abierta | Abre la vista. Luego abre Aula Cliente en otra VM (o reinícialo en una). | Aparece su tarjeta y su imagen sin tocar nada; en ese alumno aparece la franja. |
| **09-H** Se pierde la conexión | Con la vista abierta, cierra la consola (o desconecta la red de la VM del profesor). | En cada alumno la franja desaparece sola: al instante si cerraste la consola, en menos de 15 s si cortaste la red. Nadie queda con el aviso puesto. |
| **09-I** Cambiar de canal | Con la vista abierta, cambia la consola al canal 2 y luego vuelve al 1. | Al cambiar se cierra la vista y desaparecen las franjas. De vuelta en el canal 1, la vista está cerrada hasta que pulses **Ver pantallas**. |
| **09-J** Sesión bloqueada | Con la vista abierta, bloquea un alumno (Win+L). Desbloquéalo tras 10 s. | La miniatura se queda con la última imagen mientras está bloqueado (Windows no deja capturar la pantalla de bloqueo) y sigue actualizándose al desbloquear. |
| **09-K** Agente sin soporte | Si tienes una VM con un agente v0.2 (o Linux), conéctala. | Su tarjeta no tiene la etiqueta **pantalla** y la vista dice "Pantallas N de M" sin contarla. Nada falla. |
| **09-L** Nada en disco | En un alumno y en el profesor, tras la prueba, busca imágenes nuevas: `Get-ChildItem $env:LOCALAPPDATA\Aula, $env:APPDATA\Aula -Recurse -Include *.jpg,*.jpeg,*.png`. | No hay ninguna. Los logs (`aula-cliente.log`, `aula-consola.log`) solo dicen "screen stream started/stopped" y tamaños, nunca imágenes. |

**Para aceptar la etapa 09** bastan 09-B, 09-D, 09-E y 09-F en ✅, más la medición de §11.3.

### 11.3 Medición de rendimiento (CPU y red, proyectada a 20 alumnos)

El objetivo es saber cuánto cuesta la vista de pantallas: CPU en cada alumno, CPU en el
profesor y tráfico de red. Todavía **no hay una prueba automática con 20 agentes simulados**
(`muchos_alumnos` no envía miniaturas), así que se mide con agentes reales en VMs y se
**proyecta a 20**. El tráfico y la CPU del profesor crecen en proporción al número de alumnos;
la CPU de cada alumno no depende de cuántos haya.

**Qué se espera** (por diseño, `docs/decisiones.md` D-013):

| Medida | Valor esperado |
|---|---|
| Una miniatura (320 px, calidad 50, 1 por segundo) | 10–25 KiB/s por alumno |
| Vista en detalle (1280 px, calidad 70, 1 por segundo) | 60–200 KiB/s, solo ese alumno |
| 20 alumnos en miniatura | 200–500 KiB/s ≈ 1,6–4 Mbit/s hacia el profesor |

#### Antes de empezar

- Cierra en las VMs todo lo que no sea Aula (navegador, actualizaciones de Windows en curso).
- Anota de cada VM: vCPU, RAM, versión de Windows, y del anfitrión: CPU y si las VMs comparten
  disco y red. En VMs la CPU sale más alta que en un equipo físico: anótalo junto al resultado.
- En **cada alumno** deja abierto algo que cambie, por ejemplo un video en el navegador o el
  Bloc de notas con texto, para no medir una pantalla quieta (los JPEG de una pantalla quieta y
  lisa son más pequeños).

#### Herramientas

- **Administrador de tareas** (`Ctrl+Shift+Esc`) → **Más detalles**:
  - pestaña **Rendimiento → Ethernet**: gráfica de *Enviar* y *Recibir* en Kbps de la VM;
  - pestaña **Detalles**: columna **CPU** por proceso (clic derecho en el encabezado →
    *Seleccionar columnas* para agregar *Tiempo de CPU* si quieres).
- **Monitor de recursos** (`Win+R` → `resmon` → Enter):
  - pestaña **CPU**: en *Procesos*, marca la casilla de `aula-cliente.exe` (alumno) o
    `aula-consola.exe` (profesor). Las columnas **CPU** (instantánea) y **Promedio de CPU**
    (último minuto) dan el valor a anotar;
  - pestaña **Red** → *Procesos con actividad de red*: columnas **Enviar (B/s)** y **Recibir
    (B/s)** del proceso, promediadas sobre el último minuto.
- En el profesor, la imagen la dibuja WebView2: además de `aula-consola.exe` hay varios
  procesos **`msedgewebview2.exe`** hijos. Súmalos a la CPU de la consola (en el Administrador
  de tareas aparecen agrupados bajo *Aula — Consola* en la pestaña *Procesos*).

**Registro automático (opcional)** con `typeperf`, que guarda una muestra por segundo en CSV.
En PowerShell, en la VM que mides:

```powershell
# Alumno: CPU de aula-cliente y tráfico de la tarjeta de red, 120 muestras (2 min)
typeperf "\Proceso(aula-cliente)\% de tiempo de procesador" "\Interfaz de red(*)\Bytes enviados/s" -si 1 -sc 120 -o C:\aula-prueba\alumno.csv

# Profesor: CPU de aula-consola y bytes recibidos
typeperf "\Proceso(aula-consola)\% de tiempo de procesador" "\Interfaz de red(*)\Bytes recibidos/s" -si 1 -sc 120 -o C:\aula-prueba\profesor.csv
```

- En Windows en inglés los contadores se llaman `\Process(aula-cliente)\% Processor Time`,
  `\Network Interface(*)\Bytes Sent/sec` y `\Bytes Received/sec`.
- El `% de tiempo de procesador` de un proceso suma todos los núcleos: **divídelo entre el
  número de vCPU** para compararlo con el Administrador de tareas.
- El contador de red es de toda la tarjeta, no solo de Aula: por eso se resta la línea base.

#### Pasos

Sea **N** el número de alumnos conectados con la etiqueta **pantalla** (2 o 3 en VMs; más si el
anfitrión aguanta).

1. **Línea base (vista cerrada), 2 minutos.** Con todos conectados y la vista de pantallas
   cerrada, espera 2 minutos y anota de cada alumno *Promedio de CPU* y *Enviar (B/s)* de
   `aula-cliente.exe`, y del profesor la CPU de la consola (con WebView2) y *Recibir (B/s)*.
   Es el costo del latido (heartbeat): debe ser casi cero.
2. **Miniaturas, 2 minutos.** Pulsa **Ver pantallas**, espera 2 minutos completos (los promedios
   del Monitor de recursos cubren el último minuto) y anota lo mismo.
3. **Detalle, 2 minutos.** Abre la vista en detalle de un alumno, espera 2 minutos y anota los
   valores de **ese** alumno y del profesor.
4. **Cerrar, 1 minuto.** Pulsa **Cerrar**. Al minuto, los valores deben volver a la línea base
   (confirma 09-F con números).
5. **Si puedes, repite el paso 2 con más alumnos** (por ejemplo, 2 y luego 4 VMs) para comprobar
   que el tráfico del profesor crece en proporción.

#### Cálculo y proyección a 20

- **Red por alumno** = (*Recibir* del profesor con miniaturas − línea base) ÷ N. Contrástalo con
  el *Enviar* de cada alumno: deben coincidir aproximadamente.
- **Red con 20 alumnos** = red por alumno × 20 (+ la vista en detalle de uno, si está abierta).
- **CPU del profesor por alumno** = (CPU consola + WebView2 con miniaturas − línea base) ÷ N;
  **con 20** = ese valor × 20.
- **CPU del alumno**: se toma tal cual (no depende de cuántos haya).

**Referencia orientativa** para dar la medición por buena (equipo de laboratorio típico de 4
núcleos o una VM de 2 vCPU):

| Medida | Aceptable |
|---|---|
| CPU de `aula-cliente.exe` con miniaturas | ≤ 5 % de promedio |
| Red proyectada a 20 alumnos (miniaturas) | ≤ 5 Mbit/s (una red de 100 Mbit/s la soporta de sobra) |
| CPU proyectada del profesor a 20 alumnos (consola + WebView2) | ≤ 30 % |
| Tras **Cerrar** | igual a la línea base (± 1 %, ± 1 KiB/s) |

Si algo se pasa, anótalo con los datos del entorno: se ajustan el intervalo, la calidad o el
tamaño de las miniaturas (`internal/screen`, `ThumbParams`) sin cambiar el protocolo.

#### Registro

| Medida | Línea base | Miniaturas | Detalle | Tras cerrar |
|---|---|---|---|---|
| N (alumnos con pantalla) | | | — | |
| CPU `aula-cliente.exe` (promedio, alumno 1) | | | | |
| CPU `aula-cliente.exe` (promedio, alumno 2) | | | | |
| Enviar `aula-cliente.exe` (B/s, alumno 1) | | | | |
| CPU consola + WebView2 (profesor) | | | | |
| Recibir (B/s, profesor) | | | | |
| **Proyección a 20:** red (Mbit/s) | — | | | — |
| **Proyección a 20:** CPU del profesor | — | | | — |

Datos del entorno: anfitrión (CPU, RAM), hipervisor, vCPU y RAM de cada VM, versión de Windows,
resolución de pantalla de los alumnos (una pantalla más grande no cambia el tamaño de la
miniatura, pero sí el costo de capturarla).

Copia los resultados (09-A a 09-L y esta tabla) a `docs/progreso.md`, en las notas de la etapa
09.

## 12. Transmisión de la pantalla del profesor (Windows)

Comprueba que 2 o 3 alumnos reciben la pantalla del profesor y la leen nítida, que **siempre**
saben que están viendo la transmisión, que un alumno que cae vuelve solo y que la lista de
alumnos nunca sale en el video (`docs/privacidad.md`, reglas 12–16). Mide además la CPU y la
subida del profesor (`docs/decisiones.md` D-014).

### 12.1 Preparación

- Compila las dos apps (§2.1) y reemplaza los `.exe` como en §11.1. La transmisión necesita
  **las dos** apps nuevas.
- En cada VM de alumno, la tarjeta de la consola debe mostrar la etiqueta **ve tu pantalla**. Si
  no aparece:
  - el agente es viejo; o
  - su WebView2 no tiene H.264 (Windows "N" sin *Media Feature Pack*): `aula-cliente.log` dice
    `frontend media support … h264=false`.
- **Firewall:** la regla "Aula Consola" del instalador es por programa y ya cubre el UDP 47831.
  Con portables, agrega la regla de §2.5 para `aula-consola.exe` (sin puerto) o abre el UDP:
  `New-NetFirewallRule -DisplayName "Aula transmision" -Direction Inbound -Protocol UDP -LocalPort 47831 -Action Allow -Profile Private,Domain`.
- Si la consola de la VM del profesor no tiene H.264 (Windows N), al pulsar **Transmitir mi
  pantalla** dice que falta el codificador.

### 12.2 Casos

| Caso | Pasos | Esperado |
|---|---|---|
| **TP-A** Controles | En la consola, mira la barra superior. | A la derecha: selector "Monitor 1 (principal) · 1920×1080" (uno por monitor) y el botón **Transmitir mi pantalla**. |
| **TP-B** Empezar y aviso del alumno (**aceptación**) | Abre un archivo de código en el Bloc de notas o VS Code de la VM del profesor y pulsa **Transmitir mi pantalla**. | **En cada alumno, antes del video:** la ventana de Aula pasa al frente, se maximiza y muestra la franja azul "▶ Estás viendo la pantalla de <profesor> (canal 1)". La barra de la ventana dice "Aula — viendo la pantalla de …". En 1–3 s aparece el video. **En la consola:** la franja roja "● Transmitiendo tu pantalla (monitor 1) a N de N alumnos" con **Detener**. |
| **TP-C** Código nítido (**aceptación**) | Con una fuente de 14–16 pt en el editor del profesor, escribe y desplázate. | El texto se lee en los alumnos; al escribir, el cambio llega en menos de 1 s. Si la pantalla del alumno es más chica, **Tamaño real** lo muestra 1:1 con barras de desplazamiento y **Ajustar a la ventana** vuelve. |
| **TP-D** La consola no sale en el video (**privacidad**) | Pon la ventana de la consola (con la lista de alumnos) sobre el monitor transmitido. | Los alumnos **no** la ven: ven lo que hay detrás (o un rectángulo negro en Windows anteriores a 10 2004). Al **Detener**, la consola vuelve a salir en capturas normales (prueba con Win+Shift+S). |
| **TP-E** El alumno no puede ocultar el aviso | En un alumno, minimiza o achica la ventana de Aula. | La transmisión sigue. La barra de tareas sigue diciendo "Aula — viendo la pantalla de …". Al restaurar la ventana, la franja azul sigue arriba del video. |
| **TP-F** Un alumno cae y vuelve (**aceptación**) | Desconecta la red de una VM de alumno 20 s y vuelve a conectarla (o cierra y abre Aula Cliente). | Al caer, su vista se cierra sola y la consola cuenta un receptor menos. Al volver, en menos de 30 s reaparecen la franja y el video **sin tocar la consola**. |
| **TP-G** Alumno nuevo durante la transmisión | Con la transmisión activa, abre Aula Cliente en otra VM. | Recibe la transmisión sola y la consola cuenta un receptor más. |
| **TP-H** Detener | Pulsa **Detener** (en la franja o en el botón de la barra). | En cada alumno, en 1–2 s desaparecen la franja y el video, la ventana se restaura y vuelve la vista de código. La franja roja de la consola desaparece. |
| **TP-I** Cambiar de canal o cerrar | Transmite y cambia la consola al canal 2 (o ciérrala). | La transmisión termina en todos los alumnos del canal 1. |
| **TP-J** Pantalla vista + transmisión | Transmite y abre además **Ver pantallas**. | En los alumnos aparecen las dos cosas: la franja **azul** de la transmisión y el distintivo **rojo** "Tu pantalla está siendo vista por el profesor". Son avisos distintos. |
| **TP-K** Nada en disco | Tras la prueba, busca videos o imágenes nuevas en `%LOCALAPPDATA%\Aula` y `%APPDATA%\Aula` (como 09-L, agregando `*.mp4,*.webm,*.h264`). | No hay ninguna. Los logs dicen "broadcast started/stopped", intentos y tamaños, nunca SDP ni candidatos. |

**Para aceptar la transmisión** bastan TP-B, TP-C, TP-D, TP-F y TP-H en ✅, más la medición de
§12.3.

### 12.3 Medición: CPU y subida del profesor

Hay dos formas, y conviene hacer las dos:

1. **Con la consola real** (Administrador de tareas o `typeperf`, como en §11.3): CPU de
   `aula-consola.exe` y *Bytes enviados/s* de la tarjeta de red del profesor. Mide 2 minutos
   así:
   - sin transmitir (línea base);
   - transmitiendo un editor quieto;
   - transmitiendo mientras escribes y te desplazas sin parar (peor caso).
2. **Con `examples/transmision`** en la VM del profesor (cierra antes la consola real; usa el
   mismo puerto):

   ```powershell
   go build -o transmision.exe ./examples/transmision     # en tu equipo; copia el .exe a la VM
   .\transmision.exe -canal 1 -dir "$env:APPDATA\Aula\consola"           # patrón sintético de código
   .\transmision.exe -canal 1 -dir "$env:APPDATA\Aula\consola" -monitor 0 # el monitor real
   ```

   `-dir` debe apuntar a la carpeta donde está la clave con la que se instalaron los alumnos (en
   un centro, la clave del centro: usa una consola portable con esa clave). Cada 5 s imprime:
   `receptores 2/2 · video 250 kbit/s · subida ≈ 0.5 Mbit/s · 4.0 cuadros/s · codificar 15ms/cuadro · CPU 40 % de un núcleo`.

**Proyección a 40 alumnos:** la CPU casi no depende del número de alumnos, porque se codifica
una sola vez. La subida sí: *video × alumnos*.

| Medida | Aceptable (D-014) |
|---|---|
| Subida pico proyectada a 40 (tope 2,5 Mbit/s) | ≤ 100 Mbit/s y ≤ 50 % del enlace del profesor; si no, baja `broadcast_max_kbps` |
| CPU del profesor transmitiendo un editor quieto | ≤ 50 % de un núcleo |
| CPU del profesor escribiendo sin parar (peor caso) | anótala; con el codificador de software puede pasar de un núcleo en equipos modestos |

Referencia en el equipo de desarrollo (Ryzen 5 3450U, 1080p, 1 receptor): 25–45 % de un núcleo
y 100–320 kbit/s con el patrón; ~130 % de un núcleo si cada cuadro cambia.

| Medida | Línea base | Editor quieto | Escribiendo | Tras detener |
|---|---|---|---|---|
| N (receptores) | | | | |
| CPU consola (% de un núcleo) | | | | |
| Video (kbit/s, del ejemplo) | — | | | — |
| Enviados (Mbit/s, profesor) | | | | |
| **Proyección a 40:** subida (Mbit/s) | — | | | — |

Copia los resultados (TP-A a TP-K y esta tabla) a `docs/progreso.md`, en las notas de la
transmisión.

## 13. El agente como servicio de Windows (D-016)

Comprueba que el agente arranca con el equipo como servicio y abre la ventana del alumno en su
sesión, y que el alumno sigue viendo el distintivo de identidad (`docs/privacidad.md`, reglas
17–19). Hazlo en una VM: instala un servicio real.

### 13.1 Preparación

- VM del profesor con la consola abierta en el canal 1, y una VM de alumno con Windows y **dos
  cuentas**: una **estándar** (el alumno) y una de administrador.
- Instala el alumno con el `Aula-Alumno-Setup-…` de esta versión, como administrador (canal 1 y la
  clave del centro). Si no hay versión publicada con el servicio, genera los instaladores con
  `docs/desarrollo/instaladores-locales.md`. Si la VM tenía una versión anterior, el instalador la actualiza: quita el
  inicio automático por usuario y deja el servicio.

### 13.2 Casos

| Caso | Pasos | Esperado |
|---|---|---|
| **SV-A** Instalado como servicio | En la VM del alumno (administrador): `"C:\Program Files\Aula\Cliente\aula-cliente.exe" servicio estado`. | `Servicio AulaAgente: en ejecución (PID …); arranca con el equipo; se reinicia si falla`. En `services.msc` aparece "Aula — agente del aula", tipo de inicio Automático, cuenta Sistema local, y en *Recuperación* "Reiniciar el servicio" en los tres fallos. |
| **SV-B** Arranca con el equipo (**aceptación**) | Reinicia la VM del alumno y **no inicies sesión**. Mira la consola. | El equipo aparece conectado **sin usuario** y sin etiquetas de capacidades. No se le puede emitir código ni ver su pantalla. |
| **SV-C** Ventana en la sesión del alumno (**aceptación**) | Inicia sesión con la cuenta **estándar**. | En segundos se abre la ventana de Aula con "Aula — conectado a la clase de … (canal 1)". En la consola, el equipo pasa a mostrar el usuario y sus etiquetas (código, avisos, pantalla, ve tu pantalla). Emite código: llega y **Copiar** funciona. |
| **SV-D** El ayudante no tiene privilegios | En el Administrador de tareas → Detalles, agrega la columna *Elevado*. | `aula-cliente.exe` del usuario: Elevado = No, usuario = el alumno. El del servicio: usuario SYSTEM. |
| **SV-E** Cerrar la ventana a la fuerza | Como alumno, termina `aula-cliente.exe` (el de su usuario) desde el Administrador de tareas. | En pocos segundos la ventana vuelve a abrirse sola con el distintivo. En la consola el equipo se reconecta: unos segundos sin etiquetas y luego con ellas. |
| **SV-F** El alumno no puede detener el servicio | Como alumno: `sc stop AulaAgente` (o en `services.msc`). | "Acceso denegado". El agente sigue conectado. |
| **SV-G** Reinicio ante fallo | Como administrador, termina el `aula-cliente.exe` de **SYSTEM**. | A los ~5 s el servicio vuelve a estar en ejecución con otro PID (`servicio estado`), se reconecta y relanza la ventana del alumno. |
| **SV-H** Cerrar sesión y cambiar de usuario | Cierra la sesión del alumno; luego entra con otra cuenta. | Al cerrar sesión: el equipo queda sin usuario ni capacidades en la consola. Al entrar la otra cuenta: la ventana se abre en **su** sesión y la consola muestra el nuevo usuario. |
| **SV-I** Miniaturas y transmisión a través del ayudante | Con el alumno en sesión: **Ver pantallas** y luego **Transmitir mi pantalla** en la consola. | Igual que en §11 y §12: la franja roja "Tu pantalla está siendo vista" aparece en el alumno antes de la imagen, y la vista de transmisión con su franja azul. |
| **SV-J** Logs | Abre `C:\ProgramData\Aula\alumno\logs\aula-agente.log` como alumno. | Se puede leer (no modificar). Dice "helper started", "helper ready", sesiones y usuarios; nunca código ni imágenes. |
| **SV-K** Desinstalar | Desinstala Aula Cliente desde Configuración → Aplicaciones. | `sc query AulaAgente` dice que no existe; ningún `aula-cliente.exe` queda corriendo. |

**Para aceptar** bastan SV-B, SV-C, SV-E y SV-H en ✅.

Copia los resultados a `docs/progreso.md`, en las notas del agente como servicio.

## 14. Restricciones de la clase: bloqueo de apps y filtro web (D-017)

Comprueba que el profesor bloquea apps y sitios, que el alumno lo ve, y que al terminar el equipo
queda **exactamente** como estaba (`docs/privacidad.md`, reglas 20–25). Necesita el agente como
servicio (§13) y una VM, porque cambia la configuración DNS del equipo.

### 14.1 Antes de empezar (línea base)

En la VM del alumno, PowerShell **como administrador**; guarda la salida:

```powershell
Get-DnsClientNrptRule | Format-List Namespace, NameServers, Comment > C:\aula-prueba\nrpt-antes.txt
reg query HKLM\SOFTWARE\Policies /s > C:\aula-prueba\politicas-antes.txt 2>&1
Get-DnsClientServerAddress | Format-Table > C:\aula-prueba\dns-antes.txt
```

### 14.2 Casos

| Caso | Pasos | Esperado |
|---|---|---|
| **RB-A** Aviso antes de aplicar (**aceptación**) | En la consola, **Restricciones** → Aplicaciones: escribe una app instalada en `Program Files`, p. ej. `notepad++` o `code` (las de `C:\Windows`, como el Bloc de notas, están protegidas y nunca se cierran). Modo **Bloquear estas**, Todo el canal → **Aplicar**. | En el alumno, la ventana pasa al frente con "🔒 Restricciones de la clase activas: aplicaciones bloqueadas" y la barra dice "· restricciones activas". En la consola, la tarjeta muestra "🔒 apps" y el panel "activa en 1 de 1". |
| **RB-B** La app se cierra | En el alumno, abre la app bloqueada. | Se cierra en menos de 1 s y aparece "Aula cerró «…»: está bloqueada durante la clase". |
| **RB-C** Lista blanca de apps | Modo **Permitir solo estas** con `code`. Abre otra app de `Program Files`. | Se cierra; `code`, el explorador y Aula siguen abiertos. |
| **RB-D** Filtro web (**aceptación**) | Sitios: `youtube.com`, **Bloquear estos** → Aplicar. En el alumno, abre `https://www.youtube.com` en Edge y en Chrome (y Firefox si está). | No carga ("no se encuentra el sitio"), en ningún navegador. `https://python.org` sí. El aviso dice "sitios web filtrados". |
| **RB-E** Lista blanca web | **Plantilla: solo documentación** → Aplicar. | `docs.python.org` y `developer.mozilla.org` cargan; `youtube.com` no. |
| **RB-F** Ver detalles | En el alumno, **Ver detalles**. | Lista las apps y los sitios; con la plantilla de documentación muestra 14 y no dice "y N más"; con más de 15 sí. |
| **RB-G** Solo a alumnos marcados | Con 2 alumnos: marca uno, elige "Alumnos marcados" y aplica un sitio. | Solo ese alumno tiene el aviso y el bloqueo. |
| **RB-H** Quitar y equipo limpio (**aceptación**) | **Quitar todas las restricciones**. Repite los comandos de 14.1 con `-despues` y compara (`fc`). | El aviso desaparece, YouTube carga, y los tres archivos son **idénticos** a los de antes: sin regla NRPT de Aula, sin directivas nuevas, DNS de los adaptadores igual. |
| **RB-I** Desconexión | Aplica un sitio; cierra la consola. | En segundos el aviso desaparece y el sitio carga. Equipo limpio como en RB-H. |
| **RB-J** Caída del agente (**aceptación**) | Aplica un sitio. Como administrador, termina el `aula-cliente.exe` de **SYSTEM**. Espera ~10 s. | El servicio se reinicia solo, y **antes de reconectarse** deshace el filtro (el log `aula-agente.log` dice "restoring a web filter left by a previous run"). Al reconectar, la consola le reenvía la política. |
| **RB-K** Apagado a mitad de clase | Aplica un sitio y **apaga** la VM del alumno sin cerrar nada (Apagar desde el hipervisor). Enciéndela **sin** abrir la consola. | Al arrancar, el sitio carga y el equipo queda limpio (RB-H), aunque no haya consola. |
| **RB-L** No se vigila la navegación | Revisa `aula-agente.log` tras navegar con el filtro activo. | Solo dice "web filter applied/removed" con conteos; **ningún** nombre de sitio visitado. |

**Para aceptar** bastan RB-A, RB-D, RB-H y RB-J en ✅.
