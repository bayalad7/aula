// Recorre la consola del profesor en el arnés web: captura cada pantalla y
// estado, junta los errores de JS y pasa axe-core (WCAG 2.1 AA). Sale con
// error si hay errores de JS o violaciones serias o críticas.
//   node recorrido.mjs [carpeta de salida] [--solo=archivos,94-105]   (por omisión, ./salida)
import path from "node:path";
import { abrir, carpetaSalida, soloCapturas, click, cuadros, seq, sleep } from "./arnes.mjs";

const salida = path.resolve(carpetaSalida("salida"));
const incluir = soloCapturas();
const { foto, cerrar } = await abrir();
const informe = {};
const f = async (nombre, escena, o) => { if (!incluir(nombre)) return; informe[nombre] = await foto(path.join(salida, nombre + ".webp"), escena, o); };
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
// Control remoto (docs/protocol.md §5.16): desde la Pantalla grande (las acciones sobre un alumno ya
// no están en la ficha, nota de interfaz 5), la vista en vivo, la franja, la pausa y el fin.
const grande = seq(click('[data-modo="miniaturas"]'), cuadros(["LAB3-PC07", "LAB3-PC12"]), click(".alumno__pantalla:not([disabled])"), cuadros(["LAB3-PC07"]));
const grandeDe = (host) => seq(click('[data-modo="miniaturas"]'), cuadros(["LAB3-PC07", "LAB3-PC12"]),
  click(`#alumnos-cuadricula [data-clave="r:${host}"] .alumno__pantalla`), cuadros([host]));
const tomar = seq(grande, click("#pg-tomar"), async () => { await sleep(400); });
await f("54-ficha-alumno", "lab", { pasos: click(`${fila("LAB3-PC07")} .ficha`) });
await f("55-control-vista", "lab", { pasos: tomar });
await f("56-control-franja", "lab-control");
await f("57-control-pausa", "lab-control-pausa", { pasos: seq(click("#f-control-ver"), async () => { await sleep(300); }) });
await f("58-control-oscuro-900x600", "lab-control", { oscuro: true, w: 900, h: 600, pasos: seq(click("#f-control-ver"), async () => { await sleep(300); }) });
await f("59-control-terminado", "lab", { pasos: seq(tomar, click("#pg-soltar"), async () => { await sleep(400); }) });
await f("60-miniatura-grande-tomar-control", "lab", { pasos: grande });
await f("61-miniatura-grande-controlando", "lab", { pasos: seq(grande, click("#pg-tomar"), async () => { await sleep(400); }) });

// Captura de la pantalla de un alumno (docs/protocol.md §5.17): desde la Pantalla grande, la vista
// previa, guardada, fallida, en lote, durante el control y la carpeta en Ajustes › Acerca de.
const espera = async () => { await sleep(400); };
const capturar = seq(grande, click("#pg-capturar"), espera);
await f("63-captura-vista-previa", "lab", { pasos: capturar });
await f("64-captura-guardada", "lab", { pasos: seq(capturar, click("#cap-guardar"), espera) });
await f("65-captura-fallida", "lab-captura-falla", { pasos: seq(grandeDe("LAB3-PC12"), click("#pg-capturar"), espera) });
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

// Temporizador de ejercicio (prompt 10.8): panel, franja mientras corre, extender, fin y Participación.
const panelTemporizador = seq(click(fila("LAB3-PC07")), click(fila("lab-ubuntu-02")), click('[data-panel="temporizador"]'), async (p) => {
  await p.type("#temporizador-etiqueta", "Ejercicio 1");
});
await f("87-panel-temporizador", "lab", { pasos: panelTemporizador });
await f("88-temporizador-franja", "lab", { pasos: seq(panelTemporizador, click('[data-minutos="5"]'), click("#temporizador-iniciar"),
  async () => { await sleep(300); }) });
await f("89-temporizador-extender", "lab-temporizador", { pasos: click("#f-temporizador-extender") });
await f("90-temporizador-termino", "lab-temporizador-fin");
await f("91-participacion-temporizador", "lab-temporizador", { ruta: "participacion" });
await f("92-temporizador-oscuro-900x600", "lab-temporizador", { oscuro: true, w: 900, h: 600, pasos: seq(click('[data-panel="temporizador"]')) });
await f("93-temporizador-reemplaza", "lab-temporizador", { pasos: click('[data-panel="temporizador"]') });
await f("79-participacion-ficha", "lab-participacion", { pasos: click(`${fila("LAB3-PC12")} .ficha`) });
// Mensaje privado a un alumno (prompt 10.7): su diálogo desde la Pantalla grande (nota de interfaz 7),
// escribiendo, el acuse y oscuro a 900×600.
const privado = seq(grande, click("#pg-privado-abrir"),
  async (p) => { await p.type("#pg-privado-texto", "Revisa la línea 12: falta un dos puntos"); });
await f("81-privado-escribiendo", "lab", { pasos: privado });
await f("82-privado-enviado", "lab", { pasos: seq(privado, async (p) => { await p.keyboard.press("Enter"); await sleep(400); }) });
await f("83-privado-ultimos-envios", "lab", { pasos: seq(privado, async (p) => {
  await p.keyboard.press("Enter");
  await sleep(400);
  await p.evaluate(() => { location.hash = "codigo"; });
  await sleep(300);
}) });
await f("84-privado-oscuro-900x600", "lab", { oscuro: true, w: 900, h: 600, pasos: privado });
// Durante el control remoto: el diálogo sobre su pantalla; lo escrito no le llega.
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

// Archivos y plantillas (prompt 10.9): la vista Archivos del menú.
const archivos = async () => {};
const plantilla = (nombre) => async (p) => { await p.select("#archivos-plantilla", nombre); await sleep(150); };
await f("94-archivos-vacia", "lab", { ruta: "archivos", pasos: archivos });
await f("95-archivos-elegidos", "lab", { ruta: "archivos", pasos: seq(archivos, click("#archivos-elegir"),
  async (p) => { await p.type("#archivos-destino", "Ejercicio 4"); }) });
await f("96-archivos-plantilla", "lab-archivos", { ruta: "archivos", pasos: seq(archivos, plantilla("Ejercicio 4")) });
await f("97-archivos-plantilla-problema", "lab", { ruta: "archivos", pasos: seq(archivos, plantilla("Proyecto final")) });
// Nota de interfaz 17: una plantilla nueva se crea en su sección, en un diálogo; empieza con los
// archivos sueltos elegidos arriba.
await f("98-archivos-nueva-plantilla", "lab", { ruta: "archivos", pasos: seq(archivos, click("#archivos-elegir"),
  click("#plantillas-nueva"), async (p) => { await p.type("#plantilla-nombre", "Ejercicio 5"); }) });
await f("98b-archivos-que-envias", "lab-archivos", { ruta: "archivos", pasos: seq(archivos, click('input[name="archivos-que"][value="plantilla"]')) });
await f("98c-archivos-nueva-plantilla-oscuro-900x600", "lab", { ruta: "archivos", oscuro: true, w: 900, h: 600, pasos: click("#plantillas-nueva") });
await f("99-archivos-enviado", "lab", { ruta: "archivos", pasos: seq(archivos, click("#archivos-elegir"), click("#archivos-enviar")) });
await f("100-archivos-quien-falta", "lab-archivos", { ruta: "archivos", pasos: seq(archivos, click("[data-vista-de=archivos] .envio__desplegar")) });
await f("101-archivos-historial", "lab-archivos", { h: 1100, ruta: "archivos", pasos: async (p) => {
  await p.evaluate(() => document.getElementById("h-historial-archivos").scrollIntoView());
} });
await f("102-archivos-error-nombre", "lab", { ruta: "archivos", pasos: seq(archivos, async (p) => p.evaluate(() =>
  window.__emit("aula:capture", { draft: null, error: "aula:pushPathInvalid: con:a.txt: nombre o carpeta no válidos" }))) });
await f("103-archivos-900x600", "lab-archivos", { w: 900, h: 600, ruta: "archivos", pasos: seq(archivos, plantilla("Ejercicio 4")) });
await f("104-archivos-oscuro", "lab-archivos", { oscuro: true, ruta: "archivos", pasos: seq(archivos, plantilla("Ejercicio 4")) });
await f("105-ficha-recibir-archivos", "lab", { pasos: click(`${fila("LAB3-PC07")} .ficha`) });
await f("106-plantillas-y-enviados", "lab-archivos", { h: 1250, ruta: "archivos" });

// Recolecciones (prompt 10.10): la pestaña de Archivos. Vacía con la ruta que verá el alumno,
// recién creada, recogida (cada estado de alumno), error de nombre, 900×600, oscuro y la ficha.
await f("110-recolecciones-vacia", "lab", { ruta: "archivos/recolecciones", pasos: async (p) => {
  await p.type("#reco-actividad", "Ejercicio 4"); await p.type("#reco-tipos", "*.py"); await sleep(150); } });
await f("111-recolecciones-creada", "lab-recolecciones-nueva", { ruta: "archivos/recolecciones" });
await f("112-recolecciones-recogida", "lab-recolecciones", { h: 1000, ruta: "archivos/recolecciones" });
await f("113-recolecciones-error-nombre", "lab", { ruta: "archivos/recolecciones", pasos: seq(async (p) => {
  await p.type("#reco-actividad", "Ejercicio: 4"); }, click("#reco-crear")) });
await f("114-recolecciones-900x600", "lab-recolecciones", { w: 900, h: 600, ruta: "archivos/recolecciones" });
await f("115-recolecciones-oscuro", "lab-recolecciones", { oscuro: true, h: 1000, ruta: "archivos/recolecciones" });
await f("116-ficha-recoger", "lab", { pasos: click(`${fila("LAB3-PC07")} .ficha`) });
await f("107-codigo-sin-archivos", "lab-archivos", { ruta: "codigo" });
await f("108-ficha-capacidades-y-acciones", "lab", { h: 860, pasos: click(`${fila("lab-ubuntu-02")} .ficha`) });
await f("109-pantalla-grande-acciones", "lab", { pasos: grande });

// Abrir página o app (prompt 10.11): el panel de la barra, con el aviso del filtro web y las últimas
// órdenes; la app permitida; la ficha y la Pantalla grande de un alumno; Ajustes › Apps permitidas
// (también con un error); oscuro y 900×600.
const panelAbrir = seq(click(fila("LAB3-PC07")), click(fila("LAB3-PC12")), click('[data-panel="abrir"]'));
const escribirUrl = (u) => async (p) => { await p.type("#abrir-url", u); await sleep(500); };
await f("117-panel-abrir-pagina", "lab-abrir", { pasos: seq(panelAbrir, escribirUrl("youtube.com/watch")) });
await f("118-panel-abrir-app", "lab-abrir", { pasos: seq(panelAbrir, click("#abrir-tipo-app")) });
await f("119-panel-abrir-enviado", "lab", { pasos: seq(panelAbrir, escribirUrl("docs.python.org/3/tutorial"), click("#abrir-enviar"),
  async () => { await sleep(400); }) });
await f("120-panel-abrir-url-invalida", "lab", { pasos: seq(panelAbrir, escribirUrl("file:///C:/Windows"), click("#abrir-enviar"),
  async () => { await sleep(300); }) });
await f("121-dialogo-abrir-app", "lab", { pasos: seq(grande, click("#pg-abrir1-abrir"), click("#pg-abrir1-tipo-app")) });
await f("122-pantalla-grande-abrir", "lab", { pasos: seq(grande, click("#pg-abrir1-abrir")) });
await f("123-ajustes-apps-permitidas", "datos", { h: 800, pasos: ajustes("apps") });
await f("124-ajustes-apps-error", "datos", { h: 800, pasos: seq(ajustes("apps"), click("#aj-apps-agregar"), async (p) => {
  await p.type("#aj-apps-lista li:last-child .fila-app__nombre", "Consola");
  await p.type("#aj-apps-lista li:last-child .fila-app__ruta", "C:\\Windows\\System32\\cmd.exe");
}, click("#aj-apps-guardar"), async () => { await sleep(300); }) });
await f("125-panel-abrir-oscuro-900x600", "lab-abrir", { oscuro: true, w: 900, h: 600, pasos: seq(panelAbrir, escribirUrl("youtube.com")) });
await f("126-ajustes-apps-oscuro-900x600", "datos", { oscuro: true, w: 900, h: 600, pasos: ajustes("apps") });

// Encuestas (prompt 10.12): el panel de la barra (opción múltiple, sí o no y su error), la franja
// al lanzar, la sección de Participación (anónima con simulados, nominal con quién falta), cerrar
// y compartir, el aviso de que reemplaza a la abierta, y oscuro a 900×600.
const panelEncuesta = seq(click(fila("LAB3-PC07")), click(fila("lab-ubuntu-02")), click('[data-panel="encuesta"]'), async (p) => {
  await p.type("#encuesta-pregunta", "¿Qué imprime print(3 // 2)?");
  const ops = await p.$$("#encuesta-opciones input");
  for (const [i, t] of ["1.5", "1", "2"].entries()) await ops[i].type(t);
});
const aEncuestas = async (p) => { await p.evaluate(() => document.getElementById("encuestas").scrollIntoView()); await sleep(200); };
await f("127-panel-encuesta", "lab", { pasos: panelEncuesta });
await f("128-panel-encuesta-sino-nominal", "lab", { pasos: seq(click('[data-panel="encuesta"]'), async (p) => {
  await p.type("#encuesta-pregunta", "¿Entendiste cómo funciona el ciclo for?");
}, click('input[name="encuesta-tipo"][value="yesno"]'), click("#encuesta-nominal")) });
await f("129-panel-encuesta-sin-opciones", "lab", { pasos: seq(click('[data-panel="encuesta"]'), async (p) => {
  await p.type("#encuesta-pregunta", "¿Cuál es correcta?");
}, click("#encuesta-lanzar")) });
await f("130-encuesta-franja", "lab", { pasos: seq(panelEncuesta, click("#encuesta-lanzar"), async () => { await sleep(400); }) });
await f("131-participacion-encuesta", "lab-encuesta", { ruta: "participacion", pasos: aEncuestas });
await f("132-participacion-encuesta-nominal", "lab-encuesta-nominal", { ruta: "participacion", pasos: aEncuestas });
await f("133-encuesta-cerrada-compartir", "lab-encuesta", { ruta: "participacion", pasos: seq(click("#f-encuesta-cerrar"),
  async () => { await sleep(300); }, aEncuestas) });
await f("134-panel-encuesta-reemplaza", "lab-encuesta", { pasos: click('[data-panel="encuesta"]') });
await f("135-participacion-encuesta-oscuro-900x600", "lab-encuesta", { oscuro: true, w: 900, h: 600, ruta: "participacion", pasos: aEncuestas });
await f("136-panel-encuesta-oscuro-900x600", "lab", { oscuro: true, w: 900, h: 600, pasos: panelEncuesta });

// Proyectar la pantalla de un alumno (docs/protocol.md §5.24, prompt 12.1): la acción en la ficha y en
// la Pantalla grande, la franja mientras conecta y mientras dura, la vista en grande, un fallo del
// equipo y la confirmación cuando ya transmites tu pantalla.
const fichaPc07 = click(`${fila("LAB3-PC07")} .ficha`);
await f("137-ficha-marcada", "lab", { pasos: fichaPc07 });
await f("138-proyectar-conectando", "lab-proyectar-conectando", { pasos: fichaPc07 });
await f("139-proyectando-franja", "lab-proyectando", { pasos: fichaPc07 });
await f("140-proyectando-ver", "lab-proyectando", { pasos: seq(click("#f-proyectando-ver"), async () => { await sleep(300); }) });
await f("141-proyectar-fallo", "lab-proyectando", { pasos: async (p) => {
  await p.evaluate(() => window.__emit("aula:present", { active: false, sessionId: "s-LAB3-PC07", hostname: "LAB3-PC07", phase: "",
    receivers: 0, eligible: 0, ended: "failed", reason: "capture_unavailable" }));
  await sleep(300);
} });
await f("142-proyectar-reemplaza-transmision", "lab-transmitiendo", { pasos: seq(grande, click("#pg-proyectar"), async () => { await sleep(200); }) });
await f("143-proyectando-oscuro-900x600", "lab-proyectando", { oscuro: true, w: 900, h: 600, pasos: seq(click("#f-proyectando-ver"), async () => { await sleep(300); }) });
await f("144-pantalla-grande-proyectar", "lab", { pasos: grande });

// Ventana angosta y tema oscuro
await f("37-alumnos-900x600", "lab", { w: 900, h: 600, pasos: click(fila("LAB3-PC07")) });
await f("38-codigo-900x600", "datos", { w: 900, h: 600, ruta: "codigo" });
await f("39-alumnos-oscuro", "lab", { oscuro: true, pasos: click(fila("LAB3-PC07")) });
await f("40-codigo-oscuro", "datos", { oscuro: true, ruta: "codigo" });
await f("41-ficha-alumno-oscuro", "lab-errores", { oscuro: true, pasos: click(`${fila("lab-ubuntu-02")} .ficha`) });

// Notas de interfaz 001–011 y los iconos en botones y campos (2026-11-11).
await f("145-alumnos-cabecera-y-tabla", "lab");
await f("146-barra-tooltip", "lab", { w: 900, h: 600, pasos: async (p) => { await p.hover('[data-panel="temporizador"]'); await sleep(200); } });
await f("147-dialogo-privado", "lab", { pasos: seq(grande, click("#pg-privado-abrir")) });
await f("148-dialogo-abrir-pagina", "lab", { pasos: seq(grande, click("#pg-abrir1-abrir")) });
await f("149-codigo-pie-envio", "lab", { ruta: "codigo" });
await f("150-historial-titulos", "lab", { ruta: "codigo", pasos: async (p) => { await p.evaluate(() => document.getElementById("h-historial").scrollIntoView()); } });
await f("151-archivos-enviar", "lab", { ruta: "archivos" });
await f("152-archivos-enviar-oscuro-900x600", "lab", { ruta: "archivos", oscuro: true, w: 900, h: 600 });
await f("153-panel-aviso-iconos", "lab", { pasos: click('[data-panel="aviso"]') });
await f("154-panel-energia-iconos", "lab", { pasos: click('[data-panel="energia"]') });
await f("155-dialogo-privado-oscuro", "lab", { oscuro: true, pasos: seq(grande, click("#pg-privado-abrir")) });
// Mostrar mi pantalla (privacidad, regla 12): «Para» la selección y «Modo» ventana o pantalla completa;
// la franja dice el modo y a cuántos.
await f("156-panel-pantalla-seleccion-completa", "lab", { pasos: seq(click(fila("LAB3-PC07")), click(fila("LAB3-PC12")),
  click('[data-panel="pantalla"]'), click('input[name="pantalla-modo"][value="fullscreen"]')) });
await f("157-panel-pantalla-oscuro-900x600", "lab", { oscuro: true, w: 900, h: 600, pasos: click('[data-panel="pantalla"]') });
// Notas de interfaz 14-16: el pie de envío en una fila con su tooltip; la «i» abre un globo flotante
// encima; eliminar una plantilla pide confirmación.
const pieEnvio = async (p) => { await p.evaluate(() => document.querySelector(".pie-envio").scrollIntoView({ block: "center" })); };
await f("158-codigo-pie-envio", "datos", { ruta: "codigo", pasos: seq(pieEnvio, async (p) => { await p.focus("#codigo-limpiar"); await p.keyboard.press("Tab"); await sleep(150); }) });
await f("159-codigo-pie-envio-oscuro-900x600", "datos", { ruta: "codigo", oscuro: true, w: 900, h: 600, pasos: pieEnvio });
await f("160-ficha-info-globo", "lab", { pasos: seq(click(`${fila("LAB3-PC07")} .ficha`), click('#panel-alumno [data-info="capacidades"]')) });
await f("161-ficha-info-globo-oscuro-900x600", "lab", { oscuro: true, w: 900, h: 600,
  pasos: seq(click(`${fila("LAB3-PC07")} .ficha`), click('#panel-alumno [data-info="capacidades"]')) });
// Nota 19: la rejilla de duraciones y «Otro…» con los minutos.
await f("163-temporizador-duraciones", "lab", { pasos: click('[data-panel="temporizador"]') });
await f("165-temporizador-descripcion", "lab", { pasos: seq(click('[data-panel="temporizador"]'), async (p) => {
  await p.type("#temporizador-etiqueta", "Ejercicio 1");
  await p.type("#temporizador-descripcion", "Resuelve los ejercicios 3 a 5 de la hoja.");
}) });
await f("164-temporizador-otro-oscuro-900x600", "lab", { oscuro: true, w: 900, h: 600,
  pasos: seq(click('[data-panel="temporizador"]'), click("#temporizador-otro")) });
await f("162-plantilla-eliminar", "lab-archivos", { ruta: "archivos", pasos: click("#plantillas-filas tr:first-child .eliminar") });

// Pausar pantallas (docs/protocol.md §5.9, prompt 10.21): el panel de la barra, la franja con quién no
// acusó y el mensaje, la insignia «En pausa» en la fila, la acción en la Pantalla grande, el diálogo
// para cambiar el mensaje, Ctrl+Mayús+P y su versión oscura a 900×600.
const panelPausa = seq(click(fila("LAB3-PC07")), click(fila("LAB3-PC12")), click('[data-panel="pausa"]'),
  async (p) => { await p.type("#pausa-texto", "Miren al pizarrón un momento."); });
await f("166-panel-pausa", "lab", { pasos: panelPausa });
await f("167-pausa-franja", "lab", { pasos: seq(panelPausa, click("#pausa-aplicar"), async () => { await sleep(400); }) });
await f("168-pausa-sin-acusar", "lab-pausa");
await f("169-pausa-cambiar-mensaje", "lab-pausa", { pasos: click("#f-pausa-mensaje") });
await f("170-pantalla-grande-pausa", "lab-pausa", { pasos: grande });
await f("171-pausa-atajo", "lab", { pasos: async (p) => {
  await p.keyboard.down("Control"); await p.keyboard.down("Shift"); await p.keyboard.press("KeyP");
  await p.keyboard.up("Shift"); await p.keyboard.up("Control");
  await sleep(400);
} });
await f("172-pausa-oscuro-900x600", "lab-pausa", { oscuro: true, w: 900, h: 600, pasos: click('[data-panel="pausa"]') });

// Notas de interfaz v0.1.0 001–006 (2026-11-12): «Enviar a» y los botones en una fila, el canal solo
// con dígitos, las miniaturas como monitores con su tamaño, y la selección en la cabecera.
const alPie = async (p) => { await p.evaluate(() => document.querySelector("[data-vista-de] .pie-envio:not([hidden])")?.scrollIntoView({ block: "center" })); };
await f("173-pie-envio-codigo", "lab", { ruta: "codigo", pasos: alPie });
await f("174-pie-envio-archivos", "lab", { ruta: "archivos", pasos: async (p) => {
  await p.evaluate(() => document.querySelector("#archivos-panel-enviar .pie-envio").scrollIntoView({ block: "center" }));
} });
await f("175-pie-envio-oscuro-900x600", "lab", { ruta: "codigo", oscuro: true, w: 900, h: 600, pasos: alPie });
await f("176-canal-solo-digitos", "lab", { pasos: seq(click("#nav-clase"), async (p) => { await p.type("#cc-canal", "1a2b-3"); await sleep(150); }) });
const miniaturasDe = (tamano) => seq(click('[data-modo="miniaturas"]'), cuadros(["LAB3-PC07", "LAB3-PC12"]), async (p) => {
  await p.select("#tamano-miniaturas-select", tamano); await sleep(200);
});
await f("177-miniaturas-monitor", "lab", { pasos: miniaturasDe("mediano") });
await f("178-miniaturas-pequeno", "lab", { pasos: miniaturasDe("pequeno") });
await f("179-miniaturas-extra", "lab", { pasos: miniaturasDe("extra") });
await f("180-miniaturas-ajustar", "lab", { pasos: miniaturasDe("ajustar") });
await f("181-miniaturas-oscuro-900x600", "lab", { oscuro: true, w: 900, h: 600, pasos: miniaturasDe("grande") });
await f("182-seleccion-cabecera", "lab", { pasos: seq(click(fila("LAB3-PC07")), click(fila("LAB3-PC12"))) });
await f("183-seleccion-cabecera-oscuro-900x600", "lab", { oscuro: true, w: 900, h: 600, pasos: async (p) => { await p.hover("#seleccion-texto"); await sleep(200); } });

// Terminar la clase (prompt 10.14): el botón bajo el Selector de clase, su diálogo con lo activo y lo
// opcional, el resultado por alumno (limpio, en parte, sin acuse, con error, desconectado) y la
// clase sin nada activo.
const terminar = seq(click("#nav-terminar"), async () => { await sleep(200); });
const terminarYa = seq(terminar, click("#te-recoger-opcion"), click("#te-terminar"), async () => { await sleep(400); });
await f("184-terminar-dialogo", "lab-terminar", { pasos: terminar });
await f("185-terminar-resultado", "lab-terminar", { pasos: terminarYa });
await f("186-terminar-oscuro-900x600", "lab-terminar", { oscuro: true, w: 900, h: 600, pasos: terminar });
await f("187-terminar-resultado-oscuro-900x600", "lab-terminar", { oscuro: true, w: 900, h: 600, pasos: terminarYa });
await f("188-terminar-nada-activo", "datos", { pasos: terminar });

// Encender (prompt 10.19): «Mostrar apagados» con «Encendiendo…», «No respondió» y Encender en
// la fila; la opción Encender del Panel Energía, para todos o para la selección; el aviso tras
// encender uno. El interruptor se enciende por código: se recuerda entre escenas.
const verApagados = async (p) => {
  await p.evaluate(() => { const i = document.getElementById("alumnos-apagados"); if (!i.checked) i.click(); });
  await sleep(150);
};
const panelEncender = seq(click('[data-panel="energia"]'), click('#panel-energia label:has(input[value="wake"])'), async () => { await sleep(150); });
await f("189-encender-apagados", "lab-encender", { pasos: verApagados });
await f("190-encender-panel-todos", "lab", { pasos: seq(verApagados, panelEncender) });
await f("191-encender-panel-seleccion", "lab", { pasos: seq(verApagados, click(fila("LAB3-PC01")), click(fila("LAB3-PC03")), panelEncender) });
await f("192-encender-uno", "lab", { pasos: seq(verApagados, click(`${fila("LAB3-PC03")} .encender`), async () => { await sleep(300); }) });
await f("193-encender-apagados-oscuro-900x600", "lab-encender", { oscuro: true, w: 900, h: 600, pasos: verApagados });
await f("194-encender-panel-oscuro-900x600", "lab", { oscuro: true, w: 900, h: 600, pasos: seq(verApagados, panelEncender) });

// Cuestionarios (prompt 10.20; privacidad, reglas 87-91): la vista con los guardados (uno ilegible y
// un equipo que no puede), el editor con una pregunta de varias correctas y otra de sí o no, un error
// del editor, lanzar con la franja, los resultados en Participación (aciertos por pregunta y puntaje
// por alumno), cerrado con Compartir y Exportar, y oscuro a 900×600.
const editarCuest = seq(click("#cuest-filas .editar"), async () => { await sleep(300); });
await f("195-cuestionarios", "lab", { ruta: "cuestionarios" });
await f("196-cuestionario-editor", "lab", { ruta: "cuestionarios", h: 1400, pasos: editarCuest });
await f("197-cuestionario-nuevo-error", "lab", { ruta: "cuestionarios", pasos: seq(click("#cuest-nuevo"), async (p) => {
  await p.type("#cuest-titulo", "Repaso rápido");
  await p.type("#cuest-preguntas .cuest-texto", "¿Qué imprime print(2 ** 3)?");
  const ops = await p.$$("#cuest-preguntas .cuest-opcion input[type=text]");
  await ops[0].type("6");
  await ops[1].type("8");
}, click("#cuest-guardar")) });
await f("198-cuestionario-lanzado-franja", "lab", { ruta: "cuestionarios", pasos: seq(click("#cuest-filas .lanzar"), async () => { await sleep(500); }) });
await f("199-participacion-cuestionario", "lab-cuestionario", { ruta: "participacion", pasos: aEncuestas });
await f("200-cuestionario-cerrado-exportar", "lab-cuestionario-cerrado", { ruta: "participacion", pasos: seq(aEncuestas,
  click("#encuesta-actual button, #encuestas-lista details[open] .grupo-botones .btn:last-child"), async () => { await sleep(300); }) });
await f("201-cuestionarios-oscuro-900x600", "lab", { oscuro: true, w: 900, h: 600, ruta: "cuestionarios", pasos: editarCuest });
await f("202-participacion-cuestionario-oscuro-900x600", "lab-cuestionario", { oscuro: true, w: 900, h: 600, ruta: "participacion", pasos: aEncuestas });

// Salón (prompt 10.15): la vista «Salón» de Alumnos con 30 simulados acomodados, en claro, oscuro
// y a 900×600; el editor del módulo con un equipo tomado; la etiqueta desde la Pantalla grande,
// la ficha con su lugar, Ajustes › Salón, importar de otro canal y el aviso de un equipo renombrado.
const treinta = Array.from({ length: 30 }, (_, i) => "Simulado " + (i + 1));
const salonVista = (tamano) => seq(click('[data-modo="salon"]'), cuadros(["LAB3-PC07", "LAB3-PC12", ...treinta]), async (p) => {
  await p.select("#tamano-miniaturas-select", tamano); await sleep(200);
});
await f("203-salon-vista", "lab-salon", { h: 1000, pasos: salonVista("pequeno") });
await f("204-salon-vista-fila", "lab-salon", { h: 1000, pasos: seq(salonVista("pequeno"),
  click('#alumnos-salon-rejilla .salon__cabecera--fila:nth-of-type(n) button[aria-label$="fila 2"]')) });
await f("205-salon-vista-oscuro-900x600", "lab-salon", { oscuro: true, w: 900, h: 600, pasos: salonVista("pequeno") });
await f("206-salon-vista-ajustar-900x600", "lab-salon", { w: 900, h: 600, pasos: salonVista("ajustar") });
const tomarPC07 = click('#salon-plano [data-equipo="LAB3-PC07"]');
await f("207-salon-editor-llevando", "lab", { ruta: "salon", h: 900, pasos: tomarPC07 });
await f("208-salon-editor-oscuro-900x600", "lab-salon", { oscuro: true, w: 900, h: 600, ruta: "salon", pasos: tomarPC07 });
await f("209-salon-etiqueta-pantalla-grande", "lab", { pasos: seq(click('[data-modo="miniaturas"]'), cuadros(["LAB3-PC07", "LAB3-PC12"]),
  click(".alumno__pantalla:not([disabled])"), click("#pg-etiqueta")) });
await f("210-salon-ficha", "lab", { pasos: click(`${fila("LAB3-PC07")} .ficha`) });
await f("211-salon-ajustes", "lab", { pasos: ajustes("salon") });
await f("212-salon-importar-otro-canal", "lab", { ruta: "salon", pasos: click("#salon-importar") });
await f("213-salon-renombrado", "lab", { pasos: async (p) => { await p.evaluate(() => window.__renombrar("LAB3-PC03", "LAB3-03")); await sleep(200); } });

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
