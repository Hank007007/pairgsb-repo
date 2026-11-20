package wire

import (
	"errors"

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
)

// Label classifies the first byte of a label encoding.
func Label(b byte) LabelKind {
	if b&0xC0 == 0xC0 {
		return LabelPointer
	}
	if b == 0 {
		return LabelEnd
	}
	return LabelData
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

// ReadName decodes a possibly compressed name starting at off in msg. It
// returns the name and the offset immediately after its encoding in msg.
func ReadName(msg []byte, off int, b *limits.Budget) (Name, int, error) {
	var labels []string
	next := -1

	for {
		if off < 0 || off >= len(msg) {
			return Name{}, off, ErrTruncated
		}

		first := msg[off]
		switch Label(first) {
		case LabelEnd:
			off++
			if next < 0 {
				next = off
			}
			return Name{Labels: labels}, next, nil

		case LabelPointer:
			if off+1 >= len(msg) {
				return Name{}, off, ErrTruncated
			}
			ptr := int(first&0x3F)<<8 | int(msg[off+1])
			if b != nil {
				b.Steps(1)
			}
			if next < 0 {
				next = off + 2
			}
			off = ptr

		default:
			l := int(first)
			if off+1+l > len(msg) {
				return Name{}, off, ErrTruncated
			}
			labels = append(labels, string(msg[off+1:off+1+l]))
			off += 1 + l
		}
	}
}
