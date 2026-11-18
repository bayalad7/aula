//go:build linux

package platform

import (
	"os"
	"path/filepath"
	"testing"
)

// fakeTree points healthRoot at a temporary /proc and /sys with files.
func fakeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for p, data := range files {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	old := healthRoot
	healthRoot = root
	t.Cleanup(func() { healthRoot = old })
	return root
}

func TestHealthLinuxFromProc(t *testing.T) {
	fakeTree(t, map[string]string{
		"/proc/meminfo": "MemTotal:       16000000 kB\nMemFree:  100 kB\nMemAvailable:    4000000 kB\n",
		"/proc/stat":    "cpu  100 5 50 800 20 1 2 0 0 0\ncpu0 1 1 1 1\n",
		"/proc/uptime":  "3600.50 7000.00\n",
		"/sys/devices/system/cpu/cpu0/cpufreq/scaling_max_freq": "1800000\n",
		"/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq": "3600000\n",
		"/run/reboot-required": "",
	})
	h := linuxHealth{}
	if m, err := h.Memory(); err != nil || m.Total != 16000000*1024 || m.Available != 4000000*1024 {
		t.Errorf("Memory = %+v, %v", m, err)
	}
	if c, err := h.CPUTimes(); err != nil || c.Total != 978 || c.Idle != 820 {
		t.Errorf("CPUTimes = %+v, %v", c, err)
	}
	if u, err := h.Uptime(); err != nil || u.Seconds() != 3600.5 {
		t.Errorf("Uptime = %v, %v", u, err)
	}
	if p, err := h.CPULimitPct(); err != nil || p != 50 {
		t.Errorf("CPULimitPct = %d, %v", p, err)
	}
	if r, err := h.RebootPending(); err != nil || !r {
		t.Errorf("RebootPending = %v, %v", r, err)
	}
	if _, err := h.DiskFailing(); err == nil {
		t.Error("DiskFailing must say it cannot read S.M.A.R.T.")
	}
	if d, err := h.DiskSpace(t.TempDir()); err != nil || d.Total <= 0 || d.Free < 0 || d.Free > d.Total {
		t.Errorf("DiskSpace = %+v, %v", d, err)
	}
}

func TestHealthLinuxBatteryAndLink(t *testing.T) {
	root := fakeTree(t, map[string]string{
		"/sys/class/power_supply/AC/type":                  "Mains\n",
		"/sys/class/power_supply/AC/online":                "1\n",
		"/sys/class/power_supply/BAT0/type":                "Battery\n",
		"/sys/class/power_supply/BAT0/capacity":            "73\n",
		"/sys/class/power_supply/BAT0/energy_full":         "40000000\n",
		"/sys/class/power_supply/BAT0/energy_full_design":  "50000000\n",
		"/sys/class/power_supply/hidpp_battery_0/type":     "Battery\n",
		"/sys/class/power_supply/hidpp_battery_0/scope":    "Device\n",
		"/sys/class/power_supply/hidpp_battery_0/capacity": "5\n",
		"/sys/class/net/wlp2s0/type":                       "1\n",
		"/sys/class/net/wlp2s0/wireless/x":                 "",
		"/sys/class/net/wlp2s0/device/x":                   "",
		"/sys/class/net/enp3s0/type":                       "1\n",
		"/sys/class/net/enp3s0/device/x":                   "",
		"/sys/class/net/enp3s0/carrier":                    "1\n",
		"/sys/class/net/enp3s0/speed":                      "1000\n",
		"/sys/class/net/docker0/type":                      "1\n",
		"/proc/net/wireless": "Inter-| sta-|   Quality        |\n face | tus | link level noise |\n" +
			"wlp2s0: 0000   56.  -54.  -256        0      0      0      0      0        0\n",
		"/proc/asound/cards": " 0 [PCH            ]: HDA-Intel\n",
		"/proc/bus/input/devices": "I: Bus=0011\nN: Name=\"AT Translated Set 2 keyboard\"\nH: Handlers=sysrq kbd event3 leds\nB: EV=120013\n\n" +
			"I: Bus=0003\nN: Name=\"Logitech Mouse\"\nH: Handlers=mouse0 event5\nB: EV=17\n",
	})
	h := linuxHealth{}
	b, err := h.Battery()
	if err != nil || !b.Present || b.ChargePct != 73 || !b.OnAC || b.WearPct != 20 {
		t.Errorf("Battery = %+v, %v (the mouse's battery must not count)", b, err)
	}
	wifi := linkOf("wlp2s0", []string{"wlp2s0", "enp3s0", "docker0"})
	if wifi.Kind != LinkWiFi || wifi.SignalPct != 80 || !wifi.CableIdle {
		t.Errorf("Wi-Fi link = %+v", wifi)
	}
	cable := linkOf("enp3s0", nil)
	if cable.Kind != LinkEthernet || cable.Mbps != 1000 || cable.SignalPct != -1 {
		t.Errorf("cable link = %+v", cable)
	}
	if k := netKind("docker0"); k != LinkOther {
		t.Errorf("a bridge is %v, want LinkOther", k)
	}
	// The GPU's driver: a generic framebuffer is "without its driver".
	if err := os.MkdirAll(filepath.Join(root, "/sys/class/drm/card0/device"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../bus/platform/drivers/simple-framebuffer", filepath.Join(root, "/sys/class/drm/card0/device/driver")); err != nil {
		t.Fatal(err)
	}
	d, err := h.Devices()
	if err != nil || d.AudioOut != DeviceOK || d.NIC != DeviceOK || d.Keyboard != DeviceOK || d.Mouse != DeviceOK || d.GPU != DeviceBasicDriver {
		t.Errorf("Devices = %+v, %v", d, err)
	}
}

func TestHealthLinuxNoBattery(t *testing.T) {
	fakeTree(t, map[string]string{"/sys/class/power_supply/AC/type": "Mains\n", "/sys/class/power_supply/AC/online": "1\n"})
	if b, err := (linuxHealth{}).Battery(); err != nil || b.Present || b.OnAC {
		t.Errorf("Battery of a desktop = %+v, %v", b, err)
	}
}
