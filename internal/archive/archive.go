// Package archive reads the entry stream of an uploaded package.
//
// A Reader yields entries one at a time. The caller is responsible for
// consuming each entry's body before asking for the next one.
package archive

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Type classifies an archive entry.
type Type int

const (
	// TypeFile is a regular file.
	TypeFile Type = iota
	// TypeDir is a directory.
	TypeDir
	// TypeSymlink is a symbolic link.
	TypeSymlink
	// TypeHardlink is a hard link to another entry.
	TypeHardlink
	// TypeDevice is a block or character device.
	TypeDevice
	// TypeFifo is a named pipe.
	TypeFifo
	// TypeOther is any entry type not listed above.
	TypeOther
)

// String reports the entry type in a form suitable for logs.
func (t Type) String() string {
	switch t {
	case TypeFile:
		return "file"
	case TypeDir:
		return "dir"
	case TypeSymlink:
		return "symlink"
	case TypeHardlink:
		return "hardlink"
	case TypeDevice:
		return "device"
	case TypeFifo:
		return "fifo"
	default:
		return "other"
	}
}

// Entry is a single item from a package.
type Entry struct {
	// Name is the entry name exactly as it appears in the archive.
	Name string
	// Linkname is the target recorded for a symlink or hardlink.
	Linkname string
	// Type is the entry kind.
	Type Type
	// Mode is the permission and type bits recorded for the entry.
	Mode os.FileMode
	// DeclaredSize is the uncompressed size stated by the package header.
	DeclaredSize int64
	// Format is the producing archive format, for diagnostics.
	Format string

	body     io.Reader
	open     func() (io.ReadCloser, error)
	sparse   []sparseSeg
	sparseTo int64
}

// Open returns a reader for the entry body. For entries that carry no body,
// the reader is empty.
func (e *Entry) Open() (io.ReadCloser, error) {
	if e.open != nil {
		return e.open()
	}
	if e.body != nil {
		return io.NopCloser(e.body), nil
	}
	return io.NopCloser(bytes.NewReader(nil)), nil
}

// Reader yields entries from a package.
type Reader interface {
	// Next returns the next entry, or io.EOF when the package is exhausted.
	Next() (*Entry, error)
	// Close releases any resources held by the reader.
	Close() error
}

// Open inspects path and returns a Reader for the package it contains.
func Open(path string) (Reader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}

	var head [512]byte
	n, err := io.ReadFull(f, head[:])
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		f.Close()
		return nil, err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		f.Close()
		return nil, err
	}

	switch {
	case n >= 2 && head[0] == 0x1f && head[1] == 0x8b:
		gz, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			return nil, err
		}
		return newTarReader(gz, multiCloser{gz, f}, "tar.gz"), nil
	case n >= 4 && bytes.Equal(head[:4], []byte{'P', 'K', 0x03, 0x04}):
		return newZipReader(f, info.Size())
	case n >= 262 && string(head[257:262]) == "ustar":
		return newTarReader(f, f, "tar"), nil
	}

	switch strings.ToLower(filepath.Ext(path)) {
	case ".zip":
		return newZipReader(f, info.Size())
	case ".tar":
		return newTarReader(f, f, "tar"), nil
	case ".tgz", ".tar.gz":
		gz, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			return nil, err
		}
		return newTarReader(gz, multiCloser{gz, f}, "tar.gz"), nil
	}

	f.Close()
	return nil, fmt.Errorf("archive: unrecognised package format")
}

// multiCloser closes several resources in order.
type multiCloser []io.Closer

func (m multiCloser) Close() error {
	var first error
	for _, c := range m {
		if err := c.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}
