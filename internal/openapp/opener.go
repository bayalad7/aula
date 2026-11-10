package openapp

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
)

// Pace of the agent: at most Burst in a row, then one every Every.
const (
	Burst = 3
	Every = 3 * time.Second
)

// Kinds of what opened.
const (
	KindURL = "url"
	KindApp = "app"
)

// Sender sends a message to the console.
type Sender func(ctx context.Context, p protocol.Payload) error

// Opened is what the window's corner notice says (docs/privacidad.md rule
// 66): what opened, and who asked for it. Go sends the state; the window
// words it.
type Opened struct {
	ID      string    `json:"id"`      // open_id
	Kind    string    `json:"kind"`    // KindURL or KindApp
	Shown   string    `json:"shown"`   // the URL's host and path (Shown), or the app's name
	Full    string    `json:"full"`    // the whole URL, or the app's name, for the tooltip
	Teacher string    `json:"teacher"` // the console that asked, as the strip names it
	Channel string    `json:"channel"`
	At      time.Time `json:"at"`
}

// Opener answers open.url and open.app on the agent (docs/protocol.md §5.22).
type Opener struct {
	// Session opens in the student's session; nil: the agent does not
	// declare open.* (no session or no helper).
	Session Session
	// Who returns the teacher and channel of the connected console.
	Who func() (teacher, channel string)
	// OnOpened is called after the OS opened it.
	OnOpened func(Opened)
	Logger   *slog.Logger
	Now      func() time.Time // time.Now if nil

	mu     sync.Mutex
	tokens float64
	last   time.Time
	wg     sync.WaitGroup
}

// Available reports whether the agent can open anything.
func (o *Opener) Available() bool { return o != nil && o.Session != nil }

// Capabilities are open.url and open.app when Available.
func (o *Opener) Capabilities() []protocol.Capability {
	if !o.Available() {
		return nil
	}
	return []protocol.Capability{protocol.CapOpenURL, protocol.CapOpenApp}
}

func (o *Opener) now() time.Time {
	if o.Now != nil {
		return o.Now()
	}
	return time.Now()
}

func (o *Opener) log() *slog.Logger {
	if o.Logger != nil {
		return o.Logger
	}
	return slog.Default()
}

// take spends one token of the pace; it reports false when there is none.
func (o *Opener) take() bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	now := o.now()
	if o.last.IsZero() {
		o.tokens = Burst
	} else {
		o.tokens = min(Burst, o.tokens+float64(now.Sub(o.last))/float64(Every))
	}
	o.last = now
	if o.tokens < 1 {
		return false
	}
	o.tokens--
	return true
}

// Handle processes open.url and open.app and reports false for any other
// message. It opens in the background, tied to ctx (the connection), and
// sends the open.ack when the OS answered. It never logs the URL, the app's
// name or its path (docs/privacidad.md rule 70).
func (o *Opener) Handle(ctx context.Context, send Sender, env protocol.Envelope) (reply protocol.Payload, handled bool) {
	var openID, kind string
	var do func() error
	var opened Opened
	switch p := env.Payload.(type) {
	case protocol.OpenURL:
		openID, kind = p.OpenID, KindURL
		opened = Opened{ID: p.OpenID, Kind: KindURL, Shown: Shown(p.URL), Full: p.URL}
		do = func() error { return o.Session.OpenURL(p.URL) }
	case protocol.OpenApp:
		openID, kind = p.OpenID, KindApp
		opened = Opened{ID: p.OpenID, Kind: KindApp, Shown: p.Name, Full: p.Name}
		do = func() error { return o.Session.StartApp(p.Path) }
	default:
		return nil, false
	}
	if !o.Available() {
		return protocol.NewError(env.ID, protocol.CodeUnsupportedCapability, "este agente no soporta "+string(env.Type)), true
	}
	log := o.log().With("open_id", openID, "kind", kind)
	if !o.take() {
		log.Info("open refused", "reason", protocol.OpenBusy)
		return protocol.OpenAck{RefID: env.ID, OpenID: openID, Status: protocol.OpenFailed, Reason: protocol.OpenBusy}, true
	}
	if o.Who != nil {
		opened.Teacher, opened.Channel = o.Who()
	}
	o.wg.Add(1)
	go func() {
		defer o.wg.Done()
		ack := protocol.OpenAck{RefID: env.ID, OpenID: openID, Status: protocol.OpenOpened}
		if err := do(); err != nil {
			ack.Status, ack.Reason = protocol.OpenFailed, Reason(err)
			log.Info("open failed", "reason", ack.Reason)
		} else {
			log.Info("opened")
			opened.At = o.now()
			if o.OnOpened != nil {
				o.OnOpened(opened)
			}
		}
		if ctx.Err() == nil {
			if err := send(ctx, ack); err != nil {
				log.Debug("open ack not sent", "err", err)
			}
		}
	}()
	return nil, true
}

// Wait waits for the openings in flight.
func (o *Opener) Wait() { o.wg.Wait() }
