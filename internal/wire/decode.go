package wire

import "unicode/utf8"

// Value tags used inside a frame body.
const (
	tagString byte = 0x01
	tagBytes  byte = 0x02
	tagArray  byte = 0x03
	tagMap    byte = 0x04
)

// decoder holds the per-frame decoding state.
type decoder struct {
	r         *Reader
	elemCount int
}

// Decode parses one frame from buf and returns its decoded form.
//
// The returned Frame's Payload aliases buf.
func Decode(buf []byte) (*Frame, error) {
	r := NewReader(buf)

	var magic [4]byte
	for i := range magic {
		b, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		magic[i] = b
	}
	if string(magic[:]) != Magic {
		return nil, ErrInvalidMagic
	}

	version, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	if version != Version {
		return nil, ErrUnsupportedVersion
	}

	bodyLen, err := r.ReadUVarint()
	if err != nil {
		return nil, err
	}
	if bodyLen > MaxFrameSize {
		return nil, ErrFrameTooLarge
	}
	if int(bodyLen) > r.Remaining() {
		return nil, ErrLengthExceedsBuffer
	}

	body := buf[r.Pos() : r.Pos()+int(bodyLen)]
	d := &decoder{r: NewReader(body), elemCount: 1}

	root, err := d.decodeValue(1)
	if err != nil {
		return nil, err
	}
	if d.elemCount > MaxElements {
		return nil, ErrTooManyElements
	}

	fields, ok := root.(map[string]any)
	if !ok {
		return nil, ErrInvalidField
	}

	frame := &Frame{Meta: map[string]string{}}

	idVal, ok := fields["id"]
	if !ok {
		return nil, ErrInvalidField
	}
	if frame.ID, ok = idVal.(string); !ok {
		return nil, ErrInvalidField
	}

	kindVal, ok := fields["kind"]
	if !ok {
		return nil, ErrInvalidField
	}
	if frame.Kind, ok = kindVal.(string); !ok {
		return nil, ErrInvalidField
	}

	if metaVal, ok := fields["meta"]; ok {
		meta, ok := metaVal.(map[string]any)
		if !ok {
			return nil, ErrInvalidField
		}
		for k, v := range meta {
			s, ok := v.(string)
			if !ok {
				return nil, ErrInvalidField
			}
			frame.Meta[k] = s
		}
	}

	if payloadVal, ok := fields["payload"]; ok {
		payload, ok := payloadVal.([]byte)
		if !ok {
			return nil, ErrInvalidField
		}
		frame.Payload = payload
	}

	return frame, nil
}

// decodeValue consumes one tagged value at the given container depth.
func (d *decoder) decodeValue(depth int) (any, error) {
	tag, err := d.r.ReadByte()
	if err != nil {
		return nil, err
	}

	switch tag {
	case tagString:
		return d.decodeString()
	case tagBytes:
		return d.r.ReadBytes()
	case tagArray, tagMap:
		return d.decodeContainer(tag, depth)
	default:
		return nil, ErrUnknownTag
	}
}

// decodeContainer consumes an array or map header and its elements.
func (d *decoder) decodeContainer(tag byte, depth int) (any, error) {
	count, err := d.r.ReadUVarint()
	if err != nil {
		return nil, err
	}

	if tag == tagMap && depth == 1 {
		d.elemCount += 2 * int(count)
	}

	if tag == tagArray {
		return d.decodeArray(count, depth)
	}
	return d.decodeMap(count, depth)
}

// decodeArray consumes count values and returns them in order.
func (d *decoder) decodeArray(count uint32, depth int) ([]any, error) {
	out := make([]any, 0)
	for i := uint32(0); i < count; i++ {
		elem, err := d.decodeValue(depth)
		if err != nil {
			return nil, err
		}
		out = append(out, elem)
	}
	return out, nil
}

// decodeMap consumes count key/value pairs. Keys must decode to strings.
func (d *decoder) decodeMap(count uint32, depth int) (map[string]any, error) {
	out := make(map[string]any)
	for i := uint32(0); i < count; i++ {
		keyVal, err := d.decodeValue(depth)
		if err != nil {
			return nil, err
		}
		key, ok := keyVal.(string)
		if !ok {
			return nil, ErrInvalidMapKey
		}
		val, err := d.decodeValue(depth)
		if err != nil {
			return nil, err
		}
		out[key] = val
	}
	return out, nil
}

// decodeString consumes a length-prefixed string and checks it is UTF-8.
func (d *decoder) decodeString() (string, error) {
	b, err := d.r.ReadBytes()
	if err != nil {
		return "", err
	}
	if !utf8.Valid(b) {
		return "", ErrInvalidUTF8
	}
	return string(b), nil
}
