// Command kekringd serves the envelope-encryption HTTP interface.
package main

import (
	"flag"
	"log"
	"net/http"
	"time"

	"kekring/internal/httpapi"
	"kekring/internal/keyring"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "listen address")
	keyringPath := flag.String("keyring", "keyring.json", "path to the keyring file")
	flag.Parse()

	kr, err := keyring.Load(*keyringPath)
	if err != nil {
		log.Fatalf("load keyring: %v", err)
	}

	srv := &http.Server{
		Addr:              *addr,
		Handler:           httpapi.New(kr).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("kekringd listening on %s (keyring %s)", *addr, *keyringPath)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
