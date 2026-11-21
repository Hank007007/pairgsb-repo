package chain

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"witnesschain/internal/merkle"
)

// Finding describes one verification failure.
type Finding struct {
	Segment uint64 `json:"segment"`
	Seq     uint64 `json:"seq"`
	Kind    string `json:"kind"`
	Detail  string `json:"detail"`
}

func (f Finding) String() string {
	if f.Seq > 0 {
		return fmt.Sprintf("%s at segment %d seq %d: %s", f.Kind, f.Segment, f.Seq, f.Detail)
	}
	return fmt.Sprintf("%s at segment %d: %s", f.Kind, f.Segment, f.Detail)
}

// Result is the outcome of a full-store verification.
type Result struct {
	Segments int
	Records  uint64
	HeadHash string
	Findings []Finding
}

// OK reports whether verification found no problems.
func (r *Result) OK() bool { return len(r.Findings) == 0 }

// Verify walks a store and reports inconsistencies. See docs/verification.md.
func Verify(dir string) (*Result, error) {
	res := &Result{}
	ks := NewKeySet(filepath.Join(dir, "keys"))

	entries, _, err := ReadIndex(dir)
	if err != nil {
		return nil, err
	}

	// The index holds one line per appended record, so the last line seen for
	// a segment carries that segment's current count.
	counts := map[uint64]uint64{}
	var order []uint64
	for _, meta := range entries {
		if _, ok := counts[meta.Segment]; !ok {
			order = append(order, meta.Segment)
		}
		counts[meta.Segment] = meta.Count
	}

	var headSeq uint64
	for _, segNo := range order {
		logPath, ckptPath := SegmentPaths(dir, segNo)
		recs, _, err := (&Segment{Number: segNo, Dir: dir, LogPath: logPath}).ReadRecords()
		if err != nil {
			continue
		}
		for _, r := range recs {
			if err := r.Validate(); err != nil {
				res.Findings = append(res.Findings, Finding{
					Segment: segNo, Seq: r.Seq, Kind: "hash-mismatch", Detail: err.Error(),
				})
				break
			}
		}

		cp, err := ReadCheckpoint(ckptPath)
		if err != nil {
			continue
		}
		pub, err := ks.Public(cp.KeyID)
		if err != nil {
			res.Findings = append(res.Findings, Finding{
				Segment: segNo, Seq: cp.LastSeq, Kind: "unknown-key", Detail: err.Error(),
			})
			continue
		}
		if !cp.Verify(pub) {
			res.Findings = append(res.Findings, Finding{
				Segment: segNo, Seq: cp.LastSeq, Kind: "bad-signature",
				Detail: fmt.Sprintf("checkpoint for segment %d does not verify under key %q", segNo, cp.KeyID),
			})
		}
		if cp.LastSeq >= headSeq {
			headSeq = cp.LastSeq
			res.HeadHash = cp.Hash()
		}
		res.Segments++
		res.Records += counts[segNo]
	}
	return res, nil
}

// SegmentNumbers returns the segment numbers that have a checkpoint file,
// sorted ascending.
func SegmentNumbers(dir string) ([]uint64, error) {
	entries, err := os.ReadDir(filepath.Join(dir, "segments"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []uint64
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".ckpt") {
			continue
		}
		n, err := strconv.ParseUint(strings.TrimSuffix(name, ".ckpt"), 10, 64)
		if err != nil {
			continue
		}
		out = append(out, n)
	}
	return out, nil
}

// RootOf returns the Merkle root over the given records.
func RootOf(records []*Record) (string, error) {
	if len(records) == 0 {
		return "", merkle.ErrEmptyTree
	}
	hashes := make([]string, len(records))
	for i, r := range records {
		hashes[i] = r.Hash
	}
	t, err := merkle.New(hashes)
	if err != nil {
		return "", err
	}
	return t.Root(), nil
}

// ProofFor returns an inclusion proof for record i of records.
func ProofFor(records []*Record, i int) ([]merkle.Step, error) {
	hashes := make([]string, len(records))
	for j, r := range records {
		hashes[j] = r.Hash
	}
	t, err := merkle.New(hashes)
	if err != nil {
		return nil, err
	}
	return t.Proof(i)
}
