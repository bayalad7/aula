package protocol

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Limits of the screen.broadcast.* messages (docs/protocol.md §5.12, §7).
const (
	MaxBroadcastSide    = 4096
	MaxBroadcastFPS     = 30
	MaxBroadcastAttempt = 1000
	MaxSDP              = 64 << 10 // bytes
	MaxICECandidate     = 1024     // characters
	MaxSDPMid           = 32       // characters
	MaxSDPMLineIndex    = 15
	// MaxICECandidates is how many candidates each side accepts per attempt;
	// the rest are dropped (enforced by the receiver, not by Validate).
	MaxICECandidates = 50
)

// ScreenBroadcastStart announces a broadcast of the teacher's screen (P → A).
// The agent shows the broadcast view and its notice before handling the offer.
type ScreenBroadcastStart struct {
	BroadcastID string `json:"broadcast_id"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	FPS         int    `json:"fps"`
}

// MessageType implements Payload.
func (ScreenBroadcastStart) MessageType() Type { return TypeScreenBroadcastStart }

// Validate implements Payload.
func (p ScreenBroadcastStart) Validate() error {
	if !isUUIDv4(p.BroadcastID) {
		return invalid(TypeScreenBroadcastStart, "broadcast_id is not a UUID v4")
	}
	if p.Width < 1 || p.Width > MaxBroadcastSide || p.Height < 1 || p.Height > MaxBroadcastSide {
		return invalid(TypeScreenBroadcastStart, "width and height must be 1-%d", MaxBroadcastSide)
	}
	if p.FPS < 1 || p.FPS > MaxBroadcastFPS {
		return invalid(TypeScreenBroadcastStart, "fps must be 1-%d", MaxBroadcastFPS)
	}
	return nil
}

// ScreenBroadcastOffer carries the console's SDP offer (P → A).
type ScreenBroadcastOffer struct {
	BroadcastID string `json:"broadcast_id"`
	Attempt     int    `json:"attempt"`
	SDP         string `json:"sdp"`
}

// MessageType implements Payload.
func (ScreenBroadcastOffer) MessageType() Type { return TypeScreenBroadcastOffer }

// Validate implements Payload. An oversized SDP wraps ErrMessageTooLarge.
func (p ScreenBroadcastOffer) Validate() error {
	return validateSDP(TypeScreenBroadcastOffer, p.BroadcastID, p.Attempt, p.SDP)
}

// ScreenBroadcastAnswer carries the agent's SDP answer (A → P).
type ScreenBroadcastAnswer struct {
	RefID       string `json:"ref_id"` // ID of the screen.broadcast.offer
	BroadcastID string `json:"broadcast_id"`
	Attempt     int    `json:"attempt"`
	SDP         string `json:"sdp"`
}

// MessageType implements Payload.
func (ScreenBroadcastAnswer) MessageType() Type { return TypeScreenBroadcastAnswer }

// Validate implements Payload. An oversized SDP wraps ErrMessageTooLarge.
func (p ScreenBroadcastAnswer) Validate() error {
	if !isUUIDv4(p.RefID) {
		return invalid(TypeScreenBroadcastAnswer, "ref_id is not a UUID v4")
	}
	return validateSDP(TypeScreenBroadcastAnswer, p.BroadcastID, p.Attempt, p.SDP)
}

// ScreenBroadcastICE is one trickled ICE candidate (A ↔ P). An empty Candidate
// means end of candidates.
type ScreenBroadcastICE struct {
	BroadcastID   string `json:"broadcast_id"`
	Attempt       int    `json:"attempt"`
	Candidate     string `json:"candidate"`
	SDPMid        string `json:"sdp_mid"`
	SDPMLineIndex int    `json:"sdp_mline_index"`
}

// MessageType implements Payload.
func (ScreenBroadcastICE) MessageType() Type { return TypeScreenBroadcastICE }

// Validate implements Payload.
func (p ScreenBroadcastICE) Validate() error {
	if err := validateAttempt(TypeScreenBroadcastICE, p.BroadcastID, p.Attempt); err != nil {
		return err
	}
	if utf8.RuneCountInString(p.Candidate) > MaxICECandidate {
		return invalid(TypeScreenBroadcastICE, "candidate longer than %d characters", MaxICECandidate)
	}
	if p.Candidate != "" && !strings.HasPrefix(p.Candidate, "candidate:") {
		return invalid(TypeScreenBroadcastICE, `candidate must start with "candidate:"`)
	}
	if utf8.RuneCountInString(p.SDPMid) > MaxSDPMid {
		return invalid(TypeScreenBroadcastICE, "sdp_mid longer than %d characters", MaxSDPMid)
	}
	if p.SDPMLineIndex < 0 || p.SDPMLineIndex > MaxSDPMLineIndex {
		return invalid(TypeScreenBroadcastICE, "sdp_mline_index must be 0-%d", MaxSDPMLineIndex)
	}
	return nil
}

// ScreenBroadcastStop ends the broadcast (P → A). Idempotent; no reply.
type ScreenBroadcastStop struct {
	BroadcastID string `json:"broadcast_id"`
}

// MessageType implements Payload.
func (ScreenBroadcastStop) MessageType() Type { return TypeScreenBroadcastStop }

// Validate implements Payload.
func (p ScreenBroadcastStop) Validate() error {
	if !isUUIDv4(p.BroadcastID) {
		return invalid(TypeScreenBroadcastStop, "broadcast_id is not a UUID v4")
	}
	return nil
}

func validateAttempt(t Type, broadcastID string, attempt int) error {
	if !isUUIDv4(broadcastID) {
		return invalid(t, "broadcast_id is not a UUID v4")
	}
	if attempt < 1 || attempt > MaxBroadcastAttempt {
		return invalid(t, "attempt must be 1-%d", MaxBroadcastAttempt)
	}
	return nil
}

func validateSDP(t Type, broadcastID string, attempt int, sdp string) error {
	if err := validateAttempt(t, broadcastID, attempt); err != nil {
		return err
	}
	if len(sdp) > MaxSDP {
		return fmt.Errorf("%s: sdp is %d bytes, limit %d: %w", t, len(sdp), MaxSDP, ErrMessageTooLarge)
	}
	if !utf8.ValidString(sdp) || !strings.HasPrefix(sdp, "v=0") {
		return invalid(t, `sdp must be UTF-8 and start with "v=0"`)
	}
	return nil
}
