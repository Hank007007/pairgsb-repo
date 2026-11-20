package patch

import (
	"strconv"

	"patchsieve/internal/pointer"
	"patchsieve/internal/policy"
)

// Apply applies ops to doc on behalf of principal.
func Apply(doc any, ops []Operation, principal policy.Principal) (any, error) {
	if err := Check(ops); err != nil {
		return doc, err
	}

	working := doc
	for _, op := range ops {
		next, err := applyOne(working, op)
		if err != nil {
			return working, err
		}
		working = next
	}
	return working, nil
}

func applyOne(doc any, op Operation) (any, error) {
	if err := checkMembers(op); err != nil {
		return doc, err
	}
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
		return doc, nil
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

	switch node := parent.(type) {
	case map[string]any:
		node[last] = op.Value
	case []any:
		if last == "-" {
			node = append(node, op.Value)
			setAt(doc, tokens[:len(tokens)-1], node)
			return doc, nil
		}
		index, err := strconv.Atoi(last)
		if err != nil || index < 0 {
			return doc, &OperationError{Code: CodeInvalidIndex, Message: last}
		}
		if index > len(node) {
			node = append(node, op.Value)
		} else {
			node = append(node, nil)
			copy(node[index+1:], node[index:])
			node[index] = op.Value
		}
		setAt(doc, tokens[:len(tokens)-1], node)
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

	switch node := parent.(type) {
	case map[string]any:
		if _, ok := node[last]; !ok {
			return doc, &OperationError{Code: CodePathNotFound, Message: op.Path}
		}
		delete(node, last)
	case []any:
		index, err := strconv.Atoi(last)
		if err != nil || index < 0 {
			return doc, &OperationError{Code: CodeInvalidIndex, Message: last}
		}
		if index >= len(node) {
			return doc, &OperationError{Code: CodeIndexOutOfRange, Message: last}
		}
		node = append(node[:index], node[index+1:]...)
		setAt(doc, tokens[:len(tokens)-1], node)
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
		index, err := strconv.Atoi(last)
		if err != nil || index < 0 {
			return doc, &OperationError{Code: CodeInvalidIndex, Message: last}
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
	if _, err := operationTokens(op.Path); err != nil {
		return doc, err
	}

	value, err := pointer.Resolve(doc, fromTokens)
	if err != nil {
		return doc, convertPointerError(err)
	}

	removeOp := Operation{Op: "remove", Path: op.From, HasPath: true}
	if _, err := applyRemove(doc, removeOp); err != nil {
		return doc, err
	}

	addOp := Operation{Op: "add", Path: op.Path, Value: value, HasPath: true, HasValue: true}
	if _, err := applyAdd(doc, addOp); err != nil {
		return doc, err
	}
	return doc, nil
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

	addOp := Operation{Op: "add", Path: op.Path, Value: value, HasPath: true, HasValue: true}
	if _, err := applyAdd(doc, addOp); err != nil {
		return doc, err
	}
	return doc, nil
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

// setAt writes replacement over the location addressed by tokens.
func setAt(doc any, tokens []string, replacement any) {
	if len(tokens) == 0 {
		return
	}
	parent, last, err := pointer.ResolveParent(doc, tokens)
	if err != nil {
		return
	}
	switch node := parent.(type) {
	case map[string]any:
		node[last] = replacement
	case []any:
		index, err := strconv.Atoi(last)
		if err != nil || index < 0 || index >= len(node) {
			return
		}
		node[index] = replacement
	}
}

func convertPointerError(err error) error {
	if code := pointer.CodeOf(err); code != "" {
		return &OperationError{Code: code, Message: err.Error(), Cause: err}
	}
	return &OperationError{Code: CodeInvalidPointer, Message: err.Error(), Cause: err}
}
