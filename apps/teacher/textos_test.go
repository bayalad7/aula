package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/bayalad7/aula/internal/collect"
	"github.com/bayalad7/aula/internal/connectcheck"
	"github.com/bayalad7/aula/internal/filepush"
	"github.com/bayalad7/aula/internal/openapp"
	"github.com/bayalad7/aula/internal/protocol"
	"github.com/bayalad7/aula/internal/remote"
	"github.com/bayalad7/aula/internal/snapshot"
)

// Every text the console shows lives in frontend/dist/textos.es-MX.js
// (one file, ready for other languages). This test fails if index.html or
// main.js use a key that does not exist, or if the file keeps unused keys.
func TestTextsAreComplete(t *testing.T) {
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join("frontend", "dist", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	textos, html := read("textos.es-MX.js"), read("index.html")
	var js strings.Builder
	for _, line := range strings.Split(read("main.js"), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "//") {
			js.WriteString(line + "\n")
		}
	}
	main := js.String()

	defined := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^\s*"([A-Za-z0-9.]+)":\s*"`).FindAllStringSubmatch(textos, -1) {
		if defined[m[1]] {
			t.Errorf("textos: %q is defined twice", m[1])
		}
		defined[m[1]] = true
	}
	if len(defined) < 100 {
		t.Fatalf("textos.es-MX.js: only %d keys found; is the format still `\"key\": \"text\"`?", len(defined))
	}

	used := map[string]string{} // key → where
	need := func(key, where string) {
		if _, ok := used[key]; !ok {
			used[key] = where
		}
	}
	for _, m := range regexp.MustCompile(`data-t(?:-[a-z-]+)?="([A-Za-z0-9.]+)"`).FindAllStringSubmatch(html, -1) {
		need(m[1], "index.html")
	}
	// A data-info="x" button shows info.x.etiqueta and info.x.texto.
	for _, m := range regexp.MustCompile(`data-info="([a-z]+)"`).FindAllStringSubmatch(html, -1) {
		need("info."+m[1]+".etiqueta", "index.html (data-info)")
		need("info."+m[1]+".texto", "index.html (data-info)")
	}
	for _, m := range regexp.MustCompile(`\bT\("([A-Za-z0-9.]+)"`).FindAllStringSubmatch(main, -1) {
		need(m[1], "main.js")
	}
	for _, m := range regexp.MustCompile(`\bP\("([A-Za-z0-9.]+)"`).FindAllStringSubmatch(main, -1) {
		need(m[1]+".uno", "main.js (plural)")
		need(m[1]+".varios", "main.js (plural)")
	}
	// Keys built at run time ("error.x" + ".titulo", ACCION + "Sustantivo",
	// lists of keys): every literal that looks like a key must be a key, or
	// the base of one.
	sections := map[string]bool{}
	for key := range defined {
		sections[strings.SplitN(key, ".", 2)[0]] = true
	}
	for _, m := range regexp.MustCompile(`"([a-z][A-Za-z]*\.[A-Za-z0-9]+)"`).FindAllStringSubmatch(main, -1) {
		lit := m[1]
		if !sections[strings.SplitN(lit, ".", 2)[0]] {
			continue // a domain, a capability…: not a text
		}
		if defined[lit+"Sustantivo"] {
			need(lit+"Sustantivo", "main.js (composed)")
		}
		switch {
		case defined[lit]:
			need(lit, "main.js (literal)")
		case defined[lit+".titulo"] && defined[lit+".texto"]:
			need(lit+".titulo", "main.js (composed)")
			need(lit+".texto", "main.js (composed)")
		case defined[lit+".uno"] && defined[lit+".varios"]:
			need(lit+".uno", "main.js (composed)")
			need(lit+".varios", "main.js (composed)")
		default:
			t.Errorf("main.js uses %q, which is neither a text key nor the base of one", lit)
		}
	}

	// Remote control: the dialog words the status and the toast the reason
	// (remote.Control, internal/remote).
	for _, s := range []protocol.RemoteStatus{remote.StatusStarting, protocol.RemoteActive, protocol.RemotePaused,
		protocol.RemoteEnded, remote.StatusGone, remote.StatusFailed} {
		need("control.estado."+string(s), "main.js (remote.Control.Status)")
	}
	for _, r := range []string{string(protocol.RemoteByTeacher), string(protocol.RemoteReplaced), string(protocol.RemoteIdle),
		string(protocol.RemoteSession), string(protocol.RemoteError), string(remote.StatusGone), string(remote.StatusFailed)} {
		need("control.fin."+r, "main.js (remote.Control.Reason or Status)")
	}

	// Screenshots: the preview and the batch word why one failed and where
	// each one is (snapshot.Snapshot.Reason and Status).
	for _, r := range []string{snapshot.ReasonTimeout, snapshot.ReasonUnsupported, snapshot.ReasonCapture,
		snapshot.ReasonBusy, snapshot.ReasonTooLarge, snapshot.ReasonCorrupt, snapshot.ReasonGone} {
		need("captura.fallo."+r+".titulo", "main.js (snapshot.Snapshot.Reason)")
		need("captura.fallo."+r+".texto", "main.js (snapshot.Snapshot.Reason)")
	}
	for _, s := range []snapshot.Status{snapshot.StatusPending, snapshot.StatusReady} {
		need("captura.lote.estado."+string(s), "main.js (snapshot.Snapshot.Status)")
	}

	// Why a template cannot be sent (filepush.Template.Problem).
	for _, p := range []string{filepush.ProblemEmpty, filepush.ProblemTooMany, filepush.ProblemTooLarge, filepush.ProblemBadName} {
		need("archivos.problema."+agentErrorKey(protocol.ErrorCode(p)), "main.js (filepush.Template.Problem)")
	}
	// Collections (internal/collect): each student's status, and why a round
	// failed or a file was skipped (protocol reasons and the console's own).
	for _, s := range []collect.Status{collect.StatusWaiting, collect.StatusReady, collect.StatusQueued,
		collect.StatusCollecting, collect.StatusCollected, collect.StatusEmpty, collect.StatusFailed} {
		need("reco.estado."+string(s), "main.js (collect.Status)")
	}
	reasons := []string{collect.ReasonTimeout, collect.ReasonGone, collect.ReasonSave, string(protocol.CodeUnsupportedCapability)}
	for _, list := range [][]protocol.FileCollectReason{protocol.FileCollectAckReasons, protocol.FileCollectSkipReasons, protocol.FileCollectSaveReasons} {
		for _, r := range list {
			reasons = append(reasons, string(r))
		}
	}
	for _, r := range reasons {
		need("reco.motivo."+agentErrorKey(protocol.ErrorCode(r)), "main.js (collect reasons)")
	}
	// Opening a page or an app (internal/openapp): why a computer did not
	// open it, each target's status and where the allowed apps come from.
	for _, r := range []protocol.OpenReason{protocol.OpenNotAllowed, protocol.OpenNotInstalled, protocol.OpenLaunchFailed, protocol.OpenBusy} {
		need("abrir.motivo."+agentErrorKey(protocol.ErrorCode(r)), "main.js (open.ack reason)")
	}
	for _, s := range []string{openapp.StatusSent, string(protocol.OpenOpened), string(protocol.OpenFailed), openapp.StatusError} {
		need("abrir.acuse."+s, "main.js (openapp.Target.Status)")
	}
	for _, s := range []string{appsCentre, appsConsole, appsExamples} {
		need("ajustes.appsFuente."+s, "main.js (AllowedApps.Source)")
	}
	// Error texts are chosen by the code Go returns (errores.go).
	for _, code := range errorCodes {
		need("error."+code+".titulo", "errores.go")
		need("error."+code+".texto", "errores.go")
	}

	// Why the capture hotkey is off (HotkeyStatus.Reason), and the fallback.
	for _, r := range []string{hotkeyInUse, hotkeyNotSupported, "Inactivo", "Activo"} {
		need("ajustes.atajo"+r, "events.go (HotkeyStatus.Reason)")
	}

	// The connection checker's texts: comprobar.<step>.titulo, one per outcome
	// a step can end with, and one per status.
	for id, outcomes := range connectcheck.Outcomes {
		need("comprobar."+string(id)+".titulo", "connectcheck")
		for _, o := range outcomes {
			need("comprobar."+string(id)+"."+o, "connectcheck")
		}
	}
	for _, st := range connectcheck.Statuses {
		need("comprobar.estado."+string(st), "connectcheck")
	}

	// So are the texts of the students' protocol errors (agenterrors.go).
	for code := range agentAndFileHelp() {
		need("errorAlumno."+agentErrorKey(code)+".titulo", "agenterrors.go")
		need("errorAlumno."+agentErrorKey(code)+".texto", "agenterrors.go")
	}

	for key, where := range used {
		if !defined[key] {
			t.Errorf("%s uses the text %q, missing from textos.es-MX.js", where, key)
		}
	}
	for key := range defined {
		if _, ok := used[key]; !ok {
			t.Errorf("textos.es-MX.js: %q is never used", key)
		}
	}
	if strings.Contains(main, "innerHTML") {
		t.Error("main.js must not use innerHTML")
	}
}
