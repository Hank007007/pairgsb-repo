package record

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// ErrDecode reports a line that is not a valid journal record.
var ErrDecode = fmt.Errorf("record: invalid encoding")

// Decode parses the wire representation produced by Encode.
func Decode(line []byte) (Record, error) {
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.UseNumber()
	var raw map[string]any
	if err := dec.Decode(&raw); err != nil {
		return Record{}, fmt.Errorf("%w: %v", ErrDecode, err)
	}
	var r Record
	seq, ok := raw["seq"].(json.Number)
	if !ok {
		return Record{}, fmt.Errorf("%w: missing seq", ErrDecode)
	}
	n, err := strconv.ParseInt(seq.String(), 10, 64)
	if err != nil {
		return Record{}, fmt.Errorf("%w: bad seq", ErrDecode)
	}
	r.Seq = n
	r.Actor, _ = raw["actor"].(string)
	r.Action, _ = raw["action"].(string)
	if ts, _ := raw["ts"].(string); ts != "" {
		t, err := time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			return Record{}, fmt.Errorf("%w: bad ts", ErrDecode)
		}
		r.Time = t
	}
	r.PrevHash, _ = raw["prev_hash"].(string)
	r.Hash, _ = raw["hash"].(string)
	fields, ok := raw["fields"].(map[string]any)
	if !ok {
		return Record{}, fmt.Errorf("%w: missing fields", ErrDecode)
	}
	r.Fields = normalizeMap(fields)
	return r, nil
}

func normalizeMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = normalizeValue(v)
	}
	return out
}

func normalizeValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		if red, ok := asRedacted(x); ok {
			return red
		}
		return normalizeMap(x)
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = normalizeValue(item)
		}
		return out
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return i
		}
		f, err := x.Float64()
		if err != nil {
			return x.String()
		}
		return f
	default:
		return v
	}
}

// asRedacted recognises a commitment object. The redacted marker is accepted
// in either state so a record can be decoded before the marker is acted on.
func asRedacted(m map[string]any) (*Redacted, bool) {
	if _, ok := m["redacted"]; !ok {
		return nil, false
	}
	commitment, ok := m["commitment"].(string)
	if !ok {
		return nil, false
	}
	saltID, ok := m["salt_id"].(string)
	if !ok {
		return nil, false
	}
	return &Redacted{SaltID: saltID, Commitment: commitment, Marker: m["redacted"]}, true
}
