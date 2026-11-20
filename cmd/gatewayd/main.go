// Command gatewayd serves the local request-shape demo endpoints.
package main

import (
	"flag"
	"log"
	"net/http"
	"time"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "listen address")
	flag.Parse()

	log.Printf("gatewayd listening on http://%s", *addr)
	log.Printf("endpoints: account.create, event.ingest, profile.update")

	srv := &http.Server{
		Addr:              *addr,
		Handler:           &server{},
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
