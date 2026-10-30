// Interfaz de la consola del profesor. El estado vive en Go (App); aquí solo se
// dibuja y se llaman sus métodos. Todo texto visible sale de textos.es-MX.js y
// todo texto recibido entra con textContent: nunca se inserta HTML.
"use strict";

(function () {
  const api = window.go.main.App;
  const rt = window.runtime;
  const ev = window.AulaEvents;
  const TX = window.AulaTextos;
  const $ = (id) => document.getElementById(id);
  const $$ = (sel, root) => [...(root || document).querySelectorAll(sel)];
  const SVG = "http://www.w3.org/2000/svg";

  // --- Textos ----------------------------------------------------------------------------
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
    for (const e of $$("[data-t]", root)) e.textContent = T(e.dataset.t);
    for (const [attr, name] of [["tAriaLabel", "aria-label"], ["tTitle", "title"], ["tPlaceholder", "placeholder"], ["tEtiqueta", "data-etiqueta"]]) {
      const sel = "[data-" + attr.replace(/[A-Z]/g, (c) => "-" + c.toLowerCase()) + "]";
      for (const e of $$(sel, root)) e.setAttribute(name, T(e.dataset[attr]));
    }
  }

  // --- Piezas de interfaz ----------------------------------------------------------------
  function icono(id, cls) {
    const s = document.createElementNS(SVG, "svg");
    s.setAttribute("class", "icono" + (cls ? " " + cls : ""));
    s.setAttribute("aria-hidden", "true");
    const u = document.createElementNS(SVG, "use");
    u.setAttribute("href", "sistema/iconos.svg#" + id);
    s.append(u);
    return s;
  }
  function el(tag, cls, text) {
    const e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text !== undefined) e.textContent = text;
    return e;
  }
  function insignia(tipo, ic, texto) {
    const s = el("span", "insignia" + (tipo ? " insignia--" + tipo : ""));
    if (ic) s.append(icono(ic));
    s.append(document.createTextNode(texto));
    return s;
  }
  function boton(texto, cls, ic) {
    const b = el("button", "btn" + (cls ? " " + cls : ""));
    b.type = "button";
    if (ic) b.append(icono(ic));
    b.append(document.createTextNode(texto));
    return b;
  }
  const mmss = (ms) => {
    const s = Math.max(0, Math.ceil(ms / 1000));
    return Math.floor(s / 60) + ":" + String(s % 60).padStart(2, "0");
  };

  // Mensajes de error con la fórmula «qué pasó + qué hacer», y el detalle
  // técnico aparte. Los errores de Go llegan como texto.
  function amable(err) {
    const raw = String(err && err.message ? err.message : err);
    const casos = [
      [/puerto \d+ está ocupado/, "error.puertoOcupado"],
      [/^clave del centro:/, "error.claveCentro"],
      [/^clave de la consola:/, "error.claveConsola"],
      [/^certificado:/, "error.certificado"],
      [/codificador H\.264/, "error.sinH264"],
      [/ocultar la consola/, "error.ocultarConsola"],
      [/portapapeles no está disponible/, "error.sinPortapapeles"],
      [/portapapeles (está vacío|no tiene texto)/, "error.portapapelesVacio"],
    ];
    for (const [re, key] of casos) {
      if (re.test(raw)) return { titulo: T(key + ".titulo"), texto: T(key + ".texto"), detalle: raw };
    }
    const frase = raw.charAt(0).toUpperCase() + raw.slice(1);
    return { titulo: T("error.generico"), texto: /[.!?]$/.test(frase) ? frase : frase + ".", detalle: "" };
  }

  // --- Avisos flotantes ------------------------------------------------------------------
  function toast(tipo, titulo, texto, detalle) {
    const ic = { exito: "conectado", error: "error", advertencia: "advertencia", info: "info" }[tipo];
    const t = el("div", "toast toast--" + tipo);
    t.setAttribute("role", tipo === "error" ? "alert" : "status");
    t.append(icono(ic));
    const cuerpo = el("div", "toast__texto");
    cuerpo.append(el("span", "toast__titulo", titulo));
    if (texto) cuerpo.append(el("span", "pequeno", texto));
    if (detalle) {
      const d = el("details", "detalles");
      d.append(el("summary", "", T("comun.verDetalles")), el("pre", "", detalle));
      cuerpo.append(d);
    }
    const cerrar = el("button", "btn btn--fantasma btn--icono btn--sm");
    cerrar.type = "button";
    cerrar.setAttribute("aria-label", T("comun.cerrar"));
    cerrar.append(icono("cerrar"));
    cerrar.addEventListener("click", () => t.remove());
    t.append(cuerpo, cerrar);
    $("avisos").append(t);
    if (tipo !== "error") setTimeout(() => t.remove(), 6000); // los errores se quedan
  }
  const fallo = (err) => {
    const m = amable(err);
    toast("error", m.titulo, m.texto, m.detalle);
  };

  // --- Confirmación única ------------------------------------------------------------------
  // confirmar({titulo, texto, consecuencia, si, peligro}) → Promise<boolean>
  let confirmarResolver = null;
  function confirmar(o) {
    $("confirmar-titulo").textContent = o.titulo;
    $("confirmar-texto").textContent = o.texto || "";
    $("confirmar-texto").hidden = !o.texto;
    $("confirmar-consecuencia").hidden = !o.consecuencia;
    $("confirmar-consecuencia-texto").textContent = o.consecuencia || "";
    const si = $("confirmar-si");
    si.textContent = o.si;
    si.className = "btn " + (o.peligro ? "btn--peligro" : "btn--primario");
    $("confirmar").classList.toggle("dialogo--peligro", !!o.peligro);
    $("confirmar").showModal();
    $("confirmar-no").focus();
    return new Promise((resolve) => { confirmarResolver = resolve; });
  }
  $("confirmar-no").addEventListener("click", () => $("confirmar").close());
  $("confirmar-si").addEventListener("click", () => {
    const r = confirmarResolver;
    confirmarResolver = null;
    $("confirmar").close();
    if (r) r(true);
  });
  $("confirmar").addEventListener("close", () => {
    if (confirmarResolver) confirmarResolver(false);
    confirmarResolver = null;
  });
  document.addEventListener("click", (e) => {
    const c = e.target.closest("[data-cerrar-dialogo]");
    if (c) c.closest("dialog").close();
  });

  // --- Copiar ----------------------------------------------------------------------------
  document.addEventListener("click", (e) => {
    const b = e.target.closest("[data-copiar]");
    if (!b) return;
    const texto = $(b.dataset.copiar).textContent;
    const bien = () => toast("exito", T("copiar.hecho"), T("copiar.hechoTexto"));
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(texto).then(bien, () => toast("error", T("copiar.fallo"), T("copiar.falloTexto")));
    } else {
      toast("error", T("copiar.fallo"), T("copiar.falloTexto"));
    }
  });

  // --- Tema ------------------------------------------------------------------------------
  const oscuroSistema = window.matchMedia("(prefers-color-scheme: dark)");
  let tema = "sistema";
  try { tema = localStorage.getItem("aula-tema") || "sistema"; } catch { /* sin almacenamiento */ }
  const esOscuro = () => tema === "dark" || (tema === "sistema" && oscuroSistema.matches);
  function aplicarTema(t) {
    tema = t;
    if (t === "dark" || t === "light") document.documentElement.dataset.theme = t;
    else delete document.documentElement.dataset.theme;
    for (const b of $$("[data-tema]")) b.setAttribute("aria-checked", String(b.dataset.tema === t));
    try { localStorage.setItem("aula-tema", t); } catch { /* sin almacenamiento */ }
    if (window.monaco) window.monaco.editor.setTheme(esOscuro() ? "vs-dark" : "vs");
  }
  oscuroSistema.addEventListener("change", () => aplicarTema(tema));
  for (const b of $$("[data-tema]")) b.addEventListener("click", () => aplicarTema(b.dataset.tema));

  // --- Estado ----------------------------------------------------------------------------
  let setup = { recentChannels: [], version: "", defaultChannel: "1" };
  let clase = { channel: "", consoleName: "", fingerprint: "", keySource: "centro" };
  let alumnos = { students: [], connected: 0, simulated: 0, canCode: 0, canNotice: 0, canPower: 0, canAppBlock: 0, canWebFilter: 0 };
  let historial = [];
  let envios = [];
  let hotkey = { combo: "", active: true, error: "" };
  let bc = { active: false, display: -1, receivers: 0, eligible: 0, available: false };
  let restricciones = { apps: null, web: null };
  let simulada = { active: false, students: [], capabilities: [] };
  let modo = "lista";
  let vista = "alumnos";
  // La selección se guarda por equipo (sobrevive a las reconexiones, que
  // cambian el ID de sesión); al enviar se traduce a los IDs vigentes.
  const seleccion = new Set();
  const clave = (s) => (s.simulated ? "s:" : "r:") + s.hostname;

  const conectados = () => alumnos.students.filter((s) => s.state === "connected");
  const seleccionados = () => conectados().filter((s) => seleccion.has(clave(s)));
  // Destino de código y avisos: la selección, o nadie en particular (todos).
  const destinos = () => (seleccion.size ? seleccionados().map((s) => s.id) : []);
  // Destino de restricciones y energía: equipos reales, por nombre.
  const equiposReales = () => seleccionados().filter((s) => !s.simulated).map((s) => s.hostname);

  // --- Navegación ------------------------------------------------------------------------
  const VISTAS = ["alumnos", "codigo", "simulada", "ayuda", "falta"];
  function irA(nueva) {
    if (!VISTAS.includes(nueva)) nueva = "alumnos";
    const antes = vista;
    vista = nueva;
    for (const s of $$("[data-vista-de]")) s.hidden = s.dataset.vistaDe !== nueva;
    const enNav = nueva === "falta" ? "ayuda" : nueva;
    for (const a of $$(".nav-lateral [data-vista]")) {
      if (a.dataset.vista === enNav) a.setAttribute("aria-current", "page");
      else a.removeAttribute("aria-current");
    }
    if (nueva !== "alumnos") cerrarPanel();
    // Las miniaturas solo se piden mientras se ven (docs/privacidad.md regla 8).
    if (antes === "alumnos" && nueva !== "alumnos" && modo === "miniaturas") api.CloseScreens();
    if (nueva === "alumnos" && antes !== "alumnos" && modo === "miniaturas") abrirMiniaturas();
    if (nueva === "codigo" && editor) editor.layout();
    if (nueva === "falta") pintarFalta();
    if (nueva === "simulada") pintarSimulada();
    $("contenido").scrollTop = 0;
  }
  window.addEventListener("hashchange", () => irA(location.hash.slice(1)));

  // --- Arranque --------------------------------------------------------------------------
  function mostrarArranque(error) {
    $("app").hidden = true;
    $("arranque").hidden = false;
    api.Setup().then((s) => {
      setup = s;
      const fallo = error || s.startError;
      $("arranque-estado").hidden = !!fallo;
      $("arranque-estado-texto").textContent = T("arranque.abriendo", { canal: s.defaultChannel });
      $("arranque-error").hidden = !fallo;
      if (fallo) {
        const m = amable(fallo);
        $("arranque-error-titulo").textContent = m.titulo;
        $("arranque-error-texto").textContent = m.texto;
        $("arranque-error-detalle").textContent = m.detalle || String(fallo);
        $("arranque-error-detalle").parentElement.hidden = !m.detalle;
      }
      $("arranque-canal").value = s.defaultChannel;
      $("arranque-nombre").value = s.consoleName || "";
      $("arranque-recientes").replaceChildren(...s.recentChannels.slice(0, 6).map((c) => {
        const b = boton(T("canal.reciente", { canal: c }), "btn--sm");
        b.addEventListener("click", () => { $("arranque-canal").value = c; });
        return b;
      }));
      $("arranque-version").textContent = T("arranque.version", { version: s.version });
      // Mientras abre el canal predeterminado, el formulario espera.
      for (const e of $$("#arranque-form input, #arranque-form button")) e.disabled = !fallo;
    });
  }
  $("arranque-form").addEventListener("submit", (e) => {
    e.preventDefault();
    const b = $("arranque-abrir");
    b.setAttribute("aria-busy", "true");
    b.disabled = true;
    api.StartClass(String($("arranque-canal").value).trim(), $("arranque-nombre").value).then(mostrarConsola, (err) => {
      mostrarArranque(err);
    }).finally(() => { b.removeAttribute("aria-busy"); b.disabled = false; });
  });

  // --- Consola ---------------------------------------------------------------------------
  function mostrarConsola(v) {
    $("arranque").hidden = true;
    $("app").hidden = false;
    clase = v.class;
    hotkey = v.hotkey;
    $("nav-clase-texto").textContent = T("nav.clase", { canal: clase.channel, nombre: clase.consoleName });
    $("nav-clase-compacta").textContent = clase.channel;
    $("nav-clase-compacta").setAttribute("aria-label", T("nav.claseCompacta", { canal: clase.channel }));
    $("f-clave").hidden = clase.keySource === "centro";
    pintarAlumnos(v.students);
    pintarTransmision(v.broadcast);
    pintarRestricciones(v.restrictions);
    pintarHistorial(v.history);
    pintarEnvios(v.deliveries);
    api.Setup().then((s) => { setup = s; });
    api.Simulation().then(pintarSimulacion);
    if (!editor) iniciarEditor();
    irA(location.hash.slice(1) || "alumnos");
  }

  // --- Alumnos ---------------------------------------------------------------------------
  function notasDe(s) {
    const n = [];
    if (s.simulated) n.push(insignia("simulado", "simulado", T("alumnos.simulado")));
    if (s.state !== "connected") return n;
    if (s.appsActive || s.webActive) n.push(insignia("advertencia", "candado", T("alumnos.conRestricciones")));
    if (s.power) n.push(insigniaEnergia(s.power));
    if (!s.canCode) n.push(insignia("", "info", s.username ? T("alumnos.noRecibeCodigo") : T("alumnos.sinSesion")));
    else if (!s.canThumb && !s.simulated) n.push(insignia("", "info", T("alumnos.noComparte")));
    return n;
  }
  function codigoDe(s) {
    if (!s.ackedAt) return [el("span", "suave", "—")];
    if (!s.lastCodeAck) return [document.createTextNode(T("alumnos.pantallaLimpia")), el("span", "suave pequeno", " " + s.ackedAt)];
    const h = historial.find((e) => e.snippetId === s.lastCodeAck);
    return [document.createTextNode(h ? T("codigo.ejercicio", { n: h.number }) : T("alumnos.codigoRecibido")), el("span", "suave pequeno", " " + s.ackedAt)];
  }
  const estadoDe = (s) => (s.state === "connected"
    ? insignia("conectado", "conectado", T("alumnos.conectado"))
    : insignia("desconectado", "desconectado", T("alumnos.desconectado")));

  function pintarAlumnos(st) {
    alumnos = st;
    // La selección olvida a quien ya no está en la lista.
    const presentes = new Set(st.students.map(clave));
    for (const k of [...seleccion]) if (!presentes.has(k)) seleccion.delete(k);

    const reales = st.students.filter((s) => !s.simulated);
    const sims = st.students.filter((s) => s.simulated);
    const desc = reales.filter((s) => s.state !== "connected").length;
    const res = [insignia("conectado", "conectado", P("alumnos.conectados", st.connected))];
    if (desc) res.push(insignia("desconectado", "desconectado", P("alumnos.desconectados", desc)));
    if (st.simulated) res.push(insignia("simulado", "simulado", P("alumnos.simulados", st.simulated)));
    $("alumnos-resumen").replaceChildren(...res);

    const vacio = st.students.length === 0;
    $("alumnos-vacio").hidden = !vacio;
    $("alumnos-vacio-titulo").textContent = T("alumnos.vacioTitulo", { canal: clase.channel });
    $("alumnos-lista").hidden = vacio || modo !== "lista";
    $("alumnos-miniaturas").hidden = vacio || modo !== "miniaturas";

    const tplFila = $("t-fila-alumno").content.firstElementChild;
    const filas = [];
    const fila = (s) => {
      const tr = tplFila.cloneNode(true);
      const marcado = seleccion.has(clave(s));
      tr.dataset.clave = clave(s);
      tr.dataset.seleccionado = String(marcado);
      const m = tr.querySelector(".marca");
      m.checked = marcado;
      m.setAttribute("aria-label", T("alumnos.seleccionar", { equipo: s.hostname }));
      tr.querySelector(".equipo").textContent = s.hostname;
      tr.querySelector(".usuario").textContent = s.username || T("alumnos.sinSesion");
      tr.querySelector(".estado").append(estadoDe(s));
      tr.querySelector(".codigo").append(...codigoDe(s));
      const notas = notasDe(s);
      tr.querySelector(".notas").append(...(notas.length ? notas : [el("span", "suave", "—")]));
      return tr;
    };
    filas.push(...reales.map(fila));
    if (sims.length) {
      const g = el("tr", "tabla__grupo");
      const th = el("th");
      th.colSpan = 5;
      th.scope = "rowgroup";
      th.append(icono("simulado"), document.createTextNode(" " + T("alumnos.grupoSimulados")), insignia("simulado", "", T("alumnos.grupoSimuladosNota")));
      g.append(th);
      filas.push(g, ...sims.map(fila));
    }
    $("alumnos-filas").replaceChildren(...filas);
    pintarMiniaturas();
    pintarSeleccion();
    pintarEnergia();
    if (!$("panel-restricciones").hidden) pintarPanelRestricciones();
    pintarDestinoCodigo();
    actualizarAccionesCodigo();
  }

  // Miniaturas: las imágenes solo viven aquí, en memoria.
  const tarjetas = new Map(); // clave → <li>
  function pintarMiniaturas() {
    const tpl = $("t-tarjeta-alumno").content.firstElementChild;
    const reales = alumnos.students.filter((s) => !s.simulated);
    const sims = alumnos.students.filter((s) => s.simulated);
    const tarjeta = (s) => {
      const k = clave(s);
      let li = tarjetas.get(k);
      if (!li) {
        li = tpl.cloneNode(true);
        tarjetas.set(k, li);
        li.querySelector(".alumno__pantalla").addEventListener("click", () => verEnGrande(k));
      }
      li.dataset.clave = k;
      li.dataset.seleccionado = String(seleccion.has(k));
      li.classList.toggle("alumno--desconectado", s.state !== "connected");
      const m = li.querySelector(".marca");
      m.checked = seleccion.has(k);
      m.setAttribute("aria-label", T("alumnos.seleccionar", { equipo: s.hostname }));
      li.querySelector(".alumno__nombre").textContent = s.hostname;
      li.querySelector(".estado").replaceChildren(s.simulated ? insignia("simulado", "simulado", T("alumnos.simulado")) : estadoDe(s));
      li.querySelector(".alumno__sub").textContent = s.username || T("alumnos.sinSesion");
      const img = li.querySelector("img");
      const texto = li.querySelector(".alumno__pantalla-texto");
      const puede = s.state === "connected" && s.canThumb;
      const pantalla = li.querySelector(".alumno__pantalla");
      pantalla.disabled = !puede;
      // Solo se nombra como acción cuando lo es; si no, su texto dice por qué.
      if (puede) pantalla.setAttribute("aria-label", T("alumnos.verGrande", { equipo: s.hostname }));
      else pantalla.removeAttribute("aria-label");
      img.alt = T("alumnos.pantallaDe", { equipo: s.hostname });
      if (!puede) img.removeAttribute("src");
      texto.textContent = s.state !== "connected" ? T("alumnos.desconectado")
        : !s.canThumb ? (s.canCode ? T("alumnos.noComparte") : T("alumnos.sinSesion"))
          : img.getAttribute("src") ? "" : T("alumnos.esperandoImagen");
      return li;
    };
    const items = reales.map(tarjeta);
    if (sims.length) {
      const g = el("li", "cuadricula-alumnos__grupo");
      g.append(icono("simulado"), document.createTextNode(T("alumnos.grupoSimulados") + " · " + T("alumnos.grupoSimuladosNota")));
      items.push(g, ...sims.map(tarjeta));
    }
    const vivos = new Set(alumnos.students.map(clave));
    for (const k of [...tarjetas.keys()]) if (!vivos.has(k)) tarjetas.delete(k);
    $("alumnos-cuadricula").replaceChildren(...items);
  }

  function alumnoPorSesion(id) { return alumnos.students.find((s) => s.id === id); }
  let enGrande = "";
  function verEnGrande(k) {
    const s = alumnos.students.find((x) => clave(x) === k);
    if (!s || s.state !== "connected") return;
    enGrande = s.id;
    $("pg-titulo").textContent = s.hostname + (s.username ? " · " + s.username : "");
    const li = tarjetas.get(k);
    const src = li && li.querySelector("img").getAttribute("src");
    if (src) $("pg-imagen").src = src; else $("pg-imagen").removeAttribute("src");
    $("pg-imagen").alt = T("alumnos.pantallaDe", { equipo: s.hostname });
    $("pantalla-grande").showModal();
    api.ViewScreen(s.id);
  }
  $("pantalla-grande").addEventListener("close", () => {
    enGrande = "";
    $("pg-imagen").removeAttribute("src");
    api.ViewScreen("");
  });
  function alCuadro(t) {
    const s = alumnoPorSesion(t.sessionId);
    if (!s) return;
    const src = "data:image/jpeg;base64," + t.jpeg;
    const li = tarjetas.get(clave(s));
    if (li) {
      li.querySelector("img").src = src;
      li.querySelector(".alumno__pantalla-texto").textContent = "";
    }
    if (t.sessionId === enGrande) $("pg-imagen").src = src;
  }

  function abrirMiniaturas() {
    api.OpenScreens().catch(fallo);
  }
  function ponerModo(m) {
    if (m === modo) return;
    modo = m;
    for (const b of $$("[data-modo]")) b.setAttribute("aria-pressed", String(b.dataset.modo === m));
    if (m === "miniaturas") abrirMiniaturas();
    else {
      api.CloseScreens();
      for (const li of tarjetas.values()) li.querySelector("img").removeAttribute("src");
    }
    pintarAlumnos(alumnos);
  }
  for (const b of $$("[data-modo]")) b.addEventListener("click", () => ponerModo(b.dataset.modo));

  // Selección: clic en la fila o la tarjeta, la casilla, el botón o Ctrl+A.
  function alternar(k) {
    if (seleccion.has(k)) seleccion.delete(k); else seleccion.add(k);
    pintarAlumnos(alumnos);
  }
  for (const cont of [$("alumnos-filas"), $("alumnos-cuadricula")]) {
    cont.addEventListener("click", (e) => {
      if (e.target.closest(".alumno__pantalla")) return;
      const f = e.target.closest("[data-clave]");
      if (f) alternar(f.dataset.clave);
    });
  }
  function seleccionarTodos(si) {
    seleccion.clear();
    if (si) for (const s of alumnos.students) seleccion.add(clave(s));
    pintarAlumnos(alumnos);
  }
  $("alumnos-todos").addEventListener("change", (e) => seleccionarTodos(e.target.checked));
  $("seleccionar-todos").addEventListener("click", () => seleccionarTodos(seleccion.size === 0));
  document.addEventListener("keydown", (e) => {
    if (vista !== "alumnos" || $("app").hidden || !(e.ctrlKey || e.metaKey) || e.key.toLowerCase() !== "a") return;
    if (e.target.closest("input, textarea, select, [contenteditable], dialog, .monaco-editor")) return;
    e.preventDefault();
    seleccionarTodos(true);
  });

  function pintarSeleccion() {
    const n = seleccion.size;
    $("seleccion-texto").textContent = n === 0 ? T("seleccion.ninguno") : P("seleccion.alumnos", n);
    const todos = $("alumnos-todos");
    todos.checked = n > 0 && n === alumnos.students.length;
    todos.indeterminate = n > 0 && n < alumnos.students.length;
    $("seleccionar-todos-texto").textContent = n > 0 ? T("alumnos.quitarSeleccion") : T("alumnos.seleccionarTodos");
    $("seleccionar-todos-icono").setAttribute("href", "sistema/iconos.svg#" + (n > 0 ? "cerrar" : "hecho"));
    $("seleccionar-todos").disabled = alumnos.students.length === 0;
    const destino = n === 0 ? T("destino.todos", { n: conectados().length }) : P("destino.seleccion", n);
    for (const p of $$("[data-destino]")) p.textContent = destino;
    pintarDestinoEquipos();
  }

  // --- Paneles laterales ---------------------------------------------------------------------
  let panel = "";
  function abrirPanel(id) {
    panel = id;
    for (const p of $$(".panel-lateral")) p.hidden = p.id !== "panel-" + id;
    for (const b of $$("[data-panel]")) b.setAttribute("aria-expanded", String(b.dataset.panel === id));
    if (id === "pantalla") pintarPanelPantalla();
    if (id === "restricciones") pintarPanelRestricciones();
    if (id === "energia") pintarPanelEnergia();
    $("h-panel-" + id).focus();
  }
  function cerrarPanel() {
    panel = "";
    for (const p of $$(".panel-lateral")) p.hidden = true;
    for (const b of $$("[data-panel]")) b.setAttribute("aria-expanded", "false");
  }
  for (const b of $$("[data-panel]")) b.addEventListener("click", () => (panel === b.dataset.panel ? cerrarPanel() : abrirPanel(b.dataset.panel)));
  for (const b of $$("[data-cerrar-panel]")) b.addEventListener("click", cerrarPanel);
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape" && panel && !document.querySelector("dialog[open]")) cerrarPanel();
  });

  // Restricciones y energía van por equipo real: los simulados nunca las reciben.
  function pintarDestinoEquipos() {
    const n = seleccion.size;
    const reales = equiposReales();
    const simsSel = seleccionados().filter((s) => s.simulated).length;
    let texto = n === 0 ? T("destino.todosReales", { n: alumnos.connected }) : P("destino.equipos", reales.length);
    if (simsSel) texto += " " + P("destino.simuladosNo", simsSel);
    for (const p of $$("[data-destino-hosts]")) p.textContent = texto;
  }

  // Mostrar mi pantalla: va a todos los que pueden recibirla.
  let monitores = [];
  function pintarPanelPantalla() {
    api.Displays().then((ds) => {
      monitores = ds;
      const sel = $("pantalla-monitor");
      const antes = sel.value;
      sel.replaceChildren(...ds.map((d) => Object.assign(el("option", "", d.label), { value: String(d.index) })));
      if (ds.some((d) => String(d.index) === antes)) sel.value = antes;
      pintarTransmision(bc);
    });
  }
  function pintarTransmision(s) {
    bc = s;
    $("f-transmitiendo").hidden = !s.active;
    if (s.active) {
      const m = monitores.find((d) => d.index === s.display);
      $("f-transmitiendo-texto").textContent = T("transmitir.franja", { monitor: m ? m.label.split(" · ")[0] : "", r: s.receivers, n: s.eligible });
    }
    $("pantalla-destino").textContent = s.eligible ? P("transmitir.destino", s.eligible) : T("transmitir.nadie");
    $("pantalla-no-disponible").hidden = s.available;
    const b = $("pantalla-empezar");
    b.textContent = s.active ? T("transmitir.detener") : T("transmitir.empezar");
    b.className = "btn " + (s.active ? "btn--peligro" : "btn--primario");
    b.disabled = !s.available || (!s.active && $("pantalla-monitor").options.length === 0);
    $("pantalla-monitor").disabled = s.active;
  }
  $("pantalla-empezar").addEventListener("click", () => {
    const b = $("pantalla-empezar");
    b.setAttribute("aria-busy", "true");
    const p = bc.active ? api.StopBroadcast() : api.StartBroadcast(Number($("pantalla-monitor").value));
    p.then((s) => { pintarTransmision(s); if (s.active) cerrarPanel(); }, fallo).finally(() => b.removeAttribute("aria-busy"));
  });
  $("f-transmitiendo-detener").addEventListener("click", () => api.StopBroadcast().then(pintarTransmision, fallo));

  // Aviso
  $("aviso-enviar").addEventListener("click", () => {
    const texto = $("aviso-texto").value.trim();
    const err = $("aviso-error");
    if (!texto) {
      err.hidden = false;
      err.replaceChildren(icono("error", "icono--sm"), document.createTextNode(T("aviso.vacio")));
      $("aviso-texto").setAttribute("aria-invalid", "true");
      $("aviso-texto").focus();
      return;
    }
    err.hidden = true;
    $("aviso-texto").removeAttribute("aria-invalid");
    const nivel = document.querySelector('input[name="aviso-nivel"]:checked').value;
    const b = $("aviso-enviar");
    b.setAttribute("aria-busy", "true");
    api.SendNoticeTo(texto, nivel, destinos()).then((d) => {
      $("aviso-texto").value = "";
      cerrarPanel();
      toast("exito", P("aviso.enviado", d.sent + d.simSent), T("aviso.enviadoTexto"));
    }, fallo).finally(() => b.removeAttribute("aria-busy"));
  });

  // Restricciones
  const PLANTILLAS = {
    apps: [["restr.plantillaJuegos", "deny", ["steam", "epicgameslauncher", "riotclientservices", "leagueclient",
      "robloxplayerbeta", "minecraftlauncher", "discord", "whatsapp", "telegram", "spotify"]]],
    web: [["restr.plantillaRedes", "deny", ["youtube.com", "tiktok.com", "instagram.com", "facebook.com", "x.com",
      "twitter.com", "twitch.tv", "netflix.com", "roblox.com", "discord.com", "web.whatsapp.com"]],
    ["restr.plantillaDocs", "allow", ["python.org", "developer.mozilla.org", "w3schools.com", "cppreference.com",
      "docs.oracle.com", "go.dev", "learn.microsoft.com", "stackoverflow.com", "github.com", "githubusercontent.com",
      "fonts.googleapis.com", "fonts.gstatic.com", "cdnjs.cloudflare.com", "cdn.jsdelivr.net"]]],
  };
  let tipoRestr = "web";
  for (const b of $$("[data-restr-tipo]")) {
    b.addEventListener("click", () => {
      tipoRestr = b.dataset.restrTipo;
      for (const o of $$("[data-restr-tipo]")) o.setAttribute("aria-pressed", String(o === b));
      $("restr-lista").value = "";
      pintarPanelRestricciones();
    });
  }
  function pintarRestricciones(v) {
    restricciones = v;
    if (!$("panel-restricciones").hidden) pintarPanelRestricciones();
  }
  function pintarPanelRestricciones() {
    const regla = restricciones[tipoRestr];
    const puede = tipoRestr === "apps" ? alumnos.canAppBlock : alumnos.canWebFilter;
    $("restr-lista-etiqueta").textContent = T(tipoRestr === "apps" ? "restr.listaApps" : "restr.listaWeb");
    $("restr-ayuda").textContent = T(tipoRestr === "apps" ? "restr.ayudaApps" : "restr.ayudaWeb");
    if (!$("restr-lista").value && regla) {
      $("restr-modo").value = regla.mode;
      $("restr-lista").value = regla.items.join("\n");
    }
    if (regla) {
      const activa = alumnos.students.filter((s) => s.state === "connected" && (tipoRestr === "apps" ? s.appsActive : s.webActive)).length;
      $("restr-estado").textContent = T(regla.mode === "allow" ? "restr.estadoPermitir" : "restr.estadoBloquear",
        { n: regla.items.length, activa, de: puede || 0 });
    } else {
      $("restr-estado").textContent = T("restr.sinRegla");
    }
    $("restr-quitar").disabled = !regla;
    $("restr-quitar-todas").hidden = !(restricciones.apps && restricciones.web);
    $("restr-sin-soporte").hidden = puede > 0;
    $("restr-sin-soporte-texto").textContent = T("restr.nadiePuede");
    $("restr-aplicar").disabled = !puede;
    $("restr-plantillas").replaceChildren(...PLANTILLAS[tipoRestr].map(([k, m, items]) => {
      const b = boton(T(k), "btn--sm");
      b.addEventListener("click", () => { $("restr-modo").value = m; $("restr-lista").value = items.join("\n"); });
      return b;
    }));
  }
  $("restr-aplicar").addEventListener("click", () => {
    const items = $("restr-lista").value.split(/\r?\n/).map((s) => s.trim()).filter(Boolean);
    const err = $("restr-error");
    if (!items.length) {
      err.hidden = false;
      err.replaceChildren(icono("error", "icono--sm"), document.createTextNode(T("restr.vacia")));
      $("restr-lista").setAttribute("aria-invalid", "true");
      return;
    }
    if (seleccion.size && !equiposReales().length) {
      toast("advertencia", T("restr.soloSimulados"), T("restr.soloSimuladosTexto"));
      return;
    }
    const b = $("restr-aplicar");
    b.setAttribute("aria-busy", "true");
    api.SetRestriction({ kind: tipoRestr, mode: $("restr-modo").value, items, hosts: equiposReales() }).then((v) => {
      err.hidden = true;
      $("restr-lista").removeAttribute("aria-invalid");
      pintarRestricciones(v);
      toast("exito", T("restr.aplicada"), T("restr.aplicadaTexto"));
    }, (e) => {
      // El error va junto al campo, sin perder lo escrito.
      err.hidden = false;
      err.replaceChildren(icono("error", "icono--sm"), document.createTextNode(amable(e).texto));
      $("restr-lista").setAttribute("aria-invalid", "true");
    }).finally(() => b.removeAttribute("aria-busy"));
  });
  $("restr-quitar").addEventListener("click", () => {
    confirmar({
      titulo: T(tipoRestr === "apps" ? "restr.quitarAppsTitulo" : "restr.quitarWebTitulo"),
      texto: T("restr.quitarTexto"), si: T("restr.quitar"),
    }).then((si) => {
      if (!si) return;
      api.ClearRestriction(tipoRestr).then((v) => { $("restr-lista").value = ""; pintarRestricciones(v); }, fallo);
    });
  });
  $("restr-quitar-todas").addEventListener("click", () => quitarTodas());
  function quitarTodas() {
    return confirmar({ titulo: T("restr.quitarTodasTitulo"), texto: T("restr.quitarTexto"), si: T("restr.quitarTodas") }).then((si) => {
      if (si) api.ClearAllRestrictions().then(pintarRestricciones, fallo);
    });
  }

  // Energía
  const ACCION = { shutdown: "energia.apagar", reboot: "energia.reiniciar", logoff: "energia.cerrarSesion" };
  function pintarPanelEnergia() {
    const accion = document.querySelector('input[name="energia-accion"]:checked').value;
    const forzar = $("energia-forzar").checked;
    $("energia-espera").disabled = forzar;
    const objetivo = seleccion.size ? seleccionados().filter((s) => !s.simulated) : conectados().filter((s) => !s.simulated);
    const pueden = objetivo.filter((s) => s.canPower).length;
    $("energia-ejecutar").textContent = T(ACCION[accion]) + "…";
    $("energia-ejecutar").disabled = pueden === 0;
    $("energia-sin-soporte").hidden = pueden === objetivo.length && pueden > 0;
    $("energia-sin-soporte-texto").textContent = pueden === 0 ? T("energia.nadiePuede") : P("energia.algunosNo", objetivo.length - pueden);
  }
  for (const i of $$('input[name="energia-accion"], #energia-forzar')) i.addEventListener("change", pintarPanelEnergia);
  $("energia-ejecutar").addEventListener("click", () => {
    const accion = document.querySelector('input[name="energia-accion"]:checked').value;
    const forzar = $("energia-forzar").checked;
    const espera = Number($("energia-espera").value);
    const objetivo = (seleccion.size ? seleccionados() : conectados()).filter((s) => !s.simulated && s.canPower);
    const nombres = objetivo.map((s) => s.hostname);
    const lista = nombres.length <= 3 ? nombres.join(", ") : P("energia.equipos", nombres.length);
    confirmar({
      titulo: T("energia.confirmarTitulo", { accion: T(ACCION[accion]).toLowerCase(), equipos: P("energia.equipos", nombres.length) }),
      texto: forzar ? T("energia.confirmarForzado", { lista }) : T("energia.confirmarCuenta", { lista, tiempo: mmss(espera * 1000) }),
      consecuencia: forzar ? T("energia.consecuenciaForzado") : T("energia.consecuenciaCuenta"),
      si: T(ACCION[accion]) + " " + P("energia.equipos", nombres.length),
      peligro: true,
    }).then((si) => {
      if (!si) return;
      api.PowerAction({ action: accion, hosts: equiposReales(), delayS: espera, force: forzar }).then((r) => {
        cerrarPanel();
        let texto = "";
        if (r.unsupported.length) texto += T("energia.noLaSoportan", { lista: r.unsupported.join(", ") }) + " ";
        if (r.failed.length) texto += T("energia.noLlego", { lista: r.failed.join(", ") });
        toast(r.failed.length ? "advertencia" : "exito", P("energia.enviada", r.sent), texto.trim());
      }, fallo);
    });
  });

  function insigniaEnergia(p) {
    const n = T(ACCION[p.action] + "Sustantivo");
    switch (p.status) {
      case "sent": return insignia("error", "energia", T("energia.enviadaA", { accion: n }));
      case "countdown":
      case "postponed": {
        const pos = 2 - p.postpones_left;
        return insignia("error", "energia", T("energia.en", { accion: n, tiempo: mmss(p.deadline_ms - Date.now()) }) +
          (pos ? " · " + T("energia.pospuesto", { n: pos }) : ""));
      }
      case "executing": return insignia("error", "energia", T("energia.ejecutando", { accion: n }));
      case "cancelled": return insignia("", "energia", T("energia.cancelada", { accion: n }));
      default: return insignia("error", "error", T("energia.noSePudo", { accion: n }));
    }
  }
  const pendientes = () => conectados().filter((s) => s.power && ["sent", "countdown", "postponed"].includes(s.power.status));
  function pintarEnergia() {
    const p = pendientes();
    $("f-energia").hidden = p.length === 0;
    if (p.length) {
      const primero = Math.min(...p.map((s) => s.power.deadline_ms || Infinity));
      const accion = T(ACCION[p[0].power.action] + "Sustantivo");
      $("f-energia-texto").textContent = P("energia.franja", p.length, { accion }) +
        (Number.isFinite(primero) ? " · " + T("energia.primero", { tiempo: mmss(primero - Date.now()) }) : "");
    }
    if (panel === "energia") pintarPanelEnergia();
  }
  setInterval(() => { if (pendientes().length) { pintarEnergia(); pintarAlumnos(alumnos); } }, 1000);
  $("f-energia-cancelar").addEventListener("click", () => {
    api.PowerCancel([]).then((r) => toast("info", P("energia.canceladaEn", r.sent)), fallo);
  });

  // --- Código ----------------------------------------------------------------------------
  const LANGS = [
    ["python", "Python"], ["java", "Java"], ["c", "C"], ["cpp", "C++"], ["csharp", "C#"],
    ["javascript", "JavaScript"], ["typescript", "TypeScript"], ["go", "Go"], ["sql", "SQL"],
    ["html", "HTML"], ["css", "CSS"], ["shell", "Shell"], ["json", "JSON"], ["rust", "Rust"],
    ["kotlin", "Kotlin"], ["plaintext", "Texto"],
  ];
  $("codigo-lenguaje").replaceChildren(...LANGS.map(([id, nombre]) => Object.assign(el("option", "", nombre), { value: id })));
  let editor = null;
  function iniciarEditor() {
    window.MonacoEnvironment = { getWorkerUrl: () => "vendor/monaco/vs/base/worker/workerMain.js" };
    window.require.config({ paths: { vs: "vendor/monaco/vs" } });
    window.require(["vs/editor/editor.main"], () => {
      editor = window.monaco.editor.create($("editor"), {
        value: "", language: $("codigo-lenguaje").value, theme: esOscuro() ? "vs-dark" : "vs",
        automaticLayout: true, minimap: { enabled: false }, fontSize: 15, tabSize: 4, insertSpaces: true,
        renderWhitespace: "boundary", scrollBeyondLastLine: false,
        ariaLabel: T("codigo.editor"),
      });
      editor.onDidChangeModelContent(actualizarAccionesCodigo);
      actualizarAccionesCodigo();
    });
  }
  function ponerLenguaje(l) {
    $("codigo-lenguaje").value = l;
    if (editor) window.monaco.editor.setModelLanguage(editor.getModel(), l);
  }
  $("codigo-lenguaje").addEventListener("change", () => ponerLenguaje($("codigo-lenguaje").value));
  $("codigo-ayuda-teclado").textContent = T("codigo.tecladoAyuda");

  function pintarDestinoCodigo() {
    const n = seleccion.size;
    $("codigo-destino-todos").textContent = T("destino.todos", { n: conectados().length });
    $("codigo-destino-seleccion").textContent = n ? P("codigo.destinoSeleccion", n) : T("codigo.destinoSinSeleccion");
    const radio = $("codigo-destino-seleccion-radio");
    radio.disabled = n === 0;
    if (n === 0 && radio.checked) document.querySelector('input[name="codigo-destino"][value="todos"]').checked = true;
  }
  const destinoCodigo = () => (document.querySelector('input[name="codigo-destino"]:checked').value === "seleccion" ? destinos() : []);

  function actualizarAccionesCodigo() {
    const hay = editor && editor.getValue().trim() !== "";
    let motivo = "";
    if (alumnos.canCode === 0) motivo = conectados().length === 0 ? T("codigo.motivoSinAlumnos") : T("codigo.motivoNadieRecibe");
    else if (!hay) motivo = T("codigo.motivoVacio");
    $("codigo-emitir").disabled = $("codigo-agregar").disabled = alumnos.canCode === 0 || !hay;
    $("codigo-limpiar").disabled = alumnos.canCode === 0;
    $("codigo-motivo").textContent = motivo;
    $("codigo-ejercicio").textContent = T("codigo.ejercicio", { n: historial.length + 1 });
  }
  function emitir(mode) {
    const b = $(mode === "replace" ? "codigo-emitir" : "codigo-agregar");
    b.setAttribute("aria-busy", "true");
    api.Emit({ title: $("codigo-titulo").value, lang: $("codigo-lenguaje").value, content: editor.getValue(), mode, targets: destinoCodigo() })
      .then((d) => toast("exito", P("codigo.enviado", d.sent + d.simSent, { etiqueta: d.label }), T("codigo.enviadoTexto")), fallo)
      .finally(() => b.removeAttribute("aria-busy"));
  }
  $("codigo-emitir").addEventListener("click", () => emitir("replace"));
  $("codigo-agregar").addEventListener("click", () => emitir("append"));
  $("codigo-pegar").addEventListener("click", () => { api.CaptureClipboard().catch(() => {}); }); // el resultado llega también como evento
  $("codigo-limpiar").addEventListener("click", () => {
    const n = conectados().filter((s) => s.canCode).length;
    confirmar({ titulo: P("codigo.limpiarTitulo", n), texto: T("codigo.limpiarTexto"), consecuencia: T("codigo.limpiarConsecuencia"), si: P("codigo.limpiarSi", n) })
      .then((si) => { if (si) api.ClearScreens().then(() => toast("exito", T("codigo.limpiado")), fallo); });
  });

  function alCapturar(c) {
    if (c.error) {
      fallo(c.error);
      return;
    }
    const d = c.draft;
    location.hash = "codigo";
    if (editor) editor.setValue(d.content);
    ponerLenguaje(d.lang);
    $("codigo-titulo").value = "";
    $("codigo-titulo").focus();
    toast("info", T("codigo.capturado"), P("codigo.capturadoTexto", d.lines, { lenguaje: d.lang }));
  }

  // Envíos: quién lo tiene y quién falta.
  const nombreDe = (id) => {
    const s = alumnoPorSesion(id);
    return s ? s.hostname + (s.username ? " · " + s.username : "") : T("codigo.yaNoEsta");
  };
  const abiertos = new Set(); // envíos con el desglose abierto
  function pintarEnvios(lista) {
    envios = lista;
    $("envios-vacio").hidden = lista.length > 0;
    const tpl = $("t-envio").content.firstElementChild;
    $("envios-lista").replaceChildren(...lista.slice(0, 10).map((d) => {
      const li = tpl.cloneNode(true);
      li.querySelector(".envio__titulo").textContent = d.label;
      const total = d.sent + d.simSent;
      const ok = d.acked + d.simAcked;
      const completo = total > 0 && ok === total;
      const conteo = li.querySelector(".envio__conteo");
      conteo.append(insignia(completo ? "conectado" : "advertencia", completo ? "hecho" : "pendiente", T("codigo.deN", { ok, total })));
      if (d.simSent) conteo.append(insignia("simulado", "simulado", T("codigo.deSimulados", { n: d.simSent })));
      if (d.noCapability) conteo.append(insignia("", "info", P("codigo.noPueden", d.noCapability)));
      const des = li.querySelector(".envio__desplegar");
      const dl = li.querySelector(".desglose");
      const acc = li.querySelector(".envio__acciones");
      const faltan = d.pending || [];
      des.hidden = faltan.length === 0;
      des.querySelector("span").textContent = P("codigo.quienFalta", faltan.length);
      const abierto = abiertos.has(d.id);
      dl.hidden = acc.hidden = !abierto || faltan.length === 0;
      des.setAttribute("aria-expanded", String(abierto));
      dl.replaceChildren(...faltan.flatMap((id) => {
        const s = alumnoPorSesion(id);
        const estado = !s ? insignia("desconectado", "desconectado", T("codigo.yaNoEsta"))
          : s.state !== "connected" ? insignia("desconectado", "desconectado", T("alumnos.desconectado"))
            : insignia("advertencia", "pendiente", T("codigo.sinConfirmar"));
        const dd = el("dd");
        dd.append(estado);
        return [el("dt", "", nombreDe(id)), dd];
      }));
      des.addEventListener("click", () => {
        if (abiertos.has(d.id)) abiertos.delete(d.id); else abiertos.add(d.id);
        pintarEnvios(envios);
      });
      const re = li.querySelector(".envio__reenviar");
      re.querySelector("span").textContent = T("codigo.reenviar");
      re.addEventListener("click", () => {
        api.Resend(d.id).then((r) => toast("info", T("codigo.reenviado"), P("codigo.reenviadoTexto", (r.pending || []).length)), fallo);
      });
      return li;
    }));
  }

  // Historial
  function pintarHistorial(entradas) {
    historial = entradas;
    $("historial-vacio").hidden = entradas.length > 0;
    $("historial-tabla").hidden = entradas.length === 0;
    const tpl = $("t-historial").content.firstElementChild;
    $("historial-filas").replaceChildren(...entradas.slice().reverse().map((e) => {
      const tr = tpl.cloneNode(true);
      const lineas = e.content.replace(/\n$/, "").split("\n").length;
      const lang = LANGS.find(([id]) => id === e.lang);
      const celdas = [["num", String(e.number)], ["titulo", e.title || T("codigo.sinTitulo")], ["lenguaje", lang ? lang[1] : e.lang],
        ["lineas", String(lineas)], ["veces", P("codigo.veces", e.times)], ["hora", e.lastSentAt ? new Date(e.lastSentAt).toLocaleTimeString("es-MX", { hour: "2-digit", minute: "2-digit" }) : ""]];
      const etiquetas = { num: "codigo.colNumero", titulo: "codigo.colTitulo", lenguaje: "codigo.lenguaje", lineas: "codigo.colLineas", veces: "codigo.colVeces", hora: "codigo.colHora" };
      for (const [c, v] of celdas) {
        const td = tr.querySelector("." + c);
        td.textContent = v;
        td.dataset.etiqueta = T(etiquetas[c]);
      }
      const abrir = tr.querySelector(".abrir");
      abrir.querySelector("span").textContent = T("codigo.abrirEditor");
      abrir.addEventListener("click", () => {
        $("codigo-titulo").value = e.title;
        ponerLenguaje(e.lang);
        if (editor) { editor.setValue(e.content); editor.focus(); }
      });
      const re = tr.querySelector(".reemitir");
      re.textContent = T("codigo.reemitir");
      re.addEventListener("click", () => {
        const n = conectados().filter((s) => s.canCode).length;
        confirmar({ titulo: T("codigo.reemitirTitulo", { n: e.number }), texto: P("codigo.reemitirTexto", n), si: T("codigo.reemitir") })
          .then((si) => { if (si) api.Reemit(e.snippetId).then((d) => toast("exito", P("codigo.enviado", d.sent + d.simSent, { etiqueta: d.label })), fallo); });
      });
      return tr;
    }));
    actualizarAccionesCodigo();
  }

  // --- Aula simulada ---------------------------------------------------------------------
  const FALLAS = [["none", "simulada.fallaNinguna"], ["slow", "simulada.fallaLenta"], ["noack", "simulada.fallaNoAcusa"], ["disconnected", "simulada.fallaDesconectado"]];
  const CAPS = { "code.view": "simulada.capCodigo", "msg.show": "simulada.capAvisos", "screen.thumb": "simulada.capPantalla" };
  function pintarSimulacion(v) {
    simulada = v;
    $("f-simulada").hidden = !v.active;
    $("f-simulada-texto").textContent = P("simulada.franja", v.students.length);
    if (vista === "simulada") pintarSimulada();
  }
  function pintarSimulada() {
    const v = simulada;
    $("simulada-cerrada").hidden = v.active;
    $("simulada-abierta").hidden = !v.active;
    if (!$("sim-capacidades").childElementCount) {
      $("sim-capacidades").replaceChildren(...(v.capabilities.length ? v.capabilities : Object.keys(CAPS)).map((c) => {
        const l = el("label", "opcion");
        const i = Object.assign(el("input"), { type: "checkbox", value: c, checked: true });
        l.append(i, document.createTextNode(" " + T(CAPS[c] || c)));
        return l;
      }));
    }
    const tpl = $("t-fila-simulado").content.firstElementChild;
    $("sim-filas").replaceChildren(...v.students.map((s) => {
      const tr = tpl.cloneNode(true);
      tr.querySelector(".nombre").textContent = s.name;
      tr.querySelector(".detalle").textContent = { windows: "Windows", linux: "Linux", darwin: "macOS" }[s.os] + " · " +
        s.capabilities.map((c) => T(CAPS[c] || c)).join(", ");
      tr.querySelector(".estado").append(s.connected ? insignia("conectado", "conectado", T("alumnos.conectado")) : insignia("desconectado", "desconectado", T("alumnos.desconectado")));
      if (s.viewed) tr.querySelector(".estado").append(" ", insignia("advertencia", "pantalla", T("simulada.vista")));
      const p = tr.querySelector(".pantalla");
      if (s.snippets.length) {
        p.append(document.createTextNode(s.snippets.map((sn) => T("codigo.ejercicio", { n: sn.exercise }) + (sn.title ? " · " + sn.title : "")).join(", ")));
      } else p.append(el("span", "suave", T("simulada.nada")));
      if (s.notices) p.append(el("span", "celda-sub", P("simulada.avisos", s.notices)));
      const f = tr.querySelector(".falla");
      f.setAttribute("aria-label", T("simulada.fallaDe", { nombre: s.name }));
      f.replaceChildren(...FALLAS.map(([val, k]) => Object.assign(el("option", "", T(k)), { value: val })));
      f.value = s.fault;
      f.addEventListener("change", () => api.SimulationFault(s.id, f.value, 3000).then(pintarSimulacion, fallo));
      const q = tr.querySelector(".quitar");
      q.textContent = T("simulada.quitar");
      q.setAttribute("aria-label", T("simulada.quitarA", { nombre: s.name }));
      q.addEventListener("click", () => api.SimulationRemove(s.id).then(pintarSimulacion, fallo));
      return tr;
    }));
  }
  for (const b of $$("[data-abrir-simulada]")) {
    b.addEventListener("click", () => {
      b.setAttribute("aria-busy", "true");
      api.SimulationStart(0).then((v) => {
        pintarSimulacion(v);
        toast("exito", P("simulada.abierta", v.students.length), T("simulada.abiertaTexto"));
        if (vista !== "simulada") location.hash = "alumnos";
      }, fallo).finally(() => b.removeAttribute("aria-busy"));
    });
  }
  $("sim-form").addEventListener("submit", (e) => {
    e.preventDefault();
    const caps = $$("#sim-capacidades input:checked").map((i) => i.value);
    api.SimulationAdd({ os: $("sim-so").value, capabilities: caps }).then(pintarSimulacion, fallo);
  });
  const cerrarSimulada = () => confirmar({ titulo: T("simulada.cerrarTitulo"), texto: T("simulada.cerrarTexto"), si: T("simulada.salir") })
    .then((si) => { if (si) api.SimulationStop().then(pintarSimulacion, fallo); });
  $("sim-cerrar").addEventListener("click", cerrarSimulada);
  $("f-simulada-salir").addEventListener("click", cerrarSimulada);

  // --- Ayuda y «¿Falta un alumno?» -----------------------------------------------------------
  // El manual completo se genera de docs/ (entrega del manual); por ahora el
  // índice muestra lo que ya existe y lo que viene.
  const INDICE = [
    ["ayuda.primerosPasos", "flecha-der", [["ayuda.pAbrir"], ["ayuda.pEnviar"], ["ayuda.pSimulada", "#simulada"]]],
    ["ayuda.problemas", "advertencia", [["falta.titulo", "#falta"], ["ayuda.pConsolaNoAbre"], ["ayuda.pFirewall"]]],
    ["ayuda.conceptos", "info", [["ayuda.pCanal"], ["ayuda.pClave"], ["ayuda.pSimulados"]]],
    ["ayuda.comoFunciona", "red", [["ayuda.pDiagramas"], ["ayuda.pPrivacidad"]]],
  ];
  const indice = $("ayuda-indice");
  indice.replaceChildren(...INDICE.map(([titulo, ic, items]) => {
    const li = el("li", "tarjeta");
    const h = el("h2");
    h.append(icono(ic), document.createTextNode(T(titulo)));
    const ul = el("ul");
    ul.append(...items.map(([k, href]) => {
      const i = el("li");
      if (href) i.append(Object.assign(el("a", "", T(k)), { href }));
      else i.append(document.createTextNode(T(k) + " "), el("span", "suave", T("ayuda.proximamente")));
      return i;
    }));
    li.append(h, ul);
    return li;
  }));

  const RESPUESTAS = [["falta.rBuscando", "falta.cBuscando"], ["falta.rSinConfigurar", "falta.cSinConfigurar"],
    ["falta.rNoConecta", "falta.cNoConecta"], ["falta.rClave", "falta.cClave"], ["falta.rCertificado", "falta.cCertificado"],
    ["falta.rNoVeo", "falta.cNoVeo"]];
  $("falta-respuestas").replaceChildren(...RESPUESTAS.map(([r, c]) => {
    const b = boton(T(r), "btn--sm");
    b.addEventListener("click", () => {
      $("falta-causa").hidden = false;
      $("falta-causa-titulo").textContent = T(c + ".titulo");
      $("falta-causa-texto").textContent = T(c + ".texto");
    });
    return b;
  }));
  function datosInstalador(i) {
    return [T("instalador.canal", { v: i.channel }), T("instalador.equipo", { v: i.hostname + (i.addresses.length ? " (" + i.addresses.join(", ") + ")" : "") }),
      T("instalador.puertos", { tcp: i.port, udp: i.mediaPort }), T("instalador.clave", { v: i.fingerprint }),
      T("instalador.version", { v: i.version })].join("\n");
  }
  function pintarFalta() {
    api.InstallerInfo().then((i) => {
      $("falta-datos").textContent = datosInstalador(i);
      $("falta-paso1-texto").textContent = T("falta.paso1Texto", { canal: i.channel, ip: i.addresses[0] || i.hostname });
    });
  }

  // --- Ajustes ---------------------------------------------------------------------------
  const pestanas = $$("#ajustes [role=tab]");
  function elegirPestana(t, foco) {
    for (const o of pestanas) {
      const on = o === t;
      o.setAttribute("aria-selected", String(on));
      o.tabIndex = on ? 0 : -1;
      $(o.getAttribute("aria-controls")).hidden = !on;
    }
    if (foco) t.focus();
  }
  pestanas.forEach((t, i) => {
    t.addEventListener("click", () => elegirPestana(t, true));
    t.addEventListener("keydown", (e) => {
      if (e.key !== "ArrowDown" && e.key !== "ArrowUp") return;
      e.preventDefault();
      elegirPestana(pestanas[(i + (e.key === "ArrowDown" ? 1 : pestanas.length - 1)) % pestanas.length], true);
    });
  });
  function abrirAjustes(pestana) {
    $("aj-nombre").textContent = T("ajustes.nombre", { nombre: clase.consoleName });
    $("aj-canal").textContent = T("ajustes.canal", { canal: clase.channel });
    $("aj-canal-inicio").replaceChildren(icono("candado"), document.createTextNode(T("canal.reciente", { canal: setup.defaultChannel })));
    $("aj-version").textContent = T("arranque.version", { version: setup.version });
    const centro = clase.keySource === "centro";
    $("aj-clave-titulo").textContent = T(centro ? "clave.centroTitulo" : "clave.personalTitulo");
    $("aj-clave-texto").textContent = T(centro ? "clave.centroTexto" : "clave.personalTexto");
    $("aj-clave").className = "insignia " + (centro ? "insignia--conectado" : "insignia--advertencia");
    $("aj-clave-huella").textContent = clase.fingerprint;
    $("aj-atajo").textContent = hotkey.active ? T("ajustes.atajoActivo", { combo: hotkey.combo }) : (hotkey.error || T("ajustes.atajoInactivo"));
    api.InstallerInfo().then((i) => { $("aj-datos").textContent = datosInstalador(i); });
    aplicarTema(tema);
    elegirPestana($("tab-" + (pestana || "clase")), false);
    $("ajustes").showModal();
  }
  $("nav-ajustes").addEventListener("click", () => abrirAjustes());
  $("f-clave-que-hago").addEventListener("click", () => abrirAjustes("acerca"));

  // Cambiar de canal o de nombre: reabre la clase, con la consecuencia dicha
  // antes. El historial de código se conserva.
  function abrirCambiarCanal() {
    $("cc-canal").value = "";
    $("cc-error").hidden = true;
    $("cc-canal").removeAttribute("aria-invalid");
    $("cc-recientes").replaceChildren(el("span", "suave pequeno", T("canal.recientes")), ...setup.recentChannels
      .filter((c) => c !== clase.channel).slice(0, 5).map((c) => {
        const b = boton(T("canal.reciente", { canal: c }), "btn--sm");
        b.addEventListener("click", () => { $("cc-canal").value = c; actualizarCanal(); });
        return b;
      }));
    actualizarCanal();
    $("cambiar-canal").showModal();
    $("cc-canal").focus();
  }
  function actualizarCanal() {
    const c = $("cc-canal").value.trim() || "…";
    $("cc-consecuencia").textContent = T("canal.consecuencia", { n: alumnos.connected, actual: clase.channel, nuevo: c });
    $("cc-si").textContent = T("canal.si", { canal: c });
  }
  $("cc-canal").addEventListener("input", actualizarCanal);
  $("cc-form").addEventListener("submit", (e) => {
    e.preventDefault();
    const c = $("cc-canal").value.trim();
    const n = Number(c);
    const err = $("cc-error");
    if (!/^[1-9][0-9]{0,4}$/.test(c) || n > 16000 || c === clase.channel) {
      err.hidden = false;
      err.replaceChildren(icono("error", "icono--sm"), document.createTextNode(c === clase.channel ? T("canal.mismo") : T("canal.invalido")));
      $("cc-canal").setAttribute("aria-invalid", "true");
      $("cc-canal").focus();
      return;
    }
    reabrir(c, clase.consoleName, $("cambiar-canal"));
  });
  $("nav-clase").addEventListener("click", abrirCambiarCanal);
  $("nav-clase-compacta").addEventListener("click", abrirCambiarCanal);
  $("aj-cambiar-canal").addEventListener("click", () => { $("ajustes").close(); abrirCambiarCanal(); });

  $("aj-cambiar-nombre").addEventListener("click", () => {
    $("ajustes").close();
    $("cn-nombre").value = clase.consoleName;
    $("cn-error").hidden = true;
    $("cambiar-nombre").showModal();
    $("cn-nombre").select();
  });
  $("cn-form").addEventListener("submit", (e) => {
    e.preventDefault();
    const nombre = $("cn-nombre").value.trim();
    if (!nombre || nombre === clase.consoleName) {
      if (!nombre) {
        $("cn-error").hidden = false;
        $("cn-error").replaceChildren(icono("error", "icono--sm"), document.createTextNode(T("nombre.vacio")));
        return;
      }
      $("cambiar-nombre").close();
      return;
    }
    reabrir(clase.channel, nombre, $("cambiar-nombre"));
  });
  function reabrir(canal, nombre, dialogo) {
    const b = dialogo.querySelector('button[type="submit"]');
    b.setAttribute("aria-busy", "true");
    if (modo === "miniaturas") ponerModo("lista");
    api.StartClass(canal, nombre).then((v) => {
      dialogo.close();
      mostrarConsola(v);
      toast("exito", T("canal.listo", { canal: v.class.channel }), T("canal.listoTexto"));
    }, fallo).finally(() => b.removeAttribute("aria-busy"));
  }

  // --- Eventos del backend -----------------------------------------------------------------
  rt.EventsOn(ev.STUDENTS, pintarAlumnos);
  rt.EventsOn(ev.HISTORY, (h) => pintarHistorial(h.entries));
  rt.EventsOn(ev.DELIVERIES, (d) => pintarEnvios(d.deliveries));
  rt.EventsOn(ev.CAPTURE, alCapturar);
  rt.EventsOn(ev.FRAME, alCuadro);
  rt.EventsOn(ev.BROADCAST, pintarTransmision);
  rt.EventsOn(ev.RESTRICTIONS, pintarRestricciones);
  rt.EventsOn(ev.SIMULATION, pintarSimulacion);
  rt.EventsOn(ev.CLASS, (v) => (v.active ? mostrarConsola(v) : mostrarArranque()));

  // --- Arranque de la interfaz ---------------------------------------------------------------
  aplicarTextos(document);
  for (const tpl of $$("template")) aplicarTextos(tpl.content);
  aplicarTema(tema);
  for (const b of $$("[data-modo]")) b.setAttribute("aria-pressed", String(b.dataset.modo === modo));
  api.View().then((v) => (v.active ? mostrarConsola(v) : mostrarArranque()));
})();
