package simclass

import (
	"context"

	"github.com/bayalad7/aula/internal/classend"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/transport"
)

// Ending the class (docs/protocol.md §5.25): a simulated student clears its
// simulated window like the real one (code, notices, timer, poll, hand and
// messages) and acknowledges, unless its fault says otherwise. Nothing that
// controls a computer ever reached it, so there is nothing else to remove.

// handleClassEnd applies class.end; it reports whether env was one.
func (s *student) handleClassEnd(ctx context.Context, cl *transport.Client, env protocol.Envelope, fault Fault) bool {
	p, ok := env.Payload.(protocol.ClassEnd)
	if !ok {
		return false
	}
	s.board.Clear()
	s.desk.Reset()
	s.countdown.Reset()
	s.stopTimer()
	s.ballot.Reset()
	s.mu.Lock()
	s.notices, s.private, s.lastNotice, s.received = 0, 0, "", 0
	s.mu.Unlock()
	s.collect.Forget()
	s.class.changed()
	if fault != FaultNoAck {
		s.send(ctx, cl, classend.Ack(env.ID, p))
	}
	return true
}
