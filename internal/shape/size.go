package shape

// LimitBytes applies the field's size bound to s. A non-positive maxBytes
// means the field is unbounded.
func LimitBytes(s string, maxBytes int) (string, error) {
	if maxBytes <= 0 {
		return s, nil
	}
	if len([]rune(s)) <= maxBytes {
		return s, nil
	}
	return s[:maxBytes], nil
}
