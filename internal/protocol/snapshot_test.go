package protocol

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func snapshotPart1(size int) ScreenSnapshotResponse {
	return ScreenSnapshotResponse{RefID: testSnippetID, SnapshotID: testRefID, Part: 1,
		Parts: (size + SnapshotPartSize - 1) / SnapshotPartSize, Data: []byte{1},
		TakenAt: 1, Hostname: "LAB3-PC07", Width: 1920, Height: 1080, Size: size, SHA256: strings.Repeat("0", 64)}
}

func TestSnapshotValidation(t *testing.T) {
	ok := []Payload{
		ScreenSnapshotRequest{SnapshotID: testRefID, Quality: MinSnapshotQuality},
		ScreenSnapshotRequest{SnapshotID: testRefID, Quality: MaxSnapshotQuality, MaxWidth: MaxSnapshotWidth},
		snapshotPart1(MaxSnapshotBytes),
	}
	for _, p := range ok {
		if err := p.Validate(); err != nil {
			t.Errorf("%#v: %v", p, err)
		}
	}
	bad := map[string]Payload{
		"quality":        ScreenSnapshotRequest{SnapshotID: testRefID, Quality: 40},
		"width":          ScreenSnapshotRequest{SnapshotID: testRefID, Quality: 90, MaxWidth: 320},
		"wide":           ScreenSnapshotRequest{SnapshotID: testRefID, Quality: 90, MaxWidth: MaxSnapshotWidth + 1},
		"id":             ScreenSnapshotRequest{SnapshotID: "x", Quality: 90},
		"too many parts": func() Payload { p := snapshotPart1(1); p.Parts, p.Part = MaxSnapshotParts+1, 1; return p }(),
		"parts vs size":  func() Payload { p := snapshotPart1(SnapshotPartSize + 1); p.Parts = 1; return p }(),
		"no hostname":    func() Payload { p := snapshotPart1(10); p.Hostname = ""; return p }(),
		"no taken_at":    func() Payload { p := snapshotPart1(10); p.TakenAt = 0; return p }(),
		"sha256":         func() Payload { p := snapshotPart1(10); p.SHA256 = strings.Repeat("G", 64); return p }(),
		"size":           func() Payload { p := snapshotPart1(10); p.Size = MaxSnapshotBytes + 1; return p }(),
		"side":           func() Payload { p := snapshotPart1(10); p.Width = MaxSnapshotSide + 1; return p }(),
		"empty data":     ScreenSnapshotResponse{RefID: testRefID, SnapshotID: testRefID, Part: 2, Parts: 2},
	}
	for name, p := range bad {
		if err := p.Validate(); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("%s: Validate = %v, want ErrInvalidMessage", name, err)
		}
	}
}

// The largest part still fits in one envelope once Base64-encoded, with a
// full part-1 header.
func TestSnapshotPartFitsInOneMessage(t *testing.T) {
	p := snapshotPart1(MaxSnapshotBytes)
	p.Hostname, p.Username = strings.Repeat("h", MaxHostname), strings.Repeat("u", MaxUsername)
	p.Data = make([]byte, SnapshotPartSize)
	if _, err := Encode(New(p)); err != nil {
		t.Fatalf("Encode of a full part: %v", err)
	}
	p.Data = make([]byte, SnapshotPartSize+1)
	if err := p.Validate(); !errors.Is(err, ErrMessageTooLarge) {
		t.Fatalf("Validate = %v, want ErrMessageTooLarge", err)
	}
}

func TestSplitAndReassemble(t *testing.T) {
	data := make([]byte, 2*SnapshotPartSize+123)
	for i := range data {
		data[i] = byte(i * 7)
	}
	parts := SplitParts(data, SnapshotPartSize)
	if len(parts) != 3 || len(parts[0]) != SnapshotPartSize || len(parts[2]) != 123 {
		t.Fatalf("SplitParts: %d parts", len(parts))
	}
	r := NewReassembly(len(parts))
	// Out of order on purpose.
	for _, i := range []int{2, 0, 1} {
		if r.Complete() {
			t.Fatal("complete too early")
		}
		if err := r.Add(i+1, parts[i]); err != nil {
			t.Fatal(err)
		}
	}
	if !r.Complete() || !bytes.Equal(r.Bytes(), data) {
		t.Fatal("reassembled bytes differ")
	}
	if err := r.Add(2, parts[1]); !errors.Is(err, ErrPartRepeated) {
		t.Fatalf("repeated part: %v", err)
	}
	if err := r.Add(4, parts[1]); !errors.Is(err, ErrPartOutOfRange) {
		t.Fatalf("part out of range: %v", err)
	}
	if err := NewReassembly(2).Add(0, nil); !errors.Is(err, ErrPartOutOfRange) {
		t.Fatalf("part 0: %v", err)
	}
	if NewReassembly(2).Bytes() != nil {
		t.Fatal("Bytes before complete must be nil")
	}
	if SplitParts(nil, 10) != nil || len(SplitParts([]byte{1}, 10)) != 1 {
		t.Fatal("SplitParts edge cases")
	}
}
