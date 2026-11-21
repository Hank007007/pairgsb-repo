package journal_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"maskseal-journal/internal/journal"
	"maskseal-journal/internal/keystore"
	"maskseal-journal/internal/policy"
	"maskseal-journal/internal/record"
)

func regressionRecord(action string, fields map[string]any) record.Record {
	return record.Record{
		Actor:  "alice@example.com",
		Action: action,
		Time:   time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC),
		Fields: fields,
	}
}

func TestVerifyRejectsTamperedPlaintext(t *testing.T) {
	dir := t.TempDir()
	j, err := journal.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := j.Append(regressionRecord("login", map[string]any{"role": "admin"})); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := j.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	dataPath := filepath.Join(dir, journal.DataFileName)
	data, err := os.ReadFile(dataPath)
	if err != nil {
		t.Fatalf("read data: %v", err)
	}
	if !bytes.Contains(data, []byte(`"admin"`)) {
		t.Fatalf("test record does not contain admin: %s", data)
	}
	tampered := bytes.Replace(data, []byte(`"admin"`), []byte(`"admix"`), 1)
	if err := os.WriteFile(dataPath, tampered, 0o600); err != nil {
		t.Fatalf("write tampered data: %v", err)
	}

	result, err := journal.Verify(dir)
	if err == nil {
		t.Fatal("verify accepted tampered plaintext")
	}
	if result.FirstBadSeq != 1 {
		t.Fatalf("FirstBadSeq = %d, want 1", result.FirstBadSeq)
	}
	if !strings.Contains(err.Error(), "seq 1") {
		t.Fatalf("error does not report seq 1: %v", err)
	}
}

func TestVerifyRejectsTamperedCommitment(t *testing.T) {
	dir := t.TempDir()
	salts, err := keystore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open keystore: %v", err)
	}
	rules, err := policy.Parse([]byte(`{"rules":[{"path":"password","action":"redact"}]}`))
	if err != nil {
		t.Fatalf("policy: %v", err)
	}
	fields, _, err := record.Redact(map[string]any{"password": "hunter2"}, rules, salts)
	if err != nil {
		t.Fatalf("redact: %v", err)
	}
	j, err := journal.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	stored, err := j.Append(regressionRecord("login", fields))
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := j.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	red := stored.Fields["password"].(*record.Redacted)
	changed := append([]byte(nil), []byte(red.Commitment)...)
	if changed[0] == 'a' {
		changed[0] = 'b'
	} else {
		changed[0] = 'a'
	}
	dataPath := filepath.Join(dir, journal.DataFileName)
	data, err := os.ReadFile(dataPath)
	if err != nil {
		t.Fatalf("read data: %v", err)
	}
	tampered := bytes.Replace(data, []byte(red.Commitment), changed, 1)
	if err := os.WriteFile(dataPath, tampered, 0o600); err != nil {
		t.Fatalf("write tampered data: %v", err)
	}

	result, err := journal.Verify(dir)
	if err == nil {
		t.Fatal("verify accepted a tampered commitment")
	}
	if result.FirstBadSeq != 1 {
		t.Fatalf("FirstBadSeq = %d, want 1", result.FirstBadSeq)
	}
}

func TestAppendIsVisibleBeforeClose(t *testing.T) {
	dir := t.TempDir()
	j, err := journal.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer j.Close()

	for i := 0; i < 2; i++ {
		if _, err := j.Append(regressionRecord("login", map[string]any{"i": int64(i)})); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
		info, err := os.Stat(filepath.Join(dir, journal.DataFileName))
		if err != nil {
			t.Fatalf("stat data: %v", err)
		}
		if info.Size() == 0 {
			t.Fatalf("data file is empty after append %d", i)
		}
	}
}

func TestOpenRecoversFromPartialTail(t *testing.T) {
	dir := t.TempDir()
	j, err := journal.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := j.Append(regressionRecord("first", map[string]any{"n": int64(1)})); err != nil {
		t.Fatalf("append first: %v", err)
	}
	if _, err := j.Append(regressionRecord("second", map[string]any{"n": int64(2)})); err != nil {
		t.Fatalf("append second: %v", err)
	}
	entries := j.Entries()
	if err := j.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	last := entries[len(entries)-1]
	truncatedSize := last.Offset + last.Length/2
	dataPath := filepath.Join(dir, journal.DataFileName)
	if err := os.Truncate(dataPath, truncatedSize); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	reopened, err := journal.Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got := len(reopened.Entries()); got != 1 {
		t.Fatalf("entries after partial tail = %d, want 1", got)
	}
	if err := reopened.Close(); err != nil {
		t.Fatalf("close reopened: %v", err)
	}
	info, err := os.Stat(dataPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() != entries[0].Offset+entries[0].Length {
		t.Fatalf("partial tail was not discarded: size=%d", info.Size())
	}

	result, err := journal.Verify(dir)
	if err != nil {
		t.Fatalf("verify after recovery: %v", err)
	}
	if result.Records != 1 {
		t.Fatalf("verified records = %d, want 1", result.Records)
	}
}

func TestIndexAheadOfDataIsRebuilt(t *testing.T) {
	dir := t.TempDir()
	j, err := journal.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := j.Append(regressionRecord("login", map[string]any{"n": int64(1)})); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := j.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	idxPath := filepath.Join(dir, journal.IndexFileName)
	idx, err := os.ReadFile(idxPath)
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	fake := append(append([]byte(nil), idx...), []byte("2 999999 10\n")...)
	if err := os.WriteFile(idxPath, fake, 0o600); err != nil {
		t.Fatalf("write index: %v", err)
	}

	reopened, err := journal.Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got := len(reopened.Entries()); got != 1 {
		t.Fatalf("entries after rebuild = %d, want 1", got)
	}
	if err := reopened.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := journal.Verify(dir); err != nil {
		t.Fatalf("verify after rebuild: %v", err)
	}
}

func TestConcurrentAppendsKeepChainValid(t *testing.T) {
	dir := t.TempDir()
	j, err := journal.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	const writers = 40
	var wg sync.WaitGroup
	errs := make(chan error, writers)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := j.Append(regressionRecord(fmt.Sprintf("action-%d", i), map[string]any{
				"writer": int64(i),
			}))
			if err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent append: %v", err)
	}
	if err := j.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	result, err := journal.Verify(dir)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if result.Records != writers {
		t.Fatalf("verified records = %d, want %d", result.Records, writers)
	}
}
