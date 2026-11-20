package patch

import (
	"patchsieve/internal/pointer"
	"patchsieve/internal/policy"
)

// Apply applies ops to doc on behalf of principal. The input document is
// never modified: on any failure the original document is returned together
// with the error.
func Apply(doc any, ops []Operation, principal policy.Principal) (any, error) {
	if err := Check(ops); err != nil {
		return doc, err
	}
	for _, op := range ops {
		if err := validateOperation(op, principal); err != nil {
			return doc, err
		}
	}

	working := deepCopy(doc)
	for _, op := range ops {
		next, err := applyOne(working, op)
		if err != nil {
			return doc, err
		}
		working = next
	}
	return working, nil
}

// validateOperation checks an operation before anything is applied: the op
// must be one of the six supported operations, carry its required members,
// and target only fields the principal may write.
func validateOperation(op Operation, principal policy.Principal) error {
	switch op.Op {
	case "add", "remove", "replace", "move", "copy", "test":
	default:
		return &OperationError{Code: CodeUnknownOp, Message: "unsupported op " + op.Op}
	}
	if err := checkMembers(op); err != nil {
		return err
	}
	if op.Op == "test" {
		return nil
	}
	checkWritable := func(raw string) error {
		tokens, err := pointer.Parse(raw)
		if err != nil {
			// Malformed pointers are reported as invalid-pointer when the
			// operation is applied.
			return nil
		}
		if !policy.CanWrite(principal, tokens) {
			return &OperationError{Code: CodeFieldReadOnly, Message: "not writable: " + raw}
		}
		return nil
	}
	if op.HasFrom {
		if err := checkWritable(op.From); err != nil {
			return err
		}
	}
	return checkWritable(op.Path)
}

func applyOne(doc any, op Operation) (any, error) {
	switch op.Op {
	case "add":
		return applyAdd(doc, op)
	case "remove":
		return applyRemove(doc, op)
	case "replace":
		return applyReplace(doc, op)
	case "move":
		return applyMove(doc, op)
	case "copy":
		return applyCopy(doc, op)
	case "test":
		return applyTest(doc, op)
	default:
		return doc, &OperationError{Code: CodeUnknownOp, Message: "unsupported op " + op.Op}
	}
}

// checkMembers verifies that an operation carries the members it requires.
func checkMembers(op Operation) error {
	require := func(ok bool, name string) error {
		if ok {
			return nil
		}
		return &OperationError{Code: CodeMissingMember, Message: "operation requires " + name}
	}
	switch op.Op {
	case "add":
		if err := require(op.HasPath, "path"); err != nil {
			return err
		}
		return require(op.HasValue, "value")
	case "remove":
		return require(op.HasPath, "path")
	case "replace":
		if err := require(op.HasPath, "path"); err != nil {
			return err
		}
		return require(op.HasValue, "value")
	case "move", "copy":
		if err := require(op.HasFrom, "from"); err != nil {
			return err
		}
		return require(op.HasPath, "path")
	case "test":
		if err := require(op.HasPath, "path"); err != nil {
			return err
		}
		return require(op.HasValue, "value")
	}
	return nil
}

func operationTokens(raw string) ([]string, error) {
	tokens, err := pointer.Parse(raw)
	if err != nil {
		return nil, &OperationError{Code: CodeInvalidPointer, Message: err.Error(), Cause: err}
	}
	return tokens, nil
}

func applyAdd(doc any, op Operation) (any, error) {
	tokens, err := operationTokens(op.Path)
	if err != nil {
		return doc, err
	}
	parent, last, err := pointer.ResolveParent(doc, tokens)
	if err != nil {
		return doc, convertPointerError(err)
	}
	parentTokens := tokens[:len(tokens)-1]

	switch node := parent.(type) {
	case map[string]any:
		node[last] = op.Value
	case []any:
		index := len(node)
		if last != "-" {
			parsed, err := pointer.ParseIndex(last)
			if err != nil {
				return doc, convertPointerError(err)
			}
			if parsed > len(node) {
				return doc, &OperationError{Code: CodeIndexOutOfRange, Message: last}
			}
			index = parsed
		}
		node = append(node, nil)
		copy(node[index+1:], node[index:])
		node[index] = op.Value
		return setAt(doc, parentTokens, node)
	default:
		return doc, &OperationError{Code: CodePathNotContainer, Message: op.Path}
	}
	return doc, nil
}

func applyRemove(doc any, op Operation) (any, error) {
	tokens, err := operationTokens(op.Path)
	if err != nil {
		return doc, err
	}
	parent, last, err := pointer.ResolveParent(doc, tokens)
	if err != nil {
		return doc, convertPointerError(err)
	}
	parentTokens := tokens[:len(tokens)-1]

	switch node := parent.(type) {
	case map[string]any:
		if _, ok := node[last]; !ok {
			return doc, &OperationError{Code: CodePathNotFound, Message: op.Path}
		}
		delete(node, last)
	case []any:
		index, err := pointer.ParseIndex(last)
		if err != nil {
			return doc, convertPointerError(err)
		}
		if index >= len(node) {
			return doc, &OperationError{Code: CodeIndexOutOfRange, Message: last}
		}
		node = append(node[:index], node[index+1:]...)
		return setAt(doc, parentTokens, node)
	default:
		return doc, &OperationError{Code: CodePathNotContainer, Message: op.Path}
	}
	return doc, nil
}

func applyReplace(doc any, op Operation) (any, error) {
	tokens, err := operationTokens(op.Path)
	if err != nil {
		return doc, err
	}
	parent, last, err := pointer.ResolveParent(doc, tokens)
	if err != nil {
		return doc, convertPointerError(err)
	}

	switch node := parent.(type) {
	case map[string]any:
		if _, ok := node[last]; !ok {
			return doc, &OperationError{Code: CodePathNotFound, Message: op.Path}
		}
		node[last] = op.Value
	case []any:
		index, err := pointer.ParseIndex(last)
		if err != nil {
			return doc, convertPointerError(err)
		}
		if index >= len(node) {
			return doc, &OperationError{Code: CodeIndexOutOfRange, Message: last}
		}
		node[index] = op.Value
	default:
		return doc, &OperationError{Code: CodePathNotContainer, Message: op.Path}
	}
	return doc, nil
}

func applyMove(doc any, op Operation) (any, error) {
	fromTokens, err := operationTokens(op.From)
	if err != nil {
		return doc, err
	}
	pathTokens, err := operationTokens(op.Path)
	if err != nil {
		return doc, err
	}

	if isProperPrefix(fromTokens, pathTokens) {
		return doc, &OperationError{Code: CodeInvalidMove, Message: "cannot move " + op.From + " into " + op.Path}
	}

	value, err := pointer.Resolve(doc, fromTokens)
	if err != nil {
		return doc, convertPointerError(err)
	}
	if equalTokens(fromTokens, pathTokens) {
		return doc, nil
	}

	removed, err := applyRemove(doc, Operation{Op: "remove", Path: op.From, HasPath: true})
	if err != nil {
		return doc, err
	}

	addOp := Operation{Op: "add", Path: op.Path, Value: value, HasPath: true, HasValue: true}
	return applyAdd(removed, addOp)
}

func applyCopy(doc any, op Operation) (any, error) {
	fromTokens, err := operationTokens(op.From)
	if err != nil {
		return doc, err
	}
	if _, err := operationTokens(op.Path); err != nil {
		return doc, err
	}

	value, err := pointer.Resolve(doc, fromTokens)
	if err != nil {
		return doc, convertPointerError(err)
	}

	addOp := Operation{Op: "add", Path: op.Path, Value: deepCopy(value), HasPath: true, HasValue: true}
	return applyAdd(doc, addOp)
}

func applyTest(doc any, op Operation) (any, error) {
	tokens, err := operationTokens(op.Path)
	if err != nil {
		return doc, err
	}
	actual, err := pointer.Resolve(doc, tokens)
	if err != nil {
		return doc, convertPointerError(err)
	}
	if !equalJSON(actual, op.Value) {
		return doc, equalError(op.Path)
	}
	return doc, nil
}

// setAt writes replacement over the location addressed by tokens and returns
// the resulting document.
func setAt(doc any, tokens []string, replacement any) (any, error) {
	if len(tokens) == 0 {
		return replacement, nil
	}
	parent, last, err := pointer.ResolveParent(doc, tokens)
	if err != nil {
		return doc, convertPointerError(err)
	}
	switch node := parent.(type) {
	case map[string]any:
		node[last] = replacement
	case []any:
		index, err := pointer.ParseIndex(last)
		if err != nil {
			return doc, convertPointerError(err)
		}
		if index >= len(node) {
			return doc, &OperationError{Code: CodeIndexOutOfRange, Message: last}
		}
		node[index] = replacement
	default:
		return doc, &OperationError{Code: CodePathNotContainer}
	}
	return doc, nil
}

// deepCopy returns an independent copy of a decoded JSON value.
func deepCopy(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, item := range v {
			out[key] = deepCopy(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = deepCopy(item)
		}
		return out
	default:
		return value
	}
}

// isProperPrefix reports whether prefix is a prefix of tokens without being
// equal to it.
func isProperPrefix(prefix, tokens []string) bool {
	if len(prefix) >= len(tokens) {
		return false
	}
	return equalTokens(prefix, tokens[:len(prefix)])
}

func equalTokens(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func convertPointerError(err error) error {
	if code := pointer.CodeOf(err); code != "" {
		return &OperationError{Code: code, Message: err.Error(), Cause: err}
	}
	return &OperationError{Code: CodeInvalidPointer, Message: err.Error(), Cause: err}
}
