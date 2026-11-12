// Mapa de la interfaz: una captura por pantalla con sus zonas numeradas y, debajo, el nombre de
// cada zona. Los nombres salen de zonas.mjs, los mismos que muestra el anotador. Escribe
// docs/ux/MAPA.md y las imágenes en docs/ux/mapa/ (WebP).
//   node mapa.mjs [--solo=encuesta]   (vuelve a capturar solo esas imágenes; MAPA.md, siempre completo)
import fs from "node:fs";
import path from "node:path";
import { abrir, click, cuadros, raiz, seq, sleep, soloCapturas } from "./arnes.mjs";
import { zonas } from "./zonas.mjs";

const dir = path.join(raiz, "docs", "ux", "mapa");
fs.mkdirSync(dir, { recursive: true });
const fila = (host) => `#alumnos-filas tr[data-clave="r:${host}"]`;
const miniaturas = seq(click('[data-modo="miniaturas"]'), cuadros(["LAB3-PC07", "LAB3-PC12", "LAB3-PC15"]));

// Cada pantalla: app, escena, cómo llegar y qué zonas numerar (ids de zonas.mjs).
const pantallas = [
  { app: "profesor", archivo: "consola-alumnos", titulo: "Consola · Alumnos (lista)", escena: "lab",
    zonas: ["menu", "menu-clase", "menu-terminar", "cabecera", "alumnos-seleccion", "alumnos-modo", "alumnos-lista", "fila-alumno", "barra-acciones"] },
  { app: "profesor", archivo: "consola-miniaturas", titulo: "Consola · Alumnos (miniaturas)", escena: "lab", pasos: miniaturas,
    zonas: ["alumnos-tamano", "alumnos-miniaturas", "tarjeta-alumno", "barra-acciones"] },
  { app: "profesor", archivo: "consola-ficha", titulo: "Consola · Ficha del alumno", escena: "lab", h: 900,
    pasos: click(`${fila("LAB3-PC07")} .ficha`), zonas: ["ficha", "ficha-puede"] },
  { app: "profesor", archivo: "consola-pantalla-grande", titulo: "Consola · Pantalla grande", escena: "lab",
    pasos: seq(miniaturas, click(".alumno__pantalla:not([disabled])"), cuadros(["LAB3-PC07"])), zonas: ["pantalla-grande", "pg-acciones"] },
  { app: "profesor", archivo: "consola-franja-panel", titulo: "Consola · Franja global y un panel de la barra", escena: "lab-transmitiendo",
    pasos: click('.barra-acciones [data-panel="aviso"]'), zonas: ["franja", "panel-aviso", "barra-acciones"] },
  { app: "profesor", archivo: "consola-pausa", titulo: "Consola · Pausar pantallas", escena: "lab-pausa",
    pasos: click('.barra-acciones [data-panel="pausa"]'), zonas: ["franja", "panel-pausa", "barra-acciones"] },
  { app: "profesor", archivo: "consola-terminar", titulo: "Consola · Terminar la clase", escena: "lab-terminar", h: 800,
    pasos: click("#nav-terminar"), zonas: ["menu-terminar", "dlg-terminar"] },
  { app: "profesor", archivo: "consola-abrir", titulo: "Consola · Abrir página o app", escena: "lab-abrir",
    pasos: click('.barra-acciones [data-panel="abrir"]'), zonas: ["panel-abrir", "barra-acciones"] },
  { app: "profesor", archivo: "consola-ajustes-apps", titulo: "Consola · Ajustes › Apps permitidas", escena: "datos", h: 800,
    pasos: seq(click("#nav-ajustes"), click("#tab-apps")), zonas: ["ajustes", "ajustes-apps"] },
  { app: "profesor", archivo: "consola-codigo", titulo: "Consola · Código", escena: "datos", ruta: "codigo", h: 1000,
    zonas: ["codigo-editor", "editor", "enviar-a", "envios", "historial"] },
  { app: "profesor", archivo: "consola-archivos", titulo: "Consola · Archivos", escena: "lab-archivos", ruta: "archivos", h: 1250,
    pasos: async (p) => { await p.select("#archivos-plantilla", "Ejercicio 4"); },
    zonas: ["menu", "archivos-enviar", "archivos-envios", "plantillas", "archivos-historial"] },
  { app: "profesor", archivo: "consola-recolecciones", titulo: "Consola · Archivos › Recolecciones", escena: "lab-recolecciones", ruta: "archivos/recolecciones", h: 1000,
    zonas: ["archivos-pestanas", "reco-nueva", "recoleccion", "reco-fila"] },
  { app: "profesor", archivo: "consola-participacion", titulo: "Consola · Participación", escena: "lab-participacion", ruta: "participacion",
    zonas: ["cabecera", "manos", "mensajes", "part-temporizador"] },
  { app: "profesor", archivo: "consola-encuesta", titulo: "Consola · Encuesta (panel de la barra)", escena: "lab",
    pasos: click('.barra-acciones [data-panel="encuesta"]'), zonas: ["panel-encuesta", "barra-acciones"] },
  { app: "profesor", archivo: "consola-participacion-encuestas", titulo: "Consola · Participación › Encuestas", escena: "lab-encuesta", ruta: "participacion",
    pasos: async (p) => { await p.evaluate(() => document.getElementById("encuestas").scrollIntoView()); await sleep(200); },
    zonas: ["franja", "part-encuestas"] },
  { app: "alumno", archivo: "alumno-bandas", titulo: "Alumno · Tira de identidad y bandas", escena: "todo", w: 900, h: 700,
    zonas: ["tira", "tira-botones", "bandas", "vacio"] },
  { app: "alumno", archivo: "alumno-fin-clase", titulo: "Alumno · La clase terminó", escena: "fin-clase", w: 900, h: 700,
    zonas: ["tira", "fin-clase", "vacio"] },
  { app: "alumno", archivo: "alumno-codigo", titulo: "Alumno · Código recibido", escena: "varios-codigos", w: 900, h: 700,
    zonas: ["tira", "codigos", "tarjeta-codigo"] },
  { app: "alumno", archivo: "alumno-archivos", titulo: "Alumno · Archivos y mensajes", escena: "archivos", w: 900, h: 700,
    pasos: click("#escribir"), zonas: ["tira", "redactar", "archivos", "tarjeta-archivo"] },
  { app: "alumno", archivo: "alumno-encuestas", titulo: "Alumno · Encuestas", escena: "encuesta", w: 900, h: 700,
    zonas: ["tira", "encuestas", "tarjeta-encuesta"] },
  { app: "alumno", archivo: "alumno-recolecciones", titulo: "Alumno · Recolección", escena: "recoleccion-aviso", w: 900, h: 700,
    zonas: ["tira", "tarjeta-recoleccion", "avisos"] },
  { app: "alumno", archivo: "alumno-avisos", titulo: "Alumno · Mano levantada y avisos", escena: "mano", w: 900, h: 700,
    pasos: async (p) => { await p.evaluate(() => window.__emit("aula:notices", { notices: [{ id: "m-1", level: "info", text: "En 10 minutos revisamos el ejercicio.", at: new Date().toISOString() }] })); await sleep(200); },
    zonas: ["tira", "mano-estado", "avisos"] },
  { app: "alumno", archivo: "alumno-compacto", titulo: "Alumno · Modo compacto", escena: "temporizador", w: 380, h: 80,
    pasos: click("#compactar"), zonas: ["compacto"] },
];

// Dibuja un recuadro numerado sobre la primera coincidencia visible de cada zona.
const numerar = (lista) => async (p) => {
  await p.evaluate((ls) => {
    const capa = document.createElement("div");
    capa.style.cssText = "position:absolute;left:0;top:0;pointer-events:none;z-index:2147483647";
    // Un diálogo modal está en la capa superior: los números van dentro de él, en coordenadas de la ventana.
    const dialogo = document.querySelector("dialog[open]");
    const fija = !!dialogo;
    if (fija) capa.style.position = "fixed";
    (dialogo || document.body).appendChild(capa);
    const visible = (e) => { const r = e.getBoundingClientRect(); return r.width > 0 && r.height > 0 && getComputedStyle(e).visibility !== "hidden"; };
    const puestos = [];
    for (const { n, selector } of ls) {
      const e = [...document.querySelectorAll(selector)].find(visible);
      if (!e) continue;
      const r = e.getBoundingClientRect();
      const x = r.left + (fija ? 0 : scrollX), y = r.top + (fija ? 0 : scrollY);
      const caja = document.createElement("div");
      caja.style.cssText = `position:absolute;left:${x + 1}px;top:${y + 1}px;width:${r.width - 2}px;height:${r.height - 2}px;` +
        "border:2px solid #f59e0b;border-radius:4px;box-sizing:border-box";
      // Si otro número ya está en esa esquina, se corre a la derecha.
      let bx = Math.max(0, x + 2);
      const by = Math.max(0, y + 2);
      while (puestos.some(([px, py]) => Math.abs(px - bx) < 26 && Math.abs(py - by) < 22)) bx += 28;
      puestos.push([bx, by]);
      const num = document.createElement("div");
      num.textContent = n;
      num.style.cssText = `position:absolute;left:${bx}px;top:${by}px;min-width:20px;height:20px;padding:0 5px;` +
        "border-radius:10px;background:#f59e0b;color:#1f2433;font:700 12px/20px Segoe UI,system-ui,sans-serif;text-align:center";
      capa.append(caja, num);
    }
  }, lista);
};

const md = [`# Mapa de la interfaz

Los nombres de cada zona de las dos apps, para hablar de la interfaz sin ambigüedad: «en la
**Ficha del alumno**, en **Qué puede hacer su equipo**…». Son los mismos que muestra el anotador
(\`node scripts/ux/anotar.mjs\`) al pasar el mouse. La fuente es \`scripts/ux/zonas.mjs\`; este
archivo y sus imágenes los genera \`node scripts/ux/mapa.mjs\` (no se editan a mano).
`];

const incluir = soloCapturas();
for (const app of ["profesor", "alumno"]) {
  const { foto, cerrar } = await abrir({ app });
  const porId = Object.fromEntries(zonas[app].map((z) => [z.id, z]));
  md.push(`\n## ${app === "profesor" ? "Consola del profesor" : "App del alumno"}\n`);
  for (const pt of pantallas.filter((x) => x.app === app)) {
    const lista = pt.zonas.map((id, i) => ({ n: i + 1, selector: porId[id].selector }));
    const archivo = path.join(dir, pt.archivo + ".webp");
    if (incluir(pt.archivo)) {
      const r = await foto(archivo, pt.escena, { w: pt.w || 1280, h: pt.h || 760, ruta: pt.ruta || "", revisar: false,
        pasos: seq(pt.pasos || (async () => {}), numerar(lista)) });
      if (r.errores.length) console.log("  JS " + r.errores.join(" | "));
      console.log("✓ " + pt.archivo);
    }
    md.push(`### ${pt.titulo}\n\n![${pt.titulo}](mapa/${pt.archivo}.webp)\n\n| # | Zona | Qué es |\n|---|---|---|`);
    for (const [i, id] of pt.zonas.entries()) {
      const z = porId[id];
      md.push(`| ${i + 1} | **${z.nombre}** | ${z.que} |`);
    }
    md.push("");
  }
  await cerrar();
}

md.push(`\n## Todas las zonas\n\nTambién las que no salen en una imagen (diálogos y paneles). El \`id\` es el que guardan las notas.\n`);
for (const app of ["profesor", "alumno"]) {
  const porId = Object.fromEntries(zonas[app].map((z) => [z.id, z]));
  const ruta = (z) => { const n = []; for (let x = z; x; x = x.dentro ? porId[x.dentro] : null) n.unshift(x.nombre); return n.join(" › "); };
  md.push(`### ${app === "profesor" ? "Consola" : "Alumno"}\n\n| id | Zona | Qué es |\n|---|---|---|`);
  for (const z of zonas[app]) md.push(`| \`${z.id}\` | ${ruta(z)} | ${z.que} |`);
  md.push("");
}
fs.writeFileSync(path.join(raiz, "docs", "ux", "MAPA.md"), md.join("\n"));
console.log("MAPA.md escrito");
