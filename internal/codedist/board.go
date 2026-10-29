package codedist

import (
	"slices"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
)

// DefaultMaxSnippets caps how many snippets "append" keeps; the oldest are
// dropped. With up to 256 KiB each, this bounds the overlay's memory.
const DefaultMaxSnippets = 20

// Snippet is one block of code in the student's overlay.
type Snippet struct {
	ID         string    `json:"id"`
	Exercise   string    `json:"exercise"`
	Title      string    `json:"title"`
	Lang       string    `json:"lang"`
	Content    string    `json:"content"`
	ReceivedAt time.Time `json:"receivedAt"`
}

// Board is the list of snippets shown to the student. It is safe for
// concurrent use: the agent writes it, the UI reads it.
type Board struct {
	max      int
	mu       sync.Mutex
	snippets []Snippet
}

// NewBoard returns an empty board keeping at most max snippets (<= 0 means
// DefaultMaxSnippets).
func NewBoard(max int) *Board {
	if max <= 0 {
		max = DefaultMaxSnippets
	}
	return &Board{max: max}
}

// Apply adds a shared snippet: "replace" leaves it alone on the board,
// "append" adds it at the end. Re-sending a snippet with the same ID updates
// it in place instead of duplicating it. It returns the new board.
func (b *Board) Apply(p protocol.CodeShare, now time.Time) []Snippet {
	s := Snippet{ID: p.SnippetID, Exercise: p.Exercise, Title: p.Title, Lang: p.Lang, Content: p.Content, ReceivedAt: now}
	b.mu.Lock()
	defer b.mu.Unlock()
	switch {
	case p.Mode == protocol.ModeReplace:
		b.snippets = []Snippet{s}
	case slices.ContainsFunc(b.snippets, func(x Snippet) bool { return x.ID == s.ID }):
		i := slices.IndexFunc(b.snippets, func(x Snippet) bool { return x.ID == s.ID })
		b.snippets[i] = s
	default:
		b.snippets = append(b.snippets, s)
		if n := len(b.snippets); n > b.max {
			b.snippets = slices.Clone(b.snippets[n-b.max:])
		}
	}
	return slices.Clone(b.snippets)
}

// Clear empties the board.
func (b *Board) Clear() {
	b.mu.Lock()
	b.snippets = nil
	b.mu.Unlock()
}

// Snippets returns a copy of the board, oldest first.
func (b *Board) Snippets() []Snippet {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.snippets)
}

// Get returns one snippet by ID.
func (b *Board) Get(id string) (Snippet, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, s := range b.snippets {
		if s.ID == id {
			return s, true
		}
	}
	return Snippet{}, false
}
