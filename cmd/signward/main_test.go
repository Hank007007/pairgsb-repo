package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func TestRunSignsAPayload(t *testing.T) {
	cfg := config{
		keyDir:    t.TempDir(),
		auditPath: t.TempDir() + "/audit.log",
	}

	var out bytes.Buffer
	create := `{"op":"create","id":"ci","policy":{"purposes":["sign"],"maxSignaturesPerHour":5,"maxPayloadBytes":64}}` + "\n"
	script := create + `{"op":"unlock"}` + "\n"

	// A short first pass creates the key so the second pass can sign with it.
	if err := run(cfg, strings.NewReader(script), &out); err != nil {
		t.Fatalf("run: %v", err)
	}

	var created reply
	first := strings.SplitN(out.String(), "\n", 2)[0]
	if err := json.Unmarshal([]byte(first), &created); err != nil {
		t.Fatalf("decode create reply: %v", err)
	}
	if !created.OK || created.KeyHandle == "" {
		t.Fatalf("create reply = %+v", created)
	}

	out.Reset()
	sign := `{"op":"unlock"}` + "\n" +
		`{"op":"sign","keyHandle":"` + created.KeyHandle + `","purpose":"sign","payload":"` +
		base64.StdEncoding.EncodeToString([]byte("payload")) + `"}` + "\n"
	if err := run(cfg, strings.NewReader(sign), &out); err != nil {
		t.Fatalf("run: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d reply lines, want 2", len(lines))
	}
	var signed reply
	if err := json.Unmarshal([]byte(lines[1]), &signed); err != nil {
		t.Fatalf("decode sign reply: %v", err)
	}
	if !signed.OK {
		t.Fatalf("sign reply = %+v", signed)
	}
	sig, err := base64.StdEncoding.DecodeString(signed.Signature)
	if err != nil {
		t.Fatalf("signature is not base64: %v", err)
	}
	if len(sig) == 0 {
		t.Fatal("sign reply carried an empty signature")
	}
}

func TestRunRejectsUnknownOp(t *testing.T) {
	cfg := config{keyDir: t.TempDir(), auditPath: t.TempDir() + "/audit.log"}

	var out bytes.Buffer
	if err := run(cfg, strings.NewReader(`{"op":"rotate"}`+"\n"), &out); err != nil {
		t.Fatalf("run: %v", err)
	}

	var got reply
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode reply: %v", err)
	}
	if got.OK || got.Error == "" {
		t.Fatalf("reply to unknown op = %+v", got)
	}
}
