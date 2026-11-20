// Package pointer parses and evaluates JSON Pointers (RFC 6901).
package pointer

import (
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

	segments := strings.Split(ptr[1:], "/")
	tokens := make([]string, 0, len(segments))
	for _, segment := range segments {
		token, err := decodeToken(segment)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, token)
	}
	return tokens, nil
}

// decodeToken decodes the ~0 and ~1 escapes of a single reference token. A ~
// that is not followed by 0 or 1 is a malformed escape.
func decodeToken(segment string) (string, error) {
	if !strings.Contains(segment, "~") {
		return segment, nil
	}
	var decoded strings.Builder
	decoded.Grow(len(segment))
	for i := 0; i < len(segment); i++ {
		if segment[i] != '~' {
			decoded.WriteByte(segment[i])
			continue
		}
		if i+1 >= len(segment) {
			return "", &Error{Code: CodeInvalidPointer, Message: "trailing ~ in token " + segment}
		}
		switch segment[i+1] {
		case '0':
			decoded.WriteByte('~')
		case '1':
			decoded.WriteByte('/')
		default:
			return "", &Error{Code: CodeInvalidPointer, Message: "malformed escape ~" + segment[i+1:i+2] + " in token " + segment}
		}
		i++
	}
	return decoded.String(), nil
}

// maxIndex caps parsed array indices so absurdly long digit strings cannot
// overflow; any real array is far smaller, so the index is simply out of range.
const maxIndex = 1 << 30

// ParseIndex validates token as an array index: decimal digits only, no sign,
// no leading zeros. The token "-" is not an index. Violations yield
// invalid-index.
func ParseIndex(token string) (int, error) {
	if token == "" {
		return 0, &Error{Code: CodeInvalidIndex, Message: "empty array index"}
	}
	if len(token) > 1 && token[0] == '0' {
		return 0, &Error{Code: CodeInvalidIndex, Message: "leading zero in array index " + token}
	}
	value := 0
	for i := 0; i < len(token); i++ {
		digit := token[i]
		if digit < '0' || digit > '9' {
			return 0, &Error{Code: CodeInvalidIndex, Message: "array index " + token}
		}
		if value <= maxIndex/10 {
			value = value*10 + int(digit-'0')
		} else {
			value = maxIndex + 1
		}
	}
	return value, nil
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
			index, err := ParseIndex(token)
			if err != nil {
				return nil, err
			}
			if index >= len(node) {
				return nil, &Error{Code: CodeIndexOutOfRange, Message: token}
			}
			current = node[index]
		default:
			return nil, &Error{Code: CodePathNotContainer, Message: "cannot traverse into " + token}
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
