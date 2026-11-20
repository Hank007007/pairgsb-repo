package patch

import (
	"unicode/utf8"

	"patchsieve/internal/pointer"
)

// Request limits from docs/patch-policy.md section 6.
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
	for _, op := range ops {
		for _, raw := range operationPointers(op) {
			tokens, err := pointer.Parse(raw)
			if err != nil {
				// Malformed pointers are reported when the operation
				// is applied.
				continue
			}
			if len(tokens) > MaxDepth {
				return limitError("pointer deeper than 16 reference tokens")
			}
		}
		if op.HasValue && (op.Op == "add" || op.Op == "replace") {
			if err := checkStringLengths(op.Value); err != nil {
				return err
			}
		}
	}
	return nil
}

// operationPointers returns the raw path/from pointers carried by op.
func operationPointers(op Operation) []string {
	var raws []string
	if op.HasPath {
		raws = append(raws, op.Path)
	}
	if op.HasFrom {
		raws = append(raws, op.From)
	}
	return raws
}

// checkStringLengths enforces MaxStringLen on every string in a written
// value, recursing into objects and arrays.
func checkStringLengths(value any) error {
	switch v := value.(type) {
	case string:
		if utf8.RuneCountInString(v) > MaxStringLen {
			return limitError("string value longer than 4096 characters")
		}
	case []any:
		for _, item := range v {
			if err := checkStringLengths(item); err != nil {
				return err
			}
		}
	case map[string]any:
		for _, item := range v {
			if err := checkStringLengths(item); err != nil {
				return err
			}
		}
	}
	return nil
}
