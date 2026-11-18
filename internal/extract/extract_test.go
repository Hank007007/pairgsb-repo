package extract

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

type tarItem struct {
	hdr  *tar.Header
	body []byte
}

func writePackage(t *testing.T, dir, name string, items []tarItem) string {
	t.Helper()

	var raw bytes.Buffer
	tw := tar.NewWriter(&raw)
	for _, it := range items {
		if err := tw.WriteHeader(it.hdr); err != nil {
			t.Fatalf("tar header %q: %v", it.hdr.Name, err)
		}
		if len(it.body) > 0 {
			if _, err := tw.Write(it.body); err != nil {
				t.Fatalf("tar body %q: %v", it.hdr.Name, err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}

	var out bytes.Buffer
	zw := gzip.NewWriter(&out)
	if _, err := zw.Write(raw.Bytes()); err != nil {
		t.Fatalf("gzip: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
		t.Fatalf("write package: %v", err)
	}
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %q: %v", path, err)
	}
	return string(b)
}

func TestExtractPlainFiles(t *testing.T) {
	dir := t.TempDir()
	pkg := writePackage(t, dir, "pkg.tar.gz", []tarItem{
		{hdr: &tar.Header{Name: "src/", Typeflag: tar.TypeDir, Mode: 0o755}},
		{hdr: &tar.Header{Name: "src/main.go", Typeflag: tar.TypeReg, Mode: 0o644, Size: 12}, body: []byte("package main")},
		{hdr: &tar.Header{Name: "Makefile", Typeflag: tar.TypeReg, Mode: 0o644, Size: 5}, body: []byte("all:\n")},
	})

	job := filepath.Join(dir, "jobs", "plain")
	if err := Extract(job, pkg, DefaultConfig); err != nil {
		t.Fatalf("Extract: %v", err)
	}

	if got := readFile(t, filepath.Join(job, "in", "src", "main.go")); got != "package main" {
		t.Fatalf("src/main.go = %q", got)
	}
	if got := readFile(t, filepath.Join(job, "in", "Makefile")); got != "all:\n" {
		t.Fatalf("Makefile = %q", got)
	}
}

func TestExtractKeepsEmptyDirectories(t *testing.T) {
	dir := t.TempDir()
	pkg := writePackage(t, dir, "pkg.tar.gz", []tarItem{
		{hdr: &tar.Header{Name: "assets/", Typeflag: tar.TypeDir, Mode: 0o755}},
		{hdr: &tar.Header{Name: "assets/icons/", Typeflag: tar.TypeDir, Mode: 0o755}},
		{hdr: &tar.Header{Name: "assets/icons/keep.txt", Typeflag: tar.TypeReg, Mode: 0o644, Size: 1}, body: []byte("k")},
	})

	job := filepath.Join(dir, "jobs", "emptydir")
	if err := Extract(job, pkg, DefaultConfig); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	for _, rel := range []string{"assets", "assets/icons"} {
		info, err := os.Stat(filepath.Join(job, "in", rel))
		if err != nil {
			t.Fatalf("stat %q: %v", rel, err)
		}
		if !info.IsDir() {
			t.Fatalf("%q is not a directory", rel)
		}
	}
}

func TestExtractSymlinkInsidePackage(t *testing.T) {
	dir := t.TempDir()
	job := filepath.Join(dir, "jobs", "symlink")
	probe := filepath.Join(dir, "probe-link")
	if err := os.Symlink("release.txt", probe); err != nil {
		t.Skipf("symlink creation is not available: %v", err)
	}
	os.Remove(probe)

	pkg := writePackage(t, dir, "pkg.tar.gz", []tarItem{
		{hdr: &tar.Header{Name: "release.txt", Typeflag: tar.TypeReg, Mode: 0o644, Size: 3}, body: []byte("v1\n")},
		{hdr: &tar.Header{Name: "current", Typeflag: tar.TypeSymlink, Linkname: "release.txt", Mode: 0o777}},
	})

	if err := Extract(job, pkg, DefaultConfig); err != nil {
		t.Fatalf("Extract: %v", err)
	}

	link := filepath.Join(job, "in", "current")
	info, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("lstat: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("current is not a symlink")
	}
	target, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if target != "release.txt" {
		t.Fatalf("link target = %q", target)
	}
	if got := readFile(t, link); got != "v1\n" {
		t.Fatalf("through link = %q", got)
	}
}

func TestExtractZipPackage(t *testing.T) {
	dir := t.TempDir()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("lib/util.go")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("package lib\n")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	pkg := filepath.Join(dir, "pkg.zip")
	if err := os.WriteFile(pkg, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	job := filepath.Join(dir, "jobs", "zip")
	if err := Extract(job, pkg, DefaultConfig); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if got := readFile(t, filepath.Join(job, "in", "lib", "util.go")); got != "package lib\n" {
		t.Fatalf("lib/util.go = %q", got)
	}
}

func TestExtractConsumesAllEntries(t *testing.T) {
	dir := t.TempDir()
	var items []tarItem
	items = append(items, tarItem{hdr: &tar.Header{Name: "a.txt", Typeflag: tar.TypeReg, Mode: 0o644, Size: 2}, body: []byte("a\n")})
	for i := 0; i < 5; i++ {
		name := filepath.Join("nested", string(rune('a'+i))+".txt")
		items = append(items, tarItem{hdr: &tar.Header{Name: filepath.ToSlash(name), Typeflag: tar.TypeReg, Mode: 0o644, Size: 3}, body: []byte("x\ny")})
	}
	pkg := writePackage(t, dir, "pkg.tar.gz", items)

	job := filepath.Join(dir, "jobs", "many")
	if err := Extract(job, pkg, DefaultConfig); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	for i := 0; i < 5; i++ {
		name := filepath.Join(job, "in", "nested", string(rune('a'+i))+".txt")
		if got := readFile(t, name); got != "x\ny" {
			t.Fatalf("%s = %q", name, got)
		}
	}
}

func TestExtractUnreadablePackage(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "broken.tar.gz")
	if err := os.WriteFile(pkg, []byte("not an archive"), 0o644); err != nil {
		t.Fatal(err)
	}

	job := filepath.Join(dir, "jobs", "broken")
	err := Extract(job, pkg, DefaultConfig)
	if err == nil {
		t.Fatalf("Extract succeeded on a non-archive")
	}
	if _, statErr := os.Stat(filepath.Join(job, "in")); statErr != nil {
		t.Fatalf("input directory was not prepared: %v", statErr)
	}
}
