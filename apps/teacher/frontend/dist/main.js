// Interfaz de la consola del profesor. El estado vive en Go (App); aquí solo se
// dibuja y se llaman sus métodos. Todo texto recibido entra con textContent.
"use strict";

(function () {
  const api = window.go.main.App;
  const rt = window.runtime;
  const ev = window.AulaEvents;
  const $ = (id) => document.getElementById(id);

  const LANGS = [
    ["python", "Python"], ["java", "Java"], ["c", "C"], ["cpp", "C++"], ["csharp", "C#"],
    ["javascript", "JavaScript"], ["typescript", "TypeScript"], ["go", "Go"], ["sql", "SQL"],
    ["html", "HTML"], ["css", "CSS"], ["shell", "Shell"], ["json", "JSON"], ["rust", "Rust"],
    ["kotlin", "Kotlin"], ["plaintext", "Texto"],
  ];
  const CAP_NAMES = { "code.view": "código", "msg.show": "avisos", "screen.thumb": "pantalla",
    "screen.broadcast": "ve tu pantalla", "app.block": "bloqueo de apps", "web.filter": "filtro web",
    "power": "energía" };

  let editor = null;          // Monaco instance
  let students = { connected: 0, canCode: 0, canNotice: 0, students: [] };
  let deliveries = [];
  let lastDeliveryId = "";
  let restrictionsReady = false; // set once the restrictions code below has run
  let powerReady = false;        // the same for the power code

  // --- Monaco -----------------------------------------------------------------------
  function initEditor() {
    window.MonacoEnvironment = { getWorkerUrl: () => "vendor/monaco/vs/base/worker/workerMain.js" };
    window.require.config({ paths: { vs: "vendor/monaco/vs" } });
    window.require(["vs/editor/editor.main"], () => {
      const dark = window.matchMedia("(prefers-color-scheme: dark)").matches;
      editor = window.monaco.editor.create($("editor"), {
        value: "",
        language: $("code-lang").value,
        theme: dark ? "vs-dark" : "vs",
        automaticLayout: true,
        minimap: { enabled: false },
        fontSize: 15,
        tabSize: 4,
        insertSpaces: true,
        renderWhitespace: "boundary",
        scrollBeyondLastLine: false,
      });
      editor.onDidChangeModelContent(updateActions);
      updateActions();
    });
  }

  function setLang(lang) {
    $("code-lang").value = lang;
    if (editor) window.monaco.editor.setModelLanguage(editor.getModel(), lang);
  }

  // --- Setup screen -------------------------------------------------------------------
  // Normally invisible: the console opens on its default channel by itself
  // (like LanSchool). It shows while starting and when that start failed.
  function showSetup() {
    $("console").hidden = true;
    $("setup").hidden = false;
    api.Setup().then((s) => {
      $("setup-intro").textContent = s.startError
        ? "No se pudo abrir el canal " + s.defaultChannel + ". Corrige el problema y vuelve a intentarlo."
        : "Abriendo el canal " + s.defaultChannel + "…";
      $("setup-error").textContent = s.startError || "";
      $("setup-channel").value = s.defaultChannel;
      $("setup-name").value = s.consoleName || "";
      $("setup-chips").replaceChildren(...s.recentChannels.slice(0, 6).map((c) => {
        const b = document.createElement("button");
        b.type = "button";
        b.className = "chip";
        b.tabIndex = -1; // mouse shortcut; Tab goes straight from channel to name
        b.textContent = "Canal " + c;
        b.addEventListener("click", () => { $("setup-channel").value = c; });
        return b;
      }));
      $("setup-key").textContent = s.publicKey
        ? (s.keySource === "centro" ? "Clave del centro: " : "Clave personal (sin clave del centro): ") + s.fingerprint
        : "Este equipo aún no tiene clave del centro.";
      $("setup-version").textContent = "Aula Consola " + s.version;
    });
  }

  function open(channel, name, onError) {
    closeScreens(); // a new class starts with the view closed
    api.StartClass(String(channel), name).then(showConsole, onError);
  }

  $("setup-form").addEventListener("submit", (e) => {
    e.preventDefault();
    $("setup-error").textContent = "";
    open($("setup-channel").value, $("setup-name").value, (err) => { $("setup-error").textContent = String(err); });
  });

  // --- Console --------------------------------------------------------------------------
  let current = { channel: "", name: "" };

  function showConsole(view) {
    $("setup").hidden = true;
    $("console").hidden = false;
    current = { channel: view.class.channel, name: view.class.consoleName };
    $("channel-input").value = view.class.channel;
    $("name-input").value = view.class.consoleName;
    $("class-fingerprint").textContent = view.class.fingerprint;
    $("class-fingerprint").title = view.class.publicKey;
    const personal = view.class.keySource !== "centro";
    $("key-label").textContent = personal ? "Clave personal:" : "Clave del centro:";
    $("key-warning").hidden = !personal;
    $("capture-combo").textContent = view.hotkey.combo;
    const warn = $("hotkey-warning");
    warn.hidden = view.hotkey.active;
    warn.textContent = view.hotkey.error || "";
    renderStudents(view.students);
    renderBroadcast(view.broadcast);
    renderRestrictions(view.restrictions);
    loadDisplays();
    renderHistory(view.history);
    renderDeliveries(view.deliveries);
    if (!editor) initEditor();
  }

  // Change channel: you now see (and teach) the students of that channel.
  $("channel-form").addEventListener("submit", (e) => {
    e.preventDefault();
    const ch = $("channel-input").value;
    if (String(ch) === current.channel) return;
    open(ch, current.name, (err) => {
      toast(String(err), "error");
      $("channel-input").value = current.channel;
    });
  });

  // Rename: students see the new name when they reconnect (a few seconds).
  $("name-form").addEventListener("submit", (e) => {
    e.preventDefault();
    const name = $("name-input").value.trim();
    if (name === current.name) return;
    open(current.channel, name, (err) => {
      toast(String(err), "error");
      $("name-input").value = current.name;
    });
  });

  // Students
  function renderStudents(s) {
    students = s;
    $("students-count").textContent = s.connected + " conectado" + (s.connected === 1 ? "" : "s");
    $("students-empty").hidden = s.students.length > 0;
    const tpl = $("student-template").content.firstElementChild;
    $("students-list").replaceChildren(...s.students.map((st) => {
      const li = tpl.cloneNode(true);
      li.dataset.state = st.state;
      li.querySelector(".host").textContent = st.hostname;
      li.querySelector(".state").textContent = st.state === "connected" ? "conectado" : "desconectado";
      li.querySelector(".user").textContent = st.username || "(sin usuario)";
      li.querySelector(".os").textContent = [st.os, st.osVersion, "agente " + st.agentVersion].filter(Boolean).join(" · ");
      li.querySelector(".caps").replaceChildren(...st.capabilities.map((c) => {
        const span = document.createElement("span");
        span.className = "cap";
        span.textContent = CAP_NAMES[c] || c;
        return span;
      }));
      if (!st.canCode) li.querySelector(".caps").append(Object.assign(document.createElement("span"),
        { className: "cap missing", textContent: "no recibe código" }));
      if (st.appsActive) li.querySelector(".caps").append(Object.assign(document.createElement("span"),
        { className: "cap locked", textContent: "🔒 apps" }));
      if (st.webActive) li.querySelector(".caps").append(Object.assign(document.createElement("span"),
        { className: "cap locked", textContent: "🔒 sitios" }));
      const pick = li.querySelector(".pick");
      pick.checked = marked.has(st.hostname);
      pick.disabled = !st.canAppBlock && !st.canWebFilter && !st.canPower;
      pick.addEventListener("change", () => {
        if (pick.checked) marked.add(st.hostname); else marked.delete(st.hostname);
        updateMarked();
      });
      const ps = li.querySelector(".power-state");
      ps.hidden = !st.power;
      if (st.power) ps.dataset.power = JSON.stringify(st.power);
      li.querySelector(".ack").textContent = st.ackedAt
        ? (st.lastCodeAck ? "✓ código recibido " : "✓ pantalla limpia ") + st.ackedAt : "";
      return li;
    }));
    updateActions();
    renderTiles();
    if (typeof renderRestrictions === "function" && restrictionsReady) renderRestrictions(restrictions);
    if (powerReady) renderPower();
  }

  // Actions are enabled only when someone can receive them.
  function updateActions() {
    const hasCode = editor && editor.getValue().trim() !== "";
    let reason = "";
    if (students.canCode === 0) reason = students.connected === 0 ? "No hay alumnos conectados." : "Ningún alumno conectado recibe código.";
    else if (!hasCode) reason = "Escribe o captura código para emitir.";
    $("emit").disabled = $("append").disabled = students.canCode === 0 || !hasCode;
    $("clear").disabled = students.canCode === 0;
    $("code-reason").textContent = reason;
    $("notice-send").disabled = students.canNotice === 0 || $("notice-text").value.trim() === "";
  }
  $("notice-text").addEventListener("input", updateActions);

  // Emit
  function emit(mode) {
    api.Emit({ title: $("code-title").value, lang: $("code-lang").value, content: editor.getValue(), mode })
      .then(trackDelivery, (err) => toast(String(err), "error"));
  }
  $("emit").addEventListener("click", () => emit("replace"));
  $("append").addEventListener("click", () => emit("append"));
  $("clear").addEventListener("click", () => {
    if (!window.confirm("¿Borrar el código de todas las pantallas?")) return;
    api.ClearScreens().then(trackDelivery, (err) => toast(String(err), "error"));
  });
  $("notice-send").addEventListener("click", () => {
    api.SendNotice($("notice-text").value, $("notice-level").value).then((d) => {
      $("notice-text").value = "";
      updateActions();
      trackDelivery(d);
    }, (err) => toast(String(err), "error"));
  });

  // Deliveries: "Enviado a 2 · confirmados 2/2 ✓"
  function deliveryText(d) {
    let t = d.label + " — enviado a " + d.sent + " · confirmados " + d.acked + "/" + d.sent;
    if (d.sent > 0 && d.acked === d.sent) t += " ✓";
    if (d.noCapability) t += " · " + d.noCapability + " no lo soporta" + (d.noCapability === 1 ? "" : "n");
    if (d.failed) t += " · " + d.failed + " fallaron";
    return t;
  }

  function trackDelivery(d) {
    lastDeliveryId = d.id;
    renderDeliveries([d, ...deliveries.filter((x) => x.id !== d.id)]);
  }

  function renderDeliveries(list) {
    deliveries = list;
    const last = list.find((d) => d.id === lastDeliveryId);
    const p = $("last-delivery");
    p.textContent = last ? deliveryText(last) : "";
    p.dataset.complete = last && last.sent > 0 && last.acked === last.sent ? "yes" : "no";
    $("delivery-list").replaceChildren(...list.slice(0, 8).map((d) => {
      const li = document.createElement("li");
      li.textContent = deliveryText(d);
      li.dataset.complete = d.sent > 0 && d.acked === d.sent ? "yes" : "no";
      return li;
    }));
  }

  // History
  function renderHistory(entries) {
    $("history-empty").hidden = entries.length > 0;
    const tpl = $("history-template").content.firstElementChild;
    $("history-list").replaceChildren(...entries.slice().reverse().map((e) => {
      const li = tpl.cloneNode(true);
      li.querySelector(".h-label").textContent = "Ejercicio " + e.number + (e.title ? " · " + e.title : "");
      const lines = e.content.replace(/\n$/, "").split("\n").length;
      li.querySelector(".h-meta").textContent = e.lang + " · " + lines + " líneas" + (e.times > 1 ? " · emitido " + e.times + " veces" : "");
      li.querySelector(".h-reemit").addEventListener("click", () => {
        api.Reemit(e.snippetId).then(trackDelivery, (err) => toast(String(err), "error"));
      });
      li.addEventListener("dblclick", () => { // load it back into the editor
        $("code-title").value = e.title;
        setLang(e.lang);
        if (editor) editor.setValue(e.content);
      });
      li.title = "Doble clic para cargarlo en el editor";
      return li;
    }));
  }

  // Capture (hotkey or floating button): load, never emit.
  function onCapture(c) {
    if (c.error) {
      toast(c.error, "error");
      return;
    }
    const d = c.draft;
    if (editor) editor.setValue(d.content);
    setLang(d.lang);
    $("code-title").value = "";
    $("code-title").focus();
    toast("Capturado del portapapeles (" + d.lines + " líneas, " + d.lang +
      "). Revisa el lenguaje y pulsa Emitir.");
  }
  $("capture").addEventListener("click", () => {
    api.CaptureClipboard().catch(() => {}); // the result also arrives as an event
  });

  // Toasts
  let toastTimer = 0;
  function toast(text, kind) {
    const t = $("toast");
    t.textContent = text;
    t.dataset.kind = kind || "info";
    t.hidden = false;
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => { t.hidden = true; }, 5000);
  }

  // --- Boot ---------------------------------------------------------------------------
  const langSelect = $("code-lang");
  langSelect.replaceChildren(...LANGS.map(([id, name]) => Object.assign(document.createElement("option"), { value: id, textContent: name })));
  langSelect.addEventListener("change", () => setLang(langSelect.value));

  // --- Screens view -------------------------------------------------------------------
  // Thumbnails stream only while this view is open: OpenScreens sends
  // screen.thumb.start and CloseScreens screen.thumb.stop. Frames only live
  // here, in the <img> elements.
  let screensOpen = false;
  let detailId = "";
  const tiles = new Map(); // session ID → <li>

  function openScreens() {
    api.OpenScreens().then(() => {
      screensOpen = true;
      $("screens").hidden = false;
      renderTiles();
    }, (err) => toast(String(err), "error"));
  }

  function closeScreens() {
    if (!screensOpen) return;
    screensOpen = false;
    detailId = "";
    $("screens").hidden = true;
    $("screen-detail").hidden = true;
    $("detail-img").removeAttribute("src");
    tiles.clear();
    $("screens-grid").replaceChildren();
    api.CloseScreens();
  }

  function renderTiles() {
    if (!screensOpen) return;
    const list = students.students.filter((s) => s.state === "connected" && s.canThumb);
    const tpl = $("tile-template").content.firstElementChild;
    const keep = new Set(list.map((s) => s.id));
    for (const id of tiles.keys()) if (!keep.has(id)) tiles.delete(id);
    $("screens-grid").replaceChildren(...list.map((s) => {
      let li = tiles.get(s.id);
      if (!li) {
        li = tpl.cloneNode(true);
        li.querySelector(".tile-view").addEventListener("click", () => showDetail(s.id));
        tiles.set(s.id, li);
      }
      li.querySelector(".tile-name").textContent = s.hostname + (s.username ? " · " + s.username : "");
      li.querySelector("img").alt = "Pantalla de " + s.hostname;
      return li;
    }));
    $("screens-count").textContent = list.length + " de " + students.connected;
    $("screens-empty").hidden = list.length > 0;
    if (detailId && !keep.has(detailId)) hideDetail();
  }

  function showDetail(id) {
    const s = students.students.find((x) => x.id === id);
    const li = tiles.get(id);
    if (!s || !li) return;
    detailId = id;
    $("detail-host").textContent = s.hostname + (s.username ? " · " + s.username : "");
    const img = li.querySelector("img");
    if (img.src) $("detail-img").src = img.src; // until the first large frame arrives
    $("screen-detail").hidden = false;
    api.ViewScreen(id);
  }

  function hideDetail() {
    if (!detailId) return;
    detailId = "";
    $("screen-detail").hidden = true;
    $("detail-img").removeAttribute("src");
    api.ViewScreen("");
  }

  function onFrame(t) {
    if (!screensOpen) return;
    const src = "data:image/jpeg;base64," + t.jpeg;
    if (t.sessionId === detailId) $("detail-img").src = src;
    const li = tiles.get(t.sessionId);
    if (li) {
      li.querySelector("img").src = src;
      li.querySelector(".tile-wait").hidden = true;
    }
  }

  $("screens-open").addEventListener("click", openScreens);
  $("screens-close").addEventListener("click", closeScreens);
  $("detail-back").addEventListener("click", hideDetail);
  document.addEventListener("keydown", (e) => {
    if (e.key !== "Escape" || !screensOpen) return;
    if (detailId) hideDetail(); else closeScreens();
  });

  // --- Broadcast of the teacher's screen ------------------------------------------------
  // The students see it in a window with a fixed band; here the band is red
  // and says how many receive it. The console window is excluded from the
  // capture by the backend.
  let bc = { active: false, display: -1, receivers: 0, eligible: 0, available: false };

  function loadDisplays() {
    api.Displays().then((ds) => {
      const sel = $("bc-display");
      const keep = sel.value;
      sel.replaceChildren(...ds.map((d) => Object.assign(document.createElement("option"),
        { value: String(d.index), textContent: d.label })));
      if (ds.some((d) => String(d.index) === keep)) sel.value = keep;
      renderBroadcast(bc);
    });
  }

  function renderBroadcast(s) {
    if (bc.active && !s.active) loadDisplays(); // monitors may have changed meanwhile
    bc = s;
    const sel = $("bc-display");
    const btn = $("bc-toggle");
    btn.textContent = s.active ? "Detener transmisión" : "Transmitir mi pantalla";
    btn.classList.toggle("primary", !s.active);
    btn.disabled = !s.available || sel.options.length === 0;
    btn.title = s.available ? "" : "Este equipo no puede capturar sus monitores";
    sel.disabled = s.active;
    if (s.active && s.display >= 0) sel.value = String(s.display);
    $("bc-band").hidden = !s.active;
    if (s.active) {
      const n = sel.selectedIndex >= 0 ? sel.options[sel.selectedIndex].textContent.split(" · ")[0] : "";
      $("bc-text").textContent = "Transmitiendo tu pantalla (" + n.toLowerCase() + ") a " + s.receivers +
        " de " + s.eligible + " alumno" + (s.eligible === 1 ? "" : "s");
    }
  }

  function toggleBroadcast() {
    if (bc.active) {
      api.StopBroadcast().then(renderBroadcast);
      return;
    }
    $("bc-toggle").disabled = true;
    api.StartBroadcast(Number($("bc-display").value)).then(renderBroadcast, (err) => {
      toast(String(err), "error");
      renderBroadcast(bc);
    });
  }
  $("bc-toggle").addEventListener("click", toggleBroadcast);
  $("bc-stop").addEventListener("click", () => api.StopBroadcast().then(renderBroadcast));

  // --- Class restrictions -------------------------------------------------------------
  // Students are marked by hostname, which survives reconnections. The
  // backend decides who gets what and resends it to whoever reconnects.
  const TEMPLATES = {
    games: { kind: "apps", mode: "deny", items: ["steam", "epicgameslauncher", "riotclientservices", "leagueclient",
      "robloxplayerbeta", "minecraftlauncher", "discord", "whatsapp", "telegram", "spotify"] },
    social: { kind: "web", mode: "deny", items: ["youtube.com", "tiktok.com", "instagram.com", "facebook.com", "x.com",
      "twitter.com", "twitch.tv", "netflix.com", "roblox.com", "discord.com", "web.whatsapp.com"] },
    docs: { kind: "web", mode: "allow", items: ["python.org", "developer.mozilla.org", "w3schools.com", "cppreference.com",
      "docs.oracle.com", "go.dev", "learn.microsoft.com", "stackoverflow.com", "github.com", "githubusercontent.com",
      "fonts.googleapis.com", "fonts.gstatic.com", "cdnjs.cloudflare.com", "cdn.jsdelivr.net"] },
  };
  const marked = new Set();
  let restrictions = { apps: null, web: null };

  function kindPanel(kind) { return document.querySelector('.restr-kind[data-kind="' + kind + '"]'); }

  function openRestrictions() {
    $("restr").hidden = false;
    renderRestrictions(restrictions);
  }

  function renderRestrictions(v) {
    restrictions = v;
    $("restr-open").textContent = v.apps || v.web ? "Restricciones 🔒" : "Restricciones";
    for (const kind of ["apps", "web"]) {
      const panel = kindPanel(kind);
      const rule = v[kind];
      const canKey = kind === "apps" ? "canAppBlock" : "canWebFilter";
      const activeKey = kind === "apps" ? "appsActive" : "webActive";
      let state = "Sin restricción.";
      if (rule) {
        const n = students.students.filter((s) => s.state === "connected" && s[activeKey]).length;
        const scope = rule.hosts.length ? rule.hosts.length + " alumno(s) marcados" : "todo el canal";
        state = (rule.mode === "allow" ? "Solo se permiten " : "Bloqueados: ") + rule.items.length +
          " · para " + scope + " · activa en " + n + " de " + (students[canKey] || 0) + " alumnos";
      }
      panel.querySelector(".r-state").textContent = state;
      panel.querySelector(".r-clear").disabled = !rule;
    }
    const can = Math.max(students.canAppBlock || 0, students.canWebFilter || 0);
    $("restr-support").textContent = can + " de " + students.connected +
      " alumnos conectados pueden recibirlas (agente como servicio en Windows).";
  }

  function applyRestriction(kind) {
    const panel = kindPanel(kind);
    const items = panel.querySelector(".r-items").value.split(/\r?\n/).map((s) => s.trim()).filter(Boolean);
    const target = document.querySelector('input[name="restr-target"]:checked').value;
    const hosts = target === "marked" ? [...marked] : [];
    if (target === "marked" && hosts.length === 0) {
      toast("Marca al menos un alumno o elige «Todo el canal».", "error");
      return;
    }
    api.SetRestriction({ kind, mode: panel.querySelector(".r-mode").value, items, hosts })
      .then((v) => { renderRestrictions(v); toast("Restricción aplicada."); }, (err) => toast(String(err), "error"));
  }

  for (const kind of ["apps", "web"]) {
    const panel = kindPanel(kind);
    panel.querySelector(".r-apply").addEventListener("click", () => applyRestriction(kind));
    panel.querySelector(".r-clear").addEventListener("click", () => api.ClearRestriction(kind).then(renderRestrictions));
  }
  for (const b of document.querySelectorAll(".r-tpl")) {
    b.addEventListener("click", () => {
      const t = TEMPLATES[b.dataset.tpl];
      const panel = kindPanel(t.kind);
      panel.querySelector(".r-mode").value = t.mode;
      panel.querySelector(".r-items").value = t.items.join("\n");
    });
  }
  $("restr-open").addEventListener("click", openRestrictions);
  $("restr-close").addEventListener("click", () => { $("restr").hidden = true; });
  $("restr-clear-all").addEventListener("click", () => {
    if (!window.confirm("¿Quitar todas las restricciones de la clase?")) return;
    api.ClearAllRestrictions().then(renderRestrictions);
  });
  function updateMarked() {
    $("restr-marked").textContent = "Alumnos marcados (" + marked.size + ")";
    $("power-marked").textContent = "Alumnos marcados (" + marked.size + ")";
  }

  // --- Power (docs/protocol.md §5.15, docs/privacidad.md rules 26-31) -----------------
  // Every order goes through an explicit confirmation. The cards follow each
  // agent's power.ack; the countdown is drawn from its deadline.
  const POWER = {
    shutdown: { verb: "Apagar", noun: "apagado", doing: "apagándose" },
    reboot: { verb: "Reiniciar", noun: "reinicio", doing: "reiniciándose" },
    logoff: { verb: "Cerrar sesión de", noun: "cierre de sesión", doing: "cerrando sesión" },
  };
  let powerAsk = null;

  function mmss(ms) {
    const s = Math.max(0, Math.ceil(ms / 1000));
    return Math.floor(s / 60) + ":" + String(s % 60).padStart(2, "0");
  }

  function powerText(p) {
    const n = POWER[p.action] || POWER.shutdown;
    const forced = p.force ? " (forzado)" : "";
    switch (p.status) {
      case "sent": return "⏻ " + n.noun + " enviado…" + forced;
      case "countdown":
      case "postponed": {
        const used = 2 - p.postpones_left;
        return "⏻ " + n.noun + " en " + mmss(p.deadline_ms - Date.now()) + (used ? " · pospuesto (" + used + "/2)" : "");
      }
      case "executing": return "⏻ " + n.doing + "…" + forced;
      case "cancelled": return "⏻ " + n.noun + " cancelado";
      default: return "⏻ no se pudo: " + n.noun;
    }
  }

  function pendingPower() {
    return students.students.filter((s) => s.state === "connected" && s.power &&
      ["sent", "countdown", "postponed"].includes(s.power.status));
  }

  function renderPower() {
    for (const el of document.querySelectorAll(".power-state[data-power]")) {
      el.textContent = powerText(JSON.parse(el.dataset.power));
    }
    const pending = pendingPower();
    $("power-band").hidden = pending.length === 0;
    $("power-band-text").textContent = pending.length === 0 ? "" :
      "⏻ Órdenes de energía pendientes en " + pending.length + (pending.length === 1 ? " equipo" : " equipos");
    $("power-cancel").disabled = pending.length === 0;
    const can = students.canPower || 0;
    $("power-support").textContent = can + " de " + students.connected + " alumnos conectados pueden recibirlas (Windows).";
    for (const b of document.querySelectorAll(".power-do")) b.disabled = can === 0;
  }
  setInterval(() => { if (pendingPower().length) renderPower(); }, 1000);

  function powerTargets() {
    const target = document.querySelector('input[name="power-target"]:checked').value;
    const hosts = target === "marked" ? [...marked] : [];
    const want = new Set(hosts.map((h) => h.toLowerCase()));
    const list = students.students.filter((s) => s.state === "connected" && (!hosts.length || want.has(s.hostname.toLowerCase())));
    return { target, hosts, able: list.filter((s) => s.canPower).length, other: list.filter((s) => !s.canPower).length };
  }

  function askPower(action) {
    const t = powerTargets();
    if (t.target === "marked" && t.hosts.length === 0) {
      toast("Marca al menos un alumno o elige «Todo el canal».", "error");
      return;
    }
    if (t.able === 0) {
      toast("Ninguno de esos equipos puede recibir órdenes de energía.", "error");
      return;
    }
    const force = $("power-force").checked;
    const delay = Number($("power-delay").value);
    const n = POWER[action];
    const what = t.able + (t.able === 1 ? " equipo" : " equipos");
    const scope = t.target === "marked" ? "marcados" : "del canal " + $("channel-input").value;
    $("power-confirm-title").textContent = "¿" + n.verb + " " + what + " " + scope + "?";
    $("power-confirm-text").textContent = (force
      ? "Se ejecutará de inmediato, sin cuenta regresiva."
      : "Los alumnos verán un aviso con una cuenta de " + mmss(delay * 1000) + " y podrán posponerla hasta 2 veces.") +
      (t.other ? " " + t.other + " equipo(s) no lo soportan y no la recibirán." : "");
    $("power-confirm-force").hidden = !force;
    $("power-confirm-yes").textContent = n.verb + " " + what;
    powerAsk = { action, hosts: t.hosts, delayS: delay, force };
    $("power-confirm").showModal();
    $("power-confirm-no").focus();
  }

  $("power-confirm-no").addEventListener("click", () => $("power-confirm").close());
  $("power-confirm").addEventListener("close", () => { powerAsk = null; });
  $("power-confirm-yes").addEventListener("click", () => {
    const req = powerAsk;
    $("power-confirm").close();
    if (!req) return;
    api.PowerAction(req).then((r) => {
      let msg = "Orden enviada a " + r.sent + (r.sent === 1 ? " equipo." : " equipos.");
      if (r.unsupported.length) msg += " No la soportan: " + r.unsupported.join(", ") + ".";
      if (r.failed.length) msg += " No llegó a: " + r.failed.join(", ") + ".";
      toast(msg, r.failed.length ? "error" : undefined);
    }, (err) => toast(String(err), "error"));
  });
  for (const b of document.querySelectorAll(".power-do")) {
    b.addEventListener("click", () => askPower(b.dataset.action));
  }
  function cancelPower(hosts) {
    api.PowerCancel(hosts).then((r) => toast("Cancelación enviada a " + r.sent + " equipo(s)."));
  }
  $("power-cancel").addEventListener("click", () => {
    const t = powerTargets();
    cancelPower(t.target === "marked" ? t.hosts : []);
  });
  $("power-band-cancel").addEventListener("click", () => cancelPower([]));
  $("power-open").addEventListener("click", () => { $("power").hidden = false; renderPower(); });
  $("power-close").addEventListener("click", () => { $("power").hidden = true; });
  powerReady = true;

  restrictionsReady = true;
  rt.EventsOn(ev.RESTRICTIONS, renderRestrictions);
  rt.EventsOn(ev.BROADCAST, renderBroadcast);
  rt.EventsOn(ev.STUDENTS, renderStudents);
  rt.EventsOn(ev.FRAME, onFrame);
  rt.EventsOn(ev.HISTORY, (h) => renderHistory(h.entries));
  rt.EventsOn(ev.DELIVERIES, (d) => renderDeliveries(d.deliveries));
  rt.EventsOn(ev.CAPTURE, onCapture);
  rt.EventsOn(ev.CLASS, (v) => (v.active ? showConsole(v) : showSetup()));
  api.View().then((v) => (v.active ? showConsole(v) : showSetup()));
})();
