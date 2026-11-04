// Genera los diagramas de docs/v1/diagramas como SVG para la consola: una
// variante clara y una oscura por diagrama, limpias, y un manifiesto con el
// hash de cada bloque Mermaid. Los SVG se versionan: `go build` no necesita
// Node. Si cambias un bloque `mermaid` y no los regeneras, `go test ./docs`
// falla con «Diagramas desactualizados».
//   node generar-diagramas.mjs            (o generar-diagramas.sh / .ps1)
import crypto from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import puppeteer from "puppeteer";
import { renderMermaid } from "@mermaid-js/mermaid-cli";

const aqui = path.dirname(fileURLToPath(import.meta.url));
const raiz = path.resolve(aqui, "..", "..");
const dirDiagramas = path.join(raiz, "docs", "v1", "diagramas");
const dirSalida = path.join(dirDiagramas, "generados");
const version = JSON.parse(fs.readFileSync(path.join(aqui, "node_modules", "@mermaid-js", "mermaid-cli", "package.json"), "utf8")).version;

// Mermaid mide el texto al generar el SVG; el equipo que lo dibuja debe tener
// la misma letra o el texto se sale de sus cajas. Segoe UI viene en todo
// Windows (la plataforma principal, CLAUDE.md) y es la de la consola.
const letraPrincipal = "Segoe UI";
const letra = `"${letraPrincipal}", system-ui, sans-serif`;

// Los temas siguen los tokens de internal/ui/tokens.css (superficie, texto,
// bordes). El fondo es transparente: el visor pone el de la superficie.
const temas = {
  claro: {
    theme: "base",
    themeVariables: {
      fontFamily: letra, darkMode: false, background: "#ffffff",
      primaryColor: "#eef0f4", primaryTextColor: "#1a1f2b", primaryBorderColor: "#858fa0",
      secondaryColor: "#f4f5f7", tertiaryColor: "#ffffff", lineColor: "#4f5869", textColor: "#1a1f2b",
      noteBkgColor: "#fff7e6", noteTextColor: "#1a1f2b", noteBorderColor: "#c2410c",
      actorBkg: "#eef0f4", actorBorder: "#858fa0", actorTextColor: "#1a1f2b", signalColor: "#1a1f2b", signalTextColor: "#1a1f2b",
      labelBoxBkgColor: "#eef0f4", labelTextColor: "#1a1f2b", loopTextColor: "#1a1f2b",
    },
  },
  oscuro: {
    theme: "base",
    themeVariables: {
      fontFamily: letra, darkMode: true, background: "#1a1f29",
      primaryColor: "#232a37", primaryTextColor: "#e7eaf0", primaryBorderColor: "#6f7a8c",
      secondaryColor: "#1a1f29", tertiaryColor: "#12151c", lineColor: "#a7b0bf", textColor: "#e7eaf0",
      noteBkgColor: "#3a2a12", noteTextColor: "#e7eaf0", noteBorderColor: "#f3bc6a",
      actorBkg: "#232a37", actorBorder: "#6f7a8c", actorTextColor: "#e7eaf0", signalColor: "#e7eaf0", signalTextColor: "#e7eaf0",
      labelBoxBkgColor: "#232a37", labelTextColor: "#e7eaf0", loopTextColor: "#e7eaf0",
    },
  },
};

// Paleta oscura propia (FASE2-SISTEMA-DISENO.md): los `.md` traen los colores
// claros canónicos (docs/v1/diagramas/README.md) y aquí se cambian al dibujar.
const paletaOscura = [
  ["fill:#dbeafe,stroke:#1d4ed8,color:#111827", "fill:#172f52,stroke:#8db4ff,color:#e5edff"],
  ["fill:#dcfce7,stroke:#15803d,color:#111827", "fill:#123d27,stroke:#6fd697,color:#e3f7ea"],
  ["fill:#ffedd5,stroke:#c2410c,color:#111827", "fill:#44260f,stroke:#f3bc6a,color:#fdeedd"],
  ["fill:#fee2e2,stroke:#b91c1c,color:#111827", "fill:#4a1c1c,stroke:#ff9b91,color:#fde8e8"],
  // Fondos de box y rect (sequenceDiagram): más tenues que los nodos.
  ["rgb(219,234,254)", "rgb(25,39,62)"],
  ["rgb(220,252,231)", "rgb(22,46,35)"],
  ["rgb(255,237,213)", "rgb(52,36,20)"],
  ["rgb(254,226,226)", "rgb(56,28,28)"],
];

// Igual que internal/manual (parseDiagram y MermaidHash): el bloque va del
// "```mermaid\n" al siguiente "\n```", con saltos de línea \n.
function leerDiagrama(archivo) {
  const src = fs.readFileSync(archivo, "utf8").replaceAll("\r\n", "\n");
  const fm = {};
  const [, cabeza] = src.match(/^---\n([\s\S]*?)\n---\n/) || [];
  if (!cabeza) throw new Error(`${archivo}: sin front-matter`);
  for (const linea of cabeza.split("\n")) {
    const i = linea.indexOf(":");
    fm[linea.slice(0, i).trim()] = linea.slice(i + 1).trim().replace(/^"|"$/g, "");
  }
  const desde = src.indexOf("```mermaid\n");
  if (desde < 0) throw new Error(`${archivo}: sin bloque mermaid`);
  const resto = src.slice(desde + "```mermaid\n".length);
  const mermaid = resto.slice(0, resto.indexOf("\n```"));
  const lista = (v) => (v || "").replace(/^\[|\]$/g, "").split(",").map((s) => s.trim()).filter(Boolean);
  return { fm, mermaid, audiencia: lista(fm.audiencia), pantallas: lista(fm.pantallas) };
}

const hash = (texto) => "sha256:" + crypto.createHash("sha256").update(texto, "utf8").digest("hex");

// Las únicas URL que puede llevar un SVG limpio: los espacios de nombres.
const urlPermitida = /^https?:\/\/www\.w3\.org\//;

// limpiar deja solo dibujo: sin scripts, sin manejadores on*, sin
// foreignObject (htmlLabels:false lo evita; si aparece, es un error del
// diagrama) y sin referencias fuera del propio SVG. La consola además los
// muestra como <img>, donde un script nunca corre.
function limpiar(svg, id) {
  if (/<foreignObject/i.test(svg)) throw new Error(`${id}: el SVG trae foreignObject; ¿falta "htmlLabels": false en el init?`);
  let s = svg.replace(/<script[\s\S]*?<\/script>/gi, "");
  s = s.replace(/\s+on[a-z]+\s*=\s*("[^"]*"|'[^']*')/gi, "");
  s = s.replace(/\s+(xlink:)?href\s*=\s*("(?!#)[^"]*"|'(?!#)[^']*')/gi, "");
  s = s.replace(/@import[^;]*;/gi, "");
  s = s.replace(/url\(\s*(['"]?)(?!#)[^)]*\)/gi, "none");
  for (const m of s.matchAll(/https?:\/\/[^\s"'<>)]+/g)) {
    if (!urlPermitida.test(m[0])) throw new Error(`${id}: el SVG apunta a ${m[0]}`);
  }
  return s;
}

const archivos = fs.readdirSync(dirDiagramas).filter((f) => /^\d\d-[a-z0-9-]+\.md$/.test(f)).sort();
fs.mkdirSync(dirSalida, { recursive: true });
const navegador = await puppeteer.launch({ headless: true });

// Sin la letra, Chromium mide con otra (en algunos equipos, Times New Roman)
// y los SVG quedarían mal en la consola: mejor no generar.
async function comprobarLetra() {
  const p = await navegador.newPage();
  await p.setContent(`<p style='font-family:${letra}'>Aula</p>`);
  const cdp = await p.target().createCDPSession();
  await cdp.send("DOM.enable");
  await cdp.send("CSS.enable");
  const doc = await cdp.send("DOM.getDocument");
  const { nodeId } = await cdp.send("DOM.querySelector", { nodeId: doc.root.nodeId, selector: "p" });
  const { fonts } = await cdp.send("CSS.getPlatformFontsForNode", { nodeId });
  await p.close();
  if (!fonts.some((f) => f.familyName === letraPrincipal)) {
    await navegador.close();
    console.error(`No encuentro la letra «${letraPrincipal}» (Chromium usaría ${fonts.map((f) => f.familyName).join(", ")}).`);
    console.error("Genera los diagramas en un equipo con Windows.");
    process.exit(1);
  }
}
await comprobarLetra();
const manifiesto = { generador: `@mermaid-js/mermaid-cli ${version}`, diagramas: [] };
try {
  for (const f of archivos) {
    const id = f.replace(/\.md$/, "");
    const d = leerDiagrama(path.join(dirDiagramas, f));
    const entrada = {
      id, titulo: d.fm.titulo, descripcion: d.fm.descripcion, audiencia: d.audiencia, pantallas: d.pantallas,
      hash: hash(d.mermaid), alt: `${d.fm.titulo}: ${d.fm.descripcion}`,
    };
    for (const [nombre, config] of Object.entries(temas)) {
      let definicion = d.mermaid;
      if (nombre === "oscuro") for (const [de, a] of paletaOscura) definicion = definicion.replaceAll(de, a);
      const { data } = await renderMermaid(navegador, definicion, "svg", {
        backgroundColor: "transparent", svgId: `diagrama-${id.slice(0, 2)}-${nombre}`,
        mermaidConfig: { ...config, fontFamily: letra, securityLevel: "strict", handDrawnSeed: 1 }, // semilla fija: mismos SVG en cada corrida
      });
      const svg = limpiar(Buffer.from(data).toString("utf8"), id);
      const rel = `generados/${id}.${nombre}.svg`;
      fs.writeFileSync(path.join(dirDiagramas, rel), svg);
      entrada[nombre] = rel;
    }
    manifiesto.diagramas.push(entrada);
    console.log(`✓ ${id}`);
  }
} finally {
  await navegador.close();
}
// Sin SVG de diagramas que ya no existen.
const vigentes = new Set(manifiesto.diagramas.flatMap((e) => [path.basename(e.claro), path.basename(e.oscuro)]));
for (const f of fs.readdirSync(dirSalida)) {
  if (f.endsWith(".svg") && !vigentes.has(f)) fs.rmSync(path.join(dirSalida, f));
}
fs.writeFileSync(path.join(dirSalida, "manifiesto.json"), JSON.stringify(manifiesto, null, 2) + "\n");
console.log(`\n${manifiesto.diagramas.length} diagramas en ${path.relative(raiz, dirSalida)}`);
