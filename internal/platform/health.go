package platform

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"
)

// SystemHealth reads the health of the computer, never of the student
// (docs/protocol.md §5.27, docs/privacidad.md rule 92). Every method only
// reads: nothing here cleans, restarts or changes the computer. A datum the
// OS cannot give returns ErrNotSupported, and it does not travel. As a
// service, the service reads it itself (no helper), also without a session.
type SystemHealth interface {
	// SystemDrive is the root of the volume Windows lives on ("C:\").
	SystemDrive() (string, error)
	// DiskSpace gives the free and total bytes of the volume holding path.
	DiskSpace(path string) (DiskSpace, error)
	// DiskFailing reports the S.M.A.R.T. state of the system's physical
	// disk: true when the disk predicts a failure (SATA) or reports a
	// critical warning about its reliability (NVMe).
	DiskFailing() (bool, error)
	// Memory gives the physical memory.
	Memory() (Memory, error)
	// CPUTimes gives the processor's cumulative idle and total time; the
	// load is the difference between two readings.
	CPUTimes() (CPUTimes, error)
	// CPULimitPct is the frequency limit the system imposes now, in % of the
	// maximum (1–100). Below 100 on a running computer it is usually heat.
	CPULimitPct() (int, error)
	// Uptime is the time since the computer started.
	Uptime() (time.Duration, error)
	// RebootPending reports a reboot pending for updates.
	RebootPending() (bool, error)
	// Link describes the network card with the address local: kind, speed,
	// Wi-Fi signal and whether a cable is plugged into an unused card.
	Link(local net.IP) (Link, error)
	// Battery gives the battery; Present is false on a computer without one.
	Battery() (Battery, error)
	// Devices says whether the essential devices are present.
	Devices() (Devices, error)
}

// DiskSpace is the space of one volume.
type DiskSpace struct{ Free, Total int64 }

// Memory is the physical memory.
type Memory struct{ Total, Available int64 }

// CPUTimes are cumulative processor times in any unit.
type CPUTimes struct{ Idle, Total uint64 }

// LinkKind is the kind of a network card.
type LinkKind int

// Link kinds.
const (
	LinkOther LinkKind = iota
	LinkEthernet
	LinkWiFi
)

// Link is the card that reaches the console. Mbps and SignalPct are -1
// when they cannot be read. Never the network's name nor addresses.
type Link struct {
	Kind      LinkKind
	Mbps      int
	SignalPct int  // Wi-Fi only
	CableIdle bool // Wi-Fi only: a wired card is connected and unused
}

// Battery is the computer's battery. WearPct is -1 when it cannot be read.
type Battery struct {
	Present   bool
	ChargePct int
	OnAC      bool
	WearPct   int
}

// DeviceState is the state of an essential device.
type DeviceState int

// Device states.
const (
	DeviceUnknown     DeviceState = iota // could not be checked
	DeviceOK                             // present and working
	DeviceMissing                        // none present
	DeviceBasicDriver                    // display adapter without its driver
)

// Devices are the essential devices of the self-test.
type Devices struct {
	AudioOut, NIC, GPU, Keyboard, Mouse DeviceState
}

// NewSystemHealth returns the OS health reader. Where the OS has none,
// every method returns ErrNotSupported.
func NewSystemHealth() SystemHealth { return newSystemHealth() }

// DocumentsDirOf is the Documents folder of the user of a session, for the
// service (SYSTEM), which has none of its own.
func DocumentsDirOf(session uint32) (string, error) { return documentsDirOf(session) }

// Errors of DiskWriteTest, besides ErrNoSpace (the disk is full).
var (
	ErrReadOnly     = errors.New("platform: the disk is read-only")
	ErrDataMismatch = errors.New("platform: the data read back differs")
)

// DiskWriteTest writes size random bytes to a new temporary file in dir,
// reads them back, compares their SHA-256 and always deletes the file
// (docs/protocol.md §5.27.5). It never touches another file.
func DiskWriteTest(dir string, size int) (err error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return fmt.Errorf("disk test data: %w", err)
	}
	f, err := os.CreateTemp(dir, "aula-prueba-*.tmp")
	if err != nil {
		return fmt.Errorf("disk test create: %w", classifyWriteError(err))
	}
	name := f.Name()
	defer func() {
		if rmErr := os.Remove(name); rmErr != nil && err == nil {
			err = fmt.Errorf("disk test remove: %w", rmErr)
		}
	}()
	if _, err := f.Write(data); err != nil {
		f.Close()
		return fmt.Errorf("disk test write: %w", classifyWriteError(err))
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("disk test sync: %w", classifyWriteError(err))
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("disk test close: %w", classifyWriteError(err))
	}
	r, err := os.Open(name)
	if err != nil {
		return fmt.Errorf("disk test open: %w", err)
	}
	defer r.Close()
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return fmt.Errorf("disk test read: %w", err)
	}
	want := sha256.Sum256(data)
	if !bytes.Equal(h.Sum(nil), want[:]) {
		return ErrDataMismatch
	}
	return nil
}

// classifyWriteError maps an OS write error to ErrReadOnly or ErrNoSpace
// when it is one of them, keeping the original in the chain.
func classifyWriteError(err error) error {
	switch {
	case isNoSpace(err):
		return fmt.Errorf("%w: %w", ErrNoSpace, err)
	case isReadOnly(err):
		return fmt.Errorf("%w: %w", ErrReadOnly, err)
	}
	return err
}
