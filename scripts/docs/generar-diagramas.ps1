# Regenera los SVG de docs/v1/diagramas/generados (claro y oscuro) y su
# manifiesto. Requiere Node 18+ y la letra Segoe UI. Ver
# docs/v1/diagramas/README.md.
$ErrorActionPreference = 'Stop'
Push-Location $PSScriptRoot
try {
    if (-not (Test-Path node_modules)) {
        npm ci --no-audit --no-fund
        if ($LASTEXITCODE -ne 0) { throw "npm ci falló" }
    }
    node generar-diagramas.mjs
    if ($LASTEXITCODE -ne 0) { throw "generar-diagramas falló" }
} finally {
    Pop-Location
}
