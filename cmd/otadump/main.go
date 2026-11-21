// Command otadump decodes an OTA manifest and prints its structure.
//
// Usage:
//
//	otadump [-model NAME] FILE
//
// With -model the dump also reports which target Select picks for that device
// model. The output format is stable; tests compare its lines.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"cborveil/internal/manifest"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("otadump", flag.ContinueOnError)
	fs.SetOutput(stderr)
	model := fs.String("model", "", "device model to select a target for")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "usage: otadump [-model NAME] FILE\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return 2
	}

	data, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(stderr, "otadump: %v\n", err)
		return 1
	}

	m, err := manifest.Parse(data)
	if err != nil {
		fmt.Fprintf(stderr, "otadump: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "version: %s\n", m.Version)
	fmt.Fprintf(stdout, "size: %d\n", m.Size)
	fmt.Fprintf(stdout, "key_id: %s\n", m.KeyID)
	fmt.Fprintf(stdout, "sig: %d bytes\n", len(m.Sig))
	fmt.Fprintf(stdout, "targets: %d\n", len(m.Targets))
	for i, t := range m.Targets {
		fmt.Fprintf(stdout, "target[%d]: model=%s url=%s sha256=%x size=%d offset=%d\n",
			i, t.Model, t.URL, t.SHA256, t.Size, t.Offset)
	}

	if *model != "" {
		t, err := manifest.Select(m, *model)
		if err != nil {
			fmt.Fprintf(stderr, "otadump: select %q: %v\n", *model, err)
			return 1
		}
		fmt.Fprintf(stdout, "selected: model=%s url=%s sha256=%x size=%d offset=%d\n",
			t.Model, t.URL, t.SHA256, t.Size, t.Offset)
	}
	return 0
}
