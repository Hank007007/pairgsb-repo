package patch

import "reflect"

// equalJSON reports whether two decoded JSON values are equal.
func equalJSON(a, b any) bool {
	return reflect.DeepEqual(a, b)
}

// equalError builds the error returned by a failed test operation.
func equalError(path string) error {
	return &OperationError{Code: CodeTestFailed, Message: "test failed at " + path}
}
