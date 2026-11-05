// Regenera las capturas del manual (docs/v1/administracion/img/) con el
// arnés web y datos de demostración, para que nunca queden viejas a mano.
//   node capturas-manual.mjs
import path from "node:path";
import { abrir, click, raiz, seq } from "./arnes.mjs";

const img = path.join(raiz, "docs", "v1", "administracion", "img");
const fila = (host) => `#alumnos-filas tr[data-clave="r:${host}"]`;
const { foto, cerrar } = await abrir();

// archivo → escena y pasos. Cada una la usa una página del manual.
const capturas = {
  "alumnos.png": ["lab", { pasos: seq(click(fila("LAB3-PC07")), click(fila("LAB3-PC12"))) }],
  "ficha-alumno.png": ["lab-errores", { pasos: click(`${fila("lab-ubuntu-02")} .ficha`) }],
  "codigo.png": ["datos", { ruta: "codigo", pasos: async (p) => {
    await p.evaluate(() => window.monaco && window.monaco.editor.getEditors()[0].setValue(
      "def factorial(n):\n    if n <= 1:\n        return 1\n    return n * factorial(n - 1)\n\nprint(factorial(5))\n"));
    await click(".envio__desplegar")(p);
  } }],
  "simulada.png": ["simulada", { ruta: "simulada" }],
  "participacion.png": ["lab-participacion", { ruta: "participacion" }],
};

let mal = 0;
for (const [archivo, [escena, o]] of Object.entries(capturas)) {
  const r = await foto(path.join(img, archivo), escena, o);
  mal += r.errores.length;
  console.log((r.errores.length ? "✗ " : "✓ ") + path.relative(raiz, path.join(img, archivo)));
  r.errores.forEach((e) => console.log("    JS  " + e));
}
await cerrar();
if (mal) process.exit(1);
