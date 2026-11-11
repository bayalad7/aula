package poll

import "time"

// bucket is a token bucket: burst messages at once, then one every every.
type bucket struct {
	burst  int
	every  time.Duration
	tokens float64
	last   time.Time
}

func newBucket(burst int, every time.Duration, now time.Time) *bucket {
	return &bucket{burst: burst, every: every, tokens: float64(burst), last: now}
}

// take reports whether one more message fits at now, and spends it.
func (b *bucket) take(now time.Time) bool {
	if d := now.Sub(b.last); d > 0 {
		b.tokens = min(float64(b.burst), b.tokens+float64(d)/float64(b.every))
	}
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
