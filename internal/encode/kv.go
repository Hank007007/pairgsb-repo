package encode

import (
	"sort"

	"logline-audit-encode/internal/fields"
)

// encodeKV renders set as space-separated name=value pairs.
func encodeKV(set *fields.Set) []byte {
	list := orderedFields(set)

	var buf []byte
	for i, f := range list {
		if i > 0 {
			buf = append(buf, ' ')
		}
		buf = append(buf, encodeKey(f.Key)...)
		buf = append(buf, '=')
		buf = append(buf, encodeValue(f.Value)...)
	}
	return buf
}

// orderedFields returns the core fields in emission order followed by the
// extension fields sorted by name.
func orderedFields(set *fields.Set) []fields.Field {
	all := set.Fields()
	core := make([]fields.Field, 0, len(reservedNames))
	ext := make([]fields.Field, 0, len(all))

	for _, f := range all {
		if isReserved(f.Key) {
			core = append(core, f)
			continue
		}
		ext = append(ext, f)
	}

	sort.Slice(core, func(i, j int) bool {
		return reservedRank(core[i].Key) < reservedRank(core[j].Key)
	})
	sort.Slice(ext, func(i, j int) bool {
		return ext[i].Key < ext[j].Key
	})

	return append(core, ext...)
}

func isReserved(name string) bool {
	for _, n := range reservedNames {
		if n == name {
			return true
		}
	}
	return false
}

func reservedRank(name string) int {
	for i, n := range reservedNames {
		if n == name {
			return i
		}
	}
	return len(reservedNames)
}

// encodeKey writes a field name as-is.
func encodeKey(key string) []byte {
	return []byte(key)
}

// encodeValue writes a field value using the bare/quoted rules from the
// encoding specification.
func encodeValue(value string) []byte {
	if len(value) == 0 {
		return []byte{'"', '"'}
	}

	if !needsQuote(value) {
		return []byte(value)
	}

	out := make([]byte, 0, len(value)+2)
	out = append(out, '"')
	for i := 0; i < len(value); {
		c := value[i]
		switch {
		case c == '\\':
			out = append(out, '\\', '\\')
			i++
		case c == '"':
			out = append(out, '\\', '"')
			i++
		case c == '\n':
			out = append(out, '\\', 'n')
			i++
		case c == '\r':
			out = append(out, '\\', 'r')
			i++
		case c == '\t':
			out = append(out, '\\', 't')
			i++
		case c < 0x20 || c == 0x7f:
			out = appendHexByte(out, c)
			i++
		case c == 0xE2 && i+2 < len(value) && value[i+1] == 0x80 &&
			(value[i+2] == 0xA8 || value[i+2] == 0xA9):
			if value[i+2] == 0xA8 {
				out = append(out, `\u2028`...)
			} else {
				out = append(out, `\u2029`...)
			}
			i += 3
		default:
			out = append(out, c)
			i++
		}
	}
	out = append(out, '"')
	return out
}

func needsQuote(value string) bool {
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c == ' ' || c == '"' || c == '\\' || c == '=' ||
			c < 0x20 || c == 0x7f {
			return true
		}
		if c == 0xE2 && i+2 < len(value) && value[i+1] == 0x80 &&
			(value[i+2] == 0xA8 || value[i+2] == 0xA9) {
			return true
		}
	}
	return false
}

func appendHexByte(out []byte, b byte) []byte {
	const hex = "0123456789ABCDEF"
	return append(out, '\\', 'u', '0', '0', hex[b>>4], hex[b&0x0F])
}
