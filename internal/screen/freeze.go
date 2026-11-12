package screen

import (
	"context"
	"log/slog"
	"sync"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

// Fixed texts of the pause window (docs/privacidad.md rule 9). They are
// native text, drawn by the OS window, like IndicatorText.
const (
	PauseTitle = "Tu profesor pausó esta computadora"
	PauseNote  = "Si se pierde la conexión con la clase, la pausa se quita sola."
)

// PauseTextFor is the pause window's text: who paused it, on which channel,
// and the teacher's message, if any.
func PauseTextFor(teacher, channel, message string) platform.PauseText {
	who := "Tu profesor"
	if teacher != "" {
		who = teacher
	}
	detail := who + " · canal " + channel + ". Tu trabajo sigue abierto y nada se perdió: tu equipo vuelve cuando el profesor lo reanude."
	if channel == "" {
		detail = who + ". Tu trabajo sigue abierto y nada se perdió: tu equipo vuelve cuando el profesor lo reanude."
	}
	return platform.PauseText{Title: PauseTitle, Message: message, Detail: detail, Note: PauseNote}
}

// Freezer is the agent side of screen.freeze (docs/protocol.md §5.9–5.11):
// it shows the pause window, answers with screen.freeze.ack, and removes the
// pause when the connection closes (Stop).
type Freezer struct {
	// Overlay is the pause window; nil when the OS has none, and then the
	// agent does not declare screen.freeze.
	Overlay platform.PauseOverlay
	// Who returns the teacher and the channel for the fixed text.
	Who func() (teacher, channel string)
	// OnPaused is called when the pause begins or ends, so the window shows
	// its band. Optional.
	OnPaused func(paused bool)
	Logger   *slog.Logger

	mu     sync.Mutex
	frozen bool
	yield  bool
}

// Capabilities lists what this freezer lets the agent declare in hello.
func (f *Freezer) Capabilities() []protocol.Capability {
	if f.Overlay == nil {
		return nil
	}
	return []protocol.Capability{protocol.CapScreenFreeze}
}

// Frozen reports whether the pause window is up.
func (f *Freezer) Frozen() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.frozen
}

// Handle processes screen.freeze and screen.unfreeze. It reports false for
// any other message.
func (f *Freezer) Handle(_ context.Context, env protocol.Envelope) (reply protocol.Payload, handled bool) {
	switch p := env.Payload.(type) {
	case protocol.ScreenFreeze:
		if f.Overlay == nil {
			return protocol.NewError(env.ID, protocol.CodeUnsupportedCapability, "este agente no soporta screen.freeze"), true
		}
		if err := f.freeze(p.Message); err != nil {
			f.log().Warn("pause window not shown", "err", err)
			return protocol.NewError(env.ID, protocol.CodeInternal, "no se pudo mostrar la pantalla en pausa"), true
		}
		return protocol.ScreenFreezeAck{RefID: env.ID, Frozen: true}, true
	case protocol.ScreenUnfreeze:
		if f.Overlay == nil {
			return protocol.NewError(env.ID, protocol.CodeUnsupportedCapability, "este agente no soporta screen.freeze"), true
		}
		f.Stop()
		return protocol.ScreenFreezeAck{RefID: env.ID, Frozen: false}, true
	}
	return nil, false
}

func (f *Freezer) freeze(message string) error {
	var teacher, channel string
	if f.Who != nil {
		teacher, channel = f.Who()
	}
	f.mu.Lock()
	began := !f.frozen
	if began {
		// Before Show, so the windows open already in their place.
		if err := f.Overlay.Yield(f.yield); err != nil {
			f.log().Warn("pause window: yield", "err", err)
		}
	}
	if err := f.Overlay.Show(PauseTextFor(teacher, channel, message)); err != nil {
		f.mu.Unlock()
		return err
	}
	f.frozen = true
	f.mu.Unlock()
	// Never the message: it is the teacher's text (rule 11).
	f.log().Info("screen paused", "new", began, "message_len", len([]rune(message)))
	if began && f.OnPaused != nil {
		f.OnPaused(true)
	}
	return nil
}

// Stop removes the pause, if any. The agent calls it on screen.unfreeze and
// when the connection closes (docs/privacidad.md rule 10); it is idempotent.
func (f *Freezer) Stop() {
	f.mu.Lock()
	was := f.frozen
	f.frozen = false
	if was && f.Overlay != nil {
		if err := f.Overlay.Hide(); err != nil {
			f.log().Warn("pause window: hide", "err", err)
		}
	}
	f.mu.Unlock()
	if was {
		f.log().Info("screen resumed")
		if f.OnPaused != nil {
			f.OnPaused(false)
		}
	}
}

// Yield keeps the pause window below the agent's window while on: during a
// broadcast, a power countdown or a remote control the student must see
// (docs/protocol.md §5.9). It is remembered for the next pause.
func (f *Freezer) Yield(on bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.yield == on {
		return
	}
	f.yield = on
	if f.frozen && f.Overlay != nil {
		if err := f.Overlay.Yield(on); err != nil {
			f.log().Warn("pause window: yield", "err", err)
		}
	}
}

func (f *Freezer) log() *slog.Logger {
	if f.Logger != nil {
		return f.Logger
	}
	return slog.Default()
}
