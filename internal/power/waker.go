package power

import (
	"strings"
	"sync"
	"time"
)

// WakeTimeout is how long «Encendiendo…» lasts before «No respondió»: a
// computer that wakes boots Windows and starts the agent well within it.
const WakeTimeout = 3 * time.Minute

// WakeGap separates the WakeRounds sends of one order.
const WakeGap = time.Second

// Wake states of a computer the teacher asked to turn on.
const (
	WakeWaking     = "waking"     // the packets went out; waiting for its hello
	WakeNoResponse = "noresponse" // WakeTimeout passed without a hello
)

// WakeView is the wake state of one computer, as its row shows it.
type WakeView struct {
	State string `json:"state"`
	// DeadlineMS is when «Encendiendo…» ends (Unix ms); the row counts down.
	DeadlineMS int64 `json:"deadline_ms"`
	// OtherSubnet: the console has no card on the computer's subnet, so the
	// packet probably did not reach it.
	OtherSubnet bool `json:"other_subnet"`
}

// Waker follows the computers being turned on until their agent says hello.
type Waker struct {
	now     func() time.Time
	timeout time.Duration

	mu    sync.Mutex
	wakes map[string]wake // wakeKey
}

type wake struct {
	deadline    time.Time
	otherSubnet bool
}

// NewWaker returns a Waker; timeout <= 0 means WakeTimeout.
func NewWaker(timeout time.Duration) *Waker {
	if timeout <= 0 {
		timeout = WakeTimeout
	}
	return &Waker{now: time.Now, timeout: timeout, wakes: map[string]wake{}}
}

func wakeKey(channel, hostname string) string { return channel + "\x00" + strings.ToLower(hostname) }

// Timeout is how long a wake waits for the hello.
func (w *Waker) Timeout() time.Duration { return w.timeout }

// Start records that the packets for hostname went out now.
func (w *Waker) Start(channel, hostname string, otherSubnet bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.wakes[wakeKey(channel, hostname)] = wake{deadline: w.now().Add(w.timeout), otherSubnet: otherSubnet}
}

// Arrived ends the wake of hostname: its agent said hello. It reports whether
// there was one.
func (w *Waker) Arrived(channel, hostname string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	k := wakeKey(channel, hostname)
	_, ok := w.wakes[k]
	delete(w.wakes, k)
	return ok
}

// View returns the wake state of hostname, if the teacher asked to turn it on.
func (w *Waker) View(channel, hostname string) (WakeView, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	wk, ok := w.wakes[wakeKey(channel, hostname)]
	if !ok {
		return WakeView{}, false
	}
	v := WakeView{State: WakeWaking, DeadlineMS: wk.deadline.UnixMilli(), OtherSubnet: wk.otherSubnet}
	if !w.now().Before(wk.deadline) {
		v.State = WakeNoResponse
	}
	return v, true
}
