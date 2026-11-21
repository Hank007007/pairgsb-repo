package ledger

import (
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
	return len(r.Warnings) == 0
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
	for i, r := range records {
		report.CheckedRecords++
		if HashRecord(r) != r.Hash {
			report.BadSeq = r.Seq
			return report, fmt.Errorf("record %d: hash mismatch", r.Seq)
		}
		if i > 0 && r.Seq <= prev.Seq {
			report.Warnings = append(report.Warnings,
				fmt.Sprintf("sequence went backwards at seq %d (previous %d)", r.Seq, prev.Seq))
		}
		prev = r
	}

	for _, cp := range checkpoints {
		report.CheckedCheckpoints++
		if cp.Seq > int64(len(records)) {
			report.BadCheckpointSeq = cp.Seq
			return report, fmt.Errorf("checkpoint at seq %d is past the end of the ledger", cp.Seq)
		}
		if records[cp.Seq-1].Hash != cp.LastHash {
			report.BadCheckpointSeq = cp.Seq
			return report, fmt.Errorf("checkpoint at seq %d does not match the record hash", cp.Seq)
		}
	}

	report.Message = fmt.Sprintf("verified %d records, %d checkpoints",
		report.CheckedRecords, report.CheckedCheckpoints)
	return report, nil
}
