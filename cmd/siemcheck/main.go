// Command siemcheck simulates the ingest side of the pipeline. It reads a
// record stream on stdin, splits it into lines, and reports how many events
// the receiver observed. Each non-empty line is one event.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	countKeys := flag.Bool("keys", false, "print the field names seen on each event")
	flag.Parse()

	count, err := run(os.Stdin, os.Stdout, *countKeys)
	if err != nil {
		fmt.Fprintln(os.Stderr, "siemcheck:", err)
		os.Exit(1)
	}

	fmt.Printf("events=%d\n", count)
}

func run(r io.Reader, w io.Writer, showKeys bool) (int, error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	events := 0
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		events++
		if showKeys {
			fmt.Fprintf(w, "event %d: %s\n", events, strings.Join(splitKeys(line), ","))
		}
	}
	if err := scanner.Err(); err != nil {
		return events, err
	}
	return events, nil
}

// splitKeys returns the field names of a kv record in order.
func splitKeys(line string) []string {
	parts := strings.Fields(line)
	keys := make([]string, 0, len(parts))
	for _, p := range parts {
		if i := strings.IndexByte(p, '='); i >= 0 {
			keys = append(keys, p[:i])
			continue
		}
		keys = append(keys, p)
	}
	return keys
}
