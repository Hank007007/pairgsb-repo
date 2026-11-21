// Package chain implements the local append-only audit log.
//
// Each record commits to the hash of the record before it, so the log is a
// hash chain. Records are persisted as JSON lines: one record per line, in
// append order.
package chain

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Store is an append-only local audit log backed by a JSON-lines file.
//
// A Store keeps the decoded records in memory and appends new records to the
// file before acknowledging them.
type Store struct {
	mu      sync.RWMutex
	path    string
	file    *os.File
	records []Record
	now     func() time.Time
	usedTS  map[string]struct{}
}

const timestampLayout = "2006-01-02T15:04:05.000000000Z07:00"

// Option customises a Store.
type Option func(*Store)

// WithClock replaces the wall-clock source. It is intended for tests.
func WithClock(now func() time.Time) Option {
	return func(s *Store) {
		if now != nil {
			s.now = now
		}
	}
}

// Open loads the log at path, creating the file (and parent directory) when it
// does not exist yet.
func Open(path string, opts ...Option) (*Store, error) {
	s := &Store{path: path, now: time.Now}
	for _, opt := range opts {
		opt(s)
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create log directory: %w", err)
		}
	}
	records, err := readAll(path)
	if err != nil {
		return nil, err
	}
	s.records = records
	s.usedTS = make(map[string]struct{}, len(records))
	for _, rec := range records {
		s.usedTS[rec.Time] = struct{}{}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log: %w", err)
	}
	s.file = f
	return s, nil
}

func readAll(path string) ([]Record, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open log: %w", err)
	}
	defer f.Close()

	var out []Record
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	line := 0
	for sc.Scan() {
		line++
		raw := sc.Bytes()
		if len(raw) == 0 {
			continue
		}
		r, err := ParseRecord(raw)
		if err != nil {
			return nil, fmt.Errorf("log line %d: %w", line, err)
		}
		out = append(out, r)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read log: %w", err)
	}
	return out, nil
}

// Close releases the underlying file handle.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return nil
	}
	err := s.file.Close()
	s.file = nil
	return err
}

// Append adds an event to the log and returns the stored record.
func (s *Store) Append(actor, action, body string) (Record, error) {
	if actor == "" {
		return Record{}, errors.New("chain: actor is required")
	}
	if action == "" {
		return Record{}, errors.New("chain: action is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	seq := int64(1)
	prev := ""
	if n := len(s.records); n > 0 {
		last := s.records[n-1]
		seq = last.Seq + 1
		prev = last.Hash
	}

	ts := s.now().UTC()
	stamp := ts.Format(timestampLayout)
	for {
		if _, used := s.usedTS[stamp]; !used {
			break
		}
		ts = ts.Add(time.Nanosecond)
		stamp = ts.Format(timestampLayout)
	}
	rec := Record{
		Seq:    seq,
		Time:   stamp,
		Actor:  actor,
		Action: action,
		Body:   body,
		Prev:   prev,
	}
	rec.Hash = ComputeHash(rec)

	line, err := json.Marshal(rec)
	if err != nil {
		return Record{}, fmt.Errorf("encode record: %w", err)
	}
	line = append(line, '\n')
	if _, err := s.file.Write(line); err != nil {
		return Record{}, fmt.Errorf("write record: %w", err)
	}
	if err := s.file.Sync(); err != nil {
		return Record{}, fmt.Errorf("sync record: %w", err)
	}
	s.usedTS[stamp] = struct{}{}
	s.records = append(s.records, rec)
	return rec, nil
}

// Records returns a copy of every record in the log, in append order.
func (s *Store) Records() []Record {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Record, len(s.records))
	copy(out, s.records)
	return out
}

// Len reports how many records the log holds.
func (s *Store) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.records)
}

// HeadSeq returns the sequence number of the newest record, or 0 when the log
// is empty.
func (s *Store) HeadSeq() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.records) == 0 {
		return 0
	}
	return s.records[len(s.records)-1].Seq
}

// Head returns the newest record, if any.
func (s *Store) Head() (Record, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.records) == 0 {
		return Record{}, false
	}
	return s.records[len(s.records)-1], true
}

// Path returns the file backing the store.
func (s *Store) Path() string { return s.path }

// Leaves returns the raw record hashes keyed by sequence number.
func (s *Store) Leaves() map[int64][]byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int64][]byte, len(s.records))
	for _, rec := range s.records {
		if raw, err := hex.DecodeString(rec.Hash); err == nil {
			out[rec.Seq] = raw
		}
	}
	return out
}

// LeafRange returns the raw record hashes for [from, to], keyed by sequence.
func (s *Store) LeafRange(from, to int64) map[int64][]byte {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[int64][]byte)
	for _, rec := range s.records {
		if rec.Seq < from || rec.Seq > to {
			continue
		}
		if raw, err := hex.DecodeString(rec.Hash); err == nil {
			out[rec.Seq] = raw
		}
	}
	return out
}
