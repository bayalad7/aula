// Regenera las capturas del manual (docs/administracion/img/) con el
// arnés web y datos de demostración, para que nunca queden viejas a mano.
//   node capturas-manual.mjs [--solo=participacion]   (solo las que contienen alguna palabra)
import path from "node:path";
import { abrir, click, raiz, seq, sleep, soloCapturas } from "./arnes.mjs";

const img = path.join(raiz, "docs", "administracion", "img");
const fila = (host) => `#alumnos-filas tr[data-clave="r:${host}"]`;
const { foto, cerrar } = await abrir();

// archivo → escena y pasos. Cada una la usa una página del manual.
const capturas = {
  "alumnos.webp": ["lab", { pasos: seq(click(fila("LAB3-PC07")), click(fila("LAB3-PC12"))) }],
  "ficha-alumno.webp": ["lab-errores", { pasos: click(`${fila("lab-ubuntu-02")} .ficha`) }],
  "codigo.webp": ["datos", { ruta: "codigo", pasos: async (p) => {
    await p.evaluate(() => window.monaco && window.monaco.editor.getEditors()[0].setValue(
      "def factorial(n):\n    if n <= 1:\n        return 1\n    return n * factorial(n - 1)\n\nprint(factorial(5))\n"));
    await click(".envio__desplegar")(p);
  } }],
  "archivos.webp": ["lab-archivos", { ruta: "archivos", h: 1000, pasos: seq(async (p) => {
    await p.select("#archivos-plantilla", "Ejercicio 4");
    await p.evaluate(() => { document.getElementById("archivos-destino").value = "Ejercicio 4"; });
  }, click("[data-vista-de=archivos] .envio__desplegar")) }],
  "recolecciones.webp": ["lab-recolecciones", { ruta: "archivos/recolecciones", h: 800 }],
  "simulada.webp": ["simulada", { ruta: "simulada" }],
  "participacion.webp": ["lab-participacion", { ruta: "participacion" }],
  "participacion-encuesta.webp": ["lab-encuesta", { ruta: "participacion", pasos: async (p) => {
    await p.evaluate(() => document.getElementById("encuestas").scrollIntoView());
    await sleep(200);
  } }],
};
const incluir = soloCapturas();

let mal = 0;
for (const [archivo, [escena, o]] of Object.entries(capturas)) {
  if (!incluir(archivo)) continue;
  const r = await foto(path.join(img, archivo), escena, o);
  mal += r.errores.length;
  console.log((r.errores.length ? "✗ " : "✓ ") + path.relative(raiz, path.join(img, archivo)));
  r.errores.forEach((e) => console.log("    JS  " + e));
}
await cerrar();
if (mal) process.exit(1);
