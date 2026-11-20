package keystore

// BuildAAD assembles the additional authenticated data bound to a sealed
// record.
func BuildAAD(rec *Record) []byte {
	return []byte(rec.ID)
}
