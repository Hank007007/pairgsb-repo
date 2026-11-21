package record_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"maskseal-journal/internal/keystore"
	"maskseal-journal/internal/record"
)

func TestEncodeDeterministicAndUTF8Verbatim(t *testing.T) {
	fields := map[string]any{}
	for i := 0; i < 40; i++ {
		fields[fmt.Sprintf("k%02d", i)] = fmt.Sprintf("value-%d", i)
	}
	fields["user"] = map[string]any{
		"name": "Zoë Ångström 🚀",
		"note": "中文文本",
		"esc":  "quote=\" slash=\\ tab=\t",
	}
	rec := record.Record{
		Seq:      1,
		Actor:    "alice@example.com",
		Action:   "login",
		Time:     time.Date(2026, 3, 4, 5, 6, 7, 8, time.UTC),
		PrevHash: record.GenesisHash,
		Fields:   fields,
		Hash:     "abc",
	}

	first, err := record.Encode(rec)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	for i := 0; i < 100; i++ {
		got, err := record.Encode(rec)
		if err != nil {
			t.Fatalf("encode %d: %v", i, err)
		}
		if !bytes.Equal(first, got) {
			t.Fatalf("encoding changed between runs:\n%s\n%s", first, got)
		}
	}

	text := string(first)
	for _, want := range []string{"Zoë Ångström 🚀", "中文文本"} {
		if !strings.Contains(text, want) {
			t.Fatalf("encoded output lost verbatim text %q: %s", want, text)
		}
	}
	if strings.Contains(text, `\u`) {
		t.Fatalf("encoded output contains unicode escapes: %s", text)
	}

	last := -1
	for i := 0; i < 40; i++ {
		key := fmt.Sprintf(`"k%02d":`, i)
		pos := strings.Index(text, key)
		if pos < 0 {
			t.Fatalf("missing key %s in %s", key, text)
		}
		if pos < last {
			t.Fatalf("keys are not emitted in ascending order at %s", key)
		}
		last = pos
	}
}

func TestHashCoversCommitmentMetadata(t *testing.T) {
	salts, err := keystore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open keystore: %v", err)
	}
	rules := mustPolicy(t, `{"rules":[{"path":"password","action":"redact"}]}`)
	fields, _, err := record.Redact(map[string]any{
		"user": map[string]any{"password": "hunter2"},
	}, rules, salts)
	if err != nil {
		t.Fatalf("redact: %v", err)
	}
	rec := record.Record{
		Seq:      1,
		Actor:    "alice",
		Action:   "login",
		Time:     time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC),
		PrevHash: record.GenesisHash,
		Fields:   fields,
	}
	base := record.Hash(rec)
	red := fields["user"].(map[string]any)["password"].(*record.Redacted)

	flipHex := func(s string) string {
		if s[0] == 'a' {
			return "b" + s[1:]
		}
		return "a" + s[1:]
	}

	variants := []record.Redacted{
		{SaltID: red.SaltID, Commitment: flipHex(red.Commitment)},
		{SaltID: flipHex(red.SaltID), Commitment: red.Commitment},
		{SaltID: red.SaltID, Commitment: red.Commitment, Marker: false},
	}
	for i, variant := range variants {
		changed := rec
		changed.Fields = map[string]any{
			"user": map[string]any{"password": &variant},
		}
		if record.Hash(changed) == base {
			t.Fatalf("hash did not change for commitment variant %d", i)
		}
	}
}

func TestEmptyPolicyRedactsEveryLeaf(t *testing.T) {
	salts, err := keystore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open keystore: %v", err)
	}
	rules := mustPolicy(t, `{"rules":[]}`)
	out, _, err := record.Redact(map[string]any{
		"plain": "visible",
		"nested": map[string]any{
			"number": int64(7),
			"list":   []any{"a", true, nil},
		},
	}, rules, salts)
	if err != nil {
		t.Fatalf("redact: %v", err)
	}
	if _, ok := out["plain"].(*record.Redacted); !ok {
		t.Fatalf("plain leaf was not redacted: %#v", out["plain"])
	}
	nested := out["nested"].(map[string]any)
	if _, ok := nested["number"].(*record.Redacted); !ok {
		t.Fatalf("nested leaf was not redacted: %#v", nested["number"])
	}
	for i, item := range nested["list"].([]any) {
		if _, ok := item.(*record.Redacted); !ok {
			t.Fatalf("array leaf %d was not redacted: %#v", i, item)
		}
	}
}
