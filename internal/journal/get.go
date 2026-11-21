package journal

import (
	"fmt"

	"maskseal-journal/internal/record"
)

// Get returns the record with the given sequence number.
func (j *Journal) Get(seq int64) (record.Record, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, entry := range j.entries {
		if entry.Seq == seq {
			return readRecordAt(j.data, entry)
		}
	}
	return record.Record{}, fmt.Errorf("journal: record %d not found", seq)
}

// All returns every record in the journal, oldest first.
func (j *Journal) All() ([]record.Record, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	out := make([]record.Record, 0, len(j.entries))
	for _, entry := range j.entries {
		rec, err := readRecordAt(j.data, entry)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, nil
}
