// Package journal appends records to a hash-chained log and verifies it.
package journal

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

// Open opens or creates the journal in dir. The data file is the source of
// truth: the index is rebuilt from complete records on every open, and an
// incomplete tail is discarded.
func Open(dir string) (*Journal, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("journal: %w", err)
	}
	data, err := os.OpenFile(filepath.Join(dir, DataFileName), os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, fmt.Errorf("journal: %w", err)
	}
	entries, seq, prev, offset, err := rebuildIndex(dir, data)
	if err != nil {
		data.Close()
		return nil, err
	}
	if _, err := data.Seek(offset, io.SeekStart); err != nil {
		data.Close()
		return nil, fmt.Errorf("journal: %w", err)
	}
	return &Journal{
		dir:     dir,
		data:    data,
		seq:     seq,
		prev:    prev,
		offset:  offset,
		entries: entries,
	}, nil
}

// rebuildIndex scans the data file for complete newline-terminated records,
// truncates a partial tail, and rewrites the derived index.
func rebuildIndex(dir string, data *os.File) ([]Entry, int64, string, int64, error) {
	if _, err := data.Seek(0, io.SeekStart); err != nil {
		return nil, 0, "", 0, fmt.Errorf("journal: %w", err)
	}
	raw, err := io.ReadAll(data)
	if err != nil {
		return nil, 0, "", 0, fmt.Errorf("journal: %w", err)
	}

	var entries []Entry
	var offset int64
	seq := int64(0)
	prev := record.GenesisHash
	for offset < int64(len(raw)) {
		nl := bytes.IndexByte(raw[offset:], '\n')
		if nl < 0 {
			break
		}
		line := raw[offset : offset+int64(nl)]
		rec, err := record.Decode(line)
		if err != nil {
			return nil, 0, "", 0, fmt.Errorf("journal: seq %d: %w", len(entries)+1, err)
		}
		length := int64(nl) + 1
		entries = append(entries, Entry{Seq: rec.Seq, Offset: offset, Length: length})
		seq = rec.Seq
		prev = rec.Hash
		offset += length
	}
	if offset < int64(len(raw)) {
		if err := data.Truncate(offset); err != nil {
			return nil, 0, "", 0, fmt.Errorf("journal: truncate partial record: %w", err)
		}
		if err := data.Sync(); err != nil {
			return nil, 0, "", 0, fmt.Errorf("journal: %w", err)
		}
	}
	if err := writeIndexFile(dir, entries); err != nil {
		return nil, 0, "", 0, err
	}
	return entries, seq, prev, offset, nil
}

func writeIndexFile(dir string, entries []Entry) error {
	f, err := os.OpenFile(filepath.Join(dir, IndexFileName), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("journal: %w", err)
	}
	w := bufio.NewWriter(f)
	for _, entry := range entries {
		if _, err := fmt.Fprintf(w, "%d %d %d\n", entry.Seq, entry.Offset, entry.Length); err != nil {
			f.Close()
			return fmt.Errorf("journal: %w", err)
		}
	}
	if err := w.Flush(); err != nil {
		f.Close()
		return fmt.Errorf("journal: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return fmt.Errorf("journal: %w", err)
	}
	return f.Close()
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
	if j.data != nil {
		if err := j.data.Sync(); err != nil && first == nil {
			first = err
		}
		if err := j.data.Close(); err != nil && first == nil {
			first = err
		}
	}
	if j.index != nil {
		if err := j.index.Sync(); err != nil && first == nil {
			first = err
		}
		if err := j.index.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// Append seals r into the journal and returns the stored record.
func (j *Journal) Append(r record.Record) (record.Record, error) {
	j.mu.Lock()
	defer j.mu.Unlock()

	r.Seq = j.seq + 1
	r.PrevHash = j.prev
	r.Hash = record.Hash(r)
	line, err := record.Encode(r)
	if err != nil {
		return record.Record{}, err
	}

	payload := make([]byte, len(line)+1)
	copy(payload, line)
	payload[len(line)] = '\n'
	offset := j.offset
	if err := writeFullAt(j.data, payload, offset); err != nil {
		_ = j.data.Truncate(offset)
		_ = j.data.Sync()
		return record.Record{}, fmt.Errorf("journal: %w", err)
	}

	entry := Entry{Seq: r.Seq, Offset: offset, Length: int64(len(payload))}
	j.entries = append(j.entries, entry)
	j.seq = r.Seq
	j.prev = r.Hash
	j.offset += entry.Length

	// The index must never describe a record whose bytes are not present in
	// the data file. Sync the data before making the index entry visible.
	if err := j.data.Sync(); err != nil {
		return r, fmt.Errorf("journal: %w", err)
	}
	if err := j.writeIndexEntry(entry); err != nil {
		return r, err
	}
	return r, nil
}

func writeFullAt(f *os.File, buf []byte, offset int64) error {
	for len(buf) > 0 {
		n, err := f.WriteAt(buf, offset)
		offset += int64(n)
		buf = buf[n:]
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
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
	if _, err := io.WriteString(j.index, line); err != nil {
		return fmt.Errorf("journal: %w", err)
	}
	if err := j.index.Sync(); err != nil {
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
