// Pruebas de comportamiento de la app del alumno en el arnés web (prompt 19).
// No son capturas: cada caso arma una escena, actúa con el teclado o con
// eventos del backend simulado y comprueba el DOM. Sale con error si alguno
// falla.
//   node pruebas-alumno.mjs
import path from "node:path";
import { abrir, sleep } from "./arnes.mjs";

const { foto, cerrar } = await abrir({ app: "alumno" });
const fallas = [];
const pasaron = [];
const ahora = () => new Date().toISOString();
const fragmento = (id, n, titulo) => ({ id, exercise: String(n), title: titulo, lang: "python", receivedAt: ahora(),
  content: Array.from({ length: 30 }, (_, i) => `linea_${i} = ${i}`).join("\n") + "\n" });

async function caso(nombre, escena, prueba, o = {}) {
  let error = "";
  const r = await foto(path.join("salida-alumno", "prueba-" + nombre + ".png"), escena, Object.assign({ w: 900, h: 700, revisar: false,
    pasos: async (p) => { try { await prueba(p); } catch (e) { error = e.message; } } }, o));
  if (r.errores.length) error = (error ? error + "; " : "") + "JS: " + r.errores.join(" | ");
  (error ? fallas : pasaron).push(nombre + (error ? ": " + error : ""));
}
const ok = (cond, msg) => { if (!cond) throw new Error(msg); };
const emitir = (p, evento, datos) => p.evaluate((e, d) => window.__emit(e, d), evento, datos);
const ids = (p) => p.evaluate(() => [...document.querySelectorAll("#snippets > li")].map((li) => li.dataset.id));

// Lo más nuevo arriba (Go manda el más antiguo primero).
await caso("orden", "varios-codigos", async (p) => {
  const v = await ids(p);
  ok(v.join() === "sn-3,sn-2,sn-1", "orden " + v.join());
  ok(await p.evaluate(() => document.querySelector("#snippets > li").getAttribute("aria-current") === "true"),
    "el más nuevo no está seleccionado");
});

// Lo que ya estaba al abrir no es «Nuevo»; lo que llega después, sí, hasta seleccionarlo.
await caso("nuevo", "varios-codigos", async (p) => {
  ok(await p.evaluate(() => [...document.querySelectorAll(".nuevo")].every((n) => n.hidden)), "al abrir hay «Nuevo»");
  const prev = await p.evaluate(() => window.go.main.App.View().then((v) => v.snippets));
  await emitir(p, "aula:board", { snippets: [...prev, { id: "sn-9", exercise: "9", title: "Otro", lang: "go", content: "package main\n", receivedAt: new Date().toISOString() }] });
  await sleep(150);
  const nuevo = await p.evaluate(() => { const li = document.querySelector("#snippets > li"); return [li.dataset.id, li.querySelector(".nuevo").hidden]; });
  ok(nuevo[0] === "sn-9" && nuevo[1] === false, "el que llegó no dice «Nuevo»: " + nuevo);
  await p.click("#snippets > li .codigo-recibido__titulo");
  ok(await p.evaluate(() => document.querySelector("#snippets > li .nuevo").hidden), "«Nuevo» no se quita al seleccionarlo");
});

// Sin elegir nada, la selección sigue a lo más nuevo (Ctrl+Mayús+C copia lo que
// acaba de llegar); si el alumno eligió otro, se respeta.
await caso("seleccion-sigue", "varios-codigos", async (p) => {
  const prev = await p.evaluate(() => window.go.main.App.View().then((v) => v.snippets));
  const mas = (id) => [...prev, { id, exercise: "9", title: "Otro", lang: "go", content: "x\n", receivedAt: new Date().toISOString() }];
  await emitir(p, "aula:board", { snippets: mas("sn-8") });
  await sleep(100);
  let sel = await p.evaluate(() => document.querySelector('#snippets > li[aria-current="true"]').dataset.id);
  ok(sel === "sn-8", "sin elegir, la selección no pasó al nuevo: " + sel);
  await p.click('#snippets > li[data-id="sn-1"] .codigo-recibido__titulo');
  await emitir(p, "aula:board", { snippets: [...mas("sn-8"), { id: "sn-9", exercise: "10", title: "Más", lang: "go", content: "y\n", receivedAt: new Date().toISOString() }] });
  await sleep(100);
  sel = await p.evaluate(() => document.querySelector('#snippets > li[aria-current="true"]').dataset.id);
  ok(sel === "sn-1", "la elección del alumno no se respetó: " + sel);
});

// Llega código mientras el alumno lee más abajo: lo que ve no se mueve.
await caso("sin-saltos", "varios-codigos", async (p) => {
  await p.evaluate(() => window.scrollTo(0, 400));
  await sleep(150);
  const antes = await p.evaluate(() => {
    const li = document.querySelector('#snippets > li[data-id="sn-2"]');
    return { y: li.getBoundingClientRect().top, scroll: window.scrollY };
  });
  ok(antes.scroll > 0, "la página no se desplazó para la prueba");
  const prev = await p.evaluate(() => window.go.main.App.View().then((v) => v.snippets));
  await emitir(p, "aula:board", { snippets: [...prev, { id: "sn-9", exercise: "9", title: "Otro", lang: "go", content: "a\nb\nc\nd\n", receivedAt: new Date().toISOString() }] });
  await sleep(300);
  const despues = await p.evaluate(() => document.querySelector('#snippets > li[data-id="sn-2"]').getBoundingClientRect().top);
  ok(Math.abs(despues - antes.y) < 2, `lo que se leía se movió de ${antes.y} a ${despues}`);
});

// ↑/↓ cambian de fragmento; Ctrl+Mayús+C copia el seleccionado (lo copia Go, por id).
await caso("teclado", "varios-codigos", async (p) => {
  await p.keyboard.press("ArrowDown");
  await sleep(100);
  let sel = await p.evaluate(() => [document.querySelector('#snippets > li[aria-current="true"]').dataset.id, document.activeElement.dataset.id]);
  ok(sel[0] === "sn-2" && sel[1] === "sn-2", "↓ no pasó al segundo: " + sel);
  await p.keyboard.down("Control"); await p.keyboard.down("Shift"); await p.keyboard.press("KeyC");
  await p.keyboard.up("Shift"); await p.keyboard.up("Control");
  await sleep(150);
  const copias = await p.evaluate(() => window.__calls.filter((c) => c[0] === "Copy").map((c) => c[1][0]));
  ok(copias.join() === "sn-2", "Ctrl+Mayús+C copió " + copias.join());
  ok(await p.evaluate(() => document.querySelector('#snippets > li[data-id="sn-2"] .copy span').textContent === "Copiado"), "sin «Copiado»");
  await p.keyboard.press("ArrowUp");
  sel = await p.evaluate(() => document.querySelector('#snippets > li[aria-current="true"]').dataset.id);
  ok(sel === "sn-3", "↑ no volvió al primero: " + sel);
});

// Las flechas dentro del bloque de código lo desplazan, no cambian de fragmento.
await caso("flechas-en-codigo", "varios-codigos", async (p) => {
  await p.focus('#snippets > li[data-id="sn-3"] pre');
  await p.keyboard.press("ArrowDown");
  const sel = await p.evaluate(() => document.querySelector('#snippets > li[aria-current="true"]').dataset.id);
  ok(sel === "sn-3", "la flecha en el código cambió de fragmento a " + sel);
});

// Llega código: el foco se queda donde estaba el alumno.
await caso("foco", "varios-codigos", async (p) => {
  await p.focus('#snippets > li[data-id="sn-1"] .copy');
  const prev = await p.evaluate(() => window.go.main.App.View().then((v) => v.snippets));
  await emitir(p, "aula:board", { snippets: [...prev, { id: "sn-9", exercise: "9", title: "Otro", lang: "go", content: "x\n", receivedAt: new Date().toISOString() }] });
  await sleep(200);
  const en = await p.evaluate(() => document.activeElement.closest("li") && document.activeElement.closest("li").dataset.id + "|" + document.activeElement.className);
  ok(en && en.startsWith("sn-1|") && en.includes("copy"), "el foco se movió a " + en);
});

// replace deja uno; clear vuelve al vacío.
await caso("replace-clear", "varios-codigos", async (p) => {
  await emitir(p, "aula:board", { snippets: [fragmento("sn-5", 5, "Solo este")] });
  await sleep(100);
  ok((await ids(p)).join() === "sn-5", "replace dejó " + (await ids(p)).join());
  await emitir(p, "aula:board", { snippets: [] });
  await sleep(100);
  ok((await ids(p)).length === 0, "clear dejó fragmentos");
  ok(await p.evaluate(() => !document.getElementById("empty").hidden && document.getElementById("codigos-atajo").hidden), "sin estado vacío tras clear");
});

// Si copiar falla, el motivo se ve junto al código.
await caso("copia-falla", "copia-falla", async (p) => {
  await emitir(p, "aula:board", { snippets: [fragmento("sn-1", 1, "Uno")] });
  await sleep(100);
  await p.click("#snippets > li .copy");
  await sleep(150);
  const t = await p.evaluate(() => { const e = document.querySelector("#snippets > li .codigo-recibido__error"); return e.hidden ? "" : e.textContent; });
  ok(t.includes("portapapeles"), "el motivo no se ve: «" + t + "»");
});

// Modo compacto: el botón encoge la ventana (SetCompact) y muestra la tira;
// Go la devuelve con aula:compact cuando el profesor hace algo que se debe ver.
await caso("compacto", "pantalla-vista", async (p) => {
  await p.click("#compactar");
  await sleep(100);
  ok(await p.evaluate(() => !document.getElementById("compacto").hidden && document.body.classList.contains("compacto")), "no se encogió");
  ok(await p.evaluate(() => window.__calls.some((c) => c[0] === "SetCompact" && c[1][0] === true)), "no avisó a Go");
  ok(await p.evaluate(() => !document.getElementById("compacto-vista").hidden), "en compacto no se ve que la pantalla se está viendo");
  await emitir(p, "aula:compact", { on: false });
  await sleep(100);
  ok(await p.evaluate(() => document.getElementById("compacto").hidden && !document.body.classList.contains("compacto")), "aula:compact no la devolvió");
});

// Lo nuevo sin ver va al frente del título (barra de tareas) y al icono del
// área de notificación (ReportWindow); al verlo, se quita.
await caso("titulo-nuevos", "varios-codigos", async (p) => {
  const prev = await p.evaluate(() => window.go.main.App.View().then((v) => v.snippets));
  await emitir(p, "aula:board", { snippets: [...prev, { id: "sn-9", exercise: "9", title: "Otro", lang: "go", content: "x\n", receivedAt: new Date().toISOString() }] });
  await sleep(100);
  let t = await p.evaluate(() => window.__title);
  ok(t.startsWith("(1) Aula — conectado"), "título: " + t);
  ok(await p.evaluate((x) => window.__calls.some((c) => c[0] === "ReportWindow" && c[1][0] === x), t), "el icono no recibió el título");
  await p.click('#snippets > li[data-id="sn-9"] .codigo-recibido__titulo');
  t = await p.evaluate(() => window.__title);
  ok(t.startsWith("Aula — conectado"), "tras verlo, el título sigue con el contador: " + t);
});

// Avisos: en una esquina, los 3 más recientes y «Ver N más»; Esc cierra el que
// tiene el foco; cuentan como «sin ver» en el título.
await caso("avisos", "muchos-avisos", async (p) => {
  ok(await p.evaluate(() => document.querySelectorAll("#notices .toast").length === 3), "no se ven 3 avisos");
  ok(await p.evaluate(() => document.getElementById("avisos-mas").textContent === "Ver 2 avisos más"), "sin «Ver 2 avisos más»");
  ok(await p.evaluate(() => getComputedStyle(document.getElementById("avisos")).position === "fixed"), "los avisos no están en una esquina");
  ok((await p.evaluate(() => window.__title)).startsWith("(5) "), "el título no cuenta los avisos");
  await p.click("#avisos-mas");
  ok(await p.evaluate(() => document.querySelectorAll("#notices .toast").length === 5), "«Ver más» no mostró todos");
  await p.focus("#notices .toast:last-child .dismiss");
  await p.keyboard.press("Escape");
  await sleep(100);
  ok(await p.evaluate(() => window.__calls.some((c) => c[0] === "DismissNotice" && c[1][0] === "m-5")), "Esc no cerró el aviso con foco");
});

// A 480×360, un solo aviso a la vista.
await caso("avisos-angosta", "muchos-avisos", async (p) => {
  ok(await p.evaluate(() => document.querySelectorAll("#notices .toast").length === 1), "a 480 se ve más de un aviso");
}, { w: 480, h: 360 });

// El estado vacío dice qué pasa según la conexión y su causa.
await caso("vacio-causa", "sin-conexion", async (p) => {
  const t = await p.evaluate(() => [document.getElementById("vacio-titulo").textContent, document.getElementById("vacio-texto").textContent]);
  ok(t[0] === "Esperando a tu profesor" && t[1].startsWith("No se encuentra la clase del canal 1"), "vacío: " + t.join(" / "));
});

// Transparencia (prompt 19, Fase 5.2): cada banda aparece cuando su función
// está activa, no tiene control que la cierre u oculte (Esc tampoco), y se va
// sola cuando el profesor termina.
const PERMITIDOS = ["restr-toggle", "power-postpone", "power-dismiss", "bc-size", "app-closed-ok"];
async function bandaPermanente(p, sel) {
  const b = await p.evaluate((sel, permitidos) => {
    const el = document.querySelector(sel);
    const botones = [...el.querySelectorAll("button")].map((x) => x.id || x.className);
    return { visible: !!el && !el.hidden && el.getClientRects().length > 0, extra: botones.filter((x) => !permitidos.includes(x)) };
  }, sel, PERMITIDOS);
  ok(b.visible, sel + " no se ve");
  ok(b.extra.length === 0, sel + " tiene controles de más: " + b.extra.join());
  await p.keyboard.press("Escape");
  ok(await p.evaluate((sel) => !document.querySelector(sel).hidden, sel), sel + " se ocultó con Esc");
}
await caso("banda-vista", "pantalla-vista", async (p) => {
  await bandaPermanente(p, "#banda-vista");
  await emitir(p, "aula:status", { state: "connected", teacher: "Prof. Ayala", channel: "1", detail: "", cause: "", viewed: false, watching: false, restricted: false });
  ok(await p.evaluate(() => document.getElementById("banda-vista").hidden), "la banda sigue al dejar de ver la pantalla");
});
await caso("banda-restricciones", "restricciones", async (p) => {
  await bandaPermanente(p, "#restrictions");
  await emitir(p, "aula:restrictions", { apps: null, web: null });
  ok(await p.evaluate(() => document.getElementById("restrictions").hidden), "la banda sigue sin restricciones");
});
await caso("banda-energia", "energia-cuenta", async (p) => {
  await bandaPermanente(p, "#power");
  await emitir(p, "aula:power", { notice: null, teacher: "", channel: "" });
  ok(await p.evaluate(() => document.getElementById("power").hidden), "la banda sigue sin orden");
});
await caso("banda-transmision", "transmision", async (p) => {
  await bandaPermanente(p, "#broadcast .franja");
  await emitir(p, "aula:broadcast", { active: false, broadcastId: "", width: 0, height: 0, fps: 0, teacher: "", channel: "" });
  ok(await p.evaluate(() => document.getElementById("broadcast").hidden), "la vista sigue al terminar la transmisión");
});
await caso("bandas-orden", "todo", async (p) => {
  const orden = await p.evaluate(() => [...document.querySelectorAll("#bandas > .franja")].filter((b) => !b.hidden).map((b) => b.id));
  ok(orden.join() === "power,banda-vista,restrictions", "orden " + orden.join());
});

// «Aula cerró…» se queda hasta que el alumno lo lee (ya no se esfuma a los 8 s)
// y junta las apps.
await caso("app-cerrada", "restricciones", async (p) => {
  await emitir(p, "aula:app-closed", { app: "steam" });
  await emitir(p, "aula:app-closed", { app: "discord" });
  await sleep(9000);
  const t = await p.evaluate(() => document.getElementById("app-closed").hidden ? "" : document.getElementById("app-closed-text").textContent);
  ok(t.includes("«steam», «discord»"), "a los 9 s: «" + t + "»");
  await p.click("#app-closed-ok");
  ok(await p.evaluate(() => document.getElementById("app-closed").hidden), "«Entendido» no lo quitó");
  ok(await p.evaluate(() => !document.getElementById("restrictions").hidden), "«Entendido» quitó la banda de restricciones");
});

// Lo que el profesor hace sobre el equipo saca la ventana del modo compacto.
await caso("compacto-sale", "conectado", async (p) => {
  await p.click("#compactar");
  await emitir(p, "aula:restrictions", { apps: { policyId: "p", mode: "deny", items: ["steam"], total: 1 }, web: null });
  await sleep(100);
  ok(await p.evaluate(() => document.getElementById("compacto").hidden && !document.getElementById("restrictions").hidden),
    "las restricciones no sacaron la ventana del modo compacto");
});

await cerrar();
pasaron.forEach((n) => console.log("✓ " + n));
fallas.forEach((n) => console.error("✗ " + n));
console.log(`\n${pasaron.length} pasaron, ${fallas.length} fallaron`);
if (fallas.length) process.exit(1);
