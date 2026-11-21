// Package record defines audit records, their canonical wire form, and the
// commitment values used to replace redacted fields.
package record

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Redacted is the value stored in place of a field that was masked before
// reaching the journal. The original value is recoverable only with the salt
// named by SaltID.
type Redacted struct {
	SaltID     string
	Commitment string
	// Marker is the value of the "redacted" member carried by the record.
	// Commitments produced by this package leave it nil, which encodes as
	// true; a decoded record keeps the value it was read with.
	Marker any
}

// Record is one audit entry.
type Record struct {
	Seq      int64
	Actor    string
	Action   string
	Time     time.Time
	Fields   map[string]any
	PrevHash string
	Hash     string
}

// ErrEncode reports a value that cannot be represented in the journal
// encoding.
var ErrEncode = errors.New("record: unsupported field value")

// Encode returns the wire representation of r. The result contains no
// trailing newline.
func Encode(r Record) ([]byte, error) {
	var b strings.Builder
	b.WriteString(`{"seq":`)
	b.WriteString(strconv.FormatInt(r.Seq, 10))
	b.WriteString(`,"actor":`)
	writeJSONString(&b, r.Actor)
	b.WriteString(`,"action":`)
	writeJSONString(&b, r.Action)
	b.WriteString(`,"ts":`)
	writeJSONString(&b, FormatTime(r.Time))
	b.WriteString(`,"prev_hash":`)
	writeJSONString(&b, r.PrevHash)
	b.WriteString(`,"fields":`)
	if err := writeValue(&b, r.Fields); err != nil {
		return nil, err
	}
	b.WriteString(`,"hash":`)
	writeJSONString(&b, r.Hash)
	b.WriteString("}")
	return []byte(b.String()), nil
}

func writeValue(b *strings.Builder, v any) error {
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case string:
		writeJSONString(b, x)
	case bool:
		if x {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case int:
		b.WriteString(strconv.FormatInt(int64(x), 10))
	case int32:
		b.WriteString(strconv.FormatInt(int64(x), 10))
	case int64:
		b.WriteString(strconv.FormatInt(x, 10))
	case float64:
		b.WriteString(formatFloat(x))
	case *Redacted:
		if x == nil {
			b.WriteString("null")
			return nil
		}
		b.WriteString(`{"commitment":`)
		writeJSONString(b, x.Commitment)
		b.WriteString(`,"redacted":`)
		if x.Marker == nil {
			b.WriteString("true")
		} else if err := writeValue(b, x.Marker); err != nil {
			return err
		}
		b.WriteString(`,"salt_id":`)
		writeJSONString(b, x.SaltID)
		b.WriteString("}")
	case Redacted:
		cp := x
		return writeValue(b, &cp)
	case map[string]any:
		b.WriteByte('{')
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			writeJSONString(b, k)
			b.WriteByte(':')
			if err := writeValue(b, x[k]); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	case []any:
		b.WriteByte('[')
		for i, item := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := writeValue(b, item); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	default:
		return fmt.Errorf("%w: %T", ErrEncode, v)
	}
	return nil
}

// writeJSONString writes a JSON string literal using UTF-8 verbatim for every
// character that JSON does not require to be escaped.
func writeJSONString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r >= 0x20 {
				b.WriteRune(r)
				continue
			}
			fmt.Fprintf(b, `\u%04x`, r)
		}
	}
	b.WriteByte('"')
}

// FormatTime renders t in the journal timestamp format.
func FormatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}
