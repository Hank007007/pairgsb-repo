package subject

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// SystemAuthenticated is the built-in group carried by every authenticated
// subject.
const SystemAuthenticated = "system:authenticated"

// Directory holds the membership model loaded from groups.yaml.
type Directory struct {
	// Users maps a user to the groups named directly in the file.
	Users map[string][]string
	// Groups maps a group to its immediate child groups.
	Groups map[string][]string
}

// Expand returns the groups a subject belongs to, given the groups supplied in
// the review.
func (d *Directory) Expand(user string, reviewGroups []string) []string {
	result := make([]string, 0, len(reviewGroups)+4)
	for _, group := range reviewGroups {
		result = appendUnique(result, group)
		for _, child := range d.Groups[group] {
			result = appendUnique(result, child)
		}
	}
	for _, group := range d.Users[user] {
		result = appendUnique(result, group)
		for _, child := range d.Groups[group] {
			result = appendUnique(result, child)
		}
	}
	return result
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

// LoadDirectory reads a groups.yaml file from disk.
func LoadDirectory(path string) (*Directory, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ParseDirectory(string(data))
}

// ParseDirectory parses the small YAML subset used by groups.yaml:
//
//	users:
//	  alice:
//	    - platform
//	groups:
//	  platform:
//	    - sre
func ParseDirectory(text string) (*Directory, error) {
	dir := &Directory{
		Users:  map[string][]string{},
		Groups: map[string][]string{},
	}
	section := ""
	var current string
	for lineNo, raw := range strings.Split(text, "\n") {
		line := stripComment(raw)
		if strings.TrimSpace(line) == "" {
			continue
		}
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			key := strings.TrimSpace(strings.TrimSuffix(line, ":"))
			switch key {
			case "users", "groups":
				section = key
				current = ""
			default:
				return nil, fmt.Errorf("groups.yaml line %d: unknown section %q", lineNo+1, key)
			}
			continue
		}
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") {
			if current == "" {
				return nil, fmt.Errorf("groups.yaml line %d: list item without owner", lineNo+1)
			}
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
			if section == "users" {
				dir.Users[current] = append(dir.Users[current], value)
			} else if section == "groups" {
				dir.Groups[current] = append(dir.Groups[current], value)
			}
			continue
		}
		if !strings.HasSuffix(trimmed, ":") && !strings.HasSuffix(trimmed, ": []") {
			return nil, fmt.Errorf("groups.yaml line %d: expected a mapping entry", lineNo+1)
		}
		current = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(trimmed, " []"), ":"))
		if section == "users" {
			if _, ok := dir.Users[current]; !ok {
				dir.Users[current] = nil
			}
		} else if section == "groups" {
			if _, ok := dir.Groups[current]; !ok {
				dir.Groups[current] = nil
			}
		} else {
			return nil, errors.New("groups.yaml: content before a section header")
		}
	}
	return dir, nil
}

func stripComment(line string) string {
	if idx := strings.Index(line, "#"); idx >= 0 {
		return line[:idx]
	}
	return line
}
