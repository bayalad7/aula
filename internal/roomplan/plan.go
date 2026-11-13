// Package roomplan is the classroom plan of the console (prompt 10.15): the
// label of each computer and where it sits in the room, so the console shows
// «Fila 2 · PC07» instead of LAB3-PC07 and lays the screens out as in the
// room. One plan per channel (a channel is a room), saved in the teacher's
// Documents\Aula\salones\canal-<n>.json and copied to the other consoles of
// the centre with Exportar and Importar.
//
// It keeps computers, never students: hostname, MACs, label and seat
// (docs/privacidad.md rule 31 ter). No OS call: the console gives the folder.
package roomplan

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/bayalad7/aula/internal/protocol"
)

// Limits of the plan.
const (
	// Version is the version of the plan file this console writes.
	Version = 1
	// MaxSize bounds the rows and the columns of the room.
	MaxSize = 12
	// MaxComputers bounds the computers of a plan, placed or not.
	MaxComputers = 512
	// MaxMACs bounds the MACs kept per computer, as in hello.macs.
	MaxMACs = 4
	// MaxFileSize is the largest plan file the console reads.
	MaxFileSize = 256 << 10
	// DefaultRows and DefaultCols are the size of a new plan (40 seats).
	DefaultRows = 5
	DefaultCols = 8
)

// Folder is the folder of the plans inside the teacher's Documents\Aula.
const Folder = "salones"

// Errors of the plan. Every validation error wraps ErrInvalid, except a
// file of a newer version (ErrNewer) and a plan of another channel
// (ErrOtherChannel, only when importing).
var (
	ErrInvalid      = errors.New("roomplan: invalid plan")
	ErrNewer        = errors.New("roomplan: plan written by a newer version")
	ErrOtherChannel = errors.New("roomplan: plan of another channel")
	ErrUnknown      = errors.New("roomplan: unknown computer")
	ErrSpecialPlace = errors.New("roomplan: that seat is a special place")
)

// Kind is a special place of the room: not a computer.
type Kind string

// Special places. An empty cell has no entry.
const (
	KindTeacher Kind = "teacher" // the teacher's desk
	KindAisle   Kind = "aisle"   // an aisle or a column of the room
)

// Cell is a seat of the room, counted from 1 like the rows and columns the
// teacher sees («Fila 2, columna 3»).
type Cell struct {
	Row int `json:"row"`
	Col int `json:"col"`
}

// Place is a special place at a cell.
type Place struct {
	Cell
	Kind Kind `json:"kind"`
}

// Computer is one computer of the room. Hostname is its key, without case;
// MACs recognise it when it comes back with another hostname. Seat nil means
// «Sin acomodar».
type Computer struct {
	Hostname string   `json:"hostname"`
	MACs     []string `json:"macs,omitempty"`
	Label    string   `json:"label"`
	Seat     *Cell    `json:"seat,omitempty"`
}

// Plan is the content of a plan file.
type Plan struct {
	Version   int        `json:"version"`
	Channel   string     `json:"channel"`
	Rows      int        `json:"rows"`
	Cols      int        `json:"cols"`
	Places    []Place    `json:"places"`
	Computers []Computer `json:"computers"`
}

// New returns the empty plan of channel.
func New(channel string) Plan {
	return Plan{Version: Version, Channel: channel, Rows: DefaultRows, Cols: DefaultCols,
		Places: []Place{}, Computers: []Computer{}}
}

func invalidf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}

// key is the hostname as the plan compares it.
func key(hostname string) string { return strings.ToLower(hostname) }

func (c Cell) in(rows, cols int) bool {
	return c.Row >= 1 && c.Row <= rows && c.Col >= 1 && c.Col <= cols
}

func (c Cell) String() string { return fmt.Sprintf("%d,%d", c.Row, c.Col) }

// ValidHostname checks a hostname as hello does: 1 to 255 characters, no
// control characters and not only spaces.
func ValidHostname(h string) bool {
	n := utf8.RuneCountInString(h)
	return n >= 1 && n <= 255 && utf8.ValidString(h) && strings.TrimSpace(h) != "" &&
		!strings.ContainsFunc(h, unicode.IsControl)
}

// Validate checks the whole plan: version, channel, size, places and
// computers (unique hostnames, valid labels and MACs, one computer per seat
// and none on a special place).
func (p Plan) Validate() error {
	switch {
	case p.Version > Version:
		return fmt.Errorf("%w: version %d", ErrNewer, p.Version)
	case p.Version != Version:
		return invalidf("version %d", p.Version)
	case !protocol.ValidChannel(p.Channel):
		return invalidf("channel %q", p.Channel)
	case p.Rows < 1 || p.Rows > MaxSize || p.Cols < 1 || p.Cols > MaxSize:
		return invalidf("size %d×%d (1 to %d)", p.Rows, p.Cols, MaxSize)
	case len(p.Computers) > MaxComputers:
		return invalidf("%d computers (at most %d)", len(p.Computers), MaxComputers)
	}
	taken := map[Cell]string{}
	for _, pl := range p.Places {
		if pl.Kind != KindTeacher && pl.Kind != KindAisle {
			return invalidf("place %s: kind %q", pl.Cell, pl.Kind)
		}
		if !pl.Cell.in(p.Rows, p.Cols) {
			return invalidf("place %s is outside the room", pl.Cell)
		}
		if _, dup := taken[pl.Cell]; dup {
			return invalidf("place %s is repeated", pl.Cell)
		}
		taken[pl.Cell] = string(pl.Kind)
	}
	hosts := map[string]bool{}
	for _, c := range p.Computers {
		if !ValidHostname(c.Hostname) {
			return invalidf("hostname %q", c.Hostname)
		}
		if hosts[key(c.Hostname)] {
			return invalidf("computer %q is repeated", c.Hostname)
		}
		hosts[key(c.Hostname)] = true
		if err := protocol.ValidComputerLabel(c.Label); err != nil {
			return invalidf("computer %q: %v", c.Hostname, err)
		}
		if len(c.MACs) > MaxMACs {
			return invalidf("computer %q: more than %d MACs", c.Hostname, MaxMACs)
		}
		for _, m := range c.MACs {
			if !protocol.ValidMAC(m) {
				return invalidf("computer %q: MAC %q", c.Hostname, m)
			}
		}
		if c.Seat == nil {
			continue
		}
		if !c.Seat.in(p.Rows, p.Cols) {
			return invalidf("computer %q: seat %s is outside the room", c.Hostname, c.Seat)
		}
		if other, dup := taken[*c.Seat]; dup {
			return invalidf("computer %q: seat %s is taken by %q", c.Hostname, c.Seat, other)
		}
		taken[*c.Seat] = c.Hostname
	}
	return nil
}

// normalize lowercases the MACs and orders the plan the way the file keeps
// it: places and seated computers by row and column, then «Sin acomodar» by
// hostname.
func (p *Plan) normalize() {
	if p.Places == nil {
		p.Places = []Place{}
	}
	if p.Computers == nil {
		p.Computers = []Computer{}
	}
	for i := range p.Computers {
		c := &p.Computers[i]
		var macs []string
		for _, m := range c.MACs {
			if m = strings.ToLower(m); !slices.Contains(macs, m) {
				macs = append(macs, m)
			}
		}
		c.MACs = macs
	}
	sort.Slice(p.Places, func(i, j int) bool { return before(p.Places[i].Cell, p.Places[j].Cell) })
	sortComputers(p.Computers)
}

func before(a, b Cell) bool {
	if a.Row != b.Row {
		return a.Row < b.Row
	}
	return a.Col < b.Col
}

func sortComputers(cs []Computer) {
	sort.SliceStable(cs, func(i, j int) bool {
		a, b := cs[i], cs[j]
		switch {
		case a.Seat != nil && b.Seat != nil:
			return before(*a.Seat, *b.Seat)
		case a.Seat != nil || b.Seat != nil:
			return a.Seat != nil
		}
		return key(a.Hostname) < key(b.Hostname)
	})
}

// clone copies the plan deeply.
func (p Plan) clone() Plan {
	out := p
	out.Places = slices.Clone(p.Places)
	out.Computers = make([]Computer, len(p.Computers))
	for i, c := range p.Computers {
		out.Computers[i] = c.clone()
	}
	return out
}

func (c Computer) clone() Computer {
	c.MACs = slices.Clone(c.MACs)
	if c.Seat != nil {
		s := *c.Seat
		c.Seat = &s
	}
	return c
}

// placeAt is the special place at c.
func (p Plan) placeAt(c Cell) (Kind, bool) {
	for _, pl := range p.Places {
		if pl.Cell == c {
			return pl.Kind, true
		}
	}
	return "", false
}

// index is the position of hostname among the computers, or -1.
func index(cs []Computer, hostname string) int {
	k := key(hostname)
	for i, c := range cs {
		if key(c.Hostname) == k {
			return i
		}
	}
	return -1
}

// seatedAt is the computer at c among cs, or -1.
func seatedAt(cs []Computer, c Cell) int {
	for i, x := range cs {
		if x.Seat != nil && *x.Seat == c {
			return i
		}
	}
	return -1
}
