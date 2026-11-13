package roomplan

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
)

// Problems of the plan file, for the console's notice.
const (
	// ProblemDamaged: the file could not be read; a copy was kept next to it
	// (Backup) and the room starts empty.
	ProblemDamaged = "damaged"
	// ProblemNewer: the file was written by a newer Aula; it is left as it is
	// and the room starts empty in memory, never saved over it.
	ProblemNewer = "newer"
)

// FileName is the plan file of channel.
func FileName(channel string) string { return "canal-" + channel + ".json" }

// Room is the plan of the open channel, safe for concurrent use. Every change
// to the real computers is saved at once; the simulated students take seats
// only in memory and never reach the file.
type Room struct {
	mu      sync.Mutex
	path    string // "" keeps it in memory only
	plan    Plan
	sims    []Computer // the simulated students' seats and labels
	problem string
	backup  string
	now     func() time.Time
}

// ComputerView is one computer as the console draws it.
type ComputerView struct {
	Hostname  string `json:"hostname"`
	Label     string `json:"label"`
	Seat      *Cell  `json:"seat"`
	Simulated bool   `json:"simulated"`
}

// View is the plan as the console draws it: the size, the special places and
// every computer, the simulated ones marked.
type View struct {
	Channel   string         `json:"channel"`
	Rows      int            `json:"rows"`
	Cols      int            `json:"cols"`
	Places    []Place        `json:"places"`
	Computers []ComputerView `json:"computers"`
	// File is where the plan is saved ("" when Documents is unknown).
	File string `json:"file"`
	// Problem is ProblemDamaged or ProblemNewer when the file could not be
	// used; Backup, the copy of a damaged one.
	Problem string `json:"problem"`
	Backup  string `json:"backup"`
}

// Open reads the plan of channel from dir (Documents\Aula\salones); dir ""
// keeps it in memory. A missing file is an empty plan. A damaged file is
// copied aside and the room starts empty; a file of a newer version is not
// touched and the room is not saved. The Room is always usable; the error
// says what happened, for the log.
func Open(dir, channel string) (*Room, error) {
	r := &Room{plan: New(channel), now: time.Now}
	if dir == "" {
		return r, nil
	}
	r.path = filepath.Join(dir, FileName(channel))
	data, err := readLimited(r.path)
	if errors.Is(err, fs.ErrNotExist) {
		return r, nil
	}
	if err == nil {
		var p Plan
		p, err = Parse(data)
		if err == nil && p.Channel != channel {
			err = fmt.Errorf("%w: the file says channel %s", ErrInvalid, p.Channel)
		}
		if err == nil {
			r.plan = p
			return r, nil
		}
	}
	if errors.Is(err, ErrNewer) {
		r.problem, r.path = ProblemNewer, ""
		return r, fmt.Errorf("room plan %s: %w", FileName(channel), err)
	}
	r.problem = ProblemDamaged
	backup := strings.TrimSuffix(r.path, ".json") + "-danado-" + r.now().Format("20060102-150405") + ".json"
	if rerr := os.Rename(r.path, backup); rerr != nil {
		r.path = "" // never write over what could not be read nor moved aside
		return r, fmt.Errorf("room plan %s: %w (not moved aside: %v)", FileName(channel), err, rerr)
	}
	r.backup = backup
	return r, fmt.Errorf("room plan %s: %w", FileName(channel), err)
}

func readLimited(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > MaxFileSize {
		return nil, fmt.Errorf("%w: larger than %d KiB", ErrInvalid, MaxFileSize>>10)
	}
	return os.ReadFile(path)
}

// Parse reads a plan file and validates it.
func Parse(data []byte) (Plan, error) {
	if len(data) > MaxFileSize {
		return Plan{}, fmt.Errorf("%w: larger than %d KiB", ErrInvalid, MaxFileSize>>10)
	}
	var p Plan
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&p); err != nil {
		return Plan{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	p.normalize()
	if err := p.Validate(); err != nil {
		return Plan{}, err
	}
	return p, nil
}

// Encode writes a plan file.
func Encode(p Plan) ([]byte, error) {
	p = p.clone()
	p.normalize()
	if err := p.Validate(); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode room plan: %w", err)
	}
	return append(data, '\n'), nil
}

// View returns the plan for the console.
func (r *Room) View() View {
	r.mu.Lock()
	defer r.mu.Unlock()
	v := View{Channel: r.plan.Channel, Rows: r.plan.Rows, Cols: r.plan.Cols,
		Places: slices.Clone(r.plan.Places), Computers: []ComputerView{},
		File: r.path, Problem: r.problem, Backup: r.backup}
	all := make([]Computer, 0, len(r.plan.Computers)+len(r.sims))
	for _, c := range r.plan.Computers {
		all = append(all, c.clone())
	}
	n := len(all)
	for _, c := range r.sims {
		all = append(all, c.clone())
	}
	views := make([]ComputerView, len(all))
	for i := range all {
		views[i] = ComputerView{Hostname: all[i].Hostname, Label: all[i].Label, Seat: all[i].Seat, Simulated: i >= n}
	}
	sortViews(views)
	v.Computers = views
	return v
}

func sortViews(vs []ComputerView) {
	slices.SortStableFunc(vs, func(a, b ComputerView) int {
		switch {
		case a.Seat != nil && b.Seat != nil:
			if before(*a.Seat, *b.Seat) {
				return -1
			}
			if before(*b.Seat, *a.Seat) {
				return 1
			}
			return 0
		case a.Seat != nil:
			return -1
		case b.Seat != nil:
			return 1
		case a.Simulated != b.Simulated:
			if a.Simulated {
				return 1
			}
			return -1
		}
		return strings.Compare(key(a.Hostname), key(b.Hostname))
	})
}

// Plan returns a copy of the saved plan (the real computers only).
func (r *Room) Plan() Plan {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.plan.clone()
}

// Label is the label of a computer, "" if it has none or is unknown.
func (r *Room) Label(hostname string, simulated bool) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	cs := r.list(simulated)
	if i := index(*cs, hostname); i >= 0 {
		return (*cs)[i].Label
	}
	return ""
}

func (r *Room) list(simulated bool) *[]Computer {
	if simulated {
		return &r.sims
	}
	return &r.plan.Computers
}

// change applies f to a copy of the plan, validates it and keeps and saves
// it only if f and the validation succeed.
func (r *Room) change(f func(p *Plan) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p := r.plan.clone()
	if err := f(&p); err != nil {
		return err
	}
	p.normalize()
	if err := p.Validate(); err != nil {
		return err
	}
	if err := r.save(p); err != nil {
		return err
	}
	r.plan = p
	r.fitSims()
	return nil
}

// Resize changes the size of the room. Computers left outside go to «Sin
// acomodar» and special places outside are dropped.
func (r *Room) Resize(rows, cols int) error {
	if rows < 1 || rows > MaxSize || cols < 1 || cols > MaxSize {
		return invalidf("size %d×%d (1 to %d)", rows, cols, MaxSize)
	}
	return r.change(func(p *Plan) error {
		p.Rows, p.Cols = rows, cols
		p.Places = slices.DeleteFunc(p.Places, func(pl Place) bool { return !pl.Cell.in(rows, cols) })
		for i := range p.Computers {
			if s := p.Computers[i].Seat; s != nil && !s.in(rows, cols) {
				p.Computers[i].Seat = nil
			}
		}
		return nil
	})
}

// SetPlace marks c as a special place; kind "" leaves it empty. A computer
// seated there goes to «Sin acomodar».
func (r *Room) SetPlace(c Cell, kind Kind) error {
	if kind != "" && kind != KindTeacher && kind != KindAisle {
		return invalidf("kind %q", kind)
	}
	return r.change(func(p *Plan) error {
		if !c.in(p.Rows, p.Cols) {
			return invalidf("place %s is outside the room", c)
		}
		p.Places = slices.DeleteFunc(p.Places, func(pl Place) bool { return pl.Cell == c })
		if kind == "" {
			return nil
		}
		if i := seatedAt(p.Computers, c); i >= 0 {
			p.Computers[i].Seat = nil
		}
		p.Places = append(p.Places, Place{Cell: c, Kind: kind})
		return nil
	})
}

// Move seats a computer at to; to nil sends it to «Sin acomodar». A
// computer already at to swaps with it (to the mover's old seat, or to «Sin
// acomodar»). A special place cannot take a computer.
func (r *Room) Move(hostname string, simulated bool, to *Cell) error {
	if simulated {
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.moveIn(&r.plan, &r.sims, hostname, to, true)
	}
	return r.change(func(p *Plan) error { return r.moveIn(p, &p.Computers, hostname, to, false) })
}

// moveIn moves hostname inside list, swapping with a computer of either
// list at to (a real one only swaps with real ones; a simulated student
// gives way to a real computer by going to «Sin acomodar»).
func (r *Room) moveIn(p *Plan, list *[]Computer, hostname string, to *Cell, simulated bool) error {
	i := index(*list, hostname)
	if i < 0 {
		return fmt.Errorf("%w: %s", ErrUnknown, hostname)
	}
	if to == nil {
		(*list)[i].Seat = nil
		return nil
	}
	if !to.in(p.Rows, p.Cols) {
		return invalidf("seat %s is outside the room", to)
	}
	if _, special := p.placeAt(*to); special {
		return ErrSpecialPlace
	}
	from := (*list)[i].Seat
	if simulated {
		if j := seatedAt(p.Computers, *to); j >= 0 {
			return fmt.Errorf("%w: %s is taken by a real computer", ErrInvalid, to)
		}
	} else if j := seatedAt(r.sims, *to); j >= 0 {
		r.sims[j].Seat = nil // a real computer takes the seat back
	}
	if j := seatedAt(*list, *to); j >= 0 && j != i {
		(*list)[j].Seat = from
	}
	seat := *to
	(*list)[i].Seat = &seat
	return nil
}

// SetLabel changes the label of a computer. It reports whether it changed.
func (r *Room) SetLabel(hostname string, simulated bool, label string) (bool, error) {
	label = strings.TrimSpace(label)
	if err := validLabel(label); err != nil {
		return false, err
	}
	changed := false
	if simulated {
		r.mu.Lock()
		defer r.mu.Unlock()
		i := index(r.sims, hostname)
		if i < 0 {
			return false, fmt.Errorf("%w: %s", ErrUnknown, hostname)
		}
		changed = r.sims[i].Label != label
		r.sims[i].Label = label
		return changed, nil
	}
	err := r.change(func(p *Plan) error {
		i := index(p.Computers, hostname)
		if i < 0 {
			return fmt.Errorf("%w: %s", ErrUnknown, hostname)
		}
		changed = p.Computers[i].Label != label
		p.Computers[i].Label = label
		return nil
	})
	return changed, err
}

func validLabel(label string) error {
	if err := protocol.ValidComputerLabel(label); err != nil {
		return invalidf("label: %v", err)
	}
	return nil
}

// Forget removes a real computer from the plan.
func (r *Room) Forget(hostname string) error {
	return r.change(func(p *Plan) error {
		i := index(p.Computers, hostname)
		if i < 0 {
			return fmt.Errorf("%w: %s", ErrUnknown, hostname)
		}
		p.Computers = slices.Delete(p.Computers, i, i+1)
		return nil
	})
}

// Arrival says what Seen did with a hello.
type Arrival struct {
	// Added: a computer the plan did not know, now in «Sin acomodar».
	Added bool
	// RenamedFrom is the old hostname of a computer that came back with
	// another one and the same MAC: it kept its seat and its label.
	RenamedFrom string
	// Changed: the plan was saved (added, renamed or new MACs).
	Changed bool
}

// Seen records the hello of a real computer. An unknown hostname with the
// MAC of exactly one computer of the plan that is not connected is that
// computer renamed (it was reinstalled): it keeps its seat and its label.
// Otherwise an unknown computer goes to «Sin acomodar». A hello without
// MACs keeps the ones known.
func (r *Room) Seen(hostname string, macs []string, connected func(hostname string) bool) (Arrival, error) {
	if !ValidHostname(hostname) {
		return Arrival{}, invalidf("hostname %q", hostname)
	}
	macs = cleanMACs(macs)
	var a Arrival
	r.mu.Lock()
	full := len(r.plan.Computers) >= MaxComputers
	known := index(r.plan.Computers, hostname) >= 0
	r.mu.Unlock()
	if !known && full {
		return a, nil // the plan is full: it shows with its hostname
	}
	err := r.change(func(p *Plan) error {
		if i := index(p.Computers, hostname); i >= 0 {
			if len(macs) > 0 && !slices.Equal(p.Computers[i].MACs, macs) {
				p.Computers[i].MACs = macs
				a.Changed = true
			}
			return nil
		}
		if j := sameMachine(p.Computers, macs); j >= 0 && (connected == nil || !connected(p.Computers[j].Hostname)) {
			a.RenamedFrom = p.Computers[j].Hostname
			p.Computers[j].Hostname = hostname
			p.Computers[j].MACs = macs
			a.Changed = true
			return nil
		}
		p.Computers = append(p.Computers, Computer{Hostname: hostname, MACs: macs})
		a.Added, a.Changed = true, true
		return nil
	})
	if err != nil || !a.Changed {
		return Arrival{}, err
	}
	return a, nil
}

// Know adds a computer the console remembers (equipos-conocidos, prompt
// 10.19) to «Sin acomodar», so a room can be arranged while its computers
// are off. It never renames: a known hostname or MAC is left as it is.
func (r *Room) Know(hostname string, macs []string) (bool, error) {
	if !ValidHostname(hostname) {
		return false, nil
	}
	macs = cleanMACs(macs)
	r.mu.Lock()
	skip := len(r.plan.Computers) >= MaxComputers || index(r.plan.Computers, hostname) >= 0 ||
		sharesMAC(r.plan.Computers, macs)
	r.mu.Unlock()
	if skip {
		return false, nil
	}
	err := r.change(func(p *Plan) error {
		p.Computers = append(p.Computers, Computer{Hostname: hostname, MACs: macs})
		return nil
	})
	return err == nil, err
}

func cleanMACs(in []string) []string {
	var out []string
	for _, m := range in {
		m = strings.ToLower(m)
		if protocol.ValidMAC(m) && !slices.Contains(out, m) && len(out) < MaxMACs {
			out = append(out, m)
		}
	}
	return out
}

// sameMachine is the only computer of cs with one of macs, or -1 when none
// or more than one has it.
func sameMachine(cs []Computer, macs []string) int {
	found := -1
	for i, c := range cs {
		if slices.ContainsFunc(c.MACs, func(m string) bool { return slices.Contains(macs, m) }) {
			if found >= 0 {
				return -1
			}
			found = i
		}
	}
	return found
}

func sharesMAC(cs []Computer, macs []string) bool {
	for _, c := range cs {
		if slices.ContainsFunc(c.MACs, func(m string) bool { return slices.Contains(macs, m) }) {
			return true
		}
	}
	return false
}

// SyncSims makes the simulated students of the room the given names: a new
// one takes the first free seat (row by row; «Sin acomodar» if the room is
// full) and keeps it while it stays; one that left is forgotten. Never saved.
// It reports whether anything changed.
func (r *Room) SyncSims(names []string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	keep := map[string]bool{}
	for _, n := range names {
		keep[key(n)] = true
	}
	before := len(r.sims)
	r.sims = slices.DeleteFunc(r.sims, func(c Computer) bool { return !keep[key(c.Hostname)] })
	changed := len(r.sims) != before
	for _, n := range names {
		if index(r.sims, n) >= 0 {
			continue
		}
		r.sims = append(r.sims, Computer{Hostname: n, Seat: r.freeSeat()})
		changed = true
	}
	return changed
}

// freeSeat is the first cell with no place and no computer, or nil.
func (r *Room) freeSeat() *Cell {
	for row := 1; row <= r.plan.Rows; row++ {
		for col := 1; col <= r.plan.Cols; col++ {
			c := Cell{Row: row, Col: col}
			if _, special := r.plan.placeAt(c); special {
				continue
			}
			if seatedAt(r.plan.Computers, c) >= 0 || seatedAt(r.sims, c) >= 0 {
				continue
			}
			return &c
		}
	}
	return nil
}

// fitSims sends to «Sin acomodar» the simulated students left outside the
// room, on a special place or under a real computer after a change.
func (r *Room) fitSims() {
	for i := range r.sims {
		s := r.sims[i].Seat
		if s == nil {
			continue
		}
		_, special := r.plan.placeAt(*s)
		if !s.in(r.plan.Rows, r.plan.Cols) || special || seatedAt(r.plan.Computers, *s) >= 0 {
			r.sims[i].Seat = nil
		}
	}
}

// Export is the plan file to copy to another console.
func (r *Room) Export() ([]byte, error) {
	r.mu.Lock()
	p := r.plan.clone()
	r.mu.Unlock()
	return Encode(p)
}

// Import replaces the plan with a file exported by a console of the centre.
// A plan of another channel is refused with ErrOtherChannel unless
// otherChannel is true; then it takes this room's channel.
func (r *Room) Import(data []byte, otherChannel bool) error {
	p, err := Parse(data)
	if err != nil {
		return err
	}
	r.mu.Lock()
	channel := r.plan.Channel
	r.mu.Unlock()
	if p.Channel != channel {
		if !otherChannel {
			return fmt.Errorf("%w: %s", ErrOtherChannel, p.Channel)
		}
		p.Channel = channel
	}
	return r.change(func(cur *Plan) error {
		*cur = p
		return nil
	})
}

// save writes p atomically (temporary file and rename); nothing in memory
// mode.
func (r *Room) save(p Plan) error {
	if r.path == "" {
		return nil
	}
	data, err := Encode(p)
	if err != nil {
		return err
	}
	dir := filepath.Dir(r.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create room plan folder: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".salon-*.tmp")
	if err != nil {
		return fmt.Errorf("save room plan: %w", err)
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("save room plan: %w", err)
	}
	if err := os.Rename(tmp.Name(), r.path); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("save room plan: %w", err)
	}
	return nil
}
