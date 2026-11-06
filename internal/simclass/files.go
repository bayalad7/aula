package simclass

import (
	"context"
	"strings"
	"sync"

	"github.com/bayalad7/aula/internal/filepush"
	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

// Files the teacher sends (docs/protocol.md §5.20): a simulated student
// reassembles and checks them like the real agent and acknowledges them,
// unless its fault says otherwise ("noack" never acknowledges, "slow" answers
// late). It never writes to disk: its folder keeps only the names, so a
// second send of the same name is "renamed" as on a real computer.

// handleFile applies file.push; it reports whether env was one.
func (s *student) handleFile(ctx context.Context, send filepush.Sender, env protocol.Envelope) bool {
	acked := func(ctx context.Context, p protocol.Payload) error {
		s.mu.Lock()
		noack := s.fault == FaultNoAck
		s.mu.Unlock()
		if _, isAck := p.(protocol.FilePushAck); isAck && noack {
			return nil
		}
		return send(ctx, p)
	}
	reply, handled := s.files.Handle(ctx, acked, env)
	if !handled {
		return false
	}
	if reply != nil {
		if err := acked(ctx, reply); err != nil {
			s.class.log.Debug("cannot reply", "student", s.name, "type", reply.MessageType(), "err", err)
		}
	}
	return true
}

func (s *student) receivedFile(it filepush.Item) {
	if !it.Saved() {
		return
	}
	s.mu.Lock()
	s.received++
	s.mu.Unlock()
	s.class.changed()
}

// memoryFolder is the work folder of a simulated student: names only, no
// contents and no disk.
type memoryFolder struct {
	mu    sync.Mutex
	names map[string]bool
}

func (m *memoryFolder) Create(rel string, _ []byte, overwrite bool) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.names == nil {
		m.names = map[string]bool{}
	}
	key := strings.ToLower(rel)
	exists := m.names[key]
	if exists && !overwrite {
		return false, platform.ErrFileExists
	}
	m.names[key] = true
	return exists, nil
}
