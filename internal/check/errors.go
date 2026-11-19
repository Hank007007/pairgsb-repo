// Package check evaluates schema rewrites against versioned tuple snapshots.
//
// Check is the package's only public entry point. It resolves a request to a
// snapshot view, locates the requested relation in the schema and evaluates
// its rewrite expression with a bounded descent budget.
package check

import "fmt"

// Code classifies a failed check.
type Code string

const (
	// CodeInvalidRequest means the subject or object could not be parsed.
	CodeInvalidRequest Code = "invalid_request"

	// CodeUnknownRelation means the namespace or relation is not declared
	// in the schema.
	CodeUnknownRelation Code = "unknown_relation"

	// CodeDepthExceeded means one evaluation path exceeded the descent
	// budget.
	CodeDepthExceeded Code = "depth_exceeded"

	// CodeStaleRevision means the requested revision is below the store's
	// garbage-collection floor.
	CodeStaleRevision Code = "stale_revision"

	// CodeFutureRevision means the requested revision is newer than the
	// store's current revision.
	CodeFutureRevision Code = "future_revision"
)

// Error is the error type returned by Check.
type Error struct {
	Code   Code
	Detail string
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Detail == "" {
		return string(e.Code)
	}
	return string(e.Code) + ": " + e.Detail
}

func newError(code Code, format string, args ...any) *Error {
	return &Error{Code: code, Detail: fmt.Sprintf(format, args...)}
}

// CodeOf reports the classification of err. It returns the empty string for a
// nil error or an error that did not originate in this package.
func CodeOf(err error) Code {
	if err == nil {
		return ""
	}
	if e, ok := err.(*Error); ok {
		return e.Code
	}
	return ""
}
