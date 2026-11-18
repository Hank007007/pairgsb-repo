// Command call-peer drives a small RPC call against echo-peer.
package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"peerloom/internal/wire"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8443", "server address")
	certPath := flag.String("cert", "client-0.pem", "client certificate PEM")
	keyPath := flag.String("key", "client-0-key.pem", "client private key PEM")
	caPath := flag.String("ca", "ca.pem", "server CA PEM")
	serverName := flag.String("server-name", "localhost", "server name for certificate verification")
	method := flag.String("method", "/ledger.v1.Ledger/Debit", "fully qualified method")
	body := flag.String("body", "ping", "request body")
	stream := flag.Bool("stream", false, "use the streaming dispatch path")
	count := flag.Int("count", 1, "number of messages for streaming calls")
	verbose := flag.Bool("v", false, "verbose output")
	flag.Parse()

	if err := run(*addr, *certPath, *keyPath, *caPath, *serverName, *method, *body, *stream, *count, *verbose); err != nil {
		fmt.Fprintln(os.Stderr, "call-peer:", err)
		os.Exit(1)
	}
}

func run(addr, certPath, keyPath, caPath, serverName, method, body string, stream bool, count int, verbose bool) error {
	method = normalizeMethod(method)
	if method == "" {
		return fmt.Errorf("empty method name")
	}

	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return err
	}

	pool := x509.NewCertPool()
	raw, err := os.ReadFile(caPath)
	if err != nil {
		return err
	}
	if !pool.AppendCertsFromPEM(raw) {
		return fmt.Errorf("no certificates in %s", caPath)
	}

	cfg := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		RootCAs:      pool,
		Certificates: []tls.Certificate{cert},
		ServerName:   serverName,
	}

	conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", addr, cfg)
	if err != nil {
		return err
	}
	defer conn.Close()

	if verbose {
		cs := conn.ConnectionState()
		names := make([]string, 0, len(cs.PeerCertificates))
		for _, c := range cs.PeerCertificates {
			names = append(names, c.Subject.CommonName)
		}
		fmt.Printf("connected to %s, peer chain: %s\n", addr, strings.Join(names, " <- "))
	}

	if err := wire.WriteFrame(conn, wire.Message{Method: method, Body: body, Stream: stream}); err != nil {
		return err
	}

	if !stream {
		resp, err := wire.ReadFrame(conn)
		if err != nil {
			return err
		}
		printResponse(resp)
		if resp.Status != "ok" {
			return fmt.Errorf("call failed: %s", resp.Status)
		}
		return nil
	}

	for i := 0; i < count; i++ {
		if err := wire.WriteFrame(conn, wire.Message{Body: fmt.Sprintf("%s#%d", body, i+1)}); err != nil {
			return err
		}
	}
	if err := wire.WriteFrame(conn, wire.Message{End: true}); err != nil {
		return err
	}

	for {
		resp, err := wire.ReadFrame(conn)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		printResponse(resp)
		if resp.End {
			if resp.Status != "ok" {
				return fmt.Errorf("call failed: %s", resp.Status)
			}
			return nil
		}
	}
}

// normalizeMethod turns the value of -method into a canonical "/service/method"
// path. Some shells rewrite a leading slash on arguments into a filesystem
// path; when that happens the tail of the rewritten value still carries the
// original method, so it is recovered from there.
func normalizeMethod(method string) string {
	method = strings.TrimSpace(method)
	if method == "" {
		return ""
	}
	for strings.HasPrefix(method, "//") {
		method = strings.TrimPrefix(method, "/")
	}

	if c := canonical(method); c != "" {
		return c
	}
	for i := 1; i < len(method); i++ {
		if method[i] != '/' {
			continue
		}
		if c := canonical(method[i:]); c != "" {
			return c
		}
	}
	return method
}

// canonical reports the method path when s already has the "/service/method"
// shape with a dotted service name.
func canonical(s string) string {
	if !strings.HasPrefix(s, "/") {
		return ""
	}
	parts := strings.Split(strings.TrimPrefix(s, "/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return ""
	}
	if !strings.Contains(parts[0], ".") {
		return ""
	}
	return s
}

func printResponse(m wire.Message) {
	switch {
	case m.Error != "":
		fmt.Printf("%s: %s\n", m.Status, m.Error)
	case m.Body != "":
		fmt.Printf("%s: %s\n", m.Status, m.Body)
	default:
		fmt.Println(m.Status)
	}
}
