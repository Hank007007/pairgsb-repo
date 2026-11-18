package accesslog

import (
	"bytes"
	"strings"
	"testing"
)

func TestLogWritesOneRecordPerRequest(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf)
	log.Log("127.0.0.1", "GET", "/api/users", "curl/8.4.0", 200)
	log.Log("127.0.0.1", "POST", "/api/users", "curl/8.4.0", 201)

	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2: %q", len(lines), buf.String())
	}
	fields := strings.Split(lines[0], " ")
	want := []string{"127.0.0.1", "GET", "/api/users", "200", "curl/8.4.0"}
	if len(fields) != len(want) {
		t.Fatalf("fields = %q, want %q", fields, want)
	}
	for i := range want {
		if fields[i] != want[i] {
			t.Errorf("field %d = %q, want %q", i, fields[i], want[i])
		}
	}
}

func TestNilLoggerIsNoOp(t *testing.T) {
	var log *Logger
	log.Log("127.0.0.1", "GET", "/", "curl", 200)
}
