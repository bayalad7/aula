// Contrato de eventos backend → interfaz. Debe coincidir con apps/teacher/events.go
// (una prueba lo verifica). Documentado en docs/eventos-ui.md.
"use strict";
window.AulaEvents = Object.freeze({
  STUDENTS: "aula:students",
  HISTORY: "aula:history",
  DELIVERIES: "aula:deliveries",
  CAPTURE: "aula:capture",
  CLASS: "aula:class",
  FRAME: "aula:frame",
  BROADCAST: "aula:broadcast",
  RESTRICTIONS: "aula:restrictions",
  SIMULATION: "aula:simulation",
  CONNECT_CHECK: "aula:connectcheck",
  REMOTE: "aula:remote",
  REMOTE_FRAME: "aula:remote-frame",
  SNAPSHOTS: "aula:snapshots",
  PARTICIPATION: "aula:participation",
  TIMER: "aula:timer",
  FILES: "aula:files",
});
