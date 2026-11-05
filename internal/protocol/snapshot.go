package protocol

import (
	"fmt"
	"unicode/utf8"
)

// Limits of the screen.snapshot.* messages (docs/protocol.md §5.17, §7).
const (
	MinSnapshotQuality = 50
	MaxSnapshotQuality = 95
	// DefaultSnapshotQuality is what the console asks for.
	DefaultSnapshotQuality = 90
	// max_width is 0 (the display's native width) or MinSnapshotWidth-MaxSnapshotWidth.
	MinSnapshotWidth = 640
	MaxSnapshotWidth = 7680
	MaxSnapshotSide  = 8192
	// SnapshotPartSize is the image bytes of every part but the last.
	SnapshotPartSize = 512 << 10
	MaxSnapshotParts = 32
	MaxSnapshotBytes = SnapshotPartSize * MaxSnapshotParts // 16 MiB
)

// ScreenSnapshotRequest asks the agent for one still image of its primary
// display (P → A).
type ScreenSnapshotRequest struct {
	SnapshotID string `json:"snapshot_id"`
	Quality    int    `json:"quality"`
	MaxWidth   int    `json:"max_width"` // 0: native resolution
}

// MessageType implements Payload.
func (ScreenSnapshotRequest) MessageType() Type { return TypeScreenSnapshotRequest }

// Validate implements Payload.
func (p ScreenSnapshotRequest) Validate() error {
	if !isUUIDv4(p.SnapshotID) {
		return invalid(TypeScreenSnapshotRequest, "snapshot_id is not a UUID v4")
	}
	if p.Quality < MinSnapshotQuality || p.Quality > MaxSnapshotQuality {
		return invalid(TypeScreenSnapshotRequest, "quality must be %d-%d", MinSnapshotQuality, MaxSnapshotQuality)
	}
	if p.MaxWidth != 0 && (p.MaxWidth < MinSnapshotWidth || p.MaxWidth > MaxSnapshotWidth) {
		return invalid(TypeScreenSnapshotRequest, "max_width must be 0 or %d-%d", MinSnapshotWidth, MaxSnapshotWidth)
	}
	return nil
}

// ScreenSnapshotResponse is one part of the JPEG (A → P). Part 1 also carries
// what the console needs to check and file the whole image.
type ScreenSnapshotResponse struct {
	RefID      string `json:"ref_id"` // ID of the screen.snapshot.request
	SnapshotID string `json:"snapshot_id"`
	Part       int    `json:"part"`  // 1..Parts
	Parts      int    `json:"parts"` // 1..MaxSnapshotParts
	Data       []byte `json:"data"`  // Base64 on the wire

	// Part 1 only.
	TakenAt  int64  `json:"taken_at,omitempty"` // Unix ms
	Hostname string `json:"hostname,omitempty"`
	Username string `json:"username,omitempty"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
	Size     int    `json:"size,omitempty"`   // bytes of the whole JPEG
	SHA256   string `json:"sha256,omitempty"` // lowercase hex of the whole JPEG
}

// MessageType implements Payload.
func (ScreenSnapshotResponse) MessageType() Type { return TypeScreenSnapshotResponse }

// Validate implements Payload. An oversized part wraps ErrMessageTooLarge.
func (p ScreenSnapshotResponse) Validate() error {
	const t = TypeScreenSnapshotResponse
	if !isUUIDv4(p.RefID) {
		return invalid(t, "ref_id is not a UUID v4")
	}
	if !isUUIDv4(p.SnapshotID) {
		return invalid(t, "snapshot_id is not a UUID v4")
	}
	if p.Parts < 1 || p.Parts > MaxSnapshotParts || p.Part < 1 || p.Part > p.Parts {
		return invalid(t, "part and parts must satisfy 1 <= part <= parts <= %d", MaxSnapshotParts)
	}
	if len(p.Data) == 0 {
		return invalid(t, "data is empty")
	}
	if len(p.Data) > SnapshotPartSize {
		return fmt.Errorf("%s: part is %d bytes, limit %d: %w", t, len(p.Data), SnapshotPartSize, ErrMessageTooLarge)
	}
	if p.Part != 1 {
		return nil
	}
	if p.TakenAt <= 0 {
		return invalid(t, "taken_at must be a positive Unix time in ms")
	}
	if n := utf8.RuneCountInString(p.Hostname); n < 1 || n > MaxHostname {
		return invalid(t, "hostname must be 1-%d characters", MaxHostname)
	}
	if utf8.RuneCountInString(p.Username) > MaxUsername {
		return invalid(t, "username exceeds %d characters", MaxUsername)
	}
	if p.Width < 1 || p.Width > MaxSnapshotSide || p.Height < 1 || p.Height > MaxSnapshotSide {
		return invalid(t, "width and height must be 1-%d", MaxSnapshotSide)
	}
	if p.Size < 1 || p.Size > MaxSnapshotBytes {
		return invalid(t, "size must be 1-%d", MaxSnapshotBytes)
	}
	if want := (p.Size + SnapshotPartSize - 1) / SnapshotPartSize; p.Parts != want {
		return invalid(t, "parts must be %d for size %d", want, p.Size)
	}
	if !isSHA256Hex(p.SHA256) {
		return invalid(t, "sha256 must be 64 lowercase hex characters")
	}
	return nil
}

func isSHA256Hex(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}
