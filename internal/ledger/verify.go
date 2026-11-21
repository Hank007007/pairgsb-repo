package ledger

import (
	"crypto/hmac"
	"encoding/hex"
	"fmt"
)

// VerifyReport is the outcome of recomputing a ledger from scratch.
type VerifyReport struct {
	CheckedRecords     int
	CheckedCheckpoints int
	Warnings           []string
	BadSeq             int64
	BadCheckpointSeq   int64
	Message            string
}

// OK reports whether the ledger is internally consistent.
func (r VerifyReport) OK() bool {
	return len(r.Warnings) == 0 && r.BadSeq == 0 && r.BadCheckpointSeq == 0
}

// Verify recomputes every record hash, walks the chain, and checks the stored
// checkpoints against the chain.
func Verify(path string, key []byte) (VerifyReport, error) {
	var report VerifyReport

	records, err := LoadRecords(path)
	if err != nil {
		return report, err
	}
	checkpoints, err := LoadCheckpoints(path)
	if err != nil {
		return report, err
	}

	var prev Record
	recordsBySeq := make(map[int64]Record, len(records))
	for i, r := range records {
		report.CheckedRecords++
		if HashRecord(r) != r.Hash {
			report.BadSeq = r.Seq
			return report, fmt.Errorf("record %d: hash mismatch", r.Seq)
		}
		if i == 0 {
			if r.Seq != 1 {
				report.BadSeq = r.Seq
				return report, fmt.Errorf("record sequence starts at %d, want 1", r.Seq)
			}
			if r.Prev != "" {
				report.BadSeq = r.Seq
				return report, fmt.Errorf("record %d: first record has non-empty prev", r.Seq)
			}
		} else {
			if r.Seq != prev.Seq+1 {
				report.BadSeq = r.Seq
				return report, fmt.Errorf("record %d: sequence gap after %d", r.Seq, prev.Seq)
			}
			if r.Prev != prev.Hash {
				report.BadSeq = r.Seq
				return report, fmt.Errorf("record %d: prev does not match preceding record hash", r.Seq)
			}
		}
		if r.Status != "" && !r.Status.Valid() {
			report.BadSeq = r.Seq
			return report, fmt.Errorf("record %d: invalid status %q", r.Seq, r.Status)
		}
		if r.Waiver != nil {
			if err := r.Waiver.Verify(key); err != nil {
				report.BadSeq = r.Seq
				return report, fmt.Errorf("record %d: invalid waiver: %w", r.Seq, err)
			}
		}
		recordsBySeq[r.Seq] = r
		prev = r
	}

	for _, cp := range checkpoints {
		report.CheckedCheckpoints++
		if err := verifyCheckpoint(key, cp); err != nil {
			report.BadCheckpointSeq = cp.Seq
			return report, fmt.Errorf("checkpoint at seq %d: %w", cp.Seq, err)
		}
		r, ok := recordsBySeq[cp.Seq]
		if !ok {
			report.BadCheckpointSeq = cp.Seq
			return report, fmt.Errorf("checkpoint at seq %d is past the end of the ledger", cp.Seq)
		}
		if r.Hash != cp.LastHash {
			report.BadCheckpointSeq = cp.Seq
			return report, fmt.Errorf("checkpoint at seq %d does not match the record hash", cp.Seq)
		}
	}

	report.Message = fmt.Sprintf("verified %d records, %d checkpoints",
		report.CheckedRecords, report.CheckedCheckpoints)
	return report, nil
}

func verifyCheckpoint(key []byte, cp Checkpoint) error {
	if cp.Signature == "" {
		return fmt.Errorf("signature is required")
	}
	got, err := hex.DecodeString(cp.Signature)
	if err != nil {
		return fmt.Errorf("signature is not valid hex: %w", err)
	}
	want := SignCheckpoint(key, cp)
	wantBytes, err := hex.DecodeString(want)
	if err != nil {
		return fmt.Errorf("compute signature: %w", err)
	}
	if !hmac.Equal(got, wantBytes) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}
