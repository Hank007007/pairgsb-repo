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

	// Range only applies to GET; on anything else it is ignored.
	rangeHeader := ""
	if r.Method == http.MethodGet {
		rangeHeader = r.Header.Get("Range")
	}

	obj, ok := s.lookup(r.Context(), tenant, objectPath, rangeHeader != "", r.Header)
	if !ok {
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
		return
	}

	decision := cond.Evaluate(cond.Input{
		Method:       r.Method,
		ETag:         obj.ETag,
		LastModified: obj.LastModified,
		Header:       r.Header,
	})
	switch decision.Kind {
	case cond.KindNotModified:
		writeMetadata(w.Header(), obj)
		w.WriteHeader(http.StatusNotModified)
		return
	case cond.KindPreconditionFailed:
		w.WriteHeader(http.StatusPreconditionFailed)
		return
	case cond.KindIgnoreRange:
		rangeHeader = ""
	}

	var spans []rangehdr.Span
	if rangeHeader != "" {
		spec := rangehdr.Parse(rangeHeader, obj.Size)
		switch spec.Outcome {
		case rangehdr.OutcomeInvalid:
			w.WriteHeader(http.StatusBadRequest)
			return
		case rangehdr.OutcomeUnsatisfiable:
			header := w.Header()
			header.Set("Accept-Ranges", "bytes")
			header.Set("Content-Range", "bytes */"+strconv.FormatInt(obj.Size, 10))
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		case rangehdr.OutcomeRanges:
			spans = spec.Spans
		}
	}

	built := respond.Build(respond.Object{
		ETag:         obj.ETag,
		LastModified: formatHTTPDate(obj.LastModified),
		ContentType:  obj.ContentType,
		Filename:     obj.Filename,
		CacheControl: obj.CacheControl,
		Size:         obj.Size,
	}, spans)

	body := s.body(built, obj, spans)

	header := w.Header()
	for key, values := range built.HeaderMap() {
		for _, value := range values {
			header.Add(key, value)
		}
	}
	if r.Method == http.MethodHead {
		w.WriteHeader(built.Status)
		return
	}
	w.WriteHeader(built.Status)
	w.Write(body)
}

// writeMetadata echoes the validators a 304 response must carry.
func writeMetadata(header http.Header, obj *objcache.Object) {
	if obj.ETag != "" {
		header.Set("ETag", obj.ETag)
	}
	if value := formatHTTPDate(obj.LastModified); value != "" {
		header.Set("Last-Modified", value)
	}
	if obj.CacheControl != "" {
		header.Set("Cache-Control", obj.CacheControl)
	}
}

// body produces the bytes that accompany the header block Build assembled.
// Cached objects are sliced exactly like freshly fetched ones: the cache
// holds whole objects, never ready-made responses.
func (s *Server) body(resp *respond.Response, obj *objcache.Object, spans []rangehdr.Span) []byte {
	switch len(spans) {
	case 0:
		return obj.Body
	case 1:
		return cut(obj.Body, spans[0])
	default:
		body := renderParts(resp, obj.Body, spans)
		resp.HeaderMap()["Content-Length"] = []string{strconv.FormatInt(int64(len(body)), 10)}
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
//
// Every request is checked against upstream: when the cache holds a current
// entry for the object, its ETag goes out as If-None-Match so that a 304
// refreshes the cached entry's metadata while the cached body is served. A
// 200 carries the new version and replaces it. Responses to requests that
// carried a Range header are never cached.
func (s *Server) lookup(ctx context.Context, tenant, objectPath string, hasRange bool, in http.Header) (*objcache.Object, bool) {
	var cachedKey string
	var cached *objcache.Object
	if s.cache != nil {
		if key, obj, ok := s.cache.Current(tenant, objectPath); ok {
			cachedKey, cached = key, obj
		}
	}

	hdr := forwardHeaders(in)
	if cached != nil && cached.ETag != "" {
		hdr.Set("If-None-Match", cached.ETag)
	}

	resp, err := s.upstream.Get(ctx, tenant, objectPath, hdr)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		// A 304 is only meaningful as an answer to the revalidation above;
		// it refreshes the cached entry's metadata and is never cached
		// itself.
		if cached == nil {
			return nil, false
		}
		refreshObject(cached, resp.Header)
		s.cache.Put(cachedKey, cached)
		return cached, true
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false
	}

	obj := objectFromResponse(tenant, objectPath, resp, body)
	if s.cache != nil && !hasRange {
		s.cache.Put(s.cache.Key(tenant, objectPath, obj.ETag), obj)
	}
	return obj, true
}

// refreshObject folds the metadata of an upstream 304 into a cached entry.
// The body is untouched: a 304 carries no content.
func refreshObject(obj *objcache.Object, header http.Header) {
	if value := header.Get("ETag"); value != "" {
		obj.ETag = value
	}
	if value := parseHTTPDate(header.Get("Last-Modified")); !value.IsZero() {
		obj.LastModified = value
	}
	if value := header.Get("Content-Type"); value != "" {
		obj.ContentType = value
	}
	if value := header.Get("Cache-Control"); value != "" {
		obj.CacheControl = value
	}
	if value := header.Get("X-Rangecarve-Filename"); value != "" {
		obj.Filename = value
	}
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
// Hop-by-hop headers stay behind, and so do Range and the conditional
// headers: those are evaluated at the edge, and forwarding them would let
// upstream answer 206, 304 or 412 in ways the edge cannot relay.
func forwardHeaders(in http.Header) http.Header {
	out := make(http.Header, len(in))
	for key, values := range in {
		switch http.CanonicalHeaderKey(key) {
		case "Host", "Connection", "Keep-Alive", "Proxy-Connection",
			"Transfer-Encoding", "Upgrade", "Te", "Trailer",
			"Range", "If-Match", "If-None-Match", "If-Modified-Since",
			"If-Unmodified-Since", "If-Range":
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
