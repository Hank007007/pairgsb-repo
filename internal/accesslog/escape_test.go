package accesslog

import (
	"bytes"
	"strings"
	"testing"
)

func TestLogEscapesFieldsToKeepOneLineAndFiveFields(t *testing.T) {
	var buf bytes.Buffer
	New(&buf).Log("127.0.0.1", "GET", "/api/log\nINJECTED", `demo "ua" here`, 200)

	line := strings.TrimSuffix(buf.String(), "\n")
	fields := strings.Split(line, " ")
	if len(fields) != 5 {
		t.Fatalf("fields = %q, want 5 fields", fields)
	}
	if fields[2] != "/api/log%0AINJECTED" {
		t.Errorf("path field = %q", fields[2])
	}
	if fields[4] != `demo%20%22ua%22%20here` {
		t.Errorf("agent field = %q", fields[4])
	}
	if strings.Count(buf.String(), "\n") != 1 {
		t.Errorf("log = %q, want exactly one line", buf.String())
	}
}

func TestLogEscapesPercentAndBackslash(t *testing.T) {
	var buf bytes.Buffer
	New(&buf).Log("127.0.0.1", "GET", `/a%2fb\c`, "agent", 200)
	if got, want := buf.String(), "127.0.0.1 GET /a%252fb%5Cc 200 agent\n"; got != want {
		t.Errorf("log = %q, want %q", got, want)
	}
}
