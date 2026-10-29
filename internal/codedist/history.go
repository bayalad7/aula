package codedist

import (
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
)

// MaxHistory caps the snippets remembered in one class session.
const MaxHistory = 100

// HistoryEntry is a snippet the teacher emitted in this session. Its Number is
// sent as the exercise label ("Ejercicio 1", "2"…).
type HistoryEntry struct {
	SnippetID   string    `json:"snippetId"`
	Number      int       `json:"number"`
	Title       string    `json:"title"`
	Lang        string    `json:"lang"`
	Content     string    `json:"content"`
	Times       int       `json:"times"` // how many times it was emitted
	FirstSentAt time.Time `json:"firstSentAt"`
	LastSentAt  time.Time `json:"lastSentAt"`
}

// Share builds the code.share for this entry. Re-emitting keeps the same
// snippet_id, so agents update it in place instead of duplicating it.
func (e HistoryEntry) Share(mode protocol.CodeMode) protocol.CodeShare {
	return protocol.CodeShare{
		SnippetID: e.SnippetID,
		Exercise:  strconv.Itoa(e.Number),
		Title:     e.Title,
		Lang:      e.Lang,
		Content:   e.Content,
		Mode:      mode,
	}
}

// History is the numbered list of snippets of a class session. It is safe
// for concurrent use.
type History struct {
	mu      sync.Mutex
	next    int
	entries []HistoryEntry
}

// Add records a new snippet with the next number.
func (h *History) Add(title, lang, content string, now time.Time) HistoryEntry {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.next++
	e := HistoryEntry{
		SnippetID: protocol.NewID(), Number: h.next, Title: title, Lang: lang, Content: content,
		Times: 1, FirstSentAt: now, LastSentAt: now,
	}
	h.entries = append(h.entries, e)
	if len(h.entries) > MaxHistory {
		h.entries = slices.Clone(h.entries[len(h.entries)-MaxHistory:])
	}
	return e
}

// Touch marks an entry as emitted again and returns it.
func (h *History) Touch(snippetID string, now time.Time) (HistoryEntry, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i := range h.entries {
		if h.entries[i].SnippetID == snippetID {
			h.entries[i].Times++
			h.entries[i].LastSentAt = now
			return h.entries[i], true
		}
	}
	return HistoryEntry{}, false
}

// Entries returns the history, oldest first.
func (h *History) Entries() []HistoryEntry {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.entries)
}
