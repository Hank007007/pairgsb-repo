package smtp

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden snapshot")

func sampleASCII() Message {
	return Message{
		From:    "Notifications <notify@example.com>",
		To:      "alice@example.com",
		Subject: "Your order has shipped",
		Text:    "Hello Alice,\r\n\r\nYour order #1042 is on its way.\r\n",
	}
}

func TestComposePlainASCIIMatchesSnapshot(t *testing.T) {
	got, err := Compose(sampleASCII())
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}

	goldenPath := filepath.Join("testdata", "plain_ascii.eml")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("Compose output differs from golden snapshot\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestComposeStartsWithFrom(t *testing.T) {
	got, err := Compose(sampleASCII())
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if !strings.HasPrefix(string(got), "From: Notifications <notify@example.com>\r\n") {
		t.Fatalf("message does not start with From: %q", got[:80])
	}
}

func TestComposeHeaderBodySeparator(t *testing.T) {
	got, err := Compose(sampleASCII())
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if !strings.Contains(string(got), "\r\n\r\n") {
		t.Fatal("message has no blank line between header block and body")
	}
}

func TestComposeSubjectPresent(t *testing.T) {
	got, err := Compose(sampleASCII())
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if !strings.Contains(string(got), "Subject: Your order has shipped\r\n") {
		t.Fatalf("message missing subject: %s", got)
	}
}

func TestComposeRejectsMissingFrom(t *testing.T) {
	msg := sampleASCII()
	msg.From = "  "
	if _, err := Compose(msg); err == nil {
		t.Fatal("Compose accepted a message without From")
	}
}

func TestComposeTwoRecipients(t *testing.T) {
	msg := sampleASCII()
	msg.To = "alice@example.com, bob@example.com"
	got, err := Compose(msg)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	if !strings.Contains(string(got), "To: alice@example.com, bob@example.com\r\n") {
		t.Fatalf("message has unexpected To header: %s", got)
	}
}

func TestComposeMultipartWithAttachment(t *testing.T) {
	msg := sampleASCII()
	msg.Attachments = []Attachment{{Filename: "report.pdf", Data: []byte("%PDF-1.4 fake")}}
	got, err := Compose(msg)
	if err != nil {
		t.Fatalf("Compose: %v", err)
	}
	s := string(got)
	if !strings.Contains(s, `Content-Type: multipart/mixed; boundary="`) {
		t.Fatalf("message has no multipart Content-Type: %s", s)
	}
	if !strings.Contains(s, `Content-Disposition: attachment; filename="report.pdf"`) {
		t.Fatalf("message has no attachment disposition: %s", s)
	}
	if !strings.Contains(s, "Content-Transfer-Encoding: base64") {
		t.Fatalf("message has no base64 transfer encoding: %s", s)
	}
}
