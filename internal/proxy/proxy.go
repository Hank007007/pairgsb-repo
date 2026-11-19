// Package proxy implements the hopfence forwarding handler.
package proxy

import (
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"hopfence/internal/accesslog"
	"hopfence/internal/config"
)

// route is the compiled form of a config.Route.
type route struct {
	cfg      config.Route
	upstream *url.URL
	allow    []netip.Prefix
	deny     []netip.Prefix
}

// Handler forwards requests to the upstream selected by the routing table.
type Handler struct {
	routes []route
	client *http.Client
	log    *accesslog.Logger
}

// New compiles cfg into a Handler. The returned handler is safe for concurrent
// use.
func New(cfg config.Config, log *accesslog.Logger) (*Handler, error) {
	if log == nil {
		log = accesslog.New(io.Discard)
	}
	h := &Handler{log: log}
	for _, rc := range cfg.Routes {
		upstream, err := url.Parse(rc.Upstream)
		if err != nil {
			return nil, err
		}
		allow, err := config.ParseCIDRs(rc.Allow)
		if err != nil {
			return nil, err
		}
		deny, err := config.ParseCIDRs(rc.Deny)
		if err != nil {
			return nil, err
		}
		h.routes = append(h.routes, route{
			cfg:      rc,
			upstream: upstream,
			allow:    allow,
			deny:     deny,
		})
	}

	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = time.Duration(config.DefaultTimeoutSeconds) * time.Second
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableCompression = true
	h.client = &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return h, nil
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	ip := clientIP(r)
	logPath := r.URL.Path
	defer func() {
		h.log.Log(ip, r.Method, logPath, r.UserAgent(), rec.status)
	}()

	addr, err := netip.ParseAddr(ip)
	if err != nil {
		http.Error(rec, "cannot determine client address", http.StatusInternalServerError)
		return
	}
	addr = addr.Unmap()
	ip = addr.String()

	path, err := decodePath(r)
	if err != nil {
		http.Error(rec, "invalid path encoding", http.StatusBadRequest)
		return
	}
	logPath = path

	rt := h.selectRoute(path)
	if rt == nil {
		http.Error(rec, "no route", http.StatusNotFound)
		return
	}

	cleaned, ok := cleanPath(rt.cfg.Prefix, path)
	if !ok {
		http.Error(rec, "forbidden", http.StatusForbidden)
		return
	}
	logPath = cleaned

	if !matchPrefix(cleaned, &rt.cfg) || !rt.permits(addr) {
		http.Error(rec, "forbidden", http.StatusForbidden)
		return
	}

	target := h.upstreamURL(rt, cleaned, r)
	outReq, err := http.NewRequestWithContext(r.Context(), r.Method, target.String(), r.Body)
	if err != nil {
		http.Error(rec, "cannot build upstream request", http.StatusBadGateway)
		return
	}
	outReq.ContentLength = r.ContentLength
	forwardHeaders(outReq.Header, r.Header)
	outReq.Header.Set("X-Forwarded-For", ip)
	outReq.Header.Set("X-Real-IP", ip)

	resp, err := h.client.Do(outReq)
	if err != nil {
		http.Error(rec, "upstream unavailable", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	copyResponseHeaders(rec.Header(), resp.Header)
	rec.WriteHeader(resp.StatusCode)
	io.Copy(rec, resp.Body)
}

// selectRoute returns the matching route with the longest prefix. Ties are
// resolved in configuration order.
func (h *Handler) selectRoute(path string) *route {
	var best *route
	for i := range h.routes {
		rt := &h.routes[i]
		prefix := rt.cfg.Prefix
		if !pathHasPrefix(path, prefix) {
			continue
		}
		if best == nil || len(prefix) > len(best.cfg.Prefix) {
			best = rt
		}
	}
	return best
}

func (h *Handler) upstreamURL(rt *route, path string, r *http.Request) *url.URL {
	target := *rt.upstream
	target.Path = joinURLPath(target.Path, path)
	target.RawPath = ""
	target.RawQuery = r.URL.RawQuery
	return &target
}

// decodePath percent-decodes the request path exactly once. The Go HTTP
// server has already decoded URL.Path; EscapedPath returns the original
// encoded form so decoding here does not recurse.
func decodePath(r *http.Request) (string, error) {
	return url.PathUnescape(r.URL.EscapedPath())
}

// cleanPath removes empty and "." segments and resolves ".." segments
// relative to the selected route prefix. It reports false if a ".." would
// escape above the prefix root.
func cleanPath(prefix, path string) (string, bool) {
	var rest string
	switch {
	case prefix == "/":
		rest = strings.TrimPrefix(path, "/")
	case path == prefix:
		rest = ""
	default:
		rest = strings.TrimPrefix(path, prefix+"/")
	}

	segments := make([]string, 0, strings.Count(rest, "/")+1)
	for _, segment := range strings.Split(rest, "/") {
		switch segment {
		case "", ".":
			continue
		case "..":
			if len(segments) == 0 {
				return "", false
			}
			segments = segments[:len(segments)-1]
		default:
			segments = append(segments, segment)
		}
	}

	if prefix == "/" {
		if len(segments) == 0 {
			return "/", true
		}
		return "/" + strings.Join(segments, "/"), true
	}
	if len(segments) == 0 {
		return prefix, true
	}
	return prefix + "/" + strings.Join(segments, "/"), true
}

func joinURLPath(base, path string) string {
	switch {
	case strings.HasSuffix(base, "/") && strings.HasPrefix(path, "/"):
		return base + path[1:]
	case !strings.HasSuffix(base, "/") && !strings.HasPrefix(path, "/"):
		return base + "/" + path
	default:
		return base + path
	}
}

// permits reports whether addr passes the route's address ACL.
func (rt *route) permits(addr netip.Addr) bool {
	for _, prefix := range rt.deny {
		if prefix.Contains(addr) {
			return false
		}
	}
	if len(rt.allow) == 0 {
		return true
	}
	for _, prefix := range rt.allow {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// clientIP returns the address hopfence attributes the request to. It is
// derived only from the TCP peer; forwarding headers are client input and are
// never trusted.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return strings.Trim(r.RemoteAddr, "[]")
	}
	return host
}

// statusRecorder captures the status code written to the client.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wroteHeader {
		s.status = code
		s.wroteHeader = true
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(p []byte) (int, error) {
	if !s.wroteHeader {
		s.wroteHeader = true
	}
	return s.ResponseWriter.Write(p)
}

func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
