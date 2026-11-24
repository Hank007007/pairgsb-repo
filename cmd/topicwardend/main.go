// Command topicwardend runs topicwarden as a small authorization service. It
// loads a rule document, serves decisions over HTTP, and reloads the document
// on SIGHUP or on demand through the admin endpoint.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"topicwarden/internal/audit"
	"topicwarden/internal/broker"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8088", "address to listen on")
	rulesPath := flag.String("rules", "testdata/rules.yaml", "path to the rule document")
	auditPath := flag.String("audit", "", "path to append audit records to (empty disables)")
	flag.Parse()

	data, err := os.ReadFile(*rulesPath)
	if err != nil {
		log.Fatalf("read rules: %v", err)
	}

	var sink *os.File
	if *auditPath != "" {
		f, err := os.OpenFile(*auditPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			log.Fatalf("open audit log: %v", err)
		}
		defer f.Close()
		sink = f
	}

	var logger *audit.Logger
	if sink != nil {
		logger = audit.NewLogger(sink)
	} else {
		logger = audit.NewLogger(os.Stdout)
	}

	authz := broker.NewAuthorizer(nil, logger)
	if err := authz.Reload(data); err != nil {
		log.Fatalf("load rules: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/authorize", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ClientID       string `json:"client_id"`
			PrincipalClass string `json:"principal_class"`
			Action         string `json:"action"`
			Filter         string `json:"filter"`
			Topic          string `json:"topic"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		conn := &broker.Connection{ClientID: req.ClientID, PrincipalClass: req.PrincipalClass}
		var allowed bool
		if req.Action == "pub" {
			allowed = authz.AuthorizePublish(conn, req.Topic)
		} else {
			allowed = authz.AuthorizeSubscribe(conn, req.Filter)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"allowed": allowed,
			"version": authz.Store().Version(),
		})
	})
	mux.HandleFunc("/v1/reload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		data, err := os.ReadFile(*rulesPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := authz.Reload(data); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		fmt.Fprintf(w, "reloaded version %d\n", authz.Store().Version())
	})

	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	go func() {
		for range hup {
			data, err := os.ReadFile(*rulesPath)
			if err != nil {
				log.Printf("reload: %v", err)
				continue
			}
			if err := authz.Reload(data); err != nil {
				log.Printf("reload: %v", err)
				continue
			}
			log.Printf("reloaded rules version %d", authz.Store().Version())
		}
	}()

	log.Printf("topicwarden listening on %s", *addr)
	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatal(err)
	}
}
