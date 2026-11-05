// Capturas «después» de la etapa 18: las mismas 26 pantallas y estados que
// docs/v1/ux/profesor/antes/ (sacadas de la consola anterior), en la consola de
// hoy, para docs/v1/ux/profesor/RESULTADOS.md. También pasa axe-core.
//   node despues.mjs [carpeta]   (por omisión, docs/v1/ux/profesor/despues)
import path from "node:path";
import { abrir, click, cuadros, raiz, seq, sleep } from "./arnes.mjs";

const salida = path.resolve(process.argv[2] || path.join(raiz, "docs", "v1", "ux", "profesor", "despues"));
const { foto, cerrar } = await abrir();
const informe = {};
const f = async (nombre, escena, o) => { informe[nombre] = await foto(path.join(salida, nombre + ".png"), escena, o); };
const fila = (host) => `#alumnos-filas tr[data-clave="r:${host}"]`;
const codigo = async (p) => {
  await p.evaluate(() => window.monaco && window.monaco.editor.getEditors()[0].setValue(
    "def factorial(n):\n    if n <= 1:\n        return 1\n    return n * factorial(n - 1)\n\nprint(factorial(5))\n"));
  await sleep(200);
};
const miniaturas = seq(click('[data-modo="miniaturas"]'), cuadros(["LAB3-PC07", "LAB3-PC12"]));

await f("01-arranque-abriendo-canal", "setup-cargando");
await f("02-arranque-error-puerto-ocupado", "setup-error");
await f("03-consola-vacia-sin-alumnos", "vacia");
await f("04-consola-2-simulados-con-historial", "datos");
await f("05-consola-editor-con-codigo", "datos", { ruta: "codigo", pasos: codigo });
await f("06-confirmar-limpiar", "datos", { ruta: "codigo", pasos: click("#codigo-limpiar") });
await f("07-consola-toast-error", "datos", { pasos: async (p) => p.evaluate(() => window.__emit("aula:capture",
  { draft: null, error: "aula:clipboardEmpty: el portapapeles está vacío: copia el código primero (Ctrl+C)" })) });
await f("08-consola-captura-cargada", "datos", { pasos: async (p) => {
  await p.evaluate(() => window.__emit("aula:capture", { draft: { content: "for i in range(10):\n    print(i * i)\n", lang: "python", lines: 2 } }));
  await sleep(300);
} });
await f("09-consola-clave-personal", "clave-personal");
await f("10-consola-atajo-ocupado", "atajo-ocupado", { pasos: seq(click("#nav-ajustes"), click("#tab-acerca")) });
await f("11-laboratorio-tarjetas", "lab");
await f("12-laboratorio-marcar-alumnos", "lab", { pasos: seq(click(fila("LAB3-PC07")), click(fila("LAB3-PC12"))) });
await f("13-pantallas-cuadricula", "lab", { pasos: miniaturas });
await f("14-pantallas-detalle", "lab", { pasos: seq(miniaturas, click(".alumno__pantalla:not([disabled])"), cuadros(["LAB3-PC07"])) });
await f("15-pantallas-vacia-simulados", "simulada", { pasos: click('[data-modo="miniaturas"]') });
await f("16-transmitiendo-pantalla", "lab-transmitiendo");
await f("17-restricciones-vacia", "lab", { pasos: click('[data-panel="restricciones"]') });
await f("18-restricciones-activas", "lab-restr-activa", { pasos: click('[data-panel="restricciones"]') });
await f("19-restricciones-simulados-sin-soporte", "simulada", { pasos: async (p) => {
  await p.evaluate(() => [...document.querySelectorAll("#alumnos-filas tr[data-clave]")].filter((r) => r.textContent.includes("Simulado"))
    .forEach((r) => r.click()));
  await sleep(200);
  await click('[data-panel="restricciones"]')(p);
} });
await f("20-energia", "lab", { pasos: click('[data-panel="energia"]') });
await f("21-energia-confirmar-forzado", "lab", { pasos: seq(click('[data-panel="energia"]'), click("#energia-forzar"), click("#energia-ejecutar")) });
await f("22-energia-pendiente-tarjetas", "lab-energia-pendiente");
await f("23-ventana-minima-900x600", "datos", { w: 900, h: 600 });
await f("24-ventana-minima-laboratorio", "lab", { w: 900, h: 600 });
await f("25-modo-oscuro", "datos", { oscuro: true });
await f("26-modo-oscuro-energia", "lab", { oscuro: true, pasos: click('[data-panel="energia"]') });

await cerrar();
let graves = 0;
for (const [nombre, r] of Object.entries(informe)) {
  const malas = r.violaciones.filter((v) => v.impacto === "serious" || v.impacto === "critical");
  graves += r.errores.length + malas.length;
  console.log((r.errores.length || r.violaciones.length ? "✗ " : "✓ ") + nombre);
  r.errores.forEach((e) => console.log("    JS  " + e));
  r.violaciones.forEach((v) => console.log("    AXE " + v.texto));
}
console.log(`\n${Object.keys(informe).length} capturas en ${salida}`);
if (graves) process.exit(1);
