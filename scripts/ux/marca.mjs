// Genera la marca de Aula (logo Nodo) desde una sola geometría:
//   - internal/ui/marca/*.svg: isotipo, horizontal e icono de app (claro, oscuro, una tinta);
//   - apps/{teacher,student}/build/appicon.png (1024 px) y build/windows/icon.ico.
// El icono de la bandeja del alumno sale de appicon.png (apps/student/trayicon.go).
// Uso: node scripts/ux/marca.mjs (desde la raíz o desde scripts/ux). Referencia:
// docs/ux/DESIGN_SYSTEM.md, §2.
import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import puppeteer from "puppeteer";

const raiz = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../..");

const COLORES = {
  claro: { tinta: "#1a1f2b", nodo: "#1d4ed8" },
  oscuro: { tinta: "#e7eaf0", nodo: "#6b8cff" },
  "una-tinta": { tinta: "currentColor", nodo: "currentColor" },
};

// Rejilla de 32: cuatro pantallas de 12 × 12 (radio 3) con la esquina interior mordida por un
// círculo de radio 7.5 (corta los bordes en 8.77 / 23.23) y el nodo de radio 4.5 al centro.
const PANTALLAS = [
  "M5 2H11A3 3 0 0 1 14 5V8.77A7.5 7.5 0 0 0 8.77 14H5A3 3 0 0 1 2 11V5A3 3 0 0 1 5 2Z",
  "M21 2H27A3 3 0 0 1 30 5V11A3 3 0 0 1 27 14H23.23A7.5 7.5 0 0 0 18 8.77V5A3 3 0 0 1 21 2Z",
  "M5 18H8.77A7.5 7.5 0 0 0 14 23.23V27A3 3 0 0 1 11 30H5A3 3 0 0 1 2 27V21A3 3 0 0 1 5 18Z",
  "M23.23 18H27A3 3 0 0 1 30 21V27A3 3 0 0 1 27 30H21A3 3 0 0 1 18 27V23.23A7.5 7.5 0 0 0 23.23 18Z",
];
const nodo = (c) =>
  `<g fill="${c.tinta}">${PANTALLAS.map((d) => `<path d="${d}"/>`).join("")}</g><circle cx="16" cy="16" r="4.5" fill="${c.nodo}"/>`;
// «aula» dibujado (no es texto): trazo 4.4, curvas de radio 6, altura x de 12.8 a 29.2.
const letras = (c) =>
  `<g fill="none" stroke="${c.tinta}" stroke-width="4.4"><circle cx="8.2" cy="21" r="6"/><path d="M14.2 12.8V29.2M22.2 12.8V21a6 6 0 0 0 12 0M34.2 12.8V29.2M42.2 4V29.2"/><circle cx="56.2" cy="21" r="6"/><path d="M62.2 12.8V29.2"/></g>`;

const svg = (viewBox, cuerpo) =>
  `<svg xmlns="http://www.w3.org/2000/svg" viewBox="${viewBox}" role="img" aria-label="Aula">${cuerpo}</svg>\n`;
const isotipo = (c) => svg("0 0 32 32", nodo(c));
const horizontal = (c) => svg("0 0 116 40", `<g transform="scale(1.25)">${nodo(c)}</g><g transform="translate(50 0)">${letras(c)}</g>`);
// Icono de app: el isotipo oscuro sobre una placa (radio 7/32), con 3 unidades de margen.
const iconoApp = () =>
  svg("0 0 32 32", `<rect width="32" height="32" rx="7" fill="#12151c"/><g transform="translate(3 3) scale(0.8125)">${nodo(COLORES.oscuro)}</g>`);

async function escribirSVG() {
  const dir = path.join(raiz, "internal/ui/marca");
  await mkdir(dir, { recursive: true });
  const archivos = { "aula-icono-app.svg": iconoApp() };
  for (const [tema, c] of Object.entries(COLORES)) {
    const sufijo = tema === "claro" ? "" : `-${tema}`;
    archivos[`aula-isotipo${sufijo}.svg`] = isotipo(c);
    archivos[`aula-horizontal${sufijo}.svg`] = horizontal(c);
  }
  for (const [nombre, contenido] of Object.entries(archivos)) {
    await writeFile(path.join(dir, nombre), contenido);
  }
  return Object.keys(archivos);
}

// ICO con imágenes PNG (Windows Vista y posteriores): cabecera, un directorio de 16 bytes por
// tamaño y los PNG uno tras otro.
function ico(pngs) {
  const cabecera = Buffer.alloc(6);
  cabecera.writeUInt16LE(0, 0);
  cabecera.writeUInt16LE(1, 2);
  cabecera.writeUInt16LE(pngs.length, 4);
  let desplazamiento = 6 + 16 * pngs.length;
  const entradas = pngs.map(({ lado, datos }) => {
    const e = Buffer.alloc(16);
    e.writeUInt8(lado >= 256 ? 0 : lado, 0);
    e.writeUInt8(lado >= 256 ? 0 : lado, 1);
    e.writeUInt16LE(1, 4);
    e.writeUInt16LE(32, 6);
    e.writeUInt32LE(datos.length, 8);
    e.writeUInt32LE(desplazamiento, 12);
    desplazamiento += datos.length;
    return e;
  });
  return Buffer.concat([cabecera, ...entradas, ...pngs.map((p) => p.datos)]);
}

async function rasterizar(navegador, contenido, lado) {
  const pagina = await navegador.newPage();
  await pagina.setViewport({ width: lado, height: lado, deviceScaleFactor: 1 });
  const html = `<!doctype html><style>html,body{margin:0;background:transparent}svg{display:block;width:${lado}px;height:${lado}px}</style>${contenido}`;
  await pagina.setContent(html);
  const datos = await pagina.screenshot({ type: "png", omitBackground: true, clip: { x: 0, y: 0, width: lado, height: lado } });
  await pagina.close();
  return Buffer.from(datos);
}

async function main() {
  const svgs = await escribirSVG();
  const navegador = await puppeteer.launch({ headless: true });
  try {
    const app = iconoApp();
    const png1024 = await rasterizar(navegador, app, 1024);
    const tamanos = [16, 20, 24, 32, 40, 48, 64, 256];
    const pngs = [];
    for (const lado of tamanos) pngs.push({ lado, datos: await rasterizar(navegador, app, lado) });
    const icono = ico(pngs);
    for (const appDir of ["apps/teacher", "apps/student"]) {
      await writeFile(path.join(raiz, appDir, "build/appicon.png"), png1024);
      await writeFile(path.join(raiz, appDir, "build/windows/icon.ico"), icono);
    }
    console.log(`marca: ${svgs.length} SVG en internal/ui/marca; appicon.png (1024) e icon.ico (${tamanos.join(", ")}) en las dos apps`);
  } finally {
    await navegador.close();
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
