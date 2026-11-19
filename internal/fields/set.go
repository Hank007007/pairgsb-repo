// Package fields collects the named string fields that make up an audit
// event and applies the value normalization rules from the encoding
// specification.
package fields

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// MaxValueBytes is the maximum size, in bytes, of a normalized field value.
const MaxValueBytes = 1024

var (
	// ErrDuplicateField reports that a field name occurred more than once in
	// a single event.
	ErrDuplicateField = errors.New("fields: duplicate field name")

	// ErrReservedField reports that an extension field used a name that the
	// core encoder reserves for event metadata.
	ErrReservedField = errors.New("fields: reserved field name")

	// ErrInvalidKey reports that a field name is not valid in either output
	// format.
	ErrInvalidKey = errors.New("fields: invalid key")
)

// Source identifies who supplied a field.
type Source int

const (
	// SourceCore marks a field written by the core event encoder.
	SourceCore Source = iota
	// SourceExtension marks a request-derived extension field.
	SourceExtension
)

// Field is one named value in an event.
type Field struct {
	Key    string
	Value  string
	Source Source
}

// Set holds the ordered fields of a single event.
type Set struct {
	fields []Field
	index  map[string]int
}

// NewSet returns an empty field set.
func NewSet() *Set {
	return &Set{index: make(map[string]int)}
}

// Set records value under key.
func (s *Set) Set(key, value string, source Source) error {
	if !validKey(key) {
		return ErrInvalidKey
	}
	if source == SourceExtension && isReservedName(key) {
		return ErrReservedField
	}
	if _, ok := s.index[key]; ok {
		return ErrDuplicateField
	}

	value = Normalize(value)

	s.index[key] = len(s.fields)
	s.fields = append(s.fields, Field{Key: key, Value: value, Source: source})
	return nil
}

// validKey reports whether key matches [A-Za-z_][A-Za-z0-9_.-]*.
func validKey(key string) bool {
	if key == "" {
		return false
	}

	for i := 0; i < len(key); i++ {
		c := key[i]
		if i == 0 {
			if c != '_' && (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') {
				return false
			}
			continue
		}
		if c != '_' && c != '.' && c != '-' &&
			(c < '0' || c > '9') &&
			(c < 'A' || c > 'Z') &&
			(c < 'a' || c > 'z') {
			return false
		}
	}
	return true
}

// isReservedName reports whether key is one of the core field names.
func isReservedName(key string) bool {
	switch key {
	case "timestamp", "service", "event_id", "level", "tenant_id", "trace_id",
		"actor", "action", "resource", "path", "user_agent", "message":
		return true
	default:
		return false
	}
}

// Fields returns the fields in insertion order.
func (s *Set) Fields() []Field {
	out := make([]Field, len(s.fields))
	copy(out, s.fields)
	return out
}

// Len returns the number of fields in the set.
func (s *Set) Len() int {
	return len(s.fields)
}

// Normalize returns the canonical form of a field value.
func Normalize(value string) string {
	value = strings.ToValidUTF8(value, "�")
	if len(value) <= MaxValueBytes {
		return value
	}

	cut := MaxValueBytes
	for cut > 0 && !utf8.RuneStart(value[cut]) {
		cut--
	}
	return value[:cut]
}
