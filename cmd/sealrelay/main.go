// Command sealrelay is a small driver for the seal relay protocol.
//
//	sealrelay keygen -out DIR            generate a signing key and a node identity
//	sealrelay seal   -request FILE ...   seal a bundle for one node
//	sealrelay accept -node DIR -in FILE  accept an envelope on an edge node
package main

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"sealrelay/internal/agent"
	"sealrelay/internal/control"
	"sealrelay/internal/envelope"
	"sealrelay/internal/manifest"
	"sealrelay/internal/nodekeys"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "keygen":
		err = runKeygen(os.Args[2:])
	case "seal":
		err = runSeal(os.Args[2:])
	case "accept":
		err = runAccept(os.Args[2:])
	case "verify":
		err = runVerify(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "sealrelay:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `usage:
  sealrelay keygen -out DIR
  sealrelay seal   -signing DIR/signing.key -request request.json -out envelope.json
  sealrelay accept -node DIR -in envelope.json
  sealrelay verify -pub DIR/signing.pub -in envelope.json
`)
}

// NodeFile is the on-disk edge identity.
type NodeFile struct {
	ID      string `json:"id"`
	Public  string `json:"public"`
	Private string `json:"private"`
}

// RequestFile is the on-disk sealing request.
type RequestFile struct {
	Bundle      string            `json:"bundle"`
	Revision    uint64            `json:"revision"`
	ExpiresAt   int64             `json:"expiresAt"`
	RecipientID string            `json:"recipient"`
	NodePublic  string            `json:"nodePublic"`
	Secrets     map[string]string `json:"secrets"`
}

func runKeygen(args []string) error {
	fs := flag.NewFlagSet("keygen", flag.ContinueOnError)
	out := fs.String("out", ".", "directory to write keys into")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := os.MkdirAll(*out, 0o700); err != nil {
		return err
	}
	signPub, signPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	node, err := nodekeys.Generate()
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(*out, "signing.key"), []byte(hex.EncodeToString(signPriv)+"\n"), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(*out, "signing.pub"), []byte(hex.EncodeToString(signPub)+"\n"), 0o644); err != nil {
		return err
	}
	nf := NodeFile{
		ID:      node.ID,
		Public:  hex.EncodeToString(node.Public),
		Private: hex.EncodeToString(node.Private),
	}
	data, err := json.MarshalIndent(nf, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(*out, "node.json"), append(data, '\n'), 0o600); err != nil {
		return err
	}
	fmt.Println(node.ID)
	return nil
}

func runSeal(args []string) error {
	fs := flag.NewFlagSet("seal", flag.ContinueOnError)
	signing := fs.String("signing", "", "path to signing.key")
	reqPath := fs.String("request", "", "path to request.json")
	out := fs.String("out", "envelope.json", "path to write the envelope")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *signing == "" || *reqPath == "" {
		return errors.New("-signing and -request are required")
	}

	rawKey, err := os.ReadFile(*signing)
	if err != nil {
		return err
	}
	keyBytes, err := hex.DecodeString(strings.TrimSpace(string(rawKey)))
	if err != nil || len(keyBytes) != ed25519.PrivateKeySize {
		return errors.New("malformed signing key")
	}
	sealer, err := control.NewSealer(ed25519.PrivateKey(keyBytes))
	if err != nil {
		return err
	}

	rawReq, err := os.ReadFile(*reqPath)
	if err != nil {
		return err
	}
	var req RequestFile
	if err := json.Unmarshal(rawReq, &req); err != nil {
		return fmt.Errorf("parse request: %w", err)
	}
	pub, err := hex.DecodeString(strings.TrimSpace(req.NodePublic))
	if err != nil {
		return fmt.Errorf("parse node public key: %w", err)
	}
	if _, err := ecdh.X25519().NewPublicKey(pub); err != nil {
		return fmt.Errorf("node public key: %w", err)
	}
	if req.RecipientID != nodekeys.ID(pub) {
		return errors.New("request recipient does not match node public key")
	}

	names := make([]string, 0, len(req.Secrets))
	for name := range req.Secrets {
		names = append(names, name)
	}
	secrets := make([]control.Secret, 0, len(names))
	for _, name := range names {
		secrets = append(secrets, control.Secret{Name: name, Value: []byte(req.Secrets[name])})
	}

	env, err := sealer.Seal(control.Request{
		Bundle:       req.Bundle,
		Revision:     req.Revision,
		ExpiresAt:    req.ExpiresAt,
		RecipientID:  req.RecipientID,
		RecipientPub: pub,
		Secrets:      secrets,
	})
	if err != nil {
		return err
	}
	data, err := env.Marshal()
	if err != nil {
		return err
	}
	if err := os.WriteFile(*out, data, 0o600); err != nil {
		return err
	}
	fmt.Printf("%s nonce=%s\n", *out, hex.EncodeToString(env.Nonce))
	return nil
}

func runAccept(args []string) error {
	fs := flag.NewFlagSet("accept", flag.ContinueOnError)
	nodeDir := fs.String("node", "", "edge node directory")
	in := fs.String("in", "", "envelope to accept")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *nodeDir == "" || *in == "" {
		return errors.New("-node and -in are required")
	}

	rawNode, err := os.ReadFile(filepath.Join(*nodeDir, "node.json"))
	if err != nil {
		return err
	}
	var nf NodeFile
	if err := json.Unmarshal(rawNode, &nf); err != nil {
		return fmt.Errorf("parse node: %w", err)
	}
	priv, err := hex.DecodeString(strings.TrimSpace(nf.Private))
	if err != nil {
		return fmt.Errorf("parse node private key: %w", err)
	}
	rawPub, err := os.ReadFile(filepath.Join(*nodeDir, "signing.pub"))
	if err != nil {
		return err
	}
	signPub, err := hex.DecodeString(strings.TrimSpace(string(rawPub)))
	if err != nil || len(signPub) != ed25519.PublicKeySize {
		return errors.New("malformed signing.pub")
	}

	receiver, err := agent.NewReceiver(*nodeDir, nf.ID, priv, ed25519.PublicKey(signPub))
	if err != nil {
		return err
	}
	rawEnv, err := os.ReadFile(*in)
	if err != nil {
		return err
	}
	env, err := envelope.Parse(rawEnv)
	if err != nil {
		return err
	}
	st, err := receiver.Accept(env)
	if err != nil {
		return err
	}
	fmt.Printf("applied %s revision %d (%d names)\n", st.Bundle, st.Revision, len(st.Names))
	return nil
}

func runVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	pubPath := fs.String("pub", "", "path to signing.pub")
	in := fs.String("in", "", "envelope to verify")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *pubPath == "" || *in == "" {
		return errors.New("-pub and -in are required")
	}
	rawPub, err := os.ReadFile(*pubPath)
	if err != nil {
		return err
	}
	pub, err := hex.DecodeString(strings.TrimSpace(string(rawPub)))
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return errors.New("malformed signing.pub")
	}
	rawEnv, err := os.ReadFile(*in)
	if err != nil {
		return err
	}
	env, err := envelope.Parse(rawEnv)
	if err != nil {
		return err
	}
	if err := manifest.VerifyManifest(&env.Manifest, ed25519.PublicKey(pub)); err != nil {
		return err
	}
	fmt.Printf("ok %s revision %d recipient %s expiresAt %d\n",
		env.Manifest.Payload.Bundle,
		env.Manifest.Payload.Revision,
		env.Manifest.Recipient,
		env.Manifest.ExpiresAt,
	)
	return nil
}
