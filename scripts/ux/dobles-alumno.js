// Doble de los bindings de Wails de la app del alumno (window.go.main.App y
// window.runtime), para recorrer apps/student/frontend/dist sin backend. Los
// datos tienen la forma exacta de apps/student/events.go: si cambia un binding
// o un evento, cambia aquí también (una prueba de Go lo exige). La escena se
// elige con window.__SCENE. Ver scripts/ux/README.md.
(function () {
  const scene = window.__SCENE || "conectado";
  const handlers = {};
  window.runtime = {
    EventsOn(name, cb) { (handlers[name] = handlers[name] || []).push(cb); return () => {}; },
    EventsOff() {},
    WindowSetTitle(t) { window.__title = t; },
  };
  window.__emit = (name, data) => (handlers[name] || []).forEach((cb) => cb(data));
  window.__calls = [];

  const teacher = "Prof. Ayala";
  const channel = "1";
  // El estado: Go manda estados, no frases (apps/student/events.go, Status).
  const st = (state, extra) => Object.assign({ state, teacher: state === "searching" || state === "unconfigured" ? "" : teacher,
    channel: state === "unconfigured" ? "" : channel, detail: "", cause: "", viewed: false, watching: false, restricted: false }, extra);
  const conectado = st("connected");
  const status = {
    "sin-configurar": st("unconfigured", { detail: "no hay configuración: falta config.json en C:\ProgramData\Aula\alumno (canal y clave pública del centro); pide al administrador que instale Aula en este equipo" }),
    buscando: st("searching"),
    "sin-conexion": st("searching", { cause: "noConsole", detail: `No se encontró ninguna consola del canal ${channel}. Revisa que la consola del profesor esté abierta en ese canal, que la red de este equipo sea Privada o de Dominio (no Pública) y que el firewall permita Aula.` }),
    reconectando: st("disconnected"),
    "otra-clave": st("searching", { cause: "wrongKey", detail: `Se encontró la consola «Sala 2» del canal ${channel}, pero su clave no es la de este centro: no se le obedece. Si es la consola de tu profesor, este equipo tiene otra clave del centro; avisa al administrador.` }),
    "pantalla-vista": st("connected", { viewed: true }),
    transmision: st("connected", { watching: true }),
    restricciones: st("connected", { restricted: true }),
  }[scene] || conectado;

  const ahora = new Date().toISOString();
  const py = { id: "sn-1", exercise: "1", title: "Factorial recursivo", lang: "python", receivedAt: ahora,
    content: "def factorial(n):\n    if n <= 1:\n        return 1\n    return n * factorial(n - 1)\n\n\nprint(factorial(5))\n" };
  const js = { id: "sn-2", exercise: "2", title: "Suma de una lista", lang: "javascript", receivedAt: ahora,
    content: "function suma(lista) {\n  let total = 0;\n  for (const n of lista) {\n    total += n;\n  }\n  return total;\n}\n\nconsole.log(suma([1, 2, 3]));\n" };
  const java = { id: "sn-3", exercise: "3", title: "", lang: "java", receivedAt: ahora,
    content: "public class Hola {\n    public static void main(String[] args) {\n        for (int i = 0; i < 3; i++) {\n            System.out.println(\"Hola \" + i);\n        }\n    }\n}\n" };
  const snippets = { "un-codigo": [py], "varios-codigos": [py, js, java], "pantalla-vista": [py], transmision: [],
    restricciones: [py], "aviso-info": [py], "aviso-warning": [py], "varios-avisos": [py, js, java], "muchos-avisos": [py] }[scene] || [];

  const aviso = (id, level, text) => ({ id, level, text, at: ahora });
  const notices = {
    "aviso-info": [aviso("m-1", "info", "En 10 minutos revisamos el ejercicio 1 todos juntos.")],
    "aviso-warning": [aviso("m-2", "warning", "Guarden su trabajo: en 5 minutos cerramos la sesión.")],
    "varios-avisos": [aviso("m-1", "info", "En 10 minutos revisamos el ejercicio 1 todos juntos."),
      aviso("m-2", "warning", "Guarden su trabajo: en 5 minutos cerramos la sesión."),
      aviso("m-3", "info", "Si terminaste, intenta la versión iterativa del factorial.")],
    "muchos-avisos": [1, 2, 3, 4, 5].map((i) => aviso("m-" + i, i === 5 ? "warning" : "info", `Aviso número ${i} de la clase.`)),
  }[scene] || [];

  const restrictions = scene === "restricciones" ? {
    apps: { policyId: "p-apps", mode: "deny", items: ["steam", "discord", "spotify", "whatsapp", "telegram"], total: 9 },
    web: { policyId: "p-web", mode: "deny", items: ["youtube.com", "tiktok.com", "instagram.com", "facebook.com", "x.com"], total: 11 },
  } : { apps: null, web: null };

  const broadcast = scene === "transmision"
    ? { active: true, broadcastId: "b-1", width: 1920, height: 1080, fps: 15, teacher, channel }
    : { active: false, broadcastId: "", width: 0, height: 0, fps: 0, teacher: "", channel: "" };

  const potencia = (state, action, extra) => ({ notice: Object.assign({ action_id: "pw-1", action, state,
    remaining_s: 87, postpones_left: 2, force: false }, extra), teacher, channel });
  const power = {
    "energia-cuenta": potencia("countdown", "reboot"),
    "energia-forzada": potencia("executing", "shutdown", { force: true, remaining_s: 0 }),
    "energia-fallida": potencia("failed", "logoff", { remaining_s: 0 }),
  }[scene] || { notice: null, teacher: "", channel: "" };

  const ok = (v) => new Promise((r) => setTimeout(() => r(v), 5));
  // Un doble que devuelve un Error rechaza la promesa con su texto, como Wails.
  const call = (name, v) => (...args) => {
    window.__calls.push([name, args]);
    const r = typeof v === "function" ? v(...args) : v;
    return r instanceof Error ? new Promise((_, no) => setTimeout(() => no(r.message), 5)) : ok(r);
  };
  window.go = { main: { App: {
    View: call("View", () => ({ status, snippets, notices, broadcast, restrictions, power })),
    Copy: call("Copy", () => (scene === "copia-falla" ? new Error("el portapapeles no está disponible en este equipo") : null)),
    DismissNotice: call("DismissNotice"),
    ReportMedia: call("ReportMedia"),
    BroadcastAnswer: call("BroadcastAnswer"),
    BroadcastCandidate: call("BroadcastCandidate"),
    BroadcastFailed: call("BroadcastFailed"),
    PowerPostpone: call("PowerPostpone"),
    DismissPower: call("DismissPower"),
    SetCompact: call("SetCompact"),
    ReportWindow: call("ReportWindow"),
  } } };
})();
