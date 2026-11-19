package encode_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"logline-audit-encode/internal/audit"
	"logline-audit-encode/internal/encode"
	"logline-audit-encode/internal/fields"
)

func TestKVHostileUserAgentStaysOnOneRecord(t *testing.T) {
	enc, err := encode.NewEncoder(encode.FormatKV)
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}

	ev := audit.Sample()
	ev.UserAgent = "Mozilla/5.0\r\nlevel=critical user=root"

	record, err := enc.Encode(ev)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got := string(record)

	want := `user_agent="Mozilla/5.0\r\nlevel=critical user=root"`
	if !strings.Contains(got, want) {
		t.Fatalf("user_agent field = %q, want it to contain %q", got, want)
	}
	if strings.ContainsAny(got, "\r\n") {
		t.Fatalf("record contains a literal CR or LF: %q", got)
	}
	if strings.Contains(got, "\u2028") || strings.Contains(got, "\u2029") {
		t.Fatalf("record contains a literal line separator: %q", got)
	}
}

func TestKVValueQuotingAndEscaping(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain", in: "plain", want: "plain"},
		{name: "empty", in: "", want: `""`},
		{name: "space", in: "a b", want: `"a b"`},
		{name: "equals", in: "a=b", want: `"a=b"`},
		{name: "quote_and_backslash", in: `a"b\c`, want: `"a\"b\\c"`},
		{name: "short_escapes", in: "a\tb\nc\rd", want: `"a\tb\nc\rd"`},
		{name: "other_controls", in: "\x00\x1f\x7f", want: `"\u0000\u001F\u007F"`},
		{name: "line_separators", in: "\u2028\u2029", want: `"\u2028\u2029"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enc, err := encode.NewEncoder(encode.FormatKV)
			if err != nil {
				t.Fatalf("NewEncoder: %v", err)
			}

			ev := audit.Sample()
			ev.Actor = tc.in

			record, err := enc.Encode(ev)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			if !strings.Contains(string(record), "actor="+tc.want) {
				t.Fatalf("record = %q, want actor=%s", record, tc.want)
			}
		})
	}
}

func TestKVRecordContainsNoLiteralControlOrLineSeparatorBytes(t *testing.T) {
	enc, err := encode.NewEncoder(encode.FormatKV)
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}

	ev := audit.Sample()
	ev.Actor = "\x00\x01\x02\x03\x04\x05\x06\x07\x08\t\n\x0b\x0c\r\x0e\x0f" +
		"\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f" +
		"\x7f\u2028\u2029\"\\"

	record, err := enc.Encode(ev)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	for b := byte(0); b < 0x20; b++ {
		if bytes.IndexByte(record, b) >= 0 {
			t.Fatalf("record contains literal control byte %#x: %q", b, record)
		}
	}
	if bytes.IndexByte(record, 0x7f) >= 0 {
		t.Fatalf("record contains literal DEL: %q", record)
	}
	if strings.Contains(string(record), "\u2028") || strings.Contains(string(record), "\u2029") {
		t.Fatalf("record contains literal U+2028/U+2029: %q", record)
	}
}

func TestJSONEscapesControlAndLineSeparatorCharacters(t *testing.T) {
	enc, err := encode.NewEncoder(encode.FormatJSON)
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}

	ev := audit.Sample()
	ev.Message = "nul:\x00 ctl:\x01\x1f del:\x7f line:\u2028\u2029 tab:\t quote:\" slash:\\ html:<&/"

	record, err := enc.Encode(ev)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	var decoded struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(record, &decoded); err != nil {
		t.Fatalf("encoded record is not valid JSON: %v\nrecord: %q", err, record)
	}
	if decoded.Message != ev.Message {
		t.Fatalf("decoded message = %q, want %q", decoded.Message, ev.Message)
	}

	got := string(record)
	for _, want := range []string{
		`\u0000`, `\u0001`, `\u001F`, `\u007F`, `\u2028`, `\u2029`,
		`\t`, `\"`, `\\`, "<&/",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("record = %q, want it to contain %q", got, want)
		}
	}
	for b := byte(0); b < 0x20; b++ {
		if bytes.IndexByte(record, b) >= 0 {
			t.Fatalf("record contains literal control byte %#x: %q", b, record)
		}
	}
	if bytes.IndexByte(record, 0x7f) >= 0 {
		t.Fatalf("record contains literal DEL: %q", record)
	}
	if strings.Contains(got, "\u2028") || strings.Contains(got, "\u2029") {
		t.Fatalf("record contains literal U+2028/U+2029: %q", got)
	}
}

func TestEncodeRejectsInvalidExtensionFieldsInBothFormats(t *testing.T) {
	cases := []struct {
		name  string
		extra map[string]string
		want  error
	}{
		{
			name:  "reserved name",
			extra: map[string]string{"level": "critical"},
			want:  fields.ErrReservedField,
		},
		{
			name:  "invalid name",
			extra: map[string]string{"x y=z": "value"},
			want:  fields.ErrInvalidKey,
		},
	}

	for _, format := range []encode.Format{encode.FormatKV, encode.FormatJSON} {
		for _, tc := range cases {
			t.Run(string(format)+"/"+tc.name, func(t *testing.T) {
				enc, err := encode.NewEncoder(format)
				if err != nil {
					t.Fatalf("NewEncoder: %v", err)
				}

				ev := audit.Sample()
				ev.Extra = tc.extra

				record, err := enc.Encode(ev)
				if !errors.Is(err, tc.want) {
					t.Fatalf("Encode error = %v, want %v", err, tc.want)
				}
				if record != nil {
					t.Fatalf("Encode returned a partial record on error: %q", record)
				}
			})
		}
	}
}
