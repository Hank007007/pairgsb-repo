package wire

import (
	"errors"
	"testing"
	"time"

	"ptrloop/internal/limits"
)

func TestCaptureDecodeExpectations(t *testing.T) {
	tests := []struct {
		name    string
		errIs   error
		answers int
	}{
		{"bad-ascii-label.bin", ErrName, 0},
		{"case-mismatch.bin", nil, 1},
		{"cname-case-hop.bin", nil, 2},
		{"cname-chain-in.bin", nil, 2},
		{"cname-chain-out.bin", nil, 3},
		{"count-mismatch.bin", ErrRecord, 0},
		{"label-too-long.bin", ErrName, 0},
		{"label-with-dot.bin", ErrName, 0},
		{"name-too-long.bin", ErrName, 0},
		{"normal-a.bin", nil, 1},
		{"nxdomain.bin", nil, 0},
		{"opt-bad-udpsize.bin", ErrRecord, 0},
		{"opt-unknown-option.bin", nil, 0},
		{"partial-records.bin", ErrRecord, 0},
		{"ptr-forward.bin", ErrName, 0},
		{"ptr-mid-label.bin", ErrName, 0},
		{"ptr-self-loop.bin", ErrName, 0},
		{"qdcount-2.bin", ErrHeader, 0},
		{"rdata-bad-rdlength.bin", ErrRecord, 0},
		{"rdata-compressed-unknown.bin", ErrRecord, 0},
		{"rdata-name-pointer.bin", nil, 2},
		{"reserved-label.bin", ErrName, 0},
		{"suffix-confusion.bin", nil, 1},
		{"tc-set.bin", ErrHeader, 0},
		{"trailing-bytes.bin", ErrHeader, 0},
		{"ttl-huge.bin", nil, 1},
		{"ttl-wrap.bin", nil, 1},
		{"ttl-zero.bin", nil, 1},
		{"two-responses.bin", ErrHeader, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg, err := Parse(readCapture(t, tc.name), limits.New())
			if tc.errIs != nil {
				if !errors.Is(err, tc.errIs) {
					t.Fatalf("err = %v, want errors.Is(..., %v)", err, tc.errIs)
				}
				if msg != nil {
					t.Fatal("Parse returned a partial message on error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if got := len(msg.Answers); got != tc.answers {
				t.Fatalf("answers = %d, want %d", got, tc.answers)
			}
		})
	}
}

func TestRDataNamePointerIsMessageRelative(t *testing.T) {
	msg, err := Parse(readCapture(t, "rdata-name-pointer.bin"), limits.New())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	target := msg.Answers[0].Data.Names[0].String()
	if got, want := target, "cdn.example.com."; got != want {
		t.Fatalf("CNAME target = %q, want %q", got, want)
	}
}

func TestOPTUnknownOptionDropped(t *testing.T) {
	msg, err := Parse(readCapture(t, "opt-unknown-option.bin"), limits.New())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got, want := len(msg.Additionals), 1; got != want {
		t.Fatalf("additionals = %d, want %d", got, want)
	}
	opts := msg.Additionals[0].Data.Options
	if got, want := len(opts), 1; got != want {
		t.Fatalf("options = %d, want %d", got, want)
	}
	if got, want := opts[0].Code, uint16(3); got != want {
		t.Fatalf("option code = %d, want %d", got, want)
	}
}

func TestPointerLoopsTerminate(t *testing.T) {
	mutual := []byte{0x12, 0x34, 0x81, 0x80, 0, 1, 0, 0, 0, 0, 0, 0,
		0xC0, 0x0E, 0xC0, 0x0C, 0, 1, 0, 1}
	inputs := map[string][]byte{
		"ptr-self-loop.bin": readCapture(t, "ptr-self-loop.bin"),
		"ptr-mutual-loop":   mutual,
	}
	for name, raw := range inputs {
		t.Run(name, func(t *testing.T) {
			done := make(chan error, 1)
			go func() {
				_, err := Parse(raw, limits.New())
				done <- err
			}()
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("Parse accepted a pointer loop")
				}
			case <-time.After(2 * time.Second):
				t.Fatal("Parse did not terminate")
			}
		})
	}
}

func TestPointerBombExhaustsBudget(t *testing.T) {
	b := limits.New()
	_, err := Parse(readCapture(t, "ptr-bomb.bin"), b)
	if !errors.Is(err, limits.ErrExhausted) {
		t.Fatalf("err = %v, want limits.ErrExhausted", err)
	}
}

func TestParserResetsBudgetPerMessage(t *testing.T) {
	raw := readCapture(t, "normal-a.bin")
	p := NewParser()
	for i := 0; i < 3; i++ {
		if _, err := p.Parse(raw); err != nil {
			t.Fatalf("Parse #%d: %v", i+1, err)
		}
		_, names, records, bytes := p.Budget().Used()
		if names != 2 || records != 1 || bytes != len(raw) {
			t.Fatalf("parse #%d budget = names:%d records:%d bytes:%d, want 2,1,%d",
				i+1, names, records, bytes, len(raw))
		}
	}
}

func TestReservedLabelBytesRejected(t *testing.T) {
	for _, first := range []byte{0x40, 0x7F, 0x80, 0xBF} {
		msg := []byte{12: first, 13: 'a', 14: 0}
		if _, _, err := ReadName(msg, 12, limits.New()); !errors.Is(err, ErrName) {
			t.Fatalf("label byte %#x: err = %v, want ErrName", first, err)
		}
	}
}
