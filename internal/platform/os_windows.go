//go:build windows

package platform

import (
	"os"

	"golang.org/x/sys/windows"
)

func restrictToOwner(path string) error {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	inherit := uint32(windows.NO_INHERITANCE)
	if info.IsDir() {
		inherit = windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{{
		AccessPermissions: windows.GENERIC_ALL,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       inherit,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_USER,
			TrusteeValue: windows.TrusteeValueFromSID(user.User.Sid),
		},
	}}, nil)
	if err != nil {
		return err
	}
	// PROTECTED drops the ACEs inherited from the parent (Users, Administrators…).
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, acl, nil)
}

func restrictMachineSecret(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	inherit := uint32(windows.NO_INHERITANCE)
	if info.IsDir() {
		inherit = windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT
	}
	var entries []windows.EXPLICIT_ACCESS
	for _, g := range []struct {
		sid    windows.WELL_KNOWN_SID_TYPE
		access windows.ACCESS_MASK
	}{
		{windows.WinBuiltinAdministratorsSid, windows.GENERIC_ALL},
		{windows.WinLocalSystemSid, windows.GENERIC_ALL},
		{windows.WinBuiltinUsersSid, windows.GENERIC_READ | windows.GENERIC_EXECUTE},
	} {
		sid, err := windows.CreateWellKnownSid(g.sid)
		if err != nil {
			return err
		}
		entries = append(entries, windows.EXPLICIT_ACCESS{
			AccessPermissions: g.access,
			AccessMode:        windows.GRANT_ACCESS,
			Inheritance:       inherit,
			Trustee: windows.TRUSTEE{
				TrusteeForm:  windows.TRUSTEE_IS_SID,
				TrusteeType:  windows.TRUSTEE_IS_WELL_KNOWN_GROUP,
				TrusteeValue: windows.TrusteeValueFromSID(sid),
			},
		})
	}
	acl, err := windows.ACLFromEntries(entries, nil)
	if err != nil {
		return err
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, acl, nil)
}

var procAttachConsole = windows.NewLazySystemDLL("kernel32.dll").NewProc("AttachConsole")

const attachParentProcess = ^uint32(0) // ATTACH_PARENT_PROCESS

func attachParentConsole() bool {
	// A console program, or output redirected to a file or pipe, already works.
	if h, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE); err == nil && h != 0 && h != windows.InvalidHandle {
		return true
	}
	if r, _, _ := procAttachConsole.Call(uintptr(attachParentProcess)); r == 0 {
		return false
	}
	out, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	os.Stdout, os.Stderr = out, out
	return true
}

// nativeNewlines converts text to CRLF, the line ending of Windows text on the
// clipboard (CF_UNICODETEXT); editors that expect it would otherwise join lines.
func nativeNewlines(s string) string { return toCRLF(s) }
