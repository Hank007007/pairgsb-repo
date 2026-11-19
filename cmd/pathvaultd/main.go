// Command pathvaultd runs the pathvault HTTP broker.
package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strings"

	"pathvault/internal/api"
	"pathvault/internal/audit"
	"pathvault/internal/lease"
	"pathvault/internal/policy"
	"pathvault/internal/store"
	"pathvault/internal/token"
)

type stringList []string

func (s *stringList) String() string {
	return strings.Join(*s, ",")
}

func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func main() {
	var secretPath string
	var policyPaths stringList
	var addr string
	var auditPath string
	var adminToken string

	flag.StringVar(&secretPath, "secrets", "", "JSON file with the secret table (required)")
	flag.Var(&policyPaths, "policy", "policy file; repeatable")
	flag.StringVar(&addr, "addr", "127.0.0.1:8080", "listen address")
	flag.StringVar(&auditPath, "audit", "", "audit log path; empty means stdout")
	flag.StringVar(&adminToken, "admin-token", "", "token accepted for backend revocation")
	flag.Parse()

	if secretPath == "" {
		log.Fatal("-secrets is required")
	}
	if len(policyPaths) == 0 {
		log.Fatal("at least one -policy is required")
	}
	if adminToken == "" {
		log.Fatal("-admin-token is required")
	}

	secrets := store.New()
	if err := secrets.LoadFile(secretPath); err != nil {
		log.Fatal(err)
	}

	var accessors []policy.Accessor
	for _, path := range policyPaths {
		loaded, err := policy.Load(path)
		if err != nil {
			log.Fatalf("load policy %s: %v", path, err)
		}
		accessors = append(accessors, loaded...)
	}
	pol, err := policy.New(accessors)
	if err != nil {
		log.Fatal(err)
	}

	var auditLogger *audit.Logger
	if auditPath == "" {
		auditLogger = audit.NewLogger(os.Stdout)
	} else {
		auditLogger, err = audit.OpenFile(auditPath)
		if err != nil {
			log.Fatal(err)
		}
		defer auditLogger.Close()
	}

	srv := api.New(pol, secrets, lease.NewManager(), token.NewManager(), auditLogger, adminToken)
	httpServer := &http.Server{Addr: addr, Handler: srv.Handler()}

	log.Printf("pathvault listening on %s", addr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
