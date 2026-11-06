// Recorre la consola del profesor solo con el teclado, en el arnés web
// (prompt 18, Fase 7.1). Por cada vista:
//   - Tab avanza de un control a otro sin perder el foco (nunca en <body>) y
//     el foco siempre se ve (contorno o sombra);
//   - no hay trampas: desde cualquier punto se llega de nuevo al menú. El
//     editor de código atrapa Tab a propósito (sangra), y se sale con Ctrl+M y
//     Tab, como dice el manual;
//   - en paneles y diálogos, Esc cierra y el foco vuelve a quien lo abrió.
// Sale con error si algo falla.
//   node teclado.mjs
import { abrir, sleep } from "./arnes.mjs";

const MAX_TABS = 160;
const { foto, cerrar } = await abrir();
const fallas = [];
const informe = [];

// Describe el elemento con foco y si su foco se ve.
const enfocado = (p) => p.evaluate(() => {
  const e = document.activeElement;
  if (!e || e === document.body || e === document.documentElement) return { nada: true };
  const cs = getComputedStyle(e);
  const r = e.getBoundingClientRect();
  const contorno = cs.outlineStyle !== "none" && parseFloat(cs.outlineWidth) > 0;
  const sombra = cs.boxShadow && cs.boxShadow !== "none";
  const nombre = e.id ? "#" + e.id : e.tagName.toLowerCase() + (e.className && typeof e.className === "string" ? "." + e.className.trim().split(/\s+/).join(".") : "");
  return {
    nombre, texto: (e.getAttribute("aria-label") || e.textContent || "").trim().slice(0, 40),
    visible: r.width > 0 && r.height > 0, seVe: contorno || sombra,
    editor: !!e.closest(".monaco-editor"), menu: !!e.closest(".nav-lateral"),
    dialogo: e.closest("dialog[open]") ? e.closest("dialog[open]").id : "",
  };
});

// Tab por la vista: cuenta los controles, revisa cada foco y comprueba que se
// vuelve al menú.
async function recorrerVista(nombre, escena, ruta) {
  await foto(`salida-teclado/${nombre}.webp`, escena, { ruta, revisar: false, pasos: async (p) => {
    // Un ciclo completo: de vuelta al primer control. Si no vuelve, hay trampa.
    let pasos = 0, enEditor = 0, primero = "", vuelta = false, menu = false, fuera = false;
    const vistos = new Set();
    for (; pasos < MAX_TABS; pasos++) {
      await p.keyboard.press("Tab");
      const f = await enfocado(p);
      if (f.nada) {
        // Tras el último control el navegador saca el foco de la página: es la
        // vuelta, y el siguiente Tab debe entrar de nuevo. Dos seguidos: se perdió.
        if (fuera) { fallas.push(`${nombre}: el foco se perdió tras ${pasos + 1} Tab`); return; }
        fuera = true;
        continue;
      }
      fuera = false;
      if (f.editor) {
        // Dentro del editor, Tab sangra: se sale con Ctrl+M y Tab.
        if (++enEditor > 1) { fallas.push(`${nombre}: Ctrl+M y Tab no sacan del editor`); return; }
        await p.keyboard.down("Control"); await p.keyboard.press("m"); await p.keyboard.up("Control");
        continue;
      }
      const clave = f.nombre + "|" + f.texto;
      if (clave === primero) { vuelta = true; break; }
      if (!primero) primero = clave;
      if (!f.visible) fallas.push(`${nombre}: ${f.nombre} recibe el foco pero no se ve`);
      else if (!f.seVe) fallas.push(`${nombre}: el foco en ${f.nombre} («${f.texto}») no se ve`);
      if (f.menu) menu = true;
      vistos.add(clave);
    }
    if (!vuelta || !menu) fallas.push(`${nombre}: tras ${MAX_TABS} Tab no se volvió al principio (¿trampa de foco?)`);
    informe.push(`${nombre}: ${vistos.size} controles` + (enEditor ? "; del editor se sale con Ctrl+M y Tab" : ""));
  } });
}

// Abre algo con su botón (Enter), comprueba que Esc lo cierra y que el foco
// vuelve al botón.
async function escCierra(nombre, escena, ruta, boton, abierto) {
  await foto(`salida-teclado/${nombre}.webp`, escena, { ruta, revisar: false, pasos: async (p) => {
    await p.focus(boton);
    await p.keyboard.press("Enter");
    await sleep(300);
    if (!(await p.evaluate(abierto))) { fallas.push(`${nombre}: Enter en ${boton} no lo abrió`); return; }
    await p.keyboard.press("Tab"); // el foco ya está adentro; se mueve un poco
    await p.keyboard.press("Escape");
    await sleep(300);
    if (await p.evaluate(abierto)) { fallas.push(`${nombre}: Esc no lo cerró`); return; }
    const vuelve = await p.evaluate((b) => document.activeElement === document.querySelector(b), boton);
    if (!vuelve) fallas.push(`${nombre}: tras Esc el foco no volvió a ${boton} (está en ${(await enfocado(p)).nombre})`);
    informe.push(`${nombre}: Esc cierra y el foco vuelve`);
  } });
}

await recorrerVista("alumnos", "lab", "alumnos");
await recorrerVista("codigo", "datos", "codigo");
await recorrerVista("simulada", "simulada", "simulada");
await recorrerVista("ayuda", "datos", "ayuda");
await recorrerVista("falta", "vacia", "falta");

await escCierra("panel-aviso", "lab", "alumnos", '[data-panel="aviso"]', () => !document.getElementById("panel-aviso").hidden);
await escCierra("panel-energia", "lab", "alumnos", '[data-panel="energia"]', () => !document.getElementById("panel-energia").hidden);
await escCierra("panel-restricciones", "lab", "alumnos", '[data-panel="restricciones"]', () => !document.getElementById("panel-restricciones").hidden);
await escCierra("ayuda-contextual", "lab", "alumnos", "[data-vista-de=alumnos] [data-ayuda-contextual]", () => !document.getElementById("panel-ayuda").hidden);
await escCierra("cambiar-canal", "datos", "alumnos", "#nav-clase", () => document.getElementById("cambiar-canal").open);
await escCierra("ajustes", "datos", "alumnos", "#nav-ajustes", () => document.getElementById("ajustes").open);

await cerrar();
informe.forEach((l) => console.log("✓ " + l));
if (fallas.length) {
  fallas.forEach((l) => console.error("✗ " + l));
  console.error(`\n${fallas.length} problemas de teclado`);
  process.exit(1);
}
console.log("\nTeclado: sin problemas");
