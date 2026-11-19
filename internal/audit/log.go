// Package audit records security-relevant events emitted by the signing
// agent.
//
// Every record is a single JSON object on its own line so that the trail can
// be shipped to the build-machine log collector without post-processing.
package audit

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Event names the kind of thing that happened. The vocabulary is fixed by
// docs/agent-policy.md §7.
type Event string

const (
	EventKeyCreated      Event = "key.created"
	EventKeyLoaded       Event = "key.loaded"
	EventKeyDestroyed    Event = "key.destroyed"
	EventResolveDenied   Event = "resolve.denied"
	EventSignIssued      Event = "sign.issued"
	EventSignDenied      Event = "sign.denied"
	EventSessionUnlocked Event = "session.unlocked"
	EventSessionLocked   Event = "session.locked"
	EventSessionExpired  Event = "session.expired"
	EventExportDenied    Event = "export.denied"
)

// Entry is one audit record.
type Entry struct {
	Time      time.Time `json:"time"`
	Event     Event     `json:"event"`
	KeyHandle string    `json:"keyHandle,omitempty"`
	Purpose   string    `json:"purpose,omitempty"`
	Detail    string    `json:"detail,omitempty"`
}

// Log appends audit entries to an io.Writer. A Log is safe for concurrent
// use.
type Log struct {
	mu     sync.Mutex
	enc    *json.Encoder
	closer io.Closer
}

// New returns a Log that writes to w. The caller keeps ownership of w.
func New(w io.Writer) *Log {
	return &Log{
		enc: json.NewEncoder(w),
	}
}

// Open returns a Log that appends to the file at path, creating it with mode
// 0600 when it does not exist.
func Open(path string) (*Log, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, err
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	log := New(f)
	log.closer = f
	return log, nil
}

// Record appends one entry stamped with the current time.
func (l *Log) Record(ev Event, keyHandle, purpose, detail string) {
	l.RecordAt(time.Now(), ev, keyHandle, purpose, detail)
}

// RecordAt appends one entry stamped with t.
func (l *Log) RecordAt(t time.Time, ev Event, keyHandle, purpose, detail string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	_ = l.enc.Encode(Entry{
		Time:      t.UTC(),
		Event:     ev,
		KeyHandle: keyHandle,
		Purpose:   purpose,
		Detail:    detail,
	})
}

// Close releases the underlying file when the Log was created with Open.
func (l *Log) Close() error {
	if l == nil || l.closer == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	err := l.closer.Close()
	l.closer = nil
	return err
}
