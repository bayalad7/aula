package platform

import (
	"errors"
	"os"
	"testing"
)

func TestDiskWriteTestLeavesNothing(t *testing.T) {
	dir := t.TempDir()
	if err := DiskWriteTest(dir, 1<<16); err != nil {
		t.Fatalf("DiskWriteTest: %v", err)
	}
	if ents, _ := os.ReadDir(dir); len(ents) != 0 {
		t.Fatalf("the test left %d files", len(ents))
	}
}

func TestDiskWriteTestMissingDir(t *testing.T) {
	if err := DiskWriteTest(t.TempDir()+"/no-existe", 16); err == nil {
		t.Fatal("writing to a missing folder must fail")
	}
}

// TestSystemHealthReadsSanely only reads this computer (nothing changes):
// each datum is either not supported or within its range.
func TestSystemHealthReadsSanely(t *testing.T) {
	h := NewSystemHealth()
	ok := func(err error) bool {
		if err != nil && !errors.Is(err, ErrNotSupported) {
			t.Logf("unreadable here: %v", err)
		}
		return err == nil
	}
	if root, err := h.SystemDrive(); ok(err) {
		if d, err := h.DiskSpace(root); ok(err) && (d.Total <= 0 || d.Free < 0 || d.Free > d.Total) {
			t.Errorf("disk space %+v", d)
		}
	}
	if m, err := h.Memory(); ok(err) && (m.Total <= 0 || m.Available > m.Total) {
		t.Errorf("memory %+v", m)
	}
	if c, err := h.CPUTimes(); ok(err) && c.Idle > c.Total {
		t.Errorf("cpu times %+v", c)
	}
	if p, err := h.CPULimitPct(); ok(err) && (p < 1 || p > 100) {
		t.Errorf("cpu limit %d", p)
	}
	if u, err := h.Uptime(); ok(err) && u <= 0 {
		t.Errorf("uptime %v", u)
	}
	_, err := h.RebootPending()
	ok(err)
	_, err = h.DiskFailing()
	ok(err)
	if b, err := h.Battery(); ok(err) && b.Present && (b.ChargePct < 0 || b.ChargePct > 100 || b.WearPct > 100) {
		t.Errorf("battery %+v", b)
	}
	if _, err := h.Devices(); ok(err) {
		t.Log("devices read")
	}
}
