package wire

import "encoding/binary"

// Reader is a cursor over a frame buffer owned by the caller. It never
// retains or copies the buffer; decoded byte slices alias it.
type Reader struct {
	buf []byte
	pos int
}

// NewReader returns a Reader positioned at the start of buf.
func NewReader(buf []byte) *Reader {
	return &Reader{buf: buf}
}

// Remaining reports how many bytes have not been consumed yet.
func (r *Reader) Remaining() int {
	return len(r.buf) - r.pos
}

// Pos reports the current cursor offset.
func (r *Reader) Pos() int {
	return r.pos
}

// ReadByte consumes and returns a single byte.
func (r *Reader) ReadByte() (byte, error) {
	if r.Remaining() < 1 {
		return 0, ErrUnexpectedEOF
	}
	b := r.buf[r.pos]
	r.pos++
	return b, nil
}

// ReadUVarint consumes an unsigned LEB128 encoded u32 from the frame buffer.
func (r *Reader) ReadUVarint() (uint32, error) {
	var value uint32
	var shift uint
	for i := 0; i < binary.MaxVarintLen64; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		value |= uint32(b&0x7f) << shift
		if b&0x80 == 0 {
			return value, nil
		}
		shift += 7
	}
	return 0, ErrVarintOverflow
}

// ReadBytes consumes a length-prefixed byte run and returns a slice aliasing
// the frame buffer.
func (r *Reader) ReadBytes() ([]byte, error) {
	n, err := r.ReadUVarint()
	if err != nil {
		return nil, err
	}
	b := r.buf[r.pos : r.pos+int(n)]
	r.pos += int(n)
	return b, nil
}

// ReadString consumes a length-prefixed string.
func (r *Reader) ReadString() (string, error) {
	b, err := r.ReadBytes()
	if err != nil {
		return "", err
	}
	return string(b), nil
}
