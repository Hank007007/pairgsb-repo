// Command certwhirld demonstrates the certificate lifecycle: it loads a pair
// from disk, publishes a tls.Config, and serves TLS on the requested address
// for as long as it runs.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"certwhirl/internal/loader"
	"certwhirl/internal/rotator"
)

func main() {
	var (
		certFile = flag.String("cert", "", "path to the PEM certificate chain")
		keyFile  = flag.String("key", "", "path to the PEM private key")
		caFile   = flag.String("ca", "", "path to the PEM trust anchor")
		addr     = flag.String("addr", "127.0.0.1:8443", "listen address")
		identity = flag.String("identity", "", "expected DNS name of the certificate")
		lead     = flag.Duration("rotate-before", rotator.DefaultRotateBefore, "how long before expiry to rotate")
		interval = flag.Duration("check-interval", rotator.DefaultCheckInterval, "delay between rotation checks")
	)
	flag.Parse()

	if *certFile == "" || *keyFile == "" {
		log.Fatal("-cert and -key are required")
	}

	var roots *x509.CertPool
	if *caFile != "" {
		roots = x509.NewCertPool()
		data, err := os.ReadFile(*caFile)
		if err != nil {
			log.Fatalf("read trust anchor: %v", err)
		}
		if !roots.AppendCertsFromPEM(data) {
			log.Fatalf("trust anchor %s contained no certificates", *caFile)
		}
	}

	var identities []string
	if *identity != "" {
		identities = []string{*identity}
	}

	load := func(ctx context.Context) (*loader.Pair, error) {
		return loader.LoadPair(loader.Config{
			CertFile:           *certFile,
			KeyFile:            *keyFile,
			RootCAs:            roots,
			ExpectedIdentities: identities,
		})
	}

	r, err := rotator.New(rotator.Config{
		Load:          load,
		RotateBefore:  *lead,
		CheckInterval: *interval,
		OnError: func(err error) {
			log.Printf("rotation: %v", err)
		},
	})
	if err != nil {
		log.Fatalf("start rotator: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	r.Start(ctx)

	// Resolve the certificate for every new connection so that a replaced
	// pair takes effect without restarting the listener.
	base := &tls.Config{
		MinVersion: tls.VersionTLS12,
		GetConfigForClient: func(*tls.ClientHelloInfo) (*tls.Config, error) {
			return r.TLSConfig(), nil
		},
	}
	ln, err := tls.Listen("tcp", *addr, base)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	log.Printf("listening on %s", ln.Addr())

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			log.Printf("accept: %v", err)
			continue
		}
		go handle(conn)
	}
	r.Stop()
	log.Print("stopped")
}

func handle(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if tlsConn, ok := conn.(*tls.Conn); ok {
		if err := tlsConn.Handshake(); err != nil {
			log.Printf("handshake from %s: %v", conn.RemoteAddr(), err)
			return
		}
	}
	_, _ = fmt.Fprintf(conn, "certwhirl ok\n")
}
