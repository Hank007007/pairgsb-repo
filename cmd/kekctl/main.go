// Command kekctl is a small operator tool for the kekseal envelope library.
//
// Usage:
//
//	kekctl ring init  -out ring.json [-versions 2]
//	kekctl seal       -ring ring.json -tenant TENANT -id ID -in plain.txt [-out rec.json]
//	kekctl open       -ring ring.json -in rec.json [-out plain.txt]
//	kekctl rotate     -ring ring.json -dir records/
package main

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kekseal/internal/keystore"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "ring":
		err = cmdRing(os.Args[2:])
	case "seal":
		err = cmdSeal(os.Args[2:])
	case "open":
		err = cmdOpen(os.Args[2:])
	case "rotate":
		err = cmdRotate(os.Args[2:])
	case "help", "-h", "--help":
		usage()
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "kekctl:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `kekctl - envelope key operations

  kekctl ring init  -out ring.json [-versions 2] [-grace 24h]
  kekctl seal       -ring ring.json -tenant TENANT -id ID -in plain.txt [-out rec.json]
  kekctl open       -ring ring.json -in rec.json [-out plain.txt]
  kekctl rotate     -ring ring.json -dir records/
`)
}

func cmdRing(args []string) error {
	if len(args) == 0 || args[0] != "init" {
		return errors.New("usage: kekctl ring init -out ring.json")
	}
	fs := flag.NewFlagSet("ring init", flag.ContinueOnError)
	out := fs.String("out", "", "output path for the key ring")
	versions := fs.Int("versions", 1, "number of KEK versions to generate")
	grace := fs.Duration("grace", 24*time.Hour, "retirement grace period")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *out == "" {
		return errors.New("ring init: -out is required")
	}
	if *versions < 1 {
		return errors.New("ring init: -versions must be at least 1")
	}

	ring := &keystore.Ring{GracePeriod: *grace}
	now := time.Now().UTC().Truncate(time.Second)
	for v := 1; v <= *versions; v++ {
		kek := make([]byte, 32)
		if _, err := rand.Read(kek); err != nil {
			return err
		}
		ring.Versions = append(ring.Versions, keystore.KeyVersion{
			Version:   v,
			KEK:       kek,
			CreatedAt: now,
		})
	}
	if err := ring.WriteRing(*out); err != nil {
		return err
	}
	fmt.Printf("wrote %s (versions 1..%d, active %d)\n", *out, *versions, ring.LatestVersion())
	return nil
}

func cmdSeal(args []string) error {
	fs := flag.NewFlagSet("seal", flag.ContinueOnError)
	ringPath := fs.String("ring", "", "key ring path")
	tenant := fs.String("tenant", "", "tenant the record belongs to")
	id := fs.String("id", "", "record id")
	in := fs.String("in", "", "plaintext file (defaults to stdin)")
	out := fs.String("out", "", "record output path (defaults to stdout)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *ringPath == "" || *tenant == "" || *id == "" {
		return errors.New("seal: -ring, -tenant and -id are required")
	}

	ring, err := keystore.LoadRing(*ringPath)
	if err != nil {
		return err
	}
	var plaintext []byte
	if *in == "" || *in == "-" {
		plaintext, err = io.ReadAll(os.Stdin)
	} else {
		plaintext, err = os.ReadFile(*in)
	}
	if err != nil {
		return err
	}

	rec, err := keystore.NewSealer(ring).Seal(*tenant, *id, plaintext)
	if err != nil {
		return err
	}
	if *out == "" || *out == "-" {
		data, err := json.MarshalIndent(rec, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}
	return keystore.WriteRecord(*out, rec)
}

func cmdOpen(args []string) error {
	fs := flag.NewFlagSet("open", flag.ContinueOnError)
	ringPath := fs.String("ring", "", "key ring path")
	in := fs.String("in", "", "record path")
	out := fs.String("out", "", "plaintext output path (defaults to stdout)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *ringPath == "" || *in == "" {
		return errors.New("open: -ring and -in are required")
	}
	ring, err := keystore.LoadRing(*ringPath)
	if err != nil {
		return err
	}
	rec, err := keystore.ReadRecord(*in)
	if err != nil {
		return err
	}
	plaintext, err := keystore.NewSealer(ring).Open(rec)
	if err != nil {
		return err
	}
	if *out == "" || *out == "-" {
		_, err = os.Stdout.Write(plaintext)
		return err
	}
	return os.WriteFile(*out, plaintext, 0o600)
}

func cmdRotate(args []string) error {
	fs := flag.NewFlagSet("rotate", flag.ContinueOnError)
	ringPath := fs.String("ring", "", "key ring path")
	dir := fs.String("dir", "", "directory of record files")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *ringPath == "" || *dir == "" {
		return errors.New("rotate: -ring and -dir are required")
	}
	ring, err := keystore.LoadRing(*ringPath)
	if err != nil {
		return err
	}
	paths, err := recordPaths(*dir)
	if err != nil {
		return err
	}

	records := make([]*keystore.Record, 0, len(paths))
	originalVersions := make([]int, 0, len(paths))
	for _, p := range paths {
		rec, err := keystore.ReadRecord(p)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		records = append(records, rec)
		originalVersions = append(originalVersions, rec.KeyVersion)
	}

	res, err := keystore.NewSealer(ring).RotateKEK(records)
	if err != nil {
		return err
	}

	var writePaths []string
	var writeRecords []*keystore.Record
	for i, p := range paths {
		if originalVersions[i] == res.TargetVersion {
			continue
		}
		writePaths = append(writePaths, p)
		writeRecords = append(writeRecords, records[i])
	}
	if err := writeRecordsAtomic(writePaths, writeRecords); err != nil {
		return err
	}
	fmt.Printf("rotated %d record(s) to version %d, skipped %d\n",
		res.Rotated, res.TargetVersion, res.Skipped)
	return nil
}

// writeRecordsAtomic stages every record before replacing any destination, so
// a failure while preparing the batch leaves every original file in place.
func writeRecordsAtomic(paths []string, records []*keystore.Record) error {
	type stagedFile struct {
		tmp string
		dst string
	}
	staged := make([]stagedFile, 0, len(paths))
	cleanup := func() {
		for _, f := range staged {
			_ = os.Remove(f.tmp)
		}
	}

	for i, path := range paths {
		data, err := json.MarshalIndent(records[i], "", "  ")
		if err != nil {
			cleanup()
			return err
		}
		data = append(data, '\n')

		tmp, err := os.CreateTemp(filepath.Dir(path), ".kekseal-rotate-*.tmp")
		if err != nil {
			cleanup()
			return err
		}
		tmpName := tmp.Name()
		if _, err := tmp.Write(data); err != nil {
			tmp.Close()
			_ = os.Remove(tmpName)
			cleanup()
			return err
		}
		if err := tmp.Chmod(0o600); err != nil {
			tmp.Close()
			_ = os.Remove(tmpName)
			cleanup()
			return err
		}
		if err := tmp.Sync(); err != nil {
			tmp.Close()
			_ = os.Remove(tmpName)
			cleanup()
			return err
		}
		if err := tmp.Close(); err != nil {
			_ = os.Remove(tmpName)
			cleanup()
			return err
		}
		staged = append(staged, stagedFile{tmp: tmpName, dst: path})
	}

	for _, f := range staged {
		if err := os.Rename(f.tmp, f.dst); err != nil {
			cleanup()
			return fmt.Errorf("%s: %w", f.dst, err)
		}
	}
	return nil
}

func recordPaths(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		paths = append(paths, filepath.Join(dir, e.Name()))
	}
	return paths, nil
}
