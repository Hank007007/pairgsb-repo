// Command snisieved is the egress gate. It accepts connections, classifies the
// client's first flight with internal/proxy, and either dials the upstream or
// closes the connection.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"

	"snisieve/internal/policy"
	"snisieve/internal/proxy"
)

const (
	exitOK      = 0
	exitRuntime = 1
	exitUsage   = 2

	upstreamDialTimeout = 10 * time.Second
	idleTimeout         = 30 * time.Second
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	fs := flag.NewFlagSet("snisieved", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	listen := fs.String("listen", "127.0.0.1:8443", "address to accept client connections on")
	upstream := fs.String("upstream", "127.0.0.1:443", "address to forward allowed connections to")
	rulesPath := fs.String("rules", "testdata/allow.txt", "path to the allow-list")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() != 0 {
		fmt.Fprintf(os.Stderr, "snisieved: unexpected argument %q\n", fs.Arg(0))
		return exitUsage
	}

	f, err := os.Open(*rulesPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "snisieved: cannot open rules: %v\n", err)
		return exitUsage
	}
	rules, err := policy.LoadRules(f)
	f.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "snisieved: cannot load rules: %v\n", err)
		return exitUsage
	}

	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		fmt.Fprintf(os.Stderr, "snisieved: cannot listen: %v\n", err)
		return exitRuntime
	}
	defer ln.Close()
	log.Printf("snisieved: listening on %s, upstream %s, %d rules", ln.Addr(), *upstream, len(rules))

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			log.Printf("snisieved: accept failed: %v", err)
			return exitRuntime
		}
		go handle(conn, *upstream, rules)
	}
}

func handle(client net.Conn, upstream string, rules []policy.Rule) {
	defer client.Close()
	client.SetDeadline(time.Now().Add(idleTimeout))

	first := &recorder{r: client}
	decision, err := proxy.Peek(first, rules, log.Default())
	if err != nil {
		log.Printf("snisieve: classify failed: %v", err)
	}
	if !decision.Allowed {
		log.Printf("snisieve: deny (%s)", decision.Reason)
		return
	}

	server, err := net.DialTimeout("tcp", upstream, upstreamDialTimeout)
	if err != nil {
		log.Printf("snisieve: cannot reach upstream: %v", err)
		return
	}
	defer server.Close()
	server.SetDeadline(time.Now().Add(idleTimeout))
	client.SetDeadline(time.Now().Add(idleTimeout))

	// The first flight was consumed while classifying; hand it to the upstream
	// before switching to a plain byte pump.
	if _, err := server.Write(first.buf); err != nil {
		log.Printf("snisieve: cannot forward first flight: %v", err)
		return
	}

	pipe(client, server)
}

// recorder keeps a copy of everything read through it.
type recorder struct {
	r   io.Reader
	buf []byte
}

func (rec *recorder) Read(p []byte) (int, error) {
	n, err := rec.r.Read(p)
	if n > 0 {
		rec.buf = append(rec.buf, p[:n]...)
	}
	return n, err
}

// pipe forwards bytes in both directions until either side finishes.
func pipe(client, server net.Conn) {
	done := make(chan struct{}, 2)
	go func() {
		copyAndClose(server, client)
		done <- struct{}{}
	}()
	go func() {
		copyAndClose(client, server)
		done <- struct{}{}
	}()
	<-done
}

func copyAndClose(dst, src net.Conn) {
	buf := make([]byte, 32<<10)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				break
			}
		}
		if err != nil {
			break
		}
	}
	if cw, ok := dst.(interface{ CloseWrite() error }); ok {
		cw.CloseWrite()
	} else {
		dst.Close()
	}
}
