// Package journal appends records to a hash-chained log and verifies it.
package journal

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"maskseal-journal/internal/record"
)

// DataFileName is the log file inside a journal directory.
const DataFileName = "journal.log"

// IndexFileName is the index file inside a journal directory.
const IndexFileName = "journal.idx"

// Journal is an append-only record log backed by a directory.
type Journal struct {
	dir     string
	data    *os.File
	index   *os.File
	writer  *bufio.Writer
	mu      sync.Mutex
	seq     int64
	prev    string
	offset  int64
	entries []Entry
}

// Entry is one line of the index: sequence number, byte offset, byte length.
type Entry struct {
	Seq    int64
	Offset int64
	Length int64
}

// Open opens or creates the journal in dir.
func Open(dir string) (*Journal, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("journal: %w", err)
	}
	j := &Journal{dir: dir, prev: record.GenesisHash}
	if err := j.loadIndex(); err != nil {
		return nil, err
	}
	data, err := os.OpenFile(filepath.Join(dir, DataFileName), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("journal: %w", err)
	}
	j.data = data
	j.writer = bufio.NewWriter(data)
	if len(j.entries) > 0 {
		last := j.entries[len(j.entries)-1]
		j.seq = last.Seq
		j.offset = last.Offset + last.Length
		rec, err := readRecordAt(data, last)
		if err != nil {
			return nil, err
		}
		j.prev = rec.Hash
	}
	if _, err := data.Seek(j.offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("journal: %w", err)
	}
	return j, nil
}

func (j *Journal) loadIndex() error {
	raw, err := os.ReadFile(filepath.Join(j.dir, IndexFileName))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("journal: %w", err)
	}
	for _, line := range strings.Split(strings.TrimRight(string(raw), "\r\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(strings.TrimSpace(line), " ")
		if len(parts) != 3 {
			return fmt.Errorf("journal: corrupt index line %q", line)
		}
		seq, err1 := strconv.ParseInt(parts[0], 10, 64)
		off, err2 := strconv.ParseInt(parts[1], 10, 64)
		length, err3 := strconv.ParseInt(parts[2], 10, 64)
		if err1 != nil || err2 != nil || err3 != nil {
			return fmt.Errorf("journal: corrupt index line %q", line)
		}
		j.entries = append(j.entries, Entry{Seq: seq, Offset: off, Length: length})
	}
	return nil
}

func readRecordAt(f *os.File, e Entry) (record.Record, error) {
	buf := make([]byte, e.Length)
	if _, err := f.ReadAt(buf, e.Offset); err != nil {
		return record.Record{}, fmt.Errorf("journal: read seq %d: %w", e.Seq, err)
	}
	return record.Decode(buf)
}

// Close flushes and closes the journal files.
func (j *Journal) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	var first error
	if j.writer != nil {
		if err := j.writer.Flush(); err != nil && first == nil {
			first = err
		}
	}
	if j.data != nil {
		if err := j.data.Close(); err != nil && first == nil {
			first = err
		}
	}
	if j.index != nil {
		if err := j.index.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// Append seals r into the journal and returns the stored record.
func (j *Journal) Append(r record.Record) (record.Record, error) {
	prev := j.prev
	j.mu.Lock()
	defer j.mu.Unlock()

	r.Seq = j.seq + 1
	r.PrevHash = prev
	r.Hash = record.Hash(r)
	line, err := record.Encode(r)
	if err != nil {
		return record.Record{}, err
	}

	offset := j.offset
	if _, err := j.writer.Write(line); err != nil {
		return record.Record{}, fmt.Errorf("journal: %w", err)
	}
	if err := j.writer.WriteByte('\n'); err != nil {
		return record.Record{}, fmt.Errorf("journal: %w", err)
	}
	entry := Entry{Seq: r.Seq, Offset: offset, Length: int64(len(line) + 1)}
	if err := j.writeIndexEntry(entry); err != nil {
		return record.Record{}, err
	}
	j.entries = append(j.entries, entry)
	j.seq = r.Seq
	j.prev = r.Hash
	j.offset += entry.Length
	return r, nil
}

func (j *Journal) writeIndexEntry(e Entry) error {
	if j.index == nil {
		f, err := os.OpenFile(filepath.Join(j.dir, IndexFileName), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
		if err != nil {
			return fmt.Errorf("journal: %w", err)
		}
		j.index = f
	}
	line := fmt.Sprintf("%d %d %d\n", e.Seq, e.Offset, e.Length)
	if _, err := j.index.WriteString(line); err != nil {
		return fmt.Errorf("journal: %w", err)
	}
	return nil
}

// Entries returns the index entries loaded so far.
func (j *Journal) Entries() []Entry {
	j.mu.Lock()
	defer j.mu.Unlock()
	out := make([]Entry, len(j.entries))
	copy(out, j.entries)
	return out
}

// Len returns the number of records in the journal.
func (j *Journal) Len() int {
	j.mu.Lock()
	defer j.mu.Unlock()
	return len(j.entries)
}

// Dir reports the journal directory.
func (j *Journal) Dir() string {
	return j.dir
}
