package record

import (
	"encoding/json"
	"fmt"
	"strings"

	"maskseal-journal/internal/keystore"
	"maskseal-journal/internal/policy"
)

// SealedValue is produced by Redact for each field a policy marks for
// redaction.
type SealedValue struct {
	Path       string
	Commitment *Redacted
	// Ciphertext is the sealed original value. It is written to the salt
	// store entry so the value can be recovered with the salt.
	Ciphertext []byte
}

// Redact walks fields, replaces every field the policy marks for redaction
// with a commitment, and returns the sealed copies of the replaced values.
func Redact(fields map[string]any, rules *policy.RuleSet, salts *keystore.Store) (map[string]any, []SealedValue, error) {
	cloned := CloneFields(fields)
	leaves := Flatten(cloned)
	var sealed []SealedValue
	for _, leaf := range leaves {
		if _, already := leaf.Value.(*Redacted); already {
			continue
		}
		action, _ := rules.Match(leaf.Path)
		if action != policy.ActionRedact {
			continue
		}
		id, err := keystore.NewSaltID()
		if err != nil {
			return nil, nil, err
		}
		salt, err := NewSalt()
		if err != nil {
			return nil, nil, err
		}
		commitment, err := Commit(salt, leaf.Path, leaf.Value)
		if err != nil {
			return nil, nil, err
		}
		rendered, err := CanonicalValue(leaf.Value)
		if err != nil {
			return nil, nil, err
		}
		ciphertext, err := Seal(salt, string(id), leaf.Path, []byte(rendered))
		if err != nil {
			return nil, nil, err
		}
		if err := salts.Put(id, keystore.Entry{Salt: salt, Ciphertext: ciphertext}); err != nil {
			return nil, nil, err
		}
		if err := setPath(cloned, leaf.Path, &Redacted{SaltID: string(id), Commitment: commitment}); err != nil {
			return nil, nil, err
		}
		sealed = append(sealed, SealedValue{
			Path:       leaf.Path,
			Commitment: &Redacted{SaltID: string(id), Commitment: commitment},
			Ciphertext: ciphertext,
		})
	}
	return cloned, sealed, nil
}

// setPath replaces the leaf at the canonical path with value.
func setPath(fields map[string]any, path string, value any) error {
	segments := SplitPath(path)
	if len(segments) == 0 {
		return fmt.Errorf("record: empty field path")
	}
	var cur any = fields
	for i, seg := range segments {
		last := i == len(segments)-1
		if strings.HasPrefix(seg, "[") {
			idxText := strings.TrimSuffix(strings.TrimPrefix(seg, "["), "]")
			var idx int
			if _, err := fmt.Sscanf(idxText, "%d", &idx); err != nil {
				return fmt.Errorf("record: bad index %q", seg)
			}
			arr, ok := cur.([]any)
			if !ok || idx < 0 || idx >= len(arr) {
				return fmt.Errorf("record: path %q does not resolve", path)
			}
			if last {
				arr[idx] = value
				return nil
			}
			cur = arr[idx]
			continue
		}
		m, ok := cur.(map[string]any)
		if !ok {
			return fmt.Errorf("record: path %q does not resolve", path)
		}
		if last {
			m[seg] = value
			return nil
		}
		cur = m[seg]
	}
	return fmt.Errorf("record: path %q does not resolve", path)
}

// RenderPlain converts the canonical bytes of a sealed leaf back to the text
// handed to an operator. String leaves lose their JSON quotes; other leaves
// are shown in their canonical encoding.
func RenderPlain(canonical []byte) string {
	var v any
	if err := json.Unmarshal(canonical, &v); err == nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return string(canonical)
}

// Lookup returns the value stored at the canonical path.
func Lookup(fields map[string]any, path string) (any, bool) {
	segments := SplitPath(path)
	var cur any = fields
	for _, seg := range segments {
		if strings.HasPrefix(seg, "[") {
			idxText := strings.TrimSuffix(strings.TrimPrefix(seg, "["), "]")
			var idx int
			if _, err := fmt.Sscanf(idxText, "%d", &idx); err != nil {
				return nil, false
			}
			arr, ok := cur.([]any)
			if !ok || idx < 0 || idx >= len(arr) {
				return nil, false
			}
			cur = arr[idx]
			continue
		}
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[seg]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}
