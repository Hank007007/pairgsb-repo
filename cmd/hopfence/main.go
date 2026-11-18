// Command hopfence runs the hopfence forwarding proxy.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hopfence/internal/accesslog"
	"hopfence/internal/config"
	"hopfence/internal/proxy"
)

func main() {
	configPath := flag.String("config", "hopfence.json", "path to the configuration file")
	flag.Parse()

	if err := run(*configPath); err != nil {
		fmt.Fprintln(os.Stderr, "hopfence:", err)
		os.Exit(1)
	}
}

func run(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	logSink := os.Stdout
	if cfg.AccessLog != "" {
		file, err := os.OpenFile(cfg.AccessLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return fmt.Errorf("open access log: %w", err)
		}
		defer file.Close()
		logSink = file
	}

	handler, err := proxy.New(cfg, accesslog.New(logSink))
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:              cfg.Listen,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errc := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errc:
		return err
	case <-stop:
		log.Println("hopfence: shutting down")
		return server.Close()
	}
}
