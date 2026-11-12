// Iconos de referencia de Aula en botones y campos (las dos apps; internal/ui, servido bajo
// sistema/). Un botón o un campo declara su icono con data-icono="nombre" (un <symbol> de
// sistema/iconos.svg) y este script lo dibuja:
//   - botón: el icono va antes del texto. Si la página reescribe el texto del botón
//     (textContent), el icono vuelve solo: un MutationObserver lo repone antes del repintado.
//   - input, select o textarea: se envuelve en <span class="entrada"> con el icono a la izquierda.
// Solo crea elementos (createElementNS); nunca usa innerHTML.
"use strict";
window.AulaIconos = (function () {
  const NS = "http://www.w3.org/2000/svg";
  const RUTA = "sistema/iconos.svg#";

  function svg(nombre, cls) {
    const s = document.createElementNS(NS, "svg");
    s.setAttribute("class", "icono " + cls);
    s.setAttribute("aria-hidden", "true");
    const u = document.createElementNS(NS, "use");
    u.setAttribute("href", RUTA + nombre);
    s.append(u);
    return s;
  }

  function boton(b) {
    const nombre = b.dataset.icono;
    let s = b.firstElementChild;
    if (s && s.classList && s.classList.contains("icono-auto")) {
      if (s.firstElementChild.getAttribute("href") !== RUTA + nombre) s.firstElementChild.setAttribute("href", RUTA + nombre);
      return;
    }
    b.prepend(svg(nombre, "icono-auto"));
  }

  function campo(c) {
    const p = c.parentElement;
    if (p && p.classList.contains("entrada")) {
      p.querySelector(".entrada__icono use").setAttribute("href", RUTA + c.dataset.icono);
      return;
    }
    const caja = document.createElement("span");
    caja.className = "entrada" + (c.tagName === "TEXTAREA" ? " entrada--area" : "");
    c.before(caja);
    caja.append(svg(c.dataset.icono, "entrada__icono"), c);
  }

  function aplicar(e) {
    if (!e.dataset || !e.dataset.icono) return;
    if (e.tagName === "INPUT" || e.tagName === "SELECT" || e.tagName === "TEXTAREA") campo(e);
    else boton(e);
  }

  function recorrer(root) {
    if (root.nodeType !== 1) return;
    aplicar(root);
    for (const e of root.querySelectorAll("[data-icono]")) aplicar(e);
  }

  // Dibuja los iconos de root y los mantiene mientras la página cambia.
  function vigilar(root) {
    recorrer(root);
    new MutationObserver((cambios) => {
      for (const c of cambios) {
        if (c.type === "attributes") { aplicar(c.target); continue; }
        // El texto de un botón se reescribió: su icono vuelve.
        if (c.target.dataset && c.target.dataset.icono) aplicar(c.target);
        for (const n of c.addedNodes) recorrer(n);
      }
    }).observe(root, { childList: true, subtree: true, attributes: true, attributeFilter: ["data-icono"] });
  }

  return Object.freeze({ vigilar });
})();
