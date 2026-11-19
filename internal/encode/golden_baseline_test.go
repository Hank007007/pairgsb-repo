package encode_test

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"logline-audit-encode/internal/audit"
	"logline-audit-encode/internal/encode"
)

var update = flag.Bool("update", false, "rewrite golden files")

func TestGoldenBaseline(t *testing.T) {
	cases := []struct {
		name   string
		format encode.Format
		golden string
	}{
		{name: "kv", format: encode.FormatKV, golden: "baseline_kv.golden"},
		{name: "json", format: encode.FormatJSON, golden: "baseline_json.golden"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			enc, err := encode.NewEncoder(tc.format)
			if err != nil {
				t.Fatalf("NewEncoder(%q): %v", tc.format, err)
			}

			got, err := enc.Encode(audit.Sample())
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}

			path := filepath.Join("testdata", "golden", tc.golden)
			if *update {
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
			}

			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden: %v", err)
			}

			if string(got) != string(want) {
				t.Errorf("encoded record differs from golden %s\n got: %q\nwant: %q", tc.golden, got, want)
			}
		})
	}
}

// TestGoldenBaselineHasNoTerminator guards the contract that the encoder
// returns a record without a trailing record separator.
func TestGoldenBaselineHasNoTerminator(t *testing.T) {
	enc, err := encode.NewEncoder(encode.FormatKV)
	if err != nil {
		t.Fatalf("NewEncoder: %v", err)
	}
	got, err := enc.Encode(audit.Sample())
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("empty record")
	}
	if last := got[len(got)-1]; last == '\n' || last == '\r' {
		t.Fatalf("record ends with terminator byte %q", last)
	}
}
