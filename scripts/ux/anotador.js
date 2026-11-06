// Overlay del anotador de interfaz (lo inyecta anotar.mjs; no es parte de las apps). Vive en un
// shadow DOM para no tocar los estilos de la app. Con «Anotar» (tecla A), al pasar el mouse
// resalta el elemento y dice en qué zona está (zonas.mjs); un clic abre el formulario de la nota.
// Guardar, listar, borrar y cambiar de escena los hace Node por funciones expuestas (__anotador*).
(function () {
  if (window.top !== window || window.__anotadorListo) return;
  window.__anotadorListo = true;
  const C = window.__ANOTADOR; // { app, escena, ruta, tema, zonas, escenas, rutas }
  const TIPOS = [
    ["mover", "Mover de lugar"], ["texto", "Cambiar texto"], ["estilo", "Estilo / diseño"],
    ["agregar", "Agregar algo"], ["quitar", "Quitar"], ["error", "Algo falla"], ["idea", "Idea / duda"],
  ];
  const NOMBRE_APP = { profesor: "Consola", alumno: "Alumno" };

  function iniciar() {
    const host = document.createElement("div");
    host.id = "anotador-host";
    host.style.cssText = "position:fixed;left:0;top:0;width:0;height:0;z-index:2147483647";
    // Fuera de las pruebas de accesibilidad y del flujo de la app.
    host.setAttribute("aria-hidden", "false");
    document.documentElement.appendChild(host);
    const raiz = host.attachShadow({ mode: "open" });
    raiz.innerHTML = `
<style>
  :host { all: initial; }
  * { box-sizing: border-box; font: 13px/1.35 "Segoe UI", system-ui, sans-serif; }
  .barra { position: fixed; left: 12px; bottom: 12px; z-index: 2147483646; display: flex; flex-wrap: wrap; gap: 6px;
    align-items: center; max-width: calc(100vw - 24px); padding: 8px 10px; border-radius: 10px; background: #1f2433;
    color: #f2f4f8; box-shadow: 0 6px 24px rgba(0,0,0,.35); }
  .barra strong { margin-right: 4px; }
  select, button, textarea { border-radius: 6px; border: 1px solid #4a5268; background: #2b3245; color: #f2f4f8; padding: 4px 8px; }
  button { cursor: pointer; }
  button.activo { background: #f59e0b; color: #1f2433; border-color: #f59e0b; font-weight: 600; }
  button.primario { background: #3b82f6; border-color: #3b82f6; }
  .caja { position: fixed; z-index: 2147483645; pointer-events: none; border: 2px solid #f59e0b; border-radius: 4px;
    background: rgba(245,158,11,.12); display: none; }
  .etiqueta { position: fixed; z-index: 2147483646; pointer-events: none; display: none; max-width: 60vw; padding: 4px 8px;
    border-radius: 6px; background: #f59e0b; color: #1f2433; font-weight: 600; }
  .etiqueta small { display: block; font-weight: 400; }
  .ventana { position: fixed; z-index: 2147483647; right: 16px; top: 16px; width: min(440px, calc(100vw - 32px));
    max-height: calc(100vh - 32px); overflow: auto; padding: 14px; border-radius: 10px; background: #1f2433;
    color: #f2f4f8; box-shadow: 0 10px 40px rgba(0,0,0,.45); display: none; }
  .ventana h2 { margin: 0 0 6px; font-size: 15px; font-weight: 600; }
  .donde { margin: 0 0 10px; color: #c7cede; }
  .tipos { display: flex; flex-wrap: wrap; gap: 6px; margin-bottom: 10px; }
  .tipos label { display: inline-flex; gap: 4px; align-items: center; padding: 3px 8px; border: 1px solid #4a5268; border-radius: 999px; }
  .tipos input:checked + span { color: #f59e0b; font-weight: 600; }
  textarea { width: 100%; min-height: 110px; resize: vertical; }
  .acciones { display: flex; gap: 6px; justify-content: flex-end; margin-top: 10px; }
  .ayuda { color: #9aa3b8; font-size: 12px; margin-top: 6px; }
  ol { margin: 0; padding-left: 20px; } li { margin: 6px 0; } li small { color: #9aa3b8; display: block; }
  .ok { position: fixed; left: 50%; top: 16px; transform: translateX(-50%); z-index: 2147483647; padding: 8px 14px;
    border-radius: 8px; background: #16a34a; color: #fff; display: none; }
</style>
<div class="caja"></div>
<div class="etiqueta"></div>
<div class="ok"></div>
<div class="barra" role="toolbar" aria-label="Anotador de interfaz">
  <strong>Anotador</strong>
  <select id="app" title="App"></select>
  <select id="escena" title="Escena (datos de demostración)"></select>
  <select id="ruta" title="Vista de la consola"></select>
  <select id="tema" title="Tema"><option value="claro">Claro</option><option value="oscuro">Oscuro</option></select>
  <button id="anotar" title="Señala un elemento y escribe tu nota (tecla A)">Anotar (A)</button>
  <button id="general" title="Una nota sobre toda la pantalla">Nota de la pantalla</button>
  <button id="ver">Notas</button>
  <button id="ocultar" title="Esconde la barra (tecla H)">Ocultar (H)</button>
</div>
<form class="ventana" id="form">
  <h2>Nueva nota</h2>
  <p class="donde" id="donde"></p>
  <div class="tipos" id="tipos"></div>
  <textarea id="texto" placeholder="¿Qué quieres? Por ejemplo: «Esto debería ser un módulo propio en el menú» o «una grilla de iconos con tooltip»."></textarea>
  <p class="ayuda">Ctrl+Enter guarda · Esc cancela. Se guarda con una captura del elemento y de la pantalla.</p>
  <div class="acciones"><button type="button" id="cancelar">Cancelar</button><button type="submit" class="primario">Guardar nota</button></div>
</form>
<div class="ventana" id="lista"><h2>Notas guardadas</h2><ol id="notas"></ol>
  <div class="acciones"><button type="button" id="cerrar-lista">Cerrar</button></div></div>`;
    const $ = (s) => raiz.querySelector(s);
    const caja = $(".caja"), etiqueta = $(".etiqueta"), barra = $(".barra"), form = $("#form"), lista = $("#lista");

    // --- Selectores de la barra -------------------------------------------------------------
    const opciones = (sel, pares, actual) => {
      sel.replaceChildren(...pares.map(([v, t]) => Object.assign(document.createElement("option"), { value: v, textContent: t })));
      sel.value = actual;
    };
    opciones($("#app"), Object.entries(NOMBRE_APP), C.app);
    opciones($("#escena"), Object.entries(C.escenas[C.app]).map(([k, v]) => [k, v + " (" + k + ")"]), C.escena);
    opciones($("#ruta"), Object.keys(C.rutas).map((k) => [C.rutas[k], "Vista: " + k]), C.ruta);
    $("#ruta").hidden = C.app !== "profesor";
    $("#tema").value = C.tema;
    const ir = (cambio) => window.__anotadorIr(Object.assign({ app: C.app, escena: C.escena, ruta: location.hash.slice(1), tema: C.tema }, cambio));
    $("#app").onchange = (e) => ir({ app: e.target.value, escena: Object.keys(C.escenas[e.target.value])[0], ruta: "" });
    $("#escena").onchange = (e) => ir({ escena: e.target.value });
    $("#ruta").onchange = (e) => ir({ ruta: e.target.value });
    $("#tema").onchange = (e) => ir({ tema: e.target.value });

    // --- Zonas ----------------------------------------------------------------------------------
    const zonas = C.zonas[C.app];
    const porId = Object.fromEntries(zonas.map((z) => [z.id, z]));
    const profundidad = (el) => { let n = 0; for (let e = el; e; e = e.parentElement) n++; return n; };
    function zonaDe(el) {
      let mejor = null, prof = -1;
      for (const z of zonas) {
        let m = null;
        try { m = el.closest(z.selector); } catch { continue; }
        if (m) { const d = profundidad(m); if (d > prof) { mejor = z; prof = d; } }
      }
      return mejor;
    }
    function cadena(z) {
      const nombres = [];
      for (let x = z; x; x = x.dentro ? porId[x.dentro] : null) nombres.unshift(x.nombre);
      return [NOMBRE_APP[C.app]].concat(nombres).join(" › ");
    }
    // Cómo se llama el elemento: su texto visible, su etiqueta o su clave de textos.
    function describir(el) {
      const t = (s) => (s || "").replace(/\s+/g, " ").trim().slice(0, 80);
      const clave = el.closest("[data-t],[data-t-aria-label]");
      return {
        etiqueta: el.tagName.toLowerCase() + (el.id ? "#" + el.id : ""),
        texto: t(el.getAttribute("aria-label") || el.title || el.textContent),
        claveTexto: clave ? clave.dataset.t || clave.dataset.tAriaLabel || "" : "",
        selector: selectorDe(el),
      };
    }
    function selectorDe(el) {
      const partes = [];
      for (let e = el; e && e !== document.body && partes.length < 5; e = e.parentElement) {
        if (e.id) { partes.unshift("#" + CSS.escape(e.id)); break; }
        const hermanos = e.parentElement ? [...e.parentElement.children].filter((x) => x.tagName === e.tagName) : [];
        const n = hermanos.length > 1 ? `:nth-of-type(${hermanos.indexOf(e) + 1})` : "";
        const clase = [...e.classList].filter((c) => !c.startsWith("anot")).slice(0, 2).map((c) => "." + CSS.escape(c)).join("");
        partes.unshift(e.tagName.toLowerCase() + clase + n);
      }
      return partes.join(" > ");
    }

    // --- Modo anotar ------------------------------------------------------------------------
    let anotando = false, objetivo = null;
    const propio = (e) => e.composedPath().includes(host);
    // Se señala el control completo (un botón, no el texto de adentro).
    const elegir = (el) => el.closest("button, a, input, select, textarea, summary, label, [role=button], [role=tab], [role=menuitem]") || el;
    function marcar(el) {
      const r = el.getBoundingClientRect();
      Object.assign(caja.style, { display: "block", left: r.left - 2 + "px", top: r.top - 2 + "px", width: r.width + 4 + "px", height: r.height + 4 + "px" });
      const z = zonaDe(el), d = describir(el);
      etiqueta.innerHTML = "";
      etiqueta.append(z ? cadena(z) : NOMBRE_APP[C.app] + " › (zona sin nombre)");
      const small = document.createElement("small");
      small.textContent = d.texto ? "«" + d.texto + "»" : d.etiqueta;
      etiqueta.append(small);
      etiqueta.style.display = "block";
      const arriba = r.top > 60;
      etiqueta.style.left = Math.max(4, Math.min(r.left, innerWidth - etiqueta.offsetWidth - 4)) + "px";
      etiqueta.style.top = (arriba ? r.top - etiqueta.offsetHeight - 6 : r.bottom + 6) + "px";
    }
    function soltarMarca() { caja.style.display = "none"; etiqueta.style.display = "none"; }
    function modo(on) {
      anotando = on;
      $("#anotar").classList.toggle("activo", on);
      document.documentElement.style.cursor = on ? "crosshair" : "";
      if (!on) soltarMarca();
    }
    document.addEventListener("mousemove", (e) => { if (anotando && !propio(e)) marcar(elegir(e.target)); }, true);
    // Mientras se anota, ningún clic llega a la app.
    for (const tipo of ["click", "mousedown", "mouseup", "pointerdown", "pointerup", "dblclick", "contextmenu"]) {
      document.addEventListener(tipo, (e) => {
        if (!anotando || propio(e)) return;
        e.preventDefault(); e.stopPropagation(); e.stopImmediatePropagation();
        if (tipo === "click") { objetivo = elegir(e.target); modo(false); marcar(objetivo); abrirForm(objetivo); }
      }, true);
    }
    $("#anotar").onclick = () => modo(!anotando);
    $("#general").onclick = () => { modo(false); objetivo = null; soltarMarca(); abrirForm(null); };
    $("#ocultar").onclick = () => { barra.hidden = !barra.hidden; };
    document.addEventListener("keydown", (e) => {
      if (propio(e) && e.key !== "Escape") return;
      const escribiendo = /input|textarea|select/i.test((e.composedPath()[0] || {}).tagName || "") || document.activeElement?.isContentEditable;
      if (e.key === "Escape" && (anotando || form.style.display === "block")) { modo(false); cerrarForm(); e.stopPropagation(); return; }
      if (escribiendo || e.ctrlKey || e.altKey || e.metaKey) return;
      if (e.key === "a" || e.key === "A") { modo(!anotando); e.preventDefault(); }
      if (e.key === "h" || e.key === "H") { barra.hidden = !barra.hidden; e.preventDefault(); }
    }, true);

    // --- Formulario -------------------------------------------------------------------------
    $("#tipos").append(...TIPOS.map(([v, t], i) => {
      const l = document.createElement("label");
      l.innerHTML = `<input type="radio" name="tipo" value="${v}"${i === 0 ? " checked" : ""}><span></span>`;
      l.querySelector("span").textContent = t;
      return l;
    }));
    let contexto = null;
    function abrirForm(el) {
      const z = el ? zonaDe(el) : null;
      const d = el ? describir(el) : null;
      contexto = { zona: z ? { id: z.id, nombre: z.nombre, cadena: cadena(z) } : { id: "", nombre: "Pantalla completa", cadena: NOMBRE_APP[C.app] + " › Pantalla completa" },
        elemento: d };
      $("#donde").textContent = contexto.zona.cadena + (d && d.texto ? " · «" + d.texto + "»" : "");
      // Del lado contrario al elemento, para no taparlo.
      const derecha = !el || el.getBoundingClientRect().left + el.getBoundingClientRect().width / 2 < innerWidth / 2;
      form.style.left = derecha ? "auto" : "16px";
      form.style.right = derecha ? "16px" : "auto";
      form.style.display = "block";
      lista.style.display = "none";
      $("#texto").focus({ preventScroll: true });
    }
    function cerrarForm() { form.style.display = "none"; $("#texto").value = ""; soltarMarca(); objetivo = null; }
    $("#cancelar").onclick = cerrarForm;
    $("#texto").addEventListener("keydown", (e) => { if (e.key === "Enter" && e.ctrlKey) { e.preventDefault(); form.requestSubmit(); } });
    form.addEventListener("submit", async (e) => {
      e.preventDefault();
      const texto = $("#texto").value.trim();
      if (!texto) { $("#texto").focus(); return; }
      const tipo = raiz.querySelector("input[name=tipo]:checked").value;
      // La captura no lleva la barra ni el formulario; sí el recuadro del elemento señalado.
      form.style.display = "none"; barra.style.visibility = "hidden"; etiqueta.style.display = "none";
      let rect = null;
      if (objetivo) {
        marcar(objetivo); etiqueta.style.display = "none";
        // El recorte muestra la zona que contiene al elemento (si cabe en la ventana), con el
        // elemento resaltado adentro: así se entiende dónde está.
        let r = objetivo.getBoundingClientRect();
        const z = zonaDe(objetivo);
        const cont = z ? objetivo.closest(z.selector) : null;
        if (cont) {
          const rz = cont.getBoundingClientRect();
          if (rz.width * rz.height < innerWidth * innerHeight * 0.6) r = rz;
        }
        rect = { x: r.left, y: r.top, w: r.width, h: r.height, sx: scrollX, sy: scrollY };
      }
      await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
      let res;
      try {
        res = await window.__anotadorGuardar({ tipo, texto, zona: contexto.zona, elemento: contexto.elemento, rect,
          ruta: location.hash.slice(1), vista: { ancho: innerWidth, alto: innerHeight } });
      } finally { barra.style.visibility = ""; }
      cerrarForm();
      aviso("Nota " + res.n + " guardada");
    });
    function aviso(t) { const o = $(".ok"); o.textContent = t; o.style.display = "block"; setTimeout(() => { o.style.display = "none"; }, 2200); }

    // --- Lista de notas ----------------------------------------------------------------------
    $("#ver").onclick = async () => {
      const notas = await window.__anotadorListar();
      $("#notas").replaceChildren(...notas.map((n) => {
        const li = document.createElement("li");
        li.textContent = "[" + n.tipo + "] " + n.texto;
        const s = document.createElement("small");
        s.textContent = "#" + n.n + " · " + n.zona.cadena + " · " + n.escena + " · " + n.estado;
        const b = document.createElement("button");
        b.textContent = "Borrar";
        b.onclick = async () => { await window.__anotadorBorrar(n.n); $("#ver").onclick(); };
        li.append(s, b);
        return li;
      }));
      if (!notas.length) $("#notas").textContent = "Todavía no hay notas.";
      form.style.display = "none";
      lista.style.display = "block";
    };
    $("#cerrar-lista").onclick = () => { lista.style.display = "none"; };
  }

  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", iniciar);
  else iniciar();
})();
