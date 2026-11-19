// Package audit appends structured events to a JSONL audit log.
package audit

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Event is one audit record.
type Event struct {
	Time     string `json:"time"`
	Accessor string `json:"accessor"`
	Path     string `json:"path"`
	Action   string `json:"action"`
	Decision string `json:"decision"`
	LeaseID  string `json:"lease_id,omitempty"`
	Backend  string `json:"backend,omitempty"`
	// Value is never serialized; audit records must not contain secret
	// material in any form.
	Value string `json:"-"`
}

// Logger writes events as newline-delimited JSON.
type Logger struct {
	mu      sync.Mutex
	out     io.Writer
	closer  io.Closer
	nowFunc func() time.Time
}

// NewLogger writes to w. If w is also an io.Closer it is closed by Close.
func NewLogger(w io.Writer) *Logger {
	l := &Logger{out: w, nowFunc: time.Now}
	if c, ok := w.(io.Closer); ok {
		l.closer = c
	}
	return l
}

// OpenFile creates (or appends to) the audit log at path.
func OpenFile(path string) (*Logger, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open audit log: %w", err)
	}
	return NewLogger(f), nil
}

// Record writes one event, stamping the current time when it is unset.
func (l *Logger) Record(e Event) error {
	if e.Time == "" {
		e.Time = l.nowFunc().UTC().Format(time.RFC3339Nano)
	}
	line, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal audit event: %w", err)
	}
	line = append(line, '\n')
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, err := l.out.Write(line); err != nil {
		return fmt.Errorf("write audit event: %w", err)
	}
	return nil
}

// Close flushes and closes the underlying file, if any.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closer != nil {
		return l.closer.Close()
	}
	return nil
}
