package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"scopewall/internal/cache"
	"scopewall/internal/middleware"
)

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	policyPath := flag.String("policy", "policies/demo.json", "path to the policy document")
	ttl := flag.Duration("cache-ttl", 30*time.Second, "decision cache TTL")
	cacheSize := flag.Int("cache-size", 4096, "maximum number of cached decisions")
	refresh := flag.Duration("policy-refresh", 10*time.Second, "policy reload interval")
	flag.Parse()

	source := middleware.NewFileSource(*policyPath, *refresh)
	if _, err := source.Policy(); err != nil {
		log.Printf("policy not available at startup: %v", err)
	}

	authorizer := &middleware.Authorizer{
		Next:   http.HandlerFunc(backend),
		Source: source,
		Cache:  cache.New(*cacheSize, *ttl),
	}

	log.Printf("scopewall listening on %s, policy %s", *addr, *policyPath)
	if err := http.ListenAndServe(*addr, authorizer); err != nil {
		log.Fatal(err)
	}
}

// backend stands in for the service behind the decision point.
func backend(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"upstream":"ok"}`))
}
