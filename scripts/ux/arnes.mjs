// Arnés web de la consola del profesor: sirve apps/teacher/frontend/dist en
// 127.0.0.1, carga el doble de los bindings (dobles.js) y el manual real
// (exportado con TestExportManualForUX), y abre cada escena en Chromium
// headless. Wails no se puede manejar desde fuera, así que se prueba el mismo
// frontend en un navegador (prompt 18, Fase 7). Ver README.md.
import { execFileSync } from "node:child_process";
import fs from "node:fs";
import http from "node:http";
import os from "node:os";
import path from "node:path";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import puppeteer from "puppeteer";

const aqui = path.dirname(fileURLToPath(import.meta.url));
export const raiz = path.resolve(aqui, "..", "..");
const dist = path.join(raiz, "apps", "teacher", "frontend", "dist");
// El sistema de diseño vive en internal/ui y la consola lo sirve en sistema/ (ui.Assets).
const ui = path.join(raiz, "internal", "ui");
const require = createRequire(import.meta.url);
const axe = fs.readFileSync(require.resolve("axe-core/axe.min.js"), "utf8");
const dobles = fs.readFileSync(path.join(aqui, "dobles.js"), "utf8");

export const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
export const click = (sel) => async (p) => { await p.click(sel); await sleep(200); };
export const seq = (...fs) => async (p) => { for (const f of fs) await f(p); };
// Cuadros de miniatura falsos para los equipos dados.
export const cuadros = (hosts) => async (p) => p.evaluate((hs) => hs.forEach((h, i) => window.__emit("aula:frame",
  { sessionId: "s-" + h, seq: 1, width: 320, height: 180, jpeg: window.__thumb(h + " - Visual Studio Code", 210 + i * 60) })), hosts);

// El manual tal como lo devuelve Manual(), sacado del código Go.
function exportarManual() {
  const archivo = path.join(fs.mkdtempSync(path.join(os.tmpdir(), "aula-ux-")), "manual.json");
  execFileSync("go", ["test", "-count=1", "-run", "^TestExportManualForUX$", "./apps/teacher"],
    { cwd: raiz, env: { ...process.env, AULA_UX_MANUAL: archivo }, stdio: ["ignore", "ignore", "inherit"] });
  return fs.readFileSync(archivo, "utf8");
}

const tipos = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".ttf": "font/ttf", ".svg": "image/svg+xml" };

// abrir() levanta el servidor y el navegador. foto(archivo, escena, opciones)
// abre una página nueva (contexto aislado: el tema guardado no se cuela),
// ejecuta los pasos, guarda la captura y devuelve los errores de JS y las
// violaciones de axe.
export async function abrir() {
  const manual = exportarManual();
  const srv = http.createServer((q, r) => {
    const rel = decodeURIComponent(q.url === "/" ? "index.html" : q.url.split("?")[0].split("#")[0]).replace(/^\/+/, "");
    const [raizArchivo, relArchivo] = rel.startsWith("sistema/") ? [ui, rel.slice("sistema/".length)] : [dist, rel];
    const p = path.join(raizArchivo, relArchivo);
    if (!p.startsWith(raizArchivo + path.sep) || p.endsWith(".go")) { r.writeHead(403); r.end(); return; }
    fs.readFile(p, (e, d) => {
      if (e) { r.writeHead(404); r.end(); return; }
      r.writeHead(200, { "Content-Type": tipos[path.extname(p)] || "application/octet-stream" });
      r.end(d);
    });
  }).listen(0, "127.0.0.1");
  await new Promise((r) => srv.once("listening", r));
  const base = "http://127.0.0.1:" + srv.address().port + "/";
  const nav = await puppeteer.launch({ headless: true });

  async function foto(archivo, escena, { w = 1280, h = 760, oscuro = false, ruta = "", pasos = async () => {}, revisar = true } = {}) {
    const ctx = await nav.createBrowserContext();
    const p = await ctx.newPage();
    const errores = [];
    p.on("pageerror", (e) => errores.push("pageerror: " + e.message));
    p.on("console", (m) => { if (m.type() === "error") errores.push("console: " + m.text()); });
    await p.setViewport({ width: w, height: h });
    await p.emulateMediaFeatures([{ name: "prefers-color-scheme", value: oscuro ? "dark" : "light" }]);
    await p.evaluateOnNewDocument((s, d, m) => { window.__SCENE = s; window.__MANUAL = JSON.parse(m); (0, eval)(d); }, escena, dobles, manual);
    await p.goto(base + (ruta ? "#" + ruta : ""), { waitUntil: "networkidle0" });
    await sleep(700);
    await pasos(p);
    await p.mouse.move(0, 0); // sin hover a medio camino
    await sleep(300);
    fs.mkdirSync(path.dirname(archivo), { recursive: true });
    await p.screenshot({ path: archivo });
    let violaciones = [];
    if (revisar) {
      await p.evaluate(axe);
      violaciones = await p.evaluate(async () => (await window.axe.run(document, { runOnly: ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa", "best-practice"] }))
        .violations.map((x) => ({ impacto: x.impact, texto: x.impact + " " + x.id + " (" + x.nodes.length + "): " + x.nodes.slice(0, 3).map((n) => n.target.join(" ")).join(" ; ") })));
    }
    await ctx.close();
    return { errores, violaciones };
  }

  return { foto, cerrar: async () => { await nav.close(); srv.close(); } };
}
