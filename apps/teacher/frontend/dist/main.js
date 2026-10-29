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
  const CAP_NAMES = { "code.view": "código", "msg.show": "avisos" };
  const PRESETS = ["AULA_101", "AULA_102", "CURSO_PYTHON_A"];

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
  function showSetup() {
    $("console").hidden = true;
    $("setup").hidden = false;
    api.Setup().then((s) => {
      const recent = s.recentChannels.length ? s.recentChannels : [];
      const all = [...recent, ...PRESETS.filter((p) => !recent.includes(p))];
      const list = $("setup-recent");
      list.replaceChildren(...all.map((c) => Object.assign(document.createElement("option"), { value: c })));
      const chips = $("setup-chips");
      chips.replaceChildren(...all.slice(0, 6).map((c) => {
        const b = document.createElement("button");
        b.type = "button";
        b.className = "chip";
        b.tabIndex = -1; // mouse shortcut; Tab goes straight from channel to name
        b.textContent = c;
        b.addEventListener("click", () => { $("setup-channel").value = c; });
        return b;
      }));
      if (recent[0]) $("setup-channel").value = recent[0];
      if (s.consoleName) $("setup-name").value = s.consoleName;
      $("setup-key").textContent = s.publicKey
        ? "Clave de esta consola: " + s.fingerprint
        : "La clave de esta consola se creará al abrir la primera clase.";
      $("setup-channel").focus();
    });
  }

  $("setup-channel").addEventListener("input", (e) => {
    e.target.value = e.target.value.toUpperCase().replace(/[^A-Z0-9_]/g, "_");
  });

  $("setup-form").addEventListener("submit", (e) => {
    e.preventDefault();
    $("setup-error").textContent = "";
    api.StartClass($("setup-channel").value, $("setup-name").value).then(showConsole, (err) => {
      $("setup-error").textContent = String(err);
    });
  });

  // --- Console --------------------------------------------------------------------------
  function showConsole(view) {
    $("setup").hidden = true;
    $("console").hidden = false;
    $("class-channel").textContent = "Canal " + view.class.channel;
    $("class-name").textContent = view.class.consoleName;
    $("class-fingerprint").textContent = view.class.fingerprint;
    $("class-fingerprint").title = view.class.publicKey;
    $("capture-combo").textContent = view.hotkey.combo;
    const warn = $("hotkey-warning");
    warn.hidden = view.hotkey.active;
    warn.textContent = view.hotkey.error || "";
    renderStudents(view.students);
    renderHistory(view.history);
    renderDeliveries(view.deliveries);
    if (!editor) initEditor();
  }

  $("end-class").addEventListener("click", () => {
    if (!window.confirm("¿Cerrar esta clase? Los alumnos verán que la consola se desconectó.")) return;
    api.EndClass().then(showSetup);
  });

  $("copy-key").addEventListener("click", () => {
    api.CopyPublicKey().then(() => toast("Clave pública copiada. Pégala en la configuración de los alumnos."),
      (err) => toast(String(err), "error"));
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

  rt.EventsOn(ev.STUDENTS, renderStudents);
  rt.EventsOn(ev.HISTORY, (h) => renderHistory(h.entries));
  rt.EventsOn(ev.DELIVERIES, (d) => renderDeliveries(d.deliveries));
  rt.EventsOn(ev.CAPTURE, onCapture);
  api.View().then((v) => (v.active ? showConsole(v) : showSetup()));
})();
