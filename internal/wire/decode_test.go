package wire

import (
	"bytes"
	"reflect"
	"testing"
)

// testFrame assembles a frame from raw body parts.
func testFrame(body ...[]byte) []byte {
	full := bytes.Join(body, nil)
	out := make([]byte, 0, len(Magic)+6+len(full))
	out = append(out, Magic...)
	out = append(out, Version)
	out = append(out, testUVarint(uint32(len(full)))...)
	return append(out, full...)
}

func testUVarint(v uint32) []byte {
	var b []byte
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

func testString(s string) []byte {
	b := []byte{tagString}
	b = append(b, testUVarint(uint32(len(s)))...)
	return append(b, s...)
}

func testBytes(b []byte) []byte {
	out := []byte{tagBytes}
	out = append(out, testUVarint(uint32(len(b)))...)
	return append(out, b...)
}

func testMap(pairs ...[]byte) []byte {
	out := []byte{tagMap}
	out = append(out, testUVarint(uint32(len(pairs)))...)
	return append(out, bytes.Join(pairs, nil)...)
}

func testPair(key string, value []byte) []byte {
	return append(testString(key), value...)
}

func TestDecodeMinimal(t *testing.T) {
	buf := testFrame(testMap(
		testPair("id", testString("node-7")),
		testPair("kind", testString("ping")),
	))

	frame, err := Decode(buf)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if frame.ID != "node-7" {
		t.Errorf("ID = %q, want %q", frame.ID, "node-7")
	}
	if frame.Kind != "ping" {
		t.Errorf("Kind = %q, want %q", frame.Kind, "ping")
	}
	if len(frame.Meta) != 0 {
		t.Errorf("Meta = %v, want empty", frame.Meta)
	}
	if frame.Payload != nil {
		t.Errorf("Payload = %v, want nil", frame.Payload)
	}
}

func TestDecodePayloadAndMeta(t *testing.T) {
	body := testMap(
		testPair("id", testString("node-1")),
		testPair("kind", testString("event")),
		testPair("meta", testMap(
			testPair("region", testString("eu-west")),
			testPair("attempt", testString("3")),
		)),
		testPair("payload", testBytes([]byte{0xde, 0xad, 0xbe, 0xef})),
	)

	frame, err := Decode(testFrame(body))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	wantMeta := map[string]string{"region": "eu-west", "attempt": "3"}
	if !reflect.DeepEqual(frame.Meta, wantMeta) {
		t.Errorf("Meta = %v, want %v", frame.Meta, wantMeta)
	}
	if !bytes.Equal(frame.Payload, []byte{0xde, 0xad, 0xbe, 0xef}) {
		t.Errorf("Payload = %x, want deadbeef", frame.Payload)
	}
}

func TestDecodePayloadAliasesBuffer(t *testing.T) {
	buf := testFrame(testMap(
		testPair("id", testString("n")),
		testPair("kind", testString("k")),
		testPair("payload", testBytes([]byte("abc"))),
	))

	frame, err := Decode(buf)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(frame.Payload) != 3 || &frame.Payload[0] != &buf[len(buf)-3] {
		t.Fatalf("Payload does not alias the frame buffer")
	}
}

func TestDecodeIgnoresUnknownFields(t *testing.T) {
	body := testMap(
		testPair("id", testString("n")),
		testPair("kind", testString("k")),
		testPair("trace", testString("0123456789abcdef")),
		testPair("tags", []byte{tagArray, 0x02, 0x01, 0x00, 0x02, 0x00}),
	)

	frame, err := Decode(testFrame(body))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if frame.ID != "n" || frame.Kind != "k" {
		t.Fatalf("got ID=%q Kind=%q", frame.ID, frame.Kind)
	}
}

func TestDecodeEmptyPayload(t *testing.T) {
	buf := testFrame(testMap(
		testPair("id", testString("")),
		testPair("kind", testString("ok")),
		testPair("payload", testBytes(nil)),
	))

	frame, err := Decode(buf)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if frame.Payload == nil || len(frame.Payload) != 0 {
		t.Fatalf("Payload = %v, want empty non-nil slice", frame.Payload)
	}
}

func TestDecodeMultiByteLength(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), 300)
	buf := testFrame(testMap(
		testPair("id", testString("n")),
		testPair("kind", testString("k")),
		testPair("payload", testBytes(payload)),
	))

	frame, err := Decode(buf)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !bytes.Equal(frame.Payload, payload) {
		t.Fatalf("Payload mismatch")
	}
}

func TestDecodeUTF8Values(t *testing.T) {
	buf := testFrame(testMap(
		testPair("id", testString("节点-α")),
		testPair("kind", testString("心跳")),
	))

	frame, err := Decode(buf)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if frame.ID != "节点-α" || frame.Kind != "心跳" {
		t.Fatalf("got ID=%q Kind=%q", frame.ID, frame.Kind)
	}
}
