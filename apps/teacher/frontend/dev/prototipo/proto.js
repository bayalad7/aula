// Prototipo de la Fase 2 (solo desarrollo; no se embebe en la consola).
// Datos de demostración e interacción mínima para evaluar el sistema de diseño:
// menú, Ajustes, cambio de canal, selección, lista o miniaturas, paneles
// laterales, diálogos, avisos flotantes y copiar. La lógica real llegará con
// las entregas E3–E9.
// Solo prototipo: arma HTML con datos fijos de este archivo. La consola real
// sigue dibujando lo que llega del backend con textContent y plantillas.
"use strict";

(function () {
  const $ = (s, el = document) => el.querySelector(s);
  const $$ = (s, el = document) => [...el.querySelectorAll(s)];
  const params = new URLSearchParams(location.search);
  // The sprite sits next to tokens.css, wherever this page is.
  const ICONOS = document.querySelector('link[href$="sistema/tokens.css"]').getAttribute("href").replace("tokens.css", "iconos.svg");
  const icono = (id, cls = "") =>
    '<svg class="icono ' + cls + '" aria-hidden="true"><use href="' + ICONOS + "#" + id + '"/></svg>';

  // --- Tema: sistema, claro u oscuro (?tema=claro|oscuro; Ajustes › Apariencia) ----------------
  function ponerTema(t) {
    if (t === "dark" || t === "light") document.documentElement.dataset.theme = t;
    else delete document.documentElement.dataset.theme;
    for (const b of $$("[data-tema]")) {
      const on = String(b.dataset.tema === (t || "sistema"));
      b.setAttribute(b.getAttribute("role") === "radio" ? "aria-checked" : "aria-pressed", on);
    }
    try { localStorage.setItem("aula-tema", t || "sistema"); } catch { /* sin almacenamiento */ }
  }
  let temaGuardado = "sistema";
  try { temaGuardado = localStorage.getItem("aula-tema") || "sistema"; } catch { /* sin almacenamiento */ }
  const temaUrl = { oscuro: "dark", claro: "light" }[params.get("tema")];

  // --- Menú lateral, Ajustes y Cambiar de canal: iguales en todas las páginas -----------------
  const nav = $("#nav");
  if (nav) {
    const actual = params.get("escena") === "simulada" ? "simulada" : nav.dataset.actual;
    const enlace = (id, href, ic, texto) =>
      '<a href="' + href + '"' + (actual === id ? ' aria-current="page"' : "") + ">" + icono(ic) +
      '<span class="nav-lateral__texto">' + texto + "</span></a>";
    nav.innerHTML =
      '<div class="nav-lateral__marca">' + icono("codigo", "icono--lg") + "<strong>Aula</strong></div>" +
      '<button type="button" class="nav-lateral__clase" data-dialogo="cambiar-canal" title="Cambiar de canal">' +
      '<span class="suave">Clase abierta · canal</span>' + icono("flecha-abajo") + "<strong>1 · Prof. Ayala</strong></button>" +
      '<button type="button" class="nav-lateral__clase-compacta" data-dialogo="cambiar-canal" aria-label="Canal 1. Cambiar de canal">1</button>' +
      '<p class="nav-lateral__grupo">Clase</p>' +
      enlace("alumnos", "alumnos.html", "alumnos", "Alumnos") +
      enlace("codigo", "codigo.html", "codigo", "Código") +
      '<p class="nav-lateral__grupo">Pruebas</p>' +
      enlace("simulada", "alumnos.html?escena=simulada", "simulado", "Aula simulada") +
      '<div class="nav-lateral__pie">' +
      enlace("ayuda", "ayuda.html", "ayuda", "Ayuda") +
      '<button type="button" class="nav-lateral__boton" data-dialogo="ajustes">' + icono("ajustes") +
      '<span class="nav-lateral__texto">Ajustes</span></button></div>';

    document.body.insertAdjacentHTML("beforeend", `
  <dialog class="modal-ajustes" id="ajustes" aria-labelledby="aj-titulo">
    <div class="modal-ajustes__lado">
      <h2 id="aj-titulo">Ajustes</h2>
      <div class="modal-ajustes__pestanas" role="tablist" aria-orientation="vertical" aria-labelledby="aj-titulo">
      <button type="button" role="tab" id="tab-clase" aria-controls="aj-clase" aria-selected="true">${icono("alumnos")}Clase</button>
      <button type="button" role="tab" id="tab-apariencia" aria-controls="aj-apariencia" aria-selected="false" tabindex="-1">${icono("sol")}Apariencia</button>
      <button type="button" role="tab" id="tab-acerca" aria-controls="aj-acerca" aria-selected="false" tabindex="-1">${icono("info")}Acerca de</button>
      </div>
    </div>
    <div class="modal-ajustes__contenido">
      <button type="button" class="btn btn--fantasma btn--icono modal-ajustes__cerrar" data-cerrar-dialogo aria-label="Cerrar ajustes">${icono("cerrar")}</button>

      <section role="tabpanel" id="aj-clase" aria-labelledby="tab-clase">
        <h3>Clase</h3>
        <div class="ajuste">
          <div class="ajuste__texto"><strong>Tu nombre: Prof. Ayala</strong><span>Lo ven los alumnos arriba de su ventana de Aula.</span></div>
          <button type="button" class="btn" data-dialogo="cambiar-nombre">Cambiar…</button>
        </div>
        <div class="ajuste">
          <div class="ajuste__texto"><strong>Canal: 1</strong><span>El número de tu sala. Ves a los alumnos cuyas computadoras tienen ese mismo número.</span></div>
          <button type="button" class="btn" data-dialogo="cambiar-canal">Cambiar…</button>
        </div>
        <div class="ajuste">
          <div class="ajuste__texto"><strong>Canal al abrir la consola</strong><span>Lo fijó el administrador al instalar Aula en este equipo.</span></div>
          <span class="insignia">${icono("candado")}Canal 1</span>
        </div>
      </section>

      <section role="tabpanel" id="aj-apariencia" aria-labelledby="tab-apariencia" hidden>
        <h3>Apariencia</h3>
        <div class="ajuste">
          <div class="ajuste__texto"><strong>Tema</strong><span>«Sistema» sigue la configuración de Windows.</span></div>
          <div class="opciones-tarjeta" role="radiogroup" aria-label="Tema" style="grid-column: 1 / -1">
            <button type="button" class="opcion-tarjeta" role="radio" data-tema="sistema">${icono("ajustes")}Sistema</button>
            <button type="button" class="opcion-tarjeta" role="radio" data-tema="light">${icono("sol")}Claro</button>
            <button type="button" class="opcion-tarjeta" role="radio" data-tema="dark">${icono("luna")}Oscuro</button>
          </div>
        </div>
      </section>

      <section role="tabpanel" id="aj-acerca" aria-labelledby="tab-acerca" hidden>
        <h3>Acerca de</h3>
        <div class="ajuste">
          <div class="ajuste__texto"><strong>Versión</strong><span>Aula Consola 0.6.0-dev</span></div>
        </div>
        <div class="ajuste">
          <div class="ajuste__texto"><strong>Clave del centro</strong>
            <span>Prueba que esta consola es de tu centro de cómputo: los alumnos solo obedecen a consolas con esta clave. Para comprobarla con el administrador, compárenla de palabra.</span></div>
          <span class="insignia insignia--conectado">${icono("llave")}<span class="mono">K7QD-3MX2-9PLA-W4TZ</span></span>
        </div>
        <div class="ajuste">
          <div class="ajuste__texto"><strong>Datos para quien instala</strong><span>Dáselos al responsable de TI si un alumno no aparece.</span></div>
          <div class="bloque-codigo">
            <div class="bloque-codigo__cabecera"><span>Esta consola</span>
              <button type="button" class="btn btn--fantasma btn--sm" data-copiar="#datos-ajustes">${icono("copiar")}Copiar</button></div>
<pre id="datos-ajustes">Canal: 1
Equipo: PROF-LAB3 (192.168.1.20)
Puerto: TCP 47830, UDP 47831
Red de Windows: Privada
Clave del centro: K7QD-3MX2-9PLA-W4TZ
Aula Consola 0.6.0-dev</pre>
          </div>
        </div>
      </section>
    </div>
  </dialog>

  <dialog class="dialogo" id="cambiar-canal" aria-labelledby="cc-titulo">
    <div class="dialogo__cuerpo">
      <h2 id="cc-titulo">Cambiar de canal</h2>
      <label class="campo"><span class="campo__etiqueta">Canal nuevo</span>
        <input class="mono" inputmode="numeric" value="2" aria-describedby="cc-ayuda">
        <span class="campo__ayuda" id="cc-ayuda">Del 1 al 16000.</span>
      </label>
      <div class="grupo-botones"><span class="suave pequeno">Recientes:</span><button type="button" class="btn btn--sm">Canal 2</button><button type="button" class="btn btn--sm">Canal 14</button></div>
      <p class="dialogo__consecuencia">${icono("info")}<span>Los 7 alumnos del canal 1 dejarán de verte y verás a los del canal 2. Se quitan las restricciones y se detiene la transmisión. Tu historial de código se conserva.</span></p>
    </div>
    <div class="dialogo__acciones">
      <button type="button" class="btn" data-cerrar-dialogo autofocus>Volver</button>
      <button type="button" class="btn btn--primario" data-cerrar-dialogo>Cambiar al canal 2</button>
    </div>
  </dialog>

  <dialog class="dialogo" id="cambiar-nombre" aria-labelledby="cn-titulo">
    <div class="dialogo__cuerpo">
      <h2 id="cn-titulo">Cambiar tu nombre</h2>
      <label class="campo"><span class="campo__etiqueta">Como lo verán los alumnos</span><input value="Prof. Ayala" maxlength="63"></label>
      <p class="dialogo__consecuencia">${icono("info")}<span>Los alumnos se reconectan solos en unos segundos para ver el nombre nuevo. Se quitan las restricciones y se detiene la transmisión. Tu historial de código se conserva.</span></p>
    </div>
    <div class="dialogo__acciones">
      <button type="button" class="btn" data-cerrar-dialogo autofocus>Volver</button>
      <button type="button" class="btn btn--primario" data-cerrar-dialogo>Guardar nombre</button>
    </div>
  </dialog>`);

    // Pestañas de Ajustes: clic y flechas arriba/abajo.
    const tabs = $$("#ajustes [role=tab]");
    const elegir = (t, foco) => {
      for (const o of tabs) {
        const on = o === t;
        o.setAttribute("aria-selected", String(on));
        o.tabIndex = on ? 0 : -1;
        $("#" + o.getAttribute("aria-controls")).hidden = !on;
      }
      if (foco) t.focus();
    };
    tabs.forEach((t, i) => {
      t.addEventListener("click", () => elegir(t, true));
      t.addEventListener("keydown", (e) => {
        if (e.key === "ArrowDown" || e.key === "ArrowUp") {
          e.preventDefault();
          elegir(tabs[(i + (e.key === "ArrowDown" ? 1 : tabs.length - 1)) % tabs.length], true);
        }
      });
    });
    const t = params.get("ajustes") && $("#tab-" + params.get("ajustes"));
    if (t) elegir(t, false);
  }
  for (const b of $$("[data-tema]")) b.addEventListener("click", () => ponerTema(b.dataset.tema === "sistema" ? "" : b.dataset.tema));
  ponerTema(temaUrl || (temaGuardado === "sistema" ? "" : temaGuardado));

  // --- Datos de demostración -------------------------------------------------------------
  const ALUMNOS = [
    { id: "pc01", equipo: "LAB3-PC01", alumno: "Ana López", estado: "conectado", codigo: "Ejercicio 3", hace: "hace 1 min" },
    { id: "pc02", equipo: "LAB3-PC02", alumno: "Bruno Díaz", estado: "conectado", codigo: "Ejercicio 3", hace: "hace 1 min" },
    { id: "pc03", equipo: "LAB3-PC03", alumno: "", estado: "desconectado", codigo: "", hace: "hace 12 min", motivo: "Dejó de responder a las 10:31" },
    { id: "pc04", equipo: "LAB3-PC04", alumno: "Carla Ruiz", estado: "conectado", codigo: "Ejercicio 2", hace: "hace 6 min", aviso: "No recibió el ejercicio 3" },
    { id: "pc05", equipo: "LAB3-PC05", alumno: "Diego Peña", estado: "conectado", codigo: "Ejercicio 3", hace: "hace 1 min", restr: true },
    { id: "pc06", equipo: "LAB3-PC06", alumno: "", estado: "conectado", codigo: "", hace: "", sinSesion: true },
    { id: "pc07", equipo: "LAB3-PC07", alumno: "Elena Soto", estado: "conectado", codigo: "Ejercicio 3", hace: "hace 1 min" },
    { id: "ub02", equipo: "lab-ubuntu-02", alumno: "María Gil", estado: "conectado", codigo: "Ejercicio 3", hace: "hace 1 min", linux: true },
  ];
  const SIMULADOS = [
    { id: "sim1", equipo: "Simulado 1", alumno: "Alumno de prueba", estado: "conectado", codigo: "Ejercicio 3", hace: "hace 20 s", sim: true },
    { id: "sim2", equipo: "Simulado 2", alumno: "Alumno de prueba", estado: "conectado", codigo: "", hace: "", sim: true },
  ];
  const escena = params.get("escena") || "";
  // Aula simulada: una sola lista, con los simulados al final.
  const lista = escena === "simulada" ? ALUMNOS.concat(SIMULADOS) : escena === "vacia" ? [] : ALUMNOS;
  for (const el of $$("[data-solo-escena]")) el.hidden = el.dataset.soloEscena !== escena;

  const insigniaEstado = (a) => a.estado === "conectado"
    ? '<span class="insignia insignia--conectado">' + icono("conectado") + "Conectado</span>"
    : '<span class="insignia insignia--desconectado">' + icono("desconectado") + "Desconectado</span>";
  const notas = (a) => [
    a.sim ? '<span class="insignia insignia--simulado">' + icono("simulado") + "Simulado</span>" : "",
    a.restr ? '<span class="insignia insignia--advertencia">' + icono("candado") + "Restricciones</span>" : "",
    a.aviso ? '<span class="insignia insignia--advertencia">' + icono("advertencia") + a.aviso + "</span>" : "",
    a.sinSesion ? '<span class="insignia">' + icono("info") + "Sin sesión iniciada: solo energía</span>" : "",
    a.linux ? '<span class="insignia">' + icono("info") + "Linux: no comparte pantalla</span>" : "",
    a.motivo ? '<span class="insignia insignia--desconectado">' + a.motivo + "</span>" : "",
  ].join(" ");

  // --- Vista Alumnos: lista o miniaturas con selección --------------------------------------
  const seleccion = new Set((params.get("sel") || "").split(",").filter(Boolean));
  const tbody = $("#filas");
  const grid = $("#miniaturas");
  const reales = lista.filter((a) => !a.sim);
  const sims = lista.filter((a) => a.sim);

  function pintarResumen() {
    const el = $("#resumen");
    if (!el) return;
    const con = reales.filter((a) => a.estado === "conectado").length;
    const des = reales.length - con;
    el.innerHTML =
      '<span class="insignia insignia--conectado">' + icono("conectado") + con + " conectados</span>" +
      (des ? '<span class="insignia insignia--desconectado">' + icono("desconectado") + des + " desconectado" + (des === 1 ? "" : "s") + "</span>" : "") +
      (sims.length ? '<span class="insignia insignia--simulado">' + icono("simulado") + sims.length + " simulados</span>" : "");
  }

  const fila = (a) =>
    '<tr data-id="' + a.id + '" data-seleccionado="' + seleccion.has(a.id) + '">' +
    '<td class="col-marca"><input type="checkbox" aria-label="Seleccionar ' + a.equipo + '"' + (seleccion.has(a.id) ? " checked" : "") + "></td>" +
    '<td class="celda-principal" data-etiqueta="Equipo">' + a.equipo +
    '<span class="celda-sub">' + (a.alumno || (a.sinSesion ? "Sin sesión iniciada" : "—")) + "</span></td>" +
    '<td data-etiqueta="Estado">' + insigniaEstado(a) + "</td>" +
    '<td data-etiqueta="Código en pantalla">' + (a.codigo ? a.codigo + ' <span class="suave pequeno">' + a.hace + "</span>" : '<span class="suave">—</span>') + "</td>" +
    '<td data-etiqueta="Notas">' + (notas(a) || '<span class="suave">—</span>') + "</td></tr>";
  const tarjeta = (a, i) => {
    const pantalla = a.estado !== "conectado"
      ? icono("desconectado") + "<span>Desconectado</span>"
      : a.linux ? icono("info") + "<span>Linux: aún no comparte pantalla</span>"
      : a.sinSesion ? icono("info") + "<span>Sin sesión iniciada</span>"
      : '<img alt="Pantalla de ' + a.equipo + '" src="miniaturas/' + (i % 4) + '.svg">';
    return '<li class="alumno' + (a.estado !== "conectado" ? " alumno--desconectado" : "") + '" data-id="' + a.id +
      '" data-seleccionado="' + seleccion.has(a.id) + '">' +
      '<input class="alumno__marca" type="checkbox" aria-label="Seleccionar ' + a.equipo + '"' + (seleccion.has(a.id) ? " checked" : "") + ">" +
      '<div class="alumno__pantalla">' + pantalla + "</div>" +
      '<div class="alumno__pie"><span class="alumno__nombre">' + a.equipo + "</span>" +
      (a.sim ? '<span class="insignia insignia--simulado">' + icono("simulado") + "Simulado</span>" : insigniaEstado(a)) + "</div>" +
      '<div class="alumno__sub">' + (a.alumno || "Sin sesión iniciada") + (a.codigo ? " · " + a.codigo : "") + "</div></li>";
  };
  const grupoSim = (tabla) => tabla
    ? '<tr class="tabla__grupo"><th colspan="5" scope="rowgroup">' + icono("simulado") + " Alumnos simulados" +
      '<span class="insignia insignia--simulado">' + sims.length + " de prueba, no son equipos reales</span></th></tr>"
    : '<li class="cuadricula-alumnos__grupo">' + icono("simulado") + "Alumnos simulados · de prueba, no son equipos reales</li>";

  function pintar() {
    if (tbody) tbody.innerHTML = reales.map(fila).join("") + (sims.length ? grupoSim(true) + sims.map(fila).join("") : "");
    if (grid) grid.innerHTML = reales.map(tarjeta).join("") + (sims.length ? grupoSim(false) + sims.map((a, i) => tarjeta(a, i + 1)).join("") : "");
    const n = seleccion.size;
    const sel = $("#seleccion");
    if (sel) sel.textContent = n === 0 ? "Sin selección: las acciones van a todos los conectados"
      : n + (n === 1 ? " alumno seleccionado" : " alumnos seleccionados");
    const todos = $("#todos");
    if (todos) { todos.checked = n > 0 && n === lista.length; todos.indeterminate = n > 0 && n < lista.length; }
    const bt = $("#seleccionar-todos");
    if (bt) {
      $(".btn__texto", bt).textContent = n > 0 ? "Quitar selección" : "Seleccionar todos";
      $("use", bt).setAttribute("href", ICONOS + (n > 0 ? "#cerrar" : "#hecho"));
    }
    const conectados = lista.filter((a) => a.estado === "conectado").length;
    for (const el of $$("[data-destino]")) el.textContent = n === 0 ? "todos los conectados (" + conectados + ")" : n + (n === 1 ? " alumno seleccionado" : " alumnos seleccionados");
    pintarResumen();
  }

  function alternar(id) {
    if (seleccion.has(id)) seleccion.delete(id); else seleccion.add(id);
    pintar();
  }
  function seleccionarTodos(si) {
    seleccion.clear();
    if (si) lista.forEach((a) => seleccion.add(a.id));
    pintar();
  }
  for (const cont of [tbody, grid].filter(Boolean)) {
    cont.addEventListener("click", (e) => {
      const f = e.target.closest("[data-id]");
      if (f) alternar(f.dataset.id);
    });
  }
  const todos = $("#todos");
  if (todos) todos.addEventListener("change", () => seleccionarTodos(todos.checked));
  const bt = $("#seleccionar-todos");
  if (bt) bt.addEventListener("click", () => seleccionarTodos(seleccion.size === 0));
  // Ctrl+A en la vista de alumnos selecciona a todos (fuera de los campos de texto).
  if (tbody) document.addEventListener("keydown", (e) => {
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "a" && !e.target.closest("input, textarea, select, [contenteditable], dialog")) {
      e.preventDefault();
      seleccionarTodos(true);
    }
  });

  // Lista | Miniaturas (?modo=miniaturas)
  function modo(m) {
    for (const b of $$("[data-modo]")) b.setAttribute("aria-pressed", String(b.dataset.modo === m));
    const t = $("#vista-lista"), g = $("#vista-miniaturas");
    if (t) t.hidden = m !== "lista";
    if (g) g.hidden = m !== "miniaturas";
  }
  for (const b of $$("[data-modo]")) b.addEventListener("click", () => modo(b.dataset.modo));
  if ($("[data-modo]")) modo(params.get("modo") || "lista");

  // --- Paneles laterales (?panel=energia) ----------------------------------------------------
  function abrirPanel(id) {
    for (const p of $$(".panel-lateral")) p.hidden = p.id !== "panel-" + id;
    for (const b of $$("[data-panel]")) b.setAttribute("aria-expanded", String(b.dataset.panel === id));
    const p = $("#panel-" + id);
    if (p) $("h2", p).focus();
  }
  function cerrarPanel() {
    for (const p of $$(".panel-lateral")) p.hidden = true;
    for (const b of $$("[data-panel]")) b.setAttribute("aria-expanded", "false");
  }
  for (const b of $$("[data-panel]")) b.addEventListener("click", () => abrirPanel(b.dataset.panel));
  for (const b of $$("[data-cerrar-panel]")) b.addEventListener("click", cerrarPanel);
  document.addEventListener("keydown", (e) => { if (e.key === "Escape" && !$("dialog[open]")) cerrarPanel(); });
  if (params.get("panel")) abrirPanel(params.get("panel"));

  // --- Diálogos (?dialogo=id) ------------------------------------------------------------------
  document.addEventListener("click", (e) => {
    const abrir = e.target.closest("[data-dialogo]");
    if (abrir) {
      const abierto = $("dialog[open]");
      if (abierto && abierto.id !== abrir.dataset.dialogo) abierto.close();
      $("#" + abrir.dataset.dialogo).showModal();
      return;
    }
    const cerrar = e.target.closest("[data-cerrar-dialogo]");
    if (cerrar) cerrar.closest("dialog").close();
  });
  if (params.get("dialogo")) $("#" + params.get("dialogo")).showModal();

  // --- Avisos flotantes ---------------------------------------------------------------------------
  function toast(tipo, titulo, texto) {
    const zona = $("#avisos");
    const id = { exito: "conectado", error: "error", advertencia: "advertencia", info: "info" }[tipo];
    const el = document.createElement("div");
    el.className = "toast toast--" + tipo;
    el.setAttribute("role", tipo === "error" ? "alert" : "status");
    el.innerHTML = icono(id) + '<div class="toast__texto"><span class="toast__titulo"></span><span class="pequeno"></span></div>' +
      '<button class="btn btn--fantasma btn--icono btn--sm" aria-label="Cerrar">' + icono("cerrar") + "</button>";
    $(".toast__titulo", el).textContent = titulo;
    $(".pequeno", el).textContent = texto;
    $("button", el).addEventListener("click", () => el.remove());
    zona.append(el);
    if (tipo !== "error") setTimeout(() => el.remove(), 6000); // los errores se quedan
  }
  for (const b of $$("[data-toast]")) {
    b.addEventListener("click", () => {
      const [tipo, titulo, texto] = b.dataset.toast.split("|");
      toast(tipo, titulo, texto || "");
    });
  }
  if (params.get("toast")) {
    const b = $('[data-toast^="' + params.get("toast") + '"]');
    if (b) b.click();
  }

  // --- Copiar -------------------------------------------------------------------------------------
  document.addEventListener("click", (e) => {
    const b = e.target.closest("[data-copiar]");
    if (!b) return;
    const txt = $(b.dataset.copiar).textContent;
    (navigator.clipboard ? navigator.clipboard.writeText(txt) : Promise.reject()).then(
      () => toast("exito", "Copiado", "Pégalo donde lo necesites."),
      () => toast("error", "No se pudo copiar", "Selecciona el texto y usa Ctrl+C."));
  });

  // --- Visor de diagramas: zoom ---------------------------------------------------------------------
  for (const v of $$(".visor-diagrama")) {
    let z = 1;
    const lienzo = $(".visor-diagrama__lienzo", v);
    for (const b of $$("[data-zoom]", v)) {
      b.addEventListener("click", () => {
        z = b.dataset.zoom === "mas" ? Math.min(3, z + 0.25) : b.dataset.zoom === "menos" ? Math.max(0.5, z - 0.25) : 1;
        lienzo.style.setProperty("--zoom", String(z));
      });
    }
  }

  // --- Desglose de acuses (Código) ------------------------------------------------------------------
  for (const b of $$("[data-desplegar]")) {
    b.addEventListener("click", () => {
      const el = $(b.dataset.desplegar);
      el.hidden = !el.hidden;
      b.setAttribute("aria-expanded", String(!el.hidden));
    });
  }

  // --- Ayuda: filtrar el índice ---------------------------------------------------------------------
  const buscar = $("#buscar-manual");
  if (buscar) {
    buscar.addEventListener("input", () => {
      const q = buscar.value.trim().toLowerCase();
      for (const li of $$(".indice-manual > li")) {
        const seccion = $("h2", li).textContent.toLowerCase().includes(q);
        let visibles = 0;
        for (const a of $$("ul li", li)) {
          const ok = !q || seccion || a.textContent.toLowerCase().includes(q);
          a.hidden = !ok;
          if (ok) visibles++;
        }
        li.hidden = visibles === 0;
      }
      const vacio = $("#sin-resultados");
      if (vacio) vacio.hidden = $$(".indice-manual > li").some((li) => !li.hidden);
    });
  }

  pintar();
})();
