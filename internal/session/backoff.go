package session

import (
	"math/rand/v2"
	"time"
)

// Backoff computes reconnection delays: min(Max, Base × 2^n) with ±Jitter,
// where n is the number of consecutive failures (docs/protocol.md §3.5).
// The zero value uses the protocol defaults: 1 s, 30 s, ±20 %.
type Backoff struct {
	Base   time.Duration
	Max    time.Duration
	Jitter float64        // fraction, e.g. 0.2 for ±20 %
	rand   func() float64 // [0,1); replaced in tests
}

// Delay returns the wait before the next attempt after n consecutive failures.
func (b Backoff) Delay(n int) time.Duration {
	base, maxDelay, jitter := b.Base, b.Max, b.Jitter
	if base <= 0 {
		base = time.Second
	}
	if maxDelay <= 0 {
		maxDelay = 30 * time.Second
	}
	if b.Base <= 0 && b.Jitter == 0 {
		jitter = 0.2
	}
	d := base
	for i := 0; i < n && d < maxDelay; i++ {
		d *= 2
	}
	d = min(d, maxDelay)
	if jitter > 0 {
		r := rand.Float64
		if b.rand != nil {
			r = b.rand
		}
		d = time.Duration(float64(d) * (1 + jitter*(2*r()-1)))
	}
	return d
}
