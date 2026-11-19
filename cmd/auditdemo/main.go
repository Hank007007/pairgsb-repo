// Command auditdemo encodes one representative audit event and writes the
// record to stdout, followed by a single newline.
package main

import (
	"flag"
	"fmt"
	"os"

	"logline-audit-encode/internal/audit"
	"logline-audit-encode/internal/encode"
)

func main() {
	format := flag.String("format", "kv", "output format: kv or json")
	hostile := flag.Bool("hostile", false, "use an event whose user agent carries a CRLF sequence")
	terminator := flag.Bool("terminator", true, "write the trailing newline")
	flag.Parse()

	ev := audit.Sample()
	if *hostile {
		ev.UserAgent = "Mozilla/5.0\r\nlevel=critical user=root"
	}

	enc, err := encode.NewEncoder(encode.Format(*format))
	if err != nil {
		fmt.Fprintln(os.Stderr, "auditdemo:", err)
		os.Exit(1)
	}

	record, err := enc.Encode(ev)
	if err != nil {
		fmt.Fprintln(os.Stderr, "auditdemo:", err)
		os.Exit(1)
	}

	os.Stdout.Write(record)
	if *terminator {
		os.Stdout.Write([]byte{'\n'})
	}
}
