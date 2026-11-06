package protocol

import (
	"fmt"
	"unicode/utf8"
)

// Limits of the file.push messages (docs/protocol.md §5.20, §7).
const (
	// FilePushPartSize is the file bytes of every part but the last.
	FilePushPartSize = 512 << 10
	MaxFilePushParts = 32
	MaxFilePushBytes = FilePushPartSize * MaxFilePushParts // 16 MiB per file
	MaxFilePushName  = 100                                 // characters of the name and of each dest segment
	MaxFilePushDest  = 200                                 // characters of dest
	MaxFilePushDepth = 4                                   // segments of dest
	MaxFilePushLabel = 80
	MaxFilePushFiles = 20       // files in one push
	MaxFilePushTotal = 32 << 20 // bytes of one push, every file together
)

// FilePushStatus is the outcome a file.push.ack reports.
type FilePushStatus string

// File push statuses.
const (
	FilePushSaved  FilePushStatus = "saved"
	FilePushFailed FilePushStatus = "failed"
)

// FilePushReason says why a file was not saved (status failed).
type FilePushReason string

// File push failure reasons (docs/protocol.md §5.20).
const (
	FilePushInvalidPath  FilePushReason = "invalid_path"
	FilePushTooLarge     FilePushReason = "too_large"
	FilePushBusy         FilePushReason = "busy"
	FilePushIncomplete   FilePushReason = "incomplete"
	FilePushCorrupt      FilePushReason = "corrupt"
	FilePushExists       FilePushReason = "exists"
	FilePushNoSpace      FilePushReason = "no_space"
	FilePushDenied       FilePushReason = "denied"
	FilePushPathTooLong  FilePushReason = "path_too_long"
	FilePushUnsupported  FilePushReason = "unsupported"
	FilePushInternalFail FilePushReason = "internal"
)

// FilePushReasons lists every valid reason, in the order of the protocol.
var FilePushReasons = []FilePushReason{
	FilePushInvalidPath, FilePushTooLarge, FilePushBusy, FilePushIncomplete, FilePushCorrupt,
	FilePushExists, FilePushNoSpace, FilePushDenied, FilePushPathTooLong, FilePushUnsupported,
	FilePushInternalFail,
}

// FilePush is one part of one file the teacher sends (P → A). Part 1 also
// carries where to save it and how to check it. The path rules (name and dest
// segments) are checked by filepush, which answers invalid_path; Validate only
// enforces lengths.
type FilePush struct {
	PushID string `json:"push_id"`
	FileID string `json:"file_id"`
	Part   int    `json:"part"`  // 1..Parts
	Parts  int    `json:"parts"` // 1..MaxFilePushParts
	Data   []byte `json:"data"`  // Base64 on the wire

	// Part 1 only.
	Name      string `json:"name,omitempty"`
	Dest      string `json:"dest,omitempty"` // "/"-separated, relative to the work folder
	Size      int    `json:"size,omitempty"`
	SHA256    string `json:"sha256,omitempty"`
	Overwrite bool   `json:"overwrite,omitempty"`
	Label     string `json:"label,omitempty"`
	Index     int    `json:"index,omitempty"` // 1..Files
	Files     int    `json:"files,omitempty"` // files in the push
}

// MessageType implements Payload.
func (FilePush) MessageType() Type { return TypeFilePush }

// Validate implements Payload. An oversized part wraps ErrMessageTooLarge.
func (p FilePush) Validate() error {
	const t = TypeFilePush
	if !isUUIDv4(p.PushID) {
		return invalid(t, "push_id is not a UUID v4")
	}
	if !isUUIDv4(p.FileID) {
		return invalid(t, "file_id is not a UUID v4")
	}
	if p.Parts < 1 || p.Parts > MaxFilePushParts || p.Part < 1 || p.Part > p.Parts {
		return invalid(t, "part and parts must satisfy 1 <= part <= parts <= %d", MaxFilePushParts)
	}
	if len(p.Data) > FilePushPartSize {
		return fmt.Errorf("%s: part is %d bytes, limit %d: %w", t, len(p.Data), FilePushPartSize, ErrMessageTooLarge)
	}
	if p.Part != 1 {
		if len(p.Data) == 0 {
			return invalid(t, "data is empty")
		}
		return nil
	}
	if n := utf8.RuneCountInString(p.Name); n < 1 || n > MaxFilePushName {
		return invalid(t, "name must be 1-%d characters", MaxFilePushName)
	}
	if utf8.RuneCountInString(p.Dest) > MaxFilePushDest {
		return invalid(t, "dest exceeds %d characters", MaxFilePushDest)
	}
	if utf8.RuneCountInString(p.Label) > MaxFilePushLabel {
		return invalid(t, "label exceeds %d characters", MaxFilePushLabel)
	}
	if !utf8.ValidString(p.Name) || !utf8.ValidString(p.Dest) || !utf8.ValidString(p.Label) {
		return invalid(t, "name, dest and label must be valid UTF-8")
	}
	if p.Size < 0 || p.Size > MaxFilePushBytes {
		return invalid(t, "size must be 0-%d", MaxFilePushBytes)
	}
	if want := max(1, (p.Size+FilePushPartSize-1)/FilePushPartSize); p.Parts != want {
		return invalid(t, "parts must be %d for size %d", want, p.Size)
	}
	if p.Size == 0 && len(p.Data) != 0 {
		return invalid(t, "an empty file carries no data")
	}
	if p.Size > 0 && len(p.Data) == 0 {
		return invalid(t, "data is empty")
	}
	if !isSHA256Hex(p.SHA256) {
		return invalid(t, "sha256 must be 64 lowercase hex characters")
	}
	if p.Files < 1 || p.Files > MaxFilePushFiles || p.Index < 1 || p.Index > p.Files {
		return invalid(t, "index and files must satisfy 1 <= index <= files <= %d", MaxFilePushFiles)
	}
	return nil
}

// FilePushAck says whether one file was saved (A → P).
type FilePushAck struct {
	RefID    string         `json:"ref_id"` // ID of the part that completed or rejected the file
	PushID   string         `json:"push_id"`
	FileID   string         `json:"file_id"`
	Status   FilePushStatus `json:"status"`
	Path     string         `json:"path,omitempty"` // saved: final path, relative, "/"-separated
	Renamed  bool           `json:"renamed,omitempty"`
	Replaced bool           `json:"replaced,omitempty"`
	Reason   FilePushReason `json:"reason,omitempty"` // failed only
}

// MessageType implements Payload.
func (FilePushAck) MessageType() Type { return TypeFilePushAck }

// Validate implements Payload.
func (p FilePushAck) Validate() error {
	const t = TypeFilePushAck
	if !isUUIDv4(p.RefID) {
		return invalid(t, "ref_id is not a UUID v4")
	}
	if !isUUIDv4(p.PushID) {
		return invalid(t, "push_id is not a UUID v4")
	}
	if !isUUIDv4(p.FileID) {
		return invalid(t, "file_id is not a UUID v4")
	}
	switch p.Status {
	case FilePushSaved:
		if n := utf8.RuneCountInString(p.Path); n < 1 || n > MaxFilePushDest+1+MaxFilePushName+8 {
			return invalid(t, "a saved file needs its path")
		}
		if p.Reason != "" {
			return invalid(t, "a saved file has no reason")
		}
		if p.Renamed && p.Replaced {
			return invalid(t, "renamed and replaced exclude each other")
		}
	case FilePushFailed:
		if p.Path != "" || p.Renamed || p.Replaced {
			return invalid(t, "a failed file has no path")
		}
		ok := false
		for _, r := range FilePushReasons {
			ok = ok || r == p.Reason
		}
		if !ok {
			return invalid(t, "unknown reason %q", p.Reason)
		}
	default:
		return invalid(t, "status must be saved or failed")
	}
	return nil
}
