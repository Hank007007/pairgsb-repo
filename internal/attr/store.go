// Package attr holds the attribute sources used during policy evaluation.
package attr

import (
	"encoding/json"
	"strings"
	"sync"
)

// Directory is the authoritative attribute source maintained by the identity
// synchronisation job. Values are stored per principal, per resource and per
// request context.
type Directory struct {
	mu        sync.RWMutex
	principal map[string]any
	resource  map[string]any
	context   map[string]any
	types     map[string]string
}

// NewDirectory returns an empty directory.
func NewDirectory() *Directory {
	return &Directory{
		principal: make(map[string]any),
		resource:  make(map[string]any),
		context:   make(map[string]any),
		types:     make(map[string]string),
	}
}

// SetPrincipal replaces the directory view of one principal's attributes and
// merges the declared types supplied alongside it.
func (d *Directory) SetPrincipal(values map[string]any, types map[string]string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.principal = make(map[string]any, len(values))
	for k, v := range values {
		d.principal[k] = v
	}
	for k, t := range types {
		d.types["principal."+k] = t
	}
}

// SetResource replaces the directory view of one resource's attributes.
func (d *Directory) SetResource(values map[string]any, types map[string]string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.resource = make(map[string]any, len(values))
	for k, v := range values {
		d.resource[k] = v
	}
	for k, t := range types {
		d.types["resource."+k] = t
	}
}

// SetContext replaces the request context attributes.
func (d *Directory) SetContext(values map[string]any, types map[string]string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.context = make(map[string]any, len(values))
	for k, v := range values {
		d.context[k] = v
	}
	for k, t := range types {
		d.types["context."+k] = t
	}
}

// source returns the attribute map for one top-level source name.
func (d *Directory) source(name string) map[string]any {
	switch name {
	case "principal":
		return d.principal
	case "resource":
		return d.resource
	case "context":
		return d.context
	}
	return nil
}

// lookup returns the directory value stored at path and whether it exists.
func (d *Directory) lookup(path string) (any, bool) {
	parts := strings.Split(path, ".")
	if len(parts) < 2 {
		return nil, false
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	src := d.source(parts[0])
	if src == nil {
		return nil, false
	}
	return descend(src, parts[1:])
}

// Values returns a copy of the directory values for one source.
func (d *Directory) Values(source string) map[string]any {
	d.mu.RLock()
	defer d.mu.RUnlock()
	src := d.source(source)
	out := make(map[string]any, len(src))
	for k, v := range src {
		out[k] = v
	}
	return out
}

// RequestAttributes carries the attribute hints attached to a request body.
type RequestAttributes struct {
	Principal map[string]any
	Resource  map[string]any
	Context   map[string]any
	Types     map[string]string
}

func (r *RequestAttributes) source(name string) map[string]any {
	if r == nil {
		return nil
	}
	switch name {
	case "principal":
		return r.Principal
	case "resource":
		return r.Resource
	case "context":
		return r.Context
	}
	return nil
}

// lookup returns the request hint stored at path and whether it was present.
func (r *RequestAttributes) lookup(path string) (any, bool) {
	parts := strings.Split(path, ".")
	if len(parts) < 2 {
		return nil, false
	}
	src := r.source(parts[0])
	if src == nil {
		return nil, false
	}
	return descend(src, parts[1:])
}

// Store combines the directory and the request hints for one evaluation.
type Store struct {
	dir     *Directory
	request *RequestAttributes
	action  string
}

// NewStore builds the attribute view used by a single decision.
func NewStore(dir *Directory, req *RequestAttributes, action string) *Store {
	return &Store{dir: dir, request: req, action: action}
}

// Lookup resolves an attribute path and reports whether it exists. The
// directory is authoritative, so a directory value always wins over a request
// hint with the same path.
func (s *Store) Lookup(path string) (any, bool) {
	if path == "action" {
		return s.action, true
	}
	if s.dir != nil {
		if v, ok := s.dir.lookup(path); ok {
			return v, true
		}
	}
	if s.request != nil {
		if v, ok := s.request.lookup(path); ok {
			return v, true
		}
	}
	return nil, false
}

// Get resolves an attribute path. Request hints are only used for paths that
// the authoritative directory does not contain.
func (s *Store) Get(path string) any {
	v, _ := s.Lookup(path)
	return v
}

// Type reports the declared type of an attribute path.
func (s *Store) Type(path string) (string, bool) {
	if path == "action" {
		return "string", true
	}
	if s.dir != nil {
		s.dir.mu.RLock()
		t, ok := s.dir.types[path]
		s.dir.mu.RUnlock()
		if ok {
			return t, true
		}
	}
	if s.request != nil {
		if t, ok := s.request.Types[path]; ok {
			return t, true
		}
	}
	if v, ok := s.Lookup(path); ok {
		if t := inferType(v); t != "" {
			return t, true
		}
	}
	return "", false
}

func descend(current any, parts []string) (any, bool) {
	for _, part := range parts {
		switch obj := current.(type) {
		case map[string]any:
			v, ok := obj[part]
			if !ok {
				return nil, false
			}
			current = v
		case map[string]string:
			v, ok := obj[part]
			if !ok {
				return nil, false
			}
			current = v
		default:
			return nil, false
		}
	}
	return current, true
}

func inferType(v any) string {
	switch v.(type) {
	case string:
		return "string"
	case bool:
		return "bool"
	case float32, float64, int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64, json.Number:
		return "number"
	case []any, []string:
		return "list"
	}
	return ""
}
