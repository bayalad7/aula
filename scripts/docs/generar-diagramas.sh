#!/usr/bin/env sh
# Regenera los SVG de docs/v1/diagramas/generados (claro y oscuro) y su
# manifiesto. Requiere Node 18+ y la letra Segoe UI (Windows). Ver
# docs/v1/diagramas/README.md.
set -eu
cd "$(dirname "$0")"
if [ ! -d node_modules ]; then
  npm ci --no-audit --no-fund
fi
node generar-diagramas.mjs
