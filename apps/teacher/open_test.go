package main

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/bayalad7/aula/internal/openapp"
	"github.com/bayalad7/aula/internal/protocol"
)

func TestOpenPageTargetsAndAcks(t *testing.T) {
	h := newHarness(t,
		student("s1", "PC01", protocol.CapOpenURL, protocol.CapOpenApp),
		student("s2", "PC02", protocol.CapOpenURL, protocol.CapOpenApp),
		student("s3", "MAC03", protocol.CapCodeView)) // cannot open
	if st := h.app.View().Students; st.CanOpenURL != 2 || st.CanOpenApp != 2 || !st.Students[0].CanOpenURL || st.Students[2].CanOpenApp {
		t.Fatalf("counts %+v", st)
	}
	// Without a scheme it gets https://.
	res, err := h.app.OpenPage(OpenRequest{URL: " docs.python.org/3/ "})
	if err != nil {
		t.Fatal(err)
	}
	if res.Sent != 2 || !slices.Equal(res.Unsupported, []string{"MAC03"}) {
		t.Fatalf("result %+v", res)
	}
	ds := h.class.takeDirect()
	if got := sentTypes(ds); !slices.Equal(got, []string{"s1:open.url", "s2:open.url"}) {
		t.Fatalf("sent %v", got)
	}
	p := ds[0].p.(protocol.OpenURL)
	if p.URL != "https://docs.python.org/3/" || p.OpenID != res.OpenID || ds[1].p.(protocol.OpenURL).OpenID != p.OpenID {
		t.Fatalf("order %+v", p)
	}
	h.hooks.onOpen("s1", protocol.Envelope{Payload: protocol.OpenAck{RefID: ds[0].envID, OpenID: p.OpenID, Status: protocol.OpenOpened}})
	h.hooks.onOpen("s2", protocol.Envelope{Payload: protocol.NewError(ds[1].envID, protocol.CodeInternal, "x")})
	orders := h.rec.last(EventOpens).([]openapp.Order)
	if o := orders[0]; o.Opened != 1 || o.Refused != 1 || o.Pending != 0 || o.Label != p.URL || len(o.Unsupported) != 1 {
		t.Fatalf("order %+v", o)
	}

	// Marked students only.
	if res, err := h.app.OpenPage(OpenRequest{URL: "https://python.org", Hosts: []string{"pc02"}}); err != nil || res.Sent != 1 {
		t.Fatalf("%+v, %v", res, err)
	}
	if got := sentTypes(h.class.takeDirect()); !slices.Equal(got, []string{"s2:open.url"}) {
		t.Fatalf("sent %v", got)
	}

	// Never a URL that is not http or https.
	for _, u := range []string{"", "file:///C:/Windows/System32/cmd.exe", "javascript:alert(1)", "https://usuario:clave@python.org", "ms-settings:display"} {
		if _, err := h.app.OpenPage(OpenRequest{URL: u}); !isCode(err, codeInvalidURL) {
			t.Errorf("OpenPage(%q) = %v, want %s", u, err, codeInvalidURL)
		}
	}
	if len(h.class.takeDirect()) != 0 {
		t.Fatal("an invalid URL was sent")
	}
	// Nobody can: an error, nothing sent.
	if _, err := h.app.OpenPage(OpenRequest{URL: "https://python.org", Hosts: []string{"MAC03"}}); !isCode(err, codeNoOpenTargets) {
		t.Fatalf("err %v", err)
	}
}

func isCode(err error, code string) bool {
	var ce *codedError
	return errors.As(err, &ce) && ce.code == code
}

func TestOpenAppFromTheCatalogOnly(t *testing.T) {
	h := newHarness(t, student("s1", "PC01", protocol.CapOpenURL, protocol.CapOpenApp))
	h.app.siteKeyDir = filepath.Join(t.TempDir(), "centro") // no centre file
	cat := h.app.AllowedApps()
	if cat.Source != appsExamples || len(cat.Apps) != len(openapp.Defaults()) {
		t.Fatalf("catalog %+v", cat)
	}
	res, err := h.app.OpenApp(OpenRequest{AppID: "thonny"})
	if err != nil || res.Sent != 1 {
		t.Fatalf("%+v, %v", res, err)
	}
	p := h.class.takeDirect()[0].p.(protocol.OpenApp)
	if p.AppID != "thonny" || p.Name != "Thonny" || p.Path != `%LOCALAPPDATA%\Programs\Thonny\thonny.exe` {
		t.Fatalf("order %+v", p)
	}
	// An ID outside the list is never sent, whatever the window asks.
	if _, err := h.app.OpenApp(OpenRequest{AppID: "cmd"}); !isCode(err, codeOpenAppGone) {
		t.Fatalf("unknown app: %v", err)
	}
	if len(h.class.takeDirect()) != 0 {
		t.Fatal("an app outside the list was sent")
	}
}

func TestSaveAllowedApps(t *testing.T) {
	h := newHarness(t)
	centre := filepath.Join(t.TempDir(), "centro")
	h.app.siteKeyDir = centre
	saved, err := h.app.SaveAllowedApps([]openapp.App{{Name: "PSeInt", Path: `%ProgramFiles(x86)%\PSeInt\pseint.exe`}})
	if err != nil {
		t.Fatal(err)
	}
	if saved.Source != appsConsole || saved.Apps[0].ID != "pseint" || saved.Path != filepath.Join(h.app.dir, allowedAppsFile) {
		t.Fatalf("saved %+v", saved)
	}
	if got := h.app.AllowedApps(); got.Source != appsConsole || len(got.Apps) != 1 {
		t.Fatalf("read back %+v", got)
	}
	if _, ok := h.rec.last(EventAllowedApps).(AllowedApps); !ok {
		t.Fatal("no event")
	}
	for _, bad := range [][]openapp.App{
		{{Name: "Consola", Path: `C:\Windows\System32\cmd.exe`}},
		{{Name: "Juego", Path: `%TEMP%\juego.exe`}},
		{{Name: "", Path: `C:\Program Files\a.exe`}},
		{{Name: "A", Path: `C:\Program Files\a.exe`}, {Name: "a", Path: `C:\Program Files\b.exe`}},
	} {
		if _, err := h.app.SaveAllowedApps(bad); !isCode(err, codeAllowedAppInvalid) {
			t.Errorf("SaveAllowedApps(%+v) = %v", bad, err)
		}
	}
	// The centre's file wins over the console's, and saving writes it.
	if err := os.MkdirAll(centre, 0o700); err != nil {
		t.Fatal(err)
	}
	data, _ := openapp.Marshal(openapp.Defaults()[:2])
	if err := os.WriteFile(filepath.Join(centre, allowedAppsFile), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := h.app.AllowedApps(); got.Source != appsCentre || len(got.Apps) != 2 {
		t.Fatalf("centre %+v", got)
	}
	if s, err := h.app.SaveAllowedApps(openapp.Defaults()[:1]); err != nil || s.Source != appsCentre {
		t.Fatalf("%+v, %v", s, err)
	}
	b, _ := os.ReadFile(filepath.Join(centre, allowedAppsFile))
	if !strings.Contains(string(b), "bloc-de-notas") || strings.Contains(string(b), "calculadora") {
		t.Fatalf("centre file %s", b)
	}
}

func TestCheckOpenPageWarnsAboutTheWebFilter(t *testing.T) {
	h := newHarness(t,
		student("s1", "PC01", protocol.CapOpenURL, protocol.CapWebFilter),
		student("s2", "PC02", protocol.CapOpenURL, protocol.CapWebFilter))
	if c, err := h.app.CheckOpenPage("youtube.com/watch", nil); err != nil || len(c.Filtered) != 0 || c.URL != "https://youtube.com/watch" {
		t.Fatalf("no filter: %+v, %v", c, err)
	}
	if _, err := h.app.SetRestriction(RestrictionRequest{Kind: "web", Mode: "deny", Items: []string{"youtube.com"}, Hosts: []string{"PC02"}}); err != nil {
		t.Fatal(err)
	}
	if c, _ := h.app.CheckOpenPage("https://www.youtube.com/watch", nil); !slices.Equal(c.Filtered, []string{"PC02"}) {
		t.Fatalf("filtered %v", c.Filtered)
	}
	if c, _ := h.app.CheckOpenPage("https://python.org", nil); len(c.Filtered) != 0 {
		t.Fatalf("python.org filtered %v", c.Filtered)
	}
	if _, err := h.app.CheckOpenPage("ftp://x", nil); !isCode(err, codeInvalidURL) {
		t.Fatalf("err %v", err)
	}
}
