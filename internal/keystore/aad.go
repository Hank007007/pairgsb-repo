package keystore

import "encoding/binary"

// BuildAAD assembles the additional authenticated data bound to a sealed
// record. Fields are length-prefixed so distinct header tuples can never
// encode to the same byte string.
func BuildAAD(rec *Record) []byte {
	aad := make([]byte, 0, 8+len(rec.ID)+8+len(rec.Tenant)+8)
	aad = appendLengthPrefixed(aad, []byte(rec.ID))
	aad = appendLengthPrefixed(aad, []byte(rec.Tenant))
	var version [8]byte
	binary.BigEndian.PutUint64(version[:], uint64(rec.KeyVersion))
	return append(aad, version[:]...)
}

func appendLengthPrefixed(dst, field []byte) []byte {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(field)))
	dst = append(dst, length[:]...)
	return append(dst, field...)
}

// legacyAAD is the associated data used by records written before the header
// binding was implemented. It is only consulted when opening or rotating such
// records; new records always use BuildAAD.
func legacyAAD(rec *Record) []byte {
	return []byte(rec.ID)
}
