//go:build !windows && !linux

package platform

import (
	"errors"
	"net"
	"syscall"
	"time"
)

// The health of the computer is only read on Windows for now: elsewhere
// every datum is "not supported" and does not travel (docs/protocol.md
// §5.27.3).

type noHealth struct{}

func newSystemHealth() SystemHealth { return noHealth{} }

func (noHealth) SystemDrive() (string, error)        { return "", ErrNotSupported }
func (noHealth) DiskSpace(string) (DiskSpace, error) { return DiskSpace{}, ErrNotSupported }
func (noHealth) DiskFailing() (bool, error)          { return false, ErrNotSupported }
func (noHealth) Memory() (Memory, error)             { return Memory{}, ErrNotSupported }
func (noHealth) CPUTimes() (CPUTimes, error)         { return CPUTimes{}, ErrNotSupported }
func (noHealth) CPULimitPct() (int, error)           { return 0, ErrNotSupported }
func (noHealth) Uptime() (time.Duration, error)      { return 0, ErrNotSupported }
func (noHealth) RebootPending() (bool, error)        { return false, ErrNotSupported }
func (noHealth) Link(net.IP) (Link, error)           { return Link{}, ErrNotSupported }
func (noHealth) Battery() (Battery, error)           { return Battery{}, ErrNotSupported }
func (noHealth) Devices() (Devices, error)           { return Devices{}, ErrNotSupported }
func documentsDirOf(uint32) (string, error)          { return "", ErrNotSupported }
func isNoSpace(err error) bool                       { return errors.Is(err, syscall.ENOSPC) }
func isReadOnly(err error) bool                      { return errors.Is(err, syscall.EROFS) }
