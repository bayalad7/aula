package openapp

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bayalad7/aula/internal/platform"
	"github.com/bayalad7/aula/internal/protocol"
)

var folders = platform.AppFolders{
	Vars: map[string]string{
		"ProgramFiles": `C:\Program Files`, "ProgramFiles(x86)": `C:\Program Files (x86)`,
		"SystemRoot": `C:\Windows`, "LOCALAPPDATA": `C:\Users\ana\AppData\Local`,
	},
	Roots: []string{`C:\Program Files`, `C:\Program Files (x86)`, `C:\Windows`, `C:\Users\ana\AppData\Local\Programs`},
	Forbidden: []string{`C:\Users\ana\Documents`, `C:\Users\ana\Desktop`, `C:\Users\ana\Downloads`,
		`C:\Users\ana\AppData\Local\Temp`, `C:\Windows\Temp`},
}

func TestCheckPathAllows(t *testing.T) {
	for in, want := range map[string]string{
		`C:\Program Files\Thonny\thonny.exe`:                             `C:\Program Files\Thonny\thonny.exe`,
		`c:/program files (x86)/Scratch 3/Scratch 3.exe`:                 `C:\program files (x86)\Scratch 3\Scratch 3.exe`,
		`C:\Windows\System32\notepad.exe`:                                `C:\Windows\System32\notepad.exe`,
		`C:\Users\ana\AppData\Local\Programs\Microsoft VS Code\Code.exe`: `C:\Users\ana\AppData\Local\Programs\Microsoft VS Code\Code.exe`,
		`C:\Program Files\\App\\app.EXE`:                                 `C:\Program Files\App\app.EXE`,
	} {
		got, err := CheckPath(in, folders)
		if err != nil || got != want {
			t.Errorf("CheckPath(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}

// An app outside the rules is refused, whatever the console sends.
func TestCheckPathRefuses(t *testing.T) {
	for _, in := range []string{
		`C:\Windows\System32\cmd.exe`, `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`,
		`C:\Windows\System32\MSHTA.EXE`, `C:\Windows\explorer.exe`, `C:\Windows\System32\rundll32.exe`,
		`C:\Users\ana\Documents\Aula\virus.exe`, `C:\Users\ana\Desktop\juego.exe`,
		`C:\Users\ana\Downloads\setup.exe`, `C:\Users\ana\AppData\Local\Temp\x.exe`, `C:\Windows\Temp\x.exe`,
		`C:\Users\ana\AppData\Local\juego.exe`, `D:\juegos\juego.exe`, `C:\juego.exe`,
		`C:\Program Files\..\Users\ana\Documents\x.exe`, `C:\Program Files\App\.\app.exe`,
		`\\servidor\compartida\app.exe`, `\\?\C:\Program Files\app.exe`, `\\.\C:\Program Files\app.exe`,
		`C:\Program Files\App\app.bat`, `C:\Program Files\App\app.ps1`, `C:\Program Files\App\app.lnk`,
		`C:\Program Files\App\app.exe:stream`, `C:\Program Files\App\app.exe.`, `C:\Program Files\App\.exe`,
		`C:\Program Files\App\app.exe /c del`, `Program Files\App\app.exe`, ``, `C:`,
		`C:\Program Files Malos\app.exe`,
	} {
		if got, err := CheckPath(in, folders); !errors.Is(err, ErrNotAllowed) {
			t.Errorf("CheckPath(%q) = %q, %v; want not allowed", in, got, err)
		}
	}
}

func TestExpand(t *testing.T) {
	got, err := Expand(`%programfiles(x86)%\Thonny\thonny.exe`, folders.Vars)
	if err != nil || got != `C:\Program Files (x86)\Thonny\thonny.exe` {
		t.Errorf("Expand = %q, %v", got, err)
	}
	for _, in := range []string{
		`%USERPROFILE%\Documents\x.exe`, `%TEMP%\x.exe`, `%ProgramFiles%x.exe`, `%ProgramFiles%\%TEMP%\x.exe`,
		`C:\%SystemRoot%\x.exe`, `%ProgramFiles\x.exe`,
	} {
		if got, err := Expand(in, folders.Vars); !errors.Is(err, ErrNotAllowed) {
			t.Errorf("Expand(%q) = %q, %v; want not allowed", in, got, err)
		}
	}
	if _, err := Expand(`%ProgramFiles(x86)%\x.exe`, map[string]string{"ProgramFiles": `C:\PF`}); !errors.Is(err, ErrNotAllowed) {
		t.Error("a variable the computer lacks must be refused")
	}
}

func TestCheckSyntaxAndDefaults(t *testing.T) {
	apps, err := Normalize(Defaults())
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range apps {
		e, _ := Expand(a.Path, folders.Vars)
		if _, err := CheckPath(e, folders); err != nil {
			t.Errorf("example %s does not pass the agent's rules: %v", a.Name, err)
		}
	}
	for _, p := range []string{`%TEMP%\x.exe`, `C:\Windows\System32\cmd.exe`, `relativo\x.exe`, `C:\x.txt`, strings.Repeat("a", 300),
		`%LOCALAPPDATA%\Temp\x.exe`, `%localappdata%\juego.exe`} {
		if CheckSyntax(p) == nil {
			t.Errorf("CheckSyntax(%q) passed", p)
		}
	}
}

func TestNormalize(t *testing.T) {
	apps, err := Normalize([]App{
		{Name: " Visual Studio Code ", Path: `%LOCALAPPDATA%\Programs\Microsoft VS Code\Code.exe`},
		{ID: "mio", Name: "Mío", Path: `C:\Program Files\Mio\mio.exe`},
		{Name: "Árbol Ñandú", Path: `C:\Program Files\A\a.exe`},
		{ID: "mio", Name: "Otro", Path: `C:\Program Files\B\b.exe`},
		{ID: "Bad ID", Name: "Visual-Studio Code!", Path: `C:\Program Files\C\c.exe`},
	})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, a := range apps {
		ids = append(ids, a.ID)
	}
	if got := strings.Join(ids, ","); got != "visual-studio-code,mio,arbol-nandu,otro,visual-studio-code-2" {
		t.Errorf("ids %s", got)
	}
	if apps[0].Name != "Visual Studio Code" {
		t.Errorf("name not trimmed: %q", apps[0].Name)
	}
	var ee *EntryError
	if _, err := Normalize([]App{{Name: "A", Path: `C:\Program Files\a.exe`}, {Name: "a", Path: `C:\Program Files\b.exe`}}); !errors.As(err, &ee) || ee.Index != 1 || ee.Field != FieldName {
		t.Errorf("duplicate name: %v", err)
	}
	if _, err := Normalize([]App{{Name: "A", Path: `C:\Windows\System32\cmd.exe`}}); !errors.As(err, &ee) || ee.Field != FieldPath {
		t.Errorf("denied path: %v", err)
	}
	if _, err := Normalize(make([]App, MaxApps+1)); !errors.As(err, &ee) || ee.Field != FieldList {
		t.Errorf("too many: %v", err)
	}
	data, _ := Marshal(apps)
	back, err := Parse(data)
	if err != nil || len(back) != len(apps) || back[4].ID != apps[4].ID {
		t.Errorf("Parse(Marshal) = %v, %v", back, err)
	}
}

func TestShown(t *testing.T) {
	for in, want := range map[string]string{
		"https://Docs.Python.org/3/tutorial/?q=secreto#x": "docs.python.org/3/tutorial",
		"http://example.com":                              "example.com",
		"https://es.wikipedia.org/wiki/Programaci%C3%B3n": "es.wikipedia.org/wiki/Programación",
		"https://a.org/" + strings.Repeat("x", 100):       "a.org/" + strings.Repeat("x", 73) + "…",
	} {
		if got := Shown(in); got != want {
			t.Errorf("Shown(%q) = %q, want %q", in, got, want)
		}
	}
}

// fakeLauncher records what it would open; nothing reaches the desktop.
type fakeLauncher struct {
	mu      sync.Mutex
	exists  map[string]string // path → final path
	opened  []string
	started []string
	fail    error
}

func (f *fakeLauncher) Folders() (platform.AppFolders, error) { return folders, nil }
func (f *fakeLauncher) Resolve(p string) (string, error) {
	if final, ok := f.exists[strings.ToLower(p)]; ok {
		return final, nil
	}
	return "", fmt.Errorf("resolve: %w", fs.ErrNotExist)
}
func (f *fakeLauncher) OpenURL(u string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.opened = append(f.opened, u)
	return f.fail
}
func (f *fakeLauncher) StartApp(p string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.started = append(f.started, p)
	return f.fail
}

func TestLocalStartApp(t *testing.T) {
	thonny := `C:\Users\ana\AppData\Local\Programs\Thonny\thonny.exe`
	l := &fakeLauncher{exists: map[string]string{
		strings.ToLower(thonny):                          thonny,
		strings.ToLower(`C:\Program Files\Enlace\a.exe`): `C:\Users\ana\Documents\Aula\virus.exe`, // a link out
	}}
	s := Local{Launcher: l}
	if err := s.StartApp(`%LOCALAPPDATA%\Programs\Thonny\thonny.exe`); err != nil {
		t.Fatal(err)
	}
	if err := s.StartApp(`C:\Program Files\Enlace\a.exe`); !errors.Is(err, ErrNotAllowed) {
		t.Errorf("a link to the work folder: %v", err)
	}
	if err := s.StartApp(`C:\Program Files\Falta\falta.exe`); Reason(err) != protocol.OpenNotInstalled {
		t.Errorf("missing app: %v", err)
	}
	if err := s.StartApp(`%USERPROFILE%\Documents\Aula\virus.exe`); Reason(err) != protocol.OpenNotAllowed {
		t.Errorf("a variable outside the list: %v", err)
	}
	if len(l.started) != 1 || l.started[0] != thonny {
		t.Errorf("started %q, want only %q", l.started, thonny)
	}
	if err := s.OpenURL("file:///C:/Windows/System32/cmd.exe"); Reason(err) != protocol.OpenNotAllowed || len(l.opened) != 0 {
		t.Errorf("a file URL: %v, opened %q", err, l.opened)
	}
	l.fail = errors.New("no")
	if err := s.OpenURL("https://python.org"); Reason(err) != protocol.OpenLaunchFailed {
		t.Errorf("launch failure: %v", err)
	}
	for _, r := range []protocol.OpenReason{"", protocol.OpenNotAllowed, protocol.OpenNotInstalled, protocol.OpenBusy, protocol.OpenLaunchFailed} {
		if got := Reason(ReasonErr(r)); got != r {
			t.Errorf("Reason(ReasonErr(%q)) = %q", r, got)
		}
	}
}

type acks struct {
	mu   sync.Mutex
	list []protocol.OpenAck
}

func (a *acks) send(_ context.Context, p protocol.Payload) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.list = append(a.list, p.(protocol.OpenAck))
	return nil
}

func TestOpener(t *testing.T) {
	now := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	l := &fakeLauncher{}
	var opened []Opened
	var mu sync.Mutex
	o := &Opener{Session: Local{Launcher: l}, Now: func() time.Time { return now },
		Who:      func() (string, string) { return "Prof. Ruiz", "1" },
		OnOpened: func(x Opened) { mu.Lock(); opened = append(opened, x); mu.Unlock() }}
	if len(o.Capabilities()) != 2 {
		t.Fatalf("capabilities %v", o.Capabilities())
	}
	var got acks
	send := func(p protocol.Payload) protocol.Payload {
		env := protocol.New(p)
		reply, ok := o.Handle(context.Background(), got.send, env)
		if !ok {
			t.Fatalf("%s not handled", env.Type)
		}
		o.Wait()
		return reply
	}
	id := protocol.NewID()
	send(protocol.OpenURL{OpenID: id, URL: "https://docs.python.org/3/?x=1"})
	send(protocol.OpenApp{OpenID: protocol.NewID(), AppID: "cmd", Name: "Consola", Path: `C:\Windows\System32\cmd.exe`})
	send(protocol.OpenApp{OpenID: protocol.NewID(), AppID: "thonny", Name: "Thonny", Path: `C:\Program Files\Thonny\thonny.exe`})
	// The fourth in a row is over the pace.
	r := send(protocol.OpenURL{OpenID: protocol.NewID(), URL: "https://python.org"})
	if ack, ok := r.(protocol.OpenAck); !ok || ack.Reason != protocol.OpenBusy {
		t.Fatalf("over the pace: %#v", r)
	}
	now = now.Add(Every)
	send(protocol.OpenURL{OpenID: protocol.NewID(), URL: "https://python.org"})

	if len(got.list) != 4 {
		t.Fatalf("acks %+v", got.list)
	}
	want := []protocol.OpenReason{"", protocol.OpenNotAllowed, protocol.OpenNotInstalled, ""}
	for i, a := range got.list {
		if a.Reason != want[i] {
			t.Errorf("ack %d: %+v, want reason %q", i, a, want[i])
		}
	}
	if len(l.started) != 0 {
		t.Errorf("started %q: nothing may start", l.started)
	}
	if len(opened) != 2 || opened[0].Shown != "docs.python.org/3" || opened[0].Teacher != "Prof. Ruiz" || opened[0].ID != id {
		t.Errorf("opened %+v", opened)
	}
	if _, ok := o.Handle(context.Background(), got.send, protocol.New(protocol.CodeClear{})); ok {
		t.Error("code.clear handled")
	}
	none := &Opener{}
	if none.Capabilities() != nil {
		t.Error("without a session it must declare nothing")
	}
	if reply, _ := none.Handle(context.Background(), got.send, protocol.New(protocol.OpenURL{OpenID: id, URL: "https://a.org"})); reply.(protocol.ErrorPayload).Code != protocol.CodeUnsupportedCapability {
		t.Errorf("without a session: %#v", reply)
	}
}

func TestTracker(t *testing.T) {
	var tr Tracker
	id := protocol.NewID()
	tr.Start(id, KindURL, "https://python.org", time.Now())
	tr.Sent(id, "s1", "PC-01", "e1")
	tr.Sent(id, "s2", "PC-02", "e2")
	tr.Sent(id, "s3", "PC-03", "e3")
	tr.Skipped(id, "PC-04", true)
	if !tr.Ack("s1", protocol.OpenAck{OpenID: id, Status: protocol.OpenOpened}) {
		t.Fatal("ack not applied")
	}
	tr.Ack("s2", protocol.OpenAck{OpenID: id, Status: protocol.OpenFailed, Reason: protocol.OpenNotInstalled})
	if tr.Ack("s1", protocol.OpenAck{OpenID: id, Status: protocol.OpenFailed}) {
		t.Error("a second ack changed the target")
	}
	if !tr.Error("s3", "e3") || tr.Error("s3", "e3") {
		t.Error("error not applied once")
	}
	o := tr.Recent()[0]
	if o.Opened != 1 || o.Refused != 2 || o.Pending != 0 || len(o.Unsupported) != 1 {
		t.Errorf("order %+v", o)
	}
	for range MaxOrders {
		tr.Start(protocol.NewID(), KindApp, "Thonny", time.Now())
	}
	if len(tr.Recent()) != MaxOrders || tr.Ack("s1", protocol.OpenAck{OpenID: id, Status: protocol.OpenOpened}) {
		t.Error("old orders must be forgotten")
	}
	tr.Clear()
	if len(tr.Recent()) != 0 {
		t.Error("Clear")
	}
}
