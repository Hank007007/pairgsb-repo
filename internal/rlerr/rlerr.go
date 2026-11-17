// Package rlerr defines the diagnostic error type shared by the compiler
// stages. Every error carries the stage that produced it, the byte offset in
// the source rule where the problem was detected, and a short message.
package rlerr

import "fmt"

// Stage names the compilation phase that reported an error.
type Stage string

const (
	StageLex    Stage = "lex"
	StageParse  Stage = "parse"
	StageLimit  Stage = "limit"
	StageEval   Stage = "eval"
	StageConfig Stage = "config"
)

// Error is the diagnostic returned for invalid rules and failed evaluations.
type Error struct {
	Stage Stage
	Pos   int
	Msg   string
}

// New builds a diagnostic error.
func New(stage Stage, pos int, msg string) *Error {
	return &Error{Stage: stage, Pos: pos, Msg: msg}
}

// Error implements the error interface.
func (e *Error) Error() string {
	return fmt.Sprintf("%s: offset %d: %s", e.Stage, e.Pos, e.Msg)
}
