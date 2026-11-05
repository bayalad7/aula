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
    for (const e of root.querySelectorAll("[data-t-title]")) e.title = T(e.dataset.tTitle);
  }

  // --- Estado: tira de identidad, bandas y título ----------------------------------------------
  // El título junta el estado, lo que el profesor hace con este equipo y las
  // órdenes pendientes, como piden las reglas 1, 6, 12, 20 y 26.
  let status = { state: "searching", channel: "", teacher: "" };
  const ES = "estado."; // + estado de la conexión
  let restricted = false;
  let pendingPower = "";

  const TI = "titulo."; // + estado, o pendiente.<acción>
  function title() {
    const v = { canal: status.channel, profesor: status.teacher };
    let t = status.viewed ? T("titulo.vista", v) : status.watching ? T("titulo.recibiendo", v) : T(TI + status.state, v);
    if (restricted && status.state === "connected") t += T("titulo.restricciones");
    if (pendingPower && status.state !== "unconfigured") t += T(TI + "pendiente." + pendingPower);
    // Lo nuevo que el alumno aún no mira, al frente del título (barra de tareas).
    const n = sinVer();
    if (n > 0) t = T("titulo.nuevos", { n }) + t;
    if (t === document.title) return;
    document.title = t;
    if (rt.WindowSetTitle) rt.WindowSetTitle(t); // también en la barra de tareas
    api.ReportWindow(t).catch(() => {}); // y en el icono del área de notificación
    pintarCompacto();
  }

  function renderStatus(s) {
    status = s;
    const v = { canal: s.channel, profesor: s.teacher };
    $("estado").dataset.estado = s.state;
    $("estado-texto").textContent = T(ES + s.state, v);
    $("estado-detalles").hidden = !s.detail;
    $("estado-detalle").textContent = s.detail || "";
    $("banda-vista").hidden = !s.viewed;
    $("banda-vista-texto").textContent = s.viewed ? T("banda.vista", v) : "";
    pintarVacio();
    title();
    pintarCompacto();
  }

  // El estado vacío dice qué pasa y que no hay nada que hacer, sin alarmar: lo
  // técnico queda en «Detalles para TI». Si el agente sabe por qué no conecta
  // (session.Cause), lo dice en palabras llanas.
  const VACIO = "vacio."; // + estado . titulo|texto
  const CAUSA = "causa."; // + causa de session.Cause
  const ICONO_VACIO = { connected: "codigo", searching: "pendiente", disconnected: "desconectado", unconfigured: "advertencia" };
  function pintarVacio() {
    const v = { canal: status.channel, profesor: status.teacher };
    $("vacio-icono").setAttribute("href", "sistema/iconos.svg#" + (ICONO_VACIO[status.state] || "codigo"));
    $("vacio-titulo").textContent = T(VACIO + status.state + ".titulo", v);
    const porCausa = status.state !== "connected" && status.state !== "unconfigured" && status.cause;
    $("vacio-texto").textContent = porCausa ? T(CAUSA + status.cause, v) : T(VACIO + status.state + ".texto", v);
  }

  // --- Modo compacto (prompt 19) ----------------------------------------------------------------
  // La ventana se encoge a una tira en una esquina: el estado, lo nuevo y la
  // pantalla vista. No queda encima de las demás ni toma el foco.
  let compacto = false;
  // Lo que el alumno aún no ve: código «Nuevo» y avisos abiertos.
  const sinVer = () => list.querySelectorAll(".nuevo:not([hidden])").length + avisos.length;
  function pintarCompacto() {
    const v = { canal: status.channel, profesor: status.teacher };
    $("compacto").dataset.estado = status.state;
    $("compacto-texto").textContent = status.state === "connected" ? T("compacto.conectado", v) : T(ES + status.state, v);
    const n = sinVer();
    $("compacto-nuevos").hidden = n === 0;
    $("compacto-nuevos").textContent = n ? P("compacto.nuevos", n) : "";
    $("compacto").dataset.nuevo = String(n > 0);
    $("compacto-vista").hidden = !status.viewed;
  }
  function ponerCompacto(on, avisarAGo) {
    compacto = on;
    document.body.classList.toggle("compacto", on);
    $("compacto").hidden = !on;
    pintarCompacto();
    if (avisarAGo) api.SetCompact(on).catch(() => {});
    $(on ? "restaurar" : "compactar").focus();
  }
  $("compactar").addEventListener("click", () => ponerCompacto(true, true));
  $("restaurar").addEventListener("click", () => ponerCompacto(false, true));

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

  // Nombres legibles de los lenguajes (identificadores de Monaco). Son nombres
  // propios: no se traducen.
  const LENGUAJES = {
    python: "Python", javascript: "JavaScript", typescript: "TypeScript", java: "Java", c: "C", cpp: "C++",
    csharp: "C#", go: "Go", rust: "Rust", kotlin: "Kotlin", sql: "SQL", html: "HTML", css: "CSS",
    json: "JSON", shell: "Bash", powershell: "PowerShell", xml: "XML", plaintext: "Texto",
  };
  const hora = (iso) => {
    const d = new Date(iso);
    return isNaN(d) ? "" : d.toLocaleTimeString("es-MX", { hour: "2-digit", minute: "2-digit" });
  };

  // La lista: lo más nuevo arriba. Se dibuja por partes (cada fragmento es un
  // nodo con clave id + llegada) y, si el alumno está leyendo más abajo, se
  // compensa lo que se insertó arriba para que lo que ve no se mueva. Un
  // fragmento que llega con la ventana ya abierta se marca «Nuevo» hasta que el
  // alumno lo selecciona o lo copia. ↑/↓ cambian de fragmento y Ctrl+Mayús+C
  // copia el seleccionado.
  const nodos = new Map();   // clave → li
  let primeraVez = true;
  let seleccionado = "";     // id del fragmento seleccionado
  let eligio = false;        // el alumno eligió uno; si no, la selección sigue a lo más nuevo
  const clave = (s) => s.id + "|" + s.receivedAt;

  function nodoDe(s, nuevo) {
    const node = snippetTpl.content.firstElementChild.cloneNode(true);
    node.dataset.id = s.id;
    node.tabIndex = -1;
    const exercise = node.querySelector(".exercise");
    exercise.textContent = s.exercise ? T("codigo.ejercicio", { n: s.exercise }) : "";
    exercise.hidden = !s.exercise;
    const titulo = s.title || T("codigo.sinTitulo");
    node.querySelector(".title").textContent = titulo;
    node.querySelector(".nuevo").textContent = T("codigo.nuevo");
    node.querySelector(".nuevo").hidden = !nuevo;
    const meta = [LENGUAJES[s.lang] || s.lang, hora(s.receivedAt)].filter(Boolean).join(" · ");
    node.querySelector(".lang").textContent = meta;
    node.querySelector("pre").setAttribute("aria-label", T("codigo.region", { titulo }));
    highlight(node.querySelector("code"), s.lang, s.content);
    const button = node.querySelector(".copy");
    button.querySelector("span").textContent = T("codigo.copiar");
    // El nombre accesible es «Copiar <título>» y siempre contiene lo que se ve.
    button.querySelector(".solo-lector").textContent = T("codigo.copiarDe", { titulo });
    button.addEventListener("click", () => copy(s.id));
    node.addEventListener("focusin", () => seleccionar(s.id));
    node.addEventListener("pointerdown", () => seleccionar(s.id));
    return node;
  }

  // El primer fragmento visible bajo la cabecera fija, y dónde está.
  function anclaVisible() {
    if (window.scrollY <= 0) return null; // arriba del todo: lo nuevo se ve y está bien
    const tope = document.querySelector(".arriba").getBoundingClientRect().bottom;
    for (const li of list.children) {
      const r = li.getBoundingClientRect();
      if (r.bottom > tope) return { li, top: r.top };
    }
    return null;
  }

  function renderBoard(snippets) {
    const ancla = anclaVisible();
    const orden = snippets.slice().reverse(); // lo más nuevo arriba
    const vivos = new Set(orden.map(clave));
    for (const [k, li] of nodos) {
      if (!vivos.has(k)) { li.remove(); nodos.delete(k); }
    }
    let anterior = null;
    for (const s of orden) {
      const k = clave(s);
      let li = nodos.get(k);
      if (!li) {
        li = nodoDe(s, !primeraVez);
        nodos.set(k, li);
      }
      // En su lugar: después del anterior, o el primero de la lista.
      const lugar = anterior ? anterior.nextSibling : list.firstChild;
      if (li !== lugar) list.insertBefore(li, lugar);
      anterior = li;
    }
    primeraVez = false;
    if (ancla && ancla.li.isConnected) window.scrollBy(0, ancla.li.getBoundingClientRect().top - ancla.top);
    empty.hidden = snippets.length > 0;
    $("codigos-atajo").hidden = snippets.length === 0;
    if (!eligio || !orden.some((s) => s.id === seleccionado)) seleccionado = orden.length ? orden[0].id : "";
    marcarSeleccion();
    title();
  }

  function marcarSeleccion() {
    for (const li of list.children) {
      if (li.dataset.id === seleccionado) li.setAttribute("aria-current", "true");
      else li.removeAttribute("aria-current");
    }
  }
  function seleccionar(id) {
    seleccionado = id;
    eligio = true;
    const li = nodoPorId(id);
    if (li) li.querySelector(".nuevo").hidden = true; // ya lo vio
    marcarSeleccion();
    title();
  }
  const nodoPorId = (id) => [...list.children].find((li) => li.dataset.id === id);

  // ↑/↓ entre fragmentos (fuera del bloque de código, donde las flechas lo
  // desplazan) y Ctrl+Mayús+C para copiar el seleccionado.
  document.addEventListener("keydown", (e) => {
    if (e.ctrlKey && e.shiftKey && !e.altKey && e.code === "KeyC") {
      if (!seleccionado) return;
      e.preventDefault();
      copy(seleccionado);
      return;
    }
    if ((e.key !== "ArrowDown" && e.key !== "ArrowUp") || e.altKey || e.ctrlKey || e.metaKey) return;
    if (e.target.closest && e.target.closest("pre, input, textarea, select, video")) return;
    const items = [...list.children];
    if (!items.length) return;
    const i = Math.max(0, items.findIndex((li) => li.dataset.id === seleccionado));
    const j = e.key === "ArrowDown" ? Math.min(items.length - 1, i + 1) : Math.max(0, i - 1);
    e.preventDefault();
    seleccionar(items[j].dataset.id);
    items[j].focus();
  });

  // El texto lo copia Go, del fragmento tal como llegó (sangría y saltos incluidos).
  function copy(id) {
    const li = nodoPorId(id);
    if (!li) return;
    seleccionar(id);
    const button = li.querySelector(".copy");
    const error = li.querySelector(".codigo-recibido__error");
    const titulo = li.querySelector(".title").textContent;
    error.hidden = true;
    api.Copy(id).then(() => {
      button.querySelector("span").textContent = T("codigo.copiado");
      anunciar(T("codigos.anuncioCopiado", { titulo }));
      setTimeout(() => { button.querySelector("span").textContent = T("codigo.copiar"); }, 2000);
    }, (err) => {
      error.textContent = T("codigo.noSeCopio", { motivo: String(err && err.message ? err.message : err) });
      error.hidden = false;
    });
  }

  // Mensajes para lectores de pantalla (región viva, invisible).
  function anunciar(texto) {
    const r = $("anuncio");
    r.textContent = "";
    setTimeout(() => { r.textContent = texto; }, 50);
  }

  // --- Avisos del profesor ----------------------------------------------------------------------
  const noticesBox = $("notices");
  const noticeTpl = $("notice-template");
  const NIVEL = { info: ["info", ""], warning: ["advertencia", "toast--advertencia"] };

  // En una esquina, sin mover el código; se ven los 3 más recientes y «Ver N
  // más» despliega el resto. Esc cierra el aviso que tiene el foco.
  // En una ventana angosta, uno solo: más taparían el código.
  const maxAvisos = () => (window.matchMedia("(max-width: 600px)").matches ? 1 : 3);
  let avisos = [];
  let todos = false;
  function renderNotices(notices) {
    avisos = notices;
    if (notices.length <= maxAvisos()) todos = false;
    const visibles = todos ? notices : notices.slice(-maxAvisos());
    const mas = notices.length - visibles.length;
    $("avisos-mas").hidden = mas === 0;
    $("avisos-mas").textContent = mas ? P("aviso.mas", mas) : "";
    noticesBox.replaceChildren();
    for (const n of visibles) {
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
      node.addEventListener("keydown", (e) => { if (e.key === "Escape") { e.stopPropagation(); api.DismissNotice(n.id); } });
      noticesBox.appendChild(node);
    }
    title();
    pintarCompacto();
  }
  $("avisos-mas").addEventListener("click", () => { todos = true; renderNotices(avisos); });

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
  // Go sale del modo compacto cuando el profesor hace algo que el alumno debe ver.
  rt.EventsOn(ev.COMPACT, (c) => { if (!c.on && compacto) ponerCompacto(false, false); });
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
