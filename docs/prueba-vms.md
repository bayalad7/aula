# Prueba en VMs: etapas 04 y 06

Guía paso a paso para cerrar las dos etapas que siguen marcadas "falta prueba en VMs" en
`docs/progreso.md`:

| Etapa | Qué tiene que pasar en VMs reales |
|---|---|
| **04** Descubrimiento y registro | Levantas el profesor y un alumno **aparece** en el registro. Al cerrarlo (o si su equipo se cuelga) **desaparece**. |
| **06** Código en el alumno | Emites código desde la consola y el alumno lo **copia y pega intacto** en su editor. |

Cada caso tiene un identificador (04-A, 06-C…). Al final hay una tabla para anotar los
resultados (§8). Calcula unos **45 minutos** la primera vez.

---

## 1. Qué necesitas

| Dónde | Qué |
|---|---|
| Tu equipo de desarrollo (`C:\aula`) | Go y la CLI de Wails, para compilar (ya los tienes). |
| Hipervisor | VirtualBox, Hyper-V o VMware. |
| **VM del profesor** | Windows 10/11. No hay que instalar nada. |
| **VM(s) del alumno** | Windows 10/11 con **WebView2** (lo trae Windows 11 y Windows 10 actualizado; ver §2.4). Una sola VM basta; con dos se prueba mejor el registro. |
| Opcional en el alumno | Python (`winget install Python.Python.3.12`) para ejecutar lo que pegues (06-D). Sin Python, la comprobación se hace a ojo en el Bloc de notas. |

No instalas Go ni el repositorio en las VMs: copias tres archivos ya compilados.

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

# Código de muestra para enviar
Copy-Item examples\snippets\*.py dist\
```

En `dist\` quedan:

| Archivo | Va a |
|---|---|
| `dos_alumnos.exe` | VM del profesor **y** VMs de alumno (etapa 04) |
| `aula-cliente.exe` | VMs de alumno (etapa 06) |
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

### 2.4 WebView2 en la VM del alumno

`aula-cliente.exe` dibuja su ventana con WebView2. Para comprobar si está instalado, abre
**Configuración → Aplicaciones** y busca "Microsoft Edge WebView2 Runtime". Si falta, al abrir
`aula-cliente.exe` aparece un aviso para descargarlo. Acéptalo, o instálalo desde
<https://developer.microsoft.com/microsoft-edge/webview2/> ("Evergreen Bootstrapper").

### 2.5 Firewall de la VM del profesor

La primera vez que ejecutes el profesor, Windows pregunta si permites el acceso: marca **Redes
privadas** y pulsa **Permitir acceso**. Si no apareció el aviso, abre los puertos a mano
(PowerShell como administrador):

```powershell
New-NetFirewallRule -DisplayName "Aula consola (TCP 47830)" -Direction Inbound -Protocol TCP -LocalPort 47830 -Action Allow
New-NetFirewallRule -DisplayName "Aula mDNS (UDP 5353)"     -Direction Inbound -Protocol UDP -LocalPort 5353  -Action Allow
```

Las VMs de alumno no necesitan reglas: solo hacen conexiones salientes.

> `ping` entre VMs Windows suele fallar aunque la red esté bien, porque el firewall bloquea ICMP
> por defecto. Para comprobar la conectividad usa la prueba de §3.2.

---

## 3. Arrancar el profesor (común a ambas etapas)

### 3.1 En la VM del profesor

Abre PowerShell en `C:\aula-prueba\` y ejecuta:

```powershell
.\dos_alumnos.exe -rol profesor
```

Debes ver:

```
No había clave: se creó una nueva en C:\Users\...\AppData\Roaming\Aula\consola     ← solo la 1.ª vez
Consola "Consola de ejemplo" en el canal AULA_101, puerto 47830 (TLS). Ctrl+C para salir.
Clave pública para los alumnos: aula-ed25519:I5rPjgu2TFt5s5eDGz7sJst4FfG9v+bZIOwFmYuFvf0=
Huella: 7E65-1323-287C-FE59
Escribe "ayuda" para ver cómo enviar código a los alumnos.
[10:00:00] Registro AULA_101: 0 equipo(s)
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
| Alumno | `PC-ALUMNO: buscando consola en AULA_101` y luego `PC-ALUMNO: conectado (Consola de ejemplo)` |
| Profesor | `Registro AULA_101: 1 equipo(s)` con una línea `PC-ALUMNO  <usuario>  windows  conectado  <IP>:<puerto>` |

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
.\dos_alumnos.exe -rol alumno -clave $clave -canal AULA_102
```

- **Alumno:** se queda en `buscando consola en AULA_102` y **nunca** se conecta.
- **Profesor:** no aparece ningún equipo nuevo.

Con dos VMs de alumno puedes probar a la vez una en `AULA_101` (aparece) y otra en `AULA_102`
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
.\aula-cliente.exe -canal AULA_101 -clave $clave
```

Si mDNS no funcionó en 04-A, agrega `-directo <IP del profesor>:47830`.

| Dónde | Esperado |
|---|---|
| Alumno (ventana) | Barra superior con punto verde: **"Aula — conectado a la clase de Consola de ejemplo (canal AULA_101)"**. El mismo texto aparece en el título de la ventana y en la barra de tareas. En el centro: "Cuando el profesor comparta código, aparecerá aquí listo para copiar." |
| Profesor | El equipo aparece `conectado` en el registro. |

> Para no escribir los argumentos cada vez, crea `%AppData%\Aula\alumno\config.json` en la VM
> del alumno (pega la ruta en la barra del Explorador):
>
> ```json
> { "channel": "AULA_101", "console_public_key": "aula-ed25519:..." }
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
.\aula-cliente.exe -canal AULA_101 -clave aula-ed25519:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=
```

- **Alumno:** la barra dice **"Aula — buscando la clase del canal AULA_101"** y nunca pasa a
  conectado.
- **Profesor:** no aparece en el registro; si le envías `codigo …`, dice `enviado a 0 equipo(s)`.

---

## 6. Comandos de referencia

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

**Opciones** de `dos_alumnos.exe`: `-canal AULA_101`, `-puerto 47830`,
`-nombre "Consola de ejemplo"`, `-v` (logs detallados). Las de `aula-cliente.exe` son `-canal`,
`-clave`, `-directo IP:puerto` y `-config archivo.json`.

---

## 7. Si algo falla

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
| Puerto ocupado | `-puerto 47900` en el profesor (y en `-directo` si lo usas). |

Para ver logs detallados, agrega `-v` en `dos_alumnos.exe`. `aula-cliente.exe` es una app de
ventana y no muestra logs en la terminal.

---

## 8. Registro de resultados

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

Datos del entorno: hipervisor y modo de red, versión de Windows de cada VM, si se usó
`-directo`.

**Para dar por cerradas las etapas** en `docs/progreso.md`:

- **04:** basta con 04-A (o 04-G si la red no permite mDNS), 04-B y 04-C en ✅. Quita
  "(falta prueba en VMs)" de la fila 04.
- **06:** basta con 06-B, 06-C y 06-D en ✅. Quita "(falta prueba en VM)" de la fila 06.

Cualquier ❌ va a "Pendientes conocidos" de `docs/progreso.md` con el caso y las notas.
