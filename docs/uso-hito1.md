# Guía de uso — Hito 1: distribuir código en clase

Con Aula, el profesor envía código desde su equipo y cada alumno lo ve en su pantalla, resaltado
y listo para **copiar y pegar intacto** con un clic. Esta guía cubre la instalación en el
laboratorio y el uso en clase.

| Pieza | Va en | Archivo |
|---|---|---|
| **Aula Consola** | El equipo del profesor | `Aula-Consola-Setup-X.Y.Z.exe` (o el portable `aula-consola-X.Y.Z-windows-amd64.exe`) |
| **Aula Cliente** | Cada equipo de alumno | `Aula-Alumno-Setup-X.Y.Z.exe` + `aula-alumno.json` |

Descárgalos de la página de versiones del repositorio (*Releases*). Requisitos: Windows 10/11 de
64 bits con WebView2 (incluido en Windows 11 y en Windows 10 actualizado), y todos los equipos en
la **misma red local**.

---

## 1. Instalar la consola (una vez, en el equipo del profesor)

1. Ejecuta `Aula-Consola-Setup-X.Y.Z.exe` (pide permisos de administrador). Si Windows muestra
   "Windows protegió tu PC", pulsa *Más información → Ejecutar de todas formas*.
2. El instalador crea el acceso "Aula Consola" en el menú Inicio y la regla de firewall
   **"Aula Consola"**. Así Windows no pregunta en plena clase.
3. Abre **Aula Consola**.

La primera vez que abras una clase se crea la **clave de la consola**, en
`%AppData%\Aula\consola\`. Esa clave identifica tu consola ante los alumnos. **Respáldala**
(copia la carpeta): si se pierde, tendrás que reinstalar la configuración de todos los alumnos.
Si cambias de equipo, copia la carpeta al mismo lugar en el nuevo.

> ¿Sin permisos de administrador? Usa el portable `aula-consola-X.Y.Z-windows-amd64.exe`. Funciona
> igual; la primera vez, Windows preguntará por el firewall: marca **Redes privadas**.

## 2. Abrir una clase y elegir el canal

1. En la pantalla de inicio, escribe el **canal** del aula (por ejemplo `AULA_101` o
   `CURSO_PYTHON_A`) o pulsa uno de los recientes. Cada aula tiene su canal; varias clases pueden
   convivir en la misma red sin mezclarse.
2. Escribe el **nombre** que verán los alumnos, por ejemplo `Prof. Ayala — Programación I`.
3. Pulsa **Abrir clase**.

Arriba verás el canal, tu nombre y la huella de la clave (`XXXX-XXXX-XXXX-XXXX`). **Cambiar de
aula** cierra la clase y vuelve a esta pantalla.

## 3. Preparar e instalar el agente en los equipos de alumno

### 3.1 Generar la configuración (una vez por aula)

Con la clase abierta, pulsa **Configuración de alumnos** y guarda `aula-alumno.json`. Contiene
el canal y la clave **pública** de tu consola; no es secreto.

En la terminal hace lo mismo `aula-consola config-alumno -canal AULA_101`.

Pon `aula-alumno.json` en la **misma carpeta** que `Aula-Alumno-Setup-X.Y.Z.exe`, por ejemplo en
una carpeta compartida de la red o en una memoria USB.

### 3.2 Instalar en cada equipo

**Uno por uno:** ejecuta `Aula-Alumno-Setup-X.Y.Z.exe` desde esa carpeta. El instalador:

- copia Aula Cliente a `C:\Program Files\Aula\Cliente\`;
- copia tu configuración a `C:\ProgramData\Aula\alumno\config.json`, protegida para que solo
  un administrador la cambie;
- deja Aula Cliente **arrancando al iniciar sesión** cualquier usuario del equipo;
- crea la regla de firewall **"Aula Cliente (mDNS)"**.

Si el instalador avisa que no encontró `aula-alumno.json`, el agente queda instalado pero "sin
configurar". Pon el archivo junto al instalador y ejecútalo de nuevo.

**En silencio, en muchos equipos** (script de inicio, GPO o herramienta de despliegue, como
administrador):

```bat
\\servidor\aula\Aula-Alumno-Setup-X.Y.Z.exe /VERYSILENT /SUPPRESSMSGBOXES /NORESTART /CONFIG="\\servidor\aula\aula-alumno.json"
```

Sin `/CONFIG`, toma el `aula-alumno.json` que esté junto al instalador. Para actualizar a una
versión nueva, ejecuta el instalador nuevo del mismo modo: cierra el agente, lo reemplaza y
conserva la configuración. Para desinstalar, usa *Configuración → Aplicaciones → Aula Cliente*.

### 3.3 Qué ve el alumno

Al iniciar sesión se abre la ventana **Aula Cliente**. Arriba, siempre visible, dice:

> **Aula — conectado a la clase de Prof. Ayala — Programación I (canal AULA_101)**

El mismo texto aparece en el título de la ventana y en la barra de tareas. El agente nunca corre
oculto: si el alumno cierra la ventana, se desconecta, y en tu consola su tarjeta aparece como
"desconectado".

## 4. En clase: alumnos, código y avisos

### 4.1 Ver quién está conectado

A la izquierda aparece una **tarjeta por alumno** (equipo, usuario, sistema) que se actualiza en
vivo: verde si está conectado y atenuada si se desconectó.

### 4.2 Emitir código

1. Escribe o pega el código en el editor, elige el **lenguaje** y, si quieres, un **título**.
2. Pulsa uno de los botones:
   - **Emitir a todos:** cada alumno ve solo este código (reemplaza lo anterior).
   - **Agregar bloque:** se agrega debajo de lo que ya tienen.
   - **Limpiar pantallas:** borra el código de todas las pantallas.
3. Debajo verás **"enviado a N · confirmados N/N ✓"** a medida que cada equipo lo muestra. En
   cada tarjeta aparece "✓ código recibido".

Cada emisión queda numerada en el **Historial** (Ejercicio 1, 2, …). **Reemitir** vuelve a
mostrar uno anterior con el mismo número. Con doble clic lo cargas en el editor.

### 4.3 Captura rápida mientras presentas

Estás en PowerPoint, en el editor o en el navegador con código a la vista:

1. Selecciona el código y cópialo (`Ctrl+C`).
2. Pulsa **Ctrl+Alt+Shift+C** sin cambiar de ventana.
3. Cuando vuelvas a la consola, el código ya está en el editor con el lenguaje propuesto. Revísalo
   y pulsa **Emitir a todos**.

**Capturar nunca envía nada por sí solo.** El botón azul **Capturar portapapeles** (abajo a la
derecha) hace lo mismo que el atajo. Si otra aplicación ya usa la combinación, la consola lo avisa
y el botón sigue funcionando.

### 4.4 Avisos

En **Aviso a todos** escribe el texto, elige **Información** o **Advertencia** y pulsa **Enviar
aviso**. Aparece en las pantallas de los alumnos hasta que pulsan **Entendido**, y en tu consola
ves cuántos lo confirmaron.

## 5. Lo que hace el alumno

El código aparece en una tarjeta con colores. **Copiar al portapapeles** lo copia exactamente
como lo enviaste, con la sangría, las líneas en blanco y los tabuladores, listo para pegar en
IDLE, VS Code o el Bloc de notas.

## 6. Reiniciar o cambiar de equipo

- **Reiniciar la consola** (cerrarla, reiniciar el equipo o cambiar de aula y volver): los
  alumnos se reconectan **solos** en unos segundos; no hay que reinstalar nada.
- **Otro equipo de profesor:** copia `%AppData%\Aula\consola\` al nuevo equipo (misma clave) o
  genera una configuración nueva (§3.1) y reinstálala en los alumnos.

## 7. Red y firewall

- Todos los equipos en la misma red local, con perfil **Privado** o **Dominio** (en "Pública"
  Windows bloquea el descubrimiento).
- Puertos: **TCP 47830** de entrada en el profesor y **UDP 5353** (mDNS) en todos. Los
  instaladores crean las reglas; para los portables, ver `docs/configuracion.md`.
- Si la red bloquea multicast (mDNS) y los alumnos se quedan en "buscando", genera la
  configuración con la dirección del profesor:
  `aula-consola config-alumno -canal AULA_101 -directo <IP del profesor>:47830`, y reinstálala.

## 8. Problemas frecuentes

| Síntoma | Qué hacer |
|---|---|
| El alumno dice "Aula — sin configurar" | Falta `aula-alumno.json`: ponlo junto al instalador y reinstala (§3). |
| El alumno se queda en "buscando la clase del canal…" | ¿La clase está abierta con **ese** canal? ¿Misma red y perfil Privado/Dominio? Si la red bloquea mDNS, usa `-directo` (§7). |
| El alumno dice "sin conexión… reintentando" | La consola se cerró o cambió de aula; vuelve en cuanto la abras. |
| En la consola no aparece un alumno que sí dice "buscando" | La clave de su configuración no es la de esta consola (otra consola u otro usuario de Windows). Genera de nuevo la configuración (§3.1) y reinstala. |
| "el puerto 47830 está ocupado" al abrir la clase | Ya hay otra consola abierta en ese equipo. Ciérrala. |
| "confirmados 1/2" no sube | Ese alumno no está mostrando el código: mira su tarjeta y su pantalla. |
| Los botones de emisión están grises | Lee el motivo junto a ellos: no hay alumnos o el editor está vacío. |

## 9. Privacidad

Aula es una herramienta de aula transparente: el alumno siempre ve a qué clase está conectado, y
el agente no tiene modo oculto. Las confirmaciones solo dicen que el código **se mostró**, no si
se leyó o se copió. Reglas completas en `docs/privacidad.md`.
