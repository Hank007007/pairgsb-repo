package cbor

// This file contains the encoder and the canonical-encoding checker.

import (
	"fmt"
	"math"
)

// Encode serialises v. Maps are written entry by entry in the order the Map
// records them.
func Encode(v any) ([]byte, error) {
	e := &encoder{}
	if err := e.value(v); err != nil {
		return nil, err
	}
	return e.buf, nil
}

type encoder struct {
	buf []byte
}

func (e *encoder) value(v any) error {
	switch x := v.(type) {
	case nil:
		e.buf = append(e.buf, SimpleNull)
	case bool:
		if x {
			e.buf = append(e.buf, SimpleTrue)
		} else {
			e.buf = append(e.buf, SimpleFalse)
		}
	case uint64:
		e.head(MajorUnsigned, x)
	case int64:
		if x >= 0 {
			e.head(MajorUnsigned, uint64(x))
		} else {
			e.head(MajorNegative, uint64(-(x + 1)))
		}
	case int:
		return e.value(int64(x))
	case []byte:
		e.head(MajorBytes, uint64(len(x)))
		e.buf = append(e.buf, x...)
	case string:
		e.head(MajorText, uint64(len(x)))
		e.buf = append(e.buf, x...)
	case []any:
		e.head(MajorArray, uint64(len(x)))
		for _, item := range x {
			if err := e.value(item); err != nil {
				return err
			}
		}
	case *Map:
		e.head(MajorMap, uint64(x.Len()))
		for _, k := range x.Keys() {
			value, _ := x.Get(k)
			e.head(MajorText, uint64(len(k)))
			e.buf = append(e.buf, k...)
			if err := e.value(value); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("cbor: cannot encode %T", v)
	}
	return nil
}

func (e *encoder) head(major MajorType, n uint64) {
	mt := byte(major) << 5
	switch {
	case n < 24:
		e.buf = append(e.buf, mt|byte(n))
	case n <= math.MaxUint8:
		e.buf = append(e.buf, mt|AdditionalOneByte, byte(n))
	case n <= math.MaxUint16:
		e.buf = append(e.buf, mt|AdditionalTwoBytes, byte(n>>8), byte(n))
	case n <= math.MaxUint32:
		e.buf = append(e.buf, mt|AdditionalFourByte, byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
	default:
		e.buf = append(e.buf, mt|AdditionalEightBye,
			byte(n>>56), byte(n>>48), byte(n>>40), byte(n>>32),
			byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
	}
}

func compareBytes(a, b []byte) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return len(a) - len(b)
}

// IsCanonical reports whether data is a single encoding of one data item that
// follows the ordering rules in docs/manifest-cbor.md section 3.
func IsCanonical(data []byte) bool {
	w := &canonicalWalker{data: data}
	if err := w.walk(true); err != nil {
		return false
	}
	return w.pos == len(data)
}

type canonicalWalker struct {
	data []byte
	pos  int
}

func (w *canonicalWalker) walk(top bool) error {
	if w.pos >= len(w.data) {
		return errAt(w.pos, "unexpected end of input")
	}
	start := w.pos
	initial := w.data[w.pos]
	w.pos++
	major := MajorOf(initial)
	additional := initial & 0x1f

	switch major {
	case MajorUnsigned, MajorNegative:
		n, err := w.argument(additional, start)
		if err != nil {
			return err
		}
		if n > 1<<32 {
			return errAt(start, "integer %d is outside the accepted range", n)
		}
		return nil

	case MajorBytes:
		if additional == AdditionalBreak {
			return w.walkChunks(MajorBytes)
		}
		n, err := w.argument(additional, start)
		if err != nil {
			return err
		}
		return w.skip(n, start)

	case MajorText:
		if additional == AdditionalBreak {
			return w.walkChunks(MajorText)
		}
		n, err := w.argument(additional, start)
		if err != nil {
			return err
		}
		return w.skip(n, start)

	case MajorArray:
		if additional == AdditionalBreak {
			return w.walkItems(false)
		}
		n, err := w.argument(additional, start)
		if err != nil {
			return err
		}
		for i := uint64(0); i < n; i++ {
			if err := w.walk(false); err != nil {
				return err
			}
		}
		return nil

	case MajorMap:
		if additional == AdditionalBreak {
			return w.walkItems(true)
		}
		n, err := w.argument(additional, start)
		if err != nil {
			return err
		}
		return w.mapEntries(n, top)

	case MajorTag:
		return errAt(start, "tagged values are not supported")

	case MajorSimple:
		switch additional {
		case 20, 21, 22:
			return nil
		default:
			return errAt(start, "unsupported simple value %d", additional)
		}
	}

	return errAt(start, "unsupported major type %d", major)
}

func (w *canonicalWalker) mapEntries(n uint64, top bool) error {
	var prev []byte
	for i := uint64(0); i < n; i++ {
		keyStart := w.pos
		if err := w.walk(false); err != nil {
			return err
		}
		if top {
			key := w.data[keyStart:w.pos]
			if prev != nil && compareBytes(prev, key) > 0 {
				return errAt(keyStart, "map keys are not in canonical order")
			}
			prev = key
		}
		if err := w.walk(false); err != nil {
			return err
		}
	}
	return nil
}

func (w *canonicalWalker) walkChunks(want MajorType) error {
	for {
		if w.pos >= len(w.data) {
			return errAt(w.pos, "unterminated string")
		}
		if w.data[w.pos] == MarkerBreak {
			w.pos++
			return nil
		}
		if MajorOf(w.data[w.pos]) != want {
			return errAt(w.pos, "unexpected chunk type")
		}
		if err := w.walk(false); err != nil {
			return err
		}
	}
}

// walkItems walks the entries of an indefinite-length array, or the key/value
// pairs of an indefinite-length map when pairs is true.
func (w *canonicalWalker) walkItems(pairs bool) error {
	for {
		if w.pos >= len(w.data) {
			return errAt(w.pos, "unterminated container")
		}
		if w.data[w.pos] == MarkerBreak {
			w.pos++
			return nil
		}
		if err := w.walk(false); err != nil {
			return err
		}
		if pairs {
			if err := w.walk(false); err != nil {
				return err
			}
		}
	}
}

func (w *canonicalWalker) argument(additional byte, start int) (uint64, error) {
	switch {
	case additional < AdditionalOneByte:
		return uint64(additional), nil
	case additional == AdditionalOneByte:
		raw, err := w.take(1, start)
		if err != nil {
			return 0, err
		}
		return uint64(raw[0]), nil
	case additional == AdditionalTwoBytes:
		raw, err := w.take(2, start)
		if err != nil {
			return 0, err
		}
		return uint64(raw[0])<<8 | uint64(raw[1]), nil
	case additional == AdditionalFourByte:
		raw, err := w.take(4, start)
		if err != nil {
			return 0, err
		}
		return uint64(raw[0])<<24 | uint64(raw[1])<<16 | uint64(raw[2])<<8 | uint64(raw[3]), nil
	case additional == AdditionalEightBye:
		raw, err := w.take(8, start)
		if err != nil {
			return 0, err
		}
		var n uint64
		for _, b := range raw {
			n = n<<8 | uint64(b)
		}
		return n, nil
	default:
		return 0, errAt(start, "reserved additional information %d", additional)
	}
}

func (w *canonicalWalker) take(n int, start int) ([]byte, error) {
	if n > len(w.data)-w.pos {
		return nil, errAt(start, "declared length %d exceeds remaining input", n)
	}
	raw := w.data[w.pos : w.pos+n]
	w.pos += n
	return raw, nil
}

func (w *canonicalWalker) skip(n uint64, start int) error {
	if n > uint64(len(w.data)-w.pos) {
		return errAt(start, "declared length %d exceeds remaining input", n)
	}
	w.pos += int(n)
	return nil
}
