// Contrato de eventos backend → interfaz. Debe coincidir con apps/teacher/events.go
// (una prueba lo verifica). Documentado en docs/eventos-ui.md.
"use strict";
window.AulaEvents = Object.freeze({
  STUDENTS: "aula:students",
  HISTORY: "aula:history",
  DELIVERIES: "aula:deliveries",
  CAPTURE: "aula:capture",
  CLASS: "aula:class",
});
