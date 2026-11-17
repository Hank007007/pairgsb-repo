// Command kekringctl manages keyring files.
package main

import (
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"

	"kekring/internal/keyring"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "init":
		runInit(os.Args[2:])
	case "rotate":
		runRotate(os.Args[2:])
	case "status":
		runStatus(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: kekringctl <init|rotate|status> [flags]")
}

func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	masterFile := fs.String("master-file", "", "file holding the master secret")
	masterHex := fs.String("master-hex", "", "master secret as hex")
	out := fs.String("out", "keyring.json", "output keyring file")
	version := fs.Uint("version", 1, "initial key version")
	_ = fs.Parse(args)

	master, err := masterSecret(*masterFile, *masterHex)
	if err != nil {
		log.Fatalf("master secret: %v", err)
	}

	kek, err := keyring.DeriveKEK(master, uint32(*version))
	if err != nil {
		log.Fatalf("derive kek: %v", err)
	}

	kr := keyring.New([]keyring.Key{{
		Version:  uint32(*version),
		Status:   keyring.StatusActive,
		Material: kek,
	}})
	if err := keyring.Save(*out, kr); err != nil {
		log.Fatalf("save keyring: %v", err)
	}
	fmt.Printf("wrote %s with version %d\n", *out, *version)
}

func runRotate(args []string) {
	fs := flag.NewFlagSet("rotate", flag.ExitOnError)
	masterFile := fs.String("master-file", "", "file holding the master secret")
	masterHex := fs.String("master-hex", "", "master secret as hex")
	path := fs.String("keyring", "keyring.json", "keyring file")
	_ = fs.Parse(args)

	master, err := masterSecret(*masterFile, *masterHex)
	if err != nil {
		log.Fatalf("master secret: %v", err)
	}

	kr, err := keyring.Load(*path)
	if err != nil {
		log.Fatalf("load keyring: %v", err)
	}

	version := kr.NextVersion()
	kek, err := keyring.DeriveKEK(master, version)
	if err != nil {
		log.Fatalf("derive kek: %v", err)
	}

	if err := kr.Rotate(keyring.Key{
		Version:  version,
		Status:   keyring.StatusActive,
		Material: kek,
	}); err != nil {
		log.Fatalf("rotate: %v", err)
	}

	if err := keyring.Save(*path, kr); err != nil {
		log.Fatalf("save keyring: %v", err)
	}
	fmt.Printf("rotated %s to version %d\n", *path, version)
}

func runStatus(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	path := fs.String("keyring", "keyring.json", "keyring file")
	_ = fs.Parse(args)

	kr, err := keyring.Load(*path)
	if err != nil {
		log.Fatalf("load keyring: %v", err)
	}

	for _, k := range kr.Keys() {
		fmt.Printf("version=%d status=%s bytes=%d\n", k.Version, k.Status, len(k.Material))
	}
}

func masterSecret(file, hexValue string) ([]byte, error) {
	switch {
	case file != "":
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		return data, nil
	case hexValue != "":
		return hex.DecodeString(hexValue)
	default:
		return nil, fmt.Errorf("one of -master-file or -master-hex is required")
	}
}
