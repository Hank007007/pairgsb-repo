package mime

import (
	"strings"
	"testing"
)

func TestContentDispositionPlain(t *testing.T) {
	got := ContentDisposition("report.pdf")
	want := `attachment; filename="report.pdf"`
	if got != want {
		t.Fatalf("ContentDisposition = %q, want %q", got, want)
	}
}

func TestContentDispositionIsAttachment(t *testing.T) {
	got := ContentDisposition("notes.txt")
	if !strings.HasPrefix(got, "attachment;") {
		t.Fatalf("ContentDisposition = %q", got)
	}
}

func TestNewBoundaryIsUnique(t *testing.T) {
	a := NewBoundary()
	b := NewBoundary()
	if a == b {
		t.Fatalf("NewBoundary returned the same value twice: %q", a)
	}
	if !strings.HasPrefix(a, "=_headlace_") {
		t.Fatalf("NewBoundary = %q", a)
	}
}

func TestRenderBodyPlainPart(t *testing.T) {
	got := string(RenderBody("BOUND", []Part{{
		ContentType: "text/plain; charset=utf-8",
		Body:        []byte("hello"),
	}}))
	for _, want := range []string{
		"--BOUND\r\n",
		"Content-Type: text/plain; charset=utf-8\r\n",
		"Content-Transfer-Encoding: 7bit\r\n",
		"hello\r\n",
		"--BOUND--\r\n",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("RenderBody output missing %q:\n%s", want, got)
		}
	}
}

func TestRenderBodyBinaryPartIsBase64(t *testing.T) {
	got := string(RenderBody("BOUND", []Part{{
		ContentType: "application/octet-stream",
		Disposition: ContentDisposition("a.bin"),
		Body:        []byte("abc"),
		Binary:      true,
	}}))
	if !strings.Contains(got, "Content-Transfer-Encoding: base64\r\n") {
		t.Fatalf("binary part missing base64 transfer encoding:\n%s", got)
	}
	if !strings.Contains(got, "YWJj") {
		t.Fatalf("binary part missing base64 payload:\n%s", got)
	}
}

func TestContentTypeFor(t *testing.T) {
	if got := ContentTypeFor("text/html"); got != "text/html; charset=utf-8" {
		t.Fatalf("ContentTypeFor = %q", got)
	}
}
