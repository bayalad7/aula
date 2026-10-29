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
  const CAP_NAMES = { "code.view": "código", "msg.show": "avisos", "screen.thumb": "pantalla" };

  let editor = null;          // Monaco instance
  let students = { connected: 0, canCode: 0, canNotice: 0, students: [] };
  let deliveries = [];
  let lastDeliveryId = "";

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
      li.querySelector(".ack").textContent = st.ackedAt
        ? (st.lastCodeAck ? "✓ código recibido " : "✓ pantalla limpia ") + st.ackedAt : "";
      return li;
    }));
    updateActions();
    renderTiles();
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

  rt.EventsOn(ev.STUDENTS, renderStudents);
  rt.EventsOn(ev.FRAME, onFrame);
  rt.EventsOn(ev.HISTORY, (h) => renderHistory(h.entries));
  rt.EventsOn(ev.DELIVERIES, (d) => renderDeliveries(d.deliveries));
  rt.EventsOn(ev.CAPTURE, onCapture);
  rt.EventsOn(ev.CLASS, (v) => (v.active ? showConsole(v) : showSetup()));
  api.View().then((v) => (v.active ? showConsole(v) : showSetup()));
})();
