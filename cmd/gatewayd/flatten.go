package main

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"strictcast/internal/shape"
)

// flattenJSON turns a decoded JSON value into submitted entries. Scalars are
// rendered as their literal text. Object member names and array positions
// become path segments; a member name that itself contains path syntax is
// appended as written and resolved like any other key.
func flattenJSON(prefix string, v any, out *[]shape.Entry) {
	switch val := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			path := k
			if prefix != "" {
				path = prefix + "." + k
			}
			flattenJSON(path, val[k], out)
		}
	case []any:
		for i, item := range val {
			flattenJSON(prefix+"["+strconv.Itoa(i)+"]", item, out)
		}
	default:
		raw, ok := literal(val)
		if !ok {
			return
		}
		*out = append(*out, shape.Entry{Path: prefix, Values: []string{raw}})
	}
}

func literal(v any) (string, bool) {
	switch val := v.(type) {
	case nil:
		return "", false
	case string:
		return val, true
	case json.Number:
		return val.String(), true
	case bool:
		if val {
			return "true", true
		}
		return "false", true
	default:
		return fmt.Sprint(val), true
	}
}

// decodeJSON reads one JSON document and returns its entries.
func decodeJSON(r io.Reader) ([]shape.Entry, error) {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	var root any
	if err := dec.Decode(&root); err != nil {
		return nil, err
	}
	var entries []shape.Entry
	flattenJSON("", root, &entries)
	return entries, nil
}

// formEntries turns an already-parsed form into entries, one per key.
func formEntries(form map[string][]string) []shape.Entry {
	keys := make([]string, 0, len(form))
	for k := range form {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	entries := make([]shape.Entry, 0, len(keys))
	for _, k := range keys {
		entries = append(entries, shape.Entry{Path: k, Values: form[k]})
	}
	return entries
}

func isJSON(contentType string) bool {
	return strings.Contains(strings.ToLower(contentType), "application/json")
}
