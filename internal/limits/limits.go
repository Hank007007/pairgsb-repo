// Package limits holds the resource ceilings applied while compiling a rule.
// The values mirror section 6 of docs/rule-syntax.md.
package limits

const (
	// MaxSourceBytes is the maximum length of a rule source string in bytes.
	MaxSourceBytes = 8192

	// MaxTokens is the maximum number of tokens a rule may produce.
	MaxTokens = 2048

	// MaxParenDepth is the maximum nesting depth of parentheses.
	MaxParenDepth = 64

	// MaxASTNodes is the maximum number of nodes in a compiled tree.
	MaxASTNodes = 1024
)
