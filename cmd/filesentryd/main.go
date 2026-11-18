// Command filesentryd is the privileged file broker.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"syscall"

	"filesentry/internal/acl"
	"filesentry/internal/broker"
)

func main() {
	var (
		accessPath = flag.String("access", "etc/access.json", "path to the access file")
		socketPath = flag.String("socket", "/run/filesentry/broker.sock", "path to the broker socket")
		group      = flag.String("group", "", "group that owns the broker socket")
		maxRead    = flag.Int64("max-read", broker.DefaultMaxRead, "maximum bytes returned by a read")
	)
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.LUTC)
	log.SetPrefix("filesentryd: ")

	if err := run(*accessPath, *socketPath, *group, *maxRead); err != nil {
		log.Fatal(err)
	}
}

func run(accessPath, socketPath, group string, maxRead int64) error {
	policy, err := acl.LoadFile(accessPath)
	if err != nil {
		return fmt.Errorf("load access file: %w", err)
	}

	if err := makeSocketDir(socketPath); err != nil {
		return err
	}
	if err := removeStaleSocket(socketPath); err != nil {
		return err
	}

	ln, err := broker.Listen(socketPath)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer ln.Close()

	if err := applySocketMode(socketPath, group); err != nil {
		return err
	}

	log.Printf("listening on %s, %d uid entries in the policy", socketPath, len(policy.UIDs()))

	h := &broker.Handler{Policy: policy, MaxRead: maxRead}

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-signals
		log.Printf("received %s, shutting down", sig)
		ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if isClosed(err) {
				return nil
			}
			log.Printf("accept: %v", err)
			continue
		}
		go h.Handle(conn)
	}
}

// makeSocketDir creates the directory that will hold the socket.
func makeSocketDir(socketPath string) error {
	dir := filepath.Dir(socketPath)
	if dir == "." || dir == "/" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create socket directory: %w", err)
	}
	return nil
}

// removeStaleSocket unlinks a socket left behind by a previous instance.
func removeStaleSocket(socketPath string) error {
	fi, err := os.Lstat(socketPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat socket: %w", err)
	}
	if fi.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("%s exists and is not a socket", socketPath)
	}
	if err := os.Remove(socketPath); err != nil {
		return fmt.Errorf("remove stale socket: %w", err)
	}
	return nil
}

// applySocketMode gives the socket its deployed mode and group.
func applySocketMode(socketPath, group string) error {
	if group != "" {
		g, err := user.LookupGroup(group)
		if err != nil {
			return fmt.Errorf("look up group %q: %w", group, err)
		}
		gid, err := parseGID(g.Gid)
		if err != nil {
			return fmt.Errorf("group %q: %w", group, err)
		}
		if err := os.Chown(socketPath, -1, gid); err != nil {
			return fmt.Errorf("chown socket: %w", err)
		}
	}
	if err := os.Chmod(socketPath, broker.SocketMode); err != nil {
		return fmt.Errorf("chmod socket: %w", err)
	}
	return nil
}

func parseGID(s string) (int, error) {
	gid := 0
	if _, err := fmt.Sscanf(s, "%d", &gid); err != nil {
		return 0, fmt.Errorf("parse group id %q: %w", s, err)
	}
	return gid, nil
}

func isClosed(err error) bool {
	return errors.Is(err, net.ErrClosed) || errors.Is(err, os.ErrClosed)
}
