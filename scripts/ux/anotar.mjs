// Anotador de interfaz: abre la consola o la app del alumno, con datos de demostración, en una
// ventana de Chromium que manejas tú. Cada clic deja seleccionado lo que tocaste, con su cadena de
// zonas (zonas.mjs); N abre la nota y A señala sin que el clic llegue a la app. La nota se guarda en
// docs/ux/notas/<versión>/ (la de docs/ux/notas/VERSION, la versión en curso) con la zona, la
// escena, el elemento y dos capturas; la numeración empieza en 001 en cada versión. Después: /notas-ux.
// Guía: docs/desarrollo/notas-ux.md.
//   node anotar.mjs [profesor|alumno] [escena]
import fs from "node:fs";
import http from "node:http";
import os from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import puppeteer from "puppeteer";
import { exportarManual, raiz } from "./arnes.mjs";
import { escenas, rutas, zonas } from "./zonas.mjs";

const aqui = path.dirname(fileURLToPath(import.meta.url));
const prueba = process.argv.includes("--prueba");
const args = process.argv.slice(2).filter((a) => !a.startsWith("--"));
// Las notas van con el versionado x.y.z: una carpeta por versión, la que dice VERSION.
// En --prueba (CI y verificación): sin ventana y en una carpeta temporal.
const raizNotas = prueba ? fs.mkdtempSync(path.join(os.tmpdir(), "aula-notas-")) : path.join(raiz, "docs", "ux", "notas");
const version = fs.readFileSync(path.join(raiz, "docs", "ux", "notas", "VERSION"), "utf8").trim();
if (!/^v\d+\.\d+\.\d+$/.test(version)) throw new Error("docs/ux/notas/VERSION debe decir vX.Y.Z, dice: " + version);
const dirNotas = path.join(raizNotas, version);
const ui = path.join(raiz, "internal", "ui");
const apps = {
  profesor: { dist: path.join(raiz, "apps", "teacher", "frontend", "dist"), dobles: fs.readFileSync(path.join(aqui, "dobles.js"), "utf8") },
  alumno: { dist: path.join(raiz, "apps", "student", "frontend", "dist"), dobles: fs.readFileSync(path.join(aqui, "dobles-alumno.js"), "utf8") },
};
const overlay = fs.readFileSync(path.join(aqui, "anotador.js"), "utf8");

let estado = { app: args[0] === "alumno" ? "alumno" : "profesor", escena: "", ruta: "", tema: "ambos" };
estado.escena = args[1] || Object.keys(escenas[estado.app])[0];

// --- Notas -----------------------------------------------------------------------------------
fs.mkdirSync(dirNotas, { recursive: true });
const leerNotas = () => fs.readdirSync(dirNotas).filter((f) => /^\d+\.json$/.test(f)).sort()
  .map((f) => JSON.parse(fs.readFileSync(path.join(dirNotas, f), "utf8")));
const tres = (n) => String(n).padStart(3, "0");

// El índice de todas las versiones (docs/ux/notas/LEEME.md): cuántas notas tiene cada una.
function escribirIndiceVersiones() {
  const versiones = fs.readdirSync(raizNotas).filter((d) => /^v\d+\.\d+\.\d+$/.test(d))
    .sort((a, b) => a.slice(1).split(".").map(Number).reduce((r, x, i) => r || x - b.slice(1).split(".").map(Number)[i], 0));
  const fila = (v) => {
    const ns = fs.readdirSync(path.join(raizNotas, v)).filter((f) => /^\d+\.json$/.test(f))
      .map((f) => JSON.parse(fs.readFileSync(path.join(raizNotas, v, f), "utf8")));
    const cuenta = (e) => ns.filter((n) => n.estado === e).length;
    return `| [${v}](${v}/LEEME.md)${v === version ? " (en curso)" : ""} | ${ns.length} | ${cuenta("pendiente") + cuenta("en curso")} | ${cuenta("hecha")} | ${cuenta("descartada")} |`;
  };
  fs.writeFileSync(path.join(raizNotas, "LEEME.md"), `# Notas de interfaz

Las escribe el anotador (\`node scripts/ux/anotar.mjs\`) y se trabajan con \`/notas-ux\`. Van con el
versionado: una carpeta por versión (\`vX.Y.Z/\`), con la numeración desde 001 en cada una. La
versión en curso es la de \`VERSION\`; al publicarla, \`VERSION\` pasa a la siguiente y sus notas
quedan cerradas en su carpeta.

Al repositorio solo suben \`NNN.json\` y los índices. Las capturas (\`*.webp\`) se quedan en el
equipo donde se anotó (\`.gitignore\`).

| Versión | Notas | Pendientes o en curso | Hechas | Descartadas |
|---|---|---|---|---|
${versiones.map(fila).join("\n")}
`);
}

function escribirIndice() {
  const notas = leerNotas();
  const fila = (n) => `| ${n.n} | ${n.estado}${n.respuesta ? " — " + n.respuesta.replace(/\|/g, "/") : ""} | ${n.zona.cadena}${n.elemento && n.elemento.texto ? " · «" + n.elemento.texto.replace(/\|/g, "/") + "»" : ""} | ${n.tipo} | ${n.texto.replace(/\r?\n/g, " ").replace(/\|/g, "/")} | ${n.escena} · ${n.tema || "claro"} | [elemento](${tres(n.n)}-elemento.webp) · [pantalla](${tres(n.n)}-pantalla.webp)` +
    (n.tema === "ambos" ? ` · oscuro: [elemento](${tres(n.n)}-elemento-oscuro.webp) · [pantalla](${tres(n.n)}-pantalla-oscuro.webp)` : "") + " |";
  fs.writeFileSync(path.join(dirNotas, "LEEME.md"), `# Notas de interfaz · ${version}

Las escribe el anotador (\`node scripts/ux/anotar.mjs\`): cada una dice dónde (con los nombres de
\`docs/ux/MAPA.md\`), qué tipo de cambio y qué quieres, con una captura del elemento y de la
pantalla (con el tema «ambos», también en oscuro). Este índice se regenera solo; el detalle está en \`NNN.json\`. Estado: \`pendiente\`,
\`en curso\`, \`hecha\` o \`descartada\` (lo actualiza Claude al trabajarlas).

Al repositorio solo suben \`NNN.json\` y este índice. Las capturas (\`*.webp\`) se quedan en el equipo
donde se anotó (\`.gitignore\`): los enlaces de «Capturas» abren ahí. Todas las versiones:
[../LEEME.md](../LEEME.md).

| # | Estado | Dónde | Tipo | Qué quieres | Escena · tema | Capturas |
|---|---|---|---|---|---|---|
${notas.map(fila).join("\n")}
`);
  escribirIndiceVersiones();
}

// --indice: solo regenera LEEME.md (después de cambiar el estado de una nota a mano).
if (process.argv.includes("--indice")) { escribirIndice(); console.log(`Índice regenerado: ${version}, ${leerNotas().length} notas`); process.exit(0); }

console.log("Exportando el manual de la consola…");
const manual = exportarManual();

// Las dos apps en el mismo servidor: /profesor/… y /alumno/…, cada una con su sistema/.
const tipos = { ".html": "text/html", ".js": "text/javascript", ".css": "text/css", ".ttf": "font/ttf", ".svg": "image/svg+xml",
  ".png": "image/png", ".webp": "image/webp", ".json": "application/json" };
// Fuera de la ventana del anotador (otro navegador, otra pestaña) faltan los dobles y el manual:
// la app no tiene backend y quedaría en blanco. Este script lo dice.
const avisoFuera = `(function () {
  if (window.__ANOTADOR) return;
  document.addEventListener("DOMContentLoaded", function () {
    var c = document.createElement("div");
    c.style.cssText = "max-width:40rem;margin:4rem auto;padding:1.5rem;font:16px/1.5 Segoe UI,system-ui,sans-serif;" +
      "border:1px solid #d5dae2;border-radius:8px;background:#fff;color:#1a1f2b";
    var h = document.createElement("h1");
    h.style.cssText = "font-size:1.25rem;margin:0 0 .75rem";
    h.textContent = "Esta página solo funciona en la ventana del anotador";
    var p = document.createElement("p");
    p.textContent = "La abre «node anotar.mjs» (en scripts/ux) en una ventana de Chromium aparte, con los datos de " +
      "demostración y la barra del anotador. En otro navegador no tiene nada de eso y queda en blanco. " +
      "Busca esa ventana en la barra de tareas; si no se abrió, revisa los mensajes de la terminal.";
    c.append(h, p);
    document.body.replaceChildren(c);
    document.body.removeAttribute("class");
  });
})();
`;

const srv = http.createServer((q, r) => {
  const url = decodeURIComponent(q.url.split("?")[0].split("#")[0]);
  const m = /^\/(profesor|alumno)\/(.*)$/.exec(url);
  if (!m) { r.writeHead(302, { Location: "/profesor/" }); r.end(); return; }
  const rel = m[2] || "index.html";
  if (rel === "__aviso-anotador.js") { r.writeHead(200, { "Content-Type": "text/javascript" }); r.end(avisoFuera); return; }
  if (rel === "index.html") {
    // El aviso va primero en <head>: si la página se abre en otro navegador (sin lo que inyecta
    // la ventana del anotador), explica qué pasa en vez de quedar en blanco.
    const html = fs.readFileSync(path.join(apps[m[1]].dist, "index.html"), "utf8")
      .replace("<head>", '<head>\n  <script src="__aviso-anotador.js"></script>');
    r.writeHead(200, { "Content-Type": "text/html" });
    r.end(html);
    return;
  }
  const [base, archivo] = rel.startsWith("sistema/") ? [ui, rel.slice("sistema/".length)] : [apps[m[1]].dist, rel];
  const p = path.join(base, archivo);
  if (!p.startsWith(base + path.sep) || p.endsWith(".go")) { r.writeHead(403); r.end(); return; }
  fs.readFile(p, (e, d) => {
    if (e) { r.writeHead(404); r.end(); return; }
    r.writeHead(200, { "Content-Type": tipos[path.extname(p)] || "application/octet-stream" });
    r.end(d);
  });
}).listen(0, "127.0.0.1");
await new Promise((r) => srv.once("listening", r));
const origen = "http://127.0.0.1:" + srv.address().port;

const nav = await puppeteer.launch({ headless: prueba, defaultViewport: prueba ? { width: 1280, height: 760 } : null, args: ["--window-size=1320,860"] });
const [pagina] = await nav.pages();
const p = pagina || (await nav.newPage());

// El overlay usa su propio <style>; la CSP del alumno (style-src 'self') lo bloquearía. Solo aquí.
await p.setBypassCSP(true);

// Antes de cada carga: la escena de la URL, los dobles de esa app, el manual y la configuración
// del overlay, que se agrega cuando el DOM existe.
await p.evaluateOnNewDocument((d, man, ov, conf) => {
  const app = location.pathname.startsWith("/alumno/") ? "alumno" : "profesor";
  const q = new URLSearchParams(location.search);
  window.__SCENE = q.get("escena") || Object.keys(conf.escenas[app])[0];
  window.__MANUAL = JSON.parse(man);
  window.__ANOTADOR = Object.assign({}, conf, { app, escena: window.__SCENE, ruta: location.hash.slice(1), tema: q.get("tema") || "ambos" });
  (0, eval)(d[app]);
  (0, eval)(ov);
}, { profesor: apps.profesor.dobles, alumno: apps.alumno.dobles }, manual, overlay, { zonas, escenas, rutas });

// «ambos»: la ventana se ve en claro y cada nota se captura en claro y en oscuro.
const tema = (v) => p.emulateMediaFeatures([{ name: "prefers-color-scheme", value: v }]);

async function ir(nuevo) {
  estado = Object.assign(estado, nuevo);
  await tema(estado.tema === "oscuro" ? "dark" : "light");
  const url = `${origen}/${estado.app}/?escena=${encodeURIComponent(estado.escena)}&tema=${estado.tema}` + (estado.ruta ? "#" + estado.ruta : "");
  await p.goto(url, { waitUntil: "networkidle0" });
}

await p.exposeFunction("__anotadorIr", (nuevo) => { ir(nuevo).catch((e) => console.error(e.message)); return true; });
await p.exposeFunction("__anotadorListar", () => leerNotas().reverse());
await p.exposeFunction("__anotadorBorrar", (n) => {
  for (const f of fs.readdirSync(dirNotas).filter((x) => x === tres(n) + ".json" || x.startsWith(tres(n) + "-"))) fs.rmSync(path.join(dirNotas, f), { force: true });
  escribirIndice();
  return true;
});
await p.exposeFunction("__anotadorGuardar", async (nota) => {
  const n = leerNotas().reduce((m, x) => Math.max(m, x.n), 0) + 1;
  const base = path.join(dirNotas, tres(n));
  let clip = null;
  if (nota.rect && nota.rect.w > 0 && nota.rect.h > 0) {
    const vp = await p.evaluate(() => ({ w: innerWidth, h: innerHeight }));
    const m = 24; // un poco de aire alrededor
    const x = Math.max(0, nota.rect.x - m), y = Math.max(0, nota.rect.y - m);
    // clip va en coordenadas del documento: se suma lo desplazado.
    clip = { x: x + (nota.rect.sx || 0), y: y + (nota.rect.sy || 0),
      width: Math.min(vp.w - x, nota.rect.w + 2 * m), height: Math.min(vp.h - y, nota.rect.h + 2 * m) };
  }
  const capturar = async (sufijo) => {
    await p.screenshot({ path: base + "-pantalla" + sufijo + ".webp", type: "webp", quality: 85 });
    if (clip) await p.screenshot({ path: base + "-elemento" + sufijo + ".webp", type: "webp", quality: 90, clip });
    else fs.copyFileSync(base + "-pantalla" + sufijo + ".webp", base + "-elemento" + sufijo + ".webp");
  };
  await capturar("");
  if (estado.tema === "ambos") {
    await tema("dark");
    await p.evaluate(() => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r))));
    await capturar("-oscuro");
    await tema("light");
  }
  const registro = { n, version, fecha: new Date().toISOString(), estado: "pendiente", app: estado.app, escena: estado.escena,
    ruta: nota.ruta, tema: estado.tema, // «ambos»: hay capturas -oscuro también
    vista: nota.vista, zona: nota.zona, elemento: nota.elemento, tipo: nota.tipo, texto: nota.texto };
  fs.writeFileSync(base + ".json", JSON.stringify(registro, null, 2) + "\n");
  escribirIndice();
  console.log(`Nota ${n}: ${nota.zona.cadena} — ${nota.texto.split("\n")[0].slice(0, 70)}`);
  return { n };
});

await ir({});
if (prueba) await autoprueba();
console.log(`
Anotador abierto (${origen}). En la barra de abajo a la izquierda:
  · elige app, escena, vista y tema;
  · «Anotar» o la tecla A, señala algo y escribe tu nota (Ctrl+Enter guarda);
  · «Nota de la pantalla» para algo general; H esconde la barra;
  · arrastra la barra por «⠿ Anotador» si tapa algo (M la pasa arriba o abajo; doble clic la regresa).
Las notas quedan en docs/ux/notas/ (índice en LEEME.md). Cierra la ventana para terminar.`);
nav.on("disconnected", () => { srv.close(); process.exit(0); });

// --prueba: anota un elemento de cada app como lo haría una persona (tecla A, mouse, texto,
// Ctrl+Enter) y comprueba la nota guardada: zona, capturas y que el clic no llegó a la app.
async function autoprueba() {
  const fallas = [];
  const ok = (c, m) => { if (!c) fallas.push(m); };
  const etiqueta = () => p.evaluate(() => document.getElementById("anotador-host").shadowRoot.querySelector(".etiqueta").textContent);
  async function anotar(selector, texto) {
    await p.keyboard.press("a");
    const caja = await (await p.$(selector)).boundingBox();
    await p.mouse.move(caja.x + caja.width / 2, caja.y + caja.height / 2);
    await new Promise((r) => setTimeout(r, 100));
    const vista = await etiqueta();
    await p.mouse.click(caja.x + caja.width / 2, caja.y + caja.height / 2);
    await p.keyboard.type(texto);
    if (process.env.ANOTADOR_CONSERVAR) await p.screenshot({ path: path.join(dirNotas, "ui-" + (anotadas + 1) + ".webp") });
    await p.keyboard.down("Control"); await p.keyboard.press("Enter"); await p.keyboard.up("Control");
    for (let i = 0; i < 50 && leerNotas().length === 0 + anotadas; i++) await new Promise((r) => setTimeout(r, 100));
    anotadas++;
    return vista;
  }
  let anotadas = 0;

  await ir({ app: "profesor", escena: "lab", ruta: "" });
  let vista = await anotar("#alumnos-filas tr .ficha", "Prueba: la ficha debería abrirse con doble clic");
  ok(vista.startsWith("Consola › Vista Alumnos › Lista de alumnos › Fila del alumno"), "etiqueta de la consola: " + vista);
  ok(await p.$eval("#panel-alumno", (e) => e.hidden || getComputedStyle(e).display === "none" || !e.classList.contains("abierto")),
    "el clic al anotar llegó a la app (abrió la ficha)");

  await ir({ app: "alumno", escena: "archivos" });
  vista = await anotar("#archivos > li .abrir", "Prueba: el botón debería ser primario");
  ok(vista.startsWith("Alumno › Archivos recibidos › Tarjeta de archivo"), "etiqueta del alumno: " + vista);

  // Navegando normal: el clic llega a la app y deja la selección; N anota; se elige el nivel.
  const shadow = (sel) => p.evaluateHandle((x) => document.getElementById("anotador-host").shadowRoot.querySelector(x), sel);
  await p.click("#mano");
  await new Promise((r) => setTimeout(r, 200));
  ok(await p.$eval("#mano", (b) => b.getAttribute("aria-pressed") === "true"), "el clic normal no llegó a la app (la mano no se levantó)");
  const barraTexto = await p.evaluate(() => document.getElementById("anotador-host").shadowRoot.querySelector("#niveles-barra").textContent);
  ok(barraTexto.includes("Tira de identidad") && barraTexto.includes("Botones de la tira"), "la barra no muestra la selección: " + barraTexto);
  await p.keyboard.press("n");
  await new Promise((r) => setTimeout(r, 100));
  const tira = await p.evaluateHandle(() => [...document.getElementById("anotador-host").shadowRoot.querySelectorAll("#niveles-form button")]
    .find((b) => b.textContent === "Tira de identidad"));
  await tira.click();
  if (process.env.ANOTADOR_CONSERVAR) await p.screenshot({ path: path.join(dirNotas, "ui-niveles.webp") });
  await (await shadow("#texto")).focus();
  await p.keyboard.type("Prueba: la tira debería ser más baja");
  await p.keyboard.down("Control"); await p.keyboard.press("Enter"); await p.keyboard.up("Control");
  for (let i = 0; i < 50 && leerNotas().length < 3; i++) await new Promise((r) => setTimeout(r, 100));
  const tercera = leerNotas()[2];
  ok(tercera && tercera.zona.id === "tira" && tercera.zona.cadena === "Alumno › Tira de identidad", "nota por nivel: " + JSON.stringify(tercera && tercera.zona));

  // Con un diálogo modal abierto (Pantalla grande), la barra sigue visible y se puede pulsar.
  await ir({ app: "profesor", escena: "lab", ruta: "" });
  await p.click('[data-modo="miniaturas"]');
  await p.evaluate(() => window.__emit("aula:frame", { sessionId: "s-LAB3-PC07", seq: 1, width: 320, height: 180,
    jpeg: window.__thumb("LAB3-PC07 - Visual Studio Code", 210) }));
  await new Promise((r) => setTimeout(r, 200));
  await p.click(".alumno__pantalla:not([disabled])");
  await new Promise((r) => setTimeout(r, 300));
  ok(await p.evaluate(() => document.getElementById("anotador-host").parentElement.id === "pantalla-grande"),
    "con la Pantalla grande abierta, el anotador no se metió en el diálogo");
  await p.click("#pg-titulo");
  const boton = await (await shadow("#anotar-sel")).boundingBox();
  await p.mouse.click(boton.x + boton.width / 2, boton.y + boton.height / 2); // un clic real: si fuera inerte, no abriría
  await new Promise((r) => setTimeout(r, 150));
  await p.keyboard.type("Prueba: el título debería decir también la hora de conexión");
  await p.keyboard.down("Control"); await p.keyboard.press("Enter"); await p.keyboard.up("Control");
  for (let i = 0; i < 50 && leerNotas().length < 4; i++) await new Promise((r) => setTimeout(r, 100));
  const cuarta = leerNotas()[3];
  ok(cuarta && cuarta.zona.id === "pantalla-grande", "nota con la Pantalla grande abierta: " + JSON.stringify(cuarta && cuarta.zona));
  ok(await p.$eval("#pantalla-grande", (d) => d.open), "anotar cerró la Pantalla grande");
  await p.keyboard.press("Escape"); // ahora sí cierra el diálogo: no hay formulario abierto
  await new Promise((r) => setTimeout(r, 200));
  ok(await p.evaluate(() => document.getElementById("anotador-host").parentElement === document.documentElement),
    "al cerrar el diálogo, el anotador no volvió a su lugar");

  // La barra se arrastra por su asa, se queda donde la dejan al cambiar de escena y el doble clic la regresa.
  const rectBarra = () => p.evaluate(() => { const r = document.getElementById("anotador-host").shadowRoot.querySelector(".barra").getBoundingClientRect(); return { x: r.left, y: r.top, abajo: innerHeight - r.bottom }; });
  const asa = await (await shadow("#asa")).boundingBox();
  await p.mouse.move(asa.x + 10, asa.y + asa.height / 2);
  await p.mouse.down();
  await p.mouse.move(asa.x + 210, 40, { steps: 8 });
  await p.mouse.up();
  let rb = await rectBarra();
  ok(rb.y < 60, "la barra no se movió al arrastrarla: " + JSON.stringify(rb));
  await ir({ app: "profesor", escena: "lab", ruta: "" });
  const rb2 = await rectBarra();
  ok(Math.abs(rb2.x - rb.x) < 2 && Math.abs(rb2.y - rb.y) < 2, "la barra no recordó su lugar: " + JSON.stringify([rb, rb2]));
  const asa2 = await (await shadow("#asa")).boundingBox();
  await p.mouse.click(asa2.x + 10, asa2.y + asa2.height / 2);
  await p.mouse.click(asa2.x + 10, asa2.y + asa2.height / 2);
  await new Promise((r) => setTimeout(r, 100));
  rb = await rectBarra();
  ok(Math.abs(rb.abajo - 12) < 2 && Math.abs(rb.x - 12) < 2, "el doble clic no regresó la barra abajo: " + JSON.stringify(rb));

  const notas = leerNotas();
  ok(notas.length === 4, "notas guardadas: " + notas.length);
  for (const n of notas) {
    for (const f of [tres(n.n) + "-elemento.webp", tres(n.n) + "-pantalla.webp"]) ok(fs.statSync(path.join(dirNotas, f)).size > 500, "captura vacía: " + f);
  }
  // Tema «ambos» (por omisión): cada nota también en oscuro, y de verdad en otro tema.
  for (const n of notas) {
    ok(n.tema === "ambos", "tema de la nota " + n.n + ": " + n.tema);
    for (const parte of ["elemento", "pantalla"]) {
      const claro = path.join(dirNotas, tres(n.n) + "-" + parte + ".webp"), oscuro = path.join(dirNotas, tres(n.n) + "-" + parte + "-oscuro.webp");
      ok(fs.existsSync(oscuro) && !fs.readFileSync(oscuro).equals(fs.readFileSync(claro)), "falta o es igual la captura en oscuro: " + path.basename(oscuro));
    }
  }
  ok(fs.readFileSync(path.join(dirNotas, "LEEME.md"), "utf8").includes("· oscuro: [elemento]"), "el índice no enlaza las capturas en oscuro");
  ok(notas[0] && notas[0].zona.id === "fila-alumno" && notas[0].tipo === "mover" && notas[0].texto.startsWith("Prueba"), "nota 1: " + JSON.stringify(notas[0]));
  ok(notas[1] && notas[1].zona.id === "tarjeta-archivo" && notas[1].app === "alumno" && notas[1].escena === "archivos", "nota 2: " + JSON.stringify(notas[1]));
  ok(fs.readFileSync(path.join(dirNotas, "LEEME.md"), "utf8").includes("| 2 | pendiente | Alumno › Archivos recibidos"), "índice LEEME.md");
  ok(path.basename(dirNotas) === version && JSON.parse(fs.readFileSync(path.join(dirNotas, "001.json"), "utf8")).version === version,
    "la nota no quedó en la carpeta de su versión");
  ok(fs.readFileSync(path.join(raizNotas, "LEEME.md"), "utf8").includes(`| [${version}](${version}/LEEME.md) (en curso) | 4 |`),
    "el índice de versiones no cuenta las notas");
  // Abierta como en otro navegador (sin lo que inyecta la ventana): dice qué pasa, no queda en blanco.
  const ajena = await (await nav.createBrowserContext()).newPage();
  await ajena.goto(origen + "/profesor/", { waitUntil: "networkidle0" });
  await new Promise((r) => setTimeout(r, 300));
  ok((await ajena.$eval("body", (b) => b.textContent)).includes("solo funciona en la ventana del anotador"), "la página abierta fuera de la ventana queda en blanco");
  console.log(fallas.length ? "✗ anotador:\n  " + fallas.join("\n  ") : "✓ anotador: 4 notas (señalando, navegando con nivel elegido y con un diálogo abierto), capturas e índice");
  await nav.close();
  if (process.env.ANOTADOR_CONSERVAR) console.log("Notas de prueba en " + dirNotas);
  else fs.rmSync(raizNotas, { recursive: true, force: true });
  process.exit(fallas.length ? 1 : 0);
}
