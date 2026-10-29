package protocol

import "fmt"

// Limits of the screen.thumb.* messages (docs/protocol.md §5.6–5.8, §7).
const (
	MinThumbInterval = 500    // ms: at most 2 frames per second
	MaxThumbInterval = 10_000 // ms
	MinThumbQuality  = 10
	MaxThumbQuality  = 95
	MinThumbWidth    = 160
	MaxThumbWidth    = 1920
	MaxFrameSide     = 1920
	MaxFrameJPEG     = 640 << 10 // bytes, before Base64
)

// ScreenThumbStart asks the agent to stream its primary display (P → A). A new
// start replaces the parameters of the previous one.
type ScreenThumbStart struct {
	IntervalMS int `json:"interval_ms"`
	Quality    int `json:"quality"`
	MaxWidth   int `json:"max_width"`
}

// MessageType implements Payload.
func (ScreenThumbStart) MessageType() Type { return TypeScreenThumbStart }

// Validate implements Payload.
func (p ScreenThumbStart) Validate() error {
	if p.IntervalMS < MinThumbInterval || p.IntervalMS > MaxThumbInterval {
		return invalid(TypeScreenThumbStart, "interval_ms must be %d-%d", MinThumbInterval, MaxThumbInterval)
	}
	if p.Quality < MinThumbQuality || p.Quality > MaxThumbQuality {
		return invalid(TypeScreenThumbStart, "quality must be %d-%d", MinThumbQuality, MaxThumbQuality)
	}
	if p.MaxWidth < MinThumbWidth || p.MaxWidth > MaxThumbWidth {
		return invalid(TypeScreenThumbStart, "max_width must be %d-%d", MinThumbWidth, MaxThumbWidth)
	}
	return nil
}

// ScreenThumbFrame is one JPEG frame of the stream (A → P).
type ScreenThumbFrame struct {
	RefID  string `json:"ref_id"` // ID of the screen.thumb.start in force
	Seq    int    `json:"seq"`    // 1, 2, 3… per start
	Width  int    `json:"width"`
	Height int    `json:"height"`
	JPEG   []byte `json:"jpeg"` // Base64 on the wire
}

// MessageType implements Payload.
func (ScreenThumbFrame) MessageType() Type { return TypeScreenThumbFrame }

// Validate implements Payload. An oversized image wraps ErrMessageTooLarge.
func (p ScreenThumbFrame) Validate() error {
	if !isUUIDv4(p.RefID) {
		return invalid(TypeScreenThumbFrame, "ref_id is not a UUID v4")
	}
	if p.Seq < 1 {
		return invalid(TypeScreenThumbFrame, "seq must be at least 1")
	}
	if p.Width < 1 || p.Width > MaxFrameSide || p.Height < 1 || p.Height > MaxFrameSide {
		return invalid(TypeScreenThumbFrame, "width and height must be 1-%d", MaxFrameSide)
	}
	if len(p.JPEG) == 0 {
		return invalid(TypeScreenThumbFrame, "jpeg is empty")
	}
	if len(p.JPEG) > MaxFrameJPEG {
		return fmt.Errorf("%s: jpeg is %d bytes, limit %d: %w", TypeScreenThumbFrame, len(p.JPEG), MaxFrameJPEG, ErrMessageTooLarge)
	}
	return nil
}

// ScreenThumbStop ends the stream (P → A). Idempotent; no reply.
type ScreenThumbStop struct{}

// MessageType implements Payload.
func (ScreenThumbStop) MessageType() Type { return TypeScreenThumbStop }

// Validate implements Payload.
func (ScreenThumbStop) Validate() error { return nil }
