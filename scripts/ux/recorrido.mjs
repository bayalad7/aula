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
// Control remoto (docs/protocol.md §5.16): la ficha, la vista en vivo, la franja, la pausa y el fin.
const tomar = seq(click(`${fila("LAB3-PC07")} .ficha`), click("#ficha-tomar"), async () => { await sleep(400); });
await f("54-ficha-tomar-control", "lab", { pasos: click(`${fila("LAB3-PC07")} .ficha`) });
await f("55-control-vista", "lab", { pasos: tomar });
await f("56-control-franja", "lab-control");
await f("57-control-pausa", "lab-control-pausa", { pasos: seq(click("#f-control-ver"), async () => { await sleep(300); }) });
await f("58-control-oscuro-900x600", "lab-control", { oscuro: true, w: 900, h: 600, pasos: seq(click("#f-control-ver"), async () => { await sleep(300); }) });
await f("59-control-terminado", "lab", { pasos: seq(tomar, click("#pg-soltar"), async () => { await sleep(400); }) });
const grande = seq(click('[data-modo="miniaturas"]'), cuadros(["LAB3-PC07", "LAB3-PC12"]), click(".alumno__pantalla:not([disabled])"), cuadros(["LAB3-PC07"]));
await f("60-miniatura-grande-tomar-control", "lab", { pasos: grande });
await f("61-miniatura-grande-controlando", "lab", { pasos: seq(grande, click("#pg-tomar"), async () => { await sleep(400); }) });

// Captura de la pantalla de un alumno (docs/protocol.md §5.17): la ficha, la vista previa, guardada,
// fallida, en lote, durante el control y la carpeta en Ajustes › Acerca de.
const espera = async () => { await sleep(400); };
const capturar = seq(click(`${fila("LAB3-PC07")} .ficha`), click("#ficha-capturar"), espera);
await f("62-ficha-tomar-captura", "lab", { pasos: click(`${fila("LAB3-PC07")} .ficha`) });
await f("63-captura-vista-previa", "lab", { pasos: capturar });
await f("64-captura-guardada", "lab", { pasos: seq(capturar, click("#cap-guardar"), espera) });
await f("65-captura-fallida", "lab-captura-falla", { pasos: seq(click(`${fila("LAB3-PC12")} .ficha`), click("#ficha-capturar"), espera) });
await f("66-captura-tamano-real", "lab", { pasos: seq(capturar, click("#cap-marco"), espera) });
await f("67-captura-lote", "lab", { pasos: seq(click(fila("LAB3-PC07")), click(fila("LAB3-PC12")), click(fila("lab-ubuntu-02")),
  click('[data-panel="captura"]'), click("#lote-tomar"), espera) });
await f("68-captura-lote-guardadas", "lab", { pasos: seq(click('[data-panel="captura"]'), click("#lote-tomar"), espera,
  click("#lote-guardar"), espera) });
await f("69-captura-durante-control", "lab", { pasos: seq(tomar, click("#pg-capturar"), espera) });
await f("70-captura-oscuro-900x600", "lab", { oscuro: true, w: 900, h: 600, pasos: seq(capturar, click("#cap-guardar"), espera) });
await f("71-ajustes-carpeta-capturas", "datos", { pasos: seq(ajustes("acerca"), async (p) => {
  await p.evaluate(() => document.getElementById("aj-capturas-ruta").scrollIntoView());
  await sleep(200);
}) });

// Participación: manos y mensajes (docs/protocol.md §5.18): la fila, la insignia en Alumnos,
// responder, el aviso al llegar una mano, la ventana angosta y oscura, y los simulados.
const responder = seq(click("#mensajes-lista .responder"), async (p) => { await p.type("#mensajes-lista .respuesta-texto", "Es para hoy, antes de las 12."); });
await f("72-participacion", "lab-participacion", { ruta: "participacion" });
await f("73-participacion-insignias-alumnos", "lab-participacion");
await f("74-participacion-respondiendo", "lab-participacion", { ruta: "participacion", pasos: responder });
await f("75-participacion-respondido", "lab-participacion", { ruta: "participacion", pasos: seq(responder, async (p) => {
  await p.keyboard.press("Enter");
  await sleep(300);
}) });
await f("76-participacion-llega-mano", "lab-participacion", { pasos: async (p) => {
  await p.evaluate(() => window.__llegaMano("lab-ubuntu-02"));
  await sleep(200);
} });
await f("77-participacion-vacia", "lab", { ruta: "participacion" });
await f("78-participacion-oscuro-900x600", "lab-participacion", { oscuro: true, w: 900, h: 600, ruta: "participacion" });
await f("79-participacion-ficha", "lab-participacion", { pasos: click(`${fila("LAB3-PC12")} .ficha`) });
// Mensaje privado a un alumno (prompt 10.7): la ficha, escribiendo, el acuse y oscuro a 900×600.
const privado = seq(click(`${fila("LAB3-PC07")} .ficha`), click("#privado-abrir"),
  async (p) => { await p.type("#privado-texto", "Revisa la línea 12: falta un dos puntos"); });
await f("81-privado-escribiendo", "lab", { pasos: privado });
await f("82-privado-enviado", "lab", { pasos: seq(privado, async (p) => { await p.keyboard.press("Enter"); await sleep(400); }) });
await f("83-privado-ultimos-envios", "lab", { pasos: seq(privado, async (p) => {
  await p.keyboard.press("Enter");
  await sleep(400);
  await p.evaluate(() => { location.hash = "codigo"; });
  await sleep(300);
}) });
await f("84-privado-oscuro-900x600", "lab", { oscuro: true, w: 900, h: 600, pasos: privado });
// Durante el control remoto: el campo arriba de su pantalla; lo escrito no le llega.
const privadoControl = seq(tomar, click("#pg-privado-abrir"), async (p) => { await p.type("#pg-privado-texto", "Fíjate en la línea 12"); });
// Lo que se escribe en el mensaje no le llega al alumno: ninguna tecla sale por SendRemoteInput
// (si saliera, se registra como error de la página y el recorrido falla).
await f("85-privado-durante-control", "lab", { pasos: seq(privadoControl, async (p) => {
  await sleep(200);
  await p.evaluate(() => {
    const teclas = window.__calls.filter((c) => c[0] === "SendRemoteInput").flatMap((c) => c[1][0] || []).filter((e) => e.t === "key" && e.down);
    if (teclas.length) console.error("el mensaje privado mandó teclas al alumno: " + teclas.map((e) => e.code).join(","));
    if (document.activeElement.id !== "pg-privado-texto") console.error("el foco no está en el mensaje: " + document.activeElement.id);
  });
}) });
await f("86-privado-durante-control-enviado", "lab", { pasos: seq(privadoControl, async (p) => { await p.keyboard.press("Enter"); await sleep(400); }) });
await f("80-simulada-mano-y-mensaje", "simulada", { ruta: "simulada", pasos: seq(click("#sim-filas .mano"), async () => { await sleep(200); }) });

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
