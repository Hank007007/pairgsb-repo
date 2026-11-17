package wire

import (
	"errors"
	"testing"
)

// testArray wraps one element in an array header.
func testArray(elem []byte) []byte {
	out := []byte{tagArray, 0x01}
	return append(out, elem...)
}

// testNestedArrays wraps inner in n single-element array layers.
func testNestedArrays(n int, inner []byte) []byte {
	for i := 0; i < n; i++ {
		inner = testArray(inner)
	}
	return inner
}

func TestDecodeDepthLimit(t *testing.T) {
	base := func(value []byte) []byte {
		return testFrame(testMap(
			testPair("id", testString("n")),
			testPair("kind", testString("k")),
			testPair("nested", value),
		))
	}

	// Six array layers below the root map open a container at depth 7, and
	// its scalar child sits at depth 8: the limit itself is still valid.
	okBuf := base(testNestedArrays(6, testString("")))
	if _, err := Decode(okBuf); err != nil {
		t.Fatalf("depth 8 container: Decode = %v, want nil", err)
	}

	// Seven array layers try to open a ninth-depth container.
	deepBuf := base(testNestedArrays(7, testString("")))
	if _, err := Decode(deepBuf); !errors.Is(err, ErrDepthExceeded) {
		t.Fatalf("depth 9 container: err = %v, want ErrDepthExceeded", err)
	}

	// The reported eleven layers (depth 12) must be rejected as well.
	elevenBuf := base(testNestedArrays(11, testString("")))
	if _, err := Decode(elevenBuf); !errors.Is(err, ErrDepthExceeded) {
		t.Fatalf("depth 12 container: err = %v, want ErrDepthExceeded", err)
	}
}

func TestDecodeDuplicateKey(t *testing.T) {
	buf := testFrame(testMap(
		testPair("id", testString("first")),
		testPair("id", testString("second")),
		testPair("kind", testString("k")),
	))
	if _, err := Decode(buf); !errors.Is(err, ErrDuplicateKey) {
		t.Fatalf("err = %v, want ErrDuplicateKey", err)
	}

	// Duplicates in a nested map are checked too.
	nested := testMap(
		testPair("a", testString("1")),
		testPair("a", testString("2")),
	)
	buf = testFrame(testMap(
		testPair("id", testString("n")),
		testPair("kind", testString("k")),
		testPair("meta", nested),
	))
	if _, err := Decode(buf); !errors.Is(err, ErrDuplicateKey) {
		t.Fatalf("nested duplicate: err = %v, want ErrDuplicateKey", err)
	}
}

func TestDecodeTrailingBytes(t *testing.T) {
	body := testMap(
		testPair("id", testString("n")),
		testPair("kind", testString("k")),
	)

	// The declared body length must cover the stray bytes so they sit inside
	// the frame body, past the fully decoded root value.
	mk := func(extra []byte) []byte {
		padded := append(append([]byte{}, body...), extra...)
		buf := append([]byte(Magic), Version)
		buf = append(buf, testUVarint(uint32(len(padded)))...)
		return append(buf, padded...)
	}

	buf := mk([]byte{0x00, 0x00, 0x00})
	if _, err := Decode(buf); !errors.Is(err, ErrTrailingBytes) {
		t.Fatalf("err = %v, want ErrTrailingBytes", err)
	}

	// A single stray byte is already enough.
	buf = mk([]byte{0x00})
	if _, err := Decode(buf); !errors.Is(err, ErrTrailingBytes) {
		t.Fatalf("single trailing byte: err = %v, want ErrTrailingBytes", err)
	}
}

func TestDecodeElementLimit(t *testing.T) {
	// An unknown field still participates in the frame-wide value count.
	// Root(1) + two key/value pairs(4) + "blob" value and its key(2) = 7,
	// so 4089 array elements total exactly MaxElements and must succeed.
	build := func(elements uint32) []byte {
		arr := []byte{tagArray}
		arr = append(arr, testUVarint(elements)...)
		for i := uint32(0); i < elements; i++ {
			arr = append(arr, tagString, 0x00)
		}
		return testFrame(testMap(
			testPair("id", testString("n")),
			testPair("kind", testString("k")),
			testPair("blob", arr),
		))
	}

	if _, err := Decode(build(MaxElements - 7)); err != nil {
		t.Fatalf("exactly at limit: Decode = %v, want nil", err)
	}
	if _, err := Decode(build(MaxElements - 6)); !errors.Is(err, ErrTooManyElements) {
		t.Fatalf("one over limit: err = %v, want ErrTooManyElements", err)
	}

	// The reported 5000-element unknown field must be rejected.
	if _, err := Decode(build(5000)); !errors.Is(err, ErrTooManyElements) {
		t.Fatalf("5000 elements: err = %v, want ErrTooManyElements", err)
	}
}

func TestDecodeLengthExceedsBufferNoPanic(t *testing.T) {
	// A string value whose declared length runs past the frame body must
	// surface as an error, never as an unrecovered slice panic.
	body := []byte{tagMap, 0x01}
	body = append(body, testString("id")...)
	body = append(body, tagString, 0x10, 'x', 'y') // claims 16 bytes, gives 2
	buf := append(append([]byte(Magic), Version), testUVarint(uint32(len(body)))...)
	buf = append(buf, body...)

	if _, err := Decode(buf); !errors.Is(err, ErrLengthExceedsBuffer) {
		t.Fatalf("err = %v, want ErrLengthExceedsBuffer", err)
	}
}
