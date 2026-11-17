// Package httpapi exposes envelope encryption over a small HTTP interface.
package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"kekring/internal/envelope"
	"kekring/internal/keyring"
)

// maxObjectSize bounds request bodies accepted by the server.
const maxObjectSize = 32 << 20

// Server stores sealed objects and serves them over HTTP.
type Server struct {
	kr *keyring.Keyring

	mu      sync.RWMutex
	objects map[string][]byte
}

// New returns a server backed by kr.
func New(kr *keyring.Keyring) *Server {
	return &Server{
		kr:      kr,
		objects: make(map[string][]byte),
	}
}

// Handler returns the HTTP handler for the server.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("PUT /objects/{id}", s.putObject)
	mux.HandleFunc("GET /objects/{id}", s.getObject)
	mux.HandleFunc("POST /objects/{id}/rewrap", s.rewrapObject)
	mux.HandleFunc("GET /keyring", s.listKeys)
	mux.HandleFunc("POST /keyring/rotate", s.rotateKey)
	return mux
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type objectResponse struct {
	ID         string `json:"id"`
	KEKVersion uint32 `json:"kek_version"`
	Size       int    `json:"size"`
}

type keyInfo struct {
	Version uint32         `json:"version"`
	Status  keyring.Status `json:"status"`
}

type keyringResponse struct {
	Keys []keyInfo `json:"keys"`
}

type rotateRequest struct {
	KEK    string `json:"kek"`
	Status string `json:"status"`
}

func (s *Server) putObject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "object id is required")
		return
	}

	plaintext, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxObjectSize))
	if err != nil {
		writeError(w, http.StatusBadRequest, "read request body")
		return
	}

	blob, err := envelope.Seal(s.kr, id, plaintext)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	env, err := envelope.Parse(blob)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.mu.Lock()
	s.objects[id] = blob
	s.mu.Unlock()

	w.Header().Set("Location", "/objects/"+id)
	writeJSON(w, http.StatusCreated, objectResponse{
		ID:         id,
		KEKVersion: env.Header.KEKVersion,
		Size:       len(plaintext),
	})
}

func (s *Server) getObject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	s.mu.RLock()
	blob, ok := s.objects[id]
	s.mu.RUnlock()
	if !ok {
		writeError(w, http.StatusNotFound, "object not found")
		return
	}

	plaintext, err := envelope.Open(s.kr, id, blob)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(plaintext)
}

func (s *Server) rewrapObject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	s.mu.Lock()
	defer s.mu.Unlock()

	blob, ok := s.objects[id]
	if !ok {
		writeError(w, http.StatusNotFound, "object not found")
		return
	}

	rewrapped, err := envelope.Rewrap(s.kr, id, blob)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	env, err := envelope.Parse(rewrapped)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.objects[id] = rewrapped
	writeJSON(w, http.StatusOK, objectResponse{
		ID:         id,
		KEKVersion: env.Header.KEKVersion,
		Size:       len(env.Body) - envelope.TagSize,
	})
}

func (s *Server) listKeys(w http.ResponseWriter, _ *http.Request) {
	keys := s.kr.Keys()
	out := keyringResponse{Keys: make([]keyInfo, 0, len(keys))}
	for _, k := range keys {
		out.Keys = append(out.Keys, keyInfo{Version: k.Version, Status: k.Status})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) rotateKey(w http.ResponseWriter, r *http.Request) {
	var req rotateRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "decode request body")
		return
	}

	material, err := base64.StdEncoding.DecodeString(req.KEK)
	if err != nil {
		writeError(w, http.StatusBadRequest, "kek must be base64 encoded")
		return
	}

	status := keyring.StatusActive
	if req.Status != "" {
		status = keyring.Status(req.Status)
	}

	version := s.kr.NextVersion()
	if err := s.kr.Rotate(keyring.Key{
		Version:  version,
		Status:   status,
		Material: material,
	}); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, keyInfo{Version: version, Status: status})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		_, _ = fmt.Fprintln(w, `{"error":"encode response"}`)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
