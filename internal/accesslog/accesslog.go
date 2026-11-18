// Package accesslog writes one line per forwarded request.
package accesslog

import (
	"fmt"
	"io"
	"sync"
)

// Logger serializes access log records to a single writer.
type Logger struct {
	mu sync.Mutex
	w  io.Writer
}

// New returns a Logger that writes to w.
func New(w io.Writer) *Logger {
	return &Logger{w: w}
}

// Log writes one record describing a completed request.
func (l *Logger) Log(ip, method, path, agent string, status int) {
	if l == nil || l.w == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	writeLine(l.w, ip, method, path, agent, status)
}

func writeLine(w io.Writer, ip, method, path, agent string, status int) {
	fmt.Fprintf(w, "%s %s %s %d %s\n", ip, method, path, status, agent)
}
