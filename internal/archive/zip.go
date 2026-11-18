package archive

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"fmt"
	"io"
	"os"
	"strings"
)

// zipReader reads entries from a zip container.
type zipReader struct {
	zr         *zip.Reader
	file       *os.File
	next       int
	cur        io.ReadCloser
	compressed int64
}

func newZipReader(f *os.File, size int64) (*zipReader, error) {
	zr, err := zip.NewReader(f, size)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("archive: open zip: %w", err)
	}
	return &zipReader{zr: zr, file: f}, nil
}

func (z *zipReader) Close() error {
	if z.cur != nil {
		z.cur.Close()
		z.cur = nil
	}
	return z.file.Close()
}

func (z *zipReader) CompressedConsumed() int64 {
	return z.compressed
}

func (z *zipReader) Next() (*Entry, error) {
	if z.cur != nil {
		io.Copy(io.Discard, z.cur)
		z.cur.Close()
		z.cur = nil
	}
	if z.next >= len(z.zr.File) {
		return nil, io.EOF
	}
	f := z.zr.File[z.next]
	z.next++

	e := &Entry{
		Name:         f.Name,
		Mode:         f.Mode().Perm(),
		DeclaredSize: int64(f.UncompressedSize64),
		Format:       "zip",
	}

	switch {
	case f.FileInfo().IsDir() || strings.HasSuffix(f.Name, "/"):
		e.Type = TypeDir
	case f.Mode()&os.ModeSymlink != 0:
		e.Type = TypeSymlink
		target, err := z.readAll(f, 1<<16)
		if err != nil {
			return nil, err
		}
		e.Linkname = string(bytes.TrimRight(target, "\x00"))
	case f.Mode()&os.ModeDevice != 0:
		e.Type = TypeDevice
	case f.Mode()&os.ModeNamedPipe != 0:
		e.Type = TypeFifo
	case f.Mode()&os.ModeSocket != 0:
		e.Type = TypeOther
	default:
		e.Type = TypeFile
		e.open = func() (io.ReadCloser, error) {
			rc, err := z.openEntry(f)
			if err != nil {
				return nil, err
			}
			z.cur = rc
			return rc, nil
		}
	}
	return e, nil
}

func (z *zipReader) readAll(f *zip.File, limit int64) ([]byte, error) {
	rc, err := z.openEntry(f)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, limit))
}

// openEntry returns the decompressed body of f.
func (z *zipReader) openEntry(f *zip.File) (io.ReadCloser, error) {
	raw, err := f.OpenRaw()
	if err != nil {
		return nil, fmt.Errorf("archive: open %q: %w", f.Name, err)
	}
	counted := &countingReader{r: raw, n: &z.compressed}
	switch f.Method {
	case zip.Store:
		return io.NopCloser(counted), nil
	case zip.Deflate:
		return flate.NewReader(counted), nil
	default:
		return nil, fmt.Errorf("archive: unsupported compression method %d for %q", f.Method, f.Name)
	}
}
