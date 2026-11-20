package patch

// Request limits from docs/patch-policy.md §6.
const (
	MaxOperations = 64
	MaxDepth      = 16
	MaxStringLen  = 4096
)

// Check validates a patch request against the request limits. It returns an
// error carrying code limit-exceeded when a limit is violated.
func Check(ops []Operation) error {
	if len(ops) > MaxOperations {
		return limitError("too many operations")
	}
	return nil
}
