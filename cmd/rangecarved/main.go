// Command rangecarved runs the download edge in front of an object store.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"rangecarve/internal/edge"
	"rangecarve/internal/objcache"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	upstreamBase := flag.String("upstream", "http://127.0.0.1:9000", "upstream object store base URL")
	cacheEntries := flag.Int("cache-entries", 256, "maximum number of cached objects")
	cacheTTL := flag.Duration("cache-ttl", 30*time.Second, "how long a cached object stays fresh")
	flag.Parse()

	cache := objcache.New(*cacheEntries, *cacheTTL)
	upstream := &httpUpstream{base: strings.TrimRight(*upstreamBase, "/")}
	server := edge.New(upstream, edge.Config{Cache: cache})

	log.Printf("rangecarved listening on %s, upstream %s", *addr, *upstreamBase)
	if err := http.ListenAndServe(*addr, server); err != nil {
		log.Printf("listen: %v", err)
		os.Exit(1)
	}
}

type httpUpstream struct {
	base string
}

func (u *httpUpstream) Get(ctx context.Context, tenant, objectPath string, hdr http.Header) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.base+objectPath, nil)
	if err != nil {
		return nil, err
	}
	for key, values := range hdr {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
	req.Header.Set("X-Rangecarve-Tenant", tenant)
	return http.DefaultClient.Do(req)
}
