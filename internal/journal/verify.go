package journal

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"maskseal-journal/internal/record"
)

// VerifyResult describes the outcome of a verification pass.
type VerifyResult struct {
	// Records is the number of index entries examined.
	Records int
	// LastSeq is the sequence number of the last record reached.
	LastSeq int64
	// FirstBadSeq is the sequence number of the first record that did not
	// match its stored hash, or 0 when every record matched.
	FirstBadSeq int64
}

// Verify walks the journal and checks the chain.
func Verify(dir string) (VerifyResult, error) {
	var result VerifyResult
	journal, err := Open(dir)
	if err != nil {
		return result, err
	}
	defer journal.Close()

	dataPath := filepath.Join(dir, DataFileName)
	data, err := os.Open(dataPath)
	if err != nil {
		return result, fmt.Errorf("journal: %w", err)
	}
	defer data.Close()

	info, err := data.Stat()
	if err != nil {
		return result, fmt.Errorf("journal: %w", err)
	}
	fileSize := info.Size()

	prev := record.GenesisHash
	for _, entry := range journal.Entries() {
		result.Records++
		result.LastSeq = entry.Seq

		if entry.Offset+entry.Length > fileSize {
			return result, fmt.Errorf("journal: index entry for seq %d points past end of data", entry.Seq)
		}
		buf := make([]byte, entry.Length)
		if _, err := data.ReadAt(buf, entry.Offset); err != nil {
			return result, fmt.Errorf("journal: read seq %d: %w", entry.Seq, err)
		}
		rec, err := record.Decode(trimNewline(buf))
		if err != nil {
			return result, fmt.Errorf("journal: seq %d: %w", entry.Seq, err)
		}
		if rec.PrevHash != prev {
			return result, fmt.Errorf("journal: seq %d: prev_hash does not match the previous record", rec.Seq)
		}
		want := record.Hash(rec)
		if rec.Hash != want {
			log.Printf("journal: record %d does not match its hash", rec.Seq)
			result.FirstBadSeq = rec.Seq
			break
		}
		prev = rec.Hash
	}
	return result, nil
}

func trimNewline(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}
