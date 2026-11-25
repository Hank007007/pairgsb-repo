package hello_test

import (
	"bufio"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"snisieve/internal/hello"
	"snisieve/internal/record"
)

// loadFlight returns the fixture flight whose comment contains name.
func loadFlight(t *testing.T, name string) []byte {
	t.Helper()
	f, err := os.Open("../../testdata/clienthello.hex")
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	want := ""
	var raw string
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			want = strings.TrimSpace(strings.TrimPrefix(line, "#"))
			continue
		}
		if strings.Contains(want, name) {
			raw = line
			break
		}
	}
	if raw == "" {
		t.Fatalf("fixture %q not found", name)
	}
	b, err := hex.DecodeString(raw)
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	return b
}

func TestParseClientHelloSingleRecord(t *testing.T) {
	buf := loadFlight(t, "one ClientHello in one")

	recs, err := record.ReadRecords(buf)
	if err != nil {
		t.Fatalf("ReadRecords: %v", err)
	}
	msg, err := record.Reassemble(recs)
	if err != nil {
		t.Fatalf("Reassemble: %v", err)
	}
	ch, err := hello.ParseClientHello(msg)
	if err != nil {
		t.Fatalf("ParseClientHello: %v", err)
	}
	if ch.LegacyVersion != 0x0303 {
		t.Errorf("legacy_version = %#04x, want 0x0303", ch.LegacyVersion)
	}
	if len(ch.Extensions) != 2 {
		t.Errorf("got %d extensions, want 2", len(ch.Extensions))
	}
	host, err := hello.ExtractSNI(ch)
	if err != nil {
		t.Fatalf("ExtractSNI: %v", err)
	}
	if host != "example.com" {
		t.Errorf("host = %q, want %q", host, "example.com")
	}
}
