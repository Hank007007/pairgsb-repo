package audit

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecordWritesOneJSONObjectPerLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	log, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	log.Record(EventSignIssued, "kh_1", "sign", "")
	log.Record(EventSignDenied, "kh_2", "countersign", "purpose not allowed")
	if err := log.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	entries := readEntries(t, path)
	if len(entries) != 2 {
		t.Fatalf("read %d entries, want 2", len(entries))
	}
	if entries[0].Event != EventSignIssued || entries[0].KeyHandle != "kh_1" {
		t.Fatalf("first entry = %+v", entries[0])
	}
	if entries[1].Event != EventSignDenied || entries[1].Purpose != "countersign" {
		t.Fatalf("second entry = %+v", entries[1])
	}
}

func TestRecordAtKeepsSuppliedInstant(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	log, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	stamp := time.Date(2030, 5, 4, 3, 2, 1, 0, time.UTC)
	log.RecordAt(stamp, EventSessionUnlocked, "", "", "")
	if err := log.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	entries := readEntries(t, path)
	if len(entries) != 1 {
		t.Fatalf("read %d entries, want 1", len(entries))
	}
	if !entries[0].Time.Equal(stamp) {
		t.Fatalf("entry time = %v, want %v", entries[0].Time, stamp)
	}
}

func TestOpenCreatesFileWithRestrictedMode(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("file modes are not meaningful on this platform")
	}

	path := filepath.Join(t.TempDir(), "nested", "audit.log")
	log, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer log.Close()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("audit file mode = %o, want 600", mode)
	}
}

func readEntries(t *testing.T, path string) []Entry {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open audit file: %v", err)
	}
	defer f.Close()

	var out []Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e Entry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			t.Fatalf("decode audit line %q: %v", scanner.Text(), err)
		}
		out = append(out, e)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan audit file: %v", err)
	}
	return out
}
