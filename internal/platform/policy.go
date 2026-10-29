package platform

// The OS side of the class restrictions (docs/protocol.md §5.13–5.14, D-017):
// closing processes of the student's session, sending the computer's DNS to
// the agent's local resolver, and turning off the browsers' DNS over HTTPS.
// The lists, the modes and the journal that undoes everything live in
// internal/policy. Windows today, with the agent running as a service;
// elsewhere every constructor returns ErrNotSupported.

// ProcessInfo is one process of a session.
type ProcessInfo struct {
	PID  int
	Name string // executable file name, e.g. "chrome.exe"
	Path string // full path; "" if it cannot be read
}

// ProcessControl lists and closes processes.
type ProcessControl interface {
	// List returns the processes of the given session.
	List(session uint32) ([]ProcessInfo, error)
	// Kill ends the process; it is not an error if it already exited.
	Kill(pid int) error
}

// NewProcessControl returns the OS process control, or ErrNotSupported.
func NewProcessControl() (ProcessControl, error) { return newProcessControl() }

// DNSConfig is the DNS configuration of the computer's active adapters.
type DNSConfig struct {
	Servers  []string // e.g. "192.168.1.1", "2001:db8::1"; no loopback
	Suffixes []string // DNS suffixes, e.g. the school's domain
}

// DNSRedirect sends every DNS query of the computer to a local resolver
// with a rule of the system's resolver (Windows: an NRPT rule for "."),
// without touching the adapters.
type DNSRedirect interface {
	// Apply adds the rule that sends every query to server (an IP address,
	// "127.0.0.1"). The rule is tagged as Aula's.
	Apply(server string) error
	// Clear removes every rule tagged as Aula's, also those left by a crash;
	// it is not an error if there is none.
	Clear() error
	// Flush empties the system's DNS cache.
	Flush() error
	// Upstreams returns the adapters' DNS servers and suffixes, which the
	// local resolver forwards to.
	Upstreams() (DNSConfig, error)
}

// NewDNSRedirect returns the OS DNS redirect, or ErrNotSupported.
func NewDNSRedirect() (DNSRedirect, error) { return newDNSRedirect() }

// SavedSetting is the value a machine setting had before Aula changed it, so
// it can be given back exactly: also "did not exist", and the keys that had
// to be created for it.
type SavedSetting struct {
	Key     string   `json:"key"`
	Name    string   `json:"name"`
	Existed bool     `json:"existed"`
	Kind    string   `json:"kind,omitempty"` // "sz" or "dword"
	Str     string   `json:"str,omitempty"`
	Num     uint32   `json:"num,omitempty"`
	Created []string `json:"created,omitempty"` // keys that did not exist, parents first
}

// BrowserDoH turns off DNS over HTTPS in the browsers (machine policies of
// Chrome, Edge and Firefox) while the web filter is on, because a browser
// that resolves names over HTTPS would skip it.
type BrowserDoH interface {
	// Snapshot reads the current values, to store before Disable.
	Snapshot() ([]SavedSetting, error)
	// Disable sets the policies that turn DNS over HTTPS off.
	Disable() error
	// Restore puts back exactly the values of a snapshot, deleting what did
	// not exist (values and keys created for them).
	Restore([]SavedSetting) error
}

// NewBrowserDoH returns the OS browser policies, or ErrNotSupported.
func NewBrowserDoH() (BrowserDoH, error) { return newBrowserDoH() }
