//go:build windows

package platform

import (
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestRestrictToOwnerLeavesOnlyCurrentUser(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret.pem")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RestrictToOwner(path); err != nil {
		t.Fatal(err)
	}

	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	control, _, err := sd.Control()
	if err != nil {
		t.Fatal(err)
	}
	if control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatal("DACL still inherits from the parent directory")
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if dacl.AceCount != 1 {
		t.Fatalf("DACL has %d entries, want 1", dacl.AceCount)
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if err := windows.GetAce(dacl, 0, &ace); err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
	if !sid.Equals(user.User.Sid) {
		t.Fatalf("entry is for %s, want the current user %s", sid, user.User.Sid)
	}
	if b, err := os.ReadFile(path); err != nil || string(b) != "x" {
		t.Fatalf("owner can no longer read the file: %v", err)
	}
}

func TestRestrictMachineSecret(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clave-centro.pem")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := RestrictMachineSecret(path); err != nil {
		t.Fatal(err)
	}
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	control, _, _ := sd.Control()
	dacl, _, err := sd.DACL()
	if err != nil || control&windows.SE_DACL_PROTECTED == 0 || dacl.AceCount != 3 {
		t.Fatalf("protected=%v aces=%d err=%v", control&windows.SE_DACL_PROTECTED != 0, dacl.AceCount, err)
	}
	users, _ := windows.CreateWellKnownSid(windows.WinBuiltinUsersSid)
	for i := range uint32(dacl.AceCount) {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			t.Fatal(err)
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if sid.Equals(users) && ace.Mask&(windows.FILE_WRITE_DATA|windows.DELETE|windows.WRITE_DAC) != 0 {
			t.Fatalf("Users can modify the secret: mask %#x", ace.Mask)
		}
	}
}
