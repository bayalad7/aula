package simclass

import (
	"context"

	"github.com/bayalad7/aula/internal/health"
	"github.com/bayalad7/aula/internal/protocol"
)

// handleHealth answers the health messages with invented values
// (health.Simulated): the first report after health.config, one per
// «Revisar ahora» and the self-test. A simulated student reads no computer
// and changes nothing (docs/privacidad.md rule 95).
func (s *student) handleHealth(ctx context.Context, send func(context.Context, protocol.Payload) error, env protocol.Envelope) bool {
	var reply protocol.Payload
	s.mu.Lock()
	switch p := env.Payload.(type) {
	case protocol.HealthConfig:
		first := s.healthCfg == nil
		s.healthCfg = &p
		if first {
			s.healthSeq++
			reply = health.Simulated(s.index, s.healthSeq, protocol.HealthFirst, "")
		}
	case protocol.HealthRequest:
		s.healthSeq++
		reply = health.Simulated(s.index, s.healthSeq, protocol.HealthOnDemand, env.ID)
	case protocol.HealthSelfTest:
		reply = health.SimulatedSelfTest(s.index, s.healthCfg, env.ID, p.TestID)
	default:
		s.mu.Unlock()
		return false
	}
	s.mu.Unlock()
	if reply != nil {
		if err := send(ctx, reply); err != nil {
			s.class.log.Debug("cannot reply", "student", s.name, "type", reply.MessageType(), "err", err)
		}
	}
	return true
}
