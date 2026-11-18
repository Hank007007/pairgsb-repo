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
	defer func() {
		h.log.Log(ip, r.Method, r.URL.Path, r.UserAgent(), rec.status)
	}()

	addr, err := netip.ParseAddr(ip)
	if err != nil {
		http.Error(rec, "cannot determine client address", http.StatusInternalServerError)
		return
	}
	addr = addr.Unmap()

	rt := h.selectRoute(r)
	if rt == nil {
		http.Error(rec, "no route", http.StatusNotFound)
		return
	}
	if !matchPrefix(r, &rt.cfg) || !rt.permits(addr) {
		http.Error(rec, "forbidden", http.StatusForbidden)
		return
	}

	target := h.upstreamURL(rt, r)
	outReq, err := http.NewRequestWithContext(r.Context(), r.Method, target.String(), r.Body)
	if err != nil {
		http.Error(rec, "cannot build upstream request", http.StatusBadGateway)
		return
	}
	outReq.ContentLength = r.ContentLength
	forwardHeaders(outReq.Header, r.Header)
	outReq.Header.Add("X-Forwarded-For", ip)
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
func (h *Handler) selectRoute(r *http.Request) *route {
	path := r.URL.Path
	var best *route
	for i := range h.routes {
		rt := &h.routes[i]
		prefix := rt.cfg.Prefix
		if prefix != "/" && path != prefix && !strings.HasPrefix(path, prefix+"/") {
			continue
		}
		if best == nil || len(prefix) > len(best.cfg.Prefix) {
			best = rt
		}
	}
	return best
}

func (h *Handler) upstreamURL(rt *route, r *http.Request) *url.URL {
	target := *rt.upstream
	target.Path = joinURLPath(target.Path, r.URL.Path)
	target.RawPath = ""
	target.RawQuery = r.URL.RawQuery
	return &target
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

// forwardHeaders copies src into dst.
func forwardHeaders(dst, src http.Header) {
	for name, values := range src {
		for _, value := range values {
			dst.Add(name, value)
		}
	}
}

// clientIP returns the address hopfence attributes the request to.
func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		first := forwarded
		if i := strings.IndexByte(first, ','); i >= 0 {
			first = first[:i]
		}
		if first = strings.TrimSpace(first); first != "" {
			return first
		}
	}
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
