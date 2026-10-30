//go:build windows

package platform

import "testing"

// These tests never call Shutdown, Reboot or Logoff: the countdown is tested
// with a double in internal/power.

func TestPowerIsSupported(t *testing.T) {
	if p, err := NewPower(); err != nil || p == nil {
		t.Fatalf("NewPower = %v, %v", p, err)
	}
}

func TestShutdownPrivilegeResolves(t *testing.T) {
	if _, err := shutdownPrivilege(); err != nil {
		t.Fatal(err)
	}
}

func TestCurrentSession(t *testing.T) {
	if _, err := CurrentSession(); err != nil {
		t.Fatal(err)
	}
}
