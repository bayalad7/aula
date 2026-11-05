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
  });
  const ALL = ["code.view", "msg.show", "screen.thumb", "screen.broadcast", "app.block", "web.filter", "power", "remote", "screen.snapshot"];
  const student = (o) => Object.assign({
    id: "s-" + o.hostname, username: "", os: "windows", osVersion: "10.0.19045", agentVersion: "0.5.0",
    state: "connected", capabilities: [], power: null, appsActive: false, webActive: false,
    lastCodeAck: "", ackedAt: "", simulated: false, connectedAt: "10:31", lastError: null,
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
      capabilities: ["code.view", "msg.show"], lastCodeAck: "sn-2", ackedAt: "10:39:50",
      lastError: scene === "lab-errores" ? { code: "unsupported_capability", at: "10:43:12" } : null }),
    student({ hostname: "LAB3-PC03", username: "alumno03", state: "disconnected", capabilities: ALL }),
  ];
  const simStudents = [1, 2].map((i) => student({ hostname: "Simulado " + i, username: "Alumno de prueba", osVersion: "simulado",
    agentVersion: "0.6.0-dev+sim", capabilities: ["code.view", "msg.show", "screen.thumb", "screen.snapshot"], simulated: true,
    lastCodeAck: i === 1 ? "sn-3" : "", ackedAt: i === 1 ? "10:42:09" : "" }));
  let list = scene === "simulada-sola" ? simStudents.slice() : scene === "simulada" ? [...sims.slice(0, 1), ...simStudents] : scene === "vacia" || scene.startsWith("setup") ? [] : scene.startsWith("lab") ? lab : [...sims,
    student({ hostname: "SIM-03", username: "simulado", osVersion: "simulado", agentVersion: "0.1.0-sim",
      state: "disconnected", capabilities: ["code.view", "msg.show"] })];
  if (scene === "lab-energia-pendiente") {
    lab[0].power = { action_id: "p1", action: "reboot", force: false, status: "countdown", deadline_ms: now + 24000, postpones_left: 2 };
    lab[1].power = { action_id: "p1", action: "reboot", force: false, status: "postponed", deadline_ms: now + 318000, postpones_left: 1 };
    lab[2].power = { action_id: "p1", action: "reboot", force: false, status: "executing", deadline_ms: 0, postpones_left: 2 };
  }
  const studentsView = () => {
    const c = list.filter((s) => s.state === "connected");
    const real = c.filter((s) => !s.simulated);
    const n = (k) => c.filter((s) => s[k]).length;
    const r = (k) => real.filter((s) => s[k]).length;
    return { students: list, connected: real.length, simulated: c.length - real.length,
      canCode: n("canCode"), canNotice: n("canNotice"), canThumb: n("canThumb"),
      canBroadcast: r("canBroadcast"), canAppBlock: r("canAppBlock"), canWebFilter: r("canWebFilter"), canPower: r("canPower"),
      canSnapshot: n("canSnapshot") };
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
  const deliveries = deliveriesBase.map((d) => Object.assign({ pending: [], rejected: [], simSent: 0, simAcked: 0 }, d));
  const restrictions = scene === "lab-restr-activa"
    ? { apps: { policyId: "r1", mode: "deny", items: ["steam", "discord", "whatsapp"], hosts: [] },
        web: { policyId: "r2", mode: "allow", items: ["python.org", "w3schools.com"], hosts: ["LAB3-PC12"] } }
    : { apps: null, web: null };
  const personal = scene === "clave-personal";
  const cls = { channel: "1", consoleName: "Prof. Ayala", publicKey: "aula-ed25519:Hq0rB7Gm1o5dYH0yXx3l0k3pG2b8m2w7m0K1q0b9JzA=",
    fingerprint: "K7QD-3MX2-9PLA-W4TZ", keySource: personal ? "personal" : "centro", port: 47830, certExpires: "2036-10-29" };
  const bcActive = scene === "lab-transmitiendo";
  // Control remoto (apps/teacher/remote.go, remote.Control): LAB3-PC07 en lab-control*.
  const pc07 = { sessionID: "s-LAB3-PC07", hostname: "LAB3-PC07", controlID: "c-1", viewing: true };
  let remote = scene === "lab-control" ? Object.assign({ status: "active" }, pc07)
    : scene === "lab-control-pausa" ? Object.assign({ status: "paused" }, pc07)
    : { sessionID: "", hostname: "", controlID: "", status: "", viewing: false };
  // Lo que el agente contesta solo: active y una imagen al tomar, ended al soltar.
  const despues = (fn) => setTimeout(fn, 60);
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
  const view = () => ({
    active: !scene.startsWith("setup"), class: cls, students: studentsView(), history, deliveries,
    hotkey: scene === "atajo-ocupado"
      ? { combo: "Ctrl+Alt+Shift+C", active: false, reason: "InUse", error: "" }
      : { combo: "Ctrl+Alt+Shift+C", active: true, reason: "", error: "" },
    broadcast: { active: bcActive, display: bcActive ? 0 : -1, receivers: bcActive ? 2 : 0, eligible: 2, available: true },
    restrictions, remote, snapshots: snaps,
  });


  const simView = () => ({ active: scene.startsWith("simulada"), max: 40, capabilities: ["code.view", "msg.show", "screen.thumb", "screen.snapshot"],
    students: scene.startsWith("simulada") ? [
      { id: "x1", name: "Simulado 1", os: "windows", capabilities: ["code.view", "msg.show", "screen.thumb"], connected: true, fault: "none", delayMs: 0, viewed: false,
        snippets: [{ exercise: "3", title: "Factorial recursivo", lang: "python", lines: 6 }], notices: 1, lastNotice: "Guarden su trabajo" },
      { id: "x2", name: "Simulado 2", os: "linux", capabilities: ["code.view", "msg.show"], connected: true, fault: "noack", delayMs: 0, viewed: false,
        snippets: [], notices: 0, lastNotice: "" }] : [] });
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
    PowerAction: call("PowerAction", { sent: 3, unsupported: ["lab-ubuntu-02"], failed: [] }),
    PowerCancel: call("PowerCancel", { sent: 3, unsupported: [], failed: [] }),
    Restrictions: call("Restrictions", restrictions),
    Simulation: call("Simulation", simView),
    SimulationStart: call("SimulationStart", simView),
    SimulationStop: call("SimulationStop", () => ({ active: false, students: [], max: 40, capabilities: [] })),
    SimulationAdd: call("SimulationAdd", simView),
    SimulationRemove: call("SimulationRemove", simView),
    SimulationFault: call("SimulationFault", simView),
    SendNoticeTo: call("SendNoticeTo", () => deliveries[1]),
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
