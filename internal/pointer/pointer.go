// Package pointer parses and evaluates JSON Pointers (RFC 6901).
package pointer

import (
	"strconv"
	"strings"
)

// Error codes reported while parsing or evaluating a pointer.
const (
	CodeInvalidPointer   = "invalid-pointer"
	CodePathNotFound     = "path-not-found"
	CodePathNotContainer = "path-not-container"
	CodeInvalidIndex     = "invalid-index"
	CodeIndexOutOfRange  = "index-out-of-range"
)

// Error is a pointer failure carrying a stable machine-readable code.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	if e.Message == "" {
		return e.Code
	}
	return e.Code + ": " + e.Message
}

// Parse turns a JSON Pointer into its reference tokens. The empty pointer
// addresses the whole document and yields no tokens.
func Parse(ptr string) ([]string, error) {
	if ptr == "" {
		return nil, nil
	}
	if !strings.HasPrefix(ptr, "/") {
		return nil, &Error{Code: CodeInvalidPointer, Message: "pointer must be empty or start with /"}
	}

	segments := strings.Split(ptr, "/")
	tokens := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment == "" {
			continue
		}
		tokens = append(tokens, segment)
	}
	return tokens, nil
}

// Resolve walks doc following tokens and returns the addressed value.
func Resolve(doc any, tokens []string) (any, error) {
	current := doc
	for _, token := range tokens {
		switch node := current.(type) {
		case map[string]any:
			next, ok := node[token]
			if !ok {
				return nil, &Error{Code: CodePathNotFound, Message: "no member named " + token}
			}
			current = next
		case []any:
			index, err := strconv.Atoi(token)
			if err != nil {
				return nil, &Error{Code: CodeInvalidIndex, Message: "array index " + token}
			}
			if index < 0 {
				return nil, nil
			}
			if index >= len(node) {
				return nil, &Error{Code: CodeIndexOutOfRange, Message: token}
			}
			current = node[index]
		default:
			return nil, nil
		}
	}
	return current, nil
}

// ResolveParent returns the container holding the final reference token
// together with that token.
func ResolveParent(doc any, tokens []string) (any, string, error) {
	if len(tokens) == 0 {
		return nil, "", &Error{Code: CodeInvalidPointer, Message: "whole document has no parent"}
	}
	parent, err := Resolve(doc, tokens[:len(tokens)-1])
	if err != nil {
		return nil, "", err
	}
	return parent, tokens[len(tokens)-1], nil
}

// CodeOf reports the machine-readable code of err, if it carries one.
func CodeOf(err error) string {
	if e, ok := err.(*Error); ok {
		return e.Code
	}
	return ""
}
