package simclass

import (
	"context"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/transport"
)

// The exercise timer (docs/protocol.md §5.19): a simulated student counts it
// like the real window and acknowledges it, unless its fault says otherwise
// ("noack" never acknowledges, "slow" answers late). It changes no computer,
// so simulated students get it (docs/privacidad.md rule 34 does not apply).

// handleTimer applies timer.*; it reports whether env was one.
func (s *student) handleTimer(ctx context.Context, cl *transport.Client, env protocol.Envelope, fault Fault) bool {
	var acks []protocol.TimerAck
	switch p := env.Payload.(type) {
	case protocol.TimerStart:
		acks = s.countdown.Start(env.ID, p)
	case protocol.TimerExtend:
		if ack, ok := s.countdown.Extend(env.ID, p); ok {
			acks = append(acks, ack)
		}
	case protocol.TimerStop:
		if ack, ok := s.countdown.Stop(env.ID, p); ok {
			acks = append(acks, ack)
		}
	default:
		return false
	}
	if fault != FaultNoAck {
		for _, ack := range acks {
			s.sendAck(ctx, cl, ack)
		}
	}
	s.armTimer()
	s.class.changed()
	return true
}

// armTimer schedules the next tick of the countdown.
func (s *student) armTimer() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.timerT != nil {
		s.timerT.Stop()
		s.timerT = nil
	}
	if d, ok := s.countdown.Next(); ok {
		s.timerT = time.AfterFunc(d, s.timerTick)
	}
}

func (s *student) timerTick() {
	ack, ended, changed := s.countdown.Tick()
	if ended {
		s.mu.Lock()
		fault := s.fault
		s.mu.Unlock()
		if cl := s.connection(); cl != nil && fault != FaultNoAck {
			ctx, cancel := context.WithTimeout(s.class.ctx, 5*time.Second)
			s.sendAck(ctx, cl, ack)
			cancel()
		}
	}
	s.armTimer()
	if changed {
		s.class.changed()
	}
}

func (s *student) sendAck(ctx context.Context, cl *transport.Client, ack protocol.TimerAck) {
	if _, err := cl.Send(ctx, ack); err != nil {
		s.class.log.Debug("cannot send timer.ack", "student", s.name, "err", err)
	}
}

func (s *student) stopTimer() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.timerT != nil {
		s.timerT.Stop()
		s.timerT = nil
	}
}
