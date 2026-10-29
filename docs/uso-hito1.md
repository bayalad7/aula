# Guía de uso — distribuir código en clase

Con Aula, el profesor envía código desde su equipo y cada alumno lo ve en su pantalla, resaltado
y listo para **copiar y pegar intacto** con un clic. Funciona como LanSchool: cada computadora de
alumno pertenece a un **canal**, que es el número de su sala, y la consola del profesor muestra a
los alumnos del canal en el que está.

| Pieza | Va en | Archivo |
|---|---|---|
| **Aula Consola** | Cada equipo de profesor | `Aula-Consola-Setup-X.Y.Z.exe` |
| **Aula Cliente** | Cada equipo de alumno | `Aula-Alumno-Setup-X.Y.Z.exe` + `aula-centro.pub` |

Descárgalos de la página de versiones del repositorio (*Releases*). Requisitos: Windows 10/11 de
64 bits con WebView2 (incluido en Windows 11 y en Windows 10 actualizado), y todos los equipos en
la **misma red local** con perfil **Privado** o **Dominio**.

## Conceptos

- **Canal:** número del 1 al 16000 que agrupa salas y maestros. Por ejemplo, en un piso con dos
  salas:
  - centro de cómputo principal (40 alumnos) = **canal 1**;
  - centro de cómputo alterno (12 alumnos) = **canal 2**.

  Cada computadora de alumno se instala con el canal de su sala, y solo un administrador lo
  cambia. El profesor puede cambiar de canal en su consola cuando quiera.
- **Clave del centro:** una sola clave para todo el centro de cómputo. Las consolas la tienen y
  los alumnos la reconocen. Así, un alumno que instale la consola por su cuenta no controla a
  nadie. Se crea **una vez**, al instalar la primera consola.

---

## 1. Instalar la primera consola (una vez por centro)

En el equipo del profesor, como administrador, ejecuta `Aula-Consola-Setup-X.Y.Z.exe`. Si Windows
muestra "Windows protegió tu PC", pulsa *Más información → Ejecutar de todas formas*.

1. **Canal predeterminado:** el número de la sala de ese equipo, por ejemplo `1`. La consola
   arrancará en él.
2. **Clave del centro:** elige **"Crear una clave nueva"** y la carpeta donde guardar los archivos.
   Por defecto es `Documentos\Aula`. Al terminar quedan dos archivos:
   - `aula-centro.pub`: para instalar los alumnos. No es secreto.
   - `aula-centro.key`: **secreto**, solo para instalar otras consolas. Guárdalo en un lugar
     seguro.
3. El instalador crea el acceso "Aula Consola" en el menú Inicio y la regla de firewall
   **"Aula Consola"**, para que Windows no pregunte en plena clase.

## 2. Instalar más consolas (otras salas u otros profesores)

Ejecuta el mismo instalador y elige:

1. el **canal predeterminado** de esa sala, por ejemplo `2`;
2. **"Importar la clave del centro"** y selecciona `aula-centro.key`. Después **borra el archivo**
   de la USB o carpeta compartida.

Todas las consolas del centro comparten la clave: cualquier maestro puede dar clase en cualquier
sala.

## 3. Instalar los equipos de alumno

Pon `aula-centro.pub` en la **misma carpeta** que `Aula-Alumno-Setup-X.Y.Z.exe`, por ejemplo una
carpeta compartida o una memoria USB.

**Uno por uno:** ejecuta el instalador. En la página **"Canal y clave del centro"**:

- **Canal de esta computadora:** el número de su sala (`1` en la principal, `2` en la alterna…);
- **Clave pública del centro:** ya viene llena con lo que dice `aula-centro.pub`.

El instalador:

- copia Aula Cliente a `C:\Program Files\Aula\Cliente\`;
- guarda el canal y la clave en `C:\ProgramData\Aula\alumno\config.json`, protegido para que
  solo un administrador lo cambie;
- deja Aula Cliente **arrancando al iniciar sesión** cualquier usuario del equipo;
- crea la regla de firewall **"Aula Cliente (mDNS)"**.

**En silencio, en muchos equipos** (script de inicio, GPO o herramienta de despliegue, como
administrador). Una línea por sala:

```bat
\\servidor\aula\Aula-Alumno-Setup-X.Y.Z.exe /VERYSILENT /SUPPRESSMSGBOXES /NORESTART /CANAL=1 /CLAVECENTRO=\\servidor\aula\aula-centro.pub
```

**Cambiar una computadora de sala:** vuelve a ejecutar el instalador con el canal nuevo
(`/CANAL=2`). En unos segundos se conecta al canal nuevo, sin reiniciar.

**Actualizar a una versión nueva:** ejecuta el instalador nuevo del mismo modo, incluso sin
`/CANAL`. Conserva el canal y la clave.

## 4. En clase

### 4.1 La consola arranca sola en su canal

Al abrir **Aula Consola** ya está escuchando en su canal predeterminado. A la izquierda aparece
**una tarjeta por cada alumno** de ese canal, en vivo: verde si está conectado y atenuada si se
desconectó.

En la barra superior:

- **Canal [ 1 ] Cambiar:** escribe otro número y pulsa *Cambiar* para ver y dar clase a los
  alumnos de ese canal.
- **Tu nombre:** lo que ven los alumnos ("conectado a la clase de …"). Cámbialo y pulsa
  *Guardar*.
- **Clave del centro: XXXX-XXXX-XXXX-XXXX:** si en su lugar dice "Clave personal", ese equipo no
  tiene la clave del centro (ver §2).

### 4.2 Emitir código

1. Escribe o pega el código en el editor, elige el **lenguaje** y, si quieres, un **título**.
2. Pulsa uno de los botones:
   - **Emitir a todos:** cada alumno ve solo este código.
   - **Agregar bloque:** se agrega debajo de lo que ya tienen.
   - **Limpiar pantallas:** borra el código de todas las pantallas.
3. Debajo verás **"enviado a N · confirmados N/N ✓"** a medida que cada equipo lo muestra.

Cada emisión queda en el **Historial** (Ejercicio 1, 2, …), con **Reemitir** para volver a
mostrar uno anterior.

### 4.3 Captura rápida mientras presentas

En PowerPoint, el editor o el navegador:

1. selecciona el código y cópialo (`Ctrl+C`);
2. pulsa **Ctrl+Alt+Shift+C**;
3. vuelve a la consola: el código ya está en el editor con el lenguaje propuesto. Revísalo y
   pulsa **Emitir a todos**.

**Capturar nunca envía nada por sí solo.** El botón **Capturar portapapeles** hace lo mismo.

### 4.4 Avisos

En **Aviso a todos** escribe el texto, elige **Información** o **Advertencia** y pulsa **Enviar
aviso**.

## 5. Lo que ve el alumno

Arriba, siempre visible:

> **Aula — conectado a la clase de Prof. Ayala (canal 1)**

El código aparece en una tarjeta con colores. **Copiar al portapapeles** lo copia exactamente
como lo enviaste, con la sangría, las líneas en blanco y los tabuladores. El agente nunca corre
oculto: si el alumno cierra la ventana, se desconecta y su tarjeta queda atenuada en la consola.

## 6. Si un alumno no se conecta

La ventana del alumno explica el motivo bajo "buscando la clase del canal N…":

| Motivo que muestra | Qué hacer |
|---|---|
| "No se encontró ninguna consola del canal N" | ¿Hay una consola abierta **en ese canal**? ¿La red de ese equipo es Privada o de Dominio (no Pública)? ¿Ambos en la misma red? |
| "Se encontró la consola «…», pero su clave no es la de este centro" | Esa consola no tiene la clave del centro (dice "Clave personal"): impórtala (§2). O el alumno se instaló con otra `aula-centro.pub`. |
| "… pero no se pudo conectar a …" | Firewall del equipo del profesor (regla "Aula Consola") o redes distintas. |
| "Aula — sin configurar" | Falta el canal o la clave: reinstala con `aula-centro.pub` junto al instalador. |

Para un diagnóstico completo, en el equipo del alumno:

```bat
"C:\Program Files\Aula\Cliente\aula-cliente.exe" diagnostico
```

Los registros quedan en `%LOCALAPPDATA%\Aula\alumno\aula-cliente.log` y
`%LOCALAPPDATA%\Aula\consola\aula-consola.log`.

## 7. Red y firewall

- Puertos: **TCP 47830** de entrada en los profesores y **UDP 5353** (mDNS) en todos. Los
  instaladores crean las reglas.
- Si la red bloquea multicast y los alumnos nunca encuentran la consola, se puede fijar la
  dirección del profesor al instalar el alumno: `/DIRECTO=192.168.1.10:47830`.

## 8. Privacidad

Aula es una herramienta de aula transparente: el alumno siempre ve a qué clase y canal está
conectado, y el agente no tiene modo oculto. Las confirmaciones solo dicen que el código **se
mostró**, no si se leyó o se copió. Reglas completas en `docs/privacidad.md`.
