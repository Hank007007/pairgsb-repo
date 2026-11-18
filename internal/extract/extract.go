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
	inputRoot := filepath.Join(jobRoot, InputDir)
	if err := os.MkdirAll(inputRoot, 0o755); err != nil {
		return fmt.Errorf("extract: prepare job input: %w", err)
	}
	// Resolve the root once so containment checks compare real paths even when
	// the job directory itself sits behind a symlink.
	inputRoot, err := filepath.EvalSymlinks(inputRoot)
	if err != nil {
		return fmt.Errorf("extract: resolve job input: %w", err)
	}

	r, err := archive.Open(pkgPath)
	if err != nil {
		return fmt.Errorf("extract: %w", err)
	}
	defer r.Close()

	budget := quota.New(cfg.MaxBytes, cfg.MaxRatio)
	var processed int64

	for {
		entry, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("extract: read entry: %w", err)
		}
		if cfg.MaxEntries > 0 && processed >= cfg.MaxEntries {
			return fmt.Errorf("%w: entry %q", fault.ErrEntryLimit, entry.Name)
		}
		if err := extractEntry(inputRoot, entry, budget, r.CompressedConsumed); err != nil {
			return err
		}
		processed++
	}
	return nil
}

// extractEntry places one package entry under the job's input directory.
func extractEntry(root string, entry *archive.Entry, budget *quota.Budget, consumed func() int64) error {
	target, err := pathguard.Resolve(root, entry.Name)
	if err != nil {
		return err
	}

	switch entry.Type {
	case archive.TypeDevice, archive.TypeFifo, archive.TypeOther:
		return fmt.Errorf("%w: %q is a %s", fault.ErrUnsupportedType, entry.Name, entry.Type)
	}

	// Resolve any symlink already present in the landing path before the entry
	// touches the filesystem.
	dest, err := pathguard.ResolveExisting(root, target)
	if err != nil {
		return fmt.Errorf("%w: entry %q", err, entry.Name)
	}

	switch entry.Type {
	case archive.TypeDir:
		if err := os.MkdirAll(dest, 0o755); err != nil {
			return fmt.Errorf("extract: create directory %q: %w", entry.Name, err)
		}
		return nil

	case archive.TypeSymlink:
		if err := pathguard.CheckSymlinkTarget(root, dest, entry.Linkname); err != nil {
			return fmt.Errorf("%w: entry %q", err, entry.Name)
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("extract: prepare %q: %w", entry.Name, err)
		}
		if err := os.Symlink(entry.Linkname, dest); err != nil {
			return fmt.Errorf("extract: create symlink %q: %w", entry.Name, err)
		}
		return nil

	case archive.TypeHardlink:
		source, err := pathguard.Resolve(root, entry.Linkname)
		if err != nil {
			return fmt.Errorf("%w: entry %q: %v", fault.ErrHardlinkEscape, entry.Name, err)
		}
		source, err = pathguard.ResolveExisting(root, source)
		if err != nil {
			return fmt.Errorf("%w: entry %q: %v", fault.ErrHardlinkEscape, entry.Name, err)
		}
		info, err := os.Lstat(source)
		if err != nil {
			return fmt.Errorf("%w: entry %q: target %q does not exist", fault.ErrHardlinkEscape, entry.Name, entry.Linkname)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("%w: entry %q: target %q is not a regular file", fault.ErrHardlinkEscape, entry.Name, entry.Linkname)
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return fmt.Errorf("extract: prepare %q: %w", entry.Name, err)
		}
		if err := os.Link(source, dest); err != nil {
			return fmt.Errorf("extract: create hardlink %q: %w", entry.Name, err)
		}
		return nil

	case archive.TypeFile:
		return writeFile(dest, entry, budget, consumed)
	}

	return fmt.Errorf("%w: %q", fault.ErrUnsupportedType, entry.Name)
}

func writeFile(target string, entry *archive.Entry, budget *quota.Budget, consumed func() int64) error {
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

	buf := make([]byte, 32*1024)
	for {
		n, readErr := body.Read(buf)
		if n > 0 {
			// Account for what is about to be written, not what the header
			// claimed. A sparse entry or a zip with a lying size field is
			// therefore charged for every byte it actually emits.
			if err := budget.Add(int64(n)); err != nil {
				f.Close()
				return fmt.Errorf("%w: entry %q", err, entry.Name)
			}
			written, writeErr := f.Write(buf[:n])
			if writeErr != nil {
				f.Close()
				return fmt.Errorf("extract: write %q: %w", entry.Name, writeErr)
			}
			if written != n {
				f.Close()
				return fmt.Errorf("extract: write %q: %w", entry.Name, io.ErrShortWrite)
			}
			if err := budget.ObserveRatio(consumed(), budget.Used); err != nil {
				f.Close()
				return fmt.Errorf("%w: entry %q", err, entry.Name)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			f.Close()
			return fmt.Errorf("extract: write %q: %w", entry.Name, readErr)
		}
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("extract: close %q: %w", entry.Name, err)
	}
	return nil
}
