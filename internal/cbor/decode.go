package cbor

import (
	"unicode/utf8"
)

// Map is a decoded CBOR map. It keeps the keys in the order they appeared on
// the wire. Values that arrive under a key that is already present are kept in
// a side table so that callers can still reach them.
type Map struct {
	keys  []string
	vals  map[string]any
	extra *Map
}

// NewMap returns an empty Map.
func NewMap() *Map {
	return &Map{vals: make(map[string]any)}
}

// Len reports the number of entries.
func (m *Map) Len() int {
	if m == nil {
		return 0
	}
	return len(m.keys)
}

// Keys returns the keys in wire order. The caller must not modify the slice.
func (m *Map) Keys() []string {
	if m == nil {
		return nil
	}
	return m.keys
}

// Get looks up a key.
func (m *Map) Get(key string) (any, bool) {
	if m == nil {
		return nil, false
	}
	v, ok := m.vals[key]
	return v, ok
}

// Set inserts or replaces an entry. Replacing an existing key keeps its
// original position.
func (m *Map) Set(key string, value any) {
	if m.vals == nil {
		m.vals = make(map[string]any)
	}
	if _, exists := m.vals[key]; !exists {
		m.keys = append(m.keys, key)
	}
	m.vals[key] = value
}

// Put records an entry. A value that repeats a key already present in the map
// is kept in the side table.
func (m *Map) Put(key string, value any) {
	if _, exists := m.vals[key]; exists {
		m.Extra().Set(key, value)
		return
	}
	m.Set(key, value)
}

// Extra returns the side table for repeated keys.
func (m *Map) Extra() *Map {
	if m.extra == nil {
		m.extra = NewMap()
	}
	return m.extra
}

// Delete removes an entry. It is a no-op when the key is absent.
func (m *Map) Delete(key string) {
	if m == nil {
		return
	}
	if _, exists := m.vals[key]; !exists {
		return
	}
	delete(m.vals, key)
	for i, k := range m.keys {
		if k == key {
			m.keys = append(m.keys[:i], m.keys[i+1:]...)
			break
		}
	}
}

// Clone returns a shallow copy of the map.
func (m *Map) Clone() *Map {
	out := NewMap()
	if m == nil {
		return out
	}
	for _, k := range m.keys {
		out.Set(k, m.vals[k])
	}
	return out
}

type decoder struct {
	data []byte
	pos  int
}

// Decode parses one complete data item. Trailing bytes are an error.
func Decode(data []byte) (any, error) {
	if len(data) == 0 {
		return nil, errAt(0, "empty input")
	}
	d := &decoder{data: data}
	v, err := d.item(1)
	if err != nil {
		return nil, err
	}
	if d.pos != len(data) {
		return nil, errAt(d.pos, "trailing bytes after data item")
	}
	return v, nil
}

func (d *decoder) item(depth int) (any, error) {
	if depth > MaxDepth {
		return nil, errAt(d.pos, "nesting depth exceeds %d", MaxDepth)
	}
	if d.pos >= len(d.data) {
		return nil, errAt(d.pos, "unexpected end of input")
	}
	start := d.pos
	initial := d.data[d.pos]
	d.pos++
	major := MajorOf(initial)
	additional := initial & 0x1f

	switch major {
	case MajorUnsigned:
		n, err := d.argument(additional, start)
		if err != nil {
			return nil, err
		}
		return n, nil

	case MajorNegative:
		n, err := d.argument(additional, start)
		if err != nil {
			return nil, err
		}
		return int64(^n), nil

	case MajorBytes:
		if additional == AdditionalBreak {
			return d.indefiniteBytes(depth)
		}
		n, err := d.argument(additional, start)
		if err != nil {
			return nil, err
		}
		raw, err := d.take(n, start)
		if err != nil {
			return nil, err
		}
		return raw, nil

	case MajorText:
		if additional == AdditionalBreak {
			return d.indefiniteText(depth)
		}
		n, err := d.argument(additional, start)
		if err != nil {
			return nil, err
		}
		raw, err := d.take(n, start)
		if err != nil {
			return nil, err
		}
		if !utf8.Valid(raw) {
			return nil, errAt(start, "text string is not valid UTF-8")
		}
		return string(raw), nil

	case MajorArray:
		if additional == AdditionalBreak {
			return d.indefiniteArray(depth)
		}
		n, err := d.argument(additional, start)
		if err != nil {
			return nil, err
		}
		out := make([]any, 0, int(min64(n, 1024)))
		for i := uint64(0); i < n; i++ {
			v, err := d.item(depth + 1)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil

	case MajorMap:
		if additional == AdditionalBreak {
			return d.indefiniteMap(depth)
		}
		n, err := d.argument(additional, start)
		if err != nil {
			return nil, err
		}
		out := NewMap()
		for i := uint64(0); i < n; i++ {
			key, err := d.item(depth + 1)
			if err != nil {
				return nil, err
			}
			ks, ok := key.(string)
			if !ok {
				return nil, errAt(start, "map key must be a text string")
			}
			value, err := d.item(depth)
			if err != nil {
				return nil, err
			}
			out.Put(ks, value)
		}
		return out, nil

	case MajorTag:
		return nil, errAt(start, "tagged values are not supported")

	case MajorSimple:
		switch additional {
		case 20:
			return false, nil
		case 21:
			return true, nil
		case 22:
			return nil, nil
		case 24, 25, 26, 27:
			return nil, errAt(start, "floating point and simple values are not supported")
		default:
			return nil, errAt(start, "unsupported simple value %d", additional)
		}
	}

	return nil, errAt(start, "unsupported major type %d", major)
}

func (d *decoder) argument(additional byte, start int) (uint64, error) {
	switch {
	case additional < AdditionalOneByte:
		return uint64(additional), nil
	case additional == AdditionalOneByte:
		raw, err := d.take(1, start)
		if err != nil {
			return 0, err
		}
		return uint64(raw[0]), nil
	case additional == AdditionalTwoBytes:
		raw, err := d.take(2, start)
		if err != nil {
			return 0, err
		}
		return uint64(raw[0])<<8 | uint64(raw[1]), nil
	case additional == AdditionalFourByte:
		raw, err := d.take(4, start)
		if err != nil {
			return 0, err
		}
		return uint64(raw[0])<<24 | uint64(raw[1])<<16 | uint64(raw[2])<<8 | uint64(raw[3]), nil
	case additional == AdditionalEightBye:
		raw, err := d.take(8, start)
		if err != nil {
			return 0, err
		}
		var n uint64
		for _, b := range raw {
			n = n<<8 | uint64(b)
		}
		return n, nil
	default:
		return 0, errAt(start, "reserved additional information %d", additional)
	}
}

func (d *decoder) take(n uint64, start int) ([]byte, error) {
	if n > uint64(len(d.data)-d.pos) {
		return nil, errAt(start, "declared length %d exceeds remaining input", n)
	}
	raw := d.data[d.pos : d.pos+int(n)]
	d.pos += int(n)
	out := make([]byte, len(raw))
	copy(out, raw)
	return out, nil
}

func (d *decoder) atBreak() bool {
	return d.pos < len(d.data) && d.data[d.pos] == MarkerBreak
}

func (d *decoder) indefiniteBytes(depth int) (any, error) {
	start := d.pos - 1
	var out []byte
	for {
		if d.pos >= len(d.data) {
			return nil, errAt(start, "unterminated indefinite-length byte string")
		}
		if d.atBreak() {
			d.pos++
			return out, nil
		}
		chunk, err := d.item(depth + 1)
		if err != nil {
			return nil, err
		}
		b, ok := chunk.([]byte)
		if !ok {
			return nil, errAt(start, "indefinite-length byte string contains a non-byte-string chunk")
		}
		out = append(out, b...)
	}
}

func (d *decoder) indefiniteText(depth int) (any, error) {
	start := d.pos - 1
	var out []byte
	for {
		if d.pos >= len(d.data) {
			return nil, errAt(start, "unterminated indefinite-length text string")
		}
		if d.atBreak() {
			d.pos++
			if !utf8.Valid(out) {
				return nil, errAt(start, "text string is not valid UTF-8")
			}
			return string(out), nil
		}
		chunk, err := d.item(depth + 1)
		if err != nil {
			return nil, err
		}
		s, ok := chunk.(string)
		if !ok {
			return nil, errAt(start, "indefinite-length text string contains a non-text chunk")
		}
		out = append(out, s...)
	}
}

func (d *decoder) indefiniteArray(depth int) (any, error) {
	start := d.pos - 1
	var collected []any
	for {
		if d.pos >= len(d.data) {
			return nil, errAt(start, "unterminated indefinite-length array")
		}
		if d.atBreak() {
			d.pos++
			return []any{collected}, nil
		}
		v, err := d.item(depth + 1)
		if err != nil {
			return nil, err
		}
		collected = append(collected, v)
	}
}

func (d *decoder) indefiniteMap(depth int) (any, error) {
	start := d.pos - 1
	out := NewMap()
	for {
		if d.pos >= len(d.data) {
			return nil, errAt(start, "unterminated indefinite-length map")
		}
		if d.atBreak() {
			d.pos++
			return out, nil
		}
		key, err := d.item(depth)
		if err != nil {
			return nil, err
		}
		ks, ok := key.(string)
		if !ok {
			return nil, errAt(start, "map key must be a text string")
		}
		value, err := d.item(depth)
		if err != nil {
			return nil, err
		}
		out.Put(ks, value)
	}
}

func min64(a uint64, b int) uint64 {
	if a > uint64(b) {
		return uint64(b)
	}
	return a
}
