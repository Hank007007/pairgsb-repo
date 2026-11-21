package manifest

import (
	"errors"
	"fmt"
	"math"

	"cborveil/internal/cbor"
)

// Errors reported by Parse.
var (
	ErrTooLarge     = errors.New("manifest: exceeds the 1 MiB limit")
	ErrNotCanonical = errors.New("manifest: not canonically encoded")
	ErrMalformed    = errors.New("manifest: malformed structure")
)

// Target is one downloadable slice of the upgrade package.
type Target struct {
	Model  string
	URL    string
	SHA256 []byte
	Size   int64
	Offset int64
}

// Manifest is the parsed representation of an OTA manifest.
type Manifest struct {
	Version string
	Size    int64
	Targets []Target
	Sig     []byte
	KeyID   string

	raw *cbor.Map
}

// Parse validates and decodes one manifest.
func Parse(data []byte) (*Manifest, error) {
	if len(data) > cbor.MaxManifestBytes {
		return nil, ErrTooLarge
	}
	if !cbor.IsCanonical(data) {
		return nil, ErrNotCanonical
	}
	v, err := cbor.Decode(data)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMalformed, err)
	}
	root, ok := v.(*cbor.Map)
	if !ok {
		return nil, fmt.Errorf("%w: top-level item is not a map", ErrMalformed)
	}
	root = mergeExtras(root)

	m := &Manifest{raw: root}

	version, ok := textValue(root, "version")
	if !ok || version == "" {
		return nil, fmt.Errorf("%w: missing or invalid \"version\"", ErrMalformed)
	}
	m.Version = version

	size, ok := uintValue(root, "size")
	if !ok || size <= 0 {
		return nil, fmt.Errorf("%w: missing or invalid \"size\"", ErrMalformed)
	}
	m.Size = size

	sig, ok := bytesValue(root, "sig")
	if !ok || len(sig) != 64 {
		return nil, fmt.Errorf("%w: missing or invalid \"sig\"", ErrMalformed)
	}
	m.Sig = sig

	keyID, ok := textValue(root, "key_id")
	if !ok || keyID == "" {
		return nil, fmt.Errorf("%w: missing or invalid \"key_id\"", ErrMalformed)
	}
	m.KeyID = keyID

	rawTargets, ok := root.Get("targets")
	if !ok {
		return nil, fmt.Errorf("%w: missing \"targets\"", ErrMalformed)
	}
	list, ok := rawTargets.([]any)
	if !ok {
		return nil, fmt.Errorf("%w: \"targets\" is not an array", ErrMalformed)
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("%w: \"targets\" is empty", ErrManifestInvalid)
	}

	positions := make(map[string]int, len(list))
	for i, item := range list {
		tm, ok := item.(*cbor.Map)
		if !ok {
			return nil, fmt.Errorf("%w: target %d is not a map", ErrMalformed, i)
		}
		t, err := parseTarget(tm, m.Size)
		if err != nil {
			return nil, fmt.Errorf("target %d: %w", i, err)
		}
		if _, seen := positions[t.Model]; seen {
			return nil, fmt.Errorf("%w: duplicate target model %q", ErrManifestInvalid, t.Model)
		}
		positions[t.Model] = len(m.Targets)
		m.Targets = append(m.Targets, t)
	}

	return m, nil
}

// parseTarget maps one CBOR map to a Target.
func parseTarget(m *cbor.Map, manifestSize int64) (Target, error) {
	m = mergeExtras(m)
	var t Target

	model, ok := textValue(m, "model")
	if !ok || model == "" {
		return t, fmt.Errorf("%w: missing or invalid \"model\"", ErrMalformed)
	}
	t.Model = model

	url, ok := textValue(m, "url")
	if !ok || url == "" {
		return t, fmt.Errorf("%w: missing or invalid \"url\"", ErrMalformed)
	}
	t.URL = url

	sum, ok := bytesValue(m, "sha256")
	if !ok || len(sum) != 32 {
		return t, fmt.Errorf("%w: missing or invalid \"sha256\"", ErrMalformed)
	}
	t.SHA256 = sum

	size, ok := uintValue(m, "size")
	if !ok || size <= 0 {
		return t, fmt.Errorf("%w: missing or invalid \"size\"", ErrMalformed)
	}
	t.Size = size

	offset, ok := uintValue(m, "offset")
	if !ok {
		return t, fmt.Errorf("%w: missing or invalid \"offset\"", ErrMalformed)
	}
	t.Offset = offset

	if size > manifestSize || offset > manifestSize-size {
		return t, fmt.Errorf("%w: target range exceeds manifest size", ErrManifestInvalid)
	}

	return t, nil
}

// mergeExtras folds the repeated-key side table back into the visible map.
func mergeExtras(m *cbor.Map) *cbor.Map {
	if m == nil {
		return cbor.NewMap()
	}
	out := m.Clone()
	extra := m.Extra()
	for _, k := range extra.Keys() {
		v, _ := extra.Get(k)
		out.Set(k, v)
	}
	return out
}

func textValue(m *cbor.Map, key string) (string, bool) {
	v, ok := m.Get(key)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func bytesValue(m *cbor.Map, key string) ([]byte, bool) {
	v, ok := m.Get(key)
	if !ok {
		return nil, false
	}
	b, ok := v.([]byte)
	return b, ok
}

func uintValue(m *cbor.Map, key string) (int64, bool) {
	v, ok := m.Get(key)
	if !ok {
		return 0, false
	}
	n, ok := v.(uint64)
	if !ok || n > math.MaxInt64 {
		return 0, false
	}
	return int64(n), true
}
