package protocol

import (
	"errors"
	"fmt"
)

// Errors of Reassembly.Add.
var (
	ErrPartOutOfRange = errors.New("part out of range")
	ErrPartRepeated   = errors.New("part already received")
)

// SplitParts cuts data into consecutive pieces of size bytes (the last one
// may be shorter). It does not copy: the pieces share data's memory. It is
// the common chunking of messages that would exceed MaxMessageSize
// (screen.snapshot.response today, file.collect later; docs/protocol.md
// §5.17).
func SplitParts(data []byte, size int) [][]byte {
	if size < 1 || len(data) == 0 {
		return nil
	}
	out := make([][]byte, 0, (len(data)+size-1)/size)
	for len(data) > size {
		out = append(out, data[:size:size])
		data = data[size:]
	}
	return append(out, data)
}

// Reassembly puts the parts of one chunked message back together, in any
// order. It is not safe for concurrent use.
type Reassembly struct {
	parts [][]byte
	got   int
	size  int
}

// NewReassembly expects parts pieces, numbered 1..parts.
func NewReassembly(parts int) *Reassembly {
	return &Reassembly{parts: make([][]byte, max(parts, 0))}
}

// Add stores part number n (1-based). A part out of range wraps
// ErrPartOutOfRange; one already received wraps ErrPartRepeated.
func (r *Reassembly) Add(n int, data []byte) error {
	if n < 1 || n > len(r.parts) {
		return fmt.Errorf("reassembly: part %d of %d: %w", n, len(r.parts), ErrPartOutOfRange)
	}
	if r.parts[n-1] != nil {
		return fmt.Errorf("reassembly: part %d: %w", n, ErrPartRepeated)
	}
	r.parts[n-1] = append([]byte{}, data...)
	r.got++
	r.size += len(data)
	return nil
}

// Complete reports whether every part arrived.
func (r *Reassembly) Complete() bool { return len(r.parts) > 0 && r.got == len(r.parts) }

// Received is how many parts arrived, and their bytes.
func (r *Reassembly) Received() (parts, bytes int) { return r.got, r.size }

// Bytes joins the parts; nil until Complete.
func (r *Reassembly) Bytes() []byte {
	if !r.Complete() {
		return nil
	}
	out := make([]byte, 0, r.size)
	for _, p := range r.parts {
		out = append(out, p...)
	}
	return out
}
