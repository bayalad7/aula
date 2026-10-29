# Prueba en VMs: etapas 04, 06 y 07

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
compílalos con los comandos de `docs/desarrollo/publicar.md`. Guía de uso completa:
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

Anota los resultados en la tabla de §9 (agrega las filas 08-A a 08-P). **Para aceptar la v0.2.0**
bastan 08-A, 08-C, 08-D, 08-E, 08-G, 08-H, 08-I y 08-J en ✅.
