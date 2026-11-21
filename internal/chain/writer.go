package chain

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DefaultSegmentLimit is the number of records a segment holds before it is
// sealed and a new one is started.
const DefaultSegmentLimit = 64

// logWriteBuffer sizes the buffered log writer.
const logWriteBuffer = 16 * 1024

func nowUTC() time.Time { return time.Now().UTC() }

// Writer appends records to a store.
//
// One Writer may be shared by several goroutines. A Writer must be Closed
// before the store is verified.
type Writer struct {
	mu       sync.RWMutex
	ioMu     sync.Mutex
	dir      string
	limit    uint64
	key      *KeyPair
	seg      *Segment
	logFile  *os.File
	buf      *bufio.Writer
	segCount uint64
	lastHash string
	lastSeq  uint64
	closed   bool
}

// OpenWriter opens the store at dir for appending, creating it if needed.
//
// If the highest-numbered segment is already sealed, a new segment is started.
// If it is not sealed, the writer resumes it, discarding an incomplete final
// line (docs/chain-format.md §10.3).
func OpenWriter(dir string, limit uint64, kp *KeyPair) (*Writer, error) {
	if limit == 0 {
		limit = DefaultSegmentLimit
	}
	if kp == nil {
		return nil, fmt.Errorf("writer: signing key is required")
	}
	if err := os.MkdirAll(filepath.Join(dir, "segments"), 0o755); err != nil {
		return nil, err
	}
	w := &Writer{dir: dir, limit: limit, key: kp, lastHash: ZeroHash}

	last, err := highestSegment(dir)
	if err != nil {
		return nil, err
	}

	segNo := uint64(1)
	if last > 0 {
		logPath, ckptPath := SegmentPaths(dir, last)
		recs, err := readComplete(logPath)
		if err != nil {
			return nil, err
		}
		if len(recs) > 0 {
			w.lastHash = recs[len(recs)-1].Hash
			w.lastSeq = recs[len(recs)-1].Seq
			w.segCount = uint64(len(recs))
		}
		if _, err := os.Stat(ckptPath); err == nil {
			// Sealed: new records go to a fresh segment.
			segNo = last + 1
			w.segCount = 0
			w.lastHash = ZeroHash
		} else {
			segNo = last
			if err := truncateIncompleteTail(logPath); err != nil {
				return nil, err
			}
		}
	}

	seg, err := OpenSegment(dir, segNo, true)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(seg.LogPath, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	w.seg = seg
	w.logFile = f
	w.buf = bufio.NewWriterSize(f, logWriteBuffer)
	return w, nil
}

// highestSegment returns the largest segment number with a log file.
func highestSegment(dir string) (uint64, error) {
	entries, err := os.ReadDir(filepath.Join(dir, "segments"))
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	var last uint64
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".log") {
			continue
		}
		n, err := strconv.ParseUint(strings.TrimSuffix(name, ".log"), 10, 64)
		if err != nil {
			continue
		}
		if n > last {
			last = n
		}
	}
	return last, nil
}

// readComplete reads the records of a log file, stopping at the first line
// that does not parse.
func readComplete(logPath string) ([]*Record, error) {
	b, err := os.ReadFile(logPath)
	if err != nil {
		return nil, err
	}
	var recs []*Record
	for _, line := range bytes.Split(b, []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		r, err := ParseRecord(line)
		if err != nil {
			break
		}
		recs = append(recs, r)
	}
	return recs, nil
}

// truncateIncompleteTail removes a final line that has no terminating LF.
func truncateIncompleteTail(logPath string) error {
	f, err := os.OpenFile(logPath, os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return err
	}
	size := st.Size()
	if size == 0 {
		return nil
	}
	const window = 64 * 1024
	for off := size; off > 0; {
		start := off - window
		if start < 0 {
			start = 0
		}
		buf := make([]byte, off-start)
		if _, err := f.ReadAt(buf, start); err != nil {
			return err
		}
		if i := bytes.LastIndexByte(buf, '\n'); i >= 0 {
			if start+int64(i)+1 == size {
				return nil
			}
			return f.Truncate(start + int64(i) + 1)
		}
		off = start
	}
	return nil
}

// Append writes one record and returns it.
func (w *Writer) Append(actor, action, payload, outcome string) (*Record, error) {
	w.mu.RLock()
	prev := w.lastHash
	seq := w.lastSeq + 1
	w.mu.RUnlock()

	r := NewRecord(seq, nowUTC(), actor, action, payload, outcome, prev)
	line, err := r.EncodeLine()
	if err != nil {
		return nil, err
	}
	w.ioMu.Lock()
	_, err = w.buf.Write(line)
	w.ioMu.Unlock()
	if err != nil {
		return nil, err
	}

	w.mu.Lock()
	w.lastHash = r.Hash
	w.lastSeq = seq
	w.segCount++
	count := w.segCount
	seg := w.seg
	w.mu.Unlock()

	w.ioMu.Lock()
	err = AppendIndex(w.dir, SegmentMeta{
		Segment:    seg.Number,
		File:       filepath.Base(seg.LogPath),
		Checkpoint: filepath.Base(seg.Checkpoint),
		Count:      count,
		LastSeq:    seq,
	})
	w.ioMu.Unlock()
	if err != nil {
		return nil, err
	}

	if count == w.limit {
		if _, err := w.sealCurrent(true); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// sealCurrent flushes and seals the current segment. When rotate is true it
// starts the next segment.
func (w *Writer) sealCurrent(rotate bool) (*Checkpoint, error) {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil, fmt.Errorf("writer: closed")
	}
	if w.segCount == 0 {
		seg := w.seg
		err := w.logFile.Close()
		w.logFile = nil
		w.closed = true
		w.mu.Unlock()
		if err != nil {
			return nil, err
		}
		// The segment never received a record; leave no empty log behind.
		if err := os.Remove(seg.LogPath); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		return nil, nil
	}
	buf := w.buf
	logFile := w.logFile
	seg := w.seg
	w.mu.Unlock()

	if err := buf.Flush(); err != nil {
		return nil, err
	}
	if err := logFile.Sync(); err != nil {
		return nil, err
	}
	if err := logFile.Close(); err != nil {
		return nil, err
	}

	recs, err := readComplete(seg.LogPath)
	if err != nil {
		return nil, err
	}
	cp, err := seg.Seal(recs, w.key)
	if err != nil {
		return nil, err
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	w.logFile = nil
	if !rotate {
		w.closed = true
		return cp, nil
	}

	next, err := OpenSegment(w.dir, seg.Number+1, true)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(next.LogPath, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	w.seg = next
	w.logFile = f
	w.buf = bufio.NewWriterSize(f, logWriteBuffer)
	w.segCount = 0
	w.lastHash = ZeroHash
	return cp, nil
}

// Close flushes, seals the current segment, and closes the log file.
func (w *Writer) Close() error {
	_, err := w.sealCurrent(false)
	return err
}

// Dir returns the store directory.
func (w *Writer) Dir() string { return w.dir }

// LastSeq returns the sequence number of the most recently appended record.
func (w *Writer) LastSeq() uint64 {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.lastSeq
}
