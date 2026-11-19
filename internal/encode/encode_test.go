package encode_test

import (
	"errors"
	"strings"
	"testing"

	"logline-audit-encode/internal/audit"
	"logline-audit-encode/internal/encode"
)

func TestNewEncoderRejectsUnknownFormat(t *testing.T) {
	if _, err := encode.NewEncoder("xml"); !errors.Is(err, encode.ErrUnknownFormat) {
		t.Fatalf("NewEncoder(xml) error = %v, want ErrUnknownFormat", err)
	}
}

func TestEncoderReportsItsFormat(t *testing.T) {
	enc, err := encode.NewEncoder(encode.FormatJSON)
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}
	if got := enc.Format(); got != encode.FormatJSON {
		t.Fatalf("Format() = %q, want %q", got, encode.FormatJSON)
	}
}

func TestEncodeStartsWithTimestamp(t *testing.T) {
	enc, err := encode.NewEncoder(encode.FormatKV)
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}
	record, err := enc.Encode(audit.Sample())
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !strings.HasPrefix(string(record), "timestamp=2026-03-14T09:00:00Z ") {
		t.Fatalf("record does not start with timestamp: %q", record)
	}
}

func TestEncodeSortsExtensionFields(t *testing.T) {
	enc, err := encode.NewEncoder(encode.FormatKV)
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}

	ev := audit.Sample()
	ev.Extra = map[string]string{
		"zulu":  "z",
		"alpha": "a",
		"mike":  "m",
	}

	record, err := enc.Encode(ev)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	kv := string(record)
	alpha := strings.Index(kv, "alpha=a")
	mike := strings.Index(kv, "mike=m")
	zulu := strings.Index(kv, "zulu=z")
	if alpha < 0 || mike < 0 || zulu < 0 {
		t.Fatalf("extension fields missing from record: %q", kv)
	}
	if !(alpha < mike && mike < zulu) {
		t.Fatalf("extension fields not sorted: alpha=%d mike=%d zulu=%d", alpha, mike, zulu)
	}
}

func TestEncodeIsDeterministic(t *testing.T) {
	enc, err := encode.NewEncoder(encode.FormatJSON)
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}

	ev := audit.Sample()
	first, err := enc.Encode(ev)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	for i := 0; i < 20; i++ {
		next, err := enc.Encode(ev)
		if err != nil {
			t.Fatalf("Encode: %v", err)
		}
		if string(next) != string(first) {
			t.Fatalf("run %d differs: %q vs %q", i, next, first)
		}
	}
}

func TestEncodeQuotesValuesContainingSpaces(t *testing.T) {
	enc, err := encode.NewEncoder(encode.FormatKV)
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}

	ev := audit.Sample()
	ev.Actor = "user:42"
	ev.Resource = "quarterly report"

	record, err := enc.Encode(ev)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	kv := string(record)
	if !strings.Contains(kv, "actor=user:42 ") {
		t.Errorf("bare value was quoted: %q", kv)
	}
	if !strings.Contains(kv, `resource="quarterly report"`) {
		t.Errorf("value with a space was not quoted: %q", kv)
	}
}

func TestEncodeEmptyValueIsQuoted(t *testing.T) {
	enc, err := encode.NewEncoder(encode.FormatKV)
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}

	ev := audit.Sample()
	ev.Actor = ""

	record, err := enc.Encode(ev)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if !strings.Contains(string(record), `actor=""`) {
		t.Fatalf("empty value not represented as quoted empty string: %q", record)
	}
}
