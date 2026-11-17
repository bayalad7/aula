<#
.SYNOPSIS
    Compila Aula en local: una sola salida, dist\local.

.DESCRIPTION
    Hace lo mismo que el workflow de release (.github/workflows/release.yml), pero en este equipo,
    sin firma y sin subir nada. Explicación de cada ejecutable: docs/desarrollo/ejecutables.md.

    Sin -Solo: las dos apps y los dos instaladores (necesita Inno Setup 6).
      dist\local\bin\aula-consola.exe        consola portable
      dist\local\bin\aula-cliente.exe        app del alumno portable
      dist\local\Aula-Consola-Setup-<v>.exe  instalador del profesor
      dist\local\Aula-Alumno-Setup-<v>.exe   instalador del alumno (servicio AulaAgente)

    -Solo consola | alumno: solo esa app en dist\local\bin, sin instaladores ni Inno Setup.

    apps\*\build\bin es donde Wails deja su compilación (no se puede cambiar); es un paso
    intermedio y no se ejecuta nada desde ahí.

.PARAMETER Solo
    consola o alumno: compila solo esa app, sin instaladores.

.PARAMETER Version
    Lo que reportan las apps (version, barra del alumno, hello). Por defecto, la versión en
    preparación (docs/ux/notas/VERSION) con el sufijo -dev.

.PARAMETER Abrir
    Con -Solo consola: abre la consola al terminar.

.EXAMPLE
    .\scripts\compilar.ps1 -Solo consola -Abrir

.EXAMPLE
    .\scripts\compilar.ps1 -Version 0.2.0-dev
#>
[CmdletBinding()]
param(
    [ValidateSet("consola", "alumno")]
    [string]$Solo,
    [string]$Version,
    [switch]$Abrir
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$out  = Join-Path $root "dist\local"
$bin  = Join-Path $out "bin"

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
    throw "No se encontró la CLI de Wails: go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.0"
}

# Full build starts from an empty dist\local so no installer from another version stays behind.
if (-not $Solo) { Remove-Item -Recurse -Force $out -ErrorAction SilentlyContinue }
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
            & $iscc /Q "/DAppVersion=$Version" "/DNumericVersion=$numeric" "/DSourceDir=$bin" "/O$out" $iss
            if ($LASTEXITCODE -ne 0) { throw "ISCC $iss falló" }
        }
    } finally { Pop-Location }
}

Write-Host ""
Write-Host "Listo en $out"
Get-ChildItem $out -Recurse -Filter *.exe |
    Select-Object @{ n = "Archivo"; e = { $_.FullName.Substring($out.Length + 1) } },
                  @{ n = "MB"; e = { [math]::Round($_.Length / 1MB, 1) } } |
    Format-Table -AutoSize

if ($Abrir) {
    if ($Solo -eq "consola") { & (Join-Path $bin "aula-consola.exe") }
    else { Write-Warning "-Abrir solo aplica con -Solo consola" }
}
