// Interfaz del cliente del alumno. Recibe el estado del backend Go por eventos
// de Wails y llama a sus métodos (View, Copy, DismissNotice).
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

  // Subscribe first, then fetch the full state, so nothing is missed.
  rt.EventsOn(ev.STATUS, renderStatus);
  rt.EventsOn(ev.BOARD, (b) => renderBoard(b.snippets));
  rt.EventsOn(ev.NOTICES, (n) => renderNotices(n.notices));
  api.View().then((v) => {
    renderStatus(v.status);
    renderBoard(v.snippets);
    renderNotices(v.notices);
  });
})();
