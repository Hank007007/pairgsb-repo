package patch

// Error codes returned by the patch layer. See docs/patch-policy.md §7.
const (
	CodeInvalidPointer   = "invalid-pointer"
	CodePathNotFound     = "path-not-found"
	CodePathNotContainer = "path-not-container"
	CodeInvalidIndex     = "invalid-index"
	CodeIndexOutOfRange  = "index-out-of-range"
	CodeUnknownOp        = "unknown-op"
	CodeMissingMember    = "missing-member"
	CodeInvalidMove      = "invalid-move"
	CodeFieldReadOnly    = "field-read-only"
	CodeTestFailed       = "test-failed"
	CodeLimitExceeded    = "limit-exceeded"
)

// OperationError is a rejected operation carrying a stable code.
type OperationError struct {
	Code    string
	Message string
	Cause   error
}

func (e *OperationError) Error() string {
	if e.Message == "" {
		return e.Code
	}
	return e.Code + ": " + e.Message
}

func (e *OperationError) Unwrap() error {
	return e.Cause
}

// CodeOf reports the machine-readable code of err, if it carries one.
func CodeOf(err error) string {
	switch e := err.(type) {
	case *OperationError:
		return e.Code
	case interface{ Code() string }:
		return e.Code()
	}
	return ""
}

func limitError(message string) error {
	return &OperationError{Code: CodeLimitExceeded, Message: message}
}
