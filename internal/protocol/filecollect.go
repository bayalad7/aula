package protocol

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Limits of the file.collect.* messages (docs/protocol.md §5.21, §7).
const (
	// FileCollectPartSize is the file bytes of every part but the last.
	FileCollectPartSize = 512 << 10
	MaxFileCollectParts = 32
	MaxFileCollectBytes = FileCollectPartSize * MaxFileCollectParts // 16 MiB per file
	MaxFileCollectTotal = 32 << 20                                  // bytes of one round
	MaxFileCollectFiles = 50                                        // files of one round, and skipped entries
	MaxFileCollectRound = 1000
	MaxCollectActivity  = 80
	MaxCollectPatterns  = 10
	MaxCollectPattern   = 20
	// MaxFileCollectDepth is how many folder levels below the activity
	// folder a collected path may have, the file included.
	MaxFileCollectDepth = 4
	// MaxFileCollectPath bounds the characters of a collected path.
	MaxFileCollectPath = MaxFileCollectDepth*101 - 1
	// FileCollectDateLayout is the layout of date (AAAA-MM-DD).
	FileCollectDateLayout = "2006-01-02"
)

// FileCollectStatus is what a file.collect.ack reports.
type FileCollectStatus string

// File collect ack statuses.
const (
	FileCollectReady   FileCollectStatus = "ready"   // answer to open
	FileCollectSending FileCollectStatus = "sending" // files follow
	FileCollectEmpty   FileCollectStatus = "empty"   // nothing to collect
	FileCollectFailed  FileCollectStatus = "failed"
)

// FileCollectReason says why a round failed, why a file was skipped or why
// the console refused a file. Each field accepts its own subset.
type FileCollectReason string

// File collect reasons (docs/protocol.md §5.21).
const (
	CollectInvalidPath FileCollectReason = "invalid_path"
	CollectBusy        FileCollectReason = "busy"
	CollectDenied      FileCollectReason = "denied"
	CollectUnsupported FileCollectReason = "unsupported"
	CollectInternal    FileCollectReason = "internal"
	CollectTooLarge    FileCollectReason = "too_large"
	CollectTooMany     FileCollectReason = "too_many"
	CollectLink        FileCollectReason = "link"
	CollectOffline     FileCollectReason = "offline"
	CollectIncomplete  FileCollectReason = "incomplete"
	CollectCorrupt     FileCollectReason = "corrupt"
	CollectNoSpace     FileCollectReason = "no_space"
)

// Valid reasons per field.
var (
	// FileCollectAckReasons are the reasons of a failed file.collect.ack.
	FileCollectAckReasons = []FileCollectReason{CollectInvalidPath, CollectBusy, CollectDenied, CollectUnsupported, CollectInternal}
	// FileCollectSkipReasons are the reasons of a skipped file.
	FileCollectSkipReasons = []FileCollectReason{CollectTooLarge, CollectTooMany, CollectLink, CollectDenied, CollectOffline, CollectInvalidPath}
	// FileCollectSaveReasons are the reasons of a failed file.collect.response.ack.
	FileCollectSaveReasons = []FileCollectReason{CollectIncomplete, CollectCorrupt, CollectNoSpace, CollectDenied, CollectInternal}
)

func reasonIn(r FileCollectReason, list []FileCollectReason) bool {
	for _, x := range list {
		if x == r {
			return true
		}
	}
	return false
}

// FileCollectOpen opens a collection: the agent creates its activity folder
// and tells the student where to save (P → A).
type FileCollectOpen struct {
	CollectID string   `json:"collect_id"`
	Date      string   `json:"date"` // AAAA-MM-DD
	Activity  string   `json:"activity"`
	Patterns  []string `json:"patterns,omitempty"`
}

// MessageType implements Payload.
func (FileCollectOpen) MessageType() Type { return TypeFileCollectOpen }

// Validate implements Payload. The path rules of activity are checked by
// collect, which answers invalid_path; Validate only enforces lengths.
func (p FileCollectOpen) Validate() error {
	return validateCollectTarget(TypeFileCollectOpen, p.CollectID, p.Date, p.Activity, p.Patterns)
}

func validateCollectTarget(t Type, id, date, activity string, patterns []string) error {
	if !isUUIDv4(id) {
		return invalid(t, "collect_id is not a UUID v4")
	}
	if _, err := time.Parse(FileCollectDateLayout, date); err != nil || len(date) != len(FileCollectDateLayout) {
		return invalid(t, "date must be AAAA-MM-DD")
	}
	if n := utf8.RuneCountInString(activity); n < 1 || n > MaxCollectActivity || !utf8.ValidString(activity) {
		return invalid(t, "activity must be 1-%d characters of valid UTF-8", MaxCollectActivity)
	}
	if len(patterns) > MaxCollectPatterns {
		return invalid(t, "at most %d patterns", MaxCollectPatterns)
	}
	for _, pat := range patterns {
		if n := utf8.RuneCountInString(pat); n < 1 || n > MaxCollectPattern || !utf8.ValidString(pat) {
			return invalid(t, "a pattern must be 1-%d characters", MaxCollectPattern)
		}
		if strings.ContainsAny(pat, `/\`) {
			return invalid(t, "a pattern names files, not folders")
		}
	}
	return nil
}

// FileCollectRequest asks the agent for the files of the activity folder
// (P → A).
type FileCollectRequest struct {
	CollectID   string   `json:"collect_id"`
	Date        string   `json:"date"`
	Activity    string   `json:"activity"`
	Round       int      `json:"round"` // 1..MaxFileCollectRound
	Patterns    []string `json:"patterns,omitempty"`
	MaxFileSize int      `json:"max_file_size"`
	MaxTotal    int      `json:"max_total"`
	MaxFiles    int      `json:"max_files"`
}

// MessageType implements Payload.
func (FileCollectRequest) MessageType() Type { return TypeFileCollectRequest }

// Validate implements Payload.
func (p FileCollectRequest) Validate() error {
	const t = TypeFileCollectRequest
	if err := validateCollectTarget(t, p.CollectID, p.Date, p.Activity, p.Patterns); err != nil {
		return err
	}
	if p.Round < 1 || p.Round > MaxFileCollectRound {
		return invalid(t, "round must be 1-%d", MaxFileCollectRound)
	}
	if p.MaxFileSize < 1 || p.MaxFileSize > MaxFileCollectBytes {
		return invalid(t, "max_file_size must be 1-%d", MaxFileCollectBytes)
	}
	if p.MaxTotal < 1 || p.MaxTotal > MaxFileCollectTotal {
		return invalid(t, "max_total must be 1-%d", MaxFileCollectTotal)
	}
	if p.MaxFiles < 1 || p.MaxFiles > MaxFileCollectFiles {
		return invalid(t, "max_files must be 1-%d", MaxFileCollectFiles)
	}
	return nil
}

// FileCollectSkipped is a file that matched but is not sent.
type FileCollectSkipped struct {
	Path   string            `json:"path"`
	Size   int64             `json:"size"`
	Reason FileCollectReason `json:"reason"`
}

// FileCollectAck answers an open or a request (A → P).
type FileCollectAck struct {
	RefID     string               `json:"ref_id"` // ID of the open or the request
	CollectID string               `json:"collect_id"`
	Round     int                  `json:"round"` // 0 for open
	Status    FileCollectStatus    `json:"status"`
	Path      string               `json:"path,omitempty"` // activity folder, relative to the work folder
	Files     int                  `json:"files,omitempty"`
	Total     int                  `json:"total,omitempty"`
	Skipped   []FileCollectSkipped `json:"skipped,omitempty"`
	Reason    FileCollectReason    `json:"reason,omitempty"`
}

// MessageType implements Payload.
func (FileCollectAck) MessageType() Type { return TypeFileCollectAck }

// Validate implements Payload.
func (p FileCollectAck) Validate() error {
	const t = TypeFileCollectAck
	if !isUUIDv4(p.RefID) {
		return invalid(t, "ref_id is not a UUID v4")
	}
	if !isUUIDv4(p.CollectID) {
		return invalid(t, "collect_id is not a UUID v4")
	}
	if p.Round < 0 || p.Round > MaxFileCollectRound {
		return invalid(t, "round must be 0-%d", MaxFileCollectRound)
	}
	if len(p.Skipped) > MaxFileCollectFiles {
		return invalid(t, "at most %d skipped files", MaxFileCollectFiles)
	}
	for _, s := range p.Skipped {
		if n := utf8.RuneCountInString(s.Path); n < 1 || n > MaxFileCollectPath || !utf8.ValidString(s.Path) {
			return invalid(t, "a skipped file needs its path")
		}
		if s.Size < 0 || !reasonIn(s.Reason, FileCollectSkipReasons) {
			return invalid(t, "skipped file with bad size or reason %q", s.Reason)
		}
	}
	hasPath := utf8.RuneCountInString(p.Path) >= 1 && utf8.RuneCountInString(p.Path) <= 300
	switch p.Status {
	case FileCollectReady:
		if p.Round != 0 || !hasPath || p.Files != 0 || p.Total != 0 || len(p.Skipped) != 0 || p.Reason != "" {
			return invalid(t, "ready answers an open: round 0 and path only")
		}
	case FileCollectSending:
		if p.Round < 1 || !hasPath || p.Reason != "" {
			return invalid(t, "sending needs round and path")
		}
		if p.Files < 1 || p.Files > MaxFileCollectFiles || p.Total < 0 || p.Total > MaxFileCollectTotal {
			return invalid(t, "files must be 1-%d and total 0-%d", MaxFileCollectFiles, MaxFileCollectTotal)
		}
	case FileCollectEmpty:
		if p.Round < 1 || !hasPath || p.Files != 0 || p.Total != 0 || p.Reason != "" {
			return invalid(t, "empty needs round and path, no files")
		}
	case FileCollectFailed:
		if p.Path != "" || p.Files != 0 || p.Total != 0 || len(p.Skipped) != 0 {
			return invalid(t, "a failed round has only its reason")
		}
		if !reasonIn(p.Reason, FileCollectAckReasons) {
			return invalid(t, "unknown reason %q", p.Reason)
		}
	default:
		return invalid(t, "status must be ready, sending, empty or failed")
	}
	return nil
}

// FileCollectResponse is one part of one collected file (A → P). Part 1
// also carries the file's path and how to check it.
type FileCollectResponse struct {
	CollectID string `json:"collect_id"`
	Round     int    `json:"round"`
	FileID    string `json:"file_id"`
	Part      int    `json:"part"`  // 1..Parts
	Parts     int    `json:"parts"` // 1..MaxFileCollectParts
	Data      []byte `json:"data"`  // Base64 on the wire

	// Part 1 only.
	Path       string `json:"path,omitempty"` // "/"-separated, relative to the activity folder
	Size       int    `json:"size,omitempty"`
	SHA256     string `json:"sha256,omitempty"`
	ModifiedAt int64  `json:"modified_at,omitempty"` // Unix ms
	Index      int    `json:"index,omitempty"`
	Files      int    `json:"files,omitempty"`
}

// MessageType implements Payload.
func (FileCollectResponse) MessageType() Type { return TypeFileCollectResponse }

// Validate implements Payload. An oversized part wraps ErrMessageTooLarge.
// The path rules are checked by collect.
func (p FileCollectResponse) Validate() error {
	const t = TypeFileCollectResponse
	if !isUUIDv4(p.CollectID) {
		return invalid(t, "collect_id is not a UUID v4")
	}
	if !isUUIDv4(p.FileID) {
		return invalid(t, "file_id is not a UUID v4")
	}
	if p.Round < 1 || p.Round > MaxFileCollectRound {
		return invalid(t, "round must be 1-%d", MaxFileCollectRound)
	}
	if p.Parts < 1 || p.Parts > MaxFileCollectParts || p.Part < 1 || p.Part > p.Parts {
		return invalid(t, "part and parts must satisfy 1 <= part <= parts <= %d", MaxFileCollectParts)
	}
	if len(p.Data) > FileCollectPartSize {
		return fmt.Errorf("%s: part is %d bytes, limit %d: %w", t, len(p.Data), FileCollectPartSize, ErrMessageTooLarge)
	}
	if p.Part != 1 {
		if len(p.Data) == 0 {
			return invalid(t, "data is empty")
		}
		return nil
	}
	if n := utf8.RuneCountInString(p.Path); n < 1 || n > MaxFileCollectPath || !utf8.ValidString(p.Path) {
		return invalid(t, "path must be 1-%d characters of valid UTF-8", MaxFileCollectPath)
	}
	if p.Size < 0 || p.Size > MaxFileCollectBytes {
		return invalid(t, "size must be 0-%d", MaxFileCollectBytes)
	}
	if want := max(1, (p.Size+FileCollectPartSize-1)/FileCollectPartSize); p.Parts != want {
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
	if p.ModifiedAt < 0 {
		return invalid(t, "modified_at must not be negative")
	}
	if p.Files < 1 || p.Files > MaxFileCollectFiles || p.Index < 1 || p.Index > p.Files {
		return invalid(t, "index and files must satisfy 1 <= index <= files <= %d", MaxFileCollectFiles)
	}
	return nil
}

// FileCollectResponseAck says whether the console saved one file (P → A).
type FileCollectResponseAck struct {
	RefID     string            `json:"ref_id"` // ID of the part that completed or rejected the file
	CollectID string            `json:"collect_id"`
	Round     int               `json:"round"`
	FileID    string            `json:"file_id"`
	Status    FilePushStatus    `json:"status"` // saved or failed
	Reason    FileCollectReason `json:"reason,omitempty"`
}

// MessageType implements Payload.
func (FileCollectResponseAck) MessageType() Type { return TypeFileCollectResponseAck }

// Validate implements Payload.
func (p FileCollectResponseAck) Validate() error {
	const t = TypeFileCollectResponseAck
	if !isUUIDv4(p.RefID) {
		return invalid(t, "ref_id is not a UUID v4")
	}
	if !isUUIDv4(p.CollectID) {
		return invalid(t, "collect_id is not a UUID v4")
	}
	if !isUUIDv4(p.FileID) {
		return invalid(t, "file_id is not a UUID v4")
	}
	if p.Round < 1 || p.Round > MaxFileCollectRound {
		return invalid(t, "round must be 1-%d", MaxFileCollectRound)
	}
	switch p.Status {
	case FilePushSaved:
		if p.Reason != "" {
			return invalid(t, "a saved file has no reason")
		}
	case FilePushFailed:
		if !reasonIn(p.Reason, FileCollectSaveReasons) {
			return invalid(t, "unknown reason %q", p.Reason)
		}
	default:
		return invalid(t, "status must be saved or failed")
	}
	return nil
}
