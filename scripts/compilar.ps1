<#
.SYNOPSIS
    Compila Aula en local: una sola salida, dist\local (windows y linux).

.DESCRIPTION
    Hace lo mismo que el workflow de release (.github/workflows/release.yml), pero en este equipo,
    sin firma y sin subir nada. Explicación de cada ejecutable: docs/desarrollo/ejecutables.md.

    Sin -Solo: las dos apps y los dos instaladores (necesita Inno Setup 6).
      dist\local\windows\bin\aula-consola.exe        consola portable
      dist\local\windows\bin\aula-cliente.exe        app del alumno portable
      dist\local\windows\Aula-Consola-Setup-<v>.exe  instalador del profesor
      dist\local\windows\Aula-Alumno-Setup-<v>.exe   instalador del alumno (servicio AulaAgente)

    -Solo consola | alumno: solo esa app en dist\local\windows\bin, sin instaladores ni Inno Setup.

    -Linux: además, los paquetes de Linux (.deb, .rpm y AppImage de la consola) en
    dist\local\linux. No se compilan aquí (Wails en Linux necesita cgo y WebKitGTK): se bajan con
    gh de la última corrida verde del CI en develop, así que son del último push, no de cambios
    sin subir. -Solo linux hace solo eso.

    apps\*\build\bin es donde Wails deja su compilación (no se puede cambiar); es un paso
    intermedio y no se ejecuta nada desde ahí.

.PARAMETER Solo
    consola o alumno: compila solo esa app, sin instaladores. linux: solo baja los paquetes de
    Linux del CI.

.PARAMETER Linux
    También baja los paquetes de Linux del CI a dist\local\linux (necesita gh con sesión).

.PARAMETER Version
    Lo que reportan las apps (version, barra del alumno, hello). Por defecto, la versión en
    preparación (docs/ux/notas/VERSION) con el sufijo -dev.

.PARAMETER Abrir
    Con -Solo consola: abre la consola al terminar.

.EXAMPLE
    .\scripts\compilar.ps1 -Solo consola -Abrir

.EXAMPLE
    .\scripts\compilar.ps1 -Version 0.2.0-dev

.EXAMPLE
    .\scripts\compilar.ps1 -Linux
#>
[CmdletBinding()]
param(
    [ValidateSet("consola", "alumno", "linux")]
    [string]$Solo,
    [string]$Version,
    [switch]$Abrir,
    [switch]$Linux
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$out  = Join-Path $root "dist\local"
$win  = Join-Path $out "windows"
$bin  = Join-Path $win "bin"

# Linux packages come from the CI (job «Paquetes de Linux», artifact paquetes-linux): they
# cannot be built on Windows. The latest green run of develop that still has them (7 days).
function Get-LinuxPackages {
    if (-not (Get-Command gh -ErrorAction SilentlyContinue)) {
        throw "No se encontró gh (GitHub CLI): winget install GitHub.cli y luego gh auth login"
    }
    $dest = Join-Path $out "linux"
    $runs = gh run list --workflow CI --branch develop --status success --limit 10 --json databaseId,headSha,createdAt | ConvertFrom-Json
    foreach ($run in $runs) {
        Remove-Item -Recurse -Force $dest -ErrorAction SilentlyContinue
        # A run older than the artifact (or past its 7 days) has none: gh writes to stderr, which
        # Windows PowerShell 5.1 turns into an error record; only the exit code decides.
        $ErrorActionPreference = "Continue"
        try { gh run download $run.databaseId -n paquetes-linux -D $dest 2>&1 | Out-Null }
        finally { $ErrorActionPreference = "Stop" }
        if ($LASTEXITCODE -eq 0 -and (Get-ChildItem $dest -Filter *.deb -ErrorAction SilentlyContinue)) {
            $sha = $run.headSha.Substring(0, 7)
            Write-Host "== Paquetes de Linux del CI (commit $sha, $($run.createdAt)) en $dest"
            $head = (git -C $root rev-parse --short=7 HEAD).Trim()
            if ($head -ne $sha) {
                Write-Warning "Son del commit $sha y tu código está en ${head}: lo que no hayas subido no está en ellos."
            } elseif (git -C $root status --porcelain) {
                Write-Warning "Son de tu commit $sha, sin tus cambios que aún no tienen commit."
            }
            return
        }
    }
    throw "Ninguna corrida verde reciente del CI en develop tiene el artefacto paquetes-linux (dura 7 días): haz push a develop y espera el CI."
}

if ($Solo -eq "linux") {
    New-Item -ItemType Directory -Force $out | Out-Null
    Get-LinuxPackages
    Get-ChildItem (Join-Path $out "linux") | Select-Object Name, @{ n = "MB"; e = { [math]::Round($_.Length / 1MB, 1) } } | Format-Table -AutoSize
    return
}

# app folder -> Wails outputfilename (apps/*/wails.json)
$apps = [ordered]@{ teacher = "aula-consola"; student = "aula-cliente" }
if ($Solo -eq "consola") { $apps = [ordered]@{ teacher = "aula-consola" } }
if ($Solo -eq "alumno")  { $apps = [ordered]@{ student = "aula-cliente" } }

if (-not $Version) {
    $prep = "0.1.0"
    $versionFile = Join-Path $root "docs\ux\notas\VERSION"
    if (Test-Path $versionFile) { $prep = (Get-Content $versionFile -TotalCount 1).Trim().TrimStart("v") }
    $Version = "$prep-dev"
}
if ($Version -notmatch '^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$') {
    throw "La versión «$Version» no es X.Y.Z ni X.Y.Z-algo"
}
$numeric = ($Version -split "-")[0]   # Windows file versions are numeric only

if (-not (Get-Command wails -ErrorAction SilentlyContinue)) {
    throw "No se encontró la CLI de Wails: go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.1"
}

# Full build starts from an empty dist\local\windows so no installer from another version stays
# behind (dist\local\linux is left alone). Also the layout before dist\local\windows existed.
if (-not $Solo) {
    Remove-Item -Recurse -Force $win, (Join-Path $out "bin") -ErrorAction SilentlyContinue
    Remove-Item -Force (Join-Path $out "*.exe") -ErrorAction SilentlyContinue
}
New-Item -ItemType Directory -Force $bin | Out-Null

# 1. The apps, as in the release: no cgo, trimmed paths, embedded version.
$prevCgo = $env:CGO_ENABLED
$env:CGO_ENABLED = "0"
try {
    foreach ($app in $apps.Keys) {
        Write-Host "== Compilando $($apps[$app]) $Version"
        Push-Location (Join-Path $root "apps\$app")
        # Wails logs to stderr while it generates the bindings; in Windows PowerShell 5.1 that
        # becomes an error record, so only the exit code decides.
        $ErrorActionPreference = "Continue"
        try {
            wails build -clean -trimpath -ldflags "-X main.version=$Version" 2>&1 | ForEach-Object { "$_" } | Out-Host
            if ($LASTEXITCODE -ne 0) { throw "wails build de apps\$app falló" }
        } finally { $ErrorActionPreference = "Stop"; Pop-Location }
        Copy-Item (Join-Path $root "apps\$app\build\bin\$($apps[$app]).exe") $bin -Force
    }
} finally {
    $env:CGO_ENABLED = $prevCgo
}

# 2. Check the version that ended up inside.
if ($apps.Contains("teacher")) { & (Join-Path $bin "aula-consola.exe") version }
if ($apps.Contains("student")) { & (Join-Path $bin "aula-cliente.exe") -version }

# 3. The installers (Inno Setup), only on a full build.
if (-not $Solo) {
    $iscc = @("$env:LOCALAPPDATA\Programs\Inno Setup 6\ISCC.exe",
              "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe") |
        Where-Object { Test-Path $_ } | Select-Object -First 1
    if (-not $iscc) { throw "No se encontró ISCC.exe: winget install JRSoftware.InnoSetup" }
    Push-Location (Join-Path $root "empaquetado\windows")
    try {
        foreach ($iss in "aula-alumno.iss", "aula-consola.iss") {
            & $iscc /Q "/DAppVersion=$Version" "/DNumericVersion=$numeric" "/DSourceDir=$bin" "/O$win" $iss
            if ($LASTEXITCODE -ne 0) { throw "ISCC $iss falló" }
        }
    } finally { Pop-Location }
}

if ($Linux) { Get-LinuxPackages }

Write-Host ""
Write-Host "Listo en $out"
Get-ChildItem $out -Recurse -Include *.exe, *.deb, *.rpm, *.AppImage |
    Select-Object @{ n = "Archivo"; e = { $_.FullName.Substring($out.Length + 1) } },
                  @{ n = "MB"; e = { [math]::Round($_.Length / 1MB, 1) } } |
    Format-Table -AutoSize

if ($Abrir) {
    if ($Solo -eq "consola") { & (Join-Path $bin "aula-consola.exe") }
    else { Write-Warning "-Abrir solo aplica con -Solo consola" }
}
