// Package accesslog writes one line per forwarded request.
package accesslog

import (
	"fmt"
	"io"
	"strings"
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
	fmt.Fprintf(w, "%s %s %s %03d %s\n",
		escapeField(ip), escapeField(method), escapeField(path),
		status, escapeField(agent))
}

// escapeField makes a client-controlled field safe to write as one
// space-separated field: spaces, control bytes, quotes, percent signs, and
// backslashes are written as %XX using uppercase hexadecimal.
func escapeField(s string) string {
	needsEscape := false
	for i := 0; i < len(s); i++ {
		if mustEscape(s[i]) {
			needsEscape = true
			break
		}
	}
	if !needsEscape {
		return s
	}

	const hex = "0123456789ABCDEF"
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if mustEscape(c) {
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&0x0F])
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

func mustEscape(c byte) bool {
	return c == ' ' || c <= 0x1F || c == 0x7F ||
		c == '"' || c == '%' || c == '\\'
}
