// Command unpackfence unpacks a partner package into a job's input directory.
//
// Usage:
//
//	unpackfence -job jobs/<job> -pkg /path/to/package.tar.gz
//
// The command prints the refusal reason on stderr and exits non-zero when a
// package is rejected.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/pairgsb/unpackfence/internal/extract"
	"github.com/pairgsb/unpackfence/internal/fault"
)

func main() {
	job := flag.String("job", "", "job directory that receives the package")
	pkg := flag.String("pkg", "", "path to the uploaded package")
	maxBytes := flag.Int64("max-bytes", extract.DefaultConfig.MaxBytes, "budget in written bytes (0 = unbounded)")
	maxEntries := flag.Int64("max-entries", extract.DefaultConfig.MaxEntries, "entry-count limit (0 = unbounded)")
	maxRatio := flag.Float64("max-ratio", extract.DefaultConfig.MaxRatio, "compression ratio limit (0 = unbounded)")
	flag.Parse()

	if *job == "" || *pkg == "" {
		fmt.Fprintln(os.Stderr, "unpackfence: -job and -pkg are required")
		flag.Usage()
		os.Exit(2)
	}

	cfg := extract.Config{
		MaxBytes:   *maxBytes,
		MaxEntries: *maxEntries,
		MaxRatio:   *maxRatio,
	}

	if err := extract.Extract(*job, *pkg, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "unpackfence: %s: %v\n", reason(err), err)
		os.Exit(1)
	}

	fmt.Printf("unpackfence: extracted %s into %s\n", *pkg, *job)
}

// reason names the policy refusal behind an extraction error.
func reason(err error) string {
	switch {
	case errors.Is(err, fault.ErrUnsafeName):
		return "unsafe-name"
	case errors.Is(err, fault.ErrOutsideRoot):
		return "outside-root"
	case errors.Is(err, fault.ErrSymlinkEscape):
		return "symlink-escape"
	case errors.Is(err, fault.ErrHardlinkEscape):
		return "hardlink-escape"
	case errors.Is(err, fault.ErrUnsupportedType):
		return "unsupported-type"
	case errors.Is(err, fault.ErrBudgetExceeded):
		return "budget-exceeded"
	case errors.Is(err, fault.ErrEntryLimit):
		return "entry-limit"
	case errors.Is(err, fault.ErrRatioExceeded):
		return "ratio-exceeded"
	default:
		return "extract-failed"
	}
}
