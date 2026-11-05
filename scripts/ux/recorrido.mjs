// Recorre la consola del profesor en el arnés web: captura cada pantalla y
// estado, junta los errores de JS y pasa axe-core (WCAG 2.1 AA). Sale con
// error si hay errores de JS o violaciones serias o críticas.
//   node recorrido.mjs [carpeta de salida]   (por omisión, ./salida)
import path from "node:path";
import { abrir, click, cuadros, seq, sleep } from "./arnes.mjs";

const salida = path.resolve(process.argv[2] || "salida");
const { foto, cerrar } = await abrir();
const informe = {};
const f = async (nombre, escena, o) => { informe[nombre] = await foto(path.join(salida, nombre + ".png"), escena, o); };
const fila = (host) => `#alumnos-filas tr[data-clave="r:${host}"]`;
const ajustes = (pestana) => seq(click("#nav-ajustes"), click("#tab-" + pestana));

// Arranque y Alumnos
await f("01-arranque-abriendo", "setup-cargando");
await f("02-arranque-error-puerto", "setup-error");
await f("51-arranque-canal-invalido", "setup-error", { pasos: async (p) => {
  await p.evaluate(() => { document.getElementById("arranque-canal").value = ""; document.getElementById("arranque-nombre").value = ""; });
  await p.type("#arranque-canal", "20000");
  await p.type("#arranque-nombre", "Mtra. López");
  await click("#arranque-abrir")(p);
  await sleep(200);
} });
await f("03-alumnos-vacio", "vacia");
await f("04-alumnos-lista-seleccion", "lab", { pasos: seq(click(fila("LAB3-PC07")), click(fila("LAB3-PC12"))) });
await f("05-alumnos-miniaturas", "lab", { pasos: seq(click('[data-modo="miniaturas"]'), cuadros(["LAB3-PC07", "LAB3-PC12"])) });
await f("06-alumnos-pantalla-grande", "lab", { pasos: seq(click('[data-modo="miniaturas"]'), cuadros(["LAB3-PC07", "LAB3-PC12"]),
  click(".alumno__pantalla:not([disabled])"), cuadros(["LAB3-PC07"])) });
await f("07-alumnos-sistema-y-errores", "lab-errores");
await f("08-ficha-alumno", "lab-errores", { pasos: click(`${fila("lab-ubuntu-02")} .ficha`) });
await f("09-ficha-alumno-windows", "lab", { pasos: click(`${fila("LAB3-PC12")} .ficha`) });

// Acciones sobre la selección
await f("10-panel-energia", "lab", { pasos: seq(click(fila("LAB3-PC07")), click('[data-panel="energia"]')) });
await f("11-confirmar-energia", "lab", { pasos: seq(click(fila("LAB3-PC07")), click('[data-panel="energia"]'), click("#energia-ejecutar")) });
await f("12-panel-restricciones-activas", "lab-restr-activa", { pasos: click('[data-panel="restricciones"]') });
await f("13-panel-aviso-error", "lab", { pasos: seq(click('[data-panel="aviso"]'), click("#aviso-enviar")) });
await f("14-transmitiendo", "lab-transmitiendo");
await f("15-energia-pendiente", "lab-energia-pendiente");

// Código
await f("16-codigo", "datos", { ruta: "codigo", pasos: async (p) => {
  await p.evaluate(() => window.monaco && window.monaco.editor.getEditors()[0].setValue(
    "def factorial(n):\n    if n <= 1:\n        return 1\n    return n * factorial(n - 1)\n\nprint(factorial(5))\n"));
  await click(".envio__desplegar")(p);
} });
await f("17-codigo-reemitir", "datos", { ruta: "codigo", pasos: click("#historial-filas .reemitir") });
await f("18-codigo-no-pudo", "lab-errores", { ruta: "codigo", pasos: click(".envio__desplegar") });

// Aula simulada
await f("19-simulada", "simulada", { ruta: "simulada" });
await f("20-alumnos-con-simulados", "simulada");
await f("53-miniaturas-esperando-imagen", "simulada", { pasos: click('[data-modo="miniaturas"]') });

// Ajustes y canal
await f("21-ajustes-clase", "datos", { pasos: click("#nav-ajustes") });
await f("22-ajustes-acerca-clave-personal", "clave-personal", { pasos: ajustes("acerca") });
await f("23-ajustes-acerca-info", "datos", { pasos: seq(ajustes("acerca"), click("#aj-acerca [data-info=clave]")) });
await f("24-ajustes-apariencia-oscuro", "datos", { oscuro: true, pasos: ajustes("apariencia") });
await f("25-cambiar-canal", "datos", { pasos: seq(click("#nav-clase"), async (p) => { await p.type("#cc-canal", "2"); }) });
await f("26-cambiar-canal-error", "datos", { pasos: seq(click("#nav-clase"), async (p) => { await p.type("#cc-canal", "99999"); }, click("#cc-si")) });

// Ayuda y manual
await f("27-ayuda-indice", "datos", { ruta: "ayuda" });
await f("28-ayuda-pagina", "datos", { ruta: "ayuda/primeros-pasos" });
await f("29-ayuda-seccion", "datos", { ruta: "ayuda/solucion-de-problemas/el-firewall-o-la-red-pública" });
await f("52-ayuda-atajos", "datos", { h: 1100, ruta: "ayuda/tareas-frecuentes/atajos-de-teclado" });
await f("30-ayuda-busqueda", "datos", { ruta: "ayuda", pasos: async (p) => { await p.type("#ayuda-buscar", "firewall"); await sleep(600); } });
await f("31-ayuda-contextual-alumnos", "lab", { pasos: click("[data-vista-de=alumnos] [data-ayuda-contextual]") });
await f("32-ayuda-contextual-codigo", "datos", { ruta: "codigo", pasos: click("[data-vista-de=codigo] [data-ayuda-contextual]") });
await f("33-diagrama", "datos", { ruta: "ayuda/como-funciona", pasos: async (p) => {
  await p.evaluate(() => document.querySelector("#ayuda-pagina .enlace").click());
  await sleep(200);
} });
await f("34-error-con-manual", "datos", { pasos: async (p) => p.evaluate(() => window.__emit("aula:capture",
  { draft: null, error: "aula:clipboardEmpty: el portapapeles está vacío: copia el código primero (Ctrl+C)" })) });

// Diagramas: visor, galería, «Ver en grande» y los del «?» de cada vista
const galeria = "ayuda/como-funciona/todos-los-diagramas";
await f("42-diagrama-en-pagina", "datos", { ruta: "ayuda/como-funciona" });
await f("43-galeria", "datos", { ruta: galeria });
await f("44-galeria-filtrada", "datos", { ruta: galeria, pasos: async (p) => {
  await p.evaluate(() => [...document.querySelectorAll(".galeria-diagramas [aria-pressed]")].find((b) => b.textContent === "Profesor").click());
  await p.type(".galeria-diagramas input[type=search]", "canal");
  await sleep(200);
} });
await f("45-visor-grande", "datos", { ruta: galeria, pasos: async (p) => {
  await p.evaluate(() => document.querySelector(".galeria-diagramas__titulo").click());
  await sleep(200);
  await p.keyboard.press("+");
  await sleep(150);
} });
await f("46-visor-grande-oscuro", "datos", { oscuro: true, ruta: galeria, pasos: async (p) => {
  await p.evaluate(() => [...document.querySelectorAll(".galeria-diagramas__titulo")].find((b) => b.textContent.startsWith("Energía")).click());
  await sleep(200);
} });
await f("47-ayuda-contextual-falta", "vacia", { ruta: "falta", pasos: seq(click("[data-vista-de=falta] [data-ayuda-contextual]"),
  async (p) => p.evaluate(() => { const c = document.getElementById("panel-ayuda-cuerpo"); c.scrollTop = c.scrollHeight; })) });

// ¿Falta un alumno?
await f("35-falta-arbol", "vacia", { ruta: "falta" });
await f("36-falta-arbol-respondido", "vacia", { ruta: "falta", pasos: seq(click("#falta-pasos .paso--actual button:nth-child(2)"),
  click("#falta-pasos .paso--actual button:nth-child(2)")) });

// Comprobar la conexión (connectcheck) en «¿Falta un alumno?»
const comprobar = seq(click("#comprobar-iniciar"), async () => sleep(200));
await f("48-comprobar-bien", "vacia", { ruta: "falta", pasos: comprobar });
await f("49-comprobar-red-publica", "red-publica", { ruta: "falta", pasos: comprobar });
await f("50-comprobar-red-publica-oscuro", "red-publica", { oscuro: true, w: 900, h: 600, ruta: "falta", pasos: comprobar });

// Ventana angosta y tema oscuro
await f("37-alumnos-900x600", "lab", { w: 900, h: 600, pasos: click(fila("LAB3-PC07")) });
await f("38-codigo-900x600", "datos", { w: 900, h: 600, ruta: "codigo" });
await f("39-alumnos-oscuro", "lab", { oscuro: true, pasos: click(fila("LAB3-PC07")) });
await f("40-codigo-oscuro", "datos", { oscuro: true, ruta: "codigo" });
await f("41-ficha-alumno-oscuro", "lab-errores", { oscuro: true, pasos: click(`${fila("lab-ubuntu-02")} .ficha`) });

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
if (graves) {
  console.error(`${graves} errores de JS o violaciones serias/críticas de axe`);
  process.exit(1);
}
