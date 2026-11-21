package ledger

import (
	"bufio"
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

// Status is the lifecycle state of a finding.
type Status string

const (
	StatusOpen     Status = "open"
	StatusWaived   Status = "waived"
	StatusFixed    Status = "fixed"
	StatusReopened Status = "reopened"
)

// Valid reports whether s is one of the four ledger states.
func (s Status) Valid() bool {
	switch s {
	case StatusOpen, StatusWaived, StatusFixed, StatusReopened:
		return true
	}
	return false
}

// Severity is the scanner-assigned severity of a finding.
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// IsHigh reports whether the severity belongs to the high-risk band.
func (s Severity) IsHigh() bool {
	return s == SeverityHigh || s == SeverityCritical
}

// RecordKind names the reason a ledger record was written.
type RecordKind string

const (
	KindObservation RecordKind = "observation"
	KindResolved    RecordKind = "resolved"
	KindReopened    RecordKind = "reopened"
	KindWaiver      RecordKind = "waiver"
)

// Record is one line of the append-only ledger.
type Record struct {
	Seq         int64      `json:"seq"`
	Kind        RecordKind `json:"kind"`
	Fingerprint string     `json:"fingerprint,omitempty"`
	FindingID   string     `json:"findingId,omitempty"`
	RuleID      string     `json:"ruleId,omitempty"`
	File        string     `json:"file,omitempty"`
	Line        int        `json:"line,omitempty"`
	Status      Status     `json:"status,omitempty"`
	Waiver      *Waiver    `json:"waiver,omitempty"`
	At          time.Time  `json:"at"`
	Prev        string     `json:"prev"`
	Hash        string     `json:"hash"`
}

// Finding is a single scanner result as submitted in a scan file.
type Finding struct {
	RuleID   string   `json:"ruleId"`
	File     string   `json:"file"`
	Line     int      `json:"line"`
	Snippet  string   `json:"snippet,omitempty"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message,omitempty"`
}

// Input converts the finding into the value the fingerprint algorithm reads.
func (f Finding) Input() FingerprintInput {
	return FingerprintInput{
		RuleID:  f.RuleID,
		File:    f.File,
		Line:    f.Line,
		Snippet: f.Snippet,
	}
}

// Fingerprint returns the finding's stable identity.
func (f Finding) Fingerprint() string {
	return Compute(f.Input())
}

// Scan is the on-disk shape of a scanner export.
type Scan struct {
	Findings []Finding `json:"findings"`
}

// LoadScan reads a scanner export file.
func LoadScan(path string) ([]Finding, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read scan %s: %w", path, err)
	}
	var scan Scan
	if err := json.Unmarshal(data, &scan); err != nil {
		return nil, fmt.Errorf("decode scan %s: %w", path, err)
	}
	return scan.Findings, nil
}

// FindingID is the short human-facing identifier stored on ledger records.
func FindingID(fingerprint string) string {
	if len(fingerprint) > 12 {
		return fingerprint[:12]
	}
	return fingerprint
}

// HashRecord computes the record hash over the canonical JSON encoding of the
// record with the hash field cleared.
func HashRecord(r Record) string {
	r.Hash = ""
	data, err := json.Marshal(r)
	if err != nil {
		// A record contains only JSON-marshalable values.
		panic(err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Checkpoint anchors the hash chain every DefaultCheckpointEvery records.
type Checkpoint struct {
	Seq       int64     `json:"seq"`
	LastHash  string    `json:"lastHash"`
	At        time.Time `json:"at"`
	Signature string    `json:"signature"`
}

// DefaultCheckpointEvery is the checkpoint interval used by the CLI.
const DefaultCheckpointEvery = 16

// Ledger is an append-only sequence of records backed by a JSONL file.
type Ledger struct {
	path            string
	key             []byte
	checkpointEvery int
	records         []Record
}

// Open loads an existing ledger file, creating an empty ledger when the file
// does not exist yet.
func Open(path string, key []byte) (*Ledger, error) {
	records, err := LoadRecords(path)
	if err != nil {
		return nil, err
	}
	return &Ledger{
		path:            path,
		key:             key,
		checkpointEvery: DefaultCheckpointEvery,
		records:         records,
	}, nil
}

// LoadRecords reads every record from a ledger file.
func LoadRecords(path string) ([]Record, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open ledger %s: %w", path, err)
	}
	defer f.Close()

	var records []Record
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var r Record
		if err := json.Unmarshal(line, &r); err != nil {
			return nil, fmt.Errorf("decode record in %s: %w", path, err)
		}
		records = append(records, r)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read ledger %s: %w", path, err)
	}
	return records, nil
}

// Path returns the ledger file location.
func (l *Ledger) Path() string { return l.path }

// Records returns the records currently loaded in memory.
func (l *Ledger) Records() []Record { return l.records }

// SetCheckpointEvery changes the checkpoint interval. A value of zero disables
// checkpoint writing.
func (l *Ledger) SetCheckpointEvery(n int) { l.checkpointEvery = n }

// NextSeq returns the sequence number the next record is expected to carry.
func (l *Ledger) NextSeq() int64 {
	return int64(len(l.records)) + 1
}

func (l *Ledger) prevHash() string {
	if len(l.records) == 0 {
		return ""
	}
	return l.records[len(l.records)-1].Hash
}

// NewRecord allocates a record stamped with the next seq, the current chain
// head, and the current time.
func (l *Ledger) NewRecord(kind RecordKind) Record {
	return Record{
		Seq:  l.NextSeq(),
		Kind: kind,
		At:   time.Now().UTC(),
		Prev: l.prevHash(),
	}
}

// Append writes a record to the ledger file and updates the in-memory view.
func (l *Ledger) Append(r Record) error {
	r.Hash = HashRecord(r)
	line, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("encode record: %w", err)
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open ledger %s: %w", l.path, err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		f.Close()
		return fmt.Errorf("append ledger %s: %w", l.path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close ledger %s: %w", l.path, err)
	}
	l.records = append(l.records, r)
	if l.checkpointEvery > 0 && r.Seq%int64(l.checkpointEvery) == 0 {
		return l.writeCheckpoint(r)
	}
	return nil
}

// CheckpointPath returns the sidecar file holding checkpoints for a ledger.
func CheckpointPath(ledgerPath string) string {
	return ledgerPath + ".checkpoints"
}

func (l *Ledger) writeCheckpoint(r Record) error {
	cp := Checkpoint{
		Seq:      r.Seq,
		LastHash: r.Hash,
		At:       time.Now().UTC(),
	}
	cp.Signature = SignCheckpoint(l.key, cp)
	line, err := json.Marshal(cp)
	if err != nil {
		return fmt.Errorf("encode checkpoint: %w", err)
	}
	f, err := os.OpenFile(CheckpointPath(l.path), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open checkpoint file: %w", err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		f.Close()
		return fmt.Errorf("append checkpoint: %w", err)
	}
	return f.Close()
}

func checkpointPayload(cp Checkpoint) []byte {
	return []byte(fmt.Sprintf("checkpoint|%d|%s", cp.Seq, cp.LastHash))
}

// SignCheckpoint computes the MAC stored on a checkpoint.
func SignCheckpoint(key []byte, cp Checkpoint) string {
	mac := hmac.New(sha256.New, key)
	mac.Write(checkpointPayload(cp))
	return hex.EncodeToString(mac.Sum(nil))
}

// LoadCheckpoints reads the checkpoint sidecar for a ledger.
func LoadCheckpoints(ledgerPath string) ([]Checkpoint, error) {
	f, err := os.Open(CheckpointPath(ledgerPath))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open checkpoint file: %w", err)
	}
	defer f.Close()

	var checkpoints []Checkpoint
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var cp Checkpoint
		if err := json.Unmarshal(line, &cp); err != nil {
			return nil, fmt.Errorf("decode checkpoint: %w", err)
		}
		checkpoints = append(checkpoints, cp)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read checkpoint file: %w", err)
	}
	return checkpoints, nil
}
