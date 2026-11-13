// Doble de los bindings de Wails de la app del alumno (window.go.main.App y
// window.runtime), para recorrer apps/student/frontend/dist sin backend. Los
// datos tienen la forma exacta de apps/student/events.go: si cambia un binding
// o un evento, cambia aquí también (una prueba de Go lo exige). La escena se
// elige con window.__SCENE. Ver scripts/ux/README.md.
(function () {
  const scene = window.__SCENE || "conectado";
  const handlers = {};
  window.runtime = {
    EventsOn(name, cb) { (handlers[name] = handlers[name] || []).push(cb); return () => {}; },
    EventsOff() {},
    WindowSetTitle(t) { window.__title = t; },
  };
  window.__emit = (name, data) => (handlers[name] || []).forEach((cb) => cb(data));
  window.__calls = [];

  const teacher = "Prof. Ayala";
  const channel = "1";
  // El estado: Go manda estados, no frases (apps/student/events.go, Status).
  const st = (state, extra) => Object.assign({ state, teacher: state === "searching" || state === "unconfigured" ? "" : teacher,
    channel: state === "unconfigured" ? "" : channel, detail: "", cause: "", viewed: false, watching: false, restricted: false, control: "", presenting: false, paused: false }, extra);
  const conectado = st("connected");
  const status = {
    "sin-configurar": st("unconfigured", { detail: "no hay configuración: falta config.json en C:\ProgramData\Aula\alumno (canal y clave pública del centro); pide al administrador que instale Aula en este equipo" }),
    buscando: st("searching"),
    "sin-conexion": st("searching", { cause: "noConsole", detail: `No se encontró ninguna consola del canal ${channel}. Revisa que la consola del profesor esté abierta en ese canal, que la red de este equipo sea Privada o de Dominio (no Pública) y que el firewall permita Aula.` }),
    reconectando: st("disconnected"),
    "otra-clave": st("searching", { cause: "wrongKey", detail: `Se encontró la consola «Sala 2» del canal ${channel}, pero su clave no es la de este centro: no se le obedece. Si es la consola de tu profesor, este equipo tiene otra clave del centro; avisa al administrador.` }),
    "pantalla-vista": st("connected", { viewed: true }),
    transmision: st("connected", { watching: true }),
    // La pantalla de un compañero (prompt 12.1): la misma vista, con su nombre.
    "transmision-alumno": st("connected", { watching: true }),
    // Tu pantalla mostrada a la clase (docs/privacidad.md, reglas 75-82).
    mostrando: st("connected", { presenting: true }),
    restricciones: st("connected", { restricted: true }),
    // El control remoto implica la pantalla vista (docs/privacidad.md, regla 37).
    control: st("connected", { viewed: true, control: "active" }),
    // Una captura durante las miniaturas: la banda de vista sigue, el aviso es aparte.
    "captura-vista": st("connected", { viewed: true }),
    "control-pausa": st("connected", { viewed: true, control: "paused" }),
    // Pantalla en pausa (docs/privacidad.md, reglas 9-10): la ventana nativa cubre los monitores.
    pausa: st("connected", { paused: true }),
    todo: st("connected", { restricted: true, viewed: true, control: "active", presenting: true, paused: true }),
  }[scene] || conectado;

  const ahora = new Date().toISOString();
  const py = { id: "sn-1", exercise: "1", title: "Factorial recursivo", lang: "python", receivedAt: ahora,
    content: "def factorial(n):\n    if n <= 1:\n        return 1\n    return n * factorial(n - 1)\n\n\nprint(factorial(5))\n" };
  const js = { id: "sn-2", exercise: "2", title: "Suma de una lista", lang: "javascript", receivedAt: ahora,
    content: "function suma(lista) {\n  let total = 0;\n  for (const n of lista) {\n    total += n;\n  }\n  return total;\n}\n\nconsole.log(suma([1, 2, 3]));\n" };
  const java = { id: "sn-3", exercise: "3", title: "", lang: "java", receivedAt: ahora,
    content: "public class Hola {\n    public static void main(String[] args) {\n        for (int i = 0; i < 3; i++) {\n            System.out.println(\"Hola \" + i);\n        }\n    }\n}\n" };
  const snippets = { "un-codigo": [py], "varios-codigos": [py, js, java], "pantalla-vista": [py], transmision: [],
    control: [py], "control-pausa": [py], pausa: [py], captura: [py], "captura-vista": [py],
    restricciones: [py], "aviso-info": [py], "aviso-privado": [py], "aviso-warning": [py], "varios-avisos": [py, js, java], "muchos-avisos": [py], abierto: [py] }[scene] || [];

  const aviso = (id, level, text) => ({ id, level, text, at: ahora });
  const captura = (id) => ({ id, level: "warning", text: "", kind: "snapshot", at: "2026-11-05T10:42:06-06:00" });
  const notices = {
    "aviso-info": [aviso("m-1", "info", "En 10 minutos revisamos el ejercicio 1 todos juntos.")],
    "aviso-warning": [aviso("m-2", "warning", "Guarden su trabajo: en 5 minutos cerramos la sesión.")],
    "varios-avisos": [aviso("m-1", "info", "En 10 minutos revisamos el ejercicio 1 todos juntos."),
      aviso("m-2", "warning", "Guarden su trabajo: en 5 minutos cerramos la sesión."),
      aviso("m-3", "info", "Si terminaste, intenta la versión iterativa del factorial.")],
    "muchos-avisos": [1, 2, 3, 4, 5].map((i) => aviso("m-" + i, i === 5 ? "warning" : "info", `Aviso número ${i} de la clase.`)),
    // El profesor tomó una captura (docs/privacidad.md regla 44): Go manda el tipo, no la frase.
    captura: [captura("c-1")],
    "captura-vista": [captura("c-1")],
    // Un mensaje privado del profesor (prompt 10.7), junto a un aviso a todos para distinguirlos.
    "aviso-privado": [aviso("m-1", "info", "En 10 minutos revisamos el ejercicio 1 todos juntos."),
      Object.assign(aviso("m-p", "info", "Revisa la línea 3: el caso base debe devolver 1."), { personal: true })],
  }[scene] || [];

  const restrictions = scene === "restricciones" || scene === "todo" ? {
    apps: { policyId: "p-apps", mode: "deny", items: ["steam", "discord", "spotify", "whatsapp", "telegram"], total: 9 },
    web: { policyId: "p-web", mode: "deny", items: ["youtube.com", "tiktok.com", "instagram.com", "facebook.com", "x.com"], total: 11 },
  } : { apps: null, web: null };

  // «transmision»: en una ventana; «transmision-completa»: en pantalla completa (regla 12).
  const broadcast = scene === "transmision" || scene === "transmision-completa" || scene === "transmision-alumno"
    ? { active: true, broadcastId: "b-1", width: 1920, height: 1080, fps: 15, teacher, channel,
      source: scene === "transmision-alumno" ? "LAB3-PC07" : "",
      mode: scene === "transmision-completa" ? "fullscreen" : scene === "transmision" ? "window" : "" }
    : { active: false, broadcastId: "", width: 0, height: 0, fps: 0, teacher: "", channel: "", source: "" };

  // Tu pantalla mostrada a la clase (broadcast.PresentView, prompt 12.1): la decide el
  // profesor; el alumno no responde nada. Escenas: mostrando y todo (la banda).
  const present = { mostrando: { phase: "live", presentId: "pr-1" }, todo: { phase: "live", presentId: "pr-1" } }[scene] || { phase: "" };

  const potencia = (state, action, extra) => ({ notice: Object.assign({ action_id: "pw-1", action, state,
    remaining_s: 87, postpones_left: 2, force: false }, extra), teacher, channel });
  const power = {
    "energia-cuenta": potencia("countdown", "reboot"),
    todo: potencia("countdown", "reboot"),
    "energia-forzada": potencia("executing", "shutdown", { force: true, remaining_s: 0 }),
    "energia-fallida": potencia("failed", "logoff", { remaining_s: 0 }),
  }[scene] || { notice: null, teacher: "", channel: "" };

  // La mano y los mensajes al profesor (participation.DeskView). Escenas: mano (levantada
  // con motivo), mano-bajada (la bajó el profesor), mensajes (uno recibido y una respuesta),
  // sin-soporte (consola anterior). En «buscando» los botones están desactivados.
  const enviado = (id, text, status, min) => ({ messageId: id, text, status, at: new Date(Date.now() - min * 60000).toISOString() });
  let part = { hand: "down", note: "", messages: [], unsupported: false };
  if (scene === "mano") part = Object.assign(part, { hand: "up", note: "No me compila el for de la línea 12" });
  if (scene === "mano-bajada") part.hand = "lowered_by_teacher";
  if (scene === "mensajes") part.messages = [enviado("pm-1", "Ya terminé el ejercicio 1", "received", 6),
    enviado("pm-2", "¿El ejercicio 2 es para hoy?", "received", 2)];
  if (scene === "sin-soporte") part.unsupported = true;
  if (scene === "mensajes") notices.push({ id: "m-r", level: "info", text: "Sí, antes de las 12. Si te atoras, levanta la mano.",
    at: ahora, replyTo: "pm-2" });
  const copia = () => Object.assign({}, part, { messages: part.messages.slice() });
  // El temporizador del profesor (classtimer.View, prompt 10.8). Escenas: temporizador
  // (4:32), temporizador-final (0:42, último minuto) y temporizador-fin (terminó).
  const timer = { temporizador: { state: "running", timerId: "t-1", label: "Ejercicio 1", remainingMs: 272000, justEnded: false },
    // Nota de interfaz 18: con lo que hay que hacer, en su tarjeta.
    "temporizador-tarea": { state: "running", timerId: "t-1", label: "Ejercicio 1", remainingMs: 272000, justEnded: false,
      description: "Resuelve los ejercicios 3 a 5 de la hoja.\nGuarda tu archivo como ejercicio1.py en Documentos\\Aula." },
    "temporizador-final": { state: "running", timerId: "t-1", label: "Ejercicio 1", remainingMs: 42000, justEnded: false },
    "temporizador-fin": { state: "ended", timerId: "t-1", label: "Ejercicio 1", remainingMs: 0, justEnded: false },
  }[scene] || { state: "none", timerId: "", label: "", remainingMs: 0, justEnded: false };
  // Los archivos que envió el profesor (Files de apps/student/files.go, prompt 10.9).
  // Escenas: archivos (una plantilla de 3 y un archivo suelto), archivo-renombrado (ya
  // tenía uno con ese nombre) y archivo-fallo (disco lleno).
  const item = (pushId, i, files, name, extra) => Object.assign({ pushId, fileId: pushId + "-" + i, label: "Ejercicio 4", index: i,
    files, name, dest: "Ejercicio 4", path: "Ejercicio 4/" + name, size: 2048, renamed: false, replaced: false, reason: "", at: ahora }, extra);
  const plantilla = { pushId: "fp-1", label: "Ejercicio 4", files: 3, folder: "Ejercicio 4", at: ahora, items: [
    item("fp-1", 1, 3, "main.py"), item("fp-1", 2, 3, "datos.csv"), item("fp-1", 3, 3, "LEEME.txt")] };
  const suelto = { pushId: "fp-2", label: "", files: 1, folder: "", at: ahora,
    items: [item("fp-2", 1, 1, "ventas.xlsx", { label: "", dest: "", path: "ventas.xlsx" })] };
  const received = {
    archivos: [suelto, plantilla],
    "archivo-renombrado": [{ pushId: "fp-3", label: "", files: 1, folder: "Ejercicio 4", at: ahora,
      items: [item("fp-3", 1, 1, "datos.csv", { label: "", path: "Ejercicio 4/datos (2).csv", renamed: true })] }],
    "archivo-sin-carpeta": [suelto],
    "archivo-fallo": [{ pushId: "fp-4", label: "Proyecto final", files: 2, folder: "Proyecto", at: ahora,
      items: [item("fp-4", 1, 2, "base.sql", { label: "Proyecto final", dest: "Proyecto", path: "Proyecto/base.sql" }),
        item("fp-4", 2, 2, "imagenes.zip", { label: "Proyecto final", dest: "Proyecto", path: "", reason: "no_space" })] }],
  }[scene] || [];
  const files = { received, folder: "Aula" };
  // Las recolecciones del profesor (Collections de apps/student/collect.go, prompt 10.10).
  // Escenas: recoleccion (una abierta y otra recogida, con lo que no se recogió) y
  // recoleccion-aviso (los avisos de antes y después de recoger).
  const cero = "0001-01-01T00:00:00Z";
  const abierta = { id: "rc-2", date: "2026-11-06", activity: "Ejercicio 5", folder: "recolecciones/2026-11-06/Ejercicio 5",
    patterns: [], openedAt: ahora, state: "open", rounds: [] };
  const recogida = { id: "rc-1", date: "2026-11-06", activity: "Ejercicio 4", folder: "recolecciones/2026-11-06/Ejercicio 4",
    patterns: ["*.py"], openedAt: "2026-11-06T10:05:00-06:00", state: "collected", rounds: [
      { round: 1, at: "2026-11-06T10:42:00-06:00", doneAt: "2026-11-06T10:42:03-06:00",
        files: [{ path: "20231234-Ana.py", size: 3120, saved: true }],
        skipped: [{ path: "video.py", size: 52428800, reason: "too_large" }] }] };
  const recogiendo = Object.assign({}, recogida, { state: "requested", rounds: [{ round: 1, at: ahora, doneAt: cero, files: [], skipped: [] }] });
  const collections = { list: { recoleccion: [recogida, abierta], "recoleccion-aviso": [recogida], "recoleccion-recogiendo": [recogiendo] }[scene] || [], folder: "Aula" };
  if (scene === "recoleccion-aviso") {
    notices.push({ id: "rc-n1", level: "info", text: "", kind: "collect-requested", ref: "rc-1", at: "2026-11-06T10:42:00-06:00" },
      { id: "rc-n2", level: "info", text: "", kind: "collect-done", ref: "rc-1", at: "2026-11-06T10:42:03-06:00" });
  }
  // Lo que abrió el profesor (Opened de apps/student/open.go, prompt 10.11): en «abierto», una
  // página y una app, cada una con su aviso en la esquina (privacidad, regla 66).
  const opened = { list: scene === "abierto" ? [
    { id: "op-1", kind: "url", shown: "docs.python.org/3/tutorial", full: "https://docs.python.org/3/tutorial/?highlight=for",
      teacher: status.teacher, channel: status.channel, at: "2026-11-09T10:40:00-06:00" },
    { id: "op-2", kind: "app", shown: "Thonny", full: "Thonny", teacher: status.teacher, channel: status.channel, at: "2026-11-09T10:41:00-06:00" },
  ] : [] };
  if (scene === "abierto") {
    notices.push({ id: "op-n1", level: "info", text: "", kind: "opened", ref: "op-1", at: "2026-11-09T10:40:00-06:00" },
      { id: "op-n2", level: "info", text: "", kind: "opened", ref: "op-2", at: "2026-11-09T10:41:00-06:00" });
  }

  // Las encuestas del profesor (poll.BallotView de internal/poll, prompt 10.12), lo más nuevo
  // primero. Escenas: encuesta (una de opción múltiple abierta y una de sí o no cerrada con los
  // resultados de la clase) y encuesta-texto (una abierta corta, nominal).
  const tarjeta = (pollId, kind, question, options, extra) => Object.assign({ pollId, kind, question,
    options, named: false, open: true, choice: -1, text: "", vote: "none", results: null, at: ahora,
    questions: [], answers: [], submitted: false, result: null }, extra);
  // Cuestionarios (prompt 10.20): las preguntas como las ve este alumno (sin la correcta). Escenas:
  // cuestionario (abierto, la primera respondida) y cuestionario-resultado (entregado, cerrado y
  // con su resultado personal).
  const preguntas = [
    { id: 0, kind: "choice", text: "¿Qué imprime print(3 // 2)?", points: 2,
      options: [{ id: 2, text: "2" }, { id: 0, text: "1.5" }, { id: 1, text: "1" }] },
    { id: 1, kind: "choice", text: "¿Cuáles de estas palabras empiezan un ciclo en Python?", points: 3, multi: true,
      options: [{ id: 0, text: "for" }, { id: 1, text: "if" }, { id: 2, text: "while" }, { id: 3, text: "def" }] },
    { id: 2, kind: "yesno", text: "¿range(3) incluye el 3?", points: 1 },
  ];
  const cuestionario = (extra) => tarjeta("qz-1", "quiz", "Repaso de ciclos", [], Object.assign({ named: true,
    questions: preguntas, answers: [{ q: 0, choices: [1] }], vote: "accepted" }, extra));
  const polls = { arrived: "", cards: {
    encuesta: [
      tarjeta("pl-2", "choice", "¿Qué imprime print(3 // 2)?", ["1.5", "1", "2", "Error"]),
      tarjeta("pl-1", "yesno", "¿Entendiste cómo funciona el ciclo for?", [], { open: false, choice: 0, vote: "accepted",
        at: "2026-11-11T10:05:00-06:00", results: { counts: [14, 4], voters: 18, recipients: 20 } }),
    ],
    "encuesta-texto": [tarjeta("pl-3", "open", "¿Qué fue lo que más te costó del ejercicio?", [], { named: true })],
    cuestionario: [cuestionario()],
    "cuestionario-resultado": [cuestionario({ open: false, submitted: true,
      answers: [{ q: 0, choices: [1] }, { q: 1, choices: [0] }, { q: 2, choices: [1] }],
      result: { pollId: "qz-1", score: 3, max: 6, questions: [
        { q: 0, choices: [1], correct: [1], earned: 2 }, { q: 1, choices: [0], correct: [0, 2], earned: 0 },
        { q: 2, choices: [1], correct: [1], earned: 1 }] } })],
  }[scene] || [] };
  const copiaPolls = () => JSON.parse(JSON.stringify(polls));
  // Una encuesta que la prueba emite (como si llegara de Go) queda también en el doble, para
  // poder responderla.
  const emitirBase = window.__emit;
  window.__emit = (name, data) => {
    if (name === "aula:polls" && data) polls.cards = JSON.parse(JSON.stringify(data.cards || []));
    emitirBase(name, data);
  };
  const responder = (pollId, choice, text) => {
    const c = polls.cards.find((x) => x.pollId === pollId);
    if (!c) return new Error("aula:unknown");
    if (!c.open) return new Error("aula:closed");
    if (c.kind === "open" && !text.trim()) return new Error("aula:empty");
    Object.assign(c, { choice: c.kind === "open" ? -1 : choice, text: c.kind === "open" ? text.trim() : "", vote: "sending" });
    setTimeout(() => { c.vote = "accepted"; window.__emit("aula:polls", copiaPolls()); }, 300);
    return copiaPolls();
  };
  // AnswerQuiz: guarda lo marcado; «Entregar» lo deja fijo. El acuse llega poco después.
  const responderCuestionario = (pollId, answers, submit) => {
    const c = polls.cards.find((x) => x.pollId === pollId);
    if (!c) return new Error("aula:unknown");
    if (!c.open) return new Error("aula:closed");
    if (c.submitted) return new Error("aula:handedIn");
    Object.assign(c, { answers: JSON.parse(JSON.stringify(answers)), vote: "sending" });
    setTimeout(() => { c.vote = "accepted"; if (submit) c.submitted = true; window.__emit("aula:polls", copiaPolls()); }, 300);
    return copiaPolls();
  };


  // Lo que Go contestaría: el acuse llega poco después («Enviando…» → «Recibido»).
  let nMsg = 0;
  let ultimo = 0;
  const escribir = (text) => {
    if (!text.trim()) return new Error("aula:empty");
    if (Date.now() - ultimo < 3000) return new Error("aula:tooSoon");
    ultimo = Date.now();
    const m = enviado("pm-n" + ++nMsg, text.trim(), "sending", 0);
    part.messages.push(m);
    setTimeout(() => { m.status = "received"; window.__emit("aula:participation", copia()); }, 400);
    return copia();
  };

  const ok = (v) => new Promise((r) => setTimeout(() => r(v), 5));
  // Un doble que devuelve un Error rechaza la promesa con su texto, como Wails.
  const call = (name, v) => (...args) => {
    window.__calls.push([name, args]);
    const r = typeof v === "function" ? v(...args) : v;
    return r instanceof Error ? new Promise((_, no) => setTimeout(() => no(r.message), 5)) : ok(r);
  };
  // Terminar la clase (prompt 10.14): en fin-clase, la ventana vacía con la tarjeta «La clase terminó».
  let classEnd = scene === "fin-clase" ? { endId: "fin-1", at: Date.now() - 60000, removed: { control: false, present: false,
    broadcast: false, pause: false, restrictions: true, timer: true, hand: true, polls: 1, snippets: 3, notices: 1, files: 1,
    collections: 0, opened: 0, messages: 0 } } : null;
  window.go = { main: { App: {
    View: call("View", () => ({ status, snippets, notices, broadcast, restrictions, power, participation: copia(), timer, files, collections, opened, polls: copiaPolls(), present, classEnd,
      capabilities: ["code.view", "msg.show", "hand", "msg.student", "timer", "poll", "poll.quiz", "file.push", "file.collect", "screen.broadcast",
        "screen.present.view", "screen.thumb", "screen.snapshot", "screen.present", "remote", "power", "open.url", "open.app", "class.end", "computer.label"],
      // Este equipo (prompt 10.15): con etiqueta salvo en sin-etiqueta.
      computer: { hostname: "LAB3-PC07", label: scene === "sin-etiqueta" ? "" : "Fila 2 · PC07" } })),
    Copy: call("Copy", () => (scene === "copia-falla" ? new Error("el portapapeles no está disponible en este equipo") : null)),
    DismissNotice: call("DismissNotice"),
    DismissClassEnd: call("DismissClassEnd", () => { classEnd = null; return null; }),
    ReportMedia: call("ReportMedia"),
    BroadcastAnswer: call("BroadcastAnswer"),
    BroadcastCandidate: call("BroadcastCandidate"),
    BroadcastFailed: call("BroadcastFailed"),
    PowerPostpone: call("PowerPostpone"),
    DismissPower: call("DismissPower"),
    SetCompact: call("SetCompact"),
    ReportWindow: call("ReportWindow"),
    RaiseHand: call("RaiseHand", (note) => { part.hand = "up"; part.note = (note || "").trim(); return copia(); }),
    LowerHand: call("LowerHand", () => { part.hand = "down"; part.note = ""; return copia(); }),
    SendTeacherMessage: call("SendTeacherMessage", escribir),
    OpenReceivedFolder: call("OpenReceivedFolder", (id) => (scene === "archivo-sin-carpeta" ? new Error("esa carpeta ya no está disponible") : null)),
    OpenCollectionFolder: call("OpenCollectionFolder"),
    AnswerPoll: call("AnswerPoll", responder),
    AnswerQuiz: call("AnswerQuiz", responderCuestionario),
  } } };
})();
