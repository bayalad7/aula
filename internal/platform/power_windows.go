//go:build windows

package platform

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// InitiateShutdownW flags and reason (winuser.h, reason.h).
const (
	shutdownForceOthers = 0x00000001
	shutdownForceSelf   = 0x00000002
	shutdownRestart     = 0x00000004
	shutdownPowerOff    = 0x00000008
	// SHTDN_REASON_MAJOR_OTHER | SHTDN_REASON_MINOR_OTHER | SHTDN_REASON_FLAG_PLANNED.
	shutdownReason = 0x80000000

	ewxLogoff = 0x00000000
	ewxForce  = 0x00000004
)

const shutdownMessage = "Aula: el profesor de la clase apagó o reinició este equipo."

var (
	procInitiateShutdownW = windows.NewLazySystemDLL("advapi32.dll").NewProc("InitiateShutdownW")
	procWTSLogoffSession  = windows.NewLazySystemDLL("wtsapi32.dll").NewProc("WTSLogoffSession")
)

func newPower() (Power, error) { return winPower{}, nil }

func currentSession() (uint32, error) {
	var id uint32
	if err := windows.ProcessIdToSessionId(windows.GetCurrentProcessId(), &id); err != nil {
		return 0, fmt.Errorf("session of this process: %w", err)
	}
	return id, nil
}

type winPower struct{}

func (winPower) Shutdown(force bool) error { return initiateShutdown(shutdownPowerOff, force) }
func (winPower) Reboot(force bool) error   { return initiateShutdown(shutdownRestart, force) }

// Logoff uses ExitWindowsEx for the own session without force (applications
// may ask to save) and WTSLogoffSession otherwise (SYSTEM for another session).
func (winPower) Logoff(session uint32, force bool) error {
	own, err := currentSession()
	if err != nil {
		return err
	}
	if !force {
		if session != own {
			return fmt.Errorf("log off session %d without force: only the own session (%d)", session, own)
		}
		if err := windows.ExitWindowsEx(ewxLogoff, shutdownReason); err != nil {
			return fmt.Errorf("log off: %w", err)
		}
		return nil
	}
	r, _, e := procWTSLogoffSession.Call(0 /* WTS_CURRENT_SERVER_HANDLE */, uintptr(session), 0 /* do not wait */)
	if r == 0 {
		return fmt.Errorf("log off session %d: %w", session, e)
	}
	return nil
}

// initiateShutdown turns the computer off or restarts it with no grace
// period: Aula's countdown already ran.
func initiateShutdown(flags uint32, force bool) error {
	if err := enableShutdownPrivilege(); err != nil {
		return err
	}
	if force {
		flags |= shutdownForceOthers | shutdownForceSelf
	}
	msg, err := windows.UTF16PtrFromString(shutdownMessage)
	if err != nil {
		return err
	}
	r, _, _ := procInitiateShutdownW.Call(0, uintptr(unsafe.Pointer(msg)), 0, uintptr(flags), shutdownReason)
	if r != 0 { // returns a Win32 error code, ERROR_SUCCESS on success
		return fmt.Errorf("shutdown: %w", windows.Errno(r))
	}
	return nil
}

// shutdownPrivilege looks up SeShutdownPrivilege.
func shutdownPrivilege() (windows.LUID, error) {
	var luid windows.LUID
	name, err := windows.UTF16PtrFromString("SeShutdownPrivilege")
	if err != nil {
		return luid, err
	}
	if err := windows.LookupPrivilegeValue(nil, name, &luid); err != nil {
		return luid, fmt.Errorf("SeShutdownPrivilege: %w", err)
	}
	return luid, nil
}

// enableShutdownPrivilege enables SeShutdownPrivilege in the process token:
// SYSTEM and, unless a GPO removes it, desktop users have it disabled.
func enableShutdownPrivilege() error {
	luid, err := shutdownPrivilege()
	if err != nil {
		return err
	}
	var tok windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &tok); err != nil {
		return fmt.Errorf("process token: %w", err)
	}
	defer tok.Close()
	tp := windows.Tokenprivileges{PrivilegeCount: 1}
	tp.Privileges[0] = windows.LUIDAndAttributes{Luid: luid, Attributes: windows.SE_PRIVILEGE_ENABLED}
	if err := windows.AdjustTokenPrivileges(tok, false, &tp, 0, nil, nil); err != nil {
		return fmt.Errorf("enable SeShutdownPrivilege: %w", err)
	}
	return nil
}
