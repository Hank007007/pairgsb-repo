package proxy

import (
	"net/http"
	"strings"

	"hopfence/internal/config"
)

// hopHeaders are the hop-by-hop headers defined by RFC 7230 section 6.1.
var hopHeaders = []string{
	"Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

// matchPrefix reports whether path may be served by rt: the path must be
// covered by the route prefix and must not be covered by one of the route's
// path exclusions. Prefixes match on path-segment boundaries.
func matchPrefix(path string, rt *config.Route) bool {
	if !pathHasPrefix(path, rt.Prefix) {
		return false
	}
	for _, excluded := range rt.DenyPaths {
		if pathHasPrefix(path, excluded) {
			return false
		}
	}
	return true
}

// pathHasPrefix reports whether path equals prefix or is a descendant of it.
// Matching is segment-aware: /admin matches /admin and /admin/ops, but not
// /admin-backup or /adminpanel.
func pathHasPrefix(path, prefix string) bool {
	if prefix == "/" {
		return strings.HasPrefix(path, "/")
	}
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

// forwardHeaders copies end-to-end request headers into dst. Hop-by-hop
// headers, headers named by Connection, and inbound client identity headers
// are removed.
func forwardHeaders(dst, src http.Header) {
	hop := hopHeaderSet(src)
	for name, values := range src {
		lower := strings.ToLower(name)
		if _, ok := hop[lower]; ok {
			continue
		}
		if lower == "x-forwarded-for" || lower == "x-real-ip" {
			continue
		}
		for _, value := range values {
			dst.Add(name, value)
		}
	}
}

// hopHeaderSet returns the lower-case names of all hop-by-hop headers for a
// message, including those named by its Connection header.
func hopHeaderSet(src http.Header) map[string]struct{} {
	hop := make(map[string]struct{}, len(hopHeaders))
	for _, name := range hopHeaders {
		hop[strings.ToLower(name)] = struct{}{}
	}
	for name, values := range src {
		if !strings.EqualFold(name, "Connection") {
			continue
		}
		for _, value := range values {
			for _, token := range strings.Split(value, ",") {
				token = strings.TrimSpace(token)
				if token != "" {
					hop[strings.ToLower(token)] = struct{}{}
				}
			}
		}
	}
	return hop
}

// copyResponseHeaders copies upstream response headers to the client.
func copyResponseHeaders(dst, src http.Header) {
	hop := hopHeaderSet(src)
	for name, values := range src {
		if _, ok := hop[strings.ToLower(name)]; ok {
			continue
		}
		for _, value := range values {
			dst.Add(name, value)
		}
	}
}
