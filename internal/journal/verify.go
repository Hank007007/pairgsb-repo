package journal

import (
	"bytes"
	"fmt"

	"maskseal-journal/internal/record"
)

// VerifyResult describes the outcome of a verification pass.
type VerifyResult struct {
	// Records is the number of records examined.
	Records int
	// LastSeq is the sequence number of the last record reached.
	LastSeq int64
	// FirstBadSeq is the sequence number of the first record that failed a
	// check, or 0 when every record was valid.
	FirstBadSeq int64
}

// Verify walks the journal and checks the chain. It returns the first failing
// sequence number and a non-nil error when any check fails.
func Verify(dir string) (VerifyResult, error) {
	var result VerifyResult
	j, err := Open(dir)
	if err != nil {
		return result, err
	}
	defer j.Close()

	info, err := j.data.Stat()
	if err != nil {
		return result, fmt.Errorf("journal: %w", err)
	}
	fileSize := info.Size()
	entries := j.Entries()
	prev := record.GenesisHash
	expectedSeq := int64(1)

	for _, entry := range entries {
		result.Records++
		result.LastSeq = entry.Seq
		if entry.Offset < 0 || entry.Length <= 0 || entry.Offset+entry.Length > fileSize {
			return failed(result, entry.Seq, "index entry points past end of data")
		}
		buf := make([]byte, entry.Length)
		if _, err := j.data.ReadAt(buf, entry.Offset); err != nil {
			return failed(result, entry.Seq, "read record: %v", err)
		}
		line := bytes.TrimSuffix(buf, []byte{'\n'})
		rec, err := record.Decode(line)
		if err != nil {
			return failed(result, entry.Seq, "%v", err)
		}
		result.LastSeq = rec.Seq

		if rec.Seq != expectedSeq {
			return failed(result, rec.Seq, "sequence jumps from %d", expectedSeq-1)
		}
		if rec.PrevHash != prev {
			return failed(result, rec.Seq, "prev_hash does not match the previous record")
		}
		canonical, err := record.Encode(rec)
		if err != nil {
			return failed(result, rec.Seq, "%v", err)
		}
		if !bytes.Equal(canonical, line) {
			return failed(result, rec.Seq, "record is not canonically encoded")
		}
		if want := record.Hash(rec); rec.Hash != want {
			return failed(result, rec.Seq, "stored hash does not match the record contents")
		}

		prev = rec.Hash
		expectedSeq = rec.Seq + 1
	}
	return result, nil
}

func failed(result VerifyResult, seq int64, format string, args ...any) (VerifyResult, error) {
	result.FirstBadSeq = seq
	return result, fmt.Errorf("journal: seq %d: %s", seq, fmt.Sprintf(format, args...))
}
