package header

import (
	"strings"
	"testing"
)

func TestRenderSingleField(t *testing.T) {
	got := Render([]Field{{Name: "Subject", Value: "hello"}})
	if got != "Subject: hello\r\n" {
		t.Fatalf("Render = %q", got)
	}
}

func TestRenderMultipleFields(t *testing.T) {
	got := Render([]Field{
		{Name: "From", Value: "a@example.com"},
		{Name: "To", Value: "b@example.com"},
	})
	want := "From: a@example.com\r\nTo: b@example.com\r\n"
	if got != want {
		t.Fatalf("Render = %q, want %q", got, want)
	}
}

func TestRenderHasNoBlankLine(t *testing.T) {
	got := Render([]Field{{Name: "Subject", Value: "hello"}})
	if strings.Contains(got, "\r\n\r\n") {
		t.Fatalf("Render must not terminate the header block: %q", got)
	}
}
