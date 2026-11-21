package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"

	"beaconproof/internal/anchor"
	"beaconproof/internal/chain"
	"beaconproof/internal/merkle"
	"beaconproof/internal/receipt"
	"beaconproof/internal/tsa"
)

// Layout describes where a beaconproofless installation keeps its files.
type Layout struct {
	LogPath    string
	ReceiptDir string
	KeyPath    string
	PubPath    string
}

// DefaultLayout returns the layout under root.
func DefaultLayout(root string) Layout {
	return Layout{
		LogPath:    root + "/audit.log",
		ReceiptDir: root + "/receipts",
		KeyPath:    root + "/tsa.key",
		PubPath:    root + "/tsa.pub",
	}
}

// App is the command-line application.
type App struct {
	Layout  Layout
	Stdout  io.Writer
	Stderr  io.Writer
	BaseURL string
}

// NewApp builds an App writing to stdout/stderr.
func NewApp(layout Layout, stdout, stderr io.Writer) *App {
	return &App{Layout: layout, Stdout: stdout, Stderr: stderr}
}

// Run dispatches a subcommand.
func (a *App) Run(args []string) int {
	if len(args) == 0 {
		a.usage()
		return 2
	}
	switch args[0] {
	case "append":
		return a.cmdAppend(args[1:])
	case "anchor":
		return a.cmdAnchor(args[1:])
	case "verify":
		return a.cmdVerify(args[1:])
	case "authority":
		return a.cmdAuthority(args[1:])
	case "help", "-h", "--help":
		a.usage()
		return 0
	default:
		fmt.Fprintf(a.Stderr, "unknown command %q\n", args[0])
		a.usage()
		return 2
	}
}

func (a *App) usage() {
	fmt.Fprintln(a.Stderr, strings.TrimSpace(`
usage: beaconctl <command> [flags]

commands:
  append      append one audit event to the local log
  anchor      anchor pending records with the timestamp authority
  verify      replay the local chain and check every stored receipt
  authority   run a local timestamp authority
`))
}

func (a *App) openStore() (*chain.Store, error) {
	return chain.Open(a.Layout.LogPath)
}

func (a *App) loadPub() (ed25519.PublicKey, error) {
	return tsa.LoadPublicKey(a.Layout.PubPath)
}

func (a *App) receiptDir() (*receipt.Dir, error) {
	return receipt.OpenDir(a.Layout.ReceiptDir)
}

func (a *App) cmdAppend(args []string) int {
	fs := flag.NewFlagSet("append", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	actor := fs.String("actor", "", "actor responsible for the event")
	action := fs.String("action", "", "action being recorded")
	body := fs.String("body", "", "free-form event body")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *actor == "" || *action == "" {
		fmt.Fprintln(a.Stderr, "append: --actor and --action are required")
		return 2
	}

	store, err := a.openStore()
	if err != nil {
		fmt.Fprintf(a.Stderr, "append: %v\n", err)
		return 1
	}
	defer store.Close()

	rec, err := store.Append(*actor, *action, *body)
	if err != nil {
		fmt.Fprintf(a.Stderr, "append: %v\n", err)
		return 1
	}
	fmt.Fprintf(a.Stdout, "appended seq=%d at %s\n", rec.Seq, rec.Time)
	return 0
}

func (a *App) cmdAnchor(args []string) int {
	fs := flag.NewFlagSet("anchor", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	interval := fs.Int("n", 64, "records per anchored interval")
	maxAge := fs.Duration("max-age", 0, "anchor at least this often even when few records arrived")
	url := fs.String("tsa", "", "base URL of the timestamp authority")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	base := *url
	if base == "" {
		base = a.BaseURL
	}
	if base == "" {
		base = "http://127.0.0.1:8787"
	}

	store, err := a.openStore()
	if err != nil {
		fmt.Fprintf(a.Stderr, "anchor: %v\n", err)
		return 1
	}
	defer store.Close()

	dir, err := a.receiptDir()
	if err != nil {
		fmt.Fprintf(a.Stderr, "anchor: %v\n", err)
		return 1
	}
	pub, err := a.loadPub()
	if err != nil {
		fmt.Fprintf(a.Stderr, "anchor: %v\n", err)
		return 1
	}

	submitter := anchor.NewSubmitter(store, dir, tsa.NewClient(base, nil), pub, base)
	runner, err := anchor.NewRunner(anchor.Policy{Interval: *interval, MaxAge: *maxAge}, submitter, dir)
	if err != nil {
		fmt.Fprintf(a.Stderr, "anchor: %v\n", err)
		return 1
	}

	ctx := context.Background()
	dec, err := runner.Tick(ctx, store.HeadSeq())
	if err != nil {
		fmt.Fprintf(a.Stderr, "anchor: %v\n", err)
		return 1
	}
	if !dec.Anchor {
		fmt.Fprintf(a.Stdout, "nothing to anchor: %s (head=%d anchored=%d)\n", dec.Reason, store.HeadSeq(), runner.AnchoredSeq())
		return 0
	}
	fmt.Fprintf(a.Stdout, "anchored [%d, %d]: %s\n", dec.FromSeq, dec.ToSeq, dec.Reason)
	return 0
}

func (a *App) cmdVerify(args []string) int {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	store, err := a.openStore()
	if err != nil {
		fmt.Fprintf(a.Stderr, "verify: %v\n", err)
		return 1
	}
	defer store.Close()

	dir, err := a.receiptDir()
	if err != nil {
		fmt.Fprintf(a.Stderr, "verify: %v\n", err)
		return 1
	}
	receipts, err := dir.LoadAll()
	if err != nil {
		fmt.Fprintf(a.Stderr, "verify: %v\n", err)
		return 1
	}

	pub, err := a.loadPub()
	if err != nil {
		fmt.Fprintf(a.Stderr, "verify: %v\n", err)
		return 1
	}

	failed := false

	rep := chain.Verify(store.Records(), spansOf(receipts))
	if !rep.OK() {
		failed = true
		for _, p := range rep.Problems {
			fmt.Fprintf(a.Stdout, "chain: %s\n", p)
		}
	}

	ctx := receipt.CheckContext{PublicKey: pub}
	for _, f := range receipt.VerifyAll(receipts, ctx) {
		failed = true
		fmt.Fprintf(a.Stdout, "%s\n", f)
	}

	if failed {
		fmt.Fprintln(a.Stdout, "verify: FAILED")
		return 1
	}
	fmt.Fprintf(a.Stdout, "verify: OK (%d records, %d receipts)\n", store.Len(), len(receipts))
	return 0
}

func (a *App) rootFor(store *chain.Store, from, to int64) (string, error) {
	recs := store.Range(from, to)
	if len(recs) == 0 {
		return "", fmt.Errorf("no records in [%d, %d]", from, to)
	}
	leaves := make(map[int64][]byte, len(recs))
	for _, rec := range recs {
		raw, err := hex.DecodeString(rec.Hash)
		if err != nil {
			continue
		}
		leaves[rec.Seq] = raw
	}
	return hex.EncodeToString(merkle.RangeRoot(leaves, from, to)), nil
}

func (a *App) cmdAuthority(args []string) int {
	fs := flag.NewFlagSet("authority", flag.ContinueOnError)
	fs.SetOutput(a.Stderr)
	addr := fs.String("addr", "127.0.0.1:8787", "listen address")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	auth, err := tsa.LoadOrCreateAuthority(a.Layout.KeyPath)
	if err != nil {
		fmt.Fprintf(a.Stderr, "authority: %v\n", err)
		return 1
	}
	if err := auth.WritePublicKey(a.Layout.PubPath); err != nil {
		fmt.Fprintf(a.Stderr, "authority: %v\n", err)
		return 1
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		fmt.Fprintf(a.Stderr, "authority: %v\n", err)
		return 1
	}
	fmt.Fprintf(a.Stdout, "authority listening on http://%s (public key %s)\n", ln.Addr(), auth.PublicKeyHex())
	if err := http.Serve(ln, auth.Handler()); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintf(a.Stderr, "authority: %v\n", err)
		return 1
	}
	return 0
}

func spansOf(receipts []receipt.Receipt) []chain.Span {
	out := make([]chain.Span, 0, len(receipts))
	for _, r := range receipts {
		out = append(out, chain.Span{FromSeq: r.FromSeq, ToSeq: r.ToSeq})
	}
	return out
}

func main() {
	root := os.Getenv("BEACONPROOF_HOME")
	if root == "" {
		root = "."
	}
	app := NewApp(DefaultLayout(root), os.Stdout, os.Stderr)
	os.Exit(app.Run(os.Args[1:]))
}
