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

// encodeValue writes a field value. Empty values and values containing a
// space are wrapped in double quotes; everything else is written bare.
func encodeValue(value string) []byte {
	if len(value) == 0 {
		return []byte{'"', '"'}
	}

	needsQuote := false
	for i := 0; i < len(value); i++ {
		if value[i] == ' ' {
			needsQuote = true
			break
		}
	}

	if !needsQuote {
		return []byte(value)
	}

	out := make([]byte, 0, len(value)+2)
	out = append(out, '"')
	out = append(out, value...)
	out = append(out, '"')
	return out
}
