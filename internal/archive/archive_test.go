package archive

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// buildTarGz writes a small package holding a file, a directory and a symlink
// that points at another member of the same package.
func buildTarGz(t *testing.T) []byte {
	t.Helper()

	var raw bytes.Buffer
	tw := tar.NewWriter(&raw)
	entries := []struct {
		hdr  *tar.Header
		body []byte
	}{
		{&tar.Header{Name: "src/", Typeflag: tar.TypeDir, Mode: 0o755}, nil},
		{&tar.Header{Name: "src/main.go", Typeflag: tar.TypeReg, Mode: 0o644, Size: 18}, []byte("package main\n\nfunc")},
		{&tar.Header{Name: "README.md", Typeflag: tar.TypeReg, Mode: 0o644, Size: 6}, []byte("hello\n")},
		{&tar.Header{Name: "latest", Typeflag: tar.TypeSymlink, Linkname: "README.md", Mode: 0o777}, nil},
	}
	for _, e := range entries {
		if err := tw.WriteHeader(e.hdr); err != nil {
			t.Fatalf("write header %q: %v", e.hdr.Name, err)
		}
		if len(e.body) > 0 {
			if _, err := tw.Write(e.body); err != nil {
				t.Fatalf("write body %q: %v", e.hdr.Name, err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}

	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	if _, err := zw.Write(raw.Bytes()); err != nil {
		t.Fatalf("gzip: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return gz.Bytes()
}

func TestOpenReadsTarGzEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pkg.tar.gz")
	if err := os.WriteFile(path, buildTarGz(t), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer r.Close()

	var names []string
	var types []Type
	var bodies []string
	for {
		entry, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		names = append(names, entry.Name)
		types = append(types, entry.Type)
		body, err := io.ReadAll(mustOpen(t, entry))
		if err != nil {
			t.Fatalf("read %q: %v", entry.Name, err)
		}
		bodies = append(bodies, string(body))
	}

	wantNames := []string{"src/", "src/main.go", "README.md", "latest"}
	if len(names) != len(wantNames) {
		t.Fatalf("got %d entries, want %d: %v", len(names), len(wantNames), names)
	}
	for i, want := range wantNames {
		if names[i] != want {
			t.Fatalf("entry %d = %q, want %q", i, names[i], want)
		}
	}

	wantTypes := []Type{TypeDir, TypeFile, TypeFile, TypeSymlink}
	for i, want := range wantTypes {
		if types[i] != want {
			t.Fatalf("entry %q type = %v, want %v", names[i], types[i], want)
		}
	}
	if bodies[1] != "package main\n\nfunc" {
		t.Fatalf("src/main.go body = %q", bodies[1])
	}
	if bodies[2] != "hello\n" {
		t.Fatalf("README.md body = %q", bodies[2])
	}
}

func TestOpenReadsZipEntries(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create("docs/guide.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("guide\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := zw.Create("docs/"); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "pkg.zip")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer r.Close()

	first, err := r.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if first.Name != "docs/guide.txt" || first.Type != TypeFile {
		t.Fatalf("first entry = %q (%v)", first.Name, first.Type)
	}
	body, err := io.ReadAll(mustOpen(t, first))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "guide\n" {
		t.Fatalf("body = %q", body)
	}

	second, err := r.Next()
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if second.Type != TypeDir {
		t.Fatalf("second entry type = %v, want dir", second.Type)
	}
}

func mustOpen(t *testing.T, e *Entry) io.ReadCloser {
	t.Helper()
	rc, err := e.Open()
	if err != nil {
		t.Fatalf("open %q: %v", e.Name, err)
	}
	return rc
}
