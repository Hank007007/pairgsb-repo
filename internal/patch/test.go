package patch

import (
	"encoding/json"
	"reflect"
)

// equalJSON reports whether two decoded JSON values are structurally equal.
// Objects compare by member set, arrays element by element, and numbers by
// numeric value, so 1, 1.0, and 1e0 are equal.
func equalJSON(a, b any) bool {
	if aNum, ok := numberValue(a); ok {
		bNum, ok := numberValue(b)
		return ok && aNum == bNum
	}
	switch av := a.(type) {
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for key, aItem := range av {
			bItem, ok := bv[key]
			if !ok || !equalJSON(aItem, bItem) {
				return false
			}
		}
		return true
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !equalJSON(av[i], bv[i]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(a, b)
	}
}

// numberValue converts a decoded JSON number to float64 for comparison.
func numberValue(value any) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float64:
		return v, true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

// equalError builds the error returned by a failed test operation.
func equalError(path string) error {
	return &OperationError{Code: CodeTestFailed, Message: "test failed at " + path}
}
