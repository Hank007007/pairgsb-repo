package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"verbgate/internal/cache"
	"verbgate/internal/store"
	"verbgate/internal/subject"
	"verbgate/internal/webhook"
)

func main() {
	addr := flag.String("addr", envOr("VERBGATE_ADDR", ":8443"), "address to listen on")
	groupsPath := flag.String("groups", envOr("VERBGATE_GROUPS", "groups.yaml"), "path to groups.yaml")
	snapshotPath := flag.String("snapshot", envOr("VERBGATE_SNAPSHOT", "snapshot.json"), "path to the role snapshot JSON")
	cacheSize := flag.Int("cache-size", 1024, "number of decisions to keep in memory")
	flag.Parse()

	groups, err := subject.LoadDirectory(*groupsPath)
	if err != nil {
		log.Fatalf("load groups: %v", err)
	}

	roles := store.New()
	snapshot, err := store.LoadSnapshot(*snapshotPath)
	if err != nil {
		log.Printf("snapshot unavailable, starting empty: %v", err)
	} else {
		roles.Replace(*snapshot)
		log.Printf("loaded snapshot %s: %d roles, %d cluster roles, %d bindings",
			snapshot.Version, len(snapshot.Roles), len(snapshot.ClusterRoles),
			len(snapshot.RoleBindings)+len(snapshot.ClusterBindings))
	}

	authorizer := webhook.New(roles, groups, cache.New(*cacheSize))

	server := &http.Server{
		Addr:              *addr,
		Handler:           webhook.Handler(authorizer),
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("verbgate listening on %s", *addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
