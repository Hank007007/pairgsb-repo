// Command signward runs the build-machine signing agent.
//
// It loads the key store, opens the audit trail and serves signing requests
// over a small line-oriented protocol on stdin/stdout. See
// docs/agent-policy.md for the rules the agent applies.
package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"signward/internal/agent"
	"signward/internal/audit"
	"signward/internal/keystore"
	"signward/internal/policy"
)

// exportEnv names the environment variable that used to request a key dump.
const exportEnv = "SIGNWARD_EXPORT_KEYS"

type config struct {
	keyDir    string
	auditPath string
}

func main() {
	cfg := parseFlags()
	if err := run(cfg, os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "signward:", err)
		os.Exit(1)
	}
}

func parseFlags() config {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	base := filepath.Join(home, ".signward")

	var cfg config
	flag.StringVar(&cfg.keyDir, "key-dir", filepath.Join(base, "keys"), "directory holding key files")
	flag.StringVar(&cfg.auditPath, "audit-log", filepath.Join(base, "audit.log"), "audit trail path")
	flag.Parse()
	return cfg
}

func run(cfg config, in io.Reader, out io.Writer) error {
	log, err := audit.Open(cfg.auditPath)
	if err != nil {
		return fmt.Errorf("open audit log: %w", err)
	}
	defer log.Close()

	store, err := keystore.New(cfg.keyDir, log)
	if err != nil {
		return fmt.Errorf("open key store: %w", err)
	}
	a := agent.New(store, log)

	if os.Getenv(exportEnv) != "" {
		store.RefuseExport()
	}

	return serve(a, store, in, out)
}

// serve reads one JSON request per line and writes one JSON reply per line.
func serve(a *agent.Agent, store *keystore.Store, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	enc := json.NewEncoder(out)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		req, err := decodeRequest(line)
		if err != nil {
			if err := enc.Encode(reply{Error: err.Error()}); err != nil {
				return err
			}
			continue
		}
		if err := enc.Encode(dispatch(a, store, req)); err != nil {
			return err
		}
	}
	return scanner.Err()
}

type request struct {
	Op        string `json:"op"`
	KeyHandle string `json:"keyHandle,omitempty"`
	ID        string `json:"id,omitempty"`
	Purpose   string `json:"purpose,omitempty"`
	Payload   string `json:"payload,omitempty"`
	Policy    *struct {
		Purposes             []string `json:"purposes"`
		MaxSignaturesPerHour *int     `json:"maxSignaturesPerHour,omitempty"`
		MaxPayloadBytes      *int     `json:"maxPayloadBytes,omitempty"`
		NotAfter             string   `json:"notAfter,omitempty"`
	} `json:"policy,omitempty"`
}

type reply struct {
	OK        bool   `json:"ok"`
	KeyHandle string `json:"keyHandle,omitempty"`
	PublicKey string `json:"publicKey,omitempty"`
	Signature string `json:"signature,omitempty"`
	Error     string `json:"error,omitempty"`
}

func decodeRequest(line string) (request, error) {
	var req request
	if err := json.Unmarshal([]byte(line), &req); err != nil {
		return request{}, fmt.Errorf("malformed request: %w", err)
	}
	return req, nil
}

func dispatch(a *agent.Agent, store *keystore.Store, req request) reply {
	switch req.Op {
	case "unlock":
		if err := a.Unlock(); err != nil {
			return reply{Error: err.Error()}
		}
		return reply{OK: true}

	case "lock":
		if err := a.Lock(); err != nil {
			return reply{Error: err.Error()}
		}
		return reply{OK: true}

	case "sign":
		payload, err := decodePayload(req.Payload)
		if err != nil {
			return reply{Error: err.Error()}
		}
		resp, err := a.Sign(agent.Request{
			KeyHandle: req.KeyHandle,
			Purpose:   req.Purpose,
			Payload:   payload,
		})
		if err != nil {
			return reply{Error: err.Error()}
		}
		return reply{
			OK:        true,
			KeyHandle: resp.KeyHandle,
			Signature: base64.StdEncoding.EncodeToString(resp.Signature),
		}

	case "create":
		p := policy.Policy{}
		if req.Policy != nil {
			p.Purposes = req.Policy.Purposes
			if req.Policy.MaxSignaturesPerHour != nil {
				p = p.WithMaxSignaturesPerHour(*req.Policy.MaxSignaturesPerHour)
			}
			if req.Policy.MaxPayloadBytes != nil {
				p = p.WithMaxPayloadBytes(*req.Policy.MaxPayloadBytes)
			}
			if req.Policy.NotAfter != "" {
				t, err := parseTime(req.Policy.NotAfter)
				if err != nil {
					return reply{Error: err.Error()}
				}
				p.NotAfter = t
			}
		}
		handle, err := store.Generate(req.ID, p)
		if err != nil {
			return reply{Error: err.Error()}
		}
		publicKey, err := store.PublicKey(handle)
		if err != nil {
			return reply{Error: err.Error()}
		}
		return reply{
			OK:        true,
			KeyHandle: handle,
			PublicKey: base64.StdEncoding.EncodeToString(publicKey),
		}

	case "destroy":
		if err := store.Destroy(req.KeyHandle); err != nil {
			return reply{Error: err.Error()}
		}
		return reply{OK: true}

	case "export":
		store.RefuseExport()
		return reply{Error: "key export is not permitted"}

	default:
		return reply{Error: fmt.Sprintf("unknown op %q", req.Op)}
	}
}

func decodePayload(s string) ([]byte, error) {
	if s == "" {
		return nil, nil
	}
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("payload is not valid base64: %w", err)
	}
	return b, nil
}

func parseTime(s string) (t time.Time, err error) {
	t, err = time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("notAfter must be RFC3339: %w", err)
	}
	return t, nil
}
