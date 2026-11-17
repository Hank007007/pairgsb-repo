// Package parse reads sudopolicy configuration files into policy tables.
//
// The grammar is described in docs/policy-grammar.md.
package parse

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"sudopolicy/internal/policy"
)

// Parse reads a policy configuration and returns the resulting table.
//
// Rules are returned in file order. Comments and blank lines are ignored.
func Parse(r io.Reader) (*policy.Table, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	table := &policy.Table{}
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		entry, err := parseEntry(scanner.Text(), lineNo)
		if err != nil {
			return nil, err
		}
		if entry != nil {
			table.Entries = append(table.Entries, *entry)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return table, nil
}

// parseEntry parses one line. It returns nil for lines that do not produce an
// entry.
func parseEntry(line string, lineNo int) (*policy.Entry, error) {
	text := strings.TrimSpace(line)
	if text == "" || strings.HasPrefix(text, "#") {
		return nil, nil
	}

	negated := false
	if strings.HasPrefix(text, "!") {
		negated = true
		text = strings.TrimSpace(text[1:])
	}

	eq := strings.Index(text, "=")
	if eq < 0 {
		return nil, fmt.Errorf("line %d: missing '='", lineNo)
	}

	left := strings.Fields(text[:eq])
	if len(left) != 2 {
		return nil, fmt.Errorf("line %d: expected users and hosts before '='", lineNo)
	}
	users := splitList(left[0])
	hosts := splitList(left[1])
	if len(users) == 0 || len(hosts) == 0 {
		return nil, fmt.Errorf("line %d: empty user or host list", lineNo)
	}

	right := strings.TrimSpace(text[eq+1:])
	if !strings.HasPrefix(right, "(") {
		return nil, fmt.Errorf("line %d: expected target users in parentheses", lineNo)
	}
	closeParen := strings.Index(right, ")")
	if closeParen < 0 {
		return nil, fmt.Errorf("line %d: unterminated target user list", lineNo)
	}
	runas := splitList(right[1:closeParen])
	if len(runas) == 0 {
		return nil, fmt.Errorf("line %d: empty target user list", lineNo)
	}

	tokens := strings.Fields(right[closeParen+1:])
	var tags []string
	for len(tokens) > 0 && strings.HasSuffix(tokens[0], ":") {
		name := strings.TrimSuffix(tokens[0], ":")
		if name == "" {
			break
		}
		tags = append(tags, strings.ToUpper(name))
		tokens = tokens[1:]
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("line %d: missing command", lineNo)
	}

	command := tokens[0]
	if !strings.HasPrefix(command, "/") {
		return nil, fmt.Errorf("line %d: command %q is not an absolute path", lineNo, command)
	}

	entry := &policy.Entry{
		Line:    lineNo,
		Negated: negated,
		Users:   users,
		Hosts:   hosts,
		Runas:   runas,
		Tags:    tags,
		Command: command,
		Args:    append([]string(nil), tokens[1:]...),
	}
	return entry, nil
}

// splitList splits a comma separated field and drops empty elements.
func splitList(field string) []string {
	parts := strings.Split(field, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
