// Command resource serves the internal resource API behind DPoP token
// validation.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"proofbind/internal/authz"
	"proofbind/internal/token"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:8080", "address to listen on")
	introspectURL := flag.String("introspect-url", "", "authorization server introspection endpoint")
	clientID := flag.String("client-id", "", "introspection client id")
	clientSecret := flag.String("client-secret", "", "introspection client secret")
	introspectTTL := flag.Duration("introspect-ttl", 10*time.Minute, "maximum introspection cache lifetime")
	replayTTL := flag.Duration("replay-ttl", 5*time.Minute, "how long accepted proofs are remembered")
	flag.Parse()

	if *introspectURL == "" {
		log.Fatal("resource: -introspect-url is required")
	}

	tokens := token.NewClient(*introspectURL, *clientID, *clientSecret)
	tokens.TTL = *introspectTTL

	auth := authz.New(tokens)
	auth.ReplayTTL = *replayTTL

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.Handle("/api/whoami", auth.Handler(http.HandlerFunc(whoami)))
	mux.Handle("/api/documents", auth.Handler(http.HandlerFunc(documents)))

	srv := &http.Server{
		Addr:              *listen,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("resource: listening on %s", *listen)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("resource: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func whoami(w http.ResponseWriter, r *http.Request) {
	principal, ok := authz.FromContext(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	writeJSON(w, map[string]string{
		"subject": principal.Subject,
		"scope":   principal.Scope,
	})
}

func documents(w http.ResponseWriter, r *http.Request) {
	principal, ok := authz.FromContext(r.Context())
	if !ok {
		http.Error(w, "unauthenticated", http.StatusUnauthorized)
		return
	}
	writeJSON(w, map[string]any{
		"owner": principal.Subject,
		"items": []string{"quarterly-plan.md", "budget.csv"},
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
