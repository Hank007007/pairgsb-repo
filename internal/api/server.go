package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"

	"example.com/outfetch/internal/fetcher"
	"example.com/outfetch/internal/policy"
)

// 错误码，直接出现在响应体的 error 字段里。
const (
	ErrBadRequest   = "bad_request"
	ErrPolicyDenied = "policy_denied"
	ErrUpstream     = "upstream_failed"
)

type Server struct {
	Policy  *policy.Policy
	Fetcher *fetcher.Fetcher
}

func New(p *policy.Policy, f *fetcher.Fetcher) *Server {
	return &Server{Policy: p, Fetcher: f}
}

func (s *Server) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/fetch", s.handleFetch)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	})
	return mux
}

type fetchRequest struct {
	URL string `json:"url"`
}

type fetchResponse struct {
	OK     bool            `json:"ok"`
	Result *fetcher.Result `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
	Detail string          `json:"detail,omitempty"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) handleFetch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, fetchResponse{OK: false, Error: ErrBadRequest, Detail: "POST only"})
		return
	}

	var req fetchRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, fetchResponse{OK: false, Error: ErrBadRequest, Detail: "invalid json"})
		return
	}
	if req.URL == "" {
		writeJSON(w, http.StatusBadRequest, fetchResponse{OK: false, Error: ErrBadRequest, Detail: "url is required"})
		return
	}

	_, err := url.Parse(req.URL)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, fetchResponse{OK: false, Error: ErrBadRequest, Detail: "url is invalid"})
		return
	}

	res, err := s.Fetcher.Get(r.Context(), req.URL)
	if err != nil {
		if errors.Is(err, policy.ErrHostDenied) || errors.Is(err, policy.ErrBlockedIP) ||
			errors.Is(err, policy.ErrScheme) || errors.Is(err, policy.ErrPortDenied) ||
			errors.Is(err, policy.ErrNoHost) || errors.Is(err, fetcher.ErrTooManyRedirects) {
			writeJSON(w, http.StatusForbidden, fetchResponse{OK: false, Error: ErrPolicyDenied, Detail: err.Error()})
			return
		}
		writeJSON(w, http.StatusBadGateway, fetchResponse{OK: false, Error: ErrUpstream, Detail: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, fetchResponse{OK: true, Result: res})
}
