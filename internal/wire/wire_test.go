package wire

import (
	"os"
	"path/filepath"
	"testing"

	"ptrloop/internal/limits"
)

func readCapture(t *testing.T, name string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return raw
}

func TestParseNormalARecord(t *testing.T) {
	raw := readCapture(t, "normal-a.bin")

	msg, err := Parse(raw, limits.New())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got, want := msg.Header.QDCount, uint16(1); got != want {
		t.Fatalf("QDCOUNT = %d, want %d", got, want)
	}
	if got, want := len(msg.Questions), 1; got != want {
		t.Fatalf("questions = %d, want %d", got, want)
	}
	if got, want := msg.Questions[0].Name.String(), "www.example.com."; got != want {
		t.Fatalf("question name = %q, want %q", got, want)
	}
	if got, want := msg.Questions[0].Type, TypeA; got != want {
		t.Fatalf("question type = %d, want %d", got, want)
	}
	if got, want := len(msg.Answers), 1; got != want {
		t.Fatalf("answers = %d, want %d", got, want)
	}

	rr := msg.Answers[0]
	if got, want := rr.Name.String(), "www.example.com."; got != want {
		t.Fatalf("answer owner = %q, want %q", got, want)
	}
	if got, want := rr.Type, TypeA; got != want {
		t.Fatalf("answer type = %d, want %d", got, want)
	}
	if got, want := rr.TTL, uint32(300); got != want {
		t.Fatalf("answer ttl = %d, want %d", got, want)
	}
	if got, want := rr.RDLENGTH, uint16(4); got != want {
		t.Fatalf("answer rdlength = %d, want %d", got, want)
	}
	if got := rr.Data.Raw; len(got) != 4 || got[0] != 93 || got[3] != 34 {
		t.Fatalf("answer rdata = %v, want 93.184.216.34", got)
	}
}

func TestParseNXDomain(t *testing.T) {
	raw := readCapture(t, "nxdomain.bin")

	msg, err := Parse(raw, limits.New())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got, want := msg.Header.RCODE(), 3; got != want {
		t.Fatalf("RCODE = %d, want %d", got, want)
	}
	if got, want := msg.Questions[0].Name.String(), "nope.example.com."; got != want {
		t.Fatalf("question name = %q, want %q", got, want)
	}
	if len(msg.Answers) != 0 {
		t.Fatalf("answers = %d, want 0", len(msg.Answers))
	}
}

func TestReadNameUncompressed(t *testing.T) {
	msg := []byte{0x03, 'w', 'w', 'w', 0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		0x03, 'c', 'o', 'm', 0x00}
	name, next, err := ReadName(msg, 0, limits.New())
	if err != nil {
		t.Fatalf("ReadName: %v", err)
	}
	if got, want := name.String(), "www.example.com."; got != want {
		t.Fatalf("name = %q, want %q", got, want)
	}
	if got, want := next, len(msg); got != want {
		t.Fatalf("next = %d, want %d", got, want)
	}
}

func TestReadNameBackwardPointer(t *testing.T) {
	msg := []byte{0x03, 'w', 'w', 'w', 0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		0x03, 'c', 'o', 'm', 0x00, 0xC0, 0x00}
	name, next, err := ReadName(msg, 17, limits.New())
	if err != nil {
		t.Fatalf("ReadName: %v", err)
	}
	if got, want := name.String(), "www.example.com."; got != want {
		t.Fatalf("name = %q, want %q", got, want)
	}
	if got, want := next, 19; got != want {
		t.Fatalf("next = %d, want %d", got, want)
	}
}

func TestLabelClassification(t *testing.T) {
	tests := []struct {
		b    byte
		want LabelKind
	}{
		{0x00, LabelEnd},
		{0x01, LabelData},
		{0x3F, LabelData},
		{0xC0, LabelPointer},
		{0xFF, LabelPointer},
	}
	for _, tc := range tests {
		if got := Label(tc.b); got != tc.want {
			t.Errorf("Label(%#x) = %v, want %v", tc.b, got, tc.want)
		}
	}
}

func TestReadNameTruncated(t *testing.T) {
	_, _, err := ReadName([]byte{0x05, 'a', 'b'}, 0, limits.New())
	if err == nil {
		t.Fatal("ReadName accepted a label that runs past the buffer")
	}
}

func TestParserHandlesConsecutiveResponses(t *testing.T) {
	raw := readCapture(t, "normal-a.bin")

	p := NewParser()
	for i := 0; i < 2; i++ {
		msg, err := p.Parse(raw)
		if err != nil {
			t.Fatalf("Parse #%d: %v", i+1, err)
		}
		if got, want := len(msg.Answers), 1; got != want {
			t.Fatalf("Parse #%d answers = %d, want %d", i+1, got, want)
		}
	}
}
