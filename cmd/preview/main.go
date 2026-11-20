// Command preview is a small demo program. It reads sample log lines from
// testdata and renders them under each policy so the before/after difference
// can be observed on a terminal.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"termveil/internal/render"
)

func main() {
	dir := flag.String("dir", "testdata", "directory holding sample log files")
	limit := flag.Int("width", 6, "column limit used by the width policy")
	flag.Parse()

	files, err := filepath.Glob(filepath.Join(*dir, "*.log"))
	if err != nil {
		fail(err)
	}
	if len(files) == 0 {
		fail(fmt.Errorf("no sample logs under %s", *dir))
	}
	sort.Strings(files)

	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			fail(err)
		}
		data = trimNewline(data)
		fmt.Printf("=== %s ===\n", filepath.Base(path))
		fmt.Printf("raw    : %q\n", string(data))
		for _, name := range []string{"plain", "color", "width"} {
			p, err := render.PolicyFor(name, *limit)
			if err != nil {
				fail(err)
			}
			fmt.Printf("%-6s : %q\n", name, render.Apply(p, data))
		}
		fmt.Println()
	}
}

// trimNewline drops the single trailing line break a sample file carries.
func trimNewline(data []byte) []byte {
	for len(data) > 0 && (data[len(data)-1] == '\n' || data[len(data)-1] == '\r') {
		data = data[:len(data)-1]
	}
	return data
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "preview:", err)
	os.Exit(1)
}
