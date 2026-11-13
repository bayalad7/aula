package protocol

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxComputerLabel bounds the label of a computer (docs/protocol.md §5.26).
const MaxComputerLabel = 40

// ComputerLabel tells the agent the label the teacher gave its computer in
// the classroom plan (P → A). An empty label means none. The agent keeps it
// only for the session and shows it in its capabilities dialog, without a
// notice.
type ComputerLabel struct {
	Label string `json:"label"`
}

// MessageType implements Payload.
func (ComputerLabel) MessageType() Type { return TypeComputerLabel }

// Validate implements Payload.
func (p ComputerLabel) Validate() error {
	if err := ValidComputerLabel(p.Label); err != nil {
		return invalid(TypeComputerLabel, "%v", err)
	}
	return nil
}

// ValidComputerLabel checks a label: valid UTF-8, at most MaxComputerLabel
// characters and no control characters. The empty label is valid.
func ValidComputerLabel(label string) error {
	switch {
	case !utf8.ValidString(label):
		return errLabel("label is not valid UTF-8")
	case utf8.RuneCountInString(label) > MaxComputerLabel:
		return errLabel("label exceeds 40 characters")
	case strings.ContainsFunc(label, unicode.IsControl):
		return errLabel("label has control characters")
	}
	return nil
}

type errLabel string

func (e errLabel) Error() string { return string(e) }
