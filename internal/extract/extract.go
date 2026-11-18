// Package extract unpacks an uploaded package into a job's input directory.
//
// The extractor is the trust boundary: everything it writes is handed to the
// build step that follows, and that step does no further checking. Each entry
// is checked against the rules in docs/extract-policy.md before it reaches
// the filesystem.
package extract

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/pairgsb/unpackfence/internal/archive"
	"github.com/pairgsb/unpackfence/internal/fault"
	"github.com/pairgsb/unpackfence/internal/pathguard"
	"github.com/pairgsb/unpackfence/internal/quota"
)

// Config carries the limits applied to a single extraction. A zero limit
// leaves that dimension unbounded.
type Config struct {
	MaxBytes   int64
	MaxEntries int64
	MaxRatio   float64
}

// DefaultConfig is the limit set used when the caller has no opinion.
var DefaultConfig = Config{
	MaxBytes:   4 << 30,
	MaxEntries: 200000,
	MaxRatio:   250,
}

// InputDir is the directory beneath the job root that receives package
// contents.
const InputDir = "in"

// Extract unpacks pkgPath beneath jobDir and returns the fault that caused the
// package to be refused, if any.
func Extract(jobDir, pkgPath string, cfg Config) error {
	jobRoot := filepath.Clean(jobDir)
	if err := os.MkdirAll(filepath.Join(jobRoot, InputDir), 0o755); err != nil {
		return fmt.Errorf("extract: prepare job input: %w", err)
	}

	r, err := archive.Open(pkgPath)
	if err != nil {
		return fmt.Errorf("extract: %w", err)
	}
	defer r.Close()

	budget := quota.New(cfg.MaxBytes, cfg.MaxRatio)
	pkgSize := fileSize(pkgPath)

	for {
		entry, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("extract: read entry: %w", err)
		}
		if err := extractEntry(jobRoot, entry, budget); err != nil {
			return err
		}
		if err := budget.ObserveRatio(pkgSize, budget.Used); err != nil {
			return err
		}
	}
	return nil
}

// extractEntry places one package entry under the job's input directory.
func extractEntry(jobRoot string, entry *archive.Entry, budget *quota.Budget) error {
	if err := pathguard.CheckName(entry.Name); err != nil {
		return err
	}

	// Entries are staged inside the input directory of the job root.
	staged := InputDir + "/" + entry.Name
	target, err := pathguard.Resolve(jobRoot, staged)
	if err != nil {
		return err
	}

	if err := budget.Add(entry.DeclaredSize); err != nil {
		return fmt.Errorf("%w: entry %q", err, entry.Name)
	}

	switch entry.Type {
	case archive.TypeDir:
		if err := os.MkdirAll(target, 0o755); err != nil {
			return fmt.Errorf("extract: create directory %q: %w", entry.Name, err)
		}
		return nil

	case archive.TypeSymlink:
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("extract: prepare %q: %w", entry.Name, err)
		}
		if err := os.Symlink(entry.Linkname, target); err != nil {
			return fmt.Errorf("extract: create symlink %q: %w", entry.Name, err)
		}
		return nil

	case archive.TypeHardlink:
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("extract: prepare %q: %w", entry.Name, err)
		}
		source := filepath.Join(jobRoot, InputDir, entry.Linkname)
		if err := os.Link(source, target); err != nil {
			return fmt.Errorf("extract: create hardlink %q: %w", entry.Name, err)
		}
		return nil

	case archive.TypeDevice, archive.TypeFifo, archive.TypeOther:
		return fmt.Errorf("%w: %q is a %s", fault.ErrUnsupportedType, entry.Name, entry.Type)

	case archive.TypeFile:
		return writeFile(target, entry)
	}

	return fmt.Errorf("%w: %q", fault.ErrUnsupportedType, entry.Name)
}

func writeFile(target string, entry *archive.Entry) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("extract: prepare %q: %w", entry.Name, err)
	}

	body, err := entry.Open()
	if err != nil {
		return fmt.Errorf("extract: open %q: %w", entry.Name, err)
	}
	defer body.Close()

	f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("extract: create %q: %w", entry.Name, err)
	}
	if _, err := io.Copy(f, body); err != nil {
		f.Close()
		return fmt.Errorf("extract: write %q: %w", entry.Name, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("extract: close %q: %w", entry.Name, err)
	}
	return nil
}

// fileSize reports the size of the package on disk, used as the denominator
// when watching the compression ratio.
func fileSize(pkgPath string) int64 {
	info, err := os.Stat(pkgPath)
	if err != nil {
		return 0
	}
	return info.Size()
}
