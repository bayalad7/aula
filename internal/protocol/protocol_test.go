package protocol

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
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
	}
}

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
	if _, ok := RequiredCapability(TypeHeartbeat); ok {
		t.Fatal("heartbeat must not require a capability")
	}
	if !Has([]Capability{CapMsgShow}, CapMsgShow) || Has(nil, CapCodeView) {
		t.Fatal("Has misbehaves")
	}
}

func TestValidChannel(t *testing.T) {
	for _, ok := range []string{"AULA_101", "CURSO_PYTHON_A", "X", strings.Repeat("A", 32)} {
		if !ValidChannel(ok) {
			t.Errorf("ValidChannel(%q) = false", ok)
		}
	}
	for _, bad := range []string{"", "aula_101", "AULA 101", "AULA-101", strings.Repeat("A", 33)} {
		if ValidChannel(bad) {
			t.Errorf("ValidChannel(%q) = true", bad)
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
