package main

import (
	"encoding/json"
	"net/http"

	"strictcast/internal/gate"
	"strictcast/internal/shape"
)

// server serves the demo endpoints. Each endpoint is registered under a name
// that maps to a Shape in shape.Registry.
type server struct{}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name := trimSlash(r.URL.Path)
	sh, ok := shape.Get(name)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown endpoint"})
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	var entries []shape.Entry
	var err error
	if isJSON(r.Header.Get("Content-Type")) {
		entries, err = decodeJSON(r.Body)
	} else {
		if err = r.ParseForm(); err == nil {
			entries = formEntries(r.PostForm)
		}
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, &gate.BindError{Errors: []gate.FieldError{
			{Path: "", Code: shape.CodeBadPath},
		}})
		return
	}

	result, bindErr := gate.Bind(sh, entries)
	if bindErr != nil {
		writeJSON(w, http.StatusBadRequest, bindErr)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func trimSlash(p string) string {
	for len(p) > 0 && p[0] == '/' {
		p = p[1:]
	}
	for len(p) > 0 && p[len(p)-1] == '/' {
		p = p[:len(p)-1]
	}
	return p
}
