package wire

import (
	"errors"
	"fmt"

	"ptrloop/internal/limits"
)

// Name is a decoded domain name. Labels holds each label as a raw byte string,
// in wire order (leftmost label first). The root name has no labels.
type Name struct {
	Labels []string
}

// Errors returned by the wire decoder.
var (
	ErrTruncated = errors.New("wire: truncated message")
	ErrName      = errors.New("wire: malformed domain name")
	ErrHeader    = errors.New("wire: malformed header")
	ErrRecord    = errors.New("wire: malformed resource record")
)

// LabelKind is the interpretation of the first byte at a name offset.
type LabelKind int

const (
	// LabelEnd is the zero terminator.
	LabelEnd LabelKind = iota
	// LabelPointer is an RFC 1035 compression pointer.
	LabelPointer
	// LabelData is a length-prefixed label.
	LabelData
	// LabelReserved is an encoding whose high two bits are 01 or 10.
	LabelReserved
)

// Label classifies the first byte of a label encoding.
func Label(b byte) LabelKind {
	switch b & 0xC0 {
	case 0xC0:
		return LabelPointer
	case 0x00:
		if b == 0 {
			return LabelEnd
		}
		return LabelData
	default:
		return LabelReserved
	}
}

// String renders the name in presentation form with a single trailing dot.
func (n Name) String() string {
	s := ""
	for _, l := range n.Labels {
		s += l + "."
	}
	if s == "" {
		return "."
	}
	return s
}

// WireLength reports the uncompressed wire length of the name.
func (n Name) WireLength() int {
	total := 1
	for _, l := range n.Labels {
		total += 1 + len(l)
	}
	return total
}

// nameTracker records where label encodings live in a message. ReadName uses
// it to enforce that a compression pointer can only target a label start, not
// an arbitrary byte or the middle of another label.
type nameTracker struct {
	starts map[int]struct{}
	spans  [][2]int
	strict bool
}

func newNameTracker() *nameTracker {
	return &nameTracker{starts: make(map[int]struct{})}
}

func (t *nameTracker) addStart(off int) {
	t.starts[off] = struct{}{}
}

func (t *nameTracker) addSpan(start, end int) {
	if end > start {
		t.spans = append(t.spans, [2]int{start, end})
	}
}

func (t *nameTracker) validTarget(off int) bool {
	if _, ok := t.starts[off]; ok {
		return true
	}
	if t.strict {
		return false
	}
	for _, sp := range t.spans {
		if off >= sp[0] && off < sp[1] {
			return false
		}
	}
	return true
}

// ReadName decodes a possibly compressed name starting at off in msg. It
// returns the name and the offset immediately after its encoding in msg.
func ReadName(msg []byte, off int, b *limits.Budget) (Name, int, error) {
	t := newNameTracker()
	return readName(msg, off, b, t, true)
}

// readName is the decoded implementation shared by Parse and ReadName. When
// allowCompression is false a pointer in the encoding is rejected.
func readName(msg []byte, off int, b *limits.Budget, t *nameTracker, allowCompression bool) (Name, int, error) {
	var labels []string
	next := -1
	wireLen := 1
	visited := make(map[int]struct{})

	for {
		if off < 0 || off >= len(msg) {
			return Name{}, off, ErrTruncated
		}
		if _, seen := visited[off]; seen {
			return Name{}, off, fmt.Errorf("%w: compression pointer cycle", ErrName)
		}
		visited[off] = struct{}{}

		first := msg[off]
		switch Label(first) {
		case LabelEnd:
			t.addStart(off)
			off++
			if next < 0 {
				next = off
			}
			if err := recordName(b, wireLen); err != nil {
				return Name{}, off, err
			}
			return Name{Labels: labels}, next, nil

		case LabelPointer:
			if !allowCompression {
				return Name{}, off, fmt.Errorf("%w: compression pointer is not allowed here", ErrName)
			}
			if off+1 >= len(msg) {
				return Name{}, off, ErrTruncated
			}
			ptr := int(first&0x3F)<<8 | int(msg[off+1])
			if ptr >= off {
				return Name{}, off, fmt.Errorf("%w: pointer target %d is not before %d", ErrName, ptr, off)
			}
			if ptr >= len(msg) {
				return Name{}, off, ErrTruncated
			}
			if b != nil {
				if err := b.Steps(1); err != nil {
					return Name{}, off, err
				}
			}
			if !t.validTarget(ptr) {
				return Name{}, off, fmt.Errorf("%w: pointer target %d is not a label start", ErrName, ptr)
			}
			if next < 0 {
				next = off + 2
			}
			off = ptr

		case LabelData:
			l := int(first)
			if off+1+l > len(msg) {
				return Name{}, off, ErrTruncated
			}
			content := msg[off+1 : off+1+l]
			if err := validateLabel(content); err != nil {
				return Name{}, off, err
			}
			t.addStart(off)
			t.addSpan(off+1, off+1+l)
			labels = append(labels, string(content))
			wireLen += 1 + l
			if wireLen > limits.MaxNameLength {
				if b != nil {
					_ = b.Name(wireLen)
				}
				return Name{}, off, fmt.Errorf("%w: decoded name length %d exceeds %d", ErrName, wireLen, limits.MaxNameLength)
			}
			off += 1 + l

		default:
			return Name{}, off, fmt.Errorf("%w: reserved label byte %#x", ErrName, first)
		}
	}
}

func recordName(b *limits.Budget, length int) error {
	if b == nil {
		return nil
	}
	if err := b.Name(length); err != nil {
		return fmt.Errorf("%w: %w", ErrName, err)
	}
	return nil
}

func validateLabel(label []byte) error {
	for i, c := range label {
		if i == 0 && c == '-' {
			return fmt.Errorf("%w: label starts with '-'", ErrName)
		}
		if !isLetterDigitHyphen(c) {
			return fmt.Errorf("%w: invalid label byte %#x", ErrName, c)
		}
	}
	return nil
}

func isLetterDigitHyphen(c byte) bool {
	return c >= 'A' && c <= 'Z' ||
		c >= 'a' && c <= 'z' ||
		c >= '0' && c <= '9' ||
		c == '-'
}
