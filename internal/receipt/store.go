// Package receipt stores, loads and verifies timestamp receipts.
package receipt

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Receipt is one timestamp authority response bound to a record interval.
type Receipt struct {
	FromSeq int64  `json:"fromSeq"`
	ToSeq   int64  `json:"toSeq"`
	Root    string `json:"root"`
	GenTime string `json:"genTime"`
	Sig     string `json:"sig"`
	TSA     string `json:"tsa"`
}

// FileName is the conventional file name for a receipt covering an interval.
func FileName(from, to int64) string {
	return fmt.Sprintf("%06d-%06d.json", from, to)
}

// Dir is a directory of receipts on disk.
type Dir struct {
	path string
}

// OpenDir prepares dir for use, creating it when missing.
func OpenDir(path string) (*Dir, error) {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, fmt.Errorf("create receipt directory: %w", err)
	}
	return &Dir{path: path}, nil
}

// Path returns the backing directory.
func (d *Dir) Path() string { return d.path }

// Save writes r to a file named after its interval.
func (d *Dir) Save(r Receipt) (string, error) {
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode receipt: %w", err)
	}
	raw = append(raw, '\n')
	path := filepath.Join(d.path, FileName(r.FromSeq, r.ToSeq))
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return "", fmt.Errorf("write receipt: %w", err)
	}
	return path, nil
}

// LoadAll reads every *.json file in the directory and returns the receipts
// ordered by FromSeq.
func (d *Dir) LoadAll() ([]Receipt, error) {
	entries, err := os.ReadDir(d.path)
	if err != nil {
		return nil, fmt.Errorf("read receipt directory: %w", err)
	}
	var out []Receipt
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(d.path, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", entry.Name(), err)
		}
		var r Receipt
		if err := json.Unmarshal(raw, &r); err != nil {
			return nil, fmt.Errorf("decode %s: %w", entry.Name(), err)
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].FromSeq != out[j].FromSeq {
			return out[i].FromSeq < out[j].FromSeq
		}
		return out[i].ToSeq < out[j].ToSeq
	})
	return out, nil
}
