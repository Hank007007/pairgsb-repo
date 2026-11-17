// Package graphql contains the GraphQL document model and a parser for the
// executable subset of the language: operations, selections, fragments,
// directives, arguments and variables.
package graphql

// OperationType is the keyword with which an operation is introduced.
type OperationType string

const (
	Query        OperationType = "query"
	Mutation     OperationType = "mutation"
	Subscription OperationType = "subscription"
)

// Document is a parsed GraphQL request document.
type Document struct {
	Operations []*Operation
	Fragments  map[string]*Fragment
}

// Operation is a single query, mutation or subscription.
type Operation struct {
	Type         OperationType
	Name         string
	Variables    []*VariableDefinition
	Directives   []*Directive
	SelectionSet []Selection
}

// VariableDefinition declares a variable used by an operation.
type VariableDefinition struct {
	Name    string
	Type    string
	Default Value
}

// Fragment is a named fragment definition.
type Fragment struct {
	Name          string
	TypeCondition string
	Directives    []*Directive
	SelectionSet  []Selection
}

// Selection is one node inside a selection set.
type Selection interface {
	selectionNode()
}

// Field selects a field, optionally under an alias.
type Field struct {
	Alias        string
	Name         string
	Arguments    []*Argument
	Directives   []*Directive
	SelectionSet []Selection
}

func (*Field) selectionNode() {}

// ResponseName returns the key under which the field appears in the response,
// i.e. the alias when one is present and the field name otherwise.
func (f *Field) ResponseName() string {
	if f.Alias != "" {
		return f.Alias
	}
	return f.Name
}

// FragmentSpread references a named fragment.
type FragmentSpread struct {
	Name       string
	Directives []*Directive
}

func (*FragmentSpread) selectionNode() {}

// InlineFragment is an unnamed selection set with an optional type condition.
type InlineFragment struct {
	TypeCondition string
	Directives    []*Directive
	SelectionSet  []Selection
}

func (*InlineFragment) selectionNode() {}

// Argument is a name/value pair passed to a field or directive.
type Argument struct {
	Name  string
	Value Value
}

// Directive is an @-prefixed annotation attached to a selection or operation.
type Directive struct {
	Name      string
	Arguments []*Argument
}

// Value is a GraphQL input value.
type Value interface {
	valueNode()
}

// Variable references a value supplied at request time.
type Variable struct {
	Name string
}

func (*Variable) valueNode() {}

// IntValue is an integer literal.
type IntValue struct {
	Raw string
}

func (*IntValue) valueNode() {}

// FloatValue is a floating point literal.
type FloatValue struct {
	Raw string
}

func (*FloatValue) valueNode() {}

// StringValue is a string literal.
type StringValue struct {
	Value string
}

func (*StringValue) valueNode() {}

// BooleanValue is true or false.
type BooleanValue struct {
	Value bool
}

func (*BooleanValue) valueNode() {}

// NullValue is the null literal.
type NullValue struct{}

func (*NullValue) valueNode() {}

// EnumValue is an unquoted enum literal.
type EnumValue struct {
	Value string
}

func (*EnumValue) valueNode() {}

// ListValue is a list literal.
type ListValue struct {
	Values []Value
}

func (*ListValue) valueNode() {}

// ObjectValue is an input object literal.
type ObjectValue struct {
	Fields []*ObjectField
}

func (*ObjectValue) valueNode() {}

// ObjectField is one member of an input object literal.
type ObjectField struct {
	Name  string
	Value Value
}
