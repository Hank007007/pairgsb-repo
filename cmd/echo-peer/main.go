// Command echo-peer runs a small mutually authenticated RPC server.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"peerloom/internal/bundle"
	"peerloom/internal/policy"
	"peerloom/internal/rpc"
	"peerloom/internal/server"
	"peerloom/internal/wire"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:8443", "address to listen on")
	certPath := flag.String("cert", "server.pem", "server certificate PEM")
	keyPath := flag.String("key", "server-key.pem", "server private key PEM")
	caPath := flag.String("ca", "ca.pem", "client CA bundle PEM")
	bundleDir := flag.String("bundle", "internal/bundle", "trust bundle directory")
	policyDir := flag.String("policies", "policies", "policy directory")
	verbose := flag.Bool("v", false, "log handshakes and calls")
	flag.Parse()

	if err := run(*listen, *certPath, *keyPath, *caPath, *bundleDir, *policyDir, *verbose); err != nil {
		fmt.Fprintln(os.Stderr, "echo-peer:", err)
		os.Exit(1)
	}
}

func run(listen, certPath, keyPath, caPath, bundleDir, policyDir string, verbose bool) error {
	trust, err := policy.LoadTrust(policyDir + "/trust.json")
	if err != nil {
		return err
	}
	set, err := policy.Load(policyDir)
	if err != nil {
		return err
	}

	pool, err := loadClientCAs(caPath, bundleDir)
	if err != nil {
		return err
	}

	srvCert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return err
	}

	tlsCfg, err := server.NewServerTLSConfig(server.Config{
		Certificate: srvCert,
		ClientCAs:   pool,
	})
	if err != nil {
		return err
	}

	d := rpc.NewDispatcher(set)
	if err := d.Register(echoService()); err != nil {
		return err
	}

	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return err
	}
	log.Printf("echo-peer listening on %s (trust domain %s, %d rules)", ln.Addr(), trust.TrustDomain, set.Len())

	for {
		raw, err := ln.Accept()
		if err != nil {
			return err
		}
		go serveConn(raw, tlsCfg, d, trust.TrustDomain, verbose)
	}
}

func serveConn(raw net.Conn, cfg *tls.Config, d *rpc.Dispatcher, trustDomain string, verbose bool) {
	defer raw.Close()
	conn := tls.Server(raw, cfg)
	if err := conn.Handshake(); err != nil {
		if verbose {
			log.Printf("handshake from %s failed: %v", raw.RemoteAddr(), err)
		}
		return
	}
	defer conn.Close()

	id, err := rpc.PeerFromConn(conn, trustDomain)
	if err != nil {
		if verbose {
			log.Printf("peer from %s has no usable identity: %v", raw.RemoteAddr(), err)
		}
		_ = wire.WriteFrame(conn, wire.Message{Status: rpc.StatusUnauthenticated.String(), End: true})
		return
	}
	if verbose {
		log.Printf("peer %s connected from %s", id, raw.RemoteAddr())
	}

	first, err := wire.ReadFrame(conn)
	if err != nil {
		return
	}
	if verbose {
		log.Printf("request method=%q stream=%v body=%q", first.Method, first.Stream, first.Body)
	}

	ctx := rpc.NewContext(context.Background(), id)

	if !first.Stream {
		out, err := d.InvokeUnary(ctx, first.Method, []byte(first.Body))
		if err != nil {
			_ = wire.WriteFrame(conn, wire.Message{
				Status: rpc.StatusOf(err).String(),
				Error:  err.Error(),
				End:    true,
			})
			return
		}
		_ = wire.WriteFrame(conn, wire.Message{Status: rpc.StatusOK.String(), Body: string(out), End: true})
		return
	}

	recv := func() ([]byte, error) {
		m, err := wire.ReadFrame(conn)
		if err != nil {
			return nil, err
		}
		if m.End {
			return nil, io.EOF
		}
		return []byte(m.Body), nil
	}
	send := func(b []byte) error {
		return wire.WriteFrame(conn, wire.Message{Status: rpc.StatusOK.String(), Body: string(b)})
	}

	if err := d.OpenStream(ctx, first.Method, recv, send); err != nil {
		_ = wire.WriteFrame(conn, wire.Message{
			Status: rpc.StatusOf(err).String(),
			Error:  err.Error(),
			End:    true,
		})
		return
	}
	_ = wire.WriteFrame(conn, wire.Message{Status: rpc.StatusOK.String(), End: true})
}

func loadClientCAs(caPath, bundleDir string) (*x509.CertPool, error) {
	if caPath != "" {
		raw, err := os.ReadFile(caPath)
		if err == nil {
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(raw) {
				return nil, fmt.Errorf("no certificates in %s", caPath)
			}
			return pool, nil
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	b, err := bundle.LoadDir(bundleDir)
	if err != nil {
		return nil, fmt.Errorf("no client CA file and bundle not usable: %w", err)
	}
	pool := b.Pool()
	if pool == nil || len(pool.Subjects()) == 0 { //nolint:staticcheck // Subjects is the only way to count pool members
		return nil, fmt.Errorf("bundle %s holds no anchors or intermediates", bundleDir)
	}
	return pool, nil
}

func echoService() *rpc.Service {
	svc := &rpc.Service{
		Name:      "ledger.v1.Ledger",
		Unary:     map[string]rpc.HandlerFunc{},
		Streaming: map[string]rpc.StreamHandler{},
	}
	echo := func(ctx context.Context, req []byte) ([]byte, error) {
		return req, nil
	}
	for _, m := range []string{"Debit", "Balance"} {
		svc.Unary[m] = echo
		svc.Streaming[m] = streamEcho
	}
	svc.Unary["WhoAmI"] = func(ctx context.Context, req []byte) ([]byte, error) {
		id, _ := rpc.PeerFromContext(ctx)
		return []byte(id.String()), nil
	}
	return svc
}

func streamEcho(ctx context.Context, recv func() ([]byte, error), send func([]byte) error) error {
	for {
		msg, err := recv()
		if err != nil {
			return nil
		}
		if err := send(msg); err != nil {
			return err
		}
	}
}
