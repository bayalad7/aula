package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/bayalad7/aula/internal/protocol"
)

// Every error code has its message in textos.es-MX.js, so the UI never shows
// a raw Go error for a known failure.
func TestErrorCodesHaveTexts(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("frontend", "dist", "textos.es-MX.js"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, code := range errorCodes {
		if seen[code] {
			t.Errorf("code %q listed twice", code)
		}
		seen[code] = true
		for _, part := range []string{"titulo", "texto"} {
			if !strings.Contains(string(b), `"error.`+code+`.`+part+`":`) {
				t.Errorf("textos.es-MX.js lacks error.%s.%s", code, part)
			}
		}
	}
}

// The bindings fail with a code the UI understands, on the wire as
// "aula:<code>: <detail>".
func TestBindingsReturnCodedErrors(t *testing.T) {
	h := newHarness(t, student("s1", "PC01", protocol.CapCodeView, protocol.CapMsgShow))
	bare := NewApp(t.TempDir(), &fakeClipboard{}, nil, &fakeHotkeys{}, quiet)
	long := strings.Repeat("x", protocol.MaxTitle+1)
	huge := strings.Repeat("x", protocol.MaxCodeContent+1)
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"no class", call2(bare.Emit(EmitRequest{Title: "x", Lang: "python", Content: "x", Mode: "replace"})), codeNoClass},
		{"bad channel", call2(h.app.StartClass("abc", "Prof")), codeInvalidChannel},
		{"no name", call2(h.app.StartClass("1", "  ")), codeInvalidName},
		{"bad mode", call2(h.app.Emit(EmitRequest{Lang: "python", Content: "x", Mode: "x"})), codeInvalidMode},
		{"empty code", call2(h.app.Emit(EmitRequest{Lang: "python", Content: "  ", Mode: "replace"})), codeEmptyCode},
		{"long title", call2(h.app.Emit(EmitRequest{Title: long, Lang: "python", Content: "x", Mode: "replace"})), codeTitleTooLong},
		{"huge code", call2(h.app.Emit(EmitRequest{Lang: "python", Content: huge, Mode: "replace"})), codeCodeTooLarge},
		{"bad language", call2(h.app.Emit(EmitRequest{Lang: "No Valido", Content: "x", Mode: "replace"})), codeUnknownLanguage},
		{"empty notice", call2(h.app.SendNotice("", "info")), codeInvalidNotice},
		{"bad level", call2(h.app.SendNotice("hola", "x")), codeInvalidNotice},
		{"snippet gone", call2(h.app.Reemit("nope")), codeSnippetGone},
		{"delivery gone", call2(h.app.Resend("nope")), codeDeliveryGone},
		{"empty clipboard", call2(h.app.CaptureClipboard()), codeClipboardEmpty},
		{"bad kind", call2(h.app.SetRestriction(RestrictionRequest{Kind: "x", Mode: "deny", Items: []string{"a"}})), codeInvalidRestriction},
		{"empty list", call2(h.app.SetRestriction(RestrictionRequest{Kind: "web", Mode: "deny"})), codeInvalidRestriction},
		{"bad app", call2(h.app.SetRestriction(RestrictionRequest{Kind: "apps", Mode: "deny", Items: []string{"no*vale"}})), codeInvalidApp},
		{"bad domain", call2(h.app.SetRestriction(RestrictionRequest{Kind: "web", Mode: "deny", Items: []string{"no vale"}})), codeInvalidDomain},
		{"bad power", call2(h.app.PowerAction(PowerRequest{Action: "x"})), codeInvalidPower},
		{"no power targets", call2(newHarness(t).app.PowerAction(PowerRequest{Action: "reboot"})), codeNoPowerTargets},
		{"no simulation here", call2(h.app.SimulationStart(1)), codeSimUnavailable},
		{"simulation closed", call2(h.app.SimulationRemove("x")), codeSimClosed},
	}
	for _, c := range cases {
		if got := errorCode(c.err); got != c.want {
			t.Errorf("%s: code %q (%v), want %q", c.name, got, c.err, c.want)
			continue
		}
		if !strings.HasPrefix(c.err.Error(), "aula:"+c.want+": ") {
			t.Errorf("%s: wire form %q", c.name, c.err.Error())
		}
	}
}

// call2 keeps the error of a binding's (value, error) result.
func call2[T any](_ T, err error) error { return err }

// The busy port of a second console gets its code, with the real network.
func TestPortInUseIsCoded(t *testing.T) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	app := NewApp(t.TempDir(), nil, nil, &fakeHotkeys{}, quiet)
	app.port = ln.Addr().(*net.TCPAddr).Port
	_, err = app.StartClass("7", "Prueba")
	if errorCode(err) != codePortInUse {
		t.Fatalf("StartClass on a busy port: %v, want code %s", err, codePortInUse)
	}
	if !strings.Contains(err.Error(), strconv.Itoa(app.port)) {
		t.Errorf("the detail should keep the port: %v", err)
	}
}

func TestCodedKeepsTheFirstCode(t *testing.T) {
	inner := codedf(codeSiteKey, "x")
	if errorCode(coded(codeCertificate, inner)) != codeSiteKey {
		t.Error("coded replaced an existing code")
	}
	if coded(codeNoClass, nil) != nil {
		t.Error("coded(nil) must stay nil")
	}
}

// Every error links to an existing section of the manual (the manual itself
// excepted: if it cannot be read, there is nothing to link to).
func TestErrorCodesLinkToTheManual(t *testing.T) {
	app := NewApp(t.TempDir(), nil, nil, &fakeHotkeys{}, quiet)
	v, err := app.Manual()
	if err != nil {
		t.Fatal(err)
	}
	m, _, _ := app.loadManual()
	for _, code := range errorCodes {
		href, ok := v.Errors[code]
		if !ok {
			t.Errorf("error %q has no manual link in errorHelp", code)
			continue
		}
		if code == codeManualFailed {
			continue
		}
		if !m.Resolves(href, "", nil) {
			t.Errorf("error %q links to %q, which is not a section of the manual", code, href)
		}
	}
	if v.Tree == nil || len(v.Pages) == 0 || len(v.Images) == 0 {
		t.Errorf("manual view incomplete: %d pages, %d images, tree %v", len(v.Pages), len(v.Images), v.Tree != nil)
	}
	if r := app.ManualSearch("firewall"); len(r) == 0 {
		t.Error("searching the manual for firewall found nothing")
	}
}

// Every error code of internal/protocol (read from its source, so a new one
// cannot be forgotten) has its texts and a manual section.
func TestAgentErrorCodesLinkToTheManual(t *testing.T) {
	src := filepath.Join("..", "..", "internal", "protocol", "errors.go")
	f, err := parser.ParseFile(token.NewFileSet(), src, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var codes []protocol.ErrorCode
	for _, d := range f.Decls {
		g, ok := d.(*ast.GenDecl)
		if !ok || g.Tok != token.CONST {
			continue
		}
		for _, spec := range g.Specs {
			vs := spec.(*ast.ValueSpec)
			if id, ok := vs.Type.(*ast.Ident); !ok || id.Name != "ErrorCode" {
				continue
			}
			for _, v := range vs.Values {
				lit := v.(*ast.BasicLit)
				code, _ := strconv.Unquote(lit.Value)
				codes = append(codes, protocol.ErrorCode(code))
			}
		}
	}
	if len(codes) < 7 {
		t.Fatalf("found %d protocol error codes in %s; did the declaration change?", len(codes), src)
	}
	app := NewApp(t.TempDir(), nil, nil, &fakeHotkeys{}, quiet)
	m, _, err := app.loadManual()
	if err != nil {
		t.Fatal(err)
	}
	for _, code := range codes {
		href, ok := agentErrorHelp[code]
		if !ok {
			t.Errorf("protocol error %q has no manual link in agentErrorHelp", code)
			continue
		}
		if !m.Resolves(href, "", nil) {
			t.Errorf("protocol error %q links to %q, which is not a section of the manual", code, href)
		}
	}
	if len(agentErrorHelp) != len(codes) {
		t.Errorf("agentErrorHelp has %d codes, internal/protocol %d", len(agentErrorHelp), len(codes))
	}
	if got := agentErrorKey("unsupported_capability"); got != "unsupportedCapability" {
		t.Errorf("agentErrorKey = %q", got)
	}
}
