package protocol

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

const (
	testRefID     = "7d1f2e3a-4b5c-4d6e-a7f8-9a0b1c2d3e4f"
	testSnippetID = "0c9a7e52-6a4b-4f7e-8d33-2b1f4e6a9c10"
)

// samplePayloads has one valid payload per v0 message type.
func samplePayloads() []Payload {
	return []Payload{
		AuthChallenge{Nonce: bytes.Repeat([]byte{0xab}, NonceSize)},
		AuthProof{RefID: testRefID, Signature: bytes.Repeat([]byte{0xcd}, SignatureSize)},
		AuthOK{RefID: testRefID},
		HelloInfo{
			Hostname:     "LAB3-PC07",
			Username:     "alumno07",
			OS:           OSWindows,
			OSVersion:    "10.0.19045",
			AgentVersion: "0.1.0",
			Capabilities: []Capability{CapCodeView, CapMsgShow},
		},
		Heartbeat{},
		NewError(testRefID, CodeUnsupportedCapability, "este agente no soporta screen.freeze"),
		CodeShare{
			SnippetID: testSnippetID,
			Exercise:  "3.b",
			Title:     "Factorial recursivo",
			Lang:      "python",
			Content:   "def f(n):\n    return 1 if n <= 1 else n * f(n - 1)\n",
			Mode:      ModeReplace,
		},
		CodeAck{RefID: testRefID, SnippetID: testSnippetID},
		CodeClear{},
		MsgShow{Text: "Guarden su trabajo.", Level: LevelWarning},
		MsgAck{RefID: testRefID},
		ScreenThumbStart{IntervalMS: 1000, Quality: 50, MaxWidth: 320},
		ScreenThumbFrame{RefID: testRefID, Seq: 1, Width: 320, Height: 180, JPEG: []byte{0xff, 0xd8, 0xff, 0xd9}},
		ScreenThumbStop{},
		ScreenFreeze{Message: "Atención al pizarrón, por favor."},
		ScreenFreeze{},
		ScreenUnfreeze{},
		ScreenFreezeAck{RefID: testRefID, Frozen: true},
		ScreenBroadcastStart{BroadcastID: testRefID, Width: 1920, Height: 1080, FPS: 15},
		ScreenBroadcastOffer{BroadcastID: testRefID, Attempt: 1, SDP: testSDP},
		ScreenBroadcastAnswer{RefID: testSnippetID, BroadcastID: testRefID, Attempt: 1, SDP: testSDP},
		ScreenBroadcastICE{BroadcastID: testRefID, Attempt: 1, Candidate: "candidate:1 1 udp 2130706431 192.168.1.20 47831 typ host", SDPMid: "0"},
		ScreenBroadcastStop{BroadcastID: testRefID},
		AppBlockSet{PolicyID: testRefID, Mode: PolicyDeny, Apps: []string{"steam", "Discord.exe"}},
		AppBlockClear{},
		AppBlockAck{RefID: testSnippetID, PolicyID: testRefID, Active: true},
		WebFilterSet{PolicyID: testRefID, Mode: PolicyAllow, Domains: []string{"python.org", "xn--espaol-zwa.org"}},
		WebFilterClear{},
		WebFilterAck{RefID: testSnippetID},
		PowerAction{ActionID: testRefID, Action: PowerReboot, DelayS: 30},
		PowerAction{ActionID: testRefID, Action: PowerShutdown, Force: true},
		PowerCancel{ActionID: testRefID},
		PowerAck{RefID: testSnippetID, ActionID: testRefID, Status: PowerPostponed, RemainingS: 325, PostponesLeft: 1},
		RemoteStart{ControlID: testRefID, IntervalMS: 125, Quality: 60, MaxWidth: 1280},
		RemoteInput{ControlID: testRefID, Seq: 1, Events: []InputEvent{
			{T: InputMove, X: 0.5, Y: 0.25},
			{T: InputButton, B: ButtonLeft, Down: true, X: 0.5, Y: 0.25},
			{T: InputWheel, DY: -120},
			{T: InputKey, Code: "KeyA", Down: true},
		}},
		RemoteStop{ControlID: testRefID},
		RemoteAck{RefID: testSnippetID, ControlID: testRefID, Status: RemoteActive},
		RemoteAck{RefID: testSnippetID, ControlID: testRefID, Status: RemoteEnded, Reason: RemoteIdle},
		ScreenSnapshotRequest{SnapshotID: testRefID, Quality: DefaultSnapshotQuality},
		ScreenSnapshotRequest{SnapshotID: testRefID, Quality: 70, MaxWidth: 1920},
		ScreenSnapshotResponse{RefID: testSnippetID, SnapshotID: testRefID, Part: 1, Parts: 1, Data: []byte{0xff, 0xd8, 0xff, 0xd9},
			TakenAt: 1792771200300, Hostname: "LAB3-PC07", Username: "alumno07", Width: 1920, Height: 1080, Size: 4,
			SHA256: strings.Repeat("ab", 32)},
		ScreenSnapshotResponse{RefID: testSnippetID, SnapshotID: testRefID, Part: 2, Parts: 3, Data: []byte{1, 2, 3}},
		HandRaise{HandID: testRefID},
		HandRaise{HandID: testRefID, Note: "No me compila el for"},
		HandLower{HandID: testRefID, By: HandByStudent},
		HandLower{HandID: testRefID, By: HandByTeacher},
		MsgStudent{MessageID: testRefID, Text: "¿El ejercicio 3 es para hoy?"},
		MsgStudentAck{RefID: testSnippetID, MessageID: testRefID},
		MsgShow{Text: "Sí, antes de las 12.", Level: LevelInfo, ReplyTo: testRefID, Personal: true},
		MsgShow{Text: "Revisa la línea 12", Level: LevelWarning, Personal: true},
		TimerStart{TimerID: testRefID, Label: "Ejercicio 1", DurationS: 600, RemainingMS: 600000},
		TimerStart{TimerID: testRefID, DurationS: 60, RemainingMS: 32000},
		TimerStart{TimerID: testRefID, Label: "Ejercicio 2", DurationS: 600, RemainingMS: 600000, Description: "Resuelve 3 a 5.\nEntrega."},
		TimerExtend{TimerID: testRefID, AddS: 300, RemainingMS: 572000},
		TimerStop{TimerID: testRefID},
		TimerAck{RefID: testSnippetID, TimerID: testRefID, Status: TimerRunning, RemainingMS: 599980},
		TimerAck{RefID: testSnippetID, TimerID: testRefID, Status: TimerEnded},
		FilePush{PushID: testRefID, FileID: testSnippetID, Part: 1, Parts: 1, Data: []byte("id,n\n"),
			Name: "datos.csv", Dest: "Ejercicio 4/datos", Size: 5, SHA256: strings.Repeat("ab", 32),
			Overwrite: true, Label: "Ejercicio 4", Index: 2, Files: 3},
		FilePush{PushID: testRefID, FileID: testSnippetID, Part: 1, Parts: 1, Data: []byte{},
			Name: "__init__.py", SHA256: strings.Repeat("ab", 32), Index: 1, Files: 1},
		FilePush{PushID: testRefID, FileID: testSnippetID, Part: 3, Parts: 4, Data: []byte{1, 2, 3}},
		FilePushAck{RefID: testRefID, PushID: testRefID, FileID: testSnippetID, Status: FilePushSaved, Path: "Ejercicio 4/datos (2).csv", Renamed: true},
		FilePushAck{RefID: testRefID, PushID: testRefID, FileID: testSnippetID, Status: FilePushSaved, Path: "datos.csv", Replaced: true},
		FilePushAck{RefID: testRefID, PushID: testRefID, FileID: testSnippetID, Status: FilePushFailed, Reason: FilePushNoSpace},
		FileCollectOpen{CollectID: testRefID, Date: "2026-11-06", Activity: "Ejercicio 4", Patterns: []string{"*.py"}},
		FileCollectOpen{CollectID: testRefID, Date: "2026-11-06", Activity: "Tarea"},
		FileCollectRequest{CollectID: testRefID, Date: "2026-11-06", Activity: "Ejercicio 4", Round: 2,
			Patterns: []string{"*.py", "*.txt"}, MaxFileSize: 1 << 20, MaxTotal: 8 << 20, MaxFiles: 20},
		FileCollectAck{RefID: testSnippetID, CollectID: testRefID, Status: FileCollectReady, Path: "recolecciones/2026-11-06/Ejercicio 4"},
		FileCollectAck{RefID: testSnippetID, CollectID: testRefID, Round: 1, Status: FileCollectSending,
			Path: "recolecciones/2026-11-06/Ejercicio 4", Files: 2, Total: 3412,
			Skipped: []FileCollectSkipped{{Path: "video.mp4", Size: 52428800, Reason: CollectTooLarge}}},
		FileCollectAck{RefID: testSnippetID, CollectID: testRefID, Round: 1, Status: FileCollectEmpty, Path: "recolecciones/2026-11-06/Ejercicio 4"},
		FileCollectAck{RefID: testSnippetID, CollectID: testRefID, Round: 1, Status: FileCollectFailed, Reason: CollectBusy},
		FileCollectResponse{CollectID: testRefID, Round: 1, FileID: testSnippetID, Part: 1, Parts: 1, Data: []byte("print(1)\n"),
			Path: "src/20231234-Ana.py", Size: 9, SHA256: strings.Repeat("ab", 32), ModifiedAt: 1793955720000, Index: 1, Files: 2},
		FileCollectResponse{CollectID: testRefID, Round: 1, FileID: testSnippetID, Part: 1, Parts: 1, Data: []byte{},
			Path: "vacio.txt", SHA256: strings.Repeat("ab", 32), Index: 2, Files: 2},
		FileCollectResponse{CollectID: testRefID, Round: 1, FileID: testSnippetID, Part: 2, Parts: 3, Data: []byte{1, 2}},
		FileCollectResponseAck{RefID: testRefID, CollectID: testRefID, Round: 1, FileID: testSnippetID, Status: FilePushSaved},
		FileCollectResponseAck{RefID: testRefID, CollectID: testRefID, Round: 1, FileID: testSnippetID, Status: FilePushFailed, Reason: CollectCorrupt},
		OpenURL{OpenID: testSnippetID, URL: "https://docs.python.org/3/tutorial/"},
		OpenApp{OpenID: testSnippetID, AppID: "thonny", Name: "Thonny", Path: `%ProgramFiles(x86)%\Thonny\thonny.exe`},
		OpenAck{RefID: testRefID, OpenID: testSnippetID, Status: OpenOpened},
		OpenAck{RefID: testRefID, OpenID: testSnippetID, Status: OpenFailed, Reason: OpenNotAllowed},
		PollOpen{PollID: testRefID, Kind: PollKindChoice, Question: "¿Qué imprime print(3 // 2)?", Options: []string{"1.5", "1", "2", "Error"}},
		PollOpen{PollID: testRefID, Kind: PollKindYesNo, Question: "¿Entendiste el for?", Named: true},
		PollOpen{PollID: testRefID, Kind: PollKindOpen, Question: "¿Qué te costó más?"},
		PollVote{PollID: testRefID, Choice: intp(1)},
		PollVote{PollID: testRefID, Choice: intp(0)},
		PollVote{PollID: testRefID, Text: "Que el for no termina"},
		PollVoteAck{RefID: testSnippetID, PollID: testRefID, Status: PollVoteAccepted},
		PollVoteAck{RefID: testSnippetID, PollID: testRefID, Status: PollVoteClosed},
		PollClose{PollID: testRefID},
		PollResults{PollID: testRefID, Counts: []int{3, 14, 2, 1}, Voters: 20, Recipients: 22},
		PollResults{PollID: testRefID, Counts: []int{0, 0}, Voters: 0, Recipients: 0},
		PollAck{RefID: testSnippetID, PollID: testRefID, Status: PollAckShown},
		PollAck{RefID: testSnippetID, PollID: testRefID, Status: PollAckResults},
		StudentPresentStart{PresentID: testRefID, MaxWidth: 1920, FPS: 15, MaxKbps: 2500},
		StudentPresentAck{RefID: testSnippetID, PresentID: testRefID, Status: PresentAccepted, Width: 1920, Height: 1080},
		StudentPresentAck{RefID: testSnippetID, PresentID: testRefID, Status: PresentFailed, Reason: PresentCaptureUnavailable},
		StudentPresentOffer{PresentID: testRefID, Attempt: 1, SDP: "v=0\r\n"},
		StudentPresentAnswer{RefID: testSnippetID, PresentID: testRefID, Attempt: 1, SDP: "v=0\r\n"},
		StudentPresentICE{PresentID: testRefID, Attempt: 1, Candidate: "candidate:1 1 udp 1 192.168.1.5 50000 typ host", SDPMid: "0"},
		StudentPresentStop{PresentID: testRefID},
		ClassEnd{EndID: testRefID},
		ClassEndAck{RefID: testRefID, EndID: testRefID, Status: ClassEndFailed, Reason: ClassEndRestrictions},
		ScreenBroadcastStart{BroadcastID: testRefID, Width: 1280, Height: 720, FPS: 15,
			Source: &BroadcastSource{Kind: BroadcastSourceStudent, Hostname: "LAB3-PC07"}},
		ScreenBroadcastStart{BroadcastID: testRefID, Width: 1280, Height: 720, FPS: 15, Mode: BroadcastWindow},
		ScreenBroadcastStart{BroadcastID: testRefID, Width: 1280, Height: 720, FPS: 15, Mode: BroadcastFullscreen},
	}
}

func intp(n int) *int { return &n }

func TestOpenLimits(t *testing.T) {
	for _, raw := range []string{
		"", "javascript:alert(1)", "file:///C:/Windows/System32/cmd.exe", "ms-settings:display",
		"ftp://example.com/", "https://", "https:example.com", "http://user:pass@example.com/",
		"http://a@example.com/", "https://example.com/a b", "https://example.com/\x07",
		`https://example.com\..\x`, "https://" + strings.Repeat("a", MaxOpenURL),
		"C:\\Windows\\notepad.exe", "//example.com/",
	} {
		if err := (OpenURL{OpenID: testRefID, URL: raw}).Validate(); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("url %q: Validate = %v, want invalid_message", raw, err)
		}
	}
	for _, raw := range []string{"https://python.org", "HTTP://Example.com:8080/a?b=c#d", "https://xn--espaol-zwa.org/"} {
		if err := (OpenURL{OpenID: testRefID, URL: raw}).Validate(); err != nil {
			t.Errorf("url %q: Validate = %v, want valid", raw, err)
		}
	}
	app := OpenApp{OpenID: testRefID, AppID: "vscode", Name: "VS Code", Path: `C:\x.exe`}
	for _, p := range []Payload{
		OpenURL{OpenID: "x", URL: "https://python.org"},
		OpenApp{OpenID: "x", AppID: app.AppID, Name: app.Name, Path: app.Path},
		OpenApp{OpenID: testRefID, AppID: "VS Code", Name: app.Name, Path: app.Path},
		OpenApp{OpenID: testRefID, AppID: strings.Repeat("a", 33), Name: app.Name, Path: app.Path},
		OpenApp{OpenID: testRefID, AppID: app.AppID, Name: "  ", Path: app.Path},
		OpenApp{OpenID: testRefID, AppID: app.AppID, Name: strings.Repeat("ñ", MaxOpenAppName+1), Path: app.Path},
		OpenApp{OpenID: testRefID, AppID: app.AppID, Name: "a\nb", Path: app.Path},
		OpenApp{OpenID: testRefID, AppID: app.AppID, Name: app.Name, Path: ""},
		OpenApp{OpenID: testRefID, AppID: app.AppID, Name: app.Name, Path: strings.Repeat("a", MaxOpenAppPath+1)},
		OpenAck{RefID: testRefID, OpenID: testRefID, Status: OpenOpened, Reason: OpenBusy},
		OpenAck{RefID: testRefID, OpenID: testRefID, Status: OpenFailed},
		OpenAck{RefID: testRefID, OpenID: testRefID, Status: "closed"},
		OpenAck{RefID: "x", OpenID: testRefID, Status: OpenOpened},
	} {
		if err := p.Validate(); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%T %+v: Validate = %v, want invalid_message", p, p, err)
		}
	}
	if err := app.Validate(); err != nil {
		t.Errorf("a path outside the rules is still a valid message (the agent answers not_allowed): %v", err)
	}
}

func TestFileCollectLimits(t *testing.T) {
	sum := strings.Repeat("ab", 32)
	req := func(f func(*FileCollectRequest)) FileCollectRequest {
		p := FileCollectRequest{CollectID: testRefID, Date: "2026-11-06", Activity: "Ej", Round: 1,
			MaxFileSize: 1, MaxTotal: 1, MaxFiles: 1}
		f(&p)
		return p
	}
	resp := func(f func(*FileCollectResponse)) FileCollectResponse {
		p := FileCollectResponse{CollectID: testRefID, Round: 1, FileID: testSnippetID, Part: 1, Parts: 1,
			Data: []byte("x"), Path: "a.py", Size: 1, SHA256: sum, Index: 1, Files: 1}
		f(&p)
		return p
	}
	ack := func(f func(*FileCollectAck)) FileCollectAck {
		p := FileCollectAck{RefID: testRefID, CollectID: testRefID, Round: 1, Status: FileCollectSending, Path: "p", Files: 1, Total: 1}
		f(&p)
		return p
	}
	for _, p := range []Payload{
		FileCollectOpen{CollectID: "x", Date: "2026-11-06", Activity: "a"},
		FileCollectOpen{CollectID: testRefID, Date: "2026-13-01", Activity: "a"},
		FileCollectOpen{CollectID: testRefID, Date: "2026-1-01", Activity: "a"},
		FileCollectOpen{CollectID: testRefID, Date: "2026-11-06", Activity: ""},
		FileCollectOpen{CollectID: testRefID, Date: "2026-11-06", Activity: strings.Repeat("ñ", MaxCollectActivity+1)},
		FileCollectOpen{CollectID: testRefID, Date: "2026-11-06", Activity: "a", Patterns: []string{"src/*.py"}},
		FileCollectOpen{CollectID: testRefID, Date: "2026-11-06", Activity: "a", Patterns: []string{""}},
		FileCollectOpen{CollectID: testRefID, Date: "2026-11-06", Activity: "a", Patterns: make([]string, MaxCollectPatterns+1)},
		req(func(p *FileCollectRequest) { p.Round = 0 }),
		req(func(p *FileCollectRequest) { p.Round = MaxFileCollectRound + 1 }),
		req(func(p *FileCollectRequest) { p.MaxFileSize = MaxFileCollectBytes + 1 }),
		req(func(p *FileCollectRequest) { p.MaxTotal = 0 }),
		req(func(p *FileCollectRequest) { p.MaxTotal = MaxFileCollectTotal + 1 }),
		req(func(p *FileCollectRequest) { p.MaxFiles = MaxFileCollectFiles + 1 }),
		ack(func(p *FileCollectAck) { p.Status = "done" }),
		ack(func(p *FileCollectAck) { p.Files = 0 }),
		ack(func(p *FileCollectAck) { p.Path = "" }),
		ack(func(p *FileCollectAck) { p.Status = FileCollectReady }), // round 1 and files
		ack(func(p *FileCollectAck) { p.Status = FileCollectEmpty }), // files
		ack(func(p *FileCollectAck) { p.Status, p.Reason = FileCollectFailed, CollectBusy }),
		ack(func(p *FileCollectAck) { p.Skipped = []FileCollectSkipped{{Path: "a", Reason: CollectCorrupt}} }),
		ack(func(p *FileCollectAck) { p.Skipped = make([]FileCollectSkipped, MaxFileCollectFiles+1) }),
		FileCollectAck{RefID: testRefID, CollectID: testRefID, Round: 1, Status: FileCollectFailed, Reason: CollectTooLarge},
		resp(func(p *FileCollectResponse) { p.Round = 0 }),
		resp(func(p *FileCollectResponse) { p.Path = "" }),
		resp(func(p *FileCollectResponse) { p.Size = FileCollectPartSize + 1 }),
		resp(func(p *FileCollectResponse) { p.Size = 0 }),
		resp(func(p *FileCollectResponse) { p.SHA256 = strings.ToUpper(sum) }),
		resp(func(p *FileCollectResponse) { p.Index = 2 }),
		resp(func(p *FileCollectResponse) { p.Files = MaxFileCollectFiles + 1; p.Index = 1 }),
		resp(func(p *FileCollectResponse) { p.ModifiedAt = -1 }),
		FileCollectResponse{CollectID: testRefID, Round: 1, FileID: testSnippetID, Part: 2, Parts: 2},
		FileCollectResponseAck{RefID: testRefID, CollectID: testRefID, Round: 1, FileID: testSnippetID, Status: FilePushFailed, Reason: CollectBusy},
		FileCollectResponseAck{RefID: testRefID, CollectID: testRefID, Round: 1, FileID: testSnippetID, Status: FilePushSaved, Reason: CollectCorrupt},
		FileCollectResponseAck{RefID: testRefID, CollectID: testRefID, FileID: testSnippetID, Status: FilePushSaved},
	} {
		if err := p.Validate(); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%T %+v: Validate = %v, want invalid_message", p, p, err)
		}
	}
	big := FileCollectResponse{CollectID: testRefID, Round: 1, FileID: testSnippetID, Part: 2, Parts: 2, Data: make([]byte, FileCollectPartSize+1)}
	if err := big.Validate(); !errors.Is(err, ErrMessageTooLarge) {
		t.Errorf("oversized part: Validate = %v, want message_too_large", err)
	}
	ok := resp(func(p *FileCollectResponse) {
		p.Size, p.Parts, p.Data = MaxFileCollectBytes, MaxFileCollectParts, make([]byte, FileCollectPartSize)
		p.Index, p.Files = MaxFileCollectFiles, MaxFileCollectFiles
	})
	if err := ok.Validate(); err != nil {
		t.Errorf("the limits must be valid: %v", err)
	}
	okReq := req(func(p *FileCollectRequest) {
		p.MaxFileSize, p.MaxTotal, p.MaxFiles, p.Round = MaxFileCollectBytes, MaxFileCollectTotal, MaxFileCollectFiles, MaxFileCollectRound
		p.Patterns = []string{strings.Repeat("p", MaxCollectPattern)}
		p.Activity = strings.Repeat("ñ", MaxCollectActivity)
	})
	if err := okReq.Validate(); err != nil {
		t.Errorf("the request limits must be valid: %v", err)
	}
	// One full part with the envelope stays under the message limit.
	if b, err := Encode(New(ok)); err != nil || len(b) > MaxMessageSize {
		t.Errorf("a full part encodes to %d bytes (%v), limit %d", len(b), err, MaxMessageSize)
	}
}

const testSDP = "v=0\r\no=- 1 2 IN IP4 0.0.0.0\r\ns=-\r\nt=0 0\r\nm=video 9 UDP/TLS/RTP/SAVPF 102\r\na=sendonly\r\n"

func TestRoundTripEveryType(t *testing.T) {
	covered := map[Type]bool{}
	for _, p := range samplePayloads() {
		t.Run(string(p.MessageType()), func(t *testing.T) {
			in := New(p)
			data, err := Encode(in)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			out, err := Decode(data)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if !reflect.DeepEqual(in, out) {
				t.Fatalf("round trip mismatch:\n in: %#v\nout: %#v", in, out)
			}
		})
		covered[p.MessageType()] = true
	}
	for typ := range decoders {
		if !covered[typ] {
			t.Errorf("type %s has no round-trip sample", typ)
		}
	}
}

func TestTestdataValid(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "valid", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no valid testdata: %v", err)
	}
	covered := map[Type]bool{}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			e, err := Decode(data)
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if e.Payload.MessageType() != e.Type {
				t.Fatalf("payload %T does not match type %s", e.Payload, e.Type)
			}
			covered[e.Type] = true
			// Re-encoding a decoded example must succeed and decode to the same value.
			again, err := Encode(e)
			if err != nil {
				t.Fatalf("re-Encode: %v", err)
			}
			e2, err := Decode(again)
			if err != nil || !reflect.DeepEqual(e, e2) {
				t.Fatalf("re-Decode mismatch (err %v)", err)
			}
		})
	}
	for typ := range decoders {
		if !covered[typ] {
			t.Errorf("type %s has no example in testdata/valid", typ)
		}
	}
}

// Files in testdata/invalid are named "<expected error code>[-description].json".
func TestTestdataInvalid(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "invalid", "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no invalid testdata: %v", err)
	}
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".json")
		want := ErrorCode(strings.SplitN(name, "-", 2)[0])
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Decode(data)
			if err == nil {
				t.Fatal("Decode succeeded, want error")
			}
			if got := ErrorCodeFor(err); got != want {
				t.Fatalf("ErrorCodeFor = %s, want %s (err: %v)", got, want, err)
			}
		})
	}
}

func TestDecodeUnsupportedVersionKeepsID(t *testing.T) {
	data := []byte(`{"v":2,"id":"5b0e8f3c-2a6d-4c1e-9b7a-0f3d2e1c4b5a","ts":1,"type":"heartbeat","payload":{}}`)
	e, err := Decode(data)
	if !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("err = %v, want ErrUnsupportedVersion", err)
	}
	if e.ID != "5b0e8f3c-2a6d-4c1e-9b7a-0f3d2e1c4b5a" || e.V != 2 {
		t.Fatalf("header not preserved: %+v", e)
	}
}

func TestEncodeUnsupportedVersion(t *testing.T) {
	e := New(Heartbeat{})
	e.V = 2
	if _, err := Encode(e); !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("err = %v, want ErrUnsupportedVersion", err)
	}
}

func TestDecodeUnknownTypeKeepsHeader(t *testing.T) {
	data := []byte(`{"v":1,"id":"5b0e8f3c-2a6d-4c1e-9b7a-0f3d2e1c4b5a","ts":1,"type":"poll.start","payload":{"q":1}}`)
	e, err := Decode(data)
	if !errors.Is(err, ErrUnknownType) {
		t.Fatalf("err = %v, want ErrUnknownType", err)
	}
	if e.ID == "" || e.Type != "poll.start" || e.Payload != nil {
		t.Fatalf("unexpected envelope: %+v", e)
	}
}

func TestDecodeDoesNotEchoMalformedID(t *testing.T) {
	data := []byte(`{"v":1,"id":"not-a-uuid","ts":1,"type":"heartbeat","payload":{}}`)
	e, err := Decode(data)
	if !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("err = %v, want ErrInvalidMessage", err)
	}
	if e.ID != "" {
		t.Fatalf("malformed ID echoed: %q", e.ID)
	}
}

func TestDecodeTooLarge(t *testing.T) {
	data := make([]byte, MaxMessageSize+1)
	if _, err := Decode(data); !errors.Is(err, ErrMessageTooLarge) {
		t.Fatalf("err = %v, want ErrMessageTooLarge", err)
	}
}

func TestEncodeContentLimit(t *testing.T) {
	share := func(content string) CodeShare {
		return CodeShare{SnippetID: testSnippetID, Lang: "plaintext", Content: content, Mode: ModeReplace}
	}
	if _, err := Encode(New(share(strings.Repeat("a", MaxCodeContent)))); err != nil {
		t.Fatalf("content at limit: %v", err)
	}
	if _, err := Encode(New(share(strings.Repeat("a", MaxCodeContent+1)))); !errors.Is(err, ErrMessageTooLarge) {
		t.Fatalf("content over limit: err = %v, want ErrMessageTooLarge", err)
	}
	// HTML characters are not escaped, so code full of them still fits.
	if _, err := Encode(New(share(strings.Repeat("<&>", MaxCodeContent/3)))); err != nil {
		t.Fatalf("HTML-heavy content at limit: %v", err)
	}
	// Within the content limit, but JSON escaping (\u0001 is 6 bytes) pushes the
	// envelope past MaxMessageSize.
	if _, err := Encode(New(share(strings.Repeat("\x01", MaxCodeContent)))); !errors.Is(err, ErrMessageTooLarge) {
		t.Fatalf("escaped envelope over limit: err = %v, want ErrMessageTooLarge", err)
	}
}

func TestDecodeInvalidJSON(t *testing.T) {
	for _, in := range []string{``, `nul`, `[]`, `"hello"`, `{"v":1,`, `{"v":"1"}`} {
		if _, err := Decode([]byte(in)); ErrorCodeFor(err) != CodeInvalidMessage && ErrorCodeFor(err) != CodeUnsupportedVersion {
			t.Errorf("Decode(%q): err = %v, want invalid_message", in, err)
		}
	}
	if _, err := Decode([]byte(`{"v":1,`)); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("truncated JSON: err = %v, want ErrInvalidMessage", err)
	}
}

func TestEncodeRejectsInconsistentEnvelope(t *testing.T) {
	e := New(Heartbeat{})
	e.Type = TypeMsgAck
	if _, err := Encode(e); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("type mismatch: err = %v, want ErrInvalidMessage", err)
	}
	e = New(Heartbeat{})
	e.Payload = nil
	if _, err := Encode(e); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("nil payload: err = %v, want ErrInvalidMessage", err)
	}
	e = New(Heartbeat{})
	e.ID = "abc"
	if _, err := Encode(e); !errors.Is(err, ErrInvalidMessage) {
		t.Fatalf("bad id: err = %v, want ErrInvalidMessage", err)
	}
}

func TestPayloadValidation(t *testing.T) {
	share := samplePayloads()[6].(CodeShare)
	hello := samplePayloads()[3].(HelloInfo)
	tests := []struct {
		name string
		p    Payload
	}{
		{"code.share invalid utf8", func() Payload { s := share; s.Content = "\xff"; return s }()},
		{"code.share bad lang", func() Payload { s := share; s.Lang = "Python 3"; return s }()},
		{"code.share long title", func() Payload { s := share; s.Title = strings.Repeat("é", MaxTitle+1); return s }()},
		{"code.share bad snippet id", func() Payload { s := share; s.SnippetID = ""; return s }()},
		{"hello bad os", func() Payload { h := hello; h.OS = "plan9"; return h }()},
		{"hello bad version", func() Payload { h := hello; h.AgentVersion = "v0.1"; return h }()},
		{"hello empty hostname", func() Payload { h := hello; h.Hostname = ""; return h }()},
		{"hello malformed capability", func() Payload { h := hello; h.Capabilities = []Capability{"Code.View"}; return h }()},
		{"msg.show empty text", MsgShow{Text: "", Level: LevelInfo}},
		{"msg.show bad level", MsgShow{Text: "hola", Level: "error"}},
		{"auth.proof short signature", AuthProof{RefID: testRefID, Signature: []byte{1}}},
		{"error bad code", ErrorPayload{Code: "Bad Code"}},
		{"code.ack bad ref", CodeAck{RefID: "x"}},
		{"thumb.start too fast", ScreenThumbStart{IntervalMS: MinThumbInterval - 1, Quality: 50, MaxWidth: 320}},
		{"thumb.start slow", ScreenThumbStart{IntervalMS: MaxThumbInterval + 1, Quality: 50, MaxWidth: 320}},
		{"thumb.start quality", ScreenThumbStart{IntervalMS: 1000, Quality: 100, MaxWidth: 320}},
		{"thumb.start width", ScreenThumbStart{IntervalMS: 1000, Quality: 50, MaxWidth: 4000}},
		{"thumb.frame seq", ScreenThumbFrame{RefID: testRefID, Seq: 0, Width: 1, Height: 1, JPEG: []byte{1}}},
		{"thumb.frame empty", ScreenThumbFrame{RefID: testRefID, Seq: 1, Width: 1, Height: 1}},
		{"thumb.frame size", ScreenThumbFrame{RefID: testRefID, Seq: 1, Width: 0, Height: 1, JPEG: []byte{1}}},
		{"broadcast.start id", ScreenBroadcastStart{BroadcastID: "x", Width: 1920, Height: 1080, FPS: 15}},
		{"broadcast.start size", ScreenBroadcastStart{BroadcastID: testRefID, Width: 5000, Height: 1080, FPS: 15}},
		{"broadcast.start fps", ScreenBroadcastStart{BroadcastID: testRefID, Width: 1920, Height: 1080, FPS: 0}},
		{"broadcast.start mode", ScreenBroadcastStart{BroadcastID: testRefID, Width: 1920, Height: 1080, FPS: 15, Mode: "kiosk"}},
		{"broadcast.offer attempt", ScreenBroadcastOffer{BroadcastID: testRefID, Attempt: MaxBroadcastAttempt + 1, SDP: testSDP}},
		{"broadcast.offer not sdp", ScreenBroadcastOffer{BroadcastID: testRefID, Attempt: 1, SDP: "hola"}},
		{"broadcast.answer ref", ScreenBroadcastAnswer{RefID: "", BroadcastID: testRefID, Attempt: 1, SDP: testSDP}},
		{"broadcast.ice prefix", ScreenBroadcastICE{BroadcastID: testRefID, Attempt: 1, Candidate: "a=candidate:1"}},
		{"broadcast.ice long", ScreenBroadcastICE{BroadcastID: testRefID, Attempt: 1, Candidate: "candidate:" + strings.Repeat("1", MaxICECandidate)}},
		{"broadcast.ice mid", ScreenBroadcastICE{BroadcastID: testRefID, Attempt: 1, SDPMid: strings.Repeat("m", MaxSDPMid+1)}},
		{"broadcast.ice index", ScreenBroadcastICE{BroadcastID: testRefID, Attempt: 1, SDPMLineIndex: MaxSDPMLineIndex + 1}},
		{"broadcast.stop id", ScreenBroadcastStop{}},
		{"app.block no apps", AppBlockSet{PolicyID: testRefID, Mode: PolicyDeny}},
		{"app.block mode", AppBlockSet{PolicyID: testRefID, Mode: "block", Apps: []string{"steam"}}},
		{"app.block path", AppBlockSet{PolicyID: testRefID, Mode: PolicyDeny, Apps: []string{`C:\juegos\juego.exe`}}},
		{"app.block long", AppBlockSet{PolicyID: testRefID, Mode: PolicyDeny, Apps: []string{strings.Repeat("a", MaxAppName+1)}}},
		{"app.block dup", AppBlockSet{PolicyID: testRefID, Mode: PolicyDeny, Apps: []string{"chrome", "Chrome.exe"}}},
		{"app.block too many", AppBlockSet{PolicyID: testRefID, Mode: PolicyDeny, Apps: manyApps(MaxBlockedApps + 1)}},
		{"web.filter url", WebFilterSet{PolicyID: testRefID, Mode: PolicyDeny, Domains: []string{"https://x.com/a"}}},
		{"web.filter port", WebFilterSet{PolicyID: testRefID, Mode: PolicyDeny, Domains: []string{"x.com:443"}}},
		{"web.filter single label", WebFilterSet{PolicyID: testRefID, Mode: PolicyDeny, Domains: []string{"localhost"}}},
		{"web.filter accents", WebFilterSet{PolicyID: testRefID, Mode: PolicyDeny, Domains: []string{"español.org"}}},
		{"web.filter dup", WebFilterSet{PolicyID: testRefID, Mode: PolicyDeny, Domains: []string{"x.com", "X.COM."}}},
		{"web.filter id", WebFilterSet{Mode: PolicyDeny, Domains: []string{"x.com"}}},
		{"ack active without policy", AppBlockAck{RefID: testRefID, Active: true}},
		{"ack inactive with policy", WebFilterAck{RefID: testRefID, PolicyID: testRefID}},
		{"power action", PowerAction{ActionID: testRefID, Action: "hibernate"}},
		{"power id", PowerAction{Action: PowerReboot}},
		{"power delay short", PowerAction{ActionID: testRefID, Action: PowerReboot, DelayS: MinPowerDelay - 1}},
		{"power delay long", PowerAction{ActionID: testRefID, Action: PowerReboot, DelayS: MaxPowerDelay + 1}},
		{"power delay negative", PowerAction{ActionID: testRefID, Action: PowerReboot, DelayS: -5}},
		{"power cancel id", PowerCancel{ActionID: "x"}},
		{"power ack status", PowerAck{RefID: testRefID, ActionID: testRefID, Status: "done"}},
		{"power ack postpones", PowerAck{RefID: testRefID, ActionID: testRefID, Status: PowerPostponed, PostponesLeft: 3}},
		{"power ack remaining", PowerAck{RefID: testRefID, ActionID: testRefID, Status: PowerCountdown, RemainingS: -1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.p.Validate(); !errors.Is(err, ErrInvalidMessage) {
				t.Fatalf("Validate = %v, want ErrInvalidMessage", err)
			}
		})
	}
}

func TestNewErrorTruncates(t *testing.T) {
	p := NewError("", CodeInternal, strings.Repeat("ñ", MaxErrorMessage+10))
	if err := p.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestNewID(t *testing.T) {
	seen := map[string]bool{}
	for range 1000 {
		id := NewID()
		if !isUUIDv4(id) {
			t.Fatalf("NewID() = %q is not a UUID v4", id)
		}
		if seen[id] {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = true
	}
}

func TestRequiredCapability(t *testing.T) {
	if c, ok := RequiredCapability(TypeCodeShare); !ok || c != CapCodeView {
		t.Fatalf("code.share requires %q, %v", c, ok)
	}
	for typ, want := range map[Type]Capability{
		TypeScreenThumbStart: CapScreenThumb, TypeScreenThumbStop: CapScreenThumb,
		TypeScreenFreeze: CapScreenFreeze, TypeScreenUnfreeze: CapScreenFreeze,
		TypeScreenBroadcastStart: CapScreenBroadcast, TypeScreenBroadcastOffer: CapScreenBroadcast,
		TypeScreenBroadcastICE: CapScreenBroadcast, TypeScreenBroadcastStop: CapScreenBroadcast,
		TypeAppBlockSet: CapAppBlock, TypeAppBlockClear: CapAppBlock,
		TypeWebFilterSet: CapWebFilter, TypeWebFilterClear: CapWebFilter,
		TypePowerAction: CapPower, TypePowerCancel: CapPower,
		TypeRemoteStart: CapRemote, TypeRemoteInput: CapRemote, TypeRemoteStop: CapRemote,
		TypeScreenSnapshotRequest: CapScreenSnapshot,
		TypeHandLower:             CapHand, TypeMsgStudentAck: CapMsgStudent,
		TypeTimerStart: CapTimer, TypeTimerExtend: CapTimer, TypeTimerStop: CapTimer,
		TypeOpenURL: CapOpenURL, TypeOpenApp: CapOpenApp,
		TypePollOpen: CapPoll, TypePollVoteAck: CapPoll, TypePollClose: CapPoll, TypePollResults: CapPoll,
		TypeStudentPresentStart: CapScreenPresent, TypeStudentPresentOffer: CapScreenPresent,
		TypeStudentPresentICE: CapScreenPresent, TypeStudentPresentStop: CapScreenPresent,
		TypeClassEnd: CapClassEnd,
	} {
		if c, ok := RequiredCapability(typ); !ok || c != want {
			t.Errorf("%s requires %q, %v; want %q", typ, c, ok, want)
		}
	}
	for _, typ := range []Type{TypeScreenThumbFrame, TypeScreenBroadcastAnswer, TypeAppBlockAck, TypeWebFilterAck, TypePowerAck, TypeRemoteAck, TypeScreenSnapshotResponse, TypeTimerAck, TypeOpenAck, TypePollAck, TypePollVote, TypeStudentPresentAck, TypeStudentPresentAnswer, TypeScreenFreezeAck, TypeClassEndAck} {
		if _, ok := RequiredCapability(typ); ok {
			t.Errorf("%s goes to the console and must not require a capability", typ)
		}
	}
	for typ, want := range map[Type]Capability{TypeHandRaise: CapHand, TypeHandLower: CapHand, TypeMsgStudent: CapMsgStudent, TypePollVote: CapPoll} {
		if c, ok := AgentCapability(typ); !ok || c != want {
			t.Errorf("agent-started %s requires %q, %v; want %q", typ, c, ok, want)
		}
	}
	if _, ok := AgentCapability(TypeCodeAck); ok {
		t.Error("code.ack answers the console and must not require an agent capability")
	}
	if _, ok := RequiredCapability(TypeHeartbeat); ok {
		t.Fatal("heartbeat must not require a capability")
	}
	if !Has([]Capability{CapMsgShow}, CapMsgShow) || Has(nil, CapCodeView) {
		t.Fatal("Has misbehaves")
	}
}

func TestValidChannel(t *testing.T) {
	for _, ok := range []string{"1", "2", "12", "999", "16000"} {
		if !ValidChannel(ok) {
			t.Errorf("ValidChannel(%q) = false", ok)
		}
	}
	for _, bad := range []string{"", "0", "01", "16001", "99999", "-1", "1.5", " 1", "AULA_101", strings.Repeat("1", 6)} {
		if ValidChannel(bad) {
			t.Errorf("ValidChannel(%q) = true", bad)
		}
	}
	for in, want := range map[string]string{"1": "1", " 02 ": "2", "16000": "16000"} {
		if got, err := ParseChannel(in); err != nil || got != want {
			t.Errorf("ParseChannel(%q) = %q, %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "0", "16001", "uno", "AULA_101"} {
		if _, err := ParseChannel(bad); err == nil {
			t.Errorf("ParseChannel(%q) accepted", bad)
		}
	}
}

func FuzzDecode(f *testing.F) {
	files, _ := filepath.Glob(filepath.Join("testdata", "*", "*.json"))
	for _, file := range files {
		if data, err := os.ReadFile(file); err == nil {
			f.Add(data)
		}
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		e, err := Decode(data)
		if err != nil {
			return
		}
		// Anything Decode accepts must encode again.
		if _, err := Encode(e); err != nil {
			t.Fatalf("Encode after successful Decode: %v", err)
		}
	})
}

func TestFrameJPEGLimit(t *testing.T) {
	big := ScreenThumbFrame{RefID: testRefID, Seq: 1, Width: 1, Height: 1, JPEG: make([]byte, MaxFrameJPEG+1)}
	if err := big.Validate(); !errors.Is(err, ErrMessageTooLarge) {
		t.Fatalf("Validate = %v, want ErrMessageTooLarge", err)
	}
	// The largest frame still fits in one envelope once Base64-encoded.
	big.JPEG = big.JPEG[:MaxFrameJPEG]
	if _, err := Encode(New(big)); err != nil {
		t.Fatalf("Encode of a %d-byte frame: %v", MaxFrameJPEG, err)
	}
}

func TestSDPLimit(t *testing.T) {
	big := ScreenBroadcastOffer{BroadcastID: testRefID, Attempt: 1, SDP: "v=0" + strings.Repeat("x", MaxSDP)}
	if err := big.Validate(); !errors.Is(err, ErrMessageTooLarge) {
		t.Fatalf("Validate = %v, want ErrMessageTooLarge", err)
	}
	big.SDP = big.SDP[:MaxSDP]
	if _, err := Encode(New(big)); err != nil {
		t.Fatalf("Encode of a %d-byte SDP: %v", MaxSDP, err)
	}
}

func manyApps(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = "app" + strconv.Itoa(i)
	}
	return out
}

func TestPolicyKeys(t *testing.T) {
	for in, want := range map[string]string{"Chrome.EXE": "chrome", " steam ": "steam", "a.exe.exe": "a.exe"} {
		if got := AppKey(in); got != want {
			t.Errorf("AppKey(%q) = %q, want %q", in, got, want)
		}
	}
	for _, ok := range []string{"python.org", "Docs.Python.org.", "a-b.c-d.io", "xn--espaol-zwa.org"} {
		if !ValidDomain(ok) {
			t.Errorf("ValidDomain(%q) = false", ok)
		}
	}
	for _, bad := range []string{"", "org", "-a.com", "a-.com", "a..com", "a_b.com", "*.x.com", strings.Repeat("a", 64) + ".com"} {
		if ValidDomain(bad) {
			t.Errorf("ValidDomain(%q) = true", bad)
		}
	}
}

func TestRemoteValidation(t *testing.T) {
	ev := func(e InputEvent) RemoteInput {
		return RemoteInput{ControlID: testRefID, Seq: 1, Events: []InputEvent{e}}
	}
	for name, p := range map[string]Payload{
		"start id":        RemoteStart{IntervalMS: 125, Quality: 60, MaxWidth: 1280},
		"start too fast":  RemoteStart{ControlID: testRefID, IntervalMS: MinRemoteInterval - 1, Quality: 60, MaxWidth: 1280},
		"start too slow":  RemoteStart{ControlID: testRefID, IntervalMS: MaxRemoteInterval + 1, Quality: 60, MaxWidth: 1280},
		"start quality":   RemoteStart{ControlID: testRefID, IntervalMS: 125, Quality: 5, MaxWidth: 1280},
		"start width":     RemoteStart{ControlID: testRefID, IntervalMS: 125, Quality: 60, MaxWidth: 320},
		"input seq":       RemoteInput{ControlID: testRefID, Events: []InputEvent{{T: InputMove}}},
		"input empty":     RemoteInput{ControlID: testRefID, Seq: 1},
		"input too many":  RemoteInput{ControlID: testRefID, Seq: 1, Events: make([]InputEvent, MaxRemoteEvents+1)},
		"input type":      ev(InputEvent{T: "touch"}),
		"move out":        ev(InputEvent{T: InputMove, X: 1.5}),
		"move negative":   ev(InputEvent{T: InputMove, Y: -0.1}),
		"button name":     ev(InputEvent{T: InputButton, B: "back"}),
		"button out":      ev(InputEvent{T: InputButton, B: ButtonLeft, X: 2}),
		"wheel zero":      ev(InputEvent{T: InputWheel}),
		"wheel big":       ev(InputEvent{T: InputWheel, DY: MaxRemoteWheel + 1}),
		"key unknown":     ev(InputEvent{T: InputKey, Code: "MetaLeft"}),
		"key empty":       ev(InputEvent{T: InputKey}),
		"stop id":         RemoteStop{},
		"ack status":      RemoteAck{RefID: testRefID, ControlID: testRefID, Status: "asking"},
		"ack no reason":   RemoteAck{RefID: testRefID, ControlID: testRefID, Status: RemoteEnded},
		"ack bad reason":  RemoteAck{RefID: testRefID, ControlID: testRefID, Status: RemoteEnded, Reason: "student"},
		"ack reason only": RemoteAck{RefID: testRefID, ControlID: testRefID, Status: RemotePaused, Reason: RemoteIdle},
	} {
		t.Run(name, func(t *testing.T) {
			if err := p.Validate(); !errors.Is(err, ErrInvalidMessage) {
				t.Fatalf("Validate = %v, want ErrInvalidMessage", err)
			}
		})
	}
	full := make([]InputEvent, MaxRemoteEvents)
	for i := range full {
		full[i] = InputEvent{T: InputMove, X: 1, Y: 0}
	}
	if err := (RemoteInput{ControlID: testRefID, Seq: 1, Events: full}).Validate(); err != nil {
		t.Fatalf("a full batch must be valid: %v", err)
	}
	for _, c := range KeyCodes {
		if !ValidKeyCode(c) {
			t.Fatalf("%s must be valid", c)
		}
	}
}

func TestHandAndStudentMessageLimits(t *testing.T) {
	for _, p := range []Payload{
		HandRaise{HandID: "nope"},
		HandRaise{HandID: testRefID, Note: strings.Repeat("a", MaxHandNote+1)},
		HandLower{HandID: testRefID, By: "nobody"},
		HandLower{HandID: "", By: HandByStudent},
		MsgStudent{MessageID: testRefID, Text: ""},
		MsgStudent{MessageID: testRefID, Text: strings.Repeat("ñ", MaxStudentMsgText+1)},
		MsgStudent{MessageID: "x", Text: "hola"},
		MsgStudentAck{RefID: testRefID, MessageID: "x"},
		MsgShow{Text: "hola", Level: LevelInfo, ReplyTo: "x"},
	} {
		if err := p.Validate(); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%T %+v: Validate = %v, want invalid_message", p, p, err)
		}
	}
	if err := (MsgStudent{MessageID: testRefID, Text: strings.Repeat("ñ", MaxStudentMsgText)}).Validate(); err != nil {
		t.Errorf("500 characters rejected: %v", err)
	}
	low := HandLower{HandID: testRefID, By: HandByTeacher}
	if low.ValidFrom(HandByTeacher) != nil || !errors.Is(low.ValidFrom(HandByStudent), ErrInvalidMessage) {
		t.Error("ValidFrom does not check the direction")
	}
}

func TestTimerLimits(t *testing.T) {
	for _, p := range []Payload{
		TimerStart{TimerID: "x", DurationS: 60, RemainingMS: 60000},
		TimerStart{TimerID: testRefID, Label: strings.Repeat("ñ", MaxTimerLabel+1), DurationS: 60, RemainingMS: 60000},
		TimerStart{TimerID: testRefID, DurationS: 60, RemainingMS: 60000, Description: strings.Repeat("ñ", MaxTimerDescription+1)},
		TimerStart{TimerID: testRefID, DurationS: MinTimerDurationS - 1, RemainingMS: 9000},
		TimerStart{TimerID: testRefID, DurationS: MaxTimerDurationS + 1, RemainingMS: 60000},
		TimerStart{TimerID: testRefID, DurationS: 60, RemainingMS: 0},
		TimerStart{TimerID: testRefID, DurationS: 60, RemainingMS: MaxTimerRemaining + 1},
		TimerExtend{TimerID: testRefID, AddS: MinTimerExtendS - 1, RemainingMS: 60000},
		TimerExtend{TimerID: testRefID, AddS: MaxTimerExtendS + 1, RemainingMS: 60000},
		TimerExtend{TimerID: testRefID, AddS: 60, RemainingMS: 0},
		TimerStop{TimerID: ""},
		TimerAck{RefID: testRefID, TimerID: testRefID, Status: "paused"},
		TimerAck{RefID: testRefID, TimerID: testRefID, Status: TimerRunning, RemainingMS: -1},
		TimerAck{RefID: "x", TimerID: testRefID, Status: TimerEnded},
	} {
		if err := p.Validate(); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%T %+v: Validate = %v, want invalid_message", p, p, err)
		}
	}
	ok := TimerStart{TimerID: testRefID, Label: strings.Repeat("ñ", MaxTimerLabel), DurationS: MaxTimerDurationS, RemainingMS: MaxTimerRemaining}
	if err := ok.Validate(); err != nil {
		t.Errorf("the limits must be valid: %v", err)
	}
}

func TestFilePushLimits(t *testing.T) {
	sum := strings.Repeat("ab", 32)
	first := func(f func(*FilePush)) FilePush {
		p := FilePush{PushID: testRefID, FileID: testSnippetID, Part: 1, Parts: 1, Data: []byte("x"),
			Name: "a.txt", Size: 1, SHA256: sum, Index: 1, Files: 1}
		f(&p)
		return p
	}
	for _, p := range []Payload{
		first(func(p *FilePush) { p.PushID = "x" }),
		first(func(p *FilePush) { p.FileID = "" }),
		first(func(p *FilePush) { p.Part = 2 }),
		first(func(p *FilePush) { p.Parts = MaxFilePushParts + 1; p.Size = MaxFilePushBytes + 1 }),
		first(func(p *FilePush) { p.Name = "" }),
		first(func(p *FilePush) { p.Name = strings.Repeat("ñ", MaxFilePushName+1) }),
		first(func(p *FilePush) { p.Dest = strings.Repeat("d", MaxFilePushDest+1) }),
		first(func(p *FilePush) { p.Label = strings.Repeat("l", MaxFilePushLabel+1) }),
		first(func(p *FilePush) { p.Size = -1 }),
		first(func(p *FilePush) { p.Size = FilePushPartSize + 1 }), // needs 2 parts
		first(func(p *FilePush) { p.Size = 0 }),                    // empty file with data
		first(func(p *FilePush) { p.Data = nil }),                  // size 1 without data
		first(func(p *FilePush) { p.SHA256 = strings.ToUpper(sum) }),
		first(func(p *FilePush) { p.Files = 0 }),
		first(func(p *FilePush) { p.Index = 2 }),
		first(func(p *FilePush) { p.Files = MaxFilePushFiles + 1; p.Index = 1 }),
		first(func(p *FilePush) { p.Name = "a\xffb" }),
		FilePush{PushID: testRefID, FileID: testSnippetID, Part: 2, Parts: 2},
		FilePushAck{RefID: testRefID, PushID: testRefID, FileID: testSnippetID, Status: "kept", Path: "a"},
		FilePushAck{RefID: testRefID, PushID: testRefID, FileID: testSnippetID, Status: FilePushSaved},
		FilePushAck{RefID: testRefID, PushID: testRefID, FileID: testSnippetID, Status: FilePushSaved, Path: "a", Reason: FilePushBusy},
		FilePushAck{RefID: testRefID, PushID: testRefID, FileID: testSnippetID, Status: FilePushSaved, Path: "a", Renamed: true, Replaced: true},
		FilePushAck{RefID: testRefID, PushID: testRefID, FileID: testSnippetID, Status: FilePushFailed},
		FilePushAck{RefID: testRefID, PushID: testRefID, FileID: testSnippetID, Status: FilePushFailed, Reason: FilePushBusy, Path: "a"},
		FilePushAck{RefID: "x", PushID: testRefID, FileID: testSnippetID, Status: FilePushFailed, Reason: FilePushBusy},
	} {
		if err := p.Validate(); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%T %+v: Validate = %v, want invalid_message", p, p, err)
		}
	}
	big := FilePush{PushID: testRefID, FileID: testSnippetID, Part: 2, Parts: 2, Data: make([]byte, FilePushPartSize+1)}
	if err := big.Validate(); !errors.Is(err, ErrMessageTooLarge) {
		t.Errorf("oversized part: Validate = %v, want message_too_large", err)
	}
	ok := first(func(p *FilePush) {
		p.Size, p.Parts, p.Data = MaxFilePushBytes, MaxFilePushParts, make([]byte, FilePushPartSize)
		p.Name = strings.Repeat("ñ", MaxFilePushName)
		p.Dest = strings.Repeat("d", MaxFilePushDest)
		p.Label = strings.Repeat("l", MaxFilePushLabel)
		p.Index, p.Files = MaxFilePushFiles, MaxFilePushFiles
	})
	if err := ok.Validate(); err != nil {
		t.Errorf("the limits must be valid: %v", err)
	}
	// A whole part of the largest size fits in one message.
	if _, err := Encode(New(ok)); err != nil {
		t.Errorf("a full part must fit in a message: %v", err)
	}
	if c, has := RequiredCapability(TypeFilePush); !has || c != CapFilePush {
		t.Errorf("file.push requires %q, %v", c, has)
	}
	if _, has := RequiredCapability(TypeFilePushAck); has {
		t.Error("file.push.ack goes to the console and must not require a capability")
	}
}

func TestPollLimits(t *testing.T) {
	six := []string{"a", "b", "c", "d", "e", "f"}
	for _, p := range []Payload{
		PollOpen{PollID: "x", Kind: PollKindYesNo, Question: "¿Sí?"},
		PollOpen{PollID: testRefID, Kind: "rating", Question: "¿Sí?"},
		PollOpen{PollID: testRefID, Kind: PollKindYesNo, Question: "   "},
		PollOpen{PollID: testRefID, Kind: PollKindYesNo, Question: strings.Repeat("ñ", MaxPollQuestion+1)},
		PollOpen{PollID: testRefID, Kind: PollKindChoice, Question: "¿?", Options: []string{"solo una"}},
		PollOpen{PollID: testRefID, Kind: PollKindChoice, Question: "¿?", Options: append(six, "g")},
		PollOpen{PollID: testRefID, Kind: PollKindChoice, Question: "¿?", Options: []string{"a", " "}},
		PollOpen{PollID: testRefID, Kind: PollKindChoice, Question: "¿?", Options: []string{"a", strings.Repeat("ñ", MaxPollOption+1)}},
		PollOpen{PollID: testRefID, Kind: PollKindYesNo, Question: "¿?", Options: []string{"Sí", "No"}},
		PollOpen{PollID: testRefID, Kind: PollKindOpen, Question: "¿?", Options: []string{"a", "b"}},
		PollVote{PollID: testRefID},
		PollVote{PollID: testRefID, Choice: intp(1), Text: "y"},
		PollVote{PollID: testRefID, Choice: intp(-1)},
		PollVote{PollID: testRefID, Choice: intp(MaxPollOptions)},
		PollVote{PollID: testRefID, Text: "  "},
		PollVote{PollID: testRefID, Text: strings.Repeat("ñ", MaxPollText+1)},
		PollVoteAck{RefID: testRefID, PollID: testRefID, Status: "duplicate"},
		PollClose{PollID: ""},
		PollResults{PollID: testRefID, Counts: []int{1}, Voters: 1, Recipients: 1},
		PollResults{PollID: testRefID, Counts: []int{1, 2}, Voters: 4, Recipients: 4},
		PollResults{PollID: testRefID, Counts: []int{1, -1}, Voters: 0, Recipients: 0},
		PollResults{PollID: testRefID, Counts: []int{1, 2}, Voters: 3, Recipients: 2},
		PollAck{RefID: testRefID, PollID: testRefID, Status: "voted"},
	} {
		if err := p.Validate(); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%T %+v: Validate = %v, want invalid_message", p, p, err)
		}
	}
	for _, p := range []Payload{
		PollOpen{PollID: testRefID, Kind: PollKindChoice, Question: strings.Repeat("ñ", MaxPollQuestion), Options: []string{"a", strings.Repeat("ñ", MaxPollOption)}},
		PollOpen{PollID: testRefID, Kind: PollKindChoice, Question: "¿?", Options: six},
		PollVote{PollID: testRefID, Choice: intp(MaxPollOptions - 1)},
		PollVote{PollID: testRefID, Text: strings.Repeat("ñ", MaxPollText)},
		PollResults{PollID: testRefID, Counts: []int{MaxPollVoters, 0}, Voters: MaxPollVoters, Recipients: MaxPollVoters},
	} {
		if err := p.Validate(); err != nil {
			t.Errorf("%T: the limits must be valid: %v", p, err)
		}
	}
}

func TestPollVoteValidFor(t *testing.T) {
	opts := []string{"a", "b", "c"}
	for _, c := range []struct {
		vote PollVote
		kind PollKind
		ok   bool
	}{
		{PollVote{Choice: intp(2)}, PollKindChoice, true},
		{PollVote{Choice: intp(3)}, PollKindChoice, false},
		{PollVote{Text: "x"}, PollKindChoice, false},
		{PollVote{Choice: intp(1)}, PollKindYesNo, true},
		{PollVote{Choice: intp(2)}, PollKindYesNo, false},
		{PollVote{Text: "x"}, PollKindOpen, true},
		{PollVote{Choice: intp(0)}, PollKindOpen, false},
	} {
		var o []string
		if c.kind == PollKindChoice {
			o = opts
		}
		if err := c.vote.ValidFor(c.kind, o); (err == nil) != c.ok {
			t.Errorf("%+v for %s: %v, want ok=%v", c.vote, c.kind, err, c.ok)
		}
	}
}

func TestStudentPresentLimits(t *testing.T) {
	for _, p := range []Payload{
		StudentPresentStart{PresentID: "x", MaxWidth: 1920, FPS: 15, MaxKbps: 2500},
		StudentPresentStart{PresentID: testRefID, MaxWidth: MinPresentWidth - 1, FPS: 15, MaxKbps: 2500},
		StudentPresentStart{PresentID: testRefID, MaxWidth: MaxPresentWidth + 1, FPS: 15, MaxKbps: 2500},
		StudentPresentStart{PresentID: testRefID, MaxWidth: 1920, FPS: MaxPresentFPS + 1, MaxKbps: 2500},
		StudentPresentStart{PresentID: testRefID, MaxWidth: 1920, FPS: 15, MaxKbps: MinPresentKbps - 1},
		StudentPresentAck{RefID: testRefID, PresentID: testRefID, Status: "paused"},
		StudentPresentAck{RefID: testRefID, PresentID: testRefID, Status: PresentAccepted},
		StudentPresentAck{RefID: testRefID, PresentID: testRefID, Status: PresentAccepted, Width: 1921, Height: 1080},
		StudentPresentAck{RefID: testRefID, PresentID: testRefID, Status: PresentEnded, Width: 2, Height: 2},
		StudentPresentAck{RefID: testRefID, PresentID: testRefID, Status: "asking"},
		StudentPresentAck{RefID: testRefID, PresentID: testRefID, Status: "declined"},
		StudentPresentAck{RefID: testRefID, PresentID: testRefID, Status: PresentFailed},
		StudentPresentAck{RefID: testRefID, PresentID: testRefID, Status: PresentEnded, Reason: PresentBusy},
		StudentPresentOffer{PresentID: testRefID, Attempt: 0, SDP: "v=0"},
		StudentPresentOffer{PresentID: testRefID, Attempt: 1, SDP: "hola"},
		StudentPresentAnswer{RefID: "x", PresentID: testRefID, Attempt: 1, SDP: "v=0"},
		StudentPresentICE{PresentID: testRefID, Attempt: 1, Candidate: "a=candidate:1"},
		StudentPresentICE{PresentID: testRefID, Attempt: 1, SDPMLineIndex: MaxSDPMLineIndex + 1},
		StudentPresentStop{PresentID: ""},
		ScreenBroadcastStart{BroadcastID: testRefID, Width: 2, Height: 2, FPS: 1, Source: &BroadcastSource{Kind: "teacher", Hostname: "PC"}},
		ScreenBroadcastStart{BroadcastID: testRefID, Width: 2, Height: 2, FPS: 1, Source: &BroadcastSource{Kind: BroadcastSourceStudent, Hostname: " "}},
		ScreenBroadcastStart{BroadcastID: testRefID, Width: 2, Height: 2, FPS: 1, Source: &BroadcastSource{Kind: BroadcastSourceStudent, Hostname: strings.Repeat("a", MaxSourceHostname+1)}},
	} {
		if err := p.Validate(); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%T %+v: Validate = %v, want invalid_message", p, p, err)
		}
	}
	big := StudentPresentAnswer{RefID: testRefID, PresentID: testRefID, Attempt: 1, SDP: "v=0" + strings.Repeat("a", MaxSDP)}
	if err := big.Validate(); !errors.Is(err, ErrMessageTooLarge) {
		t.Errorf("oversized answer: %v, want message_too_large", err)
	}
	for _, p := range []Payload{
		StudentPresentStart{PresentID: testRefID, MaxWidth: MinPresentWidth, FPS: 1, MaxKbps: MinPresentKbps},
		StudentPresentStart{PresentID: testRefID, MaxWidth: MaxPresentWidth, FPS: MaxPresentFPS, MaxKbps: MaxPresentKbps},
		StudentPresentAck{RefID: testRefID, PresentID: testRefID, Status: PresentEnded},
		StudentPresentAck{RefID: testRefID, PresentID: testRefID, Status: PresentAccepted, Width: 4096, Height: 2},
		StudentPresentICE{PresentID: testRefID, Attempt: MaxBroadcastAttempt},
	} {
		if err := p.Validate(); err != nil {
			t.Errorf("%T: the limits must be valid: %v", p, err)
		}
	}
	for s, final := range map[PresentStatus]bool{PresentAccepted: false, PresentEnded: true, PresentFailed: true} {
		if s.Final() != final {
			t.Errorf("%s.Final() = %v", s, !final)
		}
	}
}

func TestClassEndAckValidation(t *testing.T) {
	ref, end := NewID(), NewID()
	for _, c := range []struct {
		ack ClassEndAck
		ok  bool
	}{
		{ClassEndAck{RefID: ref, EndID: end, Status: ClassEndClean}, true},
		{ClassEndAck{RefID: ref, EndID: end, Status: ClassEndFailed, Reason: ClassEndPause}, true},
		{ClassEndAck{RefID: ref, EndID: end, Status: ClassEndClean, Reason: ClassEndPause}, false},
		{ClassEndAck{RefID: ref, EndID: end, Status: ClassEndFailed}, false},
		{ClassEndAck{RefID: ref, EndID: end, Status: "done"}, false},
		{ClassEndAck{RefID: ref, EndID: "x", Status: ClassEndClean}, false},
	} {
		if err := c.ack.Validate(); (err == nil) != c.ok {
			t.Errorf("%+v: err = %v, want ok %v", c.ack, err, c.ok)
		}
	}
}
