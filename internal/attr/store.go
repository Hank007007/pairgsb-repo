// Package attr holds the attribute sources used during policy evaluation.
package attr

import (
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

// lookup returns the directory value stored at path. A source that exists but
// has no entry for the key yields the zero string.
func (d *Directory) lookup(path string) any {
	parts := strings.Split(path, ".")
	if len(parts) != 2 {
		return nil
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	src := d.source(parts[0])
	if src == nil {
		return nil
	}
	if v, ok := src[parts[1]]; ok {
		return v
	}
	return ""
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
	if len(parts) != 2 {
		return nil, false
	}
	src := r.source(parts[0])
	if src == nil {
		return nil, false
	}
	v, ok := src[parts[1]]
	return v, ok
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

// Get resolves an attribute path. Request hints are consulted first, then the
// directory.
func (s *Store) Get(path string) any {
	if path == "action" {
		return s.action
	}
	if s.request != nil {
		if v, ok := s.request.lookup(path); ok {
			return v
		}
	}
	if s.dir != nil {
		return s.dir.lookup(path)
	}
	return nil
}

// Type reports the declared type of an attribute path.
func (s *Store) Type(path string) (string, bool) {
	if s.request != nil {
		if t, ok := s.request.Types[path]; ok {
			return t, true
		}
	}
	if s.dir != nil {
		s.dir.mu.RLock()
		defer s.dir.mu.RUnlock()
		t, ok := s.dir.types[path]
		return t, ok
	}
	return "", false
}
