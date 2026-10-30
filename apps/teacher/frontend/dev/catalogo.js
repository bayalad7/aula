// Catálogo del sistema de diseño (solo desarrollo). Genera las muestras de
// color y la lista de iconos a partir de tokens.css e iconos.svg, para que el
// catálogo no se quede atrás cuando cambian.
"use strict";

(function () {
  const $ = (s) => document.querySelector(s);
  const ICONOS = "../dist/sistema/iconos.svg";

  // Colores: los --c-* declarados en :root, con su valor en el tema actual.
  function colores() {
    const nombres = new Set();
    for (const hoja of document.styleSheets) {
      let reglas;
      try { reglas = hoja.cssRules; } catch { continue; }
      for (const r of reglas) {
        if (r.selectorText === ":root" && r.style) {
          for (const p of r.style) if (p.startsWith("--c-")) nombres.add(p);
        }
      }
    }
    const cs = getComputedStyle(document.documentElement);
    $("#muestras").replaceChildren(...[...nombres].map((n) => {
      const li = document.createElement("li");
      const chip = document.createElement("span");
      chip.className = "cat-chip";
      chip.style.background = "var(" + n + ")";
      const nombre = document.createElement("code");
      nombre.textContent = n;
      const valor = document.createElement("span");
      valor.className = "suave pequeno";
      valor.textContent = cs.getPropertyValue(n).trim();
      li.append(chip, nombre, valor);
      return li;
    }));
  }

  // Iconos: cada <symbol> del sprite.
  fetch(ICONOS).then((r) => r.text()).then((txt) => {
    const doc = new DOMParser().parseFromString(txt, "image/svg+xml");
    $("#lista-iconos").replaceChildren(...[...doc.querySelectorAll("symbol")].map((s) => {
      const li = document.createElement("li");
      li.innerHTML = '<svg class="icono icono--lg" aria-hidden="true"><use href="' + ICONOS + "#" + s.id + '"/></svg>';
      const code = document.createElement("code");
      code.textContent = s.id;
      li.append(code);
      return li;
    }));
  });

  // Tabla de ejemplo, igual en el contenedor ancho y en el angosto.
  const filas = [
    ["LAB3-PC01", "Ana López", "conectado", "Conectado", "Ejercicio 3"],
    ["LAB3-PC03", "", "desconectado", "Desconectado", "—"],
    ["Simulado 1", "Alumno de prueba", "simulado", "Simulado", "Ejercicio 1"],
  ];
  const icono = { conectado: "conectado", desconectado: "desconectado", simulado: "simulado" };
  for (const t of document.querySelectorAll("#tabla table")) {
    t.innerHTML = "<thead><tr><th class=\"col-marca\"><span class=\"solo-lector\">Seleccionar</span></th><th>Equipo y alumno</th><th>Estado</th><th>Código en pantalla</th></tr></thead><tbody>" +
      filas.map((f, i) => "<tr" + (i === 0 ? ' data-seleccionado="true"' : "") + "><td class=\"col-marca\"><input type=\"checkbox\" aria-label=\"Seleccionar " + f[0] + "\"" + (i === 0 ? " checked" : "") + "></td>" +
        "<td class=\"celda-principal\" data-etiqueta=\"Equipo\">" + f[0] + "<span class=\"celda-sub\">" + (f[1] || "Sin sesión iniciada") + "</span></td>" +
        "<td data-etiqueta=\"Estado\"><span class=\"insignia insignia--" + f[2] + "\"><svg class=\"icono\" aria-hidden=\"true\"><use href=\"" + ICONOS + "#" + icono[f[2]] + "\"/></svg>" + f[3] + "</span></td>" +
        "<td data-etiqueta=\"Código en pantalla\">" + f[4] + "</td></tr>").join("") + "</tbody>";
  }

  // Tema: el botón pulsado y las muestras se actualizan.
  for (const b of document.querySelectorAll("[data-tema]")) {
    b.addEventListener("click", () => {
      for (const o of document.querySelectorAll("[data-tema]")) o.setAttribute("aria-pressed", String(o === b));
      requestAnimationFrame(colores);
    });
  }
  const t = new URLSearchParams(location.search).get("tema");
  if (t) {
    const b = document.querySelector('[data-tema="' + (t === "oscuro" ? "dark" : t === "claro" ? "light" : "sistema") + '"]');
    if (b) b.click();
  }
  colores();
})();
