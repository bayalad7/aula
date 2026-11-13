package power

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bayalad7/aula/internal/protocol"
)

// MaxKnownMachines bounds the computers remembered per channel; the ones not
// seen for the longest are dropped first.
const MaxKnownMachines = 512

// Machine is a computer the console has seen in a channel, remembered so it
// can be woken while it is off (docs/protocol.md §5.15.2). The MAC identifies
// the computer, not the student (docs/privacidad.md rule 31 bis): only the
// hostname, the MACs, the last address and when it was seen are kept.
type Machine struct {
	Hostname string    `json:"hostname"`
	MACs     []string  `json:"macs"`
	IP       string    `json:"ip"`
	LastSeen time.Time `json:"last_seen"`
}

// Machines is the console's record of known computers, by channel and by
// hostname (case-insensitive), saved in a JSON file.
type Machines struct {
	path string
	now  func() time.Time

	mu       sync.Mutex
	channels map[string]map[string]Machine
}

type machinesFile struct {
	Version  int                  `json:"version"`
	Channels map[string][]Machine `json:"channels"`
}

// LoadMachines reads the record at path; a missing file is an empty record.
// path "" keeps it in memory only. On a read or parse error the record starts
// empty and the error says why.
func LoadMachines(path string) (*Machines, error) {
	m := &Machines{path: path, now: time.Now, channels: map[string]map[string]Machine{}}
	if path == "" {
		return m, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return m, fmt.Errorf("read known machines: %w", err)
	}
	var f machinesFile
	if err := json.Unmarshal(data, &f); err != nil {
		return m, fmt.Errorf("parse known machines %s: %w", path, err)
	}
	for ch, list := range f.Channels {
		if !protocol.ValidChannel(ch) {
			continue
		}
		for _, mc := range list {
			mc.MACs = validMACs(mc.MACs)
			if mc.Hostname == "" || len(mc.MACs) == 0 {
				continue
			}
			m.put(ch, mc)
		}
	}
	return m, nil
}

func validMACs(in []string) []string {
	var out []string
	for _, s := range in {
		s = strings.ToLower(s)
		if protocol.ValidMAC(s) && !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}

func (m *Machines) put(channel string, mc Machine) {
	byHost := m.channels[channel]
	if byHost == nil {
		byHost = map[string]Machine{}
		m.channels[channel] = byHost
	}
	byHost[strings.ToLower(mc.Hostname)] = mc
	for len(byHost) > MaxKnownMachines {
		oldest := ""
		for k, v := range byHost {
			if oldest == "" || v.LastSeen.Before(byHost[oldest].LastSeen) {
				oldest = k
			}
		}
		delete(byHost, oldest)
	}
}

// Seen records a hello of hostname in channel from ip. A hello without MACs
// keeps the ones already known (an agent that could not read them), and a
// computer never seen with a MAC is not recorded: it could not be woken.
func (m *Machines) Seen(channel, hostname string, macs []string, ip string) error {
	macs = validMACs(macs)
	m.mu.Lock()
	old, known := m.channels[channel][strings.ToLower(hostname)]
	if !known && len(macs) == 0 {
		m.mu.Unlock()
		return nil
	}
	mc := Machine{Hostname: hostname, MACs: macs, IP: ip, LastSeen: m.now().UTC()}
	if len(macs) == 0 {
		mc.MACs = old.MACs
	}
	m.put(channel, mc)
	m.mu.Unlock()
	return m.save()
}

// Get returns the known computer hostname of channel.
func (m *Machines) Get(channel, hostname string) (Machine, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	mc, ok := m.channels[channel][strings.ToLower(hostname)]
	if ok {
		mc.MACs = slices.Clone(mc.MACs)
	}
	return mc, ok
}

// List returns the known computers of channel by hostname.
func (m *Machines) List(channel string) []Machine {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Machine, 0, len(m.channels[channel]))
	for _, mc := range m.channels[channel] {
		mc.MACs = slices.Clone(mc.MACs)
		out = append(out, mc)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Hostname) < strings.ToLower(out[j].Hostname) })
	return out
}

// Forget removes hostname from channel («Olvidar equipo»). It reports whether
// it was known.
func (m *Machines) Forget(channel, hostname string) (bool, error) {
	m.mu.Lock()
	_, ok := m.channels[channel][strings.ToLower(hostname)]
	delete(m.channels[channel], strings.ToLower(hostname))
	m.mu.Unlock()
	if !ok {
		return false, nil
	}
	return true, m.save()
}

// save writes the record atomically (temporary file and rename).
func (m *Machines) save() error {
	if m.path == "" {
		return nil
	}
	m.mu.Lock()
	f := machinesFile{Version: 1, Channels: map[string][]Machine{}}
	for ch, byHost := range m.channels {
		list := make([]Machine, 0, len(byHost))
		for _, mc := range byHost {
			list = append(list, mc)
		}
		sort.Slice(list, func(i, j int) bool { return list[i].Hostname < list[j].Hostname })
		f.Channels[ch] = list
	}
	m.mu.Unlock()
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("encode known machines: %w", err)
	}
	dir := filepath.Dir(m.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create known machines folder: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".equipos-*.tmp")
	if err != nil {
		return fmt.Errorf("save known machines: %w", err)
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("save known machines: %w", err)
	}
	if err := os.Rename(tmp.Name(), m.path); err != nil {
		os.Remove(tmp.Name())
		return fmt.Errorf("save known machines: %w", err)
	}
	return nil
}
