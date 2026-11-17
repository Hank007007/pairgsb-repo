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
// A u32 occupies at most 5 bytes, and the fifth byte may use only its low
// four bits; any longer encoding or any bit outside the u32 range is an
// overflow rather than a silently truncated value.
func (r *Reader) ReadUVarint() (uint32, error) {
	var value uint32
	for i := 0; i < binary.MaxVarintLen32; i++ {
		b, err := r.ReadByte()
		if err != nil {
			return 0, err
		}
		if i == binary.MaxVarintLen32-1 {
			// The fifth byte carries only the top four bits of the u32.
			if b&0xf0 != 0 {
				return 0, ErrVarintOverflow
			}
			value |= uint32(b) << 28
			// A continuation bit would require a sixth byte, which has no
			// room in a u32.
			if b&0x80 != 0 {
				return 0, ErrVarintOverflow
			}
			return value, nil
		}
		value |= uint32(b&0x7f) << (7 * i)
		if b&0x80 == 0 {
			return value, nil
		}
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
	if uint64(n) > uint64(r.Remaining()) {
		return nil, ErrLengthExceedsBuffer
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
