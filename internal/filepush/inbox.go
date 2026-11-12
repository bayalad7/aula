package filepush

import (
	"slices"
	"strings"
	"sync"
	"time"
)

// MaxInbox bounds the pushes the window lists.
const MaxInbox = 20

// Received is one push as the window shows it: a card that fills as its files
// arrive. Go sends the state; the window words it (prompt 19).
type Received struct {
	PushID string `json:"pushId"`
	Label  string `json:"label"`
	Files  int    `json:"files"` // files the push announced
	// Folder is where the saved files are, relative to the work folder and
	// "/"-separated ("" = the work folder): what «Abrir carpeta» opens.
	Folder string    `json:"folder"`
	Items  []Item    `json:"items"` // in the order of the push
	At     time.Time `json:"at"`    // the last file
}

// Inbox keeps what arrived in this session of the agent, newest first. It
// is safe for concurrent use.
type Inbox struct {
	mu   sync.Mutex
	list []Received // oldest first
}

// Add records one file and returns the list for the window.
func (in *Inbox) Add(it Item) []Received {
	in.mu.Lock()
	defer in.mu.Unlock()
	i := slices.IndexFunc(in.list, func(r Received) bool { return r.PushID == it.PushID })
	if i < 0 {
		in.list = append(in.list, Received{PushID: it.PushID})
		if len(in.list) > MaxInbox {
			in.list = slices.Clone(in.list[len(in.list)-MaxInbox:])
		}
		i = len(in.list) - 1
	}
	r := &in.list[i]
	r.Label, r.Files, r.At = it.Label, it.Files, it.At
	r.Items = slices.DeleteFunc(r.Items, func(x Item) bool { return x.FileID == it.FileID })
	r.Items = append(r.Items, it)
	slices.SortStableFunc(r.Items, func(a, b Item) int { return a.Index - b.Index })
	r.Folder = commonFolder(r.Items)
	// The push that changed comes first.
	moved := *r
	in.list = append(slices.Delete(in.list, i, i+1), moved)
	return in.snapshot()
}

// List returns the pushes, newest first.
func (in *Inbox) List() []Received {
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.snapshot()
}

// Folder returns where the files of a push are, for «Abrir carpeta». The
// window passes only the push ID, never a path.
func (in *Inbox) Folder(pushID string) (string, bool) {
	in.mu.Lock()
	defer in.mu.Unlock()
	for _, r := range in.list {
		if r.PushID == pushID && slices.ContainsFunc(r.Items, Item.Saved) {
			return r.Folder, true
		}
	}
	return "", false
}

func (in *Inbox) snapshot() []Received {
	out := make([]Received, 0, len(in.list))
	for i := len(in.list) - 1; i >= 0; i-- {
		r := in.list[i]
		r.Items = slices.Clone(r.Items)
		out = append(out, r)
	}
	return out
}

// commonFolder is the deepest folder that holds every saved file.
func commonFolder(items []Item) string {
	var common []string
	first := true
	for _, it := range items {
		if !it.Saved() {
			continue
		}
		dir := strings.Split(it.Path, "/")
		dir = dir[:len(dir)-1]
		if first {
			common, first = dir, false
			continue
		}
		n := 0
		for n < len(common) && n < len(dir) && strings.EqualFold(common[n], dir[n]) {
			n++
		}
		common = common[:n]
	}
	return strings.Join(common, "/")
}

// Clear forgets every card (the class ended, docs/protocol.md §5.25) and
// returns how many there were. The files stay where they were saved.
func (in *Inbox) Clear() int {
	in.mu.Lock()
	defer in.mu.Unlock()
	n := len(in.list)
	in.list = nil
	return n
}
