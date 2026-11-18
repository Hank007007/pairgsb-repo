package archive

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

const tarBlockSize = 512

// tarReader reads entries from a tar stream. The stream may be a plain tar
// file or the decompressed body of a gzip container.
type tarReader struct {
	src      io.Reader
	closer   io.Closer
	label    string
	consumed *int64
	cur      *tarSection
}

func newTarReader(src io.Reader, closer io.Closer, label string, consumed *int64) *tarReader {
	return &tarReader{src: src, closer: closer, label: label, consumed: consumed}
}

// tarSection is the data area of the entry currently being read, plus the
// padding that follows it. Everything in the section is consumed before the
// next header is read, whether or not the caller drained the entry body.
type tarSection struct {
	src  io.Reader
	left int64
	pad  int64
}

func (s *tarSection) Read(p []byte) (int, error) {
	if s.left <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > s.left {
		p = p[:s.left]
	}
	n, err := s.src.Read(p)
	s.left -= int64(n)
	if err == io.EOF && s.left > 0 {
		err = io.ErrUnexpectedEOF
	}
	return n, err
}

func (s *tarSection) drain() error {
	if _, err := io.Copy(io.Discard, s); err != nil {
		return err
	}
	if s.pad > 0 {
		if _, err := io.CopyN(io.Discard, s.src, s.pad); err != nil {
			return err
		}
		s.pad = 0
	}
	return nil
}

func (t *tarReader) Close() error {
	if t.closer != nil {
		return t.closer.Close()
	}
	return nil
}

func (t *tarReader) CompressedConsumed() int64 {
	if t.consumed == nil {
		return 0
	}
	return *t.consumed
}

func (t *tarReader) Next() (*Entry, error) {
	if t.cur != nil {
		if err := t.cur.drain(); err != nil {
			return nil, fmt.Errorf("archive: skip entry data: %w", err)
		}
		t.cur = nil
	}
	return t.readLoop()
}

// readLoop folds tar metadata headers into the entry they describe and maps
// each data header onto an Entry.
func (t *tarReader) readLoop() (*Entry, error) {
	var longName, longLink string
	pax := paxRecords{}
	var meta []paxRecords

	for {
		blk := make([]byte, tarBlockSize)
		if _, err := io.ReadFull(t.src, blk); err != nil {
			if err == io.EOF {
				return nil, io.EOF
			}
			if err == io.ErrUnexpectedEOF {
				return nil, fmt.Errorf("archive: truncated tar header")
			}
			return nil, err
		}
		if isZeroBlock(blk) {
			return nil, io.EOF
		}
		if !checksumValid(blk) {
			return nil, fmt.Errorf("archive: tar header checksum mismatch")
		}
		rawSize, err := parseTarNumber(blk[124:136])
		if err != nil {
			return nil, fmt.Errorf("archive: tar size field: %w", err)
		}

		var typ byte
		switch blk[156] {
		case 'x', 'X':
			data, err := t.readMeta(rawSize)
			if err != nil {
				return nil, err
			}
			recs, err := parsePAXRecords(data)
			if err != nil {
				return nil, err
			}
			meta = append(meta, recs)
			pax.merge(recs)
			continue
		case 'g':
			if _, err := t.readMeta(rawSize); err != nil {
				return nil, err
			}
			continue
		case 'L':
			data, err := t.readMeta(rawSize)
			if err != nil {
				return nil, err
			}
			longName = trimCString(data)
			continue
		case 'K':
			data, err := t.readMeta(rawSize)
			if err != nil {
				return nil, err
			}
			longLink = trimCString(data)
			continue
		default:
			typ = blk[156]
		}

		entry, err := t.entryFromBlock(blk, rawSize, pax, lastMeta(meta), longName, longLink)
		if err != nil {
			return nil, err
		}

		switch typ {
		case '0', 0, '7':
			entry.Type = TypeFile
			if entry.sparse != nil {
				src, err := newSparseReader(t.cur, entry.sparseTo, entry.sparse)
				if err != nil {
					return nil, err
				}
				entry.body = src
			}
		case 'S':
			entry.Type = TypeFile
			src, err := newSparseReader(t.cur, entry.sparseTo, entry.sparse)
			if err != nil {
				return nil, err
			}
			entry.body = src
		case '1':
			entry.Type = TypeHardlink
		case '2':
			entry.Type = TypeSymlink
		case '3', '4':
			entry.Type = TypeDevice
		case '5':
			entry.Type = TypeDir
		case '6':
			entry.Type = TypeFifo
		default:
			entry.Type = TypeOther
		}

		if entry.sparse != nil && entry.Type != TypeFile {
			return nil, fmt.Errorf("archive: sparse layout on non-file entry %q", entry.Name)
		}
		return entry, nil
	}
}

// readMeta consumes the body and padding of a metadata header.
func (t *tarReader) readMeta(size int64) ([]byte, error) {
	if size < 0 {
		return nil, fmt.Errorf("archive: negative metadata size")
	}
	data := make([]byte, size)
	if _, err := io.ReadFull(t.src, data); err != nil {
		return nil, fmt.Errorf("archive: short tar metadata: %w", err)
	}
	if pad := padLen(size); pad > 0 {
		if _, err := io.CopyN(io.Discard, t.src, pad); err != nil {
			return nil, err
		}
	}
	return data, nil
}

// lastMeta returns the most recent extended header, or an empty record set
// when the entry carried none. Sparse descriptors are read from here rather
// than from the merged set so that they do not leak between entries.
func lastMeta(meta []paxRecords) paxRecords {
	if len(meta) == 0 {
		return paxRecords{}
	}
	return meta[len(meta)-1]
}

// entryFromBlock builds the naming and sizing part of an entry from a data
// header. The entry type is filled in by the caller.
func (t *tarReader) entryFromBlock(blk []byte, rawSize int64, pax, meta paxRecords, longName, longLink string) (*Entry, error) {
	name := trimCString(blk[0:100])
	if isPOSIXUstar(blk) {
		if prefix := trimCString(blk[345:500]); prefix != "" {
			name = prefix + "/" + name
		}
	}
	link := trimCString(blk[157:257])
	if longName != "" {
		name = longName
	}
	if longLink != "" {
		link = longLink
	}
	if v, ok := pax.last("path"); ok {
		name = v
	}
	if v, ok := pax.last("linkpath"); ok {
		link = v
	}

	entry := &Entry{
		Name:         name,
		Linkname:     link,
		Mode:         os.FileMode(parseTarMode(blk[100:108])),
		DeclaredSize: rawSize,
		Format:       t.label,
	}

	storedLen := rawSize
	if v, ok := pax.last("size"); ok {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("archive: bad pax size for %q", name)
		}
		storedLen = n
	}

	// Header-only entries carry no data area in the stream.
	switch blk[156] {
	case '0', 0, '7', 'S':
	default:
		storedLen = 0
		entry.DeclaredSize = 0
	}
	if storedLen < 0 {
		return nil, fmt.Errorf("archive: negative size for %q", name)
	}

	sec := &tarSection{src: t.src, left: storedLen, pad: padLen(storedLen)}
	t.cur = sec
	entry.body = sec

	realSize, segs, sparse, err := sparseLayout(blk[156], blk, meta, t.src)
	if err != nil {
		return nil, err
	}
	if sparse {
		entry.sparseTo = realSize
		entry.sparse = segs
	}
	return entry, nil
}

// sparseLayout reports the logical size and data segments of a sparse entry.
func sparseLayout(typ byte, blk []byte, pax paxRecords, src io.Reader) (int64, []sparseSeg, bool, error) {
	if typ == 'S' {
		realSize, segs, err := readGNUSparseMap(blk, src)
		return realSize, segs, true, err
	}

	realText, ok := pax.last("GNU.sparse.realsize")
	if !ok {
		realText, ok = pax.last("GNU.sparse.size")
	}
	mapText, hasMap := pax.last("GNU.sparse.map")
	offsets := pax.all("GNU.sparse.offset")
	lengths := pax.all("GNU.sparse.numbytes")
	if !ok && !hasMap && len(offsets) == 0 {
		return 0, nil, false, nil
	}

	realSize := int64(0)
	if ok {
		n, err := strconv.ParseInt(realText, 10, 64)
		if err != nil {
			return 0, nil, false, fmt.Errorf("archive: bad sparse size")
		}
		realSize = n
	}

	var segs []sparseSeg
	if hasMap {
		parts := strings.Split(mapText, ",")
		if len(parts)%2 != 0 {
			return 0, nil, false, fmt.Errorf("archive: bad sparse map")
		}
		for i := 0; i < len(parts); i += 2 {
			off, err1 := strconv.ParseInt(strings.TrimSpace(parts[i]), 10, 64)
			ln, err2 := strconv.ParseInt(strings.TrimSpace(parts[i+1]), 10, 64)
			if err1 != nil || err2 != nil {
				return 0, nil, false, fmt.Errorf("archive: bad sparse map")
			}
			segs = append(segs, sparseSeg{Offset: off, Length: ln})
		}
	}
	if len(offsets) != len(lengths) {
		return 0, nil, false, fmt.Errorf("archive: sparse map entries do not pair up")
	}
	for i := range offsets {
		off, err1 := strconv.ParseInt(offsets[i], 10, 64)
		ln, err2 := strconv.ParseInt(lengths[i], 10, 64)
		if err1 != nil || err2 != nil {
			return 0, nil, false, fmt.Errorf("archive: bad sparse map")
		}
		segs = append(segs, sparseSeg{Offset: off, Length: ln})
	}
	if realSize == 0 {
		for _, s := range segs {
			if end := s.Offset + s.Length; end > realSize {
				realSize = end
			}
		}
	}
	return realSize, segs, true, nil
}

func readGNUSparseMap(blk []byte, src io.Reader) (int64, []sparseSeg, error) {
	realSize, err := parseTarNumber(blk[483:495])
	if err != nil {
		return 0, nil, fmt.Errorf("archive: sparse real size: %w", err)
	}
	var segs []sparseSeg
	for i := 0; i < 4; i++ {
		base := 386 + i*24
		if blk[base] == 0 {
			break
		}
		off, err := parseTarNumber(blk[base : base+12])
		if err != nil {
			return 0, nil, err
		}
		ln, err := parseTarNumber(blk[base+12 : base+24])
		if err != nil {
			return 0, nil, err
		}
		segs = append(segs, sparseSeg{Offset: off, Length: ln})
	}
	extended := blk[482] != 0
	for extended {
		next := make([]byte, tarBlockSize)
		if _, err := io.ReadFull(src, next); err != nil {
			return 0, nil, fmt.Errorf("archive: short sparse extension: %w", err)
		}
		for i := 0; i < 21; i++ {
			base := i * 24
			if next[base] == 0 {
				break
			}
			off, err := parseTarNumber(next[base : base+12])
			if err != nil {
				return 0, nil, err
			}
			ln, err := parseTarNumber(next[base+12 : base+24])
			if err != nil {
				return 0, nil, err
			}
			segs = append(segs, sparseSeg{Offset: off, Length: ln})
		}
		extended = next[504] != 0
	}
	return realSize, segs, nil
}

type sparseSeg struct {
	Offset int64
	Length int64
}

// newSparseReader builds a stream of realSize bytes from the stored data
// segments; the gaps between segments read as zeroes.
func newSparseReader(src io.Reader, realSize int64, segs []sparseSeg) (io.Reader, error) {
	if realSize < 0 {
		return nil, fmt.Errorf("archive: negative sparse size")
	}
	sort.Slice(segs, func(i, j int) bool { return segs[i].Offset < segs[j].Offset })
	var readers []io.Reader
	pos := int64(0)
	for _, s := range segs {
		if s.Offset < pos || s.Length < 0 || s.Offset+s.Length > realSize {
			return nil, fmt.Errorf("archive: sparse segments overlap or exceed the logical size")
		}
		if s.Offset > pos {
			readers = append(readers, &zeroReader{n: s.Offset - pos})
		}
		if s.Length > 0 {
			readers = append(readers, io.LimitReader(src, s.Length))
		}
		pos = s.Offset + s.Length
	}
	if pos < realSize {
		readers = append(readers, &zeroReader{n: realSize - pos})
	}
	return io.MultiReader(readers...), nil
}

type zeroReader struct {
	n int64
}

func (z *zeroReader) Read(p []byte) (int, error) {
	if z.n <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > z.n {
		p = p[:z.n]
	}
	for i := range p {
		p[i] = 0
	}
	z.n -= int64(len(p))
	return len(p), nil
}

type paxRecords map[string][]string

func (p paxRecords) merge(other paxRecords) {
	for k, vs := range other {
		p[k] = append(p[k], vs...)
	}
}

func (p paxRecords) last(key string) (string, bool) {
	vs, ok := p[key]
	if !ok || len(vs) == 0 {
		return "", false
	}
	return vs[len(vs)-1], true
}

func (p paxRecords) all(key string) []string {
	return p[key]
}

func parsePAXRecords(data []byte) (paxRecords, error) {
	recs := paxRecords{}
	for len(data) > 0 {
		sp := bytes.IndexByte(data, ' ')
		if sp < 0 {
			return nil, fmt.Errorf("archive: malformed pax record")
		}
		n, err := strconv.Atoi(string(data[:sp]))
		if err != nil || n <= sp+1 || n > len(data) {
			return nil, fmt.Errorf("archive: malformed pax record length")
		}
		rec := data[sp+1 : n]
		data = data[n:]
		rec = bytes.TrimSuffix(rec, []byte("\n"))
		eq := bytes.IndexByte(rec, '=')
		if eq < 0 {
			return nil, fmt.Errorf("archive: malformed pax record")
		}
		key := string(rec[:eq])
		recs[key] = append(recs[key], string(rec[eq+1:]))
	}
	return recs, nil
}

func isZeroBlock(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return false
		}
	}
	return true
}

func padLen(n int64) int64 {
	if r := n % tarBlockSize; r != 0 {
		return tarBlockSize - r
	}
	return 0
}

func trimCString(b []byte) string {
	if i := bytes.IndexByte(b, 0); i >= 0 {
		b = b[:i]
	}
	return string(b)
}

// isPOSIXUstar reports whether the header uses the POSIX ustar layout, which
// stores a path prefix at offset 345. GNU headers reuse that area for other
// fields and must not be read as a prefix.
func isPOSIXUstar(blk []byte) bool {
	return string(blk[257:263]) == "ustar\x00" && string(blk[263:265]) == "00"
}

func checksumValid(blk []byte) bool {
	stored, err := parseTarNumber(blk[148:156])
	if err != nil {
		return false
	}
	var unsigned, signed int64
	for i, c := range blk {
		if i >= 148 && i < 156 {
			c = ' '
		}
		unsigned += int64(c)
		signed += int64(int8(c))
	}
	return stored == unsigned || stored == signed
}

func parseTarMode(b []byte) uint32 {
	v, err := parseTarNumber(b)
	if err != nil {
		return 0
	}
	return uint32(v) & 0o7777
}

func parseTarNumber(b []byte) (int64, error) {
	if len(b) == 0 {
		return 0, nil
	}
	if b[0]&0x80 != 0 {
		// Base-256: the high bit marks the encoding and the next bit is the
		// sign. Negative values are stored as the complement of the magnitude.
		var inv byte
		if b[0]&0x40 != 0 {
			inv = 0xff
		}
		var x uint64
		for i, c := range b {
			c ^= inv
			if i == 0 {
				c &= 0x7f
			}
			x = x<<8 | uint64(c)
		}
		if inv == 0xff {
			return ^int64(x), nil
		}
		if x>>63 != 0 {
			return 0, fmt.Errorf("numeric field overflow")
		}
		return int64(x), nil
	}
	s := strings.Trim(string(b), " \x00")
	if s == "" {
		return 0, nil
	}
	v, err := strconv.ParseInt(s, 8, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid numeric field %q", s)
	}
	return v, nil
}
