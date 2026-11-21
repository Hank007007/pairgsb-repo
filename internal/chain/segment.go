package chain

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// SegmentMeta is one line of segments.idx (docs/chain-format.md §8.1).
type SegmentMeta struct {
	Segment    uint64 `json:"segment"`
	File       string `json:"file"`
	Checkpoint string `json:"checkpoint"`
	Count      uint64 `json:"count"`
	LastSeq    uint64 `json:"last_seq"`
}

// SegmentPaths returns the log and checkpoint paths for segment n.
func SegmentPaths(dir string, n uint64) (logPath, ckptPath string) {
	base := fmt.Sprintf("%06d", n)
	return filepath.Join(dir, "segments", base+".log"), filepath.Join(dir, "segments", base+".ckpt")
}

// Segment is an on-disk segment being written or read.
type Segment struct {
	Number     uint64
	Dir        string
	LogPath    string
	Checkpoint string
	Count      uint64
}

// OpenSegment returns the segment descriptor for number n in store dir,
// creating the log file if create is true.
func OpenSegment(dir string, n uint64, create bool) (*Segment, error) {
	logPath, ckptPath := SegmentPaths(dir, n)
	if create {
		if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
			return nil, err
		}
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, err
		}
		f.Close()
	}
	return &Segment{Number: n, Dir: dir, LogPath: logPath, Checkpoint: ckptPath}, nil
}

// ReadRecords reads the complete record lines of the segment log.
//
// Lines that do not parse are returned in bad so that callers can decide how
// to report them; the returned records contain only lines that parsed.
func (s *Segment) ReadRecords() (records []*Record, bad []int, err error) {
	f, err := os.Open(s.LogPath)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	lineNo := 0
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			bad = append(bad, lineNo)
			lineNo++
			continue
		}
		r, perr := ParseRecord(line)
		if perr != nil {
			bad = append(bad, lineNo)
			lineNo++
			continue
		}
		records = append(records, r)
		lineNo++
	}
	if err := sc.Err(); err != nil {
		return nil, nil, err
	}
	return records, bad, nil
}

// Seal computes the Merkle root over records, signs a checkpoint with kp, and
// writes it next to the segment log. It also rewrites segments/HEAD.ckpt.
func (s *Segment) Seal(records []*Record, kp *KeyPair) (*Checkpoint, error) {
	if len(records) == 0 {
		return nil, fmt.Errorf("segment %d: cannot seal an empty segment", s.Number)
	}
	hashes := make([]string, len(records))
	for i, r := range records {
		hashes[i] = r.Hash
	}
	tree, err := newMerkleTree(hashes)
	if err != nil {
		return nil, err
	}
	cp := &Checkpoint{
		Version:        1,
		Segment:        s.Number,
		Count:          uint64(len(records)),
		FirstSeq:       records[0].Seq,
		LastSeq:        records[len(records)-1].Seq,
		Root:           tree.Root(),
		PrevCheckpoint: ZeroHash,
	}
	cp.Sign(kp.Prv, kp.ID)

	if err := WriteCheckpoint(s.Checkpoint, cp); err != nil {
		return nil, err
	}
	headPath := filepath.Join(s.Dir, "segments", "HEAD.ckpt")
	if err := WriteCheckpoint(headPath, cp); err != nil {
		return nil, err
	}
	s.Count = cp.Count
	return cp, nil
}

// WriteCheckpoint writes cp as one compact JSON line.
func WriteCheckpoint(path string, cp *Checkpoint) error {
	b, err := json.Marshal(cp)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ReadCheckpoint reads a checkpoint file.
func ReadCheckpoint(path string) (*Checkpoint, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cp Checkpoint
	if err := json.Unmarshal(b, &cp); err != nil {
		return nil, fmt.Errorf("parse checkpoint %s: %w", filepath.Base(path), err)
	}
	return &cp, nil
}

// AppendIndex appends one line to segments.idx.
func AppendIndex(dir string, meta SegmentMeta) error {
	b, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	f, err := os.OpenFile(filepath.Join(dir, "segments.idx"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(b)
	return err
}

// ReadIndex reads segments.idx as a list of entries. Lines that do not parse
// are returned separately.
func ReadIndex(dir string) (entries []SegmentMeta, bad []int, err error) {
	b, err := os.ReadFile(filepath.Join(dir, "segments.idx"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	lineNo := 0
	start := 0
	for i := 0; i <= len(b); i++ {
		if i < len(b) && b[i] != '\n' {
			continue
		}
		line := b[start:i]
		if len(line) > 0 {
			var m SegmentMeta
			if json.Unmarshal(line, &m) == nil {
				entries = append(entries, m)
			} else {
				bad = append(bad, lineNo)
			}
		}
		start = i + 1
		lineNo++
	}
	return entries, bad, nil
}
