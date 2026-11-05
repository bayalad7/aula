// Interfaz de la app del alumno. Recibe el estado del backend Go por eventos
// de Wails y llama a sus métodos (View, Copy, DismissNotice y los de la
// transmisión del profesor: ReportMedia, BroadcastAnswer, BroadcastCandidate,
// BroadcastFailed; y los del aviso de energía: PowerPostpone, DismissPower).
// Go manda estados; los textos salen de textos.es-MX.js (T) y el título de la
// ventana lo pone esta página (docs/privacidad.md, regla 1).
// Seguridad: todo texto recibido se inserta con textContent; el único HTML es
// el que genera Prism, que escapa el código antes de colorearlo.
"use strict";

(function () {
  const api = window.go.main.App;
  const rt = window.runtime;
  const ev = window.AulaEvents;
  const TX = window.AulaTextos;

  const $ = (id) => document.getElementById(id);

  // --- Textos ------------------------------------------------------------------------------
  // T("clave", {n: 3}) sustituye {n}. Una clave que falta se ve como tal y se
  // registra: una prueba de Go impide que llegue así a una versión.
  function T(key, vars) {
    let s = TX[key];
    if (typeof s !== "string") {
      console.error("texto faltante:", key);
      return key;
    }
    if (vars) for (const [k, v] of Object.entries(vars)) s = s.split("{" + k + "}").join(String(v));
    return s;
  }
  // Plural: claves "x.uno" y "x.varios".
  const P = (key, n, vars) => T(key + (n === 1 ? ".uno" : ".varios"), Object.assign({ n }, vars));
  function aplicarTextos(root) {
    for (const e of root.querySelectorAll("[data-t]")) e.textContent = T(e.dataset.t);
    for (const e of root.querySelectorAll("[data-t-aria-label]")) e.setAttribute("aria-label", T(e.dataset.tAriaLabel));
  }

  // --- Estado: tira de identidad, bandas y título ----------------------------------------------
  // El título junta el estado, lo que el profesor hace con este equipo y las
  // órdenes pendientes, como piden las reglas 1, 6, 12, 20 y 26.
  let status = { state: "searching", channel: "", teacher: "" };
  let restricted = false;
  let pendingPower = "";

  const TI = "titulo."; // + estado, o pendiente.<acción>
  function title() {
    const v = { canal: status.channel, profesor: status.teacher };
    let t = status.viewed ? T("titulo.vista", v) : status.watching ? T("titulo.recibiendo", v) : T(TI + status.state, v);
    if (restricted && status.state === "connected") t += T("titulo.restricciones");
    if (pendingPower && status.state !== "unconfigured") t += T(TI + "pendiente." + pendingPower);
    document.title = t;
    if (rt.WindowSetTitle) rt.WindowSetTitle(t); // también en la barra de tareas
  }

  function renderStatus(s) {
    status = s;
    const v = { canal: s.channel, profesor: s.teacher };
    $("estado").dataset.estado = s.state;
    const ES = "estado."; // + estado de la conexión
    $("estado-texto").textContent = T(ES + s.state, v);
    $("estado-detalles").hidden = !s.detail;
    $("estado-detalle").textContent = s.detail || "";
    $("banda-vista").hidden = !s.viewed;
    $("banda-vista-texto").textContent = s.viewed ? T("banda.vista", v) : "";
    title();
  }

  // --- Código ---------------------------------------------------------------------------------
  const list = $("snippets");
  const empty = $("empty");
  const snippetTpl = $("snippet-template");

  // Identificadores de Monaco/protocolo → gramática de Prism.
  const aliases = {
    html: "markup", xml: "markup", svg: "markup",
    shell: "bash", sh: "bash", powershell: "bash",
    cs: "csharp", js: "javascript", ts: "typescript",
    py: "python", golang: "go", "c++": "cpp",
  };

  function highlight(codeEl, lang, content) {
    const name = aliases[lang] || lang;
    const grammar = window.Prism && window.Prism.languages[name];
    if (grammar) {
      codeEl.innerHTML = window.Prism.highlight(content, grammar, name); // escaped by Prism
      codeEl.className = "language-" + name;
    } else {
      codeEl.textContent = content; // plaintext or unknown language
    }
  }

  function renderBoard(snippets) {
    list.replaceChildren();
    empty.hidden = snippets.length > 0;
    for (const s of snippets) {
      const node = snippetTpl.content.firstElementChild.cloneNode(true);
      const exercise = node.querySelector(".exercise");
      exercise.textContent = s.exercise ? T("codigo.ejercicio", { n: s.exercise }) : "";
      exercise.hidden = !s.exercise;
      const titulo = s.title || T("codigo.sinTitulo");
      node.querySelector(".title").textContent = titulo;
      node.querySelector(".lang").textContent = s.lang;
      node.querySelector("pre").setAttribute("aria-label", T("codigo.region", { titulo }));
      highlight(node.querySelector("code"), s.lang, s.content);

      const button = node.querySelector(".copy");
      button.querySelector("span").textContent = T("codigo.copiar");
      button.addEventListener("click", () => copy(s.id, button, node.querySelector(".codigo-recibido__error")));
      list.appendChild(node);
    }
    if (snippets.length > 0) {
      list.lastElementChild.scrollIntoView({ behavior: "smooth", block: "start" });
    }
  }

  // El texto lo copia Go, del fragmento tal como llegó (sangría y saltos incluidos).
  function copy(id, button, error) {
    error.hidden = true;
    api.Copy(id).then(() => {
      button.querySelector("span").textContent = T("codigo.copiado");
      setTimeout(() => { button.querySelector("span").textContent = T("codigo.copiar"); }, 2000);
    }, (err) => {
      error.textContent = T("codigo.noSeCopio", { motivo: String(err && err.message ? err.message : err) });
      error.hidden = false;
    });
  }

  // --- Avisos del profesor ----------------------------------------------------------------------
  const noticesBox = $("notices");
  const noticeTpl = $("notice-template");
  const NIVEL = { info: ["info", ""], warning: ["advertencia", "toast--advertencia"] };

  function renderNotices(notices) {
    noticesBox.replaceChildren();
    for (const n of notices) {
      const node = noticeTpl.content.firstElementChild.cloneNode(true);
      const [icono, clase] = NIVEL[n.level] || NIVEL.info;
      if (clase) node.classList.add(clase);
      node.querySelector("use").setAttribute("href", "sistema/iconos.svg#" + icono);
      const AV = "aviso."; // + nivel del aviso (info, warning)
      node.querySelector(".toast__titulo").textContent = T(AV + (n.level === "warning" ? "warning" : "info"));
      node.querySelector(".notice-text").textContent = n.text;
      const b = node.querySelector(".dismiss");
      b.textContent = T("aviso.entendido");
      b.addEventListener("click", () => api.DismissNotice(n.id));
      noticesBox.appendChild(node);
    }
  }

  // --- Transmisión de la pantalla del profesor ------------------------------------------------------
  // El backend ya revisó la oferta (video solo de recepción) y mostró esta
  // vista antes de pasarla. Esta conexión nunca envía medios.
  const bcSection = $("broadcast");
  const video = $("bc-video");
  const wait = $("bc-wait");
  let pc = null;
  let pcKey = "";       // broadcastId#attempt of the connection in force
  let pending = [];     // remote candidates that arrived before the offer was set

  function probeMedia() {
    const webrtc = typeof window.RTCPeerConnection === "function";
    let h264 = false;
    try {
      const caps = window.RTCRtpReceiver.getCapabilities("video");
      h264 = caps.codecs.some((c) => c.mimeType.toLowerCase() === "video/h264");
    } catch (e) { h264 = false; }
    api.ReportMedia(webrtc, h264);
  }

  function renderBroadcast(b) {
    document.body.classList.toggle("watching", b.active);
    bcSection.hidden = !b.active;
    $("bc-notice").textContent = b.active ? T("banda.recibiendo", { profesor: b.teacher, canal: b.channel }) : "";
    if (!b.active) closeConnection();
  }

  function closeConnection() {
    if (pc) pc.close();
    pc = null;
    pcKey = "";
    pending = [];
    video.srcObject = null;
    wait.hidden = false;
  }

  async function answerOffer(s) {
    closeConnection();
    const key = s.broadcastId + "#" + s.attempt;
    const conn = new RTCPeerConnection({ iceServers: [] });
    pc = conn;
    pcKey = key;
    conn.ontrack = (e) => {
      video.srcObject = e.streams[0] || new MediaStream([e.track]);
    };
    conn.onicecandidate = (e) => {
      if (pcKey !== key) return;
      const c = e.candidate;
      api.BroadcastCandidate(s.broadcastId, s.attempt, c ? c.candidate : "", (c && c.sdpMid) || "0",
        (c && c.sdpMLineIndex) || 0).catch(() => {});
    };
    conn.onconnectionstatechange = () => {
      if (pcKey === key) wait.hidden = conn.connectionState === "connected";
    };
    try {
      await conn.setRemoteDescription({ type: "offer", sdp: s.sdp });
      for (const c of pending) await conn.addIceCandidate(c).catch(() => {});
      pending = [];
      await conn.setLocalDescription(await conn.createAnswer());
      if (pcKey !== key) return;
      await api.BroadcastAnswer(s.broadcastId, s.attempt, conn.localDescription.sdp);
    } catch (err) {
      if (pcKey === key) api.BroadcastFailed(s.broadcastId, s.attempt).catch(() => {});
    }
  }

  function onSignal(s) {
    if (s.kind === "offer") {
      answerOffer(s);
      return;
    }
    if (s.broadcastId + "#" + s.attempt !== pcKey || !s.candidate) return;
    const c = { candidate: s.candidate, sdpMid: s.sdpMid || "0", sdpMLineIndex: s.sdpMLineIndex };
    if (pc.remoteDescription) pc.addIceCandidate(c).catch(() => {});
    else pending.push(c);
  }

  $("bc-size").addEventListener("click", () => {
    const stage = $("bc-stage");
    const real = stage.dataset.fit !== "real";
    stage.dataset.fit = real ? "real" : "window";
    $("bc-size").textContent = real ? T("banda.ajustar") : T("banda.tamanoReal");
    $("bc-size").setAttribute("aria-pressed", String(real));
  });

  // --- Restricciones de la clase (docs/privacidad.md, reglas 20-22) --------------------------------
  // La política se muestra antes de aplicarse; los detalles listan las primeras
  // y cuántas hay.
  const restr = $("restrictions");
  const restrDetails = $("restr-details");
  const restrToggle = $("restr-toggle");
  const BR = "banda.restricciones."; // + appsDeny, appsAllow, webDeny, webAllow

  function restrList(label, s) {
    if (!s) return null;
    const p = document.createElement("p");
    const more = s.total > s.items.length ? T("banda.lista.mas", { n: s.total - s.items.length }) : "";
    p.textContent = label + (s.mode === "allow" ? T("banda.lista.soloEstas") : "") + ": " + s.items.join(", ") + more;
    return p;
  }

  function renderRestrictions(r) {
    const parts = [];
    if (r.apps) parts.push(T(BR + (r.apps.mode === "allow" ? "appsAllow" : "appsDeny")));
    if (r.web) parts.push(T(BR + (r.web.mode === "allow" ? "webAllow" : "webDeny")));
    restricted = parts.length > 0;
    restr.hidden = !restricted;
    $("restr-text").textContent = restricted ? T("banda.restricciones", { partes: parts.join(T(BR + "separador")) }) : "";
    restrDetails.replaceChildren(...[restrList(T("banda.lista.apps"), r.apps), restrList(T("banda.lista.web"), r.web)].filter(Boolean));
    if (!restricted) {
      restrDetails.hidden = true;
      restrToggle.setAttribute("aria-expanded", "false");
      restrToggle.textContent = T("banda.verDetalles");
    }
    title();
  }

  restrToggle.addEventListener("click", () => {
    const open = restrDetails.hidden;
    restrDetails.hidden = !open;
    restrToggle.setAttribute("aria-expanded", String(open));
    restrToggle.textContent = open ? T("banda.ocultarDetalles") : T("banda.verDetalles");
  });

  let closedTimer = 0;
  function onAppClosed(c) {
    $("app-closed-text").textContent = T("banda.appCerrada", { app: c.app });
    $("app-closed").hidden = false;
    clearTimeout(closedTimer);
    closedTimer = setTimeout(() => { $("app-closed").hidden = true; }, 8000);
  }

  // --- Orden de energía (docs/privacidad.md, reglas 26-28) ------------------------------------------
  // El backend manda los segundos que faltan; la ventana cuenta sola.
  const powerBox = $("power");
  const powerText = $("power-text");
  const powerPostpone = $("power-postpone");
  const powerLeft = $("power-left");
  const powerDismiss = $("power-dismiss");
  let powerView = null;
  let powerEnd = 0;
  let powerTick = 0;
  const EN = "energia."; // + cuenta|ejecutando|fallo . reboot|shutdown|logoff

  function clock(s) {
    const m = Math.floor(s / 60);
    const r = s % 60;
    return m + ":" + (r < 10 ? "0" : "") + r;
  }

  function drawPower() {
    const v = powerView;
    const n = v && v.notice;
    powerBox.hidden = !n;
    if (!n) return;
    const who = { profesor: v.teacher || T("energia.profesor"), canal: v.channel };
    powerPostpone.hidden = n.state !== "countdown";
    powerLeft.hidden = n.state !== "countdown";
    powerDismiss.hidden = n.state !== "failed";
    if (n.state === "countdown") {
      const left = Math.max(0, Math.ceil((powerEnd - Date.now()) / 1000));
      powerText.textContent = T(EN + "cuenta." + n.action, Object.assign({ tiempo: clock(left) }, who));
      powerPostpone.disabled = n.postpones_left === 0;
      powerLeft.textContent = n.postpones_left === 0 ? T("energia.yaNo") : P("energia.quedan", n.postpones_left);
    } else if (n.state === "executing") {
      powerText.textContent = T(EN + "ejecutando." + n.action, who);
    } else {
      powerText.textContent = T(EN + "fallo." + n.action);
    }
  }

  function renderPower(v) {
    powerView = v && v.notice ? v : null;
    clearInterval(powerTick);
    if (powerView && powerView.notice.state === "countdown") {
      powerEnd = Date.now() + powerView.notice.remaining_s * 1000;
      powerTick = setInterval(drawPower, 250);
    }
    pendingPower = powerView && powerView.notice.state !== "failed" ? powerView.notice.action : "";
    drawPower();
    title();
  }

  powerPostpone.addEventListener("click", () => {
    powerPostpone.disabled = true;
    api.PowerPostpone().catch((e) => {
      powerLeft.textContent = String(e);
    });
  });
  powerDismiss.addEventListener("click", () => api.DismissPower());

  // --- Arranque ------------------------------------------------------------------------------------
  aplicarTextos(document);
  // Suscribirse primero y después pedir el estado completo, para no perder nada.
  rt.EventsOn(ev.POWER, renderPower);
  rt.EventsOn(ev.RESTRICTIONS, renderRestrictions);
  rt.EventsOn(ev.APP_CLOSED, onAppClosed);
  rt.EventsOn(ev.BROADCAST, renderBroadcast);
  rt.EventsOn(ev.SIGNAL, onSignal);
  rt.EventsOn(ev.STATUS, renderStatus);
  rt.EventsOn(ev.BOARD, (b) => renderBoard(b.snippets));
  rt.EventsOn(ev.NOTICES, (n) => renderNotices(n.notices));
  api.View().then((v) => {
    renderStatus(v.status);
    renderBoard(v.snippets);
    renderNotices(v.notices);
    renderBroadcast(v.broadcast);
    renderRestrictions(v.restrictions);
    renderPower(v.power);
  });
  probeMedia();
})();
