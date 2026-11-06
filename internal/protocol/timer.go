package protocol

import "unicode/utf8"

// Limits of the timer.* messages (docs/protocol.md §5.19, §7).
const (
	MaxTimerLabel     = 40
	MinTimerDurationS = 10
	MaxTimerDurationS = 4 * 60 * 60
	MaxTimerRemaining = MaxTimerDurationS * 1000 // milliseconds
	MinTimerExtendS   = 60
	MaxTimerExtendS   = 60 * 60
)

// TimerStatus is the state a timer.ack reports.
type TimerStatus string

// Timer statuses.
const (
	TimerRunning TimerStatus = "running" // the badge is counting
	TimerEnded   TimerStatus = "ended"   // the agent's countdown reached zero
	TimerStopped TimerStatus = "stopped" // stopped or replaced by another timer
)

// TimerStart shows a countdown to the student (P → A). RemainingMS rules:
// the agent ends at its own monotonic now + RemainingMS, never at a wall time.
type TimerStart struct {
	TimerID     string `json:"timer_id"`
	Label       string `json:"label"`
	DurationS   int    `json:"duration_s"`
	RemainingMS int64  `json:"remaining_ms"`
}

// MessageType implements Payload.
func (TimerStart) MessageType() Type { return TypeTimerStart }

// Validate implements Payload.
func (p TimerStart) Validate() error {
	if !isUUIDv4(p.TimerID) {
		return invalid(TypeTimerStart, "timer_id is not a UUID v4")
	}
	if utf8.RuneCountInString(p.Label) > MaxTimerLabel {
		return invalid(TypeTimerStart, "label exceeds %d characters", MaxTimerLabel)
	}
	if p.DurationS < MinTimerDurationS || p.DurationS > MaxTimerDurationS {
		return invalid(TypeTimerStart, "duration_s must be %d-%d", MinTimerDurationS, MaxTimerDurationS)
	}
	if p.RemainingMS < 1 || p.RemainingMS > MaxTimerRemaining {
		return invalid(TypeTimerStart, "remaining_ms must be 1-%d", MaxTimerRemaining)
	}
	return nil
}

// TimerExtend sets what is left of the running timer, already extended
// (P → A). AddS only informs: the agent takes RemainingMS, so a resend never
// extends twice.
type TimerExtend struct {
	TimerID     string `json:"timer_id"`
	AddS        int    `json:"add_s"`
	RemainingMS int64  `json:"remaining_ms"`
}

// MessageType implements Payload.
func (TimerExtend) MessageType() Type { return TypeTimerExtend }

// Validate implements Payload.
func (p TimerExtend) Validate() error {
	if !isUUIDv4(p.TimerID) {
		return invalid(TypeTimerExtend, "timer_id is not a UUID v4")
	}
	if p.AddS < MinTimerExtendS || p.AddS > MaxTimerExtendS {
		return invalid(TypeTimerExtend, "add_s must be %d-%d", MinTimerExtendS, MaxTimerExtendS)
	}
	if p.RemainingMS < 1 || p.RemainingMS > MaxTimerRemaining {
		return invalid(TypeTimerExtend, "remaining_ms must be 1-%d", MaxTimerRemaining)
	}
	return nil
}

// TimerStop removes the timer (P → A).
type TimerStop struct {
	TimerID string `json:"timer_id"`
}

// MessageType implements Payload.
func (TimerStop) MessageType() Type { return TypeTimerStop }

// Validate implements Payload.
func (p TimerStop) Validate() error {
	if !isUUIDv4(p.TimerID) {
		return invalid(TypeTimerStop, "timer_id is not a UUID v4")
	}
	return nil
}

// TimerAck reports the state of a timer on the agent (A → P).
type TimerAck struct {
	RefID       string      `json:"ref_id"`
	TimerID     string      `json:"timer_id"`
	Status      TimerStatus `json:"status"`
	RemainingMS int64       `json:"remaining_ms"`
}

// MessageType implements Payload.
func (TimerAck) MessageType() Type { return TypeTimerAck }

// Validate implements Payload.
func (p TimerAck) Validate() error {
	if !isUUIDv4(p.RefID) {
		return invalid(TypeTimerAck, "ref_id is not a UUID v4")
	}
	if !isUUIDv4(p.TimerID) {
		return invalid(TypeTimerAck, "timer_id is not a UUID v4")
	}
	switch p.Status {
	case TimerRunning, TimerEnded, TimerStopped:
	default:
		return invalid(TypeTimerAck, "status must be running, ended or stopped")
	}
	if p.RemainingMS < 0 || p.RemainingMS > MaxTimerRemaining {
		return invalid(TypeTimerAck, "remaining_ms must be 0-%d", MaxTimerRemaining)
	}
	return nil
}
