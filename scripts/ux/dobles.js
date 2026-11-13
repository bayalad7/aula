// Doble de los bindings de Wails (window.go.main.App y window.runtime) para
// recorrer el frontend real de apps/teacher sin backend. Los datos tienen la
// forma exacta de apps/teacher/events.go y docs/eventos-ui.md: si cambia un
// binding o un evento, cambia aquí también. La escena se elige con
// window.__SCENE y el manual llega en window.__MANUAL (lo exporta
// TestExportManualForUX) antes de cargar la página. Ver scripts/ux/README.md.
(function () {
  const scene = window.__SCENE || "datos";
  const now = Date.now();
  const handlers = {};
  window.runtime = {
    EventsOn(name, cb) { (handlers[name] = handlers[name] || []).push(cb); return () => {}; },
    EventsOff() {},
  };
  window.__emit = (name, data) => (handlers[name] || []).forEach((cb) => cb(data));
  window.__calls = [];

  const caps = (s) => ({
    canCode: s.capabilities.includes("code.view"), canNotice: s.capabilities.includes("msg.show"),
    canThumb: s.capabilities.includes("screen.thumb"), canBroadcast: s.capabilities.includes("screen.broadcast"),
    canAppBlock: s.capabilities.includes("app.block"), canWebFilter: s.capabilities.includes("web.filter"),
    canPower: s.capabilities.includes("power"), canRemote: s.capabilities.includes("remote"),
    canSnapshot: s.capabilities.includes("screen.snapshot"),
    canHand: s.capabilities.includes("hand"), canMessage: s.capabilities.includes("msg.student"),
    canTimer: s.capabilities.includes("timer"), canPoll: s.capabilities.includes("poll"),
    canFiles: s.capabilities.includes("file.push"),
    canCollect: s.capabilities.includes("file.collect"),
    canOpenUrl: s.capabilities.includes("open.url"), canOpenApp: s.capabilities.includes("open.app"),
    canPresent: s.capabilities.includes("screen.present"), canPresentView: s.capabilities.includes("screen.present.view"),
    canPause: s.capabilities.includes("screen.freeze"), canClassEnd: s.capabilities.includes("class.end"),
  });
  const ALL = ["code.view", "msg.show", "screen.thumb", "screen.broadcast", "app.block", "web.filter", "power", "remote", "screen.snapshot",
    "hand", "msg.student", "timer", "file.push", "file.collect", "open.url", "open.app", "poll", "screen.present", "screen.present.view", "screen.freeze", "class.end"];
  const student = (o) => Object.assign({
    id: "s-" + o.hostname, username: "", os: "windows", osVersion: "10.0.19045", agentVersion: "0.5.0",
    state: "connected", capabilities: [], power: null, appsActive: false, webActive: false,
    lastCodeAck: "", ackedAt: "", simulated: false, connectedAt: "10:31", lastError: null, hand: 0, newMessages: 0,
    pause: "", pauseUnacked: false, canWake: false, wake: null,
  }, o, caps(o));

  // muchos_alumnos -n 2: SIM-01, SIM-02 (code.view y msg.show).
  const sims = [
    student({ hostname: "SIM-01", username: "simulado", osVersion: "simulado", agentVersion: "0.1.0-sim",
      capabilities: ["code.view", "msg.show"], lastCodeAck: "sn-3", ackedAt: "10:42:07" }),
    student({ hostname: "SIM-02", username: "simulado", osVersion: "simulado", agentVersion: "0.1.0-sim",
      capabilities: ["code.view", "msg.show"], lastCodeAck: "sn-2", ackedAt: "10:39:51" }),
  ];
  const lab = [
    student({ hostname: "LAB3-PC07", username: "alumno07", capabilities: ALL, lastCodeAck: "sn-3", ackedAt: "10:42:06" }),
    student({ hostname: "LAB3-PC12", username: "alumno12", capabilities: ALL, appsActive: true, webActive: true,
      lastCodeAck: "sn-3", ackedAt: "10:42:08" }),
    student({ hostname: "LAB3-PC15", username: "", capabilities: ["power"] }),
    student({ hostname: "lab-ubuntu-02", username: "maria", os: "linux", osVersion: "Ubuntu 24.04",
      capabilities: ["code.view", "msg.show", "hand", "msg.student", "timer", "poll"], lastCodeAck: "sn-2", ackedAt: "10:39:50",
      lastError: scene === "lab-errores" ? { code: "unsupported_capability", at: "10:43:12" } : null }),
    student({ hostname: "LAB3-PC03", username: "alumno03", state: "disconnected", capabilities: ALL, canWake: true }),
  ];
  for (const s of lab) if (s.os === "windows") s.canWake = true;
  // Encender (apps/teacher/wake.go): equipos que la consola conoce y no tienen sesión. En
  // lab-encender, uno está «Encendiendo…» y otro «No respondió» (y fuera de la red de la consola).
  const haceHoras = (h) => new Date(now - h * 3600000).toISOString();
  let off = scene.startsWith("lab") ? [
    { hostname: "LAB3-PC01", lastSeen: haceHoras(2), wake: null },
    { hostname: "LAB3-PC02", lastSeen: haceHoras(26), wake: null },
    { hostname: "LAB3-PC09", lastSeen: haceHoras(3), wake: null },
  ] : [];
  if (scene === "lab-encender") {
    off[1].wake = { state: "waking", deadline_ms: now + 142000, other_subnet: false };
    off[2].wake = { state: "noresponse", deadline_ms: now - 1000, other_subnet: true };
  }
  const simStudents = [1, 2].map((i) => student({ hostname: "Simulado " + i, username: "Alumno de prueba", osVersion: "simulado",
    agentVersion: "0.6.0-dev+sim", capabilities: ["code.view", "msg.show", "screen.thumb", "screen.snapshot", "file.push", "file.collect", "poll", "class.end"], simulated: true,
    lastCodeAck: i === 1 ? "sn-3" : "", ackedAt: i === 1 ? "10:42:09" : "" }));
  let list = scene === "simulada-sola" ? simStudents.slice() : scene === "simulada" ? [...sims.slice(0, 1), ...simStudents] : scene === "vacia" || scene.startsWith("setup") ? [] : scene.startsWith("lab") ? lab : [...sims,
    student({ hostname: "SIM-03", username: "simulado", osVersion: "simulado", agentVersion: "0.1.0-sim",
      state: "disconnected", capabilities: ["code.view", "msg.show"] })];
  if (scene === "lab-energia-pendiente") {
    lab[0].power = { action_id: "p1", action: "reboot", force: false, status: "countdown", deadline_ms: now + 24000, postpones_left: 2 };
    lab[1].power = { action_id: "p1", action: "reboot", force: false, status: "postponed", deadline_ms: now + 318000, postpones_left: 1 };
    lab[2].power = { action_id: "p1", action: "reboot", force: false, status: "executing", deadline_ms: 0, postpones_left: 2 };
  }
  // Participación (apps/teacher/participation.go): en lab-participacion, dos manos en la
  // fila y dos mensajes, uno nuevo y uno respondido. Los alumnos llevan su insignia.
  const quien = (s) => ({ hostname: s.hostname, username: s.username, simulated: !!s.simulated });
  const minutos = (m) => new Date(now - m * 60000).toISOString();
  let part = { hands: [], messages: [], pending: 0, arrival: null };
  if (scene === "lab-participacion") {
    part.hands = [
      Object.assign(quien(lab[1]), { handId: "h1", note: "No me compila el for de la línea 12", raisedAt: minutos(3), position: 1, sessionId: lab[1].id }),
      Object.assign(quien(lab[0]), { handId: "h2", note: "", raisedAt: minutos(1), position: 2, sessionId: lab[0].id }),
    ];
    part.messages = [
      Object.assign(quien(lab[3]), { messageId: "m2", text: "¿El ejercicio 3 es para hoy o para la próxima clase?", at: minutos(2), status: "new", sessionId: lab[3].id }),
      Object.assign(quien(lab[0]), { messageId: "m1", text: "Ya terminé el ejercicio 2", at: minutos(9), status: "replied",
        reply: "¡Bien! Sigue con el 3.", repliedAt: minutos(8), sessionId: lab[0].id }),
    ];
  }
  const marcarParticipacion = () => {
    part.pending = part.hands.length + part.messages.filter((m) => m.status === "new").length;
    for (const s of list) {
      const h = part.hands.find((x) => x.hostname === s.hostname && x.simulated === !!s.simulated);
      s.hand = h ? h.position : 0;
      s.newMessages = part.messages.filter((m) => m.hostname === s.hostname && m.status === "new").length;
    }
  };
  marcarParticipacion();
  const emitPart = (arrival) => {
    part.hands.forEach((h, i) => { h.position = i + 1; });
    marcarParticipacion();
    window.__emit("aula:participation", Object.assign({}, part, { arrival: arrival || null }));
    window.__emit("aula:students", studentsView());
  };
  // Para el recorrido: llega una mano como si un alumno la levantara.
  window.__llegaMano = (host) => {
    const s = list.find((x) => x.hostname === host);
    part.hands.push(Object.assign(quien(s), { handId: "h" + Date.now(), note: "", raisedAt: new Date().toISOString(), position: 0, sessionId: s.id }));
    emitPart(Object.assign({ kind: "hand" }, quien(s)));
  };
  const mismo = (a, b) => a.hostname === b.hostname && a.username === b.username && !!a.simulated === !!b.simulated;
  const studentsView = () => {
    const c = list.filter((s) => s.state === "connected");
    const real = c.filter((s) => !s.simulated);
    const n = (k) => c.filter((s) => s[k]).length;
    const r = (k) => real.filter((s) => s[k]).length;
    return { students: list, connected: real.length, simulated: c.length - real.length,
      canCode: n("canCode"), canNotice: n("canNotice"), canThumb: n("canThumb"),
      canBroadcast: r("canBroadcast"), canAppBlock: r("canAppBlock"), canWebFilter: r("canWebFilter"), canPower: r("canPower"),
      canSnapshot: n("canSnapshot"), canTimer: n("canTimer"), canPoll: n("canPoll"), canCollect: n("canCollect"),
      canOpenUrl: r("canOpenUrl"), canOpenApp: r("canOpenApp"), canPause: r("canPause"), off };
  };

  // Abrir página o app (apps/teacher/open.go): el catálogo de Ajustes y las últimas órdenes.
  // En lab-abrir ya hay una abierta en un equipo y fallida en otro (no tiene la app).
  const appsEjemplo = [
    { id: "bloc-de-notas", name: "Bloc de notas", path: "%SystemRoot%\\System32\\notepad.exe" },
    { id: "calculadora", name: "Calculadora", path: "%SystemRoot%\\System32\\calc.exe" },
    { id: "paint", name: "Paint", path: "%SystemRoot%\\System32\\mspaint.exe" },
    { id: "vscode", name: "Visual Studio Code", path: "%LOCALAPPDATA%\\Programs\\Microsoft VS Code\\Code.exe" },
    { id: "thonny", name: "Thonny", path: "%LOCALAPPDATA%\\Programs\\Thonny\\thonny.exe" },
    { id: "scratch", name: "Scratch 3", path: "%ProgramFiles(x86)%\\Scratch 3\\Scratch 3.exe" },
  ];
  const rutaApps = "C:\\Users\\profe\\AppData\\Roaming\\Aula\\consola\\apps-permitidas.json";
  let allowed = { apps: appsEjemplo.slice(), source: "ejemplos", path: rutaApps };
  let opens = [];
  const contar = (o) => {
    o.opened = o.targets.filter((t) => t.status === "opened").length;
    o.pending = o.targets.filter((t) => t.status === "sent").length;
    o.refused = o.targets.length - o.opened - o.pending;
    return o;
  };
  const nuevaOrden = (kind, label, hosts, capacidad) => {
    const objetivo = list.filter((s) => s.state === "connected" && !s.simulated && (!hosts.length || hosts.includes(s.hostname)));
    const o = { id: "o" + (opens.length + 1), kind, label, at: new Date().toISOString(), failed: [],
      targets: objetivo.filter((s) => s[capacidad]).map((s) => ({ sessionId: s.id, hostname: s.hostname, status: "sent" })),
      unsupported: objetivo.filter((s) => !s[capacidad]).map((s) => s.hostname) };
    opens = [contar(o), ...opens].slice(0, 5);
    return o;
  };
  if (scene === "lab-abrir") {
    const o = nuevaOrden("app", "Thonny", [], "canOpenApp");
    o.targets[0].status = "opened";
    if (o.targets[1]) Object.assign(o.targets[1], { status: "failed", reason: "not_installed" });
    contar(o);
  }
  const responderOrden = (o) => despues(() => {
    o.targets.forEach((t, i) => Object.assign(t, i === 1 && o.kind === "app" ? { status: "failed", reason: "not_installed" } : { status: "opened" }));
    contar(o);
    window.__emit("aula:opens", opens.slice());
  });
  const urlValida = (u) => {
    const s = u.trim().includes("://") ? u.trim() : "https://" + u.trim();
    return /^https?:\/\/[^\s/@]+(\/\S*)?$/i.test(s) ? s : null;
  };

  const code3 = "def factorial(n):\n    if n <= 1:\n        return 1\n    return n * factorial(n - 1)\n\nprint(factorial(5))\n";
  const history = scene === "vacia" || scene.startsWith("setup") ? [] : [
    { snippetId: "sn-1", number: 1, title: "Hola mundo", lang: "python", content: "print('Hola, mundo')\n", times: 1 },
    { snippetId: "sn-2", number: 2, title: "Suma de una lista", lang: "python", content: "nums = [3, 5, 8]\ntotal = 0\nfor n in nums:\n    total += n\nprint(total)\n", times: 2 },
    { snippetId: "sn-3", number: 3, title: "Factorial recursivo", lang: "python", content: code3, times: 1 },
  ].map((e) => Object.assign({ firstSentAt: "2026-10-29T10:30:00-06:00", lastSentAt: "2026-10-29T10:42:00-06:00" }, e));
  const deliveriesBase = history.length === 0 ? [] : [
    { id: "d5", kind: "code", label: "Ejercicio 3 · Factorial recursivo", sent: 3, acked: 2, failed: 1, noCapability: 1, at: "",
      pending: [list.length > 1 ? list[1].id : "s-x"], simSent: scene.startsWith("simulada") ? 2 : 0, simAcked: scene.startsWith("simulada") ? 1 : 0,
      rejected: scene === "lab-errores" ? [{ session: "s-lab-ubuntu-02", code: "internal" }] : [] },
    { id: "d4", kind: "notice", label: "Aviso: Guarden su trabajo: en 5 minutos cerramos.", sent: 2, acked: 2, failed: 0, noCapability: 0, at: "" },
    { id: "d3", kind: "code", label: "Ejercicio 2 · Suma de una lista (reemitido)", sent: 2, acked: 2, failed: 0, noCapability: 0, at: "" },
    { id: "d2", kind: "code", label: "Ejercicio 2 · Suma de una lista", sent: 2, acked: 1, failed: 0, noCapability: 0, at: "" },
    { id: "d1", kind: "code", label: "Ejercicio 1 · Hola mundo", sent: 2, acked: 2, failed: 0, noCapability: 0, at: "" },
  ];
  // Un envío de archivos (prompt 10.9): uno lo tiene, a otro le falta por disco lleno.
  if (scene === "lab-archivos") deliveriesBase.unshift({ id: "d6", kind: "file", label: "Archivos: Ejercicio 4 (3)", sent: 2, acked: 1,
    failed: 0, noCapability: 1, at: "", rejected: [{ session: "s-LAB3-PC12", code: "no_space" }] });
  const deliveries = deliveriesBase.map((d) => Object.assign({ pending: [], rejected: [], simSent: 0, simAcked: 0 }, d));
  const restrictions = scene === "lab-restr-activa"
    ? { apps: { policyId: "r1", mode: "deny", items: ["steam", "discord", "whatsapp"], hosts: [] },
        web: { policyId: "r2", mode: "allow", items: ["python.org", "w3schools.com"], hosts: ["LAB3-PC12"] } }
    : { apps: null, web: null };
  const personal = scene === "clave-personal";
  const cls = { channel: "1", consoleName: "Prof. Ayala", publicKey: "aula-ed25519:Hq0rB7Gm1o5dYH0yXx3l0k3pG2b8m2w7m0K1q0b9JzA=",
    fingerprint: "K7QD-3MX2-9PLA-W4TZ", keySource: personal ? "personal" : "centro", port: 47830, certExpires: "2036-10-29" };
  const bcActive = scene === "lab-transmitiendo";
  // La pantalla de un alumno mostrada a la clase (PresentState de apps/teacher/present.go,
  // prompt 12.1). Escenas: lab-proyectar-conectando y lab-proyectando (la ven 1 de 2). «Proyectar a la clase» conecta y,
  // poco después, la ven los dos (el alumno no responde nada: lo decide el profesor).
  const sinProyeccion = { active: false, sessionId: "", hostname: "", phase: "", receivers: 0, eligible: 0, ended: "", reason: "" };
  const proyeccionDe = (phase, receivers) => ({ active: true, sessionId: "s-LAB3-PC07", hostname: "LAB3-PC07", phase,
    receivers, eligible: 2, ended: "", reason: "" });
  let present = scene === "lab-proyectando" ? proyeccionDe("live", 1)
    : scene === "lab-proyectar-conectando" ? proyeccionDe("connecting", 0) : Object.assign({}, sinProyeccion);
  // Control remoto (apps/teacher/remote.go, remote.Control): LAB3-PC07 en lab-control*.
  const pc07 = { sessionID: "s-LAB3-PC07", hostname: "LAB3-PC07", controlID: "c-1", viewing: true };
  let remote = scene === "lab-control" ? Object.assign({ status: "active" }, pc07)
    : scene === "lab-control-pausa" ? Object.assign({ status: "paused" }, pc07)
    : { sessionID: "", hostname: "", controlID: "", status: "", viewing: false };
  // Lo que el agente contesta solo: active y una imagen al tomar, ended al soltar.
  const despues = (fn) => setTimeout(fn, 60);
  // Terminar la clase: la última classend.View.
  let fin = null;
  const cuentaFin = () => {
    const falta = (s) => ["waiting", "noack", "away", "failed"].includes(s.state);
    fin.missing = fin.students.filter(falta).length;
    fin.clean = fin.students.length - fin.missing;
  };
  const cuadroControl = () => window.__emit("aula:remote-frame", { sessionId: remote.sessionID, seq: 1, width: 320, height: 180,
    jpeg: window.__thumb("main.py - Visual Studio Code", 210), detail: true, at: new Date().toISOString() });
  // Capturas (apps/teacher/snapshot.go, snapshot.Snapshot): llegan «ready» poco después de
  // pedirlas; en lab-captura-falla, la de LAB3-PC12 falla como con la sesión bloqueada.
  const DOCS = "C:\\Users\\profe\\Documents\\Aula\\capturas";
  let snaps = [];
  let nSnap = 0;
  const emitSnaps = () => window.__emit("aula:snapshots", { snapshots: snaps.slice() });
  const pedirCaptura = (id) => {
    const s = list.find((x) => x.id === id) || {};
    const c = { id: "cap-" + ++nSnap, sessionId: id, hostname: s.hostname, username: s.username || "", simulated: !!s.simulated,
      status: "pending", requestedAt: new Date().toISOString() };
    snaps.push(c);
    despues(() => {
      const falla = scene === "lab-captura-falla" && s.hostname === "LAB3-PC12";
      Object.assign(c, falla ? { status: "failed", reason: "capture" }
        : { status: "ready", width: 1920, height: 1080, size: 1843211, takenAt: "2026-11-05T10:42:06-06:00" });
      emitSnaps();
    });
    return Object.assign({}, c);
  };
  // Temporizador (apps/teacher/timer.go, classtimer.Snapshot): «Ejercicio 1» con 4:32 en
  // lab-temporizador; terminado en lab-temporizador-fin. LAB3-PC15 no lo puede ver.
  const sinTemporizador = () => ({ active: false, timerId: "", label: "", durationS: 0, remainingMs: 0, ended: false, all: true,
    hosts: [], maxExtendS: 0, receivers: [], eligible: 0, unsupported: [] });
  const receptores = (estado) => list.filter((s) => s.state === "connected" && s.canTimer).map((s, i) => ({
    sessionId: s.id, hostname: s.hostname, username: s.username, simulated: !!s.simulated,
    status: estado || (i === 2 ? "sent" : "running") }));
  const temporizadorDe = (label, durationS, remainingMs, hosts) => ({ active: true, timerId: "t-" + Date.now(), label, durationS,
    remainingMs, ended: remainingMs <= 0, all: hosts.length === 0, hosts, maxExtendS: 4 * 3600 - Math.ceil(remainingMs / 1000),
    receivers: receptores(remainingMs <= 0 ? "ended" : ""), eligible: list.filter((s) => s.state === "connected" && s.canTimer).length,
    unsupported: list.filter((s) => s.state === "connected" && !s.canTimer).map((s) => s.hostname) });
  let timer = scene === "lab-temporizador" ? temporizadorDe("Ejercicio 1", 600, 272000, [])
    : scene === "lab-temporizador-fin" ? temporizadorDe("Ejercicio 1", 600, 0, []) : sinTemporizador();
  const emitTimer = () => window.__emit("aula:timer", Object.assign({}, timer));
  // Encuestas (apps/teacher/poll.go, poll.View). lab-encuesta: una de opción múltiple abierta,
  // anónima, con 3 respuestas (una simulada) y una de sí o no cerrada y compartida;
  // lab-encuesta-nominal: una abierta corta nominal, con quién respondió y quién falta.
  const conPoll = () => list.filter((s) => s.state === "connected" && s.canPoll);
  const encuestaDe = (o) => Object.assign({ pollId: "pl-" + Date.now(), kind: "choice", question: "", named: false, open: true,
    shared: false, shareable: false, all: true, hosts: [], openedAt: "2026-11-11T10:12:00-06:00", closedAt: null, options: [],
    answered: 0, answeredSimulated: 0, received: conPoll().length, eligible: conPoll().length,
    unsupported: list.filter((s) => s.state === "connected" && !s.canPoll).map((s) => s.hostname), answers: [], missing: [] }, o);
  const anterior = encuestaDe({ pollId: "pl-1", kind: "yesno", question: "¿Entendiste cómo funciona el ciclo for?", open: false,
    shared: true, openedAt: "2026-11-11T10:02:00-06:00", closedAt: "2026-11-11T10:05:00-06:00",
    options: [{ label: "", count: 2, simulated: 0 }, { label: "", count: 1, simulated: 0 }], answered: 3, unsupported: [] });
  const polls = {
    "lab-encuesta": { current: encuestaDe({ pollId: "pl-2", question: "¿Qué imprime print(3 // 2)?",
      options: [{ label: "1.5", count: 1, simulated: 0 }, { label: "1", count: 1, simulated: 1 }, { label: "2", count: 0, simulated: 0 }, { label: "Error", count: 0, simulated: 0 }],
      answered: 2, answeredSimulated: 1, eligible: 4, received: 4 }), closed: [anterior] },
    "lab-encuesta-nominal": { current: encuestaDe({ pollId: "pl-3", kind: "open", question: "¿Qué fue lo que más te costó del ejercicio?",
      named: true, answered: 2, answers: [
        { hostname: "LAB3-PC07", username: "alumno07", simulated: false, choice: -1, text: "Los índices de la lista", at: "2026-11-11T10:13:00-06:00" },
        { hostname: "LAB3-PC12", username: "alumno12", simulated: false, choice: -1, text: "Cuándo usar while", at: "2026-11-11T10:13:20-06:00" }],
      missing: [{ hostname: "lab-ubuntu-02", username: "maria", simulated: false }] }), closed: [] },
  }[scene] || { current: null, closed: [] };
  const emitPolls = () => window.__emit("aula:polls", JSON.parse(JSON.stringify(polls)));
  const copiaPolls = () => JSON.parse(JSON.stringify(polls));
  // Pausar pantallas (apps/teacher/pause.go): en lab-pausa, dos equipos en pausa, uno que no
  // acusó y el mensaje en vigor. Los dobles acusan al momento (como un agente rápido).
  let pausaMensaje = "";
  if (scene === "lab-pausa") {
    lab[0].pause = "paused";
    lab[1].pause = "sent";
    lab[1].pauseUnacked = true;
    pausaMensaje = "Miren al pizarrón un momento.";
  }
  const resumenPausas = () => {
    const c = list.filter((s) => s.state === "connected" && !s.simulated);
    return { paused: c.filter((s) => s.pause === "paused" || s.pause === "sent").length,
      unacked: c.filter((s) => s.pause === "sent" && s.pauseUnacked).length,
      failed: c.filter((s) => s.pause === "failed").length, message: pausaMensaje };
  };
  const emitPausas = () => { window.__emit("aula:pauses", resumenPausas()); window.__emit("aula:students", studentsView()); };
  const pausaDe = (hosts) => list.filter((s) => s.state === "connected" && !s.simulated && (!hosts.length || hosts.includes(s.hostname)));
  const view = () => ({
    active: !scene.startsWith("setup"), class: cls, students: studentsView(), history, deliveries,
    hotkey: scene === "atajo-ocupado"
      ? { combo: "Ctrl+Alt+Shift+C", active: false, reason: "InUse", error: "" }
      : { combo: "Ctrl+Alt+Shift+C", active: true, reason: "", error: "" },
    broadcast: { active: bcActive, display: bcActive ? 0 : -1, receivers: bcActive ? 2 : 0, eligible: 2, available: true,
      mode: bcActive ? "fullscreen" : "", targeted: 0 },
    restrictions, remote, snapshots: snaps, participation: Object.assign({}, part, { arrival: null }),
    timer: Object.assign({}, timer), polls: copiaPolls(), present: Object.assign({}, present), pauses: resumenPausas(),
  });


  let simHand = "down";
  let simSent = 0;
  const simView = () => ({ active: scene.startsWith("simulada"), max: 40,
    capabilities: ["code.view", "msg.show", "screen.thumb", "screen.snapshot", "hand", "msg.student"],
    students: scene.startsWith("simulada") ? [
      { id: "x1", name: "Simulado 1", os: "windows", capabilities: ["code.view", "msg.show", "screen.thumb", "hand", "msg.student"], connected: true, fault: "none", delayMs: 0, viewed: false,
        snippets: [{ exercise: "3", title: "Factorial recursivo", lang: "python", lines: 6 }], notices: 1, lastNotice: "Guarden su trabajo",
        hand: simHand, sent: simSent },
      { id: "x2", name: "Simulado 2", os: "linux", capabilities: ["code.view", "msg.show"], connected: true, fault: "noack", delayMs: 0, viewed: false,
        snippets: [], notices: 0, lastNotice: "", hand: "down", sent: 0 }] : [] });
  // Archivos y plantillas (FilesView de apps/teacher/files.go). Las plantillas viven en
  // Documentos\Aula\plantillas; en «plantilla-grande», una no se puede enviar.
  const tplArchivo = (rel, size) => ({ rel, size });
  const filesView = { templatesError: "", templates: scene === "vacia" ? [] : [
    { name: "Ejercicio 4", size: 873000, problem: "", files: [tplArchivo("main.py", 1200), tplArchivo("datos/ventas.csv", 870000), tplArchivo("LEEME.txt", 1800)] },
    { name: "Proyecto final", size: 40000000, problem: "too_large", files: [tplArchivo("base.sql", 40000000)] }],
    sends: scene.startsWith("lab-archivos") ? [{ id: "f1", label: "Ejercicio 4", template: "Ejercicio 4", names: ["main.py", "ventas.csv", "LEEME.txt"],
      size: 873000, dest: "Ejercicio 4", overwrite: false, times: 1, lastAt: "10:44" }] : [] };
  const elegidos = [{ path: "C:\\Users\\ayala\\Documents\\clase\\datos.csv", rel: "datos.csv", size: 245000 },
    { path: "C:\\Users\\ayala\\Documents\\clase\\main.py", rel: "main.py", size: 2100 }];
  const envioArchivos = (r) => Object.assign({}, { id: "d7", kind: "file", label: r.template ? "Archivos: " + r.template + " (3)" : "Archivos: datos.csv y 1 más",
    sent: 2, acked: 0, failed: 0, noCapability: 1, simSent: 0, simAcked: 0, at: "", pending: ["s-LAB3-PC07", "s-LAB3-PC12"], rejected: [] });

  // Recolecciones (CollectionsView de apps/teacher/collect.go, prompt 10.10). Escenas:
  // lab-recolecciones (una recogida con alumnos en cada estado) y lab-recolecciones-nueva
  // (recién creada, sin recoger).
  const win = (...p) => p.join(String.fromCharCode(92)); // rutas de Windows, sin escapar barras
  const RECO = win("C:", "Users", "profe", "Documents", "Aula", "recolecciones");
  const cero = "0001-01-01T00:00:00Z";
  const recoAlumno = (s, status, extra) => Object.assign({ session: s.id, hostname: s.hostname, username: s.username, simulated: !!s.simulated,
    key: (s.hostname + "|" + s.username).toLowerCase(), status, reason: "", round: 0, at: cero, files: [], failedFiles: 0, skipped: [],
    dir: win(RECO, "2026-11-06", "Ejercicio 4", s.hostname + (s.username ? " (" + s.username + ")" : "")) }, extra);
  const recoData = () => {
    if (scene === "lab-recolecciones-nueva") return [{ id: "rc-1", date: "2026-11-06", activity: "Ejercicio 4", patterns: ["*.py"],
      limits: { maxFileSize: 4194304, maxTotal: 16777216, maxFiles: 20 }, openedAt: "2026-11-06T10:05:00-06:00", round: 0, collecting: false,
      dir: win(RECO, "2026-11-06", "Ejercicio 4"), students: [recoAlumno(lab[0], "ready"), recoAlumno(lab[1], "ready")] }];
    if (scene !== "lab-recolecciones") return [];
    return [{ id: "rc-1", date: "2026-11-06", activity: "Ejercicio 4", patterns: ["*.py"],
      limits: { maxFileSize: 4194304, maxTotal: 16777216, maxFiles: 20 }, openedAt: "2026-11-06T10:05:00-06:00", round: 1, collecting: false,
      dir: win(RECO, "2026-11-06", "Ejercicio 4"), students: [
        recoAlumno(lab[0], "collected", { round: 1, at: "2026-11-06T10:42:03-06:00",
          files: [{ path: "20231234-Ana.py", size: 3120, sha256: "ab".repeat(32) }] }),
        recoAlumno(lab[1], "collected", { round: 1, at: "2026-11-06T10:42:05-06:00",
          files: [{ path: "main.py", size: 2048, sha256: "cd".repeat(32) }, { path: "util/datos.py", size: 9216, sha256: "ef".repeat(32) }],
          skipped: [{ path: "video.py", size: 52428800, reason: "too_large" }] }),
        recoAlumno({ id: "s-LAB3-PC09", hostname: "LAB3-PC09", username: "alumno09" }, "empty", { round: 1, at: "2026-11-06T10:42:04-06:00" }),
        recoAlumno({ id: "s-LAB3-PC03", hostname: "LAB3-PC03", username: "alumno03" }, "failed", { round: 1, reason: "gone", at: "2026-11-06T10:42:09-06:00" }),
      ] }];
  };
  let recoList = recoData();
  const collectionsView = () => ({ list: recoList, folder: RECO, folderError: "", today: "2026-11-06",
    limits: { maxFileSize: 4194304, maxTotal: 16777216, maxFiles: 20 } });

  const ok = (v) => new Promise((r) => setTimeout(() => r(v), 5));
  // Un doble que devuelve un Error rechaza la promesa con su texto, como Wails.
  const call = (name, v) => (...args) => {
    window.__calls.push([name, args]);
    const r = typeof v === "function" ? v(...args) : v;
    return r instanceof Error ? new Promise((_, no) => setTimeout(() => no(r.message), 5)) : ok(r);
  };
  window.go = { main: { App: {
    Setup: call("Setup", () => ({
      recentChannels: ["1", "2", "14"], consoleName: "Prof. Ayala", publicKey: cls.publicKey, fingerprint: cls.fingerprint,
      version: "0.6.0-dev", defaultChannel: "1", keySource: "centro",
      startError: scene === "setup-error" ? "aula:portInUse: el puerto 47830 está ocupado (¿hay otra consola abierta?): listen tcp :47830: bind: Only one usage of each socket address (protocol/network address/port) is normally permitted." : "",
    })),
    View: call("View", view),
    // En setup-error, el canal escrito no es válido: Go lo rechaza (invalidChannel).
    StartClass: call("StartClass", () => (scene === "setup-error"
      ? new Error("aula:invalidChannel: el canal debe ser un número del 1 al 16000") : view())),
    EndClass: call("EndClass"),
    Emit: call("Emit", () => deliveries[0]),
    Reemit: call("Reemit", () => deliveries[2]),
    ClearScreens: call("ClearScreens", () => deliveries[0]),
    SendNotice: call("SendNotice", () => deliveries[1]),
    OpenScreens: call("OpenScreens"),
    CloseScreens: call("CloseScreens"),
    ViewScreen: call("ViewScreen"),
    CaptureClipboard: call("CaptureClipboard", () => ({})),
    Displays: call("Displays", [
      { index: 0, label: "Monitor 1 (principal) · 1920×1080", width: 1920, height: 1080, primary: true },
      { index: 1, label: "Monitor 2 · 1280×1024", width: 1280, height: 1024, primary: false },
    ]),
    StartBroadcast: call("StartBroadcast", () => view().broadcast),
    StopBroadcast: call("StopBroadcast", () => view().broadcast),
    BroadcastState: call("BroadcastState", () => view().broadcast),
    StartPresent: call("StartPresent", (id) => {
      const s = studentsView().students.find((x) => x.id === id);
      present = Object.assign(proyeccionDe("connecting", 0), { sessionId: id, hostname: s ? s.hostname : "" });
      setTimeout(() => {
        if (!present.active) return;
        present = Object.assign({}, present, { phase: "live", receivers: 2 });
        window.__emit("aula:present", Object.assign({}, present));
      }, 400);
      return Object.assign({}, present);
    }),
    StopPresent: call("StopPresent", () => { present = Object.assign({}, sinProyeccion); return Object.assign({}, present); }),
    PresentState: call("PresentState", () => Object.assign({}, present)),
    OpenPresentView: call("OpenPresentView"),
    ClosePresentView: call("ClosePresentView"),
    PresentViewAnswer: call("PresentViewAnswer"),
    PresentViewCandidate: call("PresentViewCandidate"),
    PresentViewFailed: call("PresentViewFailed"),
    PowerAction: call("PowerAction", { sent: 3, unsupported: ["lab-ubuntu-02"], failed: [] }),
    PowerCancel: call("PowerCancel", { sent: 3, unsupported: [], failed: [] }),
    WakeComputers: call("WakeComputers", (hosts) => {
      const quiere = (h) => !hosts.length || hosts.some((x) => x.toLowerCase() === h.toLowerCase());
      const res = { sent: 0, noMac: [], failed: [], otherSubnet: [] };
      const marcar = (x) => { x.wake = { state: "waking", deadline_ms: Date.now() + 180000, other_subnet: false }; res.sent++; };
      for (const s of list) {
        if (s.simulated || s.state === "connected" || !quiere(s.hostname)) continue;
        if (s.canWake) marcar(s); else res.noMac.push(s.hostname);
      }
      for (const o of off) if (quiere(o.hostname)) marcar(o);
      if (!res.sent) {
        return res.noMac.length ? new Error("aula:wakeNoMac: ninguno de esos equipos informó su dirección física")
          : new Error("aula:noWakeTargets: no hay equipos apagados conocidos en este canal");
      }
      window.__emit("aula:students", studentsView());
      return res;
    }),
    ForgetComputer: call("ForgetComputer", (host) => {
      off = off.filter((o) => o.hostname.toLowerCase() !== String(host).toLowerCase());
      window.__emit("aula:students", studentsView());
      return null;
    }),
    PauseScreens: call("PauseScreens", (req) => {
      if ((req.message || "").length > 300) return new Error("aula:invalidPauseMessage: el mensaje de la pausa tiene más de 300 caracteres");
      const objetivo = pausaDe(req.hosts || []);
      const pueden = objetivo.filter((s) => s.canPause && !(present.active && present.sessionId === s.id));
      if (!pueden.length) return new Error("aula:noPauseTargets: ningún alumno conectado de los elegidos puede pausar su pantalla");
      pausaMensaje = req.message || "";
      for (const s of pueden) { s.pause = "sent"; s.pauseUnacked = false; }
      despues(() => { for (const s of pueden) s.pause = "paused"; emitPausas(); });
      emitPausas();
      return { sent: pueden.length, unsupported: objetivo.filter((s) => !s.canPause).map((s) => s.hostname),
        presenting: objetivo.filter((s) => s.canPause && present.active && present.sessionId === s.id).map((s) => s.hostname), failed: [] };
    }),
    ResumeScreens: call("ResumeScreens", (hosts) => {
      const objetivo = pausaDe(hosts || []).filter((s) => s.pause);
      for (const s of objetivo) { s.pause = ""; s.pauseUnacked = false; }
      if (!resumenPausas().paused) pausaMensaje = "";
      emitPausas();
      return { sent: objetivo.length, unsupported: [], presenting: [], failed: [] };
    }),
    ChangePauseMessage: call("ChangePauseMessage", (m) => {
      if ((m || "").length > 300) return new Error("aula:invalidPauseMessage: el mensaje de la pausa tiene más de 300 caracteres");
      pausaMensaje = m || "";
      emitPausas();
      return { sent: resumenPausas().paused, unsupported: [], presenting: [], failed: [] };
    }),
    Pauses: call("Pauses", () => resumenPausas()),
    // Terminar la clase (apps/teacher/classend.go): en lab-terminar hay de todo activo; el
    // resultado llega poco después, con un equipo que no acusa y uno que no pudo quitar sus
    // restricciones. LAB3-PC15 no tiene class.end (versión anterior) y LAB3-PC03 está desconectado.
    ClassEndPreview: call("ClassEndPreview", () => {
      const rico = scene === "lab-terminar";
      const con = list.filter((s) => s.state === "connected");
      return { broadcast: rico, present: "", control: "", paused: resumenPausas().paused + (rico ? 2 : 0),
        apps: list.some((s) => s.appsActive), web: list.some((s) => s.webActive), timer: rico || !!timer.active,
        poll: rico || !!polls.current, power: rico ? 2 : list.filter((s) => s.power).length,
        hands: part.hands.length, messages: part.messages.length, snippets: history.length,
        students: con.filter((s) => !s.simulated).length, simulated: con.filter((s) => s.simulated).length,
        old: con.filter((s) => !s.capabilities.includes("class.end")).length,
        disconnected: list.filter((s) => s.state !== "connected").length,
        collection: rico ? { id: "rc-1", activity: "Ejercicio 4", missing: 2 } : recoList.length ? { id: recoList[0].id, activity: recoList[0].activity, missing: 2 } : null,
        running: !!(fin && fin.running) };
    }),
    FinishClass: call("FinishClass", (req) => {
      if (fin && fin.running) return new Error("aula:endBusy: ya se está terminando la clase");
      const at = new Date().toISOString();
      const rico = scene === "lab-terminar";
      fin = { endId: "fin-1", at, running: true, steps: [], clean: 0, missing: 0,
        students: list.map((s) => ({ hostname: s.hostname, sessionId: s.id, simulated: !!s.simulated, at,
          state: s.state !== "connected" ? "away" : s.capabilities.includes("class.end") ? "waiting" : "partial" })) };
      cuentaFin();
      despues(() => {
        const pasos = [];
        if (req.collect) pasos.push({ kind: "collect", status: "done", count: 2 });
        if (rico) pasos.push({ kind: "broadcast", status: "done", count: 3 }, { kind: "pause", status: "done", count: 2 },
          { kind: "restrictions", status: "done", count: 2 }, { kind: "timer", status: "done", count: 1 },
          { kind: "poll", status: "done", count: 1 }, { kind: "power", status: req.cancelPower ? "done" : "kept", count: 2 });
        fin.steps = pasos;
        for (const s of fin.students) {
          if (s.state !== "waiting") continue;
          if (rico && s.hostname === "lab-ubuntu-02") s.state = "noack";
          else if (rico && s.hostname === "LAB3-PC12") Object.assign(s, { state: "failed", reason: "restrictions" });
          else s.state = "clean";
        }
        fin.running = false;
        cuentaFin();
        window.__emit("aula:class-end", JSON.parse(JSON.stringify(fin)));
      });
      return JSON.parse(JSON.stringify(fin));
    }),
    RetryClassEnd: call("RetryClassEnd", () => {
      if (!fin) return new Error("aula:endBusy: todavía no terminaste la clase");
      for (const s of fin.students) if (s.state === "noack" || s.state === "failed") { s.state = "clean"; delete s.reason; }
      cuentaFin();
      return JSON.parse(JSON.stringify(fin));
    }),
    ClassEnd: call("ClassEnd", () => (fin ? JSON.parse(JSON.stringify(fin)) : { endId: "", at: "0001-01-01T00:00:00Z", running: false, steps: [], students: [], clean: 0, missing: 0 })),
    AllowedApps: call("AllowedApps", () => allowed),
    DefaultAllowedApps: call("DefaultAllowedApps", () => appsEjemplo.slice()),
    SaveAllowedApps: call("SaveAllowedApps", (apps) => {
      if (apps.some((a) => !a.name.trim() || !/\.exe$/i.test(a.path.trim()) || /\\(cmd|powershell)\.exe$/i.test(a.path.trim()))) {
        return new Error("aula:allowedAppInvalid: app 1: la ruta debe ser un .exe instalado");
      }
      allowed = { apps: apps.map((a, i) => ({ id: a.id || "app-" + i, name: a.name.trim(), path: a.path.trim() })), source: "consola", path: rutaApps };
      despues(() => window.__emit("aula:allowed-apps", allowed));
      return allowed;
    }),
    CheckOpenPage: call("CheckOpenPage", (u) => {
      const v = urlValida(u);
      if (!v) return new Error("aula:invalidUrl: la dirección debe empezar con http:// o https://");
      return { url: v, filtered: scene === "lab-abrir" && /youtube\.com/i.test(v) ? ["LAB3-PC12"] : [] };
    }),
    OpenPage: call("OpenPage", (req) => {
      const v = urlValida(req.url);
      if (!v) return new Error("aula:invalidUrl: la dirección debe empezar con http:// o https://");
      const o = nuevaOrden("url", v, req.hosts, "canOpenUrl");
      if (!o.targets.length) return new Error("aula:noOpenTargets: ningún alumno conectado de la selección puede abrirlo");
      window.__emit("aula:opens", opens.slice());
      responderOrden(o);
      return { openId: o.id, sent: o.targets.length, unsupported: o.unsupported, failed: [] };
    }),
    OpenApp: call("OpenApp", (req) => {
      const app = allowed.apps.find((a) => a.id === req.appId);
      if (!app) return new Error("aula:openAppGone: esa app ya no está en la lista de apps permitidas");
      const o = nuevaOrden("app", app.name, req.hosts, "canOpenApp");
      if (!o.targets.length) return new Error("aula:noOpenTargets: ningún alumno conectado de la selección puede abrirlo");
      window.__emit("aula:opens", opens.slice());
      responderOrden(o);
      return { openId: o.id, sent: o.targets.length, unsupported: o.unsupported, failed: [] };
    }),
    OpenOrders: call("OpenOrders", () => opens.slice()),
    Restrictions: call("Restrictions", restrictions),
    Simulation: call("Simulation", simView),
    SimulationStart: call("SimulationStart", simView),
    SimulationStop: call("SimulationStop", () => ({ active: false, students: [], max: 40, capabilities: [] })),
    SimulationAdd: call("SimulationAdd", simView),
    SimulationRemove: call("SimulationRemove", simView),
    SimulationFault: call("SimulationFault", simView),
    SimulationHand: call("SimulationHand", (id, up) => { simHand = up ? "up" : "down"; return simView(); }),
    SimulationMessage: call("SimulationMessage", () => { simSent++; return simView(); }),
    Participation: call("Participation", () => Object.assign({}, part, { arrival: null })),
    LowerHand: call("LowerHand", (w) => { part.hands = part.hands.filter((h) => !mismo(h, w)); despues(() => emitPart()); return null; }),
    LowerAllHands: call("LowerAllHands", () => { const n = part.hands.length; part.hands = []; despues(() => emitPart()); return n; }),
    MarkMessageDone: call("MarkMessageDone", (w, id) => {
      const m = part.messages.find((x) => x.messageId === id);
      if (m) m.status = "done";
      despues(() => emitPart());
      return null;
    }),
    ReplyToStudent: call("ReplyToStudent", (r) => {
      const m = part.messages.find((x) => x.messageId === r.messageId);
      if (m) Object.assign(m, { status: "replied", reply: r.text, repliedAt: new Date().toISOString() });
      despues(() => emitPart());
      return Object.assign({}, deliveries[1], { id: "d-r", label: "Respuesta a " + r.hostname, sent: 1, acked: 0 });
    }),
    SendNoticeTo: call("SendNoticeTo", () => deliveries[1]),
    Timer: call("Timer", () => Object.assign({}, timer)),
    StartTimer: call("StartTimer", (r) => {
      timer = temporizadorDe(r.label, r.durationS, r.durationS * 1000, r.hosts || []);
      timer.receivers = timer.receivers.map((x) => Object.assign(x, { status: "sent" }));
      despues(() => { timer.receivers = receptores(); emitTimer(); }); // los agentes acusan
      return Object.assign({}, timer);
    }),
    ExtendTimer: call("ExtendTimer", (addS) => {
      if (!timer.active) return new Error("aula:timerGone: ya no hay un temporizador en marcha");
      const falta = Math.max(0, timer.remainingMs) + addS * 1000;
      timer = Object.assign({}, timer, { remainingMs: falta, ended: false, maxExtendS: 4 * 3600 - falta / 1000, receivers: receptores() });
      return Object.assign({}, timer);
    }),
    StopTimer: call("StopTimer", () => { timer = sinTemporizador(); return Object.assign({}, timer); }),
    Polls: call("Polls", copiaPolls),
    OpenPoll: call("OpenPoll", (r) => {
      if (!r.question.trim()) return new Error("aula:invalidPoll: encuesta no válida");
      if (polls.current) polls.closed.unshift(Object.assign(polls.current, { open: false, closedAt: new Date().toISOString(), shareable: polls.current.kind !== "open" }));
      const opciones = r.kind === "choice" ? r.options.map((label) => ({ label, count: 0, simulated: 0 }))
        : r.kind === "yesno" ? [{ label: "", count: 0, simulated: 0 }, { label: "", count: 0, simulated: 0 }] : [];
      polls.current = encuestaDe({ kind: r.kind, question: r.question, named: r.named, all: !r.hosts.length, hosts: r.hosts,
        options: opciones, openedAt: new Date().toISOString(), received: 0 });
      // Las respuestas llegan poco después, una por una.
      despues(() => {
        const c = polls.current;
        if (!c || !c.open) return;
        c.received = c.eligible;
        if (c.options.length) { c.options[c.options.length > 1 ? 1 : 0].count++; c.answered++; }
        emitPolls();
      });
      return copiaPolls();
    }),
    ClosePoll: call("ClosePoll", () => {
      if (!polls.current) return new Error("aula:pollGone: ya no hay una encuesta abierta");
      polls.closed.unshift(Object.assign(polls.current, { open: false, closedAt: new Date().toISOString(),
        shareable: polls.current.kind !== "open", missing: [], unsupported: [] }));
      polls.current = null;
      return copiaPolls();
    }),
    SharePollResults: call("SharePollResults", (id) => {
      const c = polls.closed.find((x) => x.pollId === id);
      if (!c) return new Error("aula:pollGone: esa encuesta ya no está en la clase");
      Object.assign(c, { shared: true, shareable: false });
      return copiaPolls();
    }),
    // Mensaje privado (prompt 10.7): la etiqueta no lleva el texto; el alumno lo ve poco después.
    SendPrivateNotice: call("SendPrivateNotice", (id) => {
      const s = list.find((x) => x.id === id) || {};
      const d = { id: "d-p" + Date.now(), kind: "notice", label: "Mensaje privado a " + s.hostname, sent: s.simulated ? 0 : 1, acked: 0,
        failed: 0, noCapability: 0, simSent: s.simulated ? 1 : 0, simAcked: 0, pending: [id], rejected: [], at: new Date().toISOString() };
      deliveries.unshift(d);
      despues(() => {
        Object.assign(d, s.simulated ? { simAcked: 1 } : { acked: 1 }, { pending: [] });
        window.__emit("aula:deliveries", { deliveries: deliveries.slice() });
      });
      return Object.assign({}, d);
    }),
    Manual: call("Manual", () => window.__MANUAL.manual),
    ManualSearch: call("ManualSearch", () => window.__MANUAL.search),
    Resend: call("Resend", () => deliveries[0]),
    InstallerInfo: call("InstallerInfo", () => ({ channel: "1", consoleName: "Prof. Ayala", hostname: "PROF-LAB3", addresses: ["192.168.1.20"],
      port: 47830, mediaPort: 47831, fingerprint: cls.fingerprint, keySource: cls.keySource, version: "0.6.0-dev" })),
    CheckConnection: call("CheckConnection", () => {
      const publica = scene === "red-publica";
      return { channel: "1", done: true, cause: publica ? "noConsole" : "", steps: [
        { id: "announce", status: "ok", addrs: ["192.168.1.20"] },
        { id: "port", status: "ok", port: 47830, addrs: ["192.168.1.20"] },
        { id: "secure", status: "ok" },
        publica ? { id: "network", status: "fail", cause: "noConsole", reason: "publicBlocked", port: 47830, networks: ["Red 2"] }
          : { id: "network", status: "ok", port: 47830 },
      ] };
    }),
    TakeControl: call("TakeControl", (id) => {
      const s = list.find((x) => x.id === id) || {};
      remote = { sessionID: id, hostname: s.hostname, controlID: "c-1", status: "starting", viewing: true };
      despues(() => { remote = Object.assign({}, remote, { status: "active" }); window.__emit("aula:remote", remote); cuadroControl(); });
      return remote;
    }),
    ReleaseControl: call("ReleaseControl", () => {
      despues(() => { remote = Object.assign({}, remote, { status: "ended", reason: "teacher" }); window.__emit("aula:remote", remote); });
      return null;
    }),
    ShowControl: call("ShowControl", (visible) => { remote = Object.assign({}, remote, { viewing: visible }); if (visible) despues(cuadroControl); return null; }),
    SendRemoteInput: call("SendRemoteInput"),
    TakeSnapshot: call("TakeSnapshot", (id) => pedirCaptura(id)),
    TakeSnapshots: call("TakeSnapshots", (ids) => {
      const pueden = ids.map((id) => list.find((x) => x.id === id)).filter((s) => s && s.canSnapshot);
      const omitidos = ids.map((id) => list.find((x) => x.id === id)).filter((s) => s && !s.canSnapshot).map((s) => s.hostname);
      return { snapshots: pueden.map((s) => pedirCaptura(s.id)), skipped: omitidos };
    }),
    SnapshotImage: call("SnapshotImage", (id) => {
      const c = snaps.find((x) => x.id === id);
      return window.__thumb((c ? c.hostname : "") + " - main.py - Visual Studio Code", 200);
    }),
    SaveSnapshots: call("SaveSnapshots", (ids) => snaps.filter((c) => ids.includes(c.id) && c.status === "ready").map((c) => {
      c.path = DOCS + "\\" + c.hostname + (c.username ? " (" + c.username + ")" : "") + (c.simulated ? " (simulado)" : "") + "\\2026-11-05\\10-42-06.jpg";
      return Object.assign({}, c);
    })),
    DiscardSnapshots: call("DiscardSnapshots", (ids) => { snaps = snaps.filter((c) => !ids.includes(c.id)); emitSnaps(); return null; }),
    SnapshotFolder: call("SnapshotFolder", { path: DOCS, exists: true, error: "" }),
    OpenSnapshotFolder: call("OpenSnapshotFolder", null),
    Files: call("Files", () => filesView),
    ChooseFiles: call("ChooseFiles", () => elegidos.map((e) => Object.assign({}, e))),
    SendFiles: call("SendFiles", (r) => (r.files.length === 0 && !r.template ? new Error("aula:pushNoFiles: no hay archivos para enviar") : envioArchivos(r))),
    ResendFiles: call("ResendFiles", () => envioArchivos({ template: "Ejercicio 4" })),
    SaveTemplate: call("SaveTemplate", (name, files) => (filesView.templates.some((t) => t.name === name)
      ? new Error("aula:templateExists: ya hay una plantilla con ese nombre")
      : (filesView.templates.push({ name, size: 0, problem: "", files: files.map((x) => tplArchivo(x.rel, 1000)) }), filesView))),
    OpenTemplates: call("OpenTemplates", null),
    DeleteTemplate: call("DeleteTemplate", (name) => {
      filesView.templates = filesView.templates.filter((t) => t.name !== name);
      return JSON.parse(JSON.stringify(filesView));
    }),
    Collections: call("Collections", collectionsView),
    OpenCollection: call("OpenCollection", (r) => {
      if (!r.activity.trim() || /[\\/:*?"<>|]/.test(r.activity)) return new Error("aula:collectActivity: el nombre de la actividad no sirve como carpeta");
      const pueden = list.filter((s) => s.state === "connected" && s.canCollect);
      const c = { id: "rc-" + (recoList.length + 1), date: "2026-11-06", activity: r.activity.trim(),
        patterns: r.patterns.split(/[ ,;]+/).filter(Boolean), limits: { maxFileSize: 4194304, maxTotal: 16777216, maxFiles: 20 },
        openedAt: new Date().toISOString(), round: 0, collecting: false, dir: win(RECO, "2026-11-06", r.activity.trim()),
        students: pueden.map((s) => recoAlumno(s, "ready")) };
      recoList = [c, ...recoList];
      setTimeout(() => window.__emit("aula:collections", collectionsView()), 20);
      return c;
    }),
    Collect: call("Collect", (id, keys) => {
      const c = recoList.find((x) => x.id === id);
      if (!c) return new Error("aula:collectGone: esa recolección ya no está abierta");
      c.round++;
      for (const s of c.students) if (!keys.length || keys.includes(s.key)) s.status = "queued";
      return c;
    }),
    OpenCollectionFolder: call("OpenCollectionFolder", null),
    SaveCollectionAs: call("SaveCollectionAs", () => win("C:", "Users", "profe", "Desktop", "LAB3-PC07 (alumno07)")),
    SetRestriction: call("SetRestriction", restrictions),
    ClearRestriction: call("ClearRestriction", restrictions),
    ClearAllRestrictions: call("ClearAllRestrictions", restrictions),
  } } };

  // Miniatura falsa: un escritorio con un editor, dibujada en un canvas.
  window.__thumb = (title, hue) => {
    const c = document.createElement("canvas");
    c.width = 320; c.height = 180;
    const g = c.getContext("2d");
    g.fillStyle = "hsl(" + hue + " 45% 35%)"; g.fillRect(0, 0, 320, 180);
    g.fillStyle = "#1e1e1e"; g.fillRect(20, 16, 280, 140);
    g.fillStyle = "#333"; g.fillRect(20, 16, 280, 14);
    g.fillStyle = "#ddd"; g.font = "9px sans-serif"; g.fillText(title, 26, 26);
    const colors = ["#569cd6", "#ce9178", "#9cdcfe", "#dcdcaa", "#6a9955"];
    for (let i = 0; i < 9; i++) {
      g.fillStyle = colors[i % colors.length];
      g.fillRect(30 + (i % 3) * 12, 40 + i * 12, 60 + ((i * 37) % 140), 6);
    }
    g.fillStyle = "#202020"; g.fillRect(0, 168, 320, 12);
    return c.toDataURL("image/jpeg", 0.8).split(",")[1];
  };
})();
