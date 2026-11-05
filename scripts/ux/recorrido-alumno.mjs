// Recorre la app del alumno en el arnés web (prompt 19): captura cada estado,
// junta los errores de JS y pasa axe-core (WCAG 2.1 AA). Escribe además
// axe.json con el detalle de cada violación. Sale con error si hay errores de
// JS o violaciones serias o críticas.
//   node recorrido-alumno.mjs [carpeta de salida]   (por omisión, ./salida-alumno)
import fs from "node:fs";
import path from "node:path";
import { abrir, click, seq, sleep } from "./arnes.mjs";

const salida = path.resolve(process.argv[2] || "salida-alumno");
const { foto, cerrar } = await abrir({ app: "alumno" });
const informe = {};
// La ventana del alumno abre a 900×700 (apps/student/main.go); su mínimo es 480×360.
const f = async (nombre, escena, o = {}) => {
  informe[nombre] = await foto(path.join(salida, nombre + ".png"), escena, Object.assign({ w: 900, h: 700 }, o));
};
const emitir = (evento, datos) => async (p) => { await p.evaluate((e, d) => window.__emit(e, d), evento, datos); await sleep(300); };

// Conexión
await f("01-sin-configurar", "sin-configurar");
await f("02-buscando-clase", "buscando");
await f("03-sin-conexion-diagnostico", "sin-conexion");
await f("04-reconectando", "reconectando");
await f("05-conectado-reposo", "conectado");

// Código
await f("06-un-codigo", "un-codigo");
await f("07-varios-codigos", "varios-codigos");
await f("08-codigo-copiado", "un-codigo", { pasos: click(".copy") });
await f("09-copia-falla", "copia-falla", { pasos: seq(emitir("aula:board", { snippets: [{ id: "sn-1", exercise: "1",
  title: "Factorial recursivo", lang: "python", content: "print(1)\n", receivedAt: new Date().toISOString() }] }), click(".copy")) });
await f("10-tras-limpiar", "un-codigo", { pasos: emitir("aula:board", { snippets: [] }) });

// Avisos
await f("11-aviso-info", "aviso-info");
await f("12-aviso-warning", "aviso-warning");
await f("13-varios-avisos", "varios-avisos");

// Transparencia y control del profesor
await f("14-pantalla-vista", "pantalla-vista");
await f("15-transmision", "transmision");
await f("16-restricciones", "restricciones");
await f("17-restricciones-detalles", "restricciones", { pasos: click("#restr-toggle") });
await f("18-app-cerrada", "restricciones", { pasos: emitir("aula:app-closed", { app: "steam" }) });
await f("19-energia-cuenta", "energia-cuenta");
await f("20-energia-forzada", "energia-forzada");
await f("21-energia-fallida", "energia-fallida");

// Tema y tamaño
await f("22-oscuro-varios-codigos", "varios-codigos", { oscuro: true });
await f("23-oscuro-aviso-y-restricciones", "restricciones", { oscuro: true, pasos: emitir("aula:notices",
  { notices: [{ id: "m-2", level: "warning", text: "Guarden su trabajo: en 5 minutos cerramos la sesión.", at: new Date().toISOString() }] }) });
await f("24-ventana-minima-480x360", "varios-codigos", { w: 480, h: 360 });
await f("25-ventana-minima-aviso", "aviso-warning", { w: 480, h: 360 });

await cerrar();
fs.writeFileSync(path.join(salida, "axe.json"), JSON.stringify(informe, null, 2));
let graves = 0;
for (const [nombre, r] of Object.entries(informe)) {
  const malas = r.violaciones.filter((v) => v.impacto === "serious" || v.impacto === "critical");
  graves += r.errores.length + malas.length;
  console.log((r.errores.length || r.violaciones.length ? "✗ " : "✓ ") + nombre);
  r.errores.forEach((e) => console.log("    JS  " + e));
  r.violaciones.forEach((v) => console.log("    AXE " + v.texto));
}
console.log(`\n${Object.keys(informe).length} capturas en ${salida}`);
if (graves) {
  console.error(`${graves} errores de JS o violaciones serias/críticas de axe`);
  process.exit(1);
}
