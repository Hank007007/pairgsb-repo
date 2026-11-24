// Package edge wires the cache, the upstream object store and the response
// builder into one HTTP handler.
package edge

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"rangecarve/internal/cond"
	"rangecarve/internal/objcache"
	"rangecarve/internal/rangehdr"
	"rangecarve/internal/respond"
)

// Upstream is the object store the edge forwards to. It receives the headers
// the client sent, minus the hop-by-hop ones.
type Upstream interface {
	Get(ctx context.Context, tenant, objectPath string, hdr http.Header) (*http.Response, error)
}

// Config carries the knobs the entry point can set.
type Config struct {
	Cache *objcache.Cache
}

// Server is the download edge.
type Server struct {
	upstream Upstream
	cache    *objcache.Cache
}

// New returns a Server that consults upstream and keeps objects in cache.
func New(upstream Upstream, cfg Config) *Server {
	return &Server{upstream: upstream, cache: cfg.Cache}
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.Serve(w, r)
}

// Serve routes one download request through the cache, the conditional
// headers, the Range parser and the response builder.
func (s *Server) Serve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tenant := r.Header.Get("X-Rangecarve-Tenant")
	objectPath := path.Clean("/" + strings.TrimPrefix(r.URL.Path, "/"))

	obj, fromCache, ok := s.lookup(r.Context(), tenant, objectPath, r.Header)
	if !ok {
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
		return
	}
	if obj.Status == http.StatusNotModified {
		relayNotModified(w, obj)
		return
	}

	decision := cond.Evaluate(cond.Input{
		Method:       r.Method,
		ETag:         obj.ETag,
		LastModified: obj.LastModified,
		Header:       r.Header,
	})
	_ = decision

	spans := rangehdr.Parse(r.Header.Get("Range"), obj.Size).Spans
	if len(spans) == 0 {
		spans = nil
	}
	built := respond.Build(respond.Object{
		ETag:         obj.ETag,
		LastModified: formatHTTPDate(obj.LastModified),
		ContentType:  obj.ContentType,
		Filename:     obj.Filename,
		CacheControl: obj.CacheControl,
		Size:         obj.Size,
	}, spans)

	body := s.body(built, obj, spans, fromCache)

	header := w.Header()
	for key, values := range built.HeaderMap() {
		for _, value := range values {
			header.Add(key, value)
		}
	}
	if r.Method == http.MethodHead {
		header.Set("Content-Length", strconv.FormatInt(int64(len(body)), 10))
		w.WriteHeader(built.Status)
		return
	}
	w.WriteHeader(built.Status)
	w.Write(body)
}

// relayNotModified passes a revalidation result back to the client.
func relayNotModified(w http.ResponseWriter, obj *objcache.Object) {
	header := w.Header()
	for key, value := range map[string]string{
		"ETag":          obj.ETag,
		"Last-Modified": formatHTTPDate(obj.LastModified),
		"Cache-Control": obj.CacheControl,
	} {
		if value != "" {
			header.Set(key, value)
		}
	}
	w.WriteHeader(http.StatusNotModified)
}

// body produces the bytes that accompany the header block Build assembled.
func (s *Server) body(resp *respond.Response, obj *objcache.Object, spans []rangehdr.Span, fromCache bool) []byte {
	if fromCache {
		return obj.Body
	}
	switch len(spans) {
	case 0:
		return obj.Body
	case 1:
		return cut(obj.Body, spans[0])
	default:
		body := renderParts(resp, obj.Body, spans)
		resp.HeaderMap()["Content-Length"] = []string{strconv.FormatInt(spans[0].Len(), 10)}
		return body
	}
}

// cut returns the bytes of span, clamped to what body actually holds.
func cut(body []byte, span rangehdr.Span) []byte {
	start := span.Start
	if start < 0 {
		start = 0
	}
	if start > int64(len(body)) {
		start = int64(len(body))
	}
	end := span.End + 1
	if end < start {
		end = start
	}
	if end > int64(len(body)) {
		end = int64(len(body))
	}
	return body[start:end]
}

// renderParts lays the selected spans out as a multipart/byteranges body.
func renderParts(resp *respond.Response, content []byte, spans []rangehdr.Span) []byte {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	if err := writer.SetBoundary(resp.Boundary); err != nil {
		return buf.Bytes()
	}
	for _, part := range resp.Parts {
		pw, err := writer.CreatePart(part.Header)
		if err != nil {
			continue
		}
		pw.Write(cut(content, part.Span))
	}
	writer.Close()
	return buf.Bytes()
}

// lookup answers from cache when it can and otherwise reads the object from
// upstream, keeping a copy for later requests.
func (s *Server) lookup(ctx context.Context, tenant, objectPath string, in http.Header) (*objcache.Object, bool, bool) {
	if s.cache != nil {
		if obj, ok := s.cache.Get(s.cache.Key(tenant, objectPath, "")); ok {
			return obj, true, true
		}
	}

	resp, err := s.upstream.Get(ctx, tenant, objectPath, forwardHeaders(in))
	if err != nil {
		return nil, false, false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, false
	}
	if resp.StatusCode >= http.StatusBadRequest && resp.StatusCode != http.StatusNotModified {
		return nil, false, false
	}

	obj := objectFromResponse(tenant, objectPath, resp, body)
	if s.cache != nil {
		s.cache.Put(s.cache.Key(tenant, objectPath, obj.ETag), obj)
	}
	return obj, false, true
}

func objectFromResponse(tenant, objectPath string, resp *http.Response, body []byte) *objcache.Object {
	return &objcache.Object{
		Tenant:       tenant,
		Path:         objectPath,
		ETag:         resp.Header.Get("ETag"),
		LastModified: parseHTTPDate(resp.Header.Get("Last-Modified")),
		ContentType:  resp.Header.Get("Content-Type"),
		Filename:     resp.Header.Get("X-Rangecarve-Filename"),
		CacheControl: resp.Header.Get("Cache-Control"),
		Status:       resp.StatusCode,
		Size:         int64(len(body)),
		Body:         body,
	}
}

// forwardHeaders copies the request headers the upstream store may look at.
func forwardHeaders(in http.Header) http.Header {
	out := make(http.Header, len(in))
	for key, values := range in {
		switch http.CanonicalHeaderKey(key) {
		case "Host", "Connection", "Keep-Alive", "Proxy-Connection",
			"Transfer-Encoding", "Upgrade", "Te", "Trailer":
			continue
		}
		out[http.CanonicalHeaderKey(key)] = append([]string(nil), values...)
	}
	return out
}

func parseHTTPDate(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	if t, err := http.ParseTime(value); err == nil {
		return t
	}
	return time.Time{}
}

func formatHTTPDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(http.TimeFormat)
}
