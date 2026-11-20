//go:build linux

package main

import (
	"os"
	"os/exec"
	"slices"
	"testing"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

// TestLinuxDeclaresOnlyWhatItDoes: in Linux the agent declares nothing it
// does not do (docs/privacidad.md rules 96-103, docs/compatibilidad.md). The
// screen, the remote control and the pause only in an X11 session; power
// with systemd; pages with xdg-open and apps with gio; never showing the
// screen nor restrictions.
func TestLinuxDeclaresOnlyWhatItDoes(t *testing.T) {
	for _, tc := range []struct {
		name, session, wayland, display string
		screen                          bool
	}{
		{"wayland", "wayland", "wayland-0", ":0", false},
		{"no session", "", "", "", false},
		{"x11", "x11", "", "", true}, // DISPLAY is filled in below when there is one
	} {
		t.Run(tc.name, func(t *testing.T) {
			display := tc.display
			if tc.session == "x11" {
				display = os.Getenv("DISPLAY")
				if platform.DisplayServer() != platform.DisplayX11 {
					t.Skip("no X11 session (run under Xvfb)")
				}
			}
			t.Setenv("XDG_SESSION_TYPE", tc.session)
			t.Setenv("WAYLAND_DISPLAY", tc.wayland)
			t.Setenv("DISPLAY", display)

			a := NewApp(Config{Channel: "1"}, nil, nil, localHelloWithHealth(), nil, nil, quiet)
			enablePlatform(a, quiet)
			caps := a.hello.Capabilities

			// Presenting needs X11 and an H.264 encoder (openh264, rule 106).
			if got, want := slices.Contains(caps, protocol.CapScreenPresent), tc.screen && platform.H264Available(); got != want {
				t.Errorf("screen.present declared = %v, want %v (X11 and openh264)", got, want)
			}
			// Restrictions only as a service (D-025), never in window mode.
			for _, c := range []protocol.Capability{protocol.CapAppBlock, protocol.CapWebFilter} {
				if slices.Contains(caps, c) {
					t.Errorf("declares %s, which Linux does not do yet", c)
				}
			}
			for _, c := range []protocol.Capability{protocol.CapCodeView, protocol.CapMsgShow,
				protocol.CapHand, protocol.CapMsgStudent, protocol.CapTimer, protocol.CapPoll,
				protocol.CapPollQuiz, protocol.CapClassEnd, protocol.CapComputerLabel,
				protocol.CapFilePush, protocol.CapFileCollect, protocol.CapHealth, protocol.CapHealthSelfTest} {
				if !slices.Contains(caps, c) {
					t.Errorf("does not declare %s, which works on Linux", c)
				}
			}
			// The screen, the remote control and the pause need X11 (rules 96, 97 and 101).
			for _, c := range []protocol.Capability{protocol.CapScreenThumb, protocol.CapScreenSnapshot,
				protocol.CapRemote, protocol.CapScreenFreeze} {
				if slices.Contains(caps, c) != tc.screen {
					t.Errorf("%s declared = %v, want %v (only in X11, with its notice)", c, !tc.screen, tc.screen)
				}
			}
			// Pages open with xdg-open: declared exactly when it is there.
			_, xerr := exec.LookPath("xdg-open")
			if got := slices.Contains(caps, protocol.CapOpenURL); got != (xerr == nil) {
				t.Errorf("open.url declared = %v, xdg-open present = %v", got, xerr == nil)
			}
			// Apps open by their .desktop with gio (rule 104).
			_, gerr := exec.LookPath("gio")
			if got := slices.Contains(caps, protocol.CapOpenApp); got != (xerr == nil && gerr == nil) {
				t.Errorf("open.app declared = %v, xdg-open and gio present = %v", got, xerr == nil && gerr == nil)
			}
			// Power goes through systemd: declared exactly when it is there.
			_, err := exec.LookPath("systemctl")
			if got := slices.Contains(caps, protocol.CapPower); got != (err == nil) {
				t.Errorf("power declared = %v, systemctl present = %v", got, err == nil)
			}
		})
	}
}
