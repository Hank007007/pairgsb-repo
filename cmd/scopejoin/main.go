// Command scopejoin computes the effective authorization for a request that
// carries several credentials. It reads a JSON request from stdin or from the
// file named on the command line and writes the effective grant as JSON.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"scopejoin/internal/grant"
)

type requestFile struct {
	Now   time.Time `json:"now"`
	Creds []struct {
		Kind      grant.Kind      `json:"kind"`
		Payload   json.RawMessage `json:"payload"`
		Anonymous bool            `json:"anonymous"`
		Subject   string          `json:"subject"`
	} `json:"credentials"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "scopejoin:", err)
		os.Exit(1)
	}
}

func run() error {
	flag.Parse()

	in := os.Stdin
	if flag.NArg() > 0 {
		f, err := os.Open(flag.Arg(0))
		if err != nil {
			return err
		}
		defer f.Close()
		in = f
	}

	var req requestFile
	if err := json.NewDecoder(in).Decode(&req); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}

	creds := make([]grant.Credential, 0, len(req.Creds))
	for i, entry := range req.Creds {
		if entry.Anonymous {
			creds = append(creds, grant.Anonymous(entry.Subject))
			continue
		}
		cred, err := grant.Parse(entry.Kind, entry.Payload)
		if err != nil {
			return fmt.Errorf("credential %d: %w", i, err)
		}
		creds = append(creds, cred)
	}

	result := grant.Compute(grant.Request{Now: req.Now, Creds: creds})
	out := struct {
		Subject string       `json:"subject"`
		Scopes  []string     `json:"scopes"`
		Roles   []grant.Role `json:"roles"`
	}{result.Subject, result.Scopes, result.Roles}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
