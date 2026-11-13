// Overlay del anotador de interfaz (lo inyecta anotar.mjs; no es parte de las apps). Vive en un
// shadow DOM para no tocar los estilos de la app.
//
// Mientras navegas, cada clic (que sí llega a la app) deja seleccionado lo que tocaste y la barra
// muestra su cadena de zonas (zonas.mjs): «Consola › Vista Alumnos › Ficha del alumno › …». Cada
// nivel es un botón para elegir de qué es la nota (el botón o toda la sección). N abre la nota de
// la selección. A es el modo de señalar sin que el clic llegue a la app; Z muestra el nombre de la
// zona al pasar el mouse. La barra se arrastra por su asa (M la pasa arriba o abajo). Guardar, listar, borrar y cambiar de escena los hace Node por funciones
// expuestas (__anotador*).
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
    document.documentElement.appendChild(host);
    const raiz = host.attachShadow({ mode: "open" });
    raiz.innerHTML = `
<style>
  :host { all: initial; }
  * { box-sizing: border-box; font: 13px/1.35 "Segoe UI", system-ui, sans-serif; }
  .barra { position: fixed; left: 12px; bottom: 12px; z-index: 2147483646; display: grid; gap: 6px;
    max-width: calc(100vw - 24px); padding: 8px 10px; border-radius: 10px; background: #1f2433;
    color: #f2f4f8; box-shadow: 0 6px 24px rgba(0,0,0,.35); }
  .fila { display: flex; flex-wrap: wrap; gap: 6px; align-items: center; }
  .barra strong { margin-right: 4px; }
  .asa { cursor: grab; touch-action: none; user-select: none; display: inline-flex; align-items: center; gap: 6px;
    padding: 2px 6px 2px 2px; border-radius: 6px; }
  .asa:hover { background: #2b3245; }
  .asa .puntos { color: #9aa3b8; font-size: 16px; line-height: 1; letter-spacing: -2px; }
  .arrastrando, .arrastrando * { cursor: grabbing !important; }
  .ventana h2.asa { display: block; padding: 2px 4px; margin: -2px -4px 6px; }
  select, button, textarea { border-radius: 6px; border: 1px solid #4a5268; background: #2b3245; color: #f2f4f8; padding: 4px 8px; }
  button { cursor: pointer; }
  button:disabled { opacity: .45; cursor: default; }
  button.activo { background: #f59e0b; color: #1f2433; border-color: #f59e0b; font-weight: 600; }
  button.primario { background: #3b82f6; border-color: #3b82f6; }
  .niveles { display: flex; flex-wrap: wrap; gap: 2px; align-items: center; }
  .niveles .sep { color: #9aa3b8; padding: 0 2px; }
  .niveles button { padding: 2px 8px; border-color: transparent; background: transparent; }
  .niveles button:hover { border-color: #4a5268; }
  .niveles button[aria-pressed="true"] { background: #3b82f6; border-color: #3b82f6; font-weight: 600; }
  .vacio { color: #9aa3b8; }
  .caja { position: fixed; z-index: 2147483645; pointer-events: none; border: 2px solid #f59e0b; border-radius: 4px;
    background: rgba(245,158,11,.12); display: none; }
  .sel { position: fixed; z-index: 2147483644; pointer-events: none; border: 2px dashed #3b82f6; border-radius: 4px; display: none; }
  .etiqueta { position: fixed; z-index: 2147483646; pointer-events: none; display: none; max-width: 60vw; padding: 4px 8px;
    border-radius: 6px; background: #f59e0b; color: #1f2433; font-weight: 600; }
  .etiqueta small { display: block; font-weight: 400; }
  .ventana { position: fixed; z-index: 2147483647; right: 16px; top: 16px; width: min(460px, calc(100vw - 32px));
    max-height: calc(100vh - 32px); overflow: auto; padding: 14px; border-radius: 10px; background: #1f2433;
    color: #f2f4f8; box-shadow: 0 10px 40px rgba(0,0,0,.45); display: none; }
  .ventana h2 { margin: 0 0 6px; font-size: 15px; font-weight: 600; }
  .sobre { margin: 0 0 10px; color: #c7cede; }
  .sobre .niveles { margin-top: 4px; }
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
<div class="sel"></div>
<div class="caja"></div>
<div class="etiqueta"></div>
<div class="ok"></div>
<div class="barra" role="toolbar" aria-label="Anotador de interfaz">
  <div class="fila">
    <span class="asa" id="asa" title="Arrastra para mover la barra a cualquier parte · doble clic la regresa abajo a la izquierda (tecla M la pasa arriba o abajo)"><span class="puntos" aria-hidden="true">⠿</span><strong>Anotador</strong></span>
    <select id="app" title="App"></select>
    <select id="escena" title="Escena (datos de demostración)"></select>
    <select id="ruta" title="Vista de la consola"></select>
    <select id="tema" title="Tema: con «Ambos» se ve en claro y cada nota se captura en claro y en oscuro"><option value="ambos">Ambos</option><option value="claro">Claro</option><option value="oscuro">Oscuro</option></select>
    <button id="anotar-sel" class="primario" title="Escribe la nota de lo seleccionado (tecla N)" disabled>Anotar selección (N)</button>
    <button id="anotar" title="Señala sin que el clic llegue a la app (tecla A)">Señalar (A)</button>
    <button id="explorar" title="Muestra el nombre de la zona al pasar el mouse (tecla Z)">Zonas (Z)</button>
    <button id="general" title="Una nota sobre toda la pantalla">Nota de la pantalla</button>
    <button id="ver">Notas</button>
    <button id="ocultar" title="Esconde la barra (tecla H)">Ocultar (H)</button>
  </div>
  <div class="fila"><span>Selección:</span><span class="niveles" id="niveles-barra"><span class="vacio">haz clic en la interfaz para seleccionar</span></span></div>
</div>
<form class="ventana" id="form">
  <h2 class="asa" id="asa-form" title="Arrastra para mover la ventana de la nota">Nueva nota</h2>
  <div class="sobre">Sobre: <span class="niveles" id="niveles-form"></span></div>
  <div class="tipos" id="tipos"></div>
  <textarea id="texto" placeholder="¿Qué quieres? Por ejemplo: «Esto debería ser un módulo propio en el menú» o «una grilla de iconos con tooltip»."></textarea>
  <p class="ayuda">Elige arriba si la nota es del elemento o de toda una sección. Ctrl+Enter guarda · Esc cancela. Se guarda con una captura de la sección y de la pantalla.</p>
  <div class="acciones"><button type="button" id="cancelar">Cancelar</button><button type="submit" class="primario">Guardar nota</button></div>
</form>
<div class="ventana" id="lista"><h2 class="asa" title="Arrastra para mover la lista">Notas guardadas</h2><ol id="notas"></ol>
  <div class="acciones"><button type="button" id="cerrar-lista">Cerrar</button></div></div>`;
    const $ = (s) => raiz.querySelector(s);
    const caja = $(".caja"), selCaja = $(".sel"), etiqueta = $(".etiqueta"), barra = $(".barra"), form = $("#form"), lista = $("#lista");

    // --- Mover la barra y las ventanas ---------------------------------------------------------
    // La barra se arrastra por su asa a cualquier parte, para ver o señalar lo que tapa; su lugar
    // se recuerda al cambiar de app, escena, vista o tema (la página se recarga). Doble clic en el
    // asa la regresa abajo a la izquierda; M la pasa arriba o abajo. La nota y la lista se mueven
    // por su título mientras están abiertas.
    const GUARDA = "anotador-barra";
    function colocar(el, x, y) {
      x = Math.max(0, Math.min(x, innerWidth - el.offsetWidth));
      y = Math.max(0, Math.min(y, innerHeight - el.offsetHeight));
      Object.assign(el.style, { left: x + "px", top: y + "px", right: "auto", bottom: "auto" });
    }
    // Con el puntero capturado el navegador no da «dblclick»: dos toques sin arrastrar llaman alDobleToque.
    function arrastrable(el, asa, alSoltar, alDobleToque) {
      let ultimoToque = 0;
      asa.addEventListener("pointerdown", (e) => {
        if (e.button !== 0) return;
        e.preventDefault();
        const r = el.getBoundingClientRect(), dx = e.clientX - r.left, dy = e.clientY - r.top, x0 = e.clientX, y0 = e.clientY;
        let arrastro = false;
        asa.setPointerCapture(e.pointerId);
        el.classList.add("arrastrando");
        const mover = (ev) => {
          if (!arrastro && Math.abs(ev.clientX - x0) + Math.abs(ev.clientY - y0) < 4) return;
          arrastro = true;
          colocar(el, ev.clientX - dx, ev.clientY - dy);
        };
        const soltar = () => {
          asa.removeEventListener("pointermove", mover);
          asa.removeEventListener("pointerup", soltar);
          asa.removeEventListener("pointercancel", soltar);
          el.classList.remove("arrastrando");
          if (arrastro) { ultimoToque = 0; if (alSoltar) alSoltar(); return; }
          const ahora = Date.now();
          if (alDobleToque && ahora - ultimoToque < 400) { ultimoToque = 0; alDobleToque(); } else ultimoToque = ahora;
        };
        asa.addEventListener("pointermove", mover);
        asa.addEventListener("pointerup", soltar);
        asa.addEventListener("pointercancel", soltar);
      });
    }
    const movida = () => barra.style.top !== "";
    function guardarBarra() {
      try {
        if (movida()) localStorage.setItem(GUARDA, JSON.stringify({ x: barra.offsetLeft, y: barra.offsetTop }));
        else localStorage.removeItem(GUARDA);
      } catch { /* sin almacenamiento: solo no se recuerda */ }
    }
    // Si la barra cambia de tamaño (la cadena de la selección) o la ventana se achica, sigue entera a la vista.
    function ajustarBarra() { if (movida() && !barra.hidden) colocar(barra, barra.offsetLeft, barra.offsetTop); }
    function barraAbajo() { Object.assign(barra.style, { left: "", top: "", right: "", bottom: "" }); guardarBarra(); }
    function barraArribaAbajo() {
      const arriba = barra.getBoundingClientRect().top < innerHeight / 2;
      colocar(barra, barra.offsetLeft, arriba ? innerHeight - barra.offsetHeight - 12 : 12);
      guardarBarra();
    }
    arrastrable(barra, $("#asa"), guardarBarra, barraAbajo);
    arrastrable(form, $("#asa-form"));
    arrastrable(lista, lista.querySelector("h2"));
    try {
      const pos = JSON.parse(localStorage.getItem(GUARDA) || "null");
      if (pos) colocar(barra, pos.x, pos.y);
    } catch { /* posición por omisión */ }
    new ResizeObserver(ajustarBarra).observe(barra);
    addEventListener("resize", ajustarBarra);

    // Un <dialog> modal (Pantalla grande, Ajustes, confirmaciones) se pone en una capa por encima de
    // todo y deja inerte el resto de la página: el anotador se mete en el diálogo abierto mientras
    // dure, para verse y recibir clics, y vuelve a su lugar al cerrarse.
    function reubicar() {
      const modales = [...document.querySelectorAll("dialog[open]")].filter((d) => d.matches(":modal"));
      const destino = modales.length ? modales[modales.length - 1] : document.documentElement;
      if (host.parentNode !== destino) destino.appendChild(host);
    }
    new MutationObserver(reubicar).observe(document.documentElement, { subtree: true, attributes: true, attributeFilter: ["open"] });
    document.addEventListener("close", () => setTimeout(reubicar), true);

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

    // --- Zonas y niveles ------------------------------------------------------------------------
    const zonas = C.zonas[C.app];
    const profundidad = (el) => { let n = 0; for (let e = el; e; e = e.parentElement) n++; return n; };
    const recortar = (s) => (s || "").replace(/\s+/g, " ").trim().slice(0, 80);
    // Cómo se llama el elemento: su texto visible, su etiqueta o su clave de textos.
    function describir(el) {
      const clave = el.closest("[data-t],[data-t-aria-label]");
      return {
        etiqueta: el.tagName.toLowerCase() + (el.id ? "#" + el.id : ""),
        texto: recortar(el.getAttribute("aria-label") || el.title || el.dataset.tip || el.textContent),
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
        const clase = [...e.classList].slice(0, 2).map((c) => "." + CSS.escape(c)).join("");
        partes.unshift(e.tagName.toLowerCase() + clase + n);
      }
      return partes.join(" > ");
    }
    // Los niveles de un elemento, de afuera hacia adentro: cada zona que lo contiene (con su
    // elemento) y, al final, el elemento mismo si no es ya una zona.
    function nivelesDe(el) {
      const halladas = [];
      for (const z of zonas) {
        let m = null;
        try { m = el.closest(z.selector); } catch { continue; }
        if (m && !halladas.some((h) => h.el === m)) halladas.push({ zona: z, el: m, nombre: z.nombre, prof: profundidad(m) });
      }
      halladas.sort((a, b) => a.prof - b.prof);
      if (!halladas.length || halladas[halladas.length - 1].el !== el) {
        const d = describir(el);
        halladas.push({ zona: null, el, nombre: d.texto ? "«" + d.texto.slice(0, 40) + (d.texto.length > 40 ? "…" : "") + "»" : d.etiqueta });
      }
      return halladas;
    }
    // La zona y la cadena de un nivel: la zona más interna hasta ese nivel.
    function zonaDeNivel(niveles, i) {
      const zs = niveles.slice(0, i + 1).filter((n) => n.zona);
      const z = zs.length ? zs[zs.length - 1].zona : null;
      return { id: z ? z.id : "", nombre: z ? z.nombre : "(zona sin nombre)",
        cadena: [NOMBRE_APP[C.app]].concat(zs.map((n) => n.zona.nombre)).join(" › ") + (zs.length ? "" : " › (zona sin nombre)") };
    }

    // --- Selección: cada clic normal la actualiza ---------------------------------------------
    let seleccion = null; // { niveles, i }
    const propio = (e) => e.composedPath().includes(host);
    // Se toma el control completo (un botón, no el texto de adentro).
    const elegir = (el) => el.closest("button, a, input, select, textarea, summary, label, li[data-tip], [role=button], [role=tab], [role=menuitem]") || el;
    function seleccionar(el) {
      const niveles = nivelesDe(el);
      seleccion = { niveles, i: niveles.length - 1 };
      pintarSeleccion();
    }
    function nivelActual() { return seleccion ? seleccion.niveles[seleccion.i] : null; }
    function pintarNiveles(contenedor) {
      contenedor.replaceChildren();
      if (!seleccion) {
        const v = document.createElement("span");
        v.className = "vacio";
        v.textContent = "haz clic en la interfaz para seleccionar";
        contenedor.append(v);
        return;
      }
      const app = document.createElement("span");
      app.textContent = NOMBRE_APP[C.app];
      contenedor.append(app);
      seleccion.niveles.forEach((n, i) => {
        const sep = document.createElement("span");
        sep.className = "sep";
        sep.textContent = "›";
        const b = document.createElement("button");
        b.type = "button";
        b.textContent = n.nombre;
        b.title = n.zona ? n.zona.que : "Solo este elemento";
        b.setAttribute("aria-pressed", String(i === seleccion.i));
        b.onclick = () => { seleccion.i = i; pintarSeleccion(); };
        contenedor.append(sep, b);
      });
    }
    function pintarSeleccion() {
      pintarNiveles($("#niveles-barra"));
      if (form.style.display === "block") pintarNiveles($("#niveles-form"));
      $("#anotar-sel").disabled = !seleccion;
      dibujarSeleccion();
    }
    // El recuadro punteado sigue a lo seleccionado (si sigue en la página) al desplazar o redimensionar.
    function dibujarSeleccion() {
      const n = nivelActual();
      if (!n || !n.el.isConnected || barra.hidden) { selCaja.style.display = "none"; return; }
      const r = n.el.getBoundingClientRect();
      Object.assign(selCaja.style, { display: "block", left: r.left - 3 + "px", top: r.top - 3 + "px", width: r.width + 6 + "px", height: r.height + 6 + "px" });
    }
    addEventListener("scroll", () => requestAnimationFrame(dibujarSeleccion), true);
    addEventListener("resize", () => requestAnimationFrame(dibujarSeleccion));
    // Los clics de la app pueden abrir paneles y mover cosas: se vuelve a dibujar después.
    document.addEventListener("click", (e) => {
      if (anotando || propio(e)) return;
      seleccionar(elegir(e.target));
      setTimeout(dibujarSeleccion, 250);
    }, true);

    // --- Modo señalar (A) y zonas al pasar (Z) --------------------------------------------------
    let anotando = false, explorando = false;
    function marcar(el) {
      const r = el.getBoundingClientRect();
      Object.assign(caja.style, { display: "block", left: r.left - 2 + "px", top: r.top - 2 + "px", width: r.width + 4 + "px", height: r.height + 4 + "px" });
      const niveles = nivelesDe(el);
      const z = zonaDeNivel(niveles, niveles.length - 1);
      const d = describir(el);
      etiqueta.replaceChildren(z.cadena);
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
      if (!on && !explorando) soltarMarca();
    }
    function explorar(on) {
      explorando = on;
      $("#explorar").classList.toggle("activo", on);
      if (!on && !anotando) soltarMarca();
    }
    document.addEventListener("mousemove", (e) => { if ((anotando || explorando) && !propio(e)) marcar(elegir(e.target)); }, true);
    // Al señalar (A), ningún clic llega a la app: selecciona y abre la nota.
    for (const tipo of ["click", "mousedown", "mouseup", "pointerdown", "pointerup", "dblclick", "contextmenu"]) {
      document.addEventListener(tipo, (e) => {
        if (!anotando || propio(e)) return;
        e.preventDefault(); e.stopPropagation(); e.stopImmediatePropagation();
        if (tipo === "click") { const el = elegir(e.target); modo(false); seleccionar(el); abrirForm(); }
      }, true);
    }
    $("#anotar").onclick = () => modo(!anotando);
    $("#explorar").onclick = () => explorar(!explorando);
    $("#anotar-sel").onclick = () => { if (seleccion) abrirForm(); };
    $("#general").onclick = () => { modo(false); abrirForm(true); };
    $("#ocultar").onclick = () => { barra.hidden = !barra.hidden; dibujarSeleccion(); };
    // Al volver a mostrarla, sigue a la vista aunque la ventana haya cambiado mientras estaba oculta.
    new MutationObserver(ajustarBarra).observe(barra, { attributes: true, attributeFilter: ["hidden"] });
    document.addEventListener("keydown", (e) => {
      if (propio(e) && e.key !== "Escape") return;
      const escribiendo = /input|textarea|select/i.test((e.composedPath()[0] || {}).tagName || "") || document.activeElement?.isContentEditable;
      // Esc cierra el formulario o el modo señalar, no el diálogo de la app que esté abierto.
      if (e.key === "Escape" && (anotando || form.style.display === "block")) { modo(false); cerrarForm(); e.preventDefault(); e.stopPropagation(); return; }
      if (escribiendo || e.ctrlKey || e.altKey || e.metaKey) return;
      const k = e.key.toLowerCase();
      if (k === "a") { modo(!anotando); e.preventDefault(); }
      if (k === "z") { explorar(!explorando); e.preventDefault(); }
      if (k === "n" && seleccion) { abrirForm(); e.preventDefault(); }
      if (k === "h") { barra.hidden = !barra.hidden; dibujarSeleccion(); e.preventDefault(); }
      if (k === "m" && !barra.hidden) { barraArribaAbajo(); e.preventDefault(); }
    }, true);

    // --- Formulario -------------------------------------------------------------------------
    $("#tipos").append(...TIPOS.map(([v, t], i) => {
      const l = document.createElement("label");
      const r = Object.assign(document.createElement("input"), { type: "radio", name: "tipo", value: v, checked: i === 0 });
      const s = document.createElement("span");
      s.textContent = t;
      l.append(r, s);
      return l;
    }));
    let pantallaCompleta = false;
    // general: una nota de toda la pantalla, sin selección.
    function abrirForm(general) {
      pantallaCompleta = !!general;
      if (pantallaCompleta) {
        $("#niveles-form").replaceChildren(NOMBRE_APP[C.app] + " › Pantalla completa");
      } else {
        pintarNiveles($("#niveles-form"));
      }
      // Del lado contrario a lo seleccionado, para no taparlo.
      const n = pantallaCompleta ? null : nivelActual();
      const r = n && n.el.isConnected ? n.el.getBoundingClientRect() : null;
      const derecha = !r || r.left + r.width / 2 < innerWidth / 2;
      form.style.left = derecha ? "auto" : "16px";
      form.style.right = derecha ? "16px" : "auto";
      form.style.top = "16px"; form.style.bottom = "auto"; // aunque la hayan movido la vez anterior
      form.style.display = "block";
      lista.style.display = "none";
      $("#texto").focus({ preventScroll: true });
    }
    function cerrarForm() { form.style.display = "none"; $("#texto").value = ""; soltarMarca(); dibujarSeleccion(); }
    $("#cancelar").onclick = cerrarForm;
    $("#texto").addEventListener("keydown", (e) => { if (e.key === "Enter" && e.ctrlKey) { e.preventDefault(); form.requestSubmit(); } });
    form.addEventListener("submit", async (e) => {
      e.preventDefault();
      const texto = $("#texto").value.trim();
      if (!texto) { $("#texto").focus(); return; }
      const tipo = raiz.querySelector("input[name=tipo]:checked").value;
      let zona = { id: "", nombre: "Pantalla completa", cadena: NOMBRE_APP[C.app] + " › Pantalla completa" };
      let elemento = null, rect = null;
      const n = pantallaCompleta ? null : nivelActual();
      if (n) {
        zona = zonaDeNivel(seleccion.niveles, seleccion.i);
        elemento = describir(n.el);
        if (n.el.isConnected) {
          // El recorte: lo elegido y, si es un elemento suelto, la zona que lo contiene (si cabe en la
          // ventana), con el elemento resaltado adentro: así se entiende dónde está.
          let r = n.el.getBoundingClientRect();
          if (!n.zona) {
            const cont = seleccion.niveles.slice(0, seleccion.i).reverse().find((x) => x.zona && x.el.isConnected);
            if (cont) {
              const rz = cont.el.getBoundingClientRect();
              if (rz.width * rz.height < innerWidth * innerHeight * 0.6) r = rz;
            }
          }
          rect = { x: r.left, y: r.top, w: r.width, h: r.height, sx: scrollX, sy: scrollY };
        }
      }
      // La captura no lleva la barra ni el formulario; sí el recuadro de lo elegido.
      form.style.display = "none"; barra.style.visibility = "hidden"; etiqueta.style.display = "none"; selCaja.style.display = "none";
      if (n && n.el.isConnected) { marcar(n.el); etiqueta.style.display = "none"; }
      await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
      let res;
      try {
        res = await window.__anotadorGuardar({ tipo, texto, zona, elemento, rect,
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
      Object.assign(lista.style, { left: "auto", right: "16px", top: "16px", bottom: "auto" });
      lista.style.display = "block";
    };
    $("#cerrar-lista").onclick = () => { lista.style.display = "none"; };
  }

  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", iniciar);
  else iniciar();
})();
