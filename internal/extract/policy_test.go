package extract

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/pairgsb/unpackfence/internal/fault"
)

func TestExtractRejectsPathTraversal(t *testing.T) {
	dir := t.TempDir()
	pkg := writePackage(t, dir, "pkg.tar.gz", []tarItem{
		{hdr: &tar.Header{Name: "../../abcdef/in/x", Typeflag: tar.TypeReg, Mode: 0o644, Size: 3}, body: []byte("pwn")},
	})

	job := filepath.Join(dir, "jobs", "abc")
	err := Extract(job, pkg, Config{MaxRatio: 0})
	if !errors.Is(err, fault.ErrOutsideRoot) {
		t.Fatalf("Extract = %v, want outside root", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "jobs", "abcdef", "in", "x")); statErr == nil {
		t.Fatal("traversal wrote outside the job root")
	}
}

func TestExtractRejectsUnsafeNames(t *testing.T) {
	cases := []string{
		`C:\x\y`,
		"C:/x/y",
		"c:relative",
		`\\host\share\z`,
		"//host/share/z",
		"/etc/passwd",
	}
	for i, name := range cases {
		dir := t.TempDir()
		pkg := writePackage(t, dir, fmt.Sprintf("pkg%d.tar.gz", i), []tarItem{
			{hdr: &tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o644, Size: 1}, body: []byte("x")},
		})
		job := filepath.Join(dir, "jobs", "abc")
		if err := Extract(job, pkg, Config{MaxRatio: 0}); !errors.Is(err, fault.ErrUnsafeName) {
			t.Fatalf("Extract(%q) = %v, want unsafe name", name, err)
		}
	}
}

func TestExtractRejectsBackslashTraversal(t *testing.T) {
	dir := t.TempDir()
	pkg := writePackage(t, dir, "pkg.tar.gz", []tarItem{
		{hdr: &tar.Header{Name: `..\..\abcdef\in\x`, Typeflag: tar.TypeReg, Mode: 0o644, Size: 3}, body: []byte("pwn")},
	})

	job := filepath.Join(dir, "jobs", "abc")
	if err := Extract(job, pkg, Config{MaxRatio: 0}); !errors.Is(err, fault.ErrOutsideRoot) {
		t.Fatalf("Extract = %v, want outside root", err)
	}
}

func TestExtractRejectsSymlinkTargetEscape(t *testing.T) {
	cases := []string{
		"../../outside",
		`..\..\outside`,
		"/tmp/outside",
		`C:\outside`,
		"//host/share/z",
	}
	for i, target := range cases {
		dir := t.TempDir()
		outside := filepath.Join(dir, "outside")
		if err := os.MkdirAll(outside, 0o755); err != nil {
			t.Fatal(err)
		}
		linkname := target
		if target == "../../outside" || target == `..\..\outside` {
			linkname = target
		} else if target == "/tmp/outside" {
			linkname = filepath.Join(dir, "outside")
		} else if target == `C:\outside` {
			linkname = `C:\outside`
		}

		pkg := writePackage(t, dir, fmt.Sprintf("pkg%d.tar.gz", i), []tarItem{
			{hdr: &tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: linkname, Mode: 0o777}},
			{hdr: &tar.Header{Name: "link/payload", Typeflag: tar.TypeReg, Mode: 0o644, Size: 3}, body: []byte("pwn")},
		})

		job := filepath.Join(dir, "jobs", "abc")
		if err := Extract(job, pkg, Config{MaxRatio: 0}); !errors.Is(err, fault.ErrSymlinkEscape) {
			t.Fatalf("Extract(target %q) = %v, want symlink escape", target, err)
		}
		if _, statErr := os.Stat(filepath.Join(outside, "payload")); statErr == nil {
			t.Fatalf("symlink target %q wrote outside the root", target)
		}
	}
}

func TestExtractRejectsHardlinkEscape(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "secret.txt"), []byte("secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pkg := writePackage(t, dir, "pkg.tar.gz", []tarItem{
		{hdr: &tar.Header{Name: "steal", Typeflag: tar.TypeLink, Linkname: "../../../secret.txt", Mode: 0o644}},
	})

	job := filepath.Join(dir, "jobs", "abc")
	err := Extract(job, pkg, Config{MaxRatio: 0})
	if !errors.Is(err, fault.ErrHardlinkEscape) {
		t.Fatalf("Extract = %v, want hardlink escape", err)
	}
	if _, statErr := os.Stat(filepath.Join(job, "in", "steal")); statErr == nil {
		t.Fatal("escaping hardlink was created")
	}
}

func TestExtractAllowsHardlinkInsideRoot(t *testing.T) {
	dir := t.TempDir()
	pkg := writePackage(t, dir, "pkg.tar.gz", []tarItem{
		{hdr: &tar.Header{Name: "a.txt", Typeflag: tar.TypeReg, Mode: 0o644, Size: 3}, body: []byte("abc")},
		{hdr: &tar.Header{Name: "b.txt", Typeflag: tar.TypeLink, Linkname: "a.txt", Mode: 0o644}},
	})

	job := filepath.Join(dir, "jobs", "abc")
	if err := Extract(job, pkg, Config{MaxRatio: 0}); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if got := readFile(t, filepath.Join(job, "in", "b.txt")); got != "abc" {
		t.Fatalf("b.txt = %q", got)
	}
}

func TestExtractEnforcesEntryLimit(t *testing.T) {
	dir := t.TempDir()
	var items []tarItem
	for i := 0; i < 6; i++ {
		items = append(items, tarItem{
			hdr:  &tar.Header{Name: fmt.Sprintf("f%d.txt", i), Typeflag: tar.TypeReg, Mode: 0o644, Size: 1},
			body: []byte("x"),
		})
	}
	pkg := writePackage(t, dir, "pkg.tar.gz", items)

	job := filepath.Join(dir, "jobs", "abc")
	err := Extract(job, pkg, Config{MaxEntries: 2, MaxRatio: 0})
	if !errors.Is(err, fault.ErrEntryLimit) {
		t.Fatalf("Extract = %v, want entry limit", err)
	}
	matches, globErr := filepath.Glob(filepath.Join(job, "in", "f*.txt"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(matches) > 2 {
		t.Fatalf("extracted %d entries past the limit", len(matches))
	}
}

func TestExtractEnforcesActualTarBytes(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "sparse.tar")
	writeSparseTar(t, pkg, "sparse.img", 64<<20)

	job := filepath.Join(dir, "jobs", "abc")
	err := Extract(job, pkg, Config{MaxBytes: 1 << 20, MaxRatio: 0})
	if !errors.Is(err, fault.ErrBudgetExceeded) {
		t.Fatalf("Extract = %v, want budget exceeded", err)
	}
	info, statErr := os.Stat(filepath.Join(job, "in", "sparse.img"))
	if statErr != nil {
		t.Fatal(statErr)
	}
	if info.Size() > 1<<20 {
		t.Fatalf("wrote %d bytes, budget was %d", info.Size(), 1<<20)
	}
}

func TestExtractEnforcesActualPaxSparseBytes(t *testing.T) {
	dir := t.TempDir()
	pkg := filepath.Join(dir, "pax-sparse.tar")
	writePaxSparseTar(t, pkg, "sparse.img", 64<<20)

	job := filepath.Join(dir, "jobs", "abc")
	err := Extract(job, pkg, Config{MaxBytes: 1 << 20, MaxRatio: 0})
	if !errors.Is(err, fault.ErrBudgetExceeded) {
		t.Fatalf("Extract = %v, want budget exceeded", err)
	}
	info, statErr := os.Stat(filepath.Join(job, "in", "sparse.img"))
	if statErr != nil {
		t.Fatal(statErr)
	}
	if info.Size() > 1<<20 {
		t.Fatalf("wrote %d bytes, budget was %d", info.Size(), 1<<20)
	}
}

func TestExtractEnforcesActualZipBytes(t *testing.T) {
	dir := t.TempDir()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("big.bin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(bytes.Repeat([]byte{0}, 8<<20)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	raw := zeroZipUncompressedSizes(t, buf.Bytes())

	pkg := filepath.Join(dir, "pkg.zip")
	if err := os.WriteFile(pkg, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	job := filepath.Join(dir, "jobs", "abc")
	err = Extract(job, pkg, Config{MaxBytes: 1 << 20, MaxRatio: 0})
	if !errors.Is(err, fault.ErrBudgetExceeded) {
		t.Fatalf("Extract = %v, want budget exceeded", err)
	}
	info, statErr := os.Stat(filepath.Join(job, "in", "big.bin"))
	if statErr != nil {
		t.Fatal(statErr)
	}
	if info.Size() > 1<<20 {
		t.Fatalf("wrote %d bytes, budget was %d", info.Size(), 1<<20)
	}
}

func TestExtractEnforcesCompressionRatio(t *testing.T) {
	dir := t.TempDir()
	pkg := writePackage(t, dir, "pkg.tar.gz", []tarItem{
		{hdr: &tar.Header{Name: "bomb.bin", Typeflag: tar.TypeReg, Mode: 0o644, Size: 8 << 20}, body: bytes.Repeat([]byte{0}, 8<<20)},
	})

	job := filepath.Join(dir, "jobs", "abc")
	err := Extract(job, pkg, Config{MaxRatio: 20})
	if !errors.Is(err, fault.ErrRatioExceeded) {
		t.Fatalf("Extract = %v, want ratio exceeded", err)
	}
}

func TestExtractEnforcesZipCompressionRatio(t *testing.T) {
	dir := t.TempDir()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("bomb.bin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(bytes.Repeat([]byte{0}, 8<<20)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	pkg := filepath.Join(dir, "pkg.zip")
	if err := os.WriteFile(pkg, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	job := filepath.Join(dir, "jobs", "abc")
	if err := Extract(job, pkg, Config{MaxRatio: 20}); !errors.Is(err, fault.ErrRatioExceeded) {
		t.Fatalf("Extract = %v, want ratio exceeded", err)
	}
}

func TestExtractRejectsExistingIntermediateSymlinkEscape(t *testing.T) {
	dir := t.TempDir()
	job := filepath.Join(dir, "jobs", "abc")
	input := filepath.Join(job, "in")
	if err := os.MkdirAll(input, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(dir, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(input, "link")); err != nil {
		t.Skipf("symlink creation is not available: %v", err)
	}

	pkg := writePackage(t, dir, "pkg.tar.gz", []tarItem{
		{hdr: &tar.Header{Name: "link/payload", Typeflag: tar.TypeReg, Mode: 0o644, Size: 3}, body: []byte("pwn")},
	})
	if err := Extract(job, pkg, Config{MaxRatio: 0}); !errors.Is(err, fault.ErrSymlinkEscape) {
		t.Fatalf("Extract = %v, want symlink escape", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "payload")); err == nil {
		t.Fatal("wrote through an escape symlink")
	}
}

// writeSparseTar emits an old-GNU sparse entry whose stored size is zero and
// whose logical size is realSize, matching the adversarial package in the
// policy tests.
func writeSparseTar(t *testing.T, path, name string, realSize int64) {
	t.Helper()

	hdr := tarBlock(name, 'S', 0)
	copy(hdr[257:265], []byte("ustar  \x00"))
	copy(hdr[483:495], []byte(fmt.Sprintf("%011o\x00", realSize)))
	tarChecksum(hdr)

	data := append(hdr, make([]byte, 1024)...)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

// writePaxSparseTar emits a PAX-sparse entry whose stored size is zero and
// whose GNU.sparse.realsize is realSize.
func writePaxSparseTar(t *testing.T, path, name string, realSize int64) {
	t.Helper()

	records := paxRecord("GNU.sparse.realsize", fmt.Sprint(realSize)) +
		paxRecord("GNU.sparse.map", "0,0")

	paxHdr := tarBlock("PaxHeaders/"+name, 'x', int64(len(records)))
	copy(paxHdr[257:265], []byte("ustar\x0000"))
	tarChecksum(paxHdr)

	dataHdr := tarBlock(name, '0', 0)
	copy(dataHdr[257:265], []byte("ustar\x0000"))
	tarChecksum(dataHdr)

	data := append(paxHdr, []byte(records)...)
	data = append(data, make([]byte, tarPad(int64(len(records))))...)
	data = append(data, dataHdr...)
	data = append(data, make([]byte, 1024)...)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func tarBlock(name string, typ byte, size int64) []byte {
	hdr := make([]byte, 512)
	copy(hdr[0:100], name)
	copy(hdr[100:108], []byte("0000644\x00"))
	copy(hdr[108:116], []byte("0000000\x00"))
	copy(hdr[116:124], []byte("0000000\x00"))
	copy(hdr[124:136], []byte(fmt.Sprintf("%011o\x00", size)))
	copy(hdr[136:148], []byte("00000000000\x00"))
	hdr[156] = typ
	return hdr
}

func tarChecksum(hdr []byte) {
	copy(hdr[148:156], []byte("        "))
	var sum int64
	for _, b := range hdr {
		sum += int64(b)
	}
	copy(hdr[148:156], []byte(fmt.Sprintf("%06o\x00 ", sum)))
}

func tarPad(n int64) int64 {
	if r := n % 512; r != 0 {
		return 512 - r
	}
	return 0
}

func paxRecord(key, value string) string {
	payload := key + "=" + value + "\n"
	n := len(payload) + 2
	for {
		record := fmt.Sprintf("%d %s", n, payload)
		if len(record) == n {
			return record
		}
		n = len(record)
	}
}

// zeroZipUncompressedSizes clears the uncompressed-size fields in a zip's
// headers so the extractor sees a declared size of zero while the stream still
// carries real data.
func zeroZipUncompressedSizes(t *testing.T, data []byte) []byte {
	t.Helper()

	local := bytes.Index(data, []byte("PK\x03\x04"))
	if local < 0 {
		t.Fatal("zip local header not found")
	}
	binary.LittleEndian.PutUint32(data[local+22:local+26], 0)

	pos := 0
	found := false
	for {
		off := bytes.Index(data[pos:], []byte("PK\x01\x02"))
		if off < 0 {
			break
		}
		pos += off
		binary.LittleEndian.PutUint32(data[pos+24:pos+28], 0)
		found = true
		pos += 4
	}
	if !found {
		t.Fatal("zip central directory not found")
	}
	return data
}
