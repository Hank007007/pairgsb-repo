package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"podgrate/internal/admission"
)

func main() {
	listen := flag.String("listen", ":8443", "address the admission endpoint listens on")
	policyPath := flag.String("policy", "testdata/policy.yaml", "path to the policy file")
	flag.Parse()

	if *listen == "" {
		log.Print("podgrated: -listen must not be empty")
		os.Exit(2)
	}

	handler := &admission.Server{PolicyPath: *policyPath}

	mux := http.NewServeMux()
	mux.Handle("/admit", handler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok\n"))
	})

	server := &http.Server{
		Addr:              *listen,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("podgrated: listening on %s, policy %s", *listen, *policyPath)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("podgrated: %v", err)
		os.Exit(1)
	}
}
