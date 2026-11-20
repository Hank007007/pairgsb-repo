package shape

import (
	"slices"
	"strconv"
	"strings"
)

// CoerceInt converts a submitted integer literal to the target integer type.
func CoerceInt(raw string) (int32, error) {
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, ErrInt
	}
	return int32(n), nil
}

// CoerceBool converts a submitted literal to a boolean.
func CoerceBool(raw string) (bool, error) {
	switch strings.ToLower(raw) {
	case "1", "true", "t", "yes", "y", "on":
		return true, nil
	case "0", "false", "f", "no", "n", "off":
		return false, nil
	default:
		return false, ErrBool
	}
}

// CoerceString applies the field's byte limit, character class and enum to a
// submitted string.
func CoerceString(raw string, field Field) (string, error) {
	limited, err := LimitBytes(raw, field.MaxBytes)
	if err != nil {
		return limited, err
	}
	normalized, err := CheckClass(limited, field.Class)
	if err != nil {
		return normalized, err
	}
	if len(field.Enum) > 0 && !slices.Contains(field.Enum, normalized) {
		return normalized, ErrEnum
	}
	return normalized, nil
}
