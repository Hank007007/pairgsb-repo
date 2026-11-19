package runner

import "errors"

// Errors returned by Build. Callers should compare with errors.Is.
var (
	// ErrEmptyExecutable reports that the step's executable rendered to an
	// empty string.
	ErrEmptyExecutable = errors.New("runner: executable is empty")

	// ErrUnknownPlaceholder reports a ${INPUT_name} reference that has no
	// matching entry in the plan's inputs.
	ErrUnknownPlaceholder = errors.New("runner: unknown placeholder")

	// ErrNulInArg reports an argv element containing a NUL byte.
	ErrNulInArg = errors.New("runner: NUL byte in argument")

	// ErrNewlineInArg reports an argv element containing a raw newline.
	ErrNewlineInArg = errors.New("runner: newline in argument")
)
