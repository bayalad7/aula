package protocol

import (
	"strings"
	"unicode/utf8"
)

// Limits of the student.present.* messages (docs/protocol.md §5.24, §7).
const (
	MinPresentWidth   = 640
	MaxPresentWidth   = 1920
	MaxPresentFPS     = 15
	MinPresentKbps    = 800
	MaxPresentKbps    = 2500
	MaxSourceHostname = 63
)

// PresentStatus is the state a student.present.ack reports.
type PresentStatus string

// Statuses of a presentation, as the source agent reports them. The student
// is not asked: the teacher decides when it starts and ends (like a remote
// control, D-019); the band and the badge come before the first frame.
const (
	PresentAccepted PresentStatus = "accepted" // the band is on screen and the capture ready; the offer may follow
	PresentEnded    PresentStatus = "ended"    // stopped by the console or replaced
	PresentFailed   PresentStatus = "failed"   // see Reason
)

// PresentReason explains a failed presentation.
type PresentReason string

// Reasons of a failed presentation.
const (
	PresentCaptureUnavailable PresentReason = "capture_unavailable"
	PresentEncoderUnavailable PresentReason = "encoder_unavailable"
	PresentBusy               PresentReason = "busy"
)

// Final reports whether s ends the presentation.
func (s PresentStatus) Final() bool {
	return s == PresentEnded || s == PresentFailed
}

// BroadcastSourceStudent is the kind of a broadcast that shows a student's
// screen (docs/protocol.md §5.24).
const BroadcastSourceStudent = "student"

// BroadcastSource says whose screen a broadcast shows; absent, the teacher's.
type BroadcastSource struct {
	Kind     string `json:"kind"`
	Hostname string `json:"hostname"`
}

func (s BroadcastSource) validate() error {
	if s.Kind != BroadcastSourceStudent {
		return invalid(TypeScreenBroadcastStart, "source.kind must be %q", BroadcastSourceStudent)
	}
	if n := utf8.RuneCountInString(s.Hostname); n < 1 || n > MaxSourceHostname || strings.TrimSpace(s.Hostname) == "" {
		return invalid(TypeScreenBroadcastStart, "source.hostname must be 1-%d characters", MaxSourceHostname)
	}
	return nil
}

// StudentPresentStart shows a student's screen to the class (P → A). The
// agent shows its band and badge before capturing anything.
type StudentPresentStart struct {
	PresentID string `json:"present_id"`
	MaxWidth  int    `json:"max_width"`
	FPS       int    `json:"fps"`
	MaxKbps   int    `json:"max_kbps"`
}

// MessageType implements Payload.
func (StudentPresentStart) MessageType() Type { return TypeStudentPresentStart }

// Validate implements Payload.
func (p StudentPresentStart) Validate() error {
	if !isUUIDv4(p.PresentID) {
		return invalid(TypeStudentPresentStart, "present_id is not a UUID v4")
	}
	if p.MaxWidth < MinPresentWidth || p.MaxWidth > MaxPresentWidth {
		return invalid(TypeStudentPresentStart, "max_width must be %d-%d", MinPresentWidth, MaxPresentWidth)
	}
	if p.FPS < 1 || p.FPS > MaxPresentFPS {
		return invalid(TypeStudentPresentStart, "fps must be 1-%d", MaxPresentFPS)
	}
	if p.MaxKbps < MinPresentKbps || p.MaxKbps > MaxPresentKbps {
		return invalid(TypeStudentPresentStart, "max_kbps must be %d-%d", MinPresentKbps, MaxPresentKbps)
	}
	return nil
}

// StudentPresentAck reports the state of a presentation (A → P).
type StudentPresentAck struct {
	RefID     string        `json:"ref_id"`
	PresentID string        `json:"present_id"`
	Status    PresentStatus `json:"status"`
	Width     int           `json:"width,omitempty"`
	Height    int           `json:"height,omitempty"`
	Reason    PresentReason `json:"reason,omitempty"`
}

// MessageType implements Payload.
func (StudentPresentAck) MessageType() Type { return TypeStudentPresentAck }

// Validate implements Payload.
func (p StudentPresentAck) Validate() error {
	if !isUUIDv4(p.RefID) {
		return invalid(TypeStudentPresentAck, "ref_id is not a UUID v4")
	}
	if !isUUIDv4(p.PresentID) {
		return invalid(TypeStudentPresentAck, "present_id is not a UUID v4")
	}
	switch p.Status {
	case PresentAccepted:
		if p.Width < 2 || p.Width > MaxBroadcastSide || p.Height < 2 || p.Height > MaxBroadcastSide ||
			p.Width%2 != 0 || p.Height%2 != 0 {
			return invalid(TypeStudentPresentAck, "accepted needs even width and height of 2-%d", MaxBroadcastSide)
		}
	case PresentEnded:
		if p.Width != 0 || p.Height != 0 {
			return invalid(TypeStudentPresentAck, "width and height only with accepted")
		}
	case PresentFailed:
		switch p.Reason {
		case PresentCaptureUnavailable, PresentEncoderUnavailable, PresentBusy:
		default:
			return invalid(TypeStudentPresentAck, "unknown reason %q", p.Reason)
		}
		return nil
	default:
		return invalid(TypeStudentPresentAck, "unknown status %q", p.Status)
	}
	if p.Reason != "" {
		return invalid(TypeStudentPresentAck, "reason only with failed")
	}
	return nil
}

// StudentPresentOffer carries the console's receive-only SDP offer (P → A).
type StudentPresentOffer struct {
	PresentID string `json:"present_id"`
	Attempt   int    `json:"attempt"`
	SDP       string `json:"sdp"`
}

// MessageType implements Payload.
func (StudentPresentOffer) MessageType() Type { return TypeStudentPresentOffer }

// Validate implements Payload. An oversized SDP wraps ErrMessageTooLarge.
func (p StudentPresentOffer) Validate() error {
	return validatePresentSDP(TypeStudentPresentOffer, p.PresentID, p.Attempt, p.SDP)
}

// StudentPresentAnswer carries the source agent's send-only answer (A → P).
type StudentPresentAnswer struct {
	RefID     string `json:"ref_id"` // ID of the student.present.offer
	PresentID string `json:"present_id"`
	Attempt   int    `json:"attempt"`
	SDP       string `json:"sdp"`
}

// MessageType implements Payload.
func (StudentPresentAnswer) MessageType() Type { return TypeStudentPresentAnswer }

// Validate implements Payload. An oversized SDP wraps ErrMessageTooLarge.
func (p StudentPresentAnswer) Validate() error {
	if !isUUIDv4(p.RefID) {
		return invalid(TypeStudentPresentAnswer, "ref_id is not a UUID v4")
	}
	return validatePresentSDP(TypeStudentPresentAnswer, p.PresentID, p.Attempt, p.SDP)
}

// StudentPresentICE is one trickled ICE candidate of the source leg (A ↔ P).
// An empty Candidate means end of candidates.
type StudentPresentICE struct {
	PresentID     string `json:"present_id"`
	Attempt       int    `json:"attempt"`
	Candidate     string `json:"candidate"`
	SDPMid        string `json:"sdp_mid"`
	SDPMLineIndex int    `json:"sdp_mline_index"`
}

// MessageType implements Payload.
func (StudentPresentICE) MessageType() Type { return TypeStudentPresentICE }

// Validate implements Payload.
func (p StudentPresentICE) Validate() error {
	return validateCandidate(TypeStudentPresentICE, p.PresentID, p.Attempt, p.Candidate, p.SDPMid, p.SDPMLineIndex)
}

// StudentPresentStop ends the presentation on the source (P → A).
// Idempotent; answered with ack ended.
type StudentPresentStop struct {
	PresentID string `json:"present_id"`
}

// MessageType implements Payload.
func (StudentPresentStop) MessageType() Type { return TypeStudentPresentStop }

// Validate implements Payload.
func (p StudentPresentStop) Validate() error {
	if !isUUIDv4(p.PresentID) {
		return invalid(TypeStudentPresentStop, "present_id is not a UUID v4")
	}
	return nil
}

func validatePresentSDP(t Type, presentID string, attempt int, sdp string) error {
	if !isUUIDv4(presentID) {
		return invalid(t, "present_id is not a UUID v4")
	}
	return validateSDP(t, presentID, attempt, sdp)
}
