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

// matchPrefix reports whether rt may serve r: the request path must be covered
// by the route prefix and must not be covered by one of the route's path
// exclusions.
func matchPrefix(r *http.Request, rt *config.Route) bool {
	path := r.URL.Path
	if !strings.HasPrefix(path, rt.Prefix) {
		return false
	}
	for _, excluded := range rt.DenyPaths {
		if strings.HasPrefix(path, excluded) {
			return false
		}
	}
	return true
}

// copyResponseHeaders copies upstream response headers to the client.
func copyResponseHeaders(dst, src http.Header) {
	for name, values := range src {
		skip := false
		for _, hop := range hopHeaders {
			if strings.EqualFold(name, hop) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		for _, value := range values {
			dst.Add(name, value)
		}
	}
}
