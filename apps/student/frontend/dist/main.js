// Interfaz de la app del alumno. Recibe el estado del backend Go por eventos
// de Wails y llama a sus métodos (View, Copy, DismissNotice y los de la
// transmisión del profesor: ReportMedia, BroadcastAnswer, BroadcastCandidate,
// BroadcastFailed; los del aviso de energía: PowerPostpone, DismissPower; y los
// de la mano y los mensajes al profesor: RaiseHand, LowerHand, SendTeacherMessage; y
// «Abrir carpeta» de los archivos recibidos y de las recolecciones: OpenReceivedFolder,
// OpenCollectionFolder; y la respuesta a una encuesta: AnswerPoll, y a un cuestionario: AnswerQuiz).
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
  let bcSource = "";            // el compañero cuya pantalla se ve (§5.24); "" si es la del profesor
  let presentV = { phase: "" }; // tu pantalla mostrada a la clase (reglas 75-82)

  const TI = "titulo."; // + estado, o pendiente.<acción>
  function title() {
    const v = { canal: status.channel, profesor: status.teacher };
    let t = status.control ? T("titulo.control", v) : status.paused ? T("titulo.pausa", v)
      : status.presenting ? T("titulo.mostrando", v)
      : status.viewed ? T("titulo.vista", v)
      : status.watching ? (bcSource ? T("titulo.recibiendoAlumno", { equipo: bcSource, canal: status.channel }) : T("titulo.recibiendo", v))
      : T(TI + status.state, v);
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
    const antes = status.control || "";
    const antesPausa = !!status.paused;
    status = s;
    const v = { canal: s.channel, profesor: s.teacher };
    $("estado").dataset.estado = s.state;
    $("estado-texto").textContent = T(ES + s.state, v);
    $("estado-detalles").hidden = !s.detail;
    $("estado-detalle").textContent = s.detail || "";
    // Pantalla en pausa (reglas 9-10): quién la pausó; se va sola cuando la reanuda o se pierde la clase.
    $("banda-pausa").hidden = !s.paused;
    $("banda-pausa-texto").textContent = s.paused ? T("banda.pausa", v) : "";
    if (s.paused && !antesPausa) salirDeCompacto(); // actúa sobre el equipo: sale del modo compacto
    $("banda-vista").hidden = !s.viewed;
    $("banda-vista-texto").textContent = s.viewed ? T("banda.vista", v) : "";
    // Control remoto (reglas 37-41): la banda dice quién y, en pausa, por qué.
    $("banda-control").hidden = !s.control;
    const BC = "banda.control."; // + Status.Control (active, paused)
    $("banda-control-texto").textContent = s.control ? T(BC + s.control, v) : "";
    if (s.control && !antes) { ocultarFinControl(); salirDeCompacto(); }
    if (!s.control && antes) mostrarFinControl(v);
    pintarVacio();
    title();
    pintarCompacto();
    pintarMano();
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
  // Lo que el alumno aún no ve: código y archivos «Nuevo» y avisos abiertos.
  const sinVer = () => list.querySelectorAll(".nuevo:not([hidden])").length + archivosNuevos() + recoNuevas() + encuestasNuevas() + avisos.length
    + ($("tarea-nuevo").hidden ? 0 : 1) + ($("fin-clase-nuevo").hidden ? 0 : 1);
  function pintarCompacto() {
    const v = { canal: status.channel, profesor: status.teacher };
    $("compacto").dataset.estado = status.state;
    $("compacto-texto").textContent = status.state === "connected" ? T("compacto.conectado", v) : T(ES + status.state, v);
    const n = sinVer();
    $("compacto-nuevos").hidden = n === 0;
    $("compacto-nuevos").textContent = n ? P("compacto.nuevos", n) : "";
    $("compacto").dataset.nuevo = String(n > 0);
    $("compacto-control").hidden = !status.control;
    $("compacto-pausa").hidden = !status.paused;
    // El control implica la pantalla vista: una insignia basta. Mostrarla a la clase lleva la suya.
    $("compacto-mostrando").hidden = !status.presenting;
    $("compacto-vista").hidden = !status.viewed || !!status.control || !!status.presenting;
    pintarMano();
  }
  function ponerCompacto(on, avisarAGo) {
    compacto = on;
    document.body.classList.toggle("compacto", on);
    $("compacto").hidden = !on;
    pintarCompacto();
    if (avisarAGo) api.SetCompact(on).catch(() => {});
    $(on ? "restaurar" : "compactar").focus();
  }
  // Lo que el profesor hace sobre el equipo (energía, restricciones, su
  // transmisión) no cabe en la tira: la ventana vuelve a su tamaño. Go también
  // lo hace al traerla al frente; esto cubre el caso en que ya estaba al frente.
  function salirDeCompacto() { if (compacto) ponerCompacto(false, true); }
  $("compactar").addEventListener("click", () => ponerCompacto(true, true));
  $("restaurar").addEventListener("click", () => ponerCompacto(false, true));

  // --- Código ---------------------------------------------------------------------------------
  const list = $("snippets");
  const empty = $("empty");
  const snippetTpl = $("snippet-template");
  // El estado vacío se va con el primer código o el primer archivo.
  let nCodigos = 0;
  let nRecolecciones = 0;
  function pintarListaVacia() { empty.hidden = nCodigos + nArchivos + nRecolecciones + nEncuestas > 0 || !$("tarea").hidden; }

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
    nCodigos = snippets.length;
    pintarListaVacia();
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
  // Avisos que arma la ventana a partir del tipo que manda Go (regla 44: «Captura de pantalla»).
  const TIPO = {
    snapshot: { icono: "captura", titulo: "aviso.snapshot.titulo", texto: "aviso.snapshot.texto" },
  };

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
      const propio = TIPO[n.kind]; // un aviso del propio Aula (sin texto del profesor)
      node.querySelector("use").setAttribute("href", "sistema/iconos.svg#" + (propio ? propio.icono : icono));
      const AV = "aviso."; // + nivel del aviso (info, warning)
      if (n.kind === "opened") {
        // El profesor abrió una página o una app (privacidad, regla 66): qué y quién.
        node.dataset.kind = n.kind;
        const o = abiertos.find((x) => x.id === n.ref);
        const app = o && o.kind === "app";
        node.querySelector("use").setAttribute("href", "sistema/iconos.svg#" + (app ? "app" : "externo"));
        node.querySelector(".toast__titulo").textContent = T(app ? "aviso.abierto.app.titulo" : "aviso.abierto.pagina.titulo");
        node.querySelector(".notice-text").textContent = o
          ? T(app ? "aviso.abierto.app.texto" : "aviso.abierto.pagina.texto", { profesor: o.teacher, canal: o.channel, que: o.shown, hora: hora(o.at) })
          : "";
        if (o) node.title = o.full;
      } else if (n.kind === "collect-requested" || n.kind === "collect-done") {
        node.dataset.kind = n.kind;
        node.querySelector("use").setAttribute("href", "sistema/iconos.svg#carpeta");
        const a = avisoReco(n);
        node.querySelector(".toast__titulo").textContent = a.titulo;
        node.querySelector(".notice-text").textContent = a.texto;
      } else if (propio) {
        node.dataset.kind = n.kind;
        node.querySelector(".toast__titulo").textContent = T(propio.titulo);
        node.querySelector(".notice-text").textContent = T(propio.texto, {
          profesor: status.teacher, canal: status.channel, hora: hora(n.at) });
      } else if (n.replyTo) {
        // La respuesta del profesor a un mensaje del alumno, con su cita.
        node.querySelector("use").setAttribute("href", "sistema/iconos.svg#mensaje");
        node.querySelector(".toast__titulo").textContent = T("aviso.respuesta");
        node.querySelector(".notice-text").textContent = n.text;
        const propio = participacion.messages.find((m) => m.messageId === n.replyTo);
        const cita = node.querySelector(".notice-cita");
        cita.hidden = !propio;
        cita.textContent = propio ? T("aviso.tuMensaje", { texto: propio.text }) : "";
      } else if (n.personal) {
        // Un mensaje privado del profesor (prompt 10.7): solo le llegó a este alumno.
        node.dataset.personal = "true";
        node.querySelector("use").setAttribute("href", "sistema/iconos.svg#sobre");
        node.querySelector(".toast__titulo").textContent = T("aviso.personal");
        node.querySelector(".notice-text").textContent = n.text;
      } else {
        node.querySelector(".toast__titulo").textContent = T(AV + (n.level === "warning" ? "warning" : "info"));
        node.querySelector(".notice-text").textContent = n.text;
      }
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

  // Fin del control remoto (regla 41): un aviso en la esquina, sin quitar el foco. Se va
  // con «Entendido», con Esc o solo, a los 15 s.
  let finControl = 0;
  function mostrarFinControl(v) {
    $("control-fin-texto").textContent = T("control.finTexto", v);
    $("control-fin").hidden = false;
    clearTimeout(finControl);
    finControl = setTimeout(ocultarFinControl, 15000);
  }
  function ocultarFinControl() {
    clearTimeout(finControl);
    $("control-fin").hidden = true;
  }
  $("control-fin-ok").addEventListener("click", ocultarFinControl);
  $("control-fin").addEventListener("keydown", (e) => { if (e.key === "Escape") { e.stopPropagation(); ocultarFinControl(); } });

  // --- Archivos que envió el profesor (docs/protocol.md §5.20) ---------------------------------------
  // Una tarjeta por envío, como el código: qué llegó y dónde quedó (reglas 55-58). Go manda el
  // estado (filepush.Received) y aquí se arma el texto. «Abrir carpeta» pasa solo el id del envío:
  // la carpeta la decide Go. Nunca se abre ni se ejecuta un archivo. Lo que llega con la ventana
  // abierta queda «Nuevo» hasta que el alumno lo toca o abre su carpeta.
  const archivosLista = $("archivos");
  const archivoTpl = $("archivo-template");
  const nodosArchivo = new Map(); // pushId → li
  let archivosPrimeraVez = true;
  let nArchivos = 0;
  let carpetaRaiz = "Aula";
  // Tarjetas de archivos que el alumno aún no ve.
  function archivosNuevos() { return archivosLista.querySelectorAll(".nuevo:not([hidden])").length; }
  const MOTIVO = "archivo.motivo."; // + protocol.FilePushReason en camelCase
  const camel = (s) => String(s).replace(/_([a-z])/g, (_, c) => c.toUpperCase());
  const ultimoDe = (ruta) => ruta.split("/").pop();
  const rutaWindows = (folder) => [carpetaRaiz].concat(folder ? folder.split("/") : []).join("\\");

  function nodoArchivo(r, nuevo) {
    const li = archivoTpl.content.firstElementChild.cloneNode(true);
    li.dataset.push = r.pushId;
    li.tabIndex = -1;
    li.querySelector(".nuevo").textContent = T("codigo.nuevo");
    li.querySelector(".nuevo").hidden = !nuevo;
    const visto = () => { li.querySelector(".nuevo").hidden = true; title(); };
    li.addEventListener("focusin", visto);
    li.addEventListener("pointerdown", visto);
    const abrir = li.querySelector(".abrir");
    abrir.querySelector("span").textContent = T("archivo.abrir");
    abrir.addEventListener("click", () => {
      visto();
      const error = li.querySelector(".archivo-recibido__error");
      error.hidden = true;
      api.OpenReceivedFolder(r.pushId).catch((err) => {
        error.textContent = T("archivo.noSeAbrio", { motivo: String(err && err.message ? err.message : err) });
        error.hidden = false;
      });
    });
    return li;
  }

  function pintarArchivo(li, r) {
    const guardados = r.items.filter((i) => !i.reason);
    const primero = r.items[0] || { name: "" };
    let titulo;
    if (r.files === 1) titulo = guardados.length ? T("archivo.titulo.uno", { nombre: primero.name }) : T("archivo.tituloFallo", { nombre: primero.name });
    else titulo = r.label ? T("archivo.tituloEtiqueta", { etiqueta: r.label }) : P("archivo.titulo", r.files);
    li.querySelector(".archivo-titulo").textContent = titulo;
    li.querySelector(".archivo-hora").textContent = hora(r.at);
    const carpeta = T("archivo.carpeta", { ruta: rutaWindows(r.folder) });
    let donde = guardados.length ? P("archivo.guardado", guardados.length, { carpeta }) : T("archivo.nadaGuardado");
    if (r.items.length < r.files) donde += " " + T("archivo.llegando", { n: r.items.length, total: r.files });
    li.querySelector(".archivo-recibido__donde").textContent = donde;
    // La lista dice cada archivo cuando son varios, y siempre lo que no salió como se esperaba.
    const lista = li.querySelector(".archivo-recibido__lista");
    lista.replaceChildren();
    for (const it of r.items) {
      const final = it.path ? ultimoDe(it.path) : it.name;
      let texto, estado = "saved";
      if (it.reason) { estado = "failed"; texto = T("archivo.itemFallo", { nombre: it.name, motivo: T(MOTIVO + camel(it.reason)) }); }
      else if (it.renamed) { estado = "renamed"; texto = T("archivo.itemRenombrado", { nombre: it.name, final }); }
      else if (it.replaced) { estado = "replaced"; texto = T("archivo.itemReemplazado", { nombre: it.name }); }
      else if (r.files > 1) texto = T("archivo.item", { nombre: final });
      else continue;
      const item = document.createElement("li");
      item.dataset.estado = estado;
      item.textContent = texto;
      lista.appendChild(item);
    }
    const abrir = li.querySelector(".abrir");
    abrir.hidden = guardados.length === 0;
    abrir.setAttribute("aria-label", T("archivo.abrirDe", { titulo }));
  }

  function renderArchivos(f) {
    if (!f) return;
    carpetaRaiz = f.folder || "Aula";
    const recibidos = f.received || [];
    const vivos = new Set(recibidos.map((r) => r.pushId));
    for (const [id, li] of nodosArchivo) {
      if (!vivos.has(id)) { li.remove(); nodosArchivo.delete(id); }
    }
    let anterior = null;
    for (const r of recibidos) { // lo más nuevo primero
      let li = nodosArchivo.get(r.pushId);
      if (!li) {
        li = nodoArchivo(r, !archivosPrimeraVez);
        nodosArchivo.set(r.pushId, li);
      }
      pintarArchivo(li, r);
      const lugar = anterior ? anterior.nextSibling : archivosLista.firstChild;
      if (li !== lugar) archivosLista.insertBefore(li, lugar);
      anterior = li;
    }
    archivosPrimeraVez = false;
    nArchivos = recibidos.length;
    pintarListaVacia();
    title();
    pintarCompacto();
  }

  // --- Recolecciones del profesor (docs/protocol.md §5.21) -------------------------------------------
  // Una tarjeta por recolección: dónde guardar el ejercicio y, por ronda, qué se recogió y cuándo
  // (reglas 61-62). Go manda el estado (collect.Collection) y aquí se arma el texto. «Abrir carpeta»
  // pasa solo el id: la carpeta de la actividad la decide Go. Nada aquí lee ni cambia archivos.
  const recoLista = $("recolecciones");
  const recoTpl = $("recoleccion-template");
  const nodosReco = new Map(); // id → li
  let recolecciones = [];
  let recoPrimeraVez = true;
  let recoRaiz = "Aula";
  const MOTIVO_R = "recoleccion.motivo."; // + protocol.FileCollectReason en camelCase
  const carpetaReco = (c) => T("recoleccion.carpeta", { ruta: [recoRaiz].concat(c.folder.split("/")).join("\\") });
  function recoNuevas() { return recoLista.querySelectorAll(".nuevo:not([hidden])").length; }
  function tamano(n) {
    if (n < 1024) return T("tamano.b", { n });
    if (n < 1024 * 1024) return T("tamano.kb", { n: Math.max(1, Math.round(n / 1024)) });
    return T("tamano.mb", { n: (n / 1048576).toFixed(1).replace(/\.0$/, "") });
  }
  const motivoReco = (r) => T(MOTIVO_R + camel(r));
  const rutaReco = (p) => p.split("/").join("\\");

  function nodoReco(c, nuevo) {
    const li = recoTpl.content.firstElementChild.cloneNode(true);
    aplicarTextos(li);
    li.dataset.recoleccion = c.id;
    li.tabIndex = -1;
    li.querySelector(".nuevo").textContent = T("codigo.nuevo");
    li.querySelector(".nuevo").hidden = !nuevo;
    const visto = () => { li.querySelector(".nuevo").hidden = true; title(); };
    li.addEventListener("focusin", visto);
    li.addEventListener("pointerdown", visto);
    li.querySelector(".abrir").addEventListener("click", () => {
      visto();
      const error = li.querySelector(".archivo-recibido__error");
      error.hidden = true;
      api.OpenCollectionFolder(c.id).catch((err) => {
        error.textContent = T("archivo.noSeAbrio", { motivo: String(err && err.message ? err.message : err) });
        error.hidden = false;
      });
    });
    return li;
  }

  function pintarReco(li, c) {
    const titulo = T("recoleccion.titulo", { actividad: c.activity });
    li.querySelector(".recoleccion-titulo").textContent = titulo;
    li.querySelector(".recoleccion-hora").textContent = hora(c.openedAt);
    li.querySelector(".recoleccion-donde").textContent = T("recoleccion.guarda", { carpeta: carpetaReco(c) });
    const pats = c.patterns || [];
    li.querySelector(".recoleccion-patrones").textContent = pats.length ? T("recoleccion.patrones", { patrones: pats.join(", ") }) : T("recoleccion.todo");
    const rondas = c.rounds || [];
    const ultima = rondas[rondas.length - 1];
    const estado = li.querySelector(".recoleccion-estado");
    li.dataset.estado = c.state;
    if (c.state === "requested" && ultima) estado.textContent = T("recoleccion.recogiendo", { hora: hora(ultima.at) });
    else if (!ultima) estado.textContent = T("recoleccion.abierta");
    else estado.textContent = "";
    estado.hidden = !estado.textContent;
    // Cada ronda terminada, lo más nuevo primero: qué se recogió y qué no (regla 62).
    const lista = li.querySelector(".recoleccion-lista");
    lista.replaceChildren();
    const agregar = (texto, est) => {
      const item = document.createElement("li");
      item.dataset.estado = est;
      item.textContent = texto;
      lista.appendChild(item);
    };
    for (const r of rondas.slice().reverse()) {
      if (!r.doneAt || r.doneAt.startsWith("0001")) continue; // en curso
      const files = r.files || [];
      const h = hora(r.doneAt);
      if (r.failed) agregar(T("recoleccion.rondaFallo", { hora: h, motivo: motivoReco(r.failed) }), "failed");
      else if (!files.length) agregar(T("recoleccion.rondaVacia", { hora: h }), "empty");
      else agregar(P("recoleccion.ronda", files.length, { hora: h }), "round");
      for (const f of files) {
        if (f.saved) agregar(T("recoleccion.archivo", { ruta: rutaReco(f.path), tamano: tamano(f.size) }), "saved");
        else agregar(T("recoleccion.archivoFallo", { ruta: rutaReco(f.path), motivo: motivoReco(f.reason) }), "failed");
      }
      for (const s of r.skipped || []) agregar(T("recoleccion.omitido", { ruta: rutaReco(s.path), motivo: motivoReco(s.reason) }), "skipped");
    }
    li.querySelector(".abrir").setAttribute("aria-label", T("recoleccion.abrirDe", { titulo }));
  }

  function renderRecolecciones(v) {
    if (!v) return;
    recoRaiz = v.folder || "Aula";
    recolecciones = v.list || [];
    const vivos = new Set(recolecciones.map((c) => c.id));
    for (const [id, li] of nodosReco) {
      if (!vivos.has(id)) { li.remove(); nodosReco.delete(id); }
    }
    let anterior = null;
    for (const c of recolecciones.slice().reverse()) { // lo más nuevo primero
      let li = nodosReco.get(c.id);
      if (!li) {
        li = nodoReco(c, !recoPrimeraVez);
        nodosReco.set(c.id, li);
      }
      pintarReco(li, c);
      const lugar = anterior ? anterior.nextSibling : recoLista.firstChild;
      if (li !== lugar) recoLista.insertBefore(li, lugar);
      anterior = li;
    }
    recoPrimeraVez = false;
    nRecolecciones = recolecciones.length;
    pintarListaVacia();
    renderNotices(avisos); // los avisos de una recolección toman de aquí la carpeta y lo recogido
    title();
    pintarCompacto();
  }

  // --- Encuestas del profesor (docs/protocol.md §5.23) ---------------------------------------------
  // Una tarjeta por encuesta, arriba de la lista, con «Nuevo» (regla 71): nunca un diálogo ni el
  // foco. Las opciones son radios (flechas para moverse, Enter envía) y la respuesta se puede
  // cambiar mientras siga abierta. Go manda el estado (poll.BallotView) y aquí se arma el texto.
  const encuestasLista = $("encuestas");
  const encuestaTpl = $("encuesta-template");
  const nodosEncuesta = new Map(); // pollId → li
  let encuestasPrimeraVez = true;
  let nEncuestas = 0;
  const EV = "encuesta.voto."; // + poll.VoteState
  const EE = "encuesta.error."; // + código de apps/student/poll.go
  const ET = "encuesta.tipo."; // + protocol.PollKind
  function encuestasNuevas() { return encuestasLista.querySelectorAll(".nuevo:not([hidden])").length; }
  const errorEncuesta = (err) => {
    const m = /aula:(\w+)/.exec(String(err && err.message ? err.message : err));
    return m && typeof TX[EE + m[1]] === "string" ? T(EE + m[1]) : String(err);
  };
  // Las opciones que ve el alumno: las del profesor, o «Sí» y «No».
  const opcionesDe = (c) => (c.kind === "yesno" ? [T("encuesta.si"), T("encuesta.no")] : c.options || []);
  const respuestaDe = (c) => (c.kind === "open" ? c.text : opcionesDe(c)[c.choice] || "");

  function nodoEncuesta(c, nuevo) {
    const li = encuestaTpl.content.firstElementChild.cloneNode(true);
    aplicarTextos(li);
    li.dataset.encuesta = c.pollId;
    li.tabIndex = -1;
    const marca = li.querySelector(".nuevo");
    marca.textContent = T("codigo.nuevo");
    marca.hidden = !nuevo;
    const visto = () => { if (!marca.hidden) { marca.hidden = true; title(); pintarCompacto(); } };
    li.addEventListener("focusin", visto);
    li.addEventListener("pointerdown", visto);
    const form = li.querySelector(".encuesta-form");
    const error = li.querySelector(".encuesta-error");
    const campo = li.querySelector(".encuesta-texto input");
    const ayuda = li.querySelector(".encuesta-ayuda");
    const contar = () => { ayuda.textContent = T("encuesta.ayuda", { n: [...campo.value].length }); };
    campo.addEventListener("input", () => { contar(); error.hidden = true; });
    contar();
    // Las opciones se arman una vez: así no se pierde lo que el alumno marcó al llegar un evento.
    const opciones = li.querySelector(".encuesta-opciones");
    if (c.kind === "open") {
      li.querySelector(".encuesta-texto").hidden = false;
    } else {
      opcionesDe(c).forEach((texto, i) => {
        const label = document.createElement("label");
        label.className = "opcion";
        const radio = document.createElement("input");
        radio.type = "radio";
        radio.name = "encuesta-" + c.pollId;
        radio.value = String(i);
        radio.addEventListener("change", () => { error.hidden = true; });
        const span = document.createElement("span");
        span.textContent = texto;
        label.append(radio, span);
        opciones.appendChild(label);
      });
    }
    form.addEventListener("submit", (e) => {
      e.preventDefault();
      visto();
      error.hidden = true;
      let choice = -1, text = "";
      if (c.kind === "open") {
        text = campo.value;
      } else {
        const marcado = form.querySelector("input[type=radio]:checked");
        if (!marcado) { error.textContent = T("encuesta.eligeAntes"); error.hidden = false; return; }
        choice = Number(marcado.value);
      }
      const boton = li.querySelector(".encuesta-enviar");
      boton.setAttribute("aria-busy", "true");
      api.AnswerPoll(c.pollId, choice, text).then(renderEncuestas, (err) => {
        error.textContent = errorEncuesta(err);
        error.hidden = false;
      }).finally(() => boton.removeAttribute("aria-busy"));
    });
    return li;
  }

  function pintarEncuesta(li, c) {
    li.querySelector(".encuesta-pregunta").textContent = c.question;
    li.querySelector(".encuesta-hora").textContent = hora(c.at);
    li.querySelector(".encuesta-privacidad").textContent =
      T(ET + c.kind) + " " + (c.named ? T("encuesta.nominal") : T("encuesta.anonima"));
    const insignia = li.querySelector(".encuesta-estado-insignia");
    insignia.textContent = c.open ? T("encuesta.abierta") : T("encuesta.cerrada");
    insignia.classList.toggle("insignia--conectado", c.open);
    // Cerrada: las opciones quedan a la vista, sin poder cambiarlas.
    const puede = c.open && c.vote !== "unsupported";
    for (const x of li.querySelectorAll(".encuesta-campo input")) x.disabled = !puede;
    li.querySelector(".encuesta-botones").hidden = !puede;
    const contestada = c.vote === "accepted" || c.vote === "sending";
    li.querySelector(".encuesta-enviar span").textContent = contestada ? T("encuesta.cambiar") : T("encuesta.enviar");
    // Lo que respondió, marcado, si el alumno no está eligiendo otra cosa.
    if (!li.contains(document.activeElement)) {
      if (c.kind !== "open" && c.choice >= 0) {
        const r = li.querySelector('input[type=radio][value="' + c.choice + '"]');
        if (r) r.checked = true;
      }
      if (c.kind === "open" && c.text) li.querySelector(".encuesta-texto input").value = c.text;
    }
    let voto = "";
    const respuesta = respuestaDe(c);
    if (!c.open && c.vote === "accepted") voto = T("encuesta.cerradaRespondiste", { respuesta });
    else if (!c.open && c.vote === "none") voto = T("encuesta.cerradaSinRespuesta");
    else if (c.vote === "accepted") voto = T(EV + "accepted", { respuesta });
    else if (c.vote !== "none") voto = T(EV + c.vote);
    li.querySelector(".encuesta-voto").textContent = voto;
    // Resultados de la clase, si el profesor los compartió: texto y número, no solo la barra.
    const caja = li.querySelector(".encuesta-resultados");
    caja.hidden = !c.results;
    if (!c.results) return;
    const lista = caja.querySelector(".barras");
    lista.replaceChildren();
    const total = c.results.voters;
    opcionesDe(c).forEach((texto, i) => {
      const n = c.results.counts[i] || 0;
      const pct = total ? Math.round((n * 100) / total) : 0;
      const fila = document.createElement("li");
      fila.className = "barra-opcion";
      const tuya = i === c.choice && c.vote === "accepted";
      if (tuya) fila.dataset.elegida = "true";
      const op = document.createElement("span");
      op.className = "barra-opcion__texto";
      op.textContent = tuya ? T("encuesta.tuya", { opcion: texto }) : texto;
      const barra = document.createElement("span");
      barra.className = "barra-opcion__pista";
      barra.setAttribute("aria-hidden", "true");
      const relleno = document.createElement("span");
      relleno.className = "barra-opcion__relleno";
      relleno.style.width = pct + "%";
      barra.appendChild(relleno);
      const cifra = document.createElement("span");
      cifra.className = "barra-opcion__cifra";
      cifra.textContent = T("encuesta.cifra", { n, pct });
      fila.append(op, barra, cifra);
      lista.appendChild(fila);
    });
    caja.querySelector(".encuesta-total").textContent = T("encuesta.total", { n: total, total: c.results.recipients });
  }

  // --- Cuestionarios (docs/protocol.md §5.23.1; privacidad, reglas 87-91) --------------------------
  // La tarjeta muestra una pregunta a la vez, con Anterior y Siguiente (botones, con teclado), y
  // cuántas lleva. Cada vez que marca algo se guarda (Go lo manda como mucho una vez por segundo,
  // solo lo último); «Entregar» lo deja fijo. Lo marcado vive aquí hasta que llega del backend.
  const cuestionarioTpl = $("cuestionario-template");
  const quizIdx = new Map();    // pollId → pregunta a la vista
  const quizMarcas = new Map(); // pollId → Map(qid → Set(opciones))
  const quizConfirmar = new Set(); // pollId: «Entregar así» pedido con preguntas sin responder
  const opcionesPregunta = (q) => (q.kind === "yesno"
    ? [{ id: 0, text: T("encuesta.si") }, { id: 1, text: T("encuesta.no") }] : q.options || []);
  const textoOpciones = (q, ids) => {
    const por = new Map(opcionesPregunta(q).map((o) => [o.id, o.text]));
    return ids.map((i) => por.get(i) || "").filter(Boolean).join(", ");
  };
  function marcasDe(c) {
    let m = quizMarcas.get(c.pollId);
    if (!m) {
      m = new Map((c.answers || []).map((a) => [a.q, new Set(a.choices)]));
      quizMarcas.set(c.pollId, m);
    }
    return m;
  }
  const respondidas = (c) => [...marcasDe(c).values()].filter((s) => s.size > 0).length;
  const respuestasDe = (c) => [...marcasDe(c)].filter(([, s]) => s.size > 0)
    .map(([q, s]) => ({ q, choices: [...s].sort((a, b) => a - b) })).sort((a, b) => a.q - b.q);
  const editable = (c) => c.open && !c.submitted && c.vote !== "unsupported";

  function guardarCuestionario(li, c, entregar) {
    const error = li.querySelector(".encuesta-error");
    error.hidden = true;
    return api.AnswerQuiz(c.pollId, respuestasDe(c), entregar).then(renderEncuestas, (err) => {
      error.textContent = errorEncuesta(err);
      error.hidden = false;
    });
  }

  function nodoCuestionario(c, nuevo) {
    const li = cuestionarioTpl.content.firstElementChild.cloneNode(true);
    aplicarTextos(li);
    li.dataset.encuesta = c.pollId;
    li.tabIndex = -1;
    const marca = li.querySelector(".nuevo");
    marca.textContent = T("codigo.nuevo");
    marca.hidden = !nuevo;
    const visto = () => { if (!marca.hidden) { marca.hidden = true; title(); pintarCompacto(); } };
    li.addEventListener("focusin", visto);
    li.addEventListener("pointerdown", visto);
    // La primera sin responder; si ya respondió todas, la primera.
    const m = marcasDe(c);
    const primera = (c.questions || []).findIndex((q) => !(m.get(q.id) || new Set()).size);
    quizIdx.set(c.pollId, Math.max(0, primera));
    const mover = (d) => {
      const actual = cartas.get(c.pollId) || c;
      const n = (actual.questions || []).length;
      quizIdx.set(c.pollId, Math.min(n - 1, Math.max(0, (quizIdx.get(c.pollId) || 0) + d)));
      quizConfirmar.delete(c.pollId);
      li.dataset.pregunta = "";
      pintarCuestionario(li, actual);
      li.querySelector(".cuestionario-opciones input")?.focus();
    };
    li.querySelector(".cuestionario-anterior").addEventListener("click", () => { visto(); mover(-1); });
    li.querySelector(".cuestionario-siguiente").addEventListener("click", () => { visto(); mover(1); });
    li.querySelector(".cuestionario-entregar").addEventListener("click", () => {
      visto();
      const actual = cartas.get(c.pollId) || c;
      const faltan = (actual.questions || []).length - respondidas(actual);
      if (faltan > 0 && !quizConfirmar.has(c.pollId)) {
        quizConfirmar.add(c.pollId); // el segundo clic entrega así
        pintarCuestionario(li, actual);
        return;
      }
      quizConfirmar.delete(c.pollId);
      const b = li.querySelector(".cuestionario-entregar");
      b.setAttribute("aria-busy", "true");
      guardarCuestionario(li, actual, true).finally(() => b.removeAttribute("aria-busy"));
    });
    return li;
  }

  // La pregunta a la vista: se arma de nuevo solo al cambiar de pregunta, para no perder el foco.
  function pintarPregunta(li, c) {
    const idx = quizIdx.get(c.pollId) || 0;
    const q = (c.questions || [])[idx];
    if (!q) return;
    const caja = li.querySelector(".cuestionario-opciones");
    const clave = c.pollId + ":" + q.id;
    if (li.dataset.pregunta !== clave) {
      li.dataset.pregunta = clave;
      li.querySelector(".cuestionario-texto").textContent = T("cuestionario.preguntaN", { n: idx + 1, texto: q.text });
      li.querySelector(".cuestionario-indicacion").textContent =
        (q.multi ? T("cuestionario.varias") : q.kind === "yesno" ? T("cuestionario.siNo") : T("cuestionario.una")) + " " + P("cuestionario.puntos", q.points);
      caja.replaceChildren();
      for (const o of opcionesPregunta(q)) {
        const label = document.createElement("label");
        label.className = "opcion";
        const input = document.createElement("input");
        input.type = q.multi ? "checkbox" : "radio";
        input.name = "cuestionario-" + c.pollId + "-" + q.id;
        input.value = String(o.id);
        input.addEventListener("change", () => {
          const m = marcasDe(cartas.get(c.pollId) || c);
          const sel = m.get(q.id) || new Set();
          if (q.multi) { if (input.checked) sel.add(o.id); else sel.delete(o.id); }
          else { sel.clear(); sel.add(o.id); }
          m.set(q.id, sel);
          quizConfirmar.delete(c.pollId);
          guardarCuestionario(li, cartas.get(c.pollId) || c, false);
          pintarCuestionario(li, cartas.get(c.pollId) || c);
        });
        const span = document.createElement("span");
        span.textContent = o.text;
        label.append(input, span);
        caja.appendChild(label);
      }
    }
    const sel = marcasDe(c).get(q.id) || new Set();
    for (const x of caja.querySelectorAll("input")) {
      x.checked = sel.has(Number(x.value));
      x.disabled = !editable(c);
    }
  }

  function pintarCuestionario(li, c) {
    const n = (c.questions || []).length;
    const idx = quizIdx.get(c.pollId) || 0;
    li.querySelector(".cuestionario-nombre").textContent = c.question;
    li.querySelector(".encuesta-hora").textContent = hora(c.at);
    const insignia = li.querySelector(".encuesta-estado-insignia");
    insignia.textContent = c.submitted ? T("cuestionario.entregado") : c.open ? T("encuesta.abierta") : T("encuesta.cerrada");
    insignia.classList.toggle("insignia--conectado", c.open && !c.submitted);
    const llevas = respondidas(c);
    li.querySelector(".cuestionario-progreso").textContent = T("cuestionario.progreso", { n: idx + 1, total: n, llevas });
    // Con el resultado del profesor, la pregunta y la navegación dejan su lugar al resultado.
    const conResultado = !!c.result;
    li.querySelector(".cuestionario-pregunta").hidden = conResultado;
    li.querySelector(".cuestionario-progreso").hidden = conResultado;
    if (!conResultado) pintarPregunta(li, c);
    const nav = li.querySelector(".cuestionario-nav");
    nav.hidden = conResultado;
    li.querySelector(".cuestionario-anterior").disabled = idx === 0;
    li.querySelector(".cuestionario-siguiente").disabled = idx >= n - 1;
    const entregar = li.querySelector(".cuestionario-entregar");
    entregar.hidden = !editable(c);
    const confirmar = quizConfirmar.has(c.pollId) && editable(c);
    entregar.textContent = confirmar ? T("cuestionario.entregarAsi") : T("cuestionario.entregar");
    const aviso = li.querySelector(".cuestionario-confirmar");
    aviso.hidden = !confirmar;
    if (confirmar) aviso.textContent = P("cuestionario.faltan", n - llevas);
    let voto = "";
    if (c.submitted) voto = T("cuestionario.voto.entregado");
    else if (!c.open && c.vote === "late") voto = T("cuestionario.voto.late");
    else if (!c.open && llevas === 0) voto = T("cuestionario.cerradoSinRespuesta");
    else if (!c.open) voto = T("cuestionario.cerradoGuardadas");
    else if (c.vote === "accepted") voto = T("cuestionario.voto.accepted");
    else if (c.vote === "sending") voto = T("cuestionario.voto.sending");
    else if (c.vote !== "none") voto = T(EV + c.vote);
    li.querySelector(".encuesta-voto").textContent = voto;
    // Solo su resultado (regla 90): qué acertó y la correcta, con texto e icono, no solo color.
    const caja = li.querySelector(".cuestionario-resultado");
    caja.hidden = !conResultado;
    if (!conResultado) return;
    caja.querySelector(".cuestionario-puntaje").textContent = T("cuestionario.puntaje", { score: c.result.score, max: c.result.max });
    // En el orden en que este alumno vio las preguntas.
    const porId = new Map(c.result.questions.map((r) => [r.q, r]));
    const filas = (c.questions || []).filter((q) => porId.has(q.id));
    caja.querySelector(".cuestionario-detalle").replaceChildren(...filas.map((q, i) => {
      const r = porId.get(q.id);
      const fila = document.createElement("li");
      fila.className = "cuestionario-fila";
      const bien = r.earned > 0;
      fila.dataset.acierto = bien ? "si" : "no";
      const cab = document.createElement("p");
      const ins = document.createElement("span");
      ins.className = "insignia " + (bien ? "insignia--conectado" : "insignia--advertencia");
      ins.append(iconoCap(bien ? "hecho" : "cerrar"), document.createTextNode(bien ? T("cuestionario.bien") : T("cuestionario.mal")));
      const txt = document.createElement("span");
      txt.textContent = " " + T("cuestionario.preguntaN", { n: i + 1, texto: q.text }) + " · " + P("cuestionario.puntosDe", q.points, { obtuvo: r.earned });
      cab.append(ins, txt);
      const det = document.createElement("p");
      det.className = "suave pequeno";
      const tuya = r.choices.length ? textoOpciones(q, r.choices) : T("cuestionario.sinRespuesta");
      det.textContent = T("cuestionario.tuRespuesta", { respuesta: tuya }) + (bien ? "" : " · " + T("cuestionario.correcta", { respuesta: textoOpciones(q, r.correct) }));
      fila.append(cab, det);
      return fila;
    }));
  }

  const cartas = new Map(); // pollId → la última tarjeta que mandó Go
  function renderEncuestas(v) {
    if (!v) return;
    const cards = v.cards || [];
    cartas.clear();
    for (const c of cards) cartas.set(c.pollId, c);
    const vivos = new Set(cards.map((c) => c.pollId));
    for (const [id, li] of nodosEncuesta) {
      if (!vivos.has(id)) { li.remove(); nodosEncuesta.delete(id); quizIdx.delete(id); quizMarcas.delete(id); quizConfirmar.delete(id); }
    }
    let anterior = null;
    for (const c of cards) { // lo más nuevo primero
      let li = nodosEncuesta.get(c.pollId);
      if (!li) {
        const nuevo = !encuestasPrimeraVez || v.arrived === c.pollId;
        li = c.kind === "quiz" ? nodoCuestionario(c, nuevo) : nodoEncuesta(c, nuevo);
        nodosEncuesta.set(c.pollId, li);
      }
      if (c.kind === "quiz") pintarCuestionario(li, c);
      else pintarEncuesta(li, c);
      const lugar = anterior ? anterior.nextSibling : encuestasLista.firstChild;
      if (li !== lugar) encuestasLista.insertBefore(li, lugar);
      anterior = li;
    }
    if (v.arrived) {
      const c = cards.find((x) => x.pollId === v.arrived);
      if (c) anunciar(c.kind === "quiz" ? T("cuestionario.anuncio", { pregunta: c.question }) : T("encuesta.anuncio", { pregunta: c.question }));
    }
    encuestasPrimeraVez = false;
    nEncuestas = cards.length;
    pintarListaVacia();
    title();
    pintarCompacto();
  }

  // Lo que abrió el profesor (docs/protocol.md §5.22): los avisos «opened» toman de aquí qué
  // se abrió y quién lo pidió. La ventana no se trae al frente: el foco ya lo tiene lo abierto.
  let abiertos = [];
  function renderAbiertos(v) {
    abiertos = (v && v.list) || [];
    renderNotices(avisos);
  }

  // El texto de los avisos de una recolección (reglas 61-62), con los datos de su tarjeta.
  function avisoReco(n) {
    const c = recolecciones.find((x) => x.id === n.ref);
    if (n.kind === "collect-requested") {
      return { titulo: T("aviso.recogiendo.titulo"), texto: c ? T("aviso.recogiendo.texto", { carpeta: carpetaReco(c) }) : "" };
    }
    const titulo = T("aviso.recogido.titulo");
    const rondas = c ? c.rounds || [] : [];
    const r = rondas[rondas.length - 1];
    if (!r) return { titulo, texto: "" };
    const files = (r.files || []).filter((f) => f.saved);
    if (r.failed || (r.files || []).length && !files.length) return { titulo, texto: T("aviso.recogido.fallo") };
    if (!files.length) return { titulo, texto: T("aviso.recogido.vacio", { carpeta: carpetaReco(c) }) };
    const total = tamano(files.reduce((s, f) => s + f.size, 0));
    const nombre = files[0].path.split("/").pop();
    return { titulo, texto: P("aviso.recogido.texto", files.length, { nombre, tamano: total }) };
  }

  // --- Temporizador del profesor (docs/protocol.md §5.19) -------------------------------------------
  // Una insignia en la tira (y en el modo compacto) con lo que falta. Go manda el estado
  // (classtimer.View) y lo que faltaba al enviarlo; aquí se cuenta con performance.now, sin
  // depender de la hora del equipo. Sin animación: el número cambia una vez por segundo. Al
  // terminar, un aviso suave en la esquina: Go solo llama a notify, nunca a raise.
  let temporizador = { state: "none", label: "", remainingMs: 0 };
  let temporizadorEn = 0;
  let tic = 0;
  let finTemporizador = 0;
  const hmmss = (ms) => {
    const s = Math.max(0, Math.ceil(ms / 1000));
    const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60), r = String(s % 60).padStart(2, "0");
    return h ? h + ":" + String(m).padStart(2, "0") + ":" + r : m + ":" + r;
  };
  // La tarjeta con lo que hay que hacer (nota de interfaz 18): solo si el profesor escribió una
  // descripción. «Nuevo» cuando llega un temporizador distinto, no al abrir la ventana ni al
  // reenviarse el mismo al reconectar.
  let tareaId = "";
  let tareaPrimeraVez = true;
  function pintarTarea(v) {
    const d = v.state !== "none" ? v.description || "" : "";
    $("tarea").hidden = !d;
    if (d) {
      $("tarea-titulo").textContent = T("tarea.titulo", { nombre: v.label || T("temporizador.sinNombre") });
      $("tarea-texto").textContent = d;
      if (v.timerId !== tareaId) $("tarea-nuevo").hidden = tareaPrimeraVez;
    } else {
      $("tarea-nuevo").hidden = true;
    }
    tareaId = d ? v.timerId : "";
    tareaPrimeraVez = false;
    pintarListaVacia();
    title();
    pintarCompacto();
  }
  $("tarea-nuevo").textContent = T("codigo.nuevo");
  const tareaVista = () => { if (!$("tarea-nuevo").hidden) { $("tarea-nuevo").hidden = true; title(); pintarCompacto(); } };
  $("tarea").addEventListener("focusin", tareaVista);
  $("tarea").addEventListener("pointerdown", tareaVista);

  function renderTemporizador(v) {
    temporizador = v;
    temporizadorEn = performance.now();
    pintarTarea(v);
    clearInterval(tic);
    if (v.state === "running") tic = setInterval(pintarTemporizador, 1000);
    pintarTemporizador();
    if (v.justEnded) mostrarFinTemporizador();
    if (v.state === "none") ocultarFinTemporizador();
  }
  function pintarTemporizador() {
    const v = temporizador;
    const nombre = v.label || T("temporizador.sinNombre");
    const falta = Math.max(0, v.remainingMs - (performance.now() - temporizadorEn));
    const termino = v.state === "ended" || (v.state === "running" && falta <= 0);
    const texto = termino ? T("temporizador.termino", { nombre }) : T("temporizador.cuenta", { nombre, tiempo: hmmss(falta) });
    $("tarea-tiempo").textContent = termino ? T("tarea.termino") : hmmss(falta);
    for (const [caja, linea] of [["temporizador", "temporizador-texto"], ["compacto-temporizador", "compacto-temporizador-texto"]]) {
      $(caja).hidden = v.state === "none";
      $(linea).textContent = texto;
      // El último minuto y el fin, en el color de advertencia: sin parpadeo ni movimiento.
      $(caja).classList.toggle("insignia--info", !termino && falta > 60000);
      $(caja).classList.toggle("insignia--advertencia", termino || falta <= 60000);
    }
    if (termino) clearInterval(tic);
  }
  function mostrarFinTemporizador() {
    $("temporizador-fin-texto").textContent = T("temporizador.finTexto", { nombre: temporizador.label || T("temporizador.sinNombre") });
    $("temporizador-fin").hidden = false;
    clearTimeout(finTemporizador);
    finTemporizador = setTimeout(ocultarFinTemporizador, 30000);
  }
  function ocultarFinTemporizador() {
    clearTimeout(finTemporizador);
    $("temporizador-fin").hidden = true;
  }
  $("temporizador-fin-ok").addEventListener("click", ocultarFinTemporizador);
  $("temporizador-fin").addEventListener("keydown", (e) => { if (e.key === "Escape") { e.stopPropagation(); ocultarFinTemporizador(); } });

  // --- La clase terminó (docs/protocol.md §5.25, docs/privacidad.md regla 83) ----------------------
  // Go manda la tarjeta (classend.Card: la hora y lo que se quitó) y aquí se arma el texto. Llega
  // con notify, nunca trae la ventana; el alumno la cierra con «Entendido» y se va sola con código
  // nuevo (null). Lo que tenía banda ya la quitó al quitarse.
  const FC = "finClase.quitado."; // + campo de classend.Removed
  let finClaseId = "";
  function renderFinClase(c) {
    $("fin-clase").hidden = !c;
    if (!c) {
      $("fin-clase-nuevo").hidden = true;
      finClaseId = "";
      title();
      return;
    }
    const r = c.removed || {};
    const quitado = ["restrictions", "pause", "control", "present", "broadcast", "timer", "polls", "snippets", "notices",
      "files", "collections", "opened", "hand", "messages"].filter((k) => r[k]).map((k) => T(FC + k));
    $("fin-clase-hora").textContent = T("finClase.hora", { hora: new Date(c.at).toLocaleTimeString("es-MX", { hour: "2-digit", minute: "2-digit" }) });
    let texto = quitado.length ? T("finClase.texto", { lista: quitado.join(", ") }) : T("finClase.nada");
    if (r.files || r.collections) texto += " " + T("finClase.archivos");
    $("fin-clase-texto").textContent = texto;
    if (c.endId !== finClaseId) {
      $("fin-clase-nuevo").hidden = false;
      anunciar(T("finClase.titulo") + ". " + texto);
    }
    finClaseId = c.endId;
    title();
  }
  $("fin-clase-nuevo").textContent = T("codigo.nuevo");
  const finClaseVista = () => { if (!$("fin-clase-nuevo").hidden) { $("fin-clase-nuevo").hidden = true; title(); } };
  $("fin-clase").addEventListener("focusin", finClaseVista);
  $("fin-clase").addEventListener("pointerdown", finClaseVista);
  $("fin-clase-ok").addEventListener("click", () => { api.DismissClassEnd(); renderFinClase(null); });

  // --- Levantar la mano y escribir al profesor (docs/protocol.md §5.18) -----------------------------
  // Lo hace el alumno: nada de esto quita el foco ni trae la ventana, y no hay
  // banda. Go manda el estado (participation.DeskView) y aquí se arma el texto.
  // Los errores llegan como «aula:<código>» y se dicen con mano.error.<código>.
  let participacion = { hand: "down", note: "", messages: [], unsupported: false };
  const ME = "mano.error."; // + código de apps/student/participation.go
  const EM = "mensaje.estado."; // + participation.SentStatus
  const errorDe = (err) => {
    const m = /aula:(\w+)/.exec(String(err && err.message ? err.message : err));
    return m && typeof TX[ME + m[1]] === "string" ? T(ME + m[1]) : String(err);
  };
  const conectado = () => status.state === "connected";
  // participation.HandState → clave: «lowered_by_teacher» → «loweredByTeacher».
  const estadoMano = (h) => "mano.estado." + String(h).replace(/_([a-z])/g, (_, c) => c.toUpperCase());
  const maxEnviados = 3;

  function pintarMano() {
    const arriba = participacion.hand === "up";
    const puede = conectado() && !participacion.unsupported;
    const ayuda = !conectado() ? T("mano.ayudaSinConexion") : participacion.unsupported ? T("mano.ayudaSinSoporte") : "";
    const etiqueta = arriba ? T("mano.bajar") : T("mano.levantar");
    for (const b of [$("mano"), $("compacto-mano")]) {
      b.setAttribute("aria-pressed", String(arriba));
      // Bajar la mano siempre se puede, aun sin conexión: es del alumno.
      b.disabled = !puede && !arriba;
      b.title = ayuda || etiqueta;
    }
    $("mano-texto").textContent = etiqueta;
    $("compacto-mano").setAttribute("aria-label", etiqueta);
    for (const b of [$("escribir"), $("compacto-escribir")]) {
      b.disabled = !puede;
      b.title = ayuda || T("mensaje.boton");
    }
    if (!puede && !$("redactar").hidden) cerrarRedactar(false);
  }

  function renderParticipacion(v) {
    const antes = participacion.hand;
    participacion = v;
    pintarMano();
    // La línea de estado: la mano levantada (con su motivo) o que el profesor la bajó.
    const linea = $("mano-estado");
    const visible = v.hand === "up" || v.hand === "lowered_by_teacher" || v.unsupported;
    linea.hidden = !visible;
    linea.dataset.estado = v.unsupported ? "unsupported" : v.hand;
    let texto = v.unsupported ? T("mano.sinSoporte") : visible ? T(estadoMano(v.hand)) : "";
    if (v.hand === "up" && v.note) texto += " " + T("mano.motivo", { texto: v.note });
    $("mano-estado-texto").textContent = texto;
    $("motivo-abrir").hidden = v.hand !== "up" || !$("motivo").hidden;
    $("motivo-abrir").textContent = v.note ? T("mano.cambiarMotivo") : T("mano.agregarMotivo");
    if (v.hand !== "up") $("motivo").hidden = true;
    if (antes !== v.hand) anunciar(v.hand === "up" ? T("mano.anuncioArriba") : v.hand === "lowered_by_teacher" ? T(estadoMano(v.hand)) : T("mano.anuncioAbajo"));
    // Los últimos mensajes del alumno y si ya llegaron.
    const ultimos = v.messages.slice(-maxEnviados).reverse();
    $("enviados").hidden = ultimos.length === 0;
    $("enviados-lista").replaceChildren(...ultimos.map((m) => {
      const li = document.createElement("li");
      li.className = "enviado";
      const t = document.createElement("span");
      t.className = "enviado__texto";
      t.textContent = m.text;
      const e = document.createElement("span");
      e.className = "insignia" + (m.status === "received" ? " insignia--conectado" : m.status === "failed" ? " insignia--error" : "");
      e.textContent = T(EM + m.status);
      const h = document.createElement("span");
      h.className = "codigo-recibido__meta";
      h.textContent = hora(m.at);
      li.append(t, h, e);
      return li;
    }));
    renderNotices(avisos); // una respuesta cita el mensaje: ya se conoce
  }

  function alternarMano() {
    const arriba = participacion.hand === "up";
    (arriba ? api.LowerHand() : api.RaiseHand("")).then(renderParticipacion, (err) => {
      anunciar(errorDe(err));
      $("mano-estado").hidden = false;
      $("mano-estado").dataset.estado = "error";
      $("mano-estado-texto").textContent = errorDe(err);
    });
  }
  $("mano").addEventListener("click", alternarMano);
  $("compacto-mano").addEventListener("click", alternarMano);

  // El motivo («pedir ayuda»): levanta otra vez la misma mano, que conserva su lugar.
  $("motivo-abrir").addEventListener("click", () => {
    $("motivo").hidden = false;
    $("motivo-abrir").hidden = true;
    $("motivo-texto").value = participacion.note || "";
    $("motivo-texto").focus();
  });
  const cerrarMotivo = () => {
    $("motivo").hidden = true;
    $("motivo-abrir").hidden = participacion.hand !== "up";
    $("motivo-abrir").focus();
  };
  $("motivo-texto").addEventListener("keydown", (e) => {
    if (e.key === "Escape") { e.preventDefault(); e.stopPropagation(); cerrarMotivo(); }
  });
  $("motivo").addEventListener("submit", (e) => {
    e.preventDefault();
    api.RaiseHand($("motivo-texto").value).then((v) => { renderParticipacion(v); cerrarMotivo(); },
      (err) => { $("mano-estado-texto").textContent = errorDe(err); });
  });

  // El mensaje se escribe en su propio diálogo (nota de interfaz 12): Enter envía, Esc cancela.
  // El formulario lleva `hidden` mientras el diálogo está cerrado (lo miran las pruebas y la tira).
  const dlgMensaje = $("dlg-mensaje");
  const area = $("redactar-texto");
  const pintarAyuda = () => {
    $("redactar-ayuda").textContent = T("mensaje.ayuda", { n: [...area.value].length });
  };
  function abrirRedactar() {
    if (compacto) ponerCompacto(false, true); // lo pide el alumno: necesita la ventana
    $("redactar").hidden = false;
    if (!dlgMensaje.open) dlgMensaje.showModal();
    $("redactar-titulo").textContent = T("mensaje.titulo", { profesor: status.teacher || T("energia.profesor") });
    $("escribir").setAttribute("aria-expanded", "true");
    pintarAyuda();
    area.focus();
  }
  function cerrarRedactar(foco) {
    $("redactar").hidden = true;
    if (dlgMensaje.open) dlgMensaje.close();
    $("redactar-error").hidden = true;
    $("escribir").setAttribute("aria-expanded", "false");
    if (foco) $("escribir").focus();
  }
  $("escribir").addEventListener("click", () => ($("redactar").hidden ? abrirRedactar() : cerrarRedactar(true)));
  $("compacto-escribir").addEventListener("click", abrirRedactar);
  $("redactar-cancelar").addEventListener("click", () => cerrarRedactar(true));
  $("redactar-cerrar").addEventListener("click", () => cerrarRedactar(true));
  // Esc del diálogo (fuera del campo) cierra igual que «Cancelar».
  dlgMensaje.addEventListener("cancel", (e) => { e.preventDefault(); cerrarRedactar(true); });
  area.addEventListener("input", () => { $("redactar-error").hidden = true; area.removeAttribute("aria-invalid"); pintarAyuda(); });
  area.addEventListener("keydown", (e) => {
    if (e.key === "Enter" && !e.shiftKey && !e.isComposing) { e.preventDefault(); $("redactar").requestSubmit(); }
    if (e.key === "Escape") { e.preventDefault(); e.stopPropagation(); cerrarRedactar(true); }
  });
  $("redactar").addEventListener("submit", (e) => {
    e.preventDefault();
    const b = $("redactar-enviar");
    b.disabled = true;
    api.SendTeacherMessage(area.value).then((v) => {
      area.value = "";
      renderParticipacion(v);
      anunciar(T("mensaje.anuncioEnviado"));
      cerrarRedactar(true);
    }, (err) => {
      $("redactar-error").textContent = errorDe(err);
      $("redactar-error").hidden = false;
      area.setAttribute("aria-invalid", "true");
      area.focus();
    }).finally(() => { b.disabled = false; });
  });


  // --- Transmisión de la pantalla del profesor ------------------------------------------------------
  // El backend ya revisó la oferta (video solo de recepción) y mostró esta
  // vista antes de pasarla. Esta conexión nunca envía medios.
  const bcSection = $("broadcast");
  const video = $("bc-video");
  const wait = $("bc-wait");
  // La conexión la arma el receptor compartido (sistema/receptor-video.js), el mismo de la
  // Pantalla grande de la consola.
  const receptor = window.AulaReceptor.crear({ video, espera: wait,
    answer: (...a) => api.BroadcastAnswer(...a), candidate: (...a) => api.BroadcastCandidate(...a),
    failed: (...a) => api.BroadcastFailed(...a) });
  const closeConnection = receptor.cerrar;
  const onSignal = receptor.senal;

  function probeMedia() {
    const webrtc = typeof window.RTCPeerConnection === "function";
    let h264 = false;
    try {
      const caps = window.RTCRtpReceiver.getCapabilities("video");
      h264 = caps.codecs.some((c) => c.mimeType.toLowerCase() === "video/h264");
    } catch (e) { h264 = false; }
    api.ReportMedia(webrtc, h264);
  }

  // La pantalla de un compañero (§5.24) usa la misma vista: la franja dice de quién es.
  function renderBroadcast(b) {
    document.body.classList.toggle("watching", b.active);
    bcSection.hidden = !b.active;
    bcSource = b.active ? b.source || "" : "";
    $("bc-notice").textContent = !b.active ? ""
      : bcSource ? T("banda.recibiendoAlumno", { equipo: bcSource, canal: b.channel })
      : T("banda.recibiendo", { profesor: b.teacher, canal: b.channel });
    wait.textContent = bcSource ? T("banda.conectandoAlumno", { equipo: bcSource }) : T("banda.conectando");
    // En una ventana o en pantalla completa (regla 12) la ventana no se cierra ni se encoge hasta
    // que termine; la franja lo dice. Sin modo (una consola anterior) es la vista de siempre.
    const modo = b.active ? b.mode || "" : "";
    bcSection.dataset.modo = modo;
    const BM = "banda.modo."; // + BroadcastView.Mode (window, fullscreen)
    $("bc-modo").textContent = modo ? T(BM + modo) : "";
    $("compactar").hidden = !!modo;
    title();
    if (b.active) salirDeCompacto();
    if (!b.active) closeConnection();
  }

  $("bc-size").addEventListener("click", () => {
    const stage = $("bc-stage");
    const real = stage.dataset.fit !== "real";
    stage.dataset.fit = real ? "real" : "window";
    $("bc-size").textContent = real ? T("banda.ajustar") : T("banda.tamanoReal");
    $("bc-size").setAttribute("aria-pressed", String(real));
  });

  // --- Qué puede hacer Aula en este equipo (nota de interfaz 13) ------------------------------
  // La misma grilla que la ficha del alumno en la consola, desde las capacidades que este equipo
  // declara (View().capabilities). Se pide al abrir: cambian si el ayudante del servicio cambia.
  const PUEDE = [
    ["code.view", "codigo", () => T("capacidad.codigo")], ["msg.show", "aviso", () => T("capacidad.avisos")],
    ["hand", "mano", () => T("capacidad.mano")], ["msg.student", "mensaje", () => T("capacidad.mensajes")],
    ["timer", "reloj", () => T("capacidad.temporizador")], ["poll", "encuesta", () => T("capacidad.encuestas")],
    ["poll.quiz", "cuestionario", () => T("capacidad.cuestionarios")],
    ["file.push", "archivo", () => T("capacidad.archivos")], ["file.collect", "carpeta", () => T("capacidad.recoger")],
    ["screen.broadcast", "transmitir", () => T("capacidad.transmision")], ["screen.present.view", "ojo", () => T("capacidad.verCompanero")],
    ["screen.thumb", "miniaturas", () => T("capacidad.miniaturas")], ["screen.snapshot", "captura", () => T("capacidad.captura")],
    ["screen.present", "pantalla", () => T("capacidad.proyectar")], ["remote", "control", () => T("capacidad.control")],
    ["power", "energia", () => T("capacidad.energia")], ["app.block", "candado", () => T("capacidad.apps")],
    ["web.filter", "filtro", () => T("capacidad.web")], ["open.url", "externo", () => T("capacidad.abrirPagina")],
    ["open.app", "app", () => T("capacidad.abrirApp")],
  ];
  const SVGNS = "http://www.w3.org/2000/svg";
  function iconoCap(id, cls) {
    const s = document.createElementNS(SVGNS, "svg");
    s.setAttribute("class", "icono" + (cls ? " " + cls : ""));
    s.setAttribute("aria-hidden", "true");
    const u = document.createElementNS(SVGNS, "use");
    u.setAttribute("href", "sistema/iconos.svg#" + id);
    s.append(u);
    return s;
  }
  function pintarCapacidades(caps) {
    const tiene = new Set(caps || []);
    $("capacidades-lista").replaceChildren(...PUEDE.map(([c, ic, nombre]) => {
      const si = tiene.has(c);
      const texto = si ? T("capacidades.si", { accion: nombre() }) : T("capacidades.no", { accion: nombre() });
      const li = document.createElement("li");
      li.className = "capacidad" + (si ? " capacidad--si" : "");
      li.dataset.tip = texto;
      li.tabIndex = 0;
      const sr = document.createElement("span");
      sr.className = "solo-lector";
      sr.textContent = texto;
      li.append(iconoCap(ic), iconoCap(si ? "hecho" : "cerrar", "capacidad__marca"), sr);
      return li;
    }));
  }
  // Este equipo: su nombre y, si el profesor se la puso en su salón, su etiqueta (§5.26).
  function pintarEquipo(c) {
    $("capacidades-equipo").hidden = !(c && c.hostname);
    if (!c || !c.hostname) return;
    $("capacidades-equipo-texto").textContent = c.label
      ? T("capacidades.equipoEtiqueta", { etiqueta: c.label, equipo: c.hostname })
      : T("capacidades.equipo", { equipo: c.hostname });
  }
  const dlgCapacidades = $("dlg-capacidades");
  $("capacidades").addEventListener("click", () => {
    api.View().then((v) => { pintarCapacidades(v.capabilities); pintarEquipo(v.computer); }, () => { pintarCapacidades([]); pintarEquipo(null); });
    dlgCapacidades.showModal();
    $("capacidades-entendido").focus();
  });
  for (const id of ["capacidades-cerrar", "capacidades-entendido"]) $(id).addEventListener("click", () => dlgCapacidades.close());
  dlgCapacidades.addEventListener("close", () => $("capacidades").focus());

  // --- Mostrar tu pantalla a la clase (docs/privacidad.md, reglas 75-82) ------------------------
  // Lo decide el profesor, como el control remoto: Go trae la ventana al frente y manda la fase
  // (live o nada) antes del primer cuadro. La banda no tiene botones: se va cuando el profesor
  // detiene la proyección.
  function renderPresent(v) {
    const antes = presentV.phase;
    presentV = v && v.phase ? v : { phase: "" };
    const live = presentV.phase === "live";
    $("banda-mostrando").hidden = !live;
    if (live && antes !== "live") salirDeCompacto();
    if (antes === "live" && !live) anunciar(T("banda.mostrandoFin"));
    title();
    pintarCompacto();
  }

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
    const antes = restricted;
    restricted = parts.length > 0;
    if (restricted && !antes) salirDeCompacto();
    restr.hidden = !restricted;
    $("restr-text").textContent = restricted ? T("banda.restricciones", { partes: parts.join(T(BR + "separador")) }) : "";
    restrDetails.replaceChildren(...[restrList(T("banda.lista.apps"), r.apps), restrList(T("banda.lista.web"), r.web)].filter(Boolean));
    if (!restricted) {
      cerradas = [];
      pintarCerradas();
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

  // Las apps que Aula cerró se juntan en la banda de restricciones y se quedan
  // hasta que el alumno pulsa «Entendido»: si no estaba mirando, sabe por qué
  // se cerró su app (regla 22).
  let cerradas = [];
  function pintarCerradas() {
    $("app-closed").hidden = cerradas.length === 0;
    $("app-closed-text").textContent = cerradas.length
      ? P("banda.appCerrada", cerradas.length, { apps: cerradas.map((a) => "«" + a + "»").join(", ") }) : "";
  }
  function onAppClosed(c) {
    if (!cerradas.includes(c.app)) cerradas.push(c.app);
    pintarCerradas();
  }
  $("app-closed-ok").addEventListener("click", () => { cerradas = []; pintarCerradas(); });

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
    if (pendingPower) salirDeCompacto();
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
  // Los iconos de referencia de botones y campos (data-icono, sistema/iconos.js): también los de
  // lo que se agregue después.
  window.AulaIconos.vigilar(document.body);
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
  rt.EventsOn(ev.PARTICIPATION, renderParticipacion);
  rt.EventsOn(ev.TIMER, renderTemporizador);
  rt.EventsOn(ev.FILES, renderArchivos);
  rt.EventsOn(ev.COLLECTIONS, renderRecolecciones);
  rt.EventsOn(ev.OPENED, renderAbiertos);
  rt.EventsOn(ev.POLLS, renderEncuestas);
  rt.EventsOn(ev.CLASS_END, renderFinClase);
  rt.EventsOn(ev.PRESENT, renderPresent);
  api.View().then((v) => {
    abiertos = (v.opened && v.opened.list) || [];
    renderStatus(v.status);
    renderBoard(v.snippets);
    renderNotices(v.notices);
    renderBroadcast(v.broadcast);
    renderRestrictions(v.restrictions);
    renderPower(v.power);
    if (v.participation) renderParticipacion(v.participation);
    if (v.timer) renderTemporizador(v.timer);
    renderArchivos(v.files);
    renderRecolecciones(v.collections);
    renderEncuestas(v.polls);
    renderPresent(v.present);
    if (v.classEnd) renderFinClase(v.classEnd);
  });
  probeMedia();
})();
