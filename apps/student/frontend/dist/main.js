// Interfaz del cliente del alumno. Recibe el estado del backend Go por eventos
// de Wails y llama a sus métodos (View, Copy, DismissNotice y los de la
// transmisión del profesor: ReportMedia, BroadcastAnswer, BroadcastCandidate,
// BroadcastFailed).
// Seguridad: todo texto recibido se inserta con textContent; el único HTML es
// el que genera Prism, que escapa el código antes de colorearlo.
"use strict";

(function () {
  const api = window.go.main.App;
  const rt = window.runtime;
  const ev = window.AulaEvents;

  const $ = (id) => document.getElementById(id);
  const banner = $("banner");
  const bannerText = $("banner-text");
  const bannerDetail = $("banner-detail");
  const list = $("snippets");
  const empty = $("empty");
  const noticesBox = $("notices");
  const snippetTpl = $("snippet-template");
  const noticeTpl = $("notice-template");

  // Identificadores de Monaco/protocolo → gramática de Prism.
  const aliases = {
    html: "markup", xml: "markup", svg: "markup",
    shell: "bash", sh: "bash", powershell: "bash",
    cs: "csharp", js: "javascript", ts: "typescript",
    py: "python", golang: "go", "c++": "cpp",
  };

  function renderStatus(s) {
    banner.dataset.state = s.state;
    banner.dataset.viewed = s.viewed ? "true" : "false";
    bannerText.textContent = s.banner;
    bannerDetail.textContent = s.detail || "";
  }

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
      exercise.textContent = s.exercise ? "Ejercicio " + s.exercise : "";
      exercise.hidden = !s.exercise;
      node.querySelector(".title").textContent = s.title || "Código del profesor";
      node.querySelector(".lang").textContent = s.lang;
      highlight(node.querySelector("code"), s.lang, s.content);

      const button = node.querySelector(".copy");
      button.addEventListener("click", () => copy(s.id, button));
      list.appendChild(node);
    }
    if (snippets.length > 0) {
      list.lastElementChild.scrollIntoView({ behavior: "smooth", block: "start" });
    }
  }

  function copy(id, button) {
    api.Copy(id).then(
      () => flash(button, "Copiado ✓", "ok"),
      (err) => flash(button, "No se pudo copiar", "error", String(err)),
    );
  }

  function flash(button, text, kind, title) {
    const original = "Copiar al portapapeles";
    button.textContent = text;
    button.dataset.kind = kind;
    button.title = title || "";
    setTimeout(() => {
      button.textContent = original;
      delete button.dataset.kind;
      button.title = "";
    }, 2000);
  }

  function renderNotices(notices) {
    noticesBox.replaceChildren();
    for (const n of notices) {
      const node = noticeTpl.content.firstElementChild.cloneNode(true);
      node.dataset.level = n.level;
      node.querySelector(".notice-text").textContent = n.text;
      node.querySelector(".dismiss").addEventListener("click", () => api.DismissNotice(n.id));
      noticesBox.appendChild(node);
    }
  }

  // --- Broadcast of the teacher's screen ------------------------------------------
  // The backend has already checked the offer (receive-only video) and shown
  // this view before handing it over. This connection never sends media.
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
    $("bc-notice").textContent = b.notice || "";
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
    $("bc-size").textContent = real ? "Ajustar a la ventana" : "Tamaño real";
    $("bc-size").setAttribute("aria-pressed", String(real));
  });

  // Subscribe first, then fetch the full state, so nothing is missed.
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
  });
  probeMedia();
})();
