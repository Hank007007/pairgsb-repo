package record_test

import (
	"bufio"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"snisieve/internal/record"
)

// loadFlight reads the testdata fixture and returns the named flight. Lines
// beginning with '#' are comments; the line after one is that flight.
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

func TestReadRecordsSplitsTwoRecords(t *testing.T) {
	buf := loadFlight(t, "split across two")
	recs, err := record.ReadRecords(buf)
	if err != nil {
		t.Fatalf("ReadRecords: %v", err)
	}
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2", len(recs))
	}
	for i, rec := range recs {
		if rec.Type != record.TypeHandshake {
			t.Errorf("record %d: type %d, want handshake", i, rec.Type)
		}
		if rec.Version != 0x0301 {
			t.Errorf("record %d: version %#04x, want 0x0301", i, rec.Version)
		}
		if len(rec.Payload) == 0 {
			t.Errorf("record %d: empty payload", i)
		}
	}
	if len(recs[0].Payload) != 40 {
		t.Errorf("first payload: %d bytes, want 40", len(recs[0].Payload))
	}
}
