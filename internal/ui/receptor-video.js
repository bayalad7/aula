// Receptor de video de Aula: la conexión RTCPeerConnection que pinta una transmisión (la
// pantalla del profesor o la de un alumno, docs/protocol.md §5.12 y §5.24). Una sola copia para
// las dos apps (internal/ui, servido bajo sistema/): la vista de la transmisión del alumno y la
// Pantalla grande de la consola. Solo recibe: nunca envía medios ni datos propios.
//
// Uso:
//   const r = AulaReceptor.crear({ video, espera, answer, candidate, failed });
//   r.senal(s);   // cada broadcast.Signal (offer o ice) que manda Go
//   r.cerrar();   // al terminar la transmisión
// answer(id, intento, sdp), candidate(id, intento, candidato, mid, linea) y failed(id, intento)
// son los bindings de la app y devuelven promesas.
"use strict";
window.AulaReceptor = Object.freeze({
  crear(o) {
    let pc = null;
    let clave = "";    // broadcastId#attempt de la conexión vigente
    let pendientes = []; // candidatos que llegaron antes que la oferta

    function cerrar() {
      if (pc) pc.close();
      pc = null;
      clave = "";
      pendientes = [];
      o.video.srcObject = null;
      if (o.espera) o.espera.hidden = false;
    }

    async function responder(s) {
      cerrar();
      const k = s.broadcastId + "#" + s.attempt;
      const conn = new RTCPeerConnection({ iceServers: [] });
      pc = conn;
      clave = k;
      conn.ontrack = (e) => {
        o.video.srcObject = e.streams[0] || new MediaStream([e.track]);
      };
      conn.onicecandidate = (e) => {
        if (clave !== k) return;
        const c = e.candidate;
        o.candidate(s.broadcastId, s.attempt, c ? c.candidate : "", (c && c.sdpMid) || "0",
          (c && c.sdpMLineIndex) || 0).catch(() => {});
      };
      conn.onconnectionstatechange = () => {
        if (clave === k && o.espera) o.espera.hidden = conn.connectionState === "connected";
      };
      try {
        await conn.setRemoteDescription({ type: "offer", sdp: s.sdp });
        for (const c of pendientes) await conn.addIceCandidate(c).catch(() => {});
        pendientes = [];
        await conn.setLocalDescription(await conn.createAnswer());
        if (clave !== k) return;
        await o.answer(s.broadcastId, s.attempt, conn.localDescription.sdp);
      } catch (err) {
        if (clave === k) o.failed(s.broadcastId, s.attempt).catch(() => {});
      }
    }

    function senal(s) {
      if (s.kind === "offer") {
        responder(s);
        return;
      }
      if (s.broadcastId + "#" + s.attempt !== clave || !s.candidate) return;
      const c = { candidate: s.candidate, sdpMid: s.sdpMid || "0", sdpMLineIndex: s.sdpMLineIndex };
      if (pc.remoteDescription) pc.addIceCandidate(c).catch(() => {});
      else pendientes.push(c);
    }

    return { senal, cerrar };
  },
});
