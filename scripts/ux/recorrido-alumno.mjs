// Recorre la app del alumno en el arnés web (prompt 19): captura cada estado,
// junta los errores de JS y pasa axe-core (WCAG 2.1 AA). Escribe además
// axe.json con el detalle de cada violación. Sale con error si hay errores de
// JS o violaciones serias o críticas.
//   node recorrido-alumno.mjs [carpeta de salida] [--solo=archivos,94-105]   (por omisión, ./salida-alumno)
import fs from "node:fs";
import path from "node:path";
import { abrir, carpetaSalida, soloCapturas, click, seq, sleep } from "./arnes.mjs";

const salida = path.resolve(carpetaSalida("salida-alumno"));
const incluir = soloCapturas();
const { foto, cerrar } = await abrir({ app: "alumno" });
const informe = {};
// La ventana del alumno abre a 900×700 (apps/student/main.go); su mínimo es 480×360.
const f = async (nombre, escena, o = {}) => {
  if (!incluir(nombre)) return;
  informe[nombre] = await foto(path.join(salida, nombre + ".webp"), escena, Object.assign({ w: 900, h: 700 }, o));
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
await f("26-codigo-nuevo-arriba", "varios-codigos", { pasos: async (p) => {
  const prev = await p.evaluate(() => window.go.main.App.View().then((v) => v.snippets));
  await emitir("aula:board", { snippets: [...prev, { id: "sn-4", exercise: "4", title: "Tabla de multiplicar", lang: "python",
    content: "for i in range(1, 11):\n    print(f\"5 x {i} = {5 * i}\")\n", receivedAt: new Date().toISOString() }] })(p);
} });
await f("27-seleccion-con-teclado", "varios-codigos", { pasos: async (p) => { await p.keyboard.press("ArrowDown"); await sleep(200); } });

// Avisos
await f("11-aviso-info", "aviso-info");
await f("12-aviso-warning", "aviso-warning");
await f("13-varios-avisos", "varios-avisos");
await f("30-muchos-avisos", "muchos-avisos");
await f("31-muchos-avisos-todos", "muchos-avisos", { pasos: click("#avisos-mas") });
await f("32-otra-clave", "otra-clave");
await f("33-otra-clave-detalles", "otra-clave", { pasos: click("#estado-detalles summary") });

// Transparencia y control del profesor
await f("14-pantalla-vista", "pantalla-vista");
await f("15-transmision", "transmision");
await f("15b-transmision-completa", "transmision-completa");
await f("16-restricciones", "restricciones");
await f("17-restricciones-detalles", "restricciones", { pasos: click("#restr-toggle") });
await f("18-app-cerrada", "restricciones", { pasos: emitir("aula:app-closed", { app: "steam" }) });
await f("34-todas-las-bandas", "todo", { pasos: emitir("aula:app-closed", { app: "steam" }) });
await f("35-todas-las-bandas-oscuro", "todo", { oscuro: true });
await f("19-energia-cuenta", "energia-cuenta");
await f("36-control", "control");
await f("37-control-pausa", "control-pausa");
await f("38-control-terminado", "control", { pasos: emitir("aula:status", { state: "connected", teacher: "Prof. Ayala", channel: "1",
  detail: "", cause: "", viewed: false, watching: false, restricted: false, control: "" }) });
await f("39-control-oscuro", "control", { oscuro: true });
await f("41-captura-tomada", "captura");
await f("42-captura-con-pantalla-vista", "captura-vista");
await f("43-captura-oscuro", "captura", { oscuro: true });
await f("44-captura-angosta", "captura", { w: 480, h: 360 });
await f("20-energia-forzada", "energia-forzada");
await f("21-energia-fallida", "energia-fallida");

// Levantar la mano y escribir al profesor (prompt 10.6)
const escribe = (texto) => seq(click("#escribir"), async (p) => { await p.type("#redactar-texto", texto); });
await f("45-mano-levantada", "mano");
await f("46-mano-motivo", "mano", { pasos: click("#motivo-abrir") });
await f("47-mano-bajada-por-profesor", "mano-bajada");
await f("48-mensaje-escribiendo", "conectado", { pasos: escribe("¿Puede repetir el último paso?") });
await f("49-mensaje-enviado", "conectado", { pasos: seq(escribe("¿Puede repetir el último paso?"), async (p) => {
  await p.keyboard.press("Enter");
  await sleep(700);
}) });
await f("50-mensaje-vacio", "conectado", { pasos: seq(click("#escribir"), async (p) => { await p.keyboard.press("Enter"); await sleep(200); }) });
await f("51-respuesta-del-profesor", "mensajes");
await f("52-sin-conexion-botones", "buscando");
await f("53-consola-sin-soporte", "sin-soporte");
await f("54-mano-oscuro", "mano", { oscuro: true });
await f("55-mano-angosta", "mano", { w: 480, h: 360 });
await f("56-compacto-mano", "mano", { w: 380, h: 80, pasos: click("#compactar") });

// Mensaje privado del profesor (prompt 10.7)
await f("57-aviso-privado", "aviso-privado");
await f("58-aviso-privado-oscuro", "aviso-privado", { oscuro: true });

// Temporizador del profesor (prompt 10.8): insignia en la tira, último minuto, fin y compacto.
const finTemporizador = emitir("aula:timer", { state: "ended", timerId: "t-1", label: "Ejercicio 1", remainingMs: 0, justEnded: true });
await f("59-temporizador", "temporizador");
await f("60-temporizador-ultimo-minuto", "temporizador-final");
await f("61-temporizador-termino", "temporizador-final", { pasos: finTemporizador });
await f("62-temporizador-oscuro", "temporizador", { oscuro: true });
await f("63-temporizador-angosta", "temporizador", { w: 480, h: 360 });
await f("64-compacto-temporizador", "temporizador", { w: 380, h: 80, pasos: click("#compactar") });
// Nota de interfaz 18: la tarjeta con lo que hay que hacer.
await f("64b-temporizador-tarea", "temporizador-tarea");
await f("64c-temporizador-tarea-oscuro-angosta", "temporizador-tarea", { oscuro: true, w: 480, h: 360 });

// Archivos del profesor (prompt 10.9): tarjetas en la lista, renombrado, fallo, oscuro, angosta y compacto.
await f("65-archivos-recibidos", "archivos");
await f("66-archivo-renombrado", "archivo-renombrado");
await f("67-archivo-fallo", "archivo-fallo");
await f("68-archivos-oscuro", "archivos", { oscuro: true });
await f("69-archivos-angosta", "archivos", { w: 480, h: 360 });
await f("70-compacto-archivo-nuevo", "conectado", { w: 380, h: 80, pasos: seq(click("#compactar"), emitir("aula:files", { folder: "Aula",
  received: [{ pushId: "fp-9", label: "", files: 1, folder: "", at: new Date().toISOString(), items: [{ pushId: "fp-9", fileId: "f", label: "",
    index: 1, files: 1, name: "datos.csv", dest: "", path: "datos.csv", size: 10, renamed: false, replaced: false, reason: "",
    at: new Date().toISOString() }] }] })) });

// Recolecciones (prompt 10.10): la tarjeta con la carpeta y lo recogido, los avisos de antes y
// después, recogiendo, oscuro y angosta.
await f("71-recoleccion", "recoleccion");
await f("72-recoleccion-avisos", "recoleccion-aviso");
await f("73-recoleccion-recogiendo", "recoleccion-recogiendo");
await f("74-recoleccion-oscuro", "recoleccion", { oscuro: true });
await f("75-recoleccion-angosta", "recoleccion-aviso", { w: 480, h: 360 });

// Abrir página o app (prompt 10.11; regla 66): el aviso en la esquina dice qué se abrió y quién.
await f("76-abierto", "abierto");
await f("77-abierto-oscuro", "abierto", { oscuro: true });

// Encuestas (prompt 10.12): la tarjeta arriba con «Nuevo» y si es anónima, la respuesta enviada
// (con teclado), una cerrada con los resultados de la clase, la abierta corta nominal, oscuro,
// ventana mínima y el modo compacto cuando llega una.
await f("78-encuesta", "encuesta");
await f("79-encuesta-respondida", "encuesta", { pasos: seq(click('#encuestas > li:first-child input[value="1"]'),
  click("#encuestas > li:first-child .encuesta-enviar"), async () => { await sleep(500); }) });
await f("80-encuesta-texto", "encuesta-texto", { pasos: async (p) => {
  await p.type("#encuestas > li:first-child .encuesta-texto input", "Los índices de la lista");
} });
await f("81-encuesta-oscuro", "encuesta", { oscuro: true });
await f("82-encuesta-angosta", "encuesta", { w: 480, h: 360 });
await f("83-compacto-encuesta-nueva", "conectado", { w: 380, h: 80, pasos: seq(click("#compactar"), emitir("aula:polls", { arrived: "pl-9", cards: [{ pollId: "pl-9", kind: "choice", question: "¿Qué imprime print(3 // 2)?",
  options: ["1.5", "1", "2", "Error"], named: false, open: true, choice: -1, text: "", vote: "none", results: null, at: new Date().toISOString() }] })) });

// Tu pantalla mostrada a la clase (prompt 12.1; privacidad, reglas 75-82): la banda sin botones (la
// decide el profesor), el oscuro, la ventana mínima, el compacto y la pantalla de un compañero.
await f("84-mostrando", "mostrando");
await f("85-mostrando-oscuro", "mostrando", { oscuro: true });
await f("86-mostrando-angosta", "mostrando", { w: 480, h: 360 });
await f("87-mostrando-compacto", "mostrando", { w: 380, h: 80, pasos: click("#compactar") });
await f("88-viendo-companero", "transmision-alumno");

// Notas de interfaz 12 y 13: el mensaje al profesor en su diálogo y «Qué puede hacer Aula».
await f("89-mensaje-dialogo", "conectado", { pasos: click("#escribir") });
await f("90-capacidades", "conectado", { pasos: seq(click("#capacidades"), async () => { await sleep(200); }) });
await f("91-capacidades-oscuro-angosta", "conectado", { oscuro: true, w: 480, h: 560, pasos: seq(click("#capacidades"), async () => { await sleep(200); }) });

// Modo compacto (la ventana real se encoge a 380×112: aquí, la tira en ese tamaño)
await f("28-compacto", "conectado", { w: 380, h: 80, pasos: click("#compactar") });
await f("29-compacto-nuevos-y-vista", "pantalla-vista", { w: 380, h: 80, pasos: seq(click("#compactar"), async (p) => {
  const prev = await p.evaluate(() => window.go.main.App.View().then((v) => v.snippets));
  await emitir("aula:board", { snippets: [...prev, { id: "sn-4", exercise: "4", title: "Tabla", lang: "python", content: "print(5)\n",
    receivedAt: new Date().toISOString() }] })(p);
}) });

await f("40-compacto-controlado", "control", { w: 380, h: 80, pasos: click("#compactar") });
// Pantalla en pausa (prompt 10.21): la banda al volver a la ventana y la insignia del modo compacto.
// La ventana de pausa es nativa: el arnés no la ve (caso manual A-xx).
await f("92-banda-pausa", "pausa");
await f("93-compacto-en-pausa", "pausa", { w: 380, h: 80, pasos: click("#compactar") });
await f("94-banda-pausa-oscuro", "todo", { oscuro: true });

// Terminar la clase (prompt 10.14; regla 83): la ventana como recién abierta con la tarjeta «La clase
// terminó», la hora y lo que se quitó; en oscuro y en el modo compacto (cuenta como «Nuevo»).
await f("95-fin-de-clase", "fin-clase");
await f("96-fin-de-clase-oscuro", "fin-clase", { oscuro: true });
await f("97-compacto-fin-de-clase", "fin-clase", { w: 380, h: 80, pasos: click("#compactar") });

// Cuestionarios (prompt 10.20; privacidad, reglas 87-91): la tarjeta con una pregunta a la vez y
// cuántas lleva, una de varias correctas, «Entregar así» con preguntas sin responder, el resultado
// personal, oscuro y ventana mínima.
await f("98-cuestionario", "cuestionario");
await f("99-cuestionario-varias", "cuestionario", { pasos: seq(click("#encuestas > li:first-child .cuestionario-siguiente"),
  click('#encuestas > li:first-child .cuestionario-opciones input[value="0"]'), async () => { await sleep(500); }) });
await f("100-cuestionario-entregar-asi", "cuestionario", { pasos: click("#encuestas > li:first-child .cuestionario-entregar") });
await f("101-cuestionario-resultado", "cuestionario-resultado");
await f("102-cuestionario-oscuro", "cuestionario", { oscuro: true });
await f("103-cuestionario-angosta", "cuestionario-resultado", { w: 480, h: 360 });

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
