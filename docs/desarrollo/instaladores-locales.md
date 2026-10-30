# Instaladores de prueba en local

Para probar en VMs o en el laboratorio lo que está en `develop` **sin publicar una versión**.
Genera los mismos instaladores que el workflow de release (`.github/workflows/release.yml`),
pero en tu equipo, sin firma y sin subir nada. Para publicar una versión, ver `publicar.md`.

## Qué necesitas (una sola vez)

- Go 1.26 o posterior y la CLI de Wails v2 (`wails doctor` debe terminar en *SUCCESS*; ver
  "Entorno de desarrollo" en `docs/progreso.md`).
- **Inno Setup 6**: `winget install JRSoftware.InnoSetup`. Se instala en
  `%LOCALAPPDATA%\Programs\Inno Setup 6` (por usuario) o en `C:\Program Files (x86)\Inno Setup 6`.
- No hace falta administrador para compilar. Solo para **instalar** el resultado.

## Compilar

En PowerShell, desde la raíz del repositorio (`C:\aula`). Cambia `$version` a lo que quieras ver
en las apps (`aula-consola version`, la barra del alumno y el `hello`); usa un sufijo `-dev` para
no confundirlo con una versión publicada.

```powershell
$version = "0.6.0-dev"                  # lo que reportan las apps
$numeric = ($version -split "-")[0]      # metadatos del .exe: solo números
$out     = "dist\local"                  # dist/ está en .gitignore

# 1. Las dos apps, como en el release (sin cgo, rutas recortadas, versión embebida)
Remove-Item -Recurse -Force $out -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force "$out\bin" | Out-Null
foreach ($app in "student", "teacher") {
    Push-Location "apps\$app"
    wails build -clean -trimpath -ldflags "-X main.version=$version"
    Pop-Location
    if ($LASTEXITCODE -ne 0) { throw "wails build de $app falló" }
}
Copy-Item apps\teacher\build\bin\aula-consola.exe, apps\student\build\bin\aula-cliente.exe "$out\bin"

# 2. Comprobar la versión que quedó dentro
& "$out\bin\aula-consola.exe" version      # aula-consola 0.6.0-dev
& "$out\bin\aula-cliente.exe" -version     # aula-cliente 0.6.0-dev

# 3. Los instaladores (Inno Setup)
$iscc = @("$env:LOCALAPPDATA\Programs\Inno Setup 6\ISCC.exe",
          "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe") | Where-Object { Test-Path $_ } | Select-Object -First 1
if (-not $iscc) { throw "No se encontró ISCC.exe: instala Inno Setup 6" }
Push-Location empaquetado\windows
foreach ($iss in "aula-alumno.iss", "aula-consola.iss") {
    & $iscc /Q "/DAppVersion=$version" "/DNumericVersion=$numeric" "/DSourceDir=..\..\$out\bin" "/O..\..\$out" $iss
    if ($LASTEXITCODE -ne 0) { Pop-Location; throw "ISCC $iss falló" }
}
Pop-Location
Get-ChildItem $out -Filter *.exe | Select-Object Name, @{n="MB"; e={[math]::Round($_.Length/1MB, 1)}}
```

Tarda entre 1 y 2 minutos. Resultado en `dist\local`:

| Archivo | Uso |
|---|---|
| `Aula-Consola-Setup-<versión>.exe` | Equipo o VM del profesor |
| `Aula-Alumno-Setup-<versión>.exe` | Equipos o VMs de alumno (instala el servicio `AulaAgente`) |
| `bin\aula-consola.exe`, `bin\aula-cliente.exe` | Portables, para probar sin instalar |

Compila lo que tengas en el árbol de trabajo, con o sin commit. Para probar exactamente un
commit, haz antes `git status` (limpio) y anota el hash (`git log --oneline -1`) junto al
resultado de la prueba.

## Instalar en las VMs

1. Copia los dos setups a las VMs (carpeta compartida, USB o `Copy-Item` a `\\vm\c$`).
2. **Profesor:** ejecuta `Aula-Consola-Setup-…` como administrador. Elige el canal y crea la clave
   del centro (o importa `aula-centro.key`). Deja `aula-centro.pub` en la carpeta que elijas.
3. **Alumnos:** pon `aula-centro.pub` **junto al** `Aula-Alumno-Setup-…` y ejecútalo como
   administrador; pide el canal. En silencio (como lo haría una GPO):

   ```bat
   Aula-Alumno-Setup-0.4.0-dev.exe /VERYSILENT /SUPPRESSMSGBOXES /NORESTART /CANAL=1
   ```

4. Si la VM ya tenía otra versión, el instalador la actualiza: detiene el agente, reemplaza el
   ejecutable y deja el servicio en marcha.

Los setups locales **no van firmados**: Windows puede mostrar el aviso de SmartScreen ("Windows
protegió su PC") → **Más información** → **Ejecutar de todas formas**. Los de un release sí se
pueden verificar (`publicar.md` §5).

## Después de probar

- Casos y registro: `docs/prueba-vms.md` (§11 miniaturas, §12 transmisión, §13 servicio).
- Logs si algo falla:
  - servicio del alumno: `C:\ProgramData\Aula\alumno\logs\aula-agente.log`;
  - ventana del alumno: `%LOCALAPPDATA%\Aula\alumno\aula-cliente.log`;
  - consola: `%LOCALAPPDATA%\Aula\consola\aula-consola.log`;
  - y en el alumno, `aula-cliente.exe diagnostico` en `cmd`.
- Para dejar una VM limpia: desinstala "Aula Cliente" y "Aula Consola" desde Configuración →
  Aplicaciones. El servicio se borra con el alumno (`sc query AulaAgente` debe decir que no
  existe). Para quitar los instaladores locales de tu equipo, borra `dist\local`.
- Cuando la prueba salga bien y quieras llevarlo al laboratorio, publica una prerelease
  (`vX.Y.Z-rc.N`) con `publicar.md`.
