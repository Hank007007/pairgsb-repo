// Command witnessctl manages and verifies witnesschain audit stores.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"witnesschain/internal/chain"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]
	var err error
	switch cmd {
	case "init":
		err = cmdInit(args)
	case "append":
		err = cmdAppend(args)
	case "verify":
		err = cmdVerify(args)
	case "keygen":
		err = cmdKeygen(args)
	case "list":
		err = cmdList(args)
	case "prove":
		err = cmdProve(args)
	case "help", "-h", "--help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "witnessctl: unknown command %q\n", cmd)
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "witnessctl: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: witnessctl <command> [options]

commands:
  init   [options] <dir>                  create a store and its first signing key
  append [options] <dir>                  append one record
  verify [options] <dir>                  verify the whole store
  keygen [options] <dir>                  add a signing key (rotation)
  list   <dir>                            list segments from the index
  prove  -segment N -index I <dir>        print an inclusion proof as JSON

append options:
  -actor NAME -action NAME [-payload TEXT] [-outcome TEXT]
`)
}

// reorderArgs moves flags ahead of positional arguments so that the directory
// may be given before or after the options. Flags that take a value consume
// the following token unless written as -name=value.
func reorderArgs(fs *flag.FlagSet, args []string) []string {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if len(a) > 1 && a[0] == '-' {
			name := strings.TrimLeft(a, "-")
			flags = append(flags, a)
			if strings.ContainsRune(name, '=') {
				continue
			}
			isBool := false
			if f := fs.Lookup(name); f != nil {
				if b, ok := f.Value.(interface{ IsBoolFlag() bool }); ok {
					isBool = b.IsBoolFlag()
				}
			}
			if !isBool && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		pos = append(pos, a)
	}
	return append(flags, pos...)
}

func cmdInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	keyID := fs.String("key-id", "", "key identifier (default: date-stamped)")
	if err := fs.Parse(reorderArgs(fs, args)); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("init needs exactly one directory")
	}
	dir := fs.Arg(0)
	if err := os.MkdirAll(filepath.Join(dir, "segments"), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dir, "keys"), 0o700); err != nil {
		return err
	}
	id := *keyID
	if id == "" {
		id = defaultKeyID()
	}
	kp, err := chain.GenerateKeyPair(id)
	if err != nil {
		return err
	}
	if err := chain.SaveKeys(filepath.Join(dir, "keys"), kp); err != nil {
		return err
	}
	fmt.Printf("initialized %s with key %s\n", dir, id)
	return nil
}

func cmdKeygen(args []string) error {
	fs := flag.NewFlagSet("keygen", flag.ExitOnError)
	id := fs.String("id", "", "key identifier (default: date-stamped)")
	if err := fs.Parse(reorderArgs(fs, args)); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("keygen needs exactly one directory")
	}
	dir := fs.Arg(0)
	keyID := *id
	if keyID == "" {
		keyID = defaultKeyID()
	}
	kp, err := chain.GenerateKeyPair(keyID)
	if err != nil {
		return err
	}
	if err := chain.SaveKeys(filepath.Join(dir, "keys"), kp); err != nil {
		return err
	}
	fmt.Printf("generated key %s\n", keyID)
	return nil
}

func defaultKeyID() string {
	return "k" + time.Now().UTC().Format("20060102T150405")
}

func openWriter(dir string) (*chain.Writer, error) {
	keys := chain.NewKeySet(filepath.Join(dir, "keys"))
	id, err := keys.NewestKey()
	if err != nil {
		return nil, err
	}
	prv, err := chain.LoadPrivateKey(filepath.Join(dir, "keys"), id)
	if err != nil {
		return nil, err
	}
	pub, err := chain.LoadPublicKey(filepath.Join(dir, "keys"), id)
	if err != nil {
		return nil, err
	}
	kp := &chain.KeyPair{ID: id, Pub: pub, Prv: prv}
	return chain.OpenWriter(dir, chain.DefaultSegmentLimit, kp)
}

func cmdAppend(args []string) error {
	fs := flag.NewFlagSet("append", flag.ExitOnError)
	actor := fs.String("actor", "", "actor name")
	action := fs.String("action", "", "action name")
	payload := fs.String("payload", "", "payload text")
	outcome := fs.String("outcome", "", "outcome text")
	if err := fs.Parse(reorderArgs(fs, args)); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("append needs exactly one directory")
	}
	if *actor == "" || *action == "" {
		return fmt.Errorf("append requires -actor and -action")
	}
	dir := fs.Arg(0)
	w, err := openWriter(dir)
	if err != nil {
		return err
	}
	rec, err := w.Append(*actor, *action, *payload, *outcome)
	if err != nil {
		w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	fmt.Printf("appended seq %d %s\n", rec.Seq, rec.Hash)
	return nil
}

func cmdVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	jsonOut := fs.Bool("json", false, "emit findings as JSON")
	if err := fs.Parse(reorderArgs(fs, args)); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("verify needs exactly one directory")
	}
	dir := fs.Arg(0)
	res, err := chain.Verify(dir)
	if err != nil {
		return err
	}
	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
	if res.OK() {
		fmt.Printf("OK %d segments, %d records, head %s\n", res.Segments, res.Records, shortHash(res.HeadHash))
		return nil
	}
	f := res.Findings[0]
	fmt.Printf("FAIL %s\n", f.Kind)
	fmt.Printf("  segment: %d\n", f.Segment)
	if f.Seq > 0 {
		fmt.Printf("  seq: %d\n", f.Seq)
	}
	fmt.Printf("  detail: %s\n", f.Detail)
	for _, extra := range res.Findings[1:] {
		fmt.Printf("  also: %s\n", extra)
	}
	return fmt.Errorf("verification failed")
}

func cmdList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	if err := fs.Parse(reorderArgs(fs, args)); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("list needs exactly one directory")
	}
	dir := fs.Arg(0)
	entries, bad, err := chain.ReadIndex(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		fmt.Printf("segment %d file=%s count=%d last_seq=%d\n", e.Segment, e.File, e.Count, e.LastSeq)
	}
	for _, b := range bad {
		fmt.Printf("segment ? index line %d unreadable\n", b)
	}
	return nil
}

func cmdProve(args []string) error {
	fs := flag.NewFlagSet("prove", flag.ExitOnError)
	segNo := fs.Uint64("segment", 0, "segment number")
	index := fs.Int("index", -1, "leaf index within the segment")
	if err := fs.Parse(reorderArgs(fs, args)); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("prove needs exactly one directory")
	}
	if *segNo == 0 || *index < 0 {
		return fmt.Errorf("prove requires -segment and -index")
	}
	dir := fs.Arg(0)
	logPath, ckptPath := chain.SegmentPaths(dir, *segNo)
	recs, _, err := (&chain.Segment{Number: *segNo, Dir: dir, LogPath: logPath}).ReadRecords()
	if err != nil {
		return err
	}
	proof, err := chain.ProofFor(recs, *index)
	if err != nil {
		return err
	}
	cp, err := chain.ReadCheckpoint(ckptPath)
	if err != nil {
		return err
	}
	out := map[string]any{
		"segment": *segNo,
		"index":   *index,
		"count":   cp.Count,
		"root":    cp.Root,
		"leaf":    recs[*index].Hash,
		"proof":   proof,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func shortHash(h string) string {
	if len(h) <= 16 {
		return h
	}
	return h[:16]
}
