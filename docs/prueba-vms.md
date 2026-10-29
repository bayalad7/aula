# Prueba en VMs: etapa 04 (descubrimiento y registro)

Objetivo: en la consola del profesor, un alumno **aparece** en el registro al arrancar y
**desaparece** (pasa a `desconectado`) al cerrarlo.

## 1. Qué hay que instalar

**Nada en las VMs** si usas el ejecutable compilado. Solo tu equipo de desarrollo necesita Go.

En tu equipo (`C:\aula`), compila el ejemplo una vez para cada sistema operativo de tus VMs:

```powershell
# Windows (VMs con Windows 10/11)
$env:CGO_ENABLED='0'; go build -o dist\dos_alumnos.exe ./examples/dos_alumnos

# Linux (si alguna VM es Linux)
$env:CGO_ENABLED='0'; $env:GOOS='linux'; go build -o dist/dos_alumnos ./examples/dos_alumnos; Remove-Item Env:GOOS
```

Copia el archivo a cada VM (carpeta compartida, USB o arrastrar y soltar). `dist/` está en
`.gitignore` porque son binarios, así que no se sube al repositorio.

> `go run ./examples/dos_alumnos` también funciona, pero solo en una VM con Go instalado y el
> repositorio clonado. Para la prueba es más simple copiar el `.exe`.

## 2. Red de las VMs

Las VMs tienen que estar **en la misma red local** para que mDNS (multicast) las encuentre:

- VirtualBox: adaptador en **Adaptador puente** (bridged) o en la misma **Red interna**.
  En modo **NAT** las VMs no se ven entre sí.
- Hyper-V: un **conmutador externo** o el mismo conmutador interno para todas.
- VMware: **Bridged**, o todas en el mismo segmento LAN.

Comprueba que se ven: desde la VM del alumno, `ping <IP de la VM del profesor>`.
La IP sale con `ipconfig` (Windows) o `ip a` (Linux).

## 3. Ejecutar

**VM del profesor** (abre una terminal en la carpeta del `.exe`):

```powershell
.\dos_alumnos.exe -rol profesor
```

Desde la etapa 05 la conexión va cifrada (TLS) y el alumno solo acepta a la consola cuya clave
conoce. Al arrancar, el profesor imprime su **clave pública**; si no había clave, la crea. Copia
la línea completa:

```
Clave pública para los alumnos: aula-ed25519:I5rPjgu2TFt5s5eDGz7sJst4FfG9v+bZIOwFmYuFvf0=
```

La primera vez, Windows pregunta por el **firewall**: marca *Redes privadas* y pulsa
*Permitir acceso*. Si no aparece el aviso o la red es "Pública", abre los puertos a mano
(PowerShell como administrador):

```powershell
New-NetFirewallRule -DisplayName "Aula consola (TCP 47830)" -Direction Inbound -Protocol TCP -LocalPort 47830 -Action Allow
New-NetFirewallRule -DisplayName "Aula mDNS (UDP 5353)" -Direction Inbound -Protocol UDP -LocalPort 5353 -Action Allow
```

**Cada VM de alumno:**

```powershell
.\dos_alumnos.exe -rol alumno -clave aula-ed25519:<la clave que imprimió el profesor>
```

En Linux: `chmod +x dos_alumnos && ./dos_alumnos -rol alumno -clave aula-ed25519:...`.

La clave no es secreta (es la parte pública); puedes pegarla en un archivo de texto compartido.

## 4. Qué deberías ver

En la VM del alumno:

```
[10:00:01] PC-ALUMNO: buscando consola en AULA_101
[10:00:02] PC-ALUMNO: conectado (Consola de ejemplo)
```

En la VM del profesor, el registro se imprime cada vez que cambia:

```
[10:00:02] Registro AULA_101: 1 equipo(s)
  PC-ALUMNO      alumno       windows  conectado     192.168.1.23:50312
```

Después **cierra el alumno** con `Ctrl+C`. En el profesor la línea cambia a `desconectado`
al instante.

Para probar el timeout de 15 s, **pausa o suspende la VM del alumno** en lugar de cerrarlo. A
los ~15 s el profesor lo marca `desconectado`. Al reanudarla, el alumno reconecta solo, con
un retroceso de hasta 30 s.

## 5. Si algo falla

| Síntoma | Qué probar |
|---|---|
| El alumno se queda en "buscando consola" | mDNS bloqueado. Conecta directo: `.\dos_alumnos.exe -rol alumno -clave aula-ed25519:... -directo <IP del profesor>:47830`. Si así funciona, el problema es el multicast de la red o la VM (revisa el modo de red, §2). |
| Ni con `-directo` conecta | Firewall del profesor (§3) o las VMs no se ven (`ping`). |
| El alumno muestra "console rejected … signature does not verify" (con `-v`) | La clave `-clave` no es la de esa consola. Cópiala otra vez de la salida del profesor. Esto también es una prueba de seguridad: con una clave inventada **no** debe conectarse. |
| Quieres más detalle | Agrega `-v` en ambos lados para ver los logs. |
| Varias aulas a la vez | Usa el mismo `-canal` en profesor y alumnos, p. ej. `-canal AULA_102`. Un alumno de otro canal no debe aparecer. |
| Puerto ocupado | `-puerto 47900` en el profesor (y en `-directo` si lo usas). |

## 6. Al terminar

Anota el resultado en `docs/progreso.md` (fila de la etapa 04) y quita
"(falta prueba en VMs)" si salió bien.

## 7. Etapa 06: código de punta a punta

Además de `dos_alumnos.exe`, copia a la VM del alumno la app real `aula-cliente.exe`
(`cd apps/student && wails build`; queda en `apps/student/build/bin/`).

1. **Profesor:** `.\dos_alumnos.exe -rol profesor`. Copia la clave pública que imprime.
2. **Alumno:** abre una terminal en la carpeta del `.exe` y ejecuta
   `.\aula-cliente.exe -canal AULA_101 -clave aula-ed25519:...` (agrega
   `-directo <IP del profesor>:47830` si mDNS no funciona). Arriba debe decir
   "Aula — conectado a la clase de Consola de ejemplo (canal AULA_101)".
3. **Profesor:** escribe en su terminal `codigo C:\ruta\a\un_archivo.py Ejercicio 1`
   (también `agregar`, `limpiar`, `aviso <texto>`, `alerta <texto>`; `ayuda` los lista).
4. **Alumno:** la ventana pasa al frente con el código resaltado. Pulsa **Copiar al
   portapapeles** y pega en su editor (IDLE, VS Code, Bloc de notas). La sangría y los saltos de
   línea deben quedar idénticos; en Python, el programa debe correr sin `IndentationError`.
5. **Profesor:** en el registro aparece `código xxxxxxxx ✓` junto al equipo del alumno.

Para no escribir los argumentos cada vez, crea `%AppData%\Aula\alumno\config.json`:

```json
{ "channel": "AULA_101", "console_public_key": "aula-ed25519:..." }
```
