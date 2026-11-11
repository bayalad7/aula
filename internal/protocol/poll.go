package protocol

import (
	"strings"
	"unicode/utf8"
)

// Limits of the poll.* messages (docs/protocol.md §5.23, §7).
const (
	MaxPollQuestion = 200
	MaxPollOption   = 80
	MinPollOptions  = 2
	MaxPollOptions  = 6
	MaxPollText     = 140
	MaxPollVoters   = 10000
)

// PollKind is the kind of question of a poll.
type PollKind string

// Poll kinds.
const (
	PollKindChoice PollKind = "choice" // multiple choice, 2–6 options
	PollKindYesNo  PollKind = "yesno"  // yes (0) or no (1); the window writes the words
	PollKindOpen   PollKind = "open"   // short free text
)

// Options returns how many options a vote may choose from: len(options) for
// choice, 2 for yesno and 0 for open.
func (k PollKind) Options(options []string) int {
	switch k {
	case PollKindChoice:
		return len(options)
	case PollKindYesNo:
		return 2
	}
	return 0
}

// PollVoteStatus is the state a poll.vote.ack reports.
type PollVoteStatus string

// Vote statuses.
const (
	PollVoteAccepted PollVoteStatus = "accepted" // it counts, replacing the previous one
	PollVoteClosed   PollVoteStatus = "closed"   // the poll was already closed
	PollVoteUnknown  PollVoteStatus = "unknown"  // unknown poll, or not for this student
)

// PollAckStatus is the state a poll.ack reports.
type PollAckStatus string

// Poll ack statuses.
const (
	PollAckShown   PollAckStatus = "shown"   // answers poll.open
	PollAckClosed  PollAckStatus = "closed"  // answers poll.close
	PollAckResults PollAckStatus = "results" // answers poll.results
)

// PollOpen shows a question to the student (P → A). Named says whether the
// teacher sees each student's answer; the card says so before answering.
type PollOpen struct {
	PollID   string   `json:"poll_id"`
	Kind     PollKind `json:"kind"`
	Question string   `json:"question"`
	Options  []string `json:"options,omitempty"`
	Named    bool     `json:"named"`
}

// MessageType implements Payload.
func (PollOpen) MessageType() Type { return TypePollOpen }

// Validate implements Payload.
func (p PollOpen) Validate() error {
	if !isUUIDv4(p.PollID) {
		return invalid(TypePollOpen, "poll_id is not a UUID v4")
	}
	if n := utf8.RuneCountInString(strings.TrimSpace(p.Question)); n < 1 || utf8.RuneCountInString(p.Question) > MaxPollQuestion {
		return invalid(TypePollOpen, "question must be 1-%d characters", MaxPollQuestion)
	}
	switch p.Kind {
	case PollKindChoice:
		if len(p.Options) < MinPollOptions || len(p.Options) > MaxPollOptions {
			return invalid(TypePollOpen, "a choice poll has %d-%d options", MinPollOptions, MaxPollOptions)
		}
		for _, o := range p.Options {
			if strings.TrimSpace(o) == "" || utf8.RuneCountInString(o) > MaxPollOption {
				return invalid(TypePollOpen, "each option must be 1-%d characters", MaxPollOption)
			}
		}
	case PollKindYesNo, PollKindOpen:
		if len(p.Options) != 0 {
			return invalid(TypePollOpen, "only a choice poll has options")
		}
	default:
		return invalid(TypePollOpen, "kind must be choice, yesno or open")
	}
	return nil
}

// PollVote is the student's answer (A → P): Choice for choice and yesno polls,
// Text for open ones. Exactly one of them.
type PollVote struct {
	PollID string `json:"poll_id"`
	Choice *int   `json:"choice,omitempty"`
	Text   string `json:"text,omitempty"`
}

// MessageType implements Payload.
func (PollVote) MessageType() Type { return TypePollVote }

// Validate implements Payload.
func (p PollVote) Validate() error {
	if !isUUIDv4(p.PollID) {
		return invalid(TypePollVote, "poll_id is not a UUID v4")
	}
	if (p.Choice == nil) == (p.Text == "") {
		return invalid(TypePollVote, "a vote carries either choice or text")
	}
	if p.Choice != nil && (*p.Choice < 0 || *p.Choice >= MaxPollOptions) {
		return invalid(TypePollVote, "choice must be 0-%d", MaxPollOptions-1)
	}
	if p.Choice == nil {
		if n := utf8.RuneCountInString(p.Text); strings.TrimSpace(p.Text) == "" || n > MaxPollText {
			return invalid(TypePollVote, "text must be 1-%d characters", MaxPollText)
		}
	}
	return nil
}

// ValidFor checks the vote against the poll it answers: a choice within the
// options of a choice or yesno poll, or a text for an open one.
func (p PollVote) ValidFor(kind PollKind, options []string) error {
	if kind == PollKindOpen {
		if p.Choice != nil {
			return invalid(TypePollVote, "an open poll takes text")
		}
		return nil
	}
	if p.Choice == nil {
		return invalid(TypePollVote, "this poll takes a choice")
	}
	if *p.Choice >= kind.Options(options) {
		return invalid(TypePollVote, "choice is not one of the options")
	}
	return nil
}

// PollVoteAck tells the agent whether its answer counts (P → A).
type PollVoteAck struct {
	RefID  string         `json:"ref_id"`
	PollID string         `json:"poll_id"`
	Status PollVoteStatus `json:"status"`
}

// MessageType implements Payload.
func (PollVoteAck) MessageType() Type { return TypePollVoteAck }

// Validate implements Payload.
func (p PollVoteAck) Validate() error {
	if !isUUIDv4(p.RefID) {
		return invalid(TypePollVoteAck, "ref_id is not a UUID v4")
	}
	if !isUUIDv4(p.PollID) {
		return invalid(TypePollVoteAck, "poll_id is not a UUID v4")
	}
	switch p.Status {
	case PollVoteAccepted, PollVoteClosed, PollVoteUnknown:
	default:
		return invalid(TypePollVoteAck, "status must be accepted, closed or unknown")
	}
	return nil
}

// PollClose stops accepting answers (P → A).
type PollClose struct {
	PollID string `json:"poll_id"`
}

// MessageType implements Payload.
func (PollClose) MessageType() Type { return TypePollClose }

// Validate implements Payload.
func (p PollClose) Validate() error {
	if !isUUIDv4(p.PollID) {
		return invalid(TypePollClose, "poll_id is not a UUID v4")
	}
	return nil
}

// PollResults shares the totals of a closed poll (P → A). Never who chose
// what.
type PollResults struct {
	PollID     string `json:"poll_id"`
	Counts     []int  `json:"counts"`
	Voters     int    `json:"voters"`
	Recipients int    `json:"recipients"`
}

// MessageType implements Payload.
func (PollResults) MessageType() Type { return TypePollResults }

// Validate implements Payload.
func (p PollResults) Validate() error {
	if !isUUIDv4(p.PollID) {
		return invalid(TypePollResults, "poll_id is not a UUID v4")
	}
	if len(p.Counts) < MinPollOptions || len(p.Counts) > MaxPollOptions {
		return invalid(TypePollResults, "counts must have %d-%d entries", MinPollOptions, MaxPollOptions)
	}
	sum := 0
	for _, c := range p.Counts {
		if c < 0 || c > MaxPollVoters {
			return invalid(TypePollResults, "each count must be 0-%d", MaxPollVoters)
		}
		sum += c
	}
	if p.Voters < 0 || p.Voters > MaxPollVoters || sum != p.Voters {
		return invalid(TypePollResults, "voters must be the sum of counts, 0-%d", MaxPollVoters)
	}
	if p.Recipients < p.Voters || p.Recipients > MaxPollVoters {
		return invalid(TypePollResults, "recipients must be voters-%d", MaxPollVoters)
	}
	return nil
}

// PollAck reports what the window did with a poll message (A → P).
type PollAck struct {
	RefID  string        `json:"ref_id"`
	PollID string        `json:"poll_id"`
	Status PollAckStatus `json:"status"`
}

// MessageType implements Payload.
func (PollAck) MessageType() Type { return TypePollAck }

// Validate implements Payload.
func (p PollAck) Validate() error {
	if !isUUIDv4(p.RefID) {
		return invalid(TypePollAck, "ref_id is not a UUID v4")
	}
	if !isUUIDv4(p.PollID) {
		return invalid(TypePollAck, "poll_id is not a UUID v4")
	}
	switch p.Status {
	case PollAckShown, PollAckClosed, PollAckResults:
	default:
		return invalid(TypePollAck, "status must be shown, closed or results")
	}
	return nil
}
