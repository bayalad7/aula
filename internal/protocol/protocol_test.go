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
		TypeScreenBroadcastStart: CapScreenBroadcast, TypeScreenBroadcastOffer: CapScreenBroadcast,
		TypeScreenBroadcastICE: CapScreenBroadcast, TypeScreenBroadcastStop: CapScreenBroadcast,
		TypeAppBlockSet: CapAppBlock, TypeAppBlockClear: CapAppBlock,
		TypeWebFilterSet: CapWebFilter, TypeWebFilterClear: CapWebFilter,
	} {
		if c, ok := RequiredCapability(typ); !ok || c != want {
			t.Errorf("%s requires %q, %v; want %q", typ, c, ok, want)
		}
	}
	for _, typ := range []Type{TypeScreenThumbFrame, TypeScreenBroadcastAnswer, TypeAppBlockAck, TypeWebFilterAck} {
		if _, ok := RequiredCapability(typ); ok {
			t.Errorf("%s goes to the console and must not require a capability", typ)
		}
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
