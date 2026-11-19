// Package plugin loads plugin.yaml manifests that describe the steps a
// third-party plugin contributes to a CI pipeline.
//
// Only a small, indentation-based subset of YAML is understood: nested
// mappings, sequences of scalars, quoted or plain scalars, and full-line
// comments. That is enough for the manifest format documented in
// docs/design.md and keeps the loader dependency free.
package plugin

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Step is one executable unit declared by a plugin.
type Step struct {
	// Executable is the program that is started for this step.
	Executable string

	// Args holds the argument templates, in order, exactly as declared.
	Args []string

	// Env holds the environment entries declared by the plugin, in order.
	// Entries are kept as raw "KEY=VALUE" strings so that the declaration
	// order is preserved.
	Env []string

	// Workdir is the working directory template for the step.
	Workdir string

	// Positionals holds the templates for trailing, non-option arguments.
	Positionals []string

	// HasPositionals reports whether the step declaration carried a
	// positionals field at all. An explicitly empty list still counts.
	HasPositionals bool
}

// Plugin is a parsed plugin manifest.
type Plugin struct {
	Name  string
	Steps map[string]Step

	order []string
}

// StepNames returns the declared step names in manifest order.
func (p *Plugin) StepNames() []string {
	out := make([]string, len(p.order))
	copy(out, p.order)
	return out
}

// Step returns the named step.
func (p *Plugin) Step(name string) (Step, bool) {
	s, ok := p.Steps[name]
	return s, ok
}

// LoadFile reads and parses a plugin manifest from disk.
func LoadFile(path string) (*Plugin, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("plugin: read %s: %w", path, err)
	}
	p, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("plugin: parse %s: %w", path, err)
	}
	return p, nil
}

// Parse decodes a plugin manifest.
func Parse(data []byte) (*Plugin, error) {
	root, err := parseYAML(string(data))
	if err != nil {
		return nil, err
	}
	doc, ok := root.(*mapping)
	if !ok {
		return nil, errors.New("manifest root must be a mapping")
	}

	p := &Plugin{Steps: map[string]Step{}}
	if name, ok := doc.str("name"); ok {
		p.Name = name
	}

	steps, ok := doc.child("steps")
	if !ok {
		return nil, errors.New("manifest is missing a steps mapping")
	}

	for _, name := range steps.keys {
		raw, ok := steps.mapAt(name)
		if !ok {
			return nil, fmt.Errorf("step %q must be a mapping", name)
		}
		step, err := decodeStep(raw)
		if err != nil {
			return nil, fmt.Errorf("step %q: %w", name, err)
		}
		if step.Executable == "" {
			return nil, fmt.Errorf("step %q: executable is required", name)
		}
		p.Steps[name] = step
		p.order = append(p.order, name)
	}
	if len(p.order) == 0 {
		return nil, errors.New("manifest declares no steps")
	}
	return p, nil
}

func decodeStep(raw *mapping) (Step, error) {
	var step Step

	if v, ok := raw.str("executable"); ok {
		step.Executable = v
	}
	if v, ok := raw.str("workdir"); ok {
		step.Workdir = v
	}

	if v, ok := raw.value("args"); ok {
		args, err := stringList(v, "args")
		if err != nil {
			return Step{}, err
		}
		step.Args = args
	}
	if v, ok := raw.value("env"); ok {
		env, err := stringList(v, "env")
		if err != nil {
			return Step{}, err
		}
		step.Env = env
	}
	if v, ok := raw.value("positionals"); ok {
		pos, err := stringList(v, "positionals")
		if err != nil {
			return Step{}, err
		}
		step.Positionals = pos
		step.HasPositionals = true
	}

	return step, nil
}

func stringList(v any, field string) ([]string, error) {
	if v == nil {
		return nil, nil
	}
	list, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be a sequence", field)
	}
	out := make([]string, 0, len(list))
	for i, item := range list {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("%s[%d] must be a scalar", field, i)
		}
		out = append(out, s)
	}
	return out, nil
}

// --- minimal YAML subset -------------------------------------------------

// mapping keeps insertion order so that declarations round-trip predictably.
type mapping struct {
	keys []string
	vals map[string]any
}

func newMapping() *mapping {
	return &mapping{vals: map[string]any{}}
}

func (m *mapping) set(key string, v any) {
	if _, exists := m.vals[key]; !exists {
		m.keys = append(m.keys, key)
	}
	m.vals[key] = v
}

func (m *mapping) value(key string) (any, bool) {
	v, ok := m.vals[key]
	return v, ok
}

func (m *mapping) str(key string) (string, bool) {
	v, ok := m.vals[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func (m *mapping) child(key string) (*mapping, bool) {
	v, ok := m.vals[key]
	if !ok {
		return nil, false
	}
	c, ok := v.(*mapping)
	return c, ok
}

func (m *mapping) mapAt(key string) (*mapping, bool) { return m.child(key) }

type yamlLine struct {
	indent int
	text   string
	line   int
}

func parseYAML(src string) (any, error) {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	lines := make([]yamlLine, 0, 32)
	for i, raw := range strings.Split(src, "\n") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		trimmed := strings.TrimLeft(raw, " \t")
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		leading := raw[:len(raw)-len(trimmed)]
		if strings.ContainsRune(leading, '\t') {
			return nil, fmt.Errorf("line %d: tabs are not allowed for indentation", i+1)
		}
		lines = append(lines, yamlLine{indent: len(leading), text: strings.TrimRight(trimmed, " "), line: i + 1})
	}
	if len(lines) == 0 {
		return nil, errors.New("manifest is empty")
	}
	node, next, err := parseBlock(lines, 0, lines[0].indent)
	if err != nil {
		return nil, err
	}
	if next != len(lines) {
		return nil, fmt.Errorf("line %d: unexpected indentation", lines[next].line)
	}
	return node, nil
}

func parseBlock(lines []yamlLine, start, indent int) (any, int, error) {
	if isSequenceItem(lines[start].text) {
		return parseSequence(lines, start, indent)
	}
	return parseMapping(lines, start, indent)
}

func isSequenceItem(text string) bool {
	return text == "-" || strings.HasPrefix(text, "- ")
}

func parseSequence(lines []yamlLine, start, indent int) (any, int, error) {
	items := []any{}
	i := start
	for i < len(lines) {
		ln := lines[i]
		if ln.indent < indent {
			break
		}
		if ln.indent > indent {
			return nil, 0, fmt.Errorf("line %d: unexpected indentation", ln.line)
		}
		if !isSequenceItem(ln.text) {
			break
		}
		rest := strings.TrimSpace(strings.TrimPrefix(ln.text, "-"))
		items = append(items, parseScalar(rest))
		i++
	}
	return items, i, nil
}

func parseMapping(lines []yamlLine, start, indent int) (any, int, error) {
	obj := newMapping()
	i := start
	for i < len(lines) {
		ln := lines[i]
		if ln.indent < indent {
			break
		}
		if ln.indent > indent {
			return nil, 0, fmt.Errorf("line %d: unexpected indentation", ln.line)
		}
		key, rest, ok := splitKey(ln.text)
		if !ok {
			return nil, 0, fmt.Errorf("line %d: expected \"key: value\"", ln.line)
		}
		i++
		if rest != "" {
			obj.set(key, parseScalar(rest))
			continue
		}
		if i < len(lines) && lines[i].indent > indent {
			child, next, err := parseBlock(lines, i, lines[i].indent)
			if err != nil {
				return nil, 0, err
			}
			obj.set(key, child)
			i = next
			continue
		}
		if i < len(lines) && lines[i].indent == indent && isSequenceItem(lines[i].text) {
			child, next, err := parseSequence(lines, i, indent)
			if err != nil {
				return nil, 0, err
			}
			obj.set(key, child)
			i = next
			continue
		}
		obj.set(key, nil)
	}
	return obj, i, nil
}

func splitKey(text string) (key, rest string, ok bool) {
	idx := strings.Index(text, ":")
	if idx < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(text[:idx])
	rest = strings.TrimSpace(text[idx+1:])
	if key == "" {
		return "", "", false
	}
	return key, rest, true
}

func parseScalar(text string) any {
	text = strings.TrimSpace(text)
	if len(text) >= 2 {
		if text[0] == '"' && text[len(text)-1] == '"' {
			if s, err := strconv.Unquote(text); err == nil {
				return s
			}
			return text[1 : len(text)-1]
		}
		if text[0] == '\'' && text[len(text)-1] == '\'' {
			return strings.ReplaceAll(text[1:len(text)-1], "''", "'")
		}
	}
	return text
}
