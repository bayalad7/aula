// Contrato de eventos backend → interfaz. Debe coincidir con apps/student/events.go
// (una prueba lo verifica). Documentado en docs/eventos-ui.md.
"use strict";
window.AulaEvents = Object.freeze({
  STATUS: "aula:status",
  BOARD: "aula:board",
  NOTICES: "aula:notices",
  BROADCAST: "aula:broadcast",
  SIGNAL: "aula:broadcast-signal",
});
