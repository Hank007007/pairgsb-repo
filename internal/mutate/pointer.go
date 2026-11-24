package mutate

// EscapeSegment escapes one RFC 6901 reference token. A token that contains
// `~` or `/` is rewritten so it can be joined into a JSON Pointer.
func EscapeSegment(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '~':
			out = append(out, '~', '0')
		case '/':
			out = append(out, '~', '1')
		default:
			out = append(out, s[i])
		}
	}
	return string(out)
}
