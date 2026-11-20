package header

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestNeedsEncodingASCII(t *testing.T) {
	if NeedsEncoding("Weekly digest: 3 new articles") {
		t.Fatal("plain ASCII value reported as needing encoding")
	}
}

func TestNeedsEncodingNonASCII(t *testing.T) {
	if !NeedsEncoding("每周摘要") {
		t.Fatal("CJK value reported as not needing encoding")
	}
}

func TestEncodeValueLeavesASCIIAlone(t *testing.T) {
	in := "Your order has shipped"
	if got := EncodeValue(in); got != in {
		t.Fatalf("EncodeValue(%q) = %q, want unchanged", in, got)
	}
}

func TestEncodeValueEncodesNonASCII(t *testing.T) {
	in := "每周摘要"
	got := EncodeValue(in)
	if !strings.HasPrefix(got, encodedWordPrefix) || !strings.HasSuffix(got, encodedWordSuffix) {
		t.Fatalf("EncodeValue(%q) = %q, want an encoded-word", in, got)
	}
	payload := strings.TrimSuffix(strings.TrimPrefix(got, encodedWordPrefix), encodedWordSuffix)
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		t.Fatalf("encoded-word payload is not valid base64: %v", err)
	}
	if string(raw) != in {
		t.Fatalf("decoded payload = %q, want %q", raw, in)
	}
}

func TestFoldKeepsShortLines(t *testing.T) {
	line := "Subject: hello"
	if got := Fold(line); got != line {
		t.Fatalf("Fold(%q) = %q, want unchanged", line, got)
	}
}

func TestFoldHeaderCombinesNameAndValue(t *testing.T) {
	got := FoldHeader("Subject", "hello")
	if got != "Subject: hello" {
		t.Fatalf("FoldHeader = %q", got)
	}
}
