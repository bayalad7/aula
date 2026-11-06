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

// Captura de pantalla (docs/privacidad.md regla 44): un aviso en la esquina, no una
// banda. La ventana arma la frase con el profesor y la hora; cuenta como «sin ver», no
// quita el foco y se cierra con «Entendido» o Esc, como cualquier aviso.
await caso("aviso-captura", "un-codigo", async (p) => {
  await p.focus('#snippets > li .copy');
  await emitir(p, "aula:notices", { notices: [{ id: "c-1", level: "warning", text: "", kind: "snapshot", at: ahora() }] });
  await sleep(150);
  const a = await p.evaluate(() => {
    const t = document.querySelector('#notices .toast[data-kind="snapshot"]');
    return t && { titulo: t.querySelector(".toast__titulo").textContent, texto: t.querySelector(".notice-text").textContent,
      icono: t.querySelector("use").getAttribute("href"), enBandas: !!t.closest(".bandas"), foco: document.activeElement.className };
  });
  ok(a, "no apareció el aviso de captura");
  ok(a.titulo === "Captura de pantalla", "título: " + a.titulo);
  ok(a.texto.startsWith("Prof. Ayala (canal 1) tomó una captura de tu pantalla ("), "texto: " + a.texto);
  ok(a.icono.endsWith("#captura"), "icono: " + a.icono);
  ok(!a.enBandas, "el aviso de captura no es una banda");
  ok(a.foco.includes("copy"), "el aviso le quitó el foco al alumno: " + a.foco);
  ok((await p.evaluate(() => window.__title)).startsWith("(1) "), "el título no lo cuenta como sin ver");
  await p.focus('#notices .toast[data-kind="snapshot"] .dismiss');
  await p.keyboard.press("Escape");
  await sleep(100);
  ok(await p.evaluate(() => window.__calls.some((c) => c[0] === "DismissNotice" && c[1][0] === "c-1")), "Esc no cerró el aviso de captura");
});

// Levantar la mano y escribir al profesor (prompt 10.6): lo hace el alumno, con el teclado;
// nada de esto es una banda ni le quita el foco, y lo que llega del profesor tampoco.
const bandasVisibles = (p) => p.evaluate(() => [...document.querySelectorAll("#bandas > *")].filter((b) => !b.hidden).length);
await caso("mano-levantar-y-bajar", "conectado", async (p) => {
  await p.focus("#mano");
  await p.keyboard.press("Enter");
  await sleep(150);
  let m = await p.evaluate(() => ({ pulsado: document.getElementById("mano").getAttribute("aria-pressed"), foco: document.activeElement.id,
    linea: !document.getElementById("mano-estado").hidden, compacto: document.getElementById("compacto-mano").getAttribute("aria-pressed") }));
  ok(m.pulsado === "true" && m.compacto === "true", "aria-pressed no cambió: " + JSON.stringify(m));
  ok(m.foco === "mano", "el foco se fue de «Levantar la mano»: " + m.foco);
  ok(m.linea, "no dice que la mano está levantada");
  ok(await bandasVisibles(p) === 0, "levantar la mano mostró una banda");
  ok(await p.evaluate(() => window.__calls.some((c) => c[0] === "RaiseHand")), "no llamó a RaiseHand");
  await p.keyboard.press("Enter");
  await sleep(150);
  ok(await p.evaluate(() => window.__calls.some((c) => c[0] === "LowerHand")), "no llamó a LowerHand");
  ok(await p.evaluate(() => document.getElementById("mano").getAttribute("aria-pressed") === "false"), "la mano no bajó");
});
await caso("mano-bajada-por-profesor", "mano", async (p) => {
  await p.focus("#escribir");
  await emitir(p, "aula:participation", { hand: "lowered_by_teacher", note: "", messages: [], unsupported: false });
  await sleep(150);
  const m = await p.evaluate(() => ({ pulsado: document.getElementById("mano").getAttribute("aria-pressed"),
    texto: document.getElementById("mano-estado-texto").textContent, foco: document.activeElement.id }));
  ok(m.pulsado === "false", "el botón sigue pulsado");
  ok(m.texto === "Tu profesor bajó tu mano.", "texto: " + m.texto);
  ok(m.foco === "escribir", "le quitó el foco al alumno: " + m.foco);
  ok(await bandasVisibles(p) === 0, "bajar la mano mostró una banda");
});
await caso("mano-motivo", "mano", async (p) => {
  await p.click("#motivo-abrir");
  ok(await p.evaluate(() => document.activeElement.id === "motivo-texto"), "el foco no pasó al motivo");
  ok(await p.evaluate(() => document.activeElement.value === "No me compila el for de la línea 12"), "el campo no trae el motivo actual");
  await p.evaluate(() => { document.activeElement.value = ""; });
  await p.keyboard.type("Necesito ayuda con el for");
  await p.keyboard.press("Enter");
  await sleep(150);
  const c = await p.evaluate(() => window.__calls.filter((x) => x[0] === "RaiseHand").pop());
  ok(c && c[1][0] === "Necesito ayuda con el for", "RaiseHand con el motivo: " + JSON.stringify(c));
});
await caso("mensaje-enter-y-esc", "conectado", async (p) => {
  await p.click("#escribir");
  ok(await p.evaluate(() => document.activeElement.id === "redactar-texto"), "el foco no pasó al mensaje");
  await p.keyboard.type("borrador");
  await p.keyboard.press("Escape");
  await sleep(100);
  let m = await p.evaluate(() => ({ abierta: !document.getElementById("redactar").hidden, foco: document.activeElement.id,
    enviados: window.__calls.filter((c) => c[0] === "SendTeacherMessage").length }));
  ok(!m.abierta && m.foco === "escribir" && m.enviados === 0, "Esc no canceló: " + JSON.stringify(m));
  await p.click("#escribir");
  await p.evaluate(() => { document.getElementById("redactar-texto").value = ""; });
  await p.keyboard.type("¿Puede repetir el último paso?");
  await p.keyboard.press("Enter");
  await sleep(150);
  m = await p.evaluate(() => ({ abierta: !document.getElementById("redactar").hidden, foco: document.activeElement.id,
    estado: document.querySelector("#enviados-lista .insignia").textContent,
    llamada: window.__calls.filter((c) => c[0] === "SendTeacherMessage").pop() }));
  ok(m.llamada && m.llamada[1][0] === "¿Puede repetir el último paso?", "Enter no envió: " + JSON.stringify(m.llamada));
  ok(!m.abierta && m.foco === "escribir", "tras enviar la tarjeta sigue abierta o el foco se perdió: " + m.foco);
  ok(m.estado === "Enviando…", "estado: " + m.estado);
  await sleep(600);
  ok(await p.evaluate(() => document.querySelector("#enviados-lista .insignia").textContent === "Recibido"), "no pasó a «Recibido»");
});
await caso("mensaje-vacio", "conectado", async (p) => {
  await p.click("#escribir");
  await p.keyboard.press("Enter");
  await sleep(150);
  const e = await p.evaluate(() => [document.getElementById("redactar-error").hidden, document.getElementById("redactar-error").textContent,
    document.activeElement.id]);
  ok(!e[0] && e[1] === "Escribe tu mensaje antes de enviarlo." && e[2] === "redactar-texto", "error: " + e.join(" / "));
});
await caso("respuesta-del-profesor", "mensajes", async (p) => {
  const a = await p.evaluate(() => {
    const t = [...document.querySelectorAll("#notices .toast")].find((x) => x.querySelector(".toast__titulo").textContent === "Respuesta a tu mensaje");
    return t && { cita: t.querySelector(".notice-cita").textContent, oculta: t.querySelector(".notice-cita").hidden, enBandas: !!t.closest(".bandas") };
  });
  ok(a, "no apareció «Respuesta a tu mensaje»");
  ok(!a.oculta && a.cita === "Tú escribiste: «¿El ejercicio 2 es para hoy?»", "cita: " + a.cita);
  ok(!a.enBandas, "la respuesta es una banda");
});
await caso("mano-sin-conexion", "buscando", async (p) => {
  const d = await p.evaluate(() => [document.getElementById("mano").disabled, document.getElementById("escribir").disabled,
    document.getElementById("mano").title]);
  ok(d[0] && d[1], "los botones no se desactivan sin conexión");
  ok(d[2].startsWith("Podrás levantar la mano"), "no dice por qué: " + d[2]);
});
await caso("mano-compacto", "conectado", async (p) => {
  await p.click("#compactar");
  await p.click("#compacto-mano");
  await sleep(150);
  ok(await p.evaluate(() => document.body.classList.contains("compacto")), "levantar la mano sacó la ventana del modo compacto");
  ok(await p.evaluate(() => document.getElementById("compacto-mano").getAttribute("aria-pressed") === "true"), "la mano no se levantó en compacto");
  await p.click("#compacto-escribir");
  await sleep(150);
  const m = await p.evaluate(() => [document.body.classList.contains("compacto"), document.activeElement.id]);
  ok(!m[0] && m[1] === "redactar-texto", "«Mensaje al profesor» no abrió la tarjeta: " + m.join());
}, { w: 380, h: 80 });

// Mensaje privado (prompt 10.7): se distingue de un aviso a todos, no quita el foco y no es banda.
await caso("aviso-privado", "un-codigo", async (p) => {
  await p.focus("#snippets > li .copy");
  await emitir(p, "aula:notices", { notices: [
    { id: "m-1", level: "info", text: "Para todos", at: ahora() },
    { id: "m-p", level: "info", text: "Solo para ti", at: ahora(), personal: true }] });
  await sleep(150);
  const a = await p.evaluate(() => [...document.querySelectorAll("#notices .toast")].map((t) => ({
    titulo: t.querySelector(".toast__titulo").textContent, icono: t.querySelector("use").getAttribute("href"),
    personal: t.dataset.personal === "true", enBandas: !!t.closest(".bandas") })));
  const priv = a.find((x) => x.personal);
  ok(priv && priv.titulo === "Mensaje para ti" && priv.icono.endsWith("#sobre"), "privado: " + JSON.stringify(priv));
  ok(a.find((x) => !x.personal).titulo === "Aviso", "el aviso a todos cambió de título");
  ok(!priv.enBandas, "el privado es una banda");
  ok(await p.evaluate(() => document.activeElement.classList.contains("copy")), "el privado le quitó el foco al alumno");
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
// Control remoto (reglas 37-41): la banda no se cierra ni corta el control; al soltar se va,
// y el aviso de fin aparece en la esquina sin quitarle el foco al alumno.
await caso("banda-control", "control", async (p) => {
  await bandaPermanente(p, "#banda-control");
  const t = await p.evaluate(() => [document.getElementById("banda-control-texto").textContent, document.title]);
  ok(t[0].startsWith("Prof. Ayala (canal 1) está controlando tu equipo"), "banda: " + t[0]);
  ok(t[1] === "Aula — Prof. Ayala controla este equipo (canal 1)", "título: " + t[1]);
  await emitir(p, "aula:status", { state: "connected", teacher: "Prof. Ayala", channel: "1", detail: "", cause: "", viewed: true, watching: false, restricted: false, control: "paused" });
  ok((await p.evaluate(() => document.getElementById("banda-control-texto").textContent)).includes("en pausa"), "la pausa no se dice");
  await p.focus(".copy");
  await emitir(p, "aula:status", { state: "connected", teacher: "Prof. Ayala", channel: "1", detail: "", cause: "", viewed: false, watching: false, restricted: false, control: "" });
  const fin = await p.evaluate(() => ({ banda: document.getElementById("banda-control").hidden, aviso: !document.getElementById("control-fin").hidden,
    foco: document.activeElement.classList.contains("copy"), titulo: document.title }));
  ok(fin.banda, "la banda sigue al soltar");
  ok(fin.aviso, "no se avisó el fin del control");
  ok(fin.foco, "el aviso de fin le quitó el foco al alumno");
  ok(fin.titulo.startsWith("Aula — conectado"), "título al soltar: " + fin.titulo);
  await p.click("#control-fin-ok");
  ok(await p.evaluate(() => document.getElementById("control-fin").hidden), "«Entendido» no quitó el aviso de fin");
});
// Tomar el control saca la ventana del modo compacto; dentro del modo compacto, se ve.
await caso("compacto-control", "conectado", async (p) => {
  await p.click("#compactar");
  await emitir(p, "aula:status", { state: "connected", teacher: "Prof. Ayala", channel: "1", detail: "", cause: "", viewed: true, watching: false, restricted: false, control: "active" });
  ok(await p.evaluate(() => document.getElementById("compacto").hidden && !document.getElementById("banda-control").hidden),
    "el control no sacó la ventana del modo compacto");
  await p.click("#compactar");
  ok(await p.evaluate(() => !document.getElementById("compacto-control").hidden && document.getElementById("compacto-vista").hidden),
    "en el modo compacto no se ve «Controlado»");
});
await caso("bandas-orden", "todo", async (p) => {
  const orden = await p.evaluate(() => [...document.querySelectorAll("#bandas > .franja")].filter((b) => !b.hidden).map((b) => b.id));
  ok(orden.join() === "power,banda-control,banda-vista,restrictions", "orden " + orden.join());
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

// Teclado (Fase 4.1): con Tab se recorre la ventana entera y se vuelve al
// principio (sin trampas), el foco nunca se pierde y siempre se ve.
async function cicloTab(p, nombre) {
  let primero = "", fuera = false;
  for (let i = 0; i < 80; i++) {
    await p.keyboard.press("Tab");
    const f = await p.evaluate(() => {
      const e = document.activeElement;
      if (!e || e === document.body) return null;
      const cs = getComputedStyle(e);
      return { k: (e.id || e.className) + "|" + (e.textContent || "").trim().slice(0, 20),
        ve: (cs.outlineStyle !== "none" && parseFloat(cs.outlineWidth) > 0) || cs.boxShadow !== "none" };
    });
    if (!f) { ok(!fuera, nombre + ": el foco se perdió"); fuera = true; continue; }
    fuera = false;
    if (f.k === primero) return;
    if (!primero) primero = f.k;
    ok(f.ve, nombre + ": el foco en " + f.k + " no se ve");
  }
  throw new Error(nombre + ": tras 80 Tab no volvió al principio");
}
await caso("teclado-codigos", "varios-codigos", (p) => cicloTab(p, "códigos"));
await caso("teclado-bandas", "todo", (p) => cicloTab(p, "bandas"));
await caso("teclado-avisos", "muchos-avisos", (p) => cicloTab(p, "avisos"));
await caso("teclado-compacto", "conectado", async (p) => {
  await p.click("#compactar");
  ok(await p.evaluate(() => document.activeElement.id === "restaurar"), "al compactar, el foco no pasó a «Volver al tamaño normal»");
  await cicloTab(p, "compacto");
});

// Rendimiento (Fase 4.2): en reposo no corre ninguna animación ni temporizador
// visible; con «reducir movimiento», las transiciones duran 0.
await caso("sin-animaciones", "varios-codigos", async (p) => {
  await sleep(500);
  const n = await p.evaluate(() => document.getAnimations().filter((a) => a.playState === "running").length);
  ok(n === 0, n + " animaciones corriendo en reposo");
});
await caso("reducir-movimiento", "conectado", async (p) => {
  await p.emulateMediaFeatures([{ name: "prefers-reduced-motion", value: "reduce" }]);
  const t = await p.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue("--transicion").trim());
  ok(t === "0ms", "--transicion con reducir movimiento: " + t);
});

await cerrar();
pasaron.forEach((n) => console.log("✓ " + n));
fallas.forEach((n) => console.error("✗ " + n));
console.log(`\n${pasaron.length} pasaron, ${fallas.length} fallaron`);
if (fallas.length) process.exit(1);
