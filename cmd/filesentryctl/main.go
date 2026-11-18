// Command filesentryctl sends a single request to a running broker.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"filesentry/internal/broker"
	"filesentry/internal/wire"
)

func main() {
	var (
		socket  = flag.String("socket", "/run/filesentry/broker.sock", "path to the broker socket")
		op      = flag.String("op", wire.OpRead, "operation: read, write, list, or chown")
		path    = flag.String("path", "", "target path")
		payload = flag.String("data", "", "contents for a write request")
		uid     = flag.Int("uid", -1, "owner for a chown request")
		gid     = flag.Int("gid", -1, "group for a chown request")
		mode    = flag.Uint("mode", 0, "mode bits for a write request")
		asJSON  = flag.Bool("json", false, "print the raw response as JSON")
	)
	flag.Parse()

	if *path == "" {
		fmt.Fprintln(os.Stderr, "filesentryctl: -path is required")
		os.Exit(2)
	}

	req := &wire.Request{
		Version: wire.Version,
		Op:      *op,
		Path:    *path,
		Data:    []byte(*payload),
		Mode:    uint32(*mode),
	}
	if *op == wire.OpChown {
		req.UID = *uid
		req.GID = *gid
	}

	resp, err := send(*socket, req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "filesentryctl: %v\n", err)
		os.Exit(1)
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(resp); err != nil {
			fmt.Fprintf(os.Stderr, "filesentryctl: %v\n", err)
			os.Exit(1)
		}
	} else {
		printResponse(resp)
	}

	if !resp.OK {
		os.Exit(1)
	}
}

func send(socketPath string, req *wire.Request) (*wire.Response, error) {
	conn, err := broker.Dial(socketPath)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	if err := wire.EncodeRequest(conn, req); err != nil {
		return nil, err
	}

	resp, err := wire.DecodeResponse(bufio.NewReader(conn))
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func printResponse(resp *wire.Response) {
	if !resp.OK {
		fmt.Printf("refused: %s\n", resp.Reason)
		if resp.Error != "" {
			fmt.Printf("detail: %s\n", resp.Error)
		}
		return
	}

	if len(resp.Entries) > 0 {
		for _, entry := range resp.Entries {
			kind := "file"
			if entry.IsDir {
				kind = "dir "
			}
			fmt.Printf("%s %8d %04o %d:%d %s\n", kind, entry.Size, entry.Mode, entry.UID, entry.GID, entry.Name)
		}
		return
	}

	if len(resp.Data) > 0 {
		os.Stdout.Write(resp.Data)
		if resp.Data[len(resp.Data)-1] != '\n' {
			fmt.Println()
		}
		return
	}

	fmt.Println("ok")
}
