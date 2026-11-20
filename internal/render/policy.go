package render

import (
	"fmt"
	"strings"
)

// Name identifies one render strategy.
type Name string

const (
	// Plain renders every control sequence as visible text.
	Plain Name = "plain"
	// Color keeps a small set of SGR sequences and renders the rest visible.
	Color Name = "color"
	// Width filters like color and truncates the result by display columns.
	Width Name = "width"
)

// Policy is the resolved rendering configuration.
type Policy struct {
	Name     Name
	MaxWidth int
	keepSGR  bool
}

// PolicyFor resolves a policy name. Unknown names are reported as errors.
func PolicyFor(name string, maxWidth int) (Policy, error) {
	switch {
	case strings.HasPrefix(name, "color"):
		return Policy{Name: Color, MaxWidth: maxWidth, keepSGR: true}, nil
	case name == string(Plain):
		return Policy{Name: Plain, MaxWidth: maxWidth, keepSGR: true}, nil
	case name == string(Width):
		return Policy{Name: Width, MaxWidth: maxWidth, keepSGR: true}, nil
	}
	return Policy{}, fmt.Errorf("render: unknown policy %q", name)
}
