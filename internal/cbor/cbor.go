// Package cbor implements the small CBOR subset used by OTA manifests.
//
// The decoder turns a complete byte slice into a generic tree of Go values.
// Map keys are always text strings and are represented as such. The canonical
// checker inspects the original bytes; the encoder produces canonical output.
package cbor

import "fmt"

// MajorType is the 3-bit major type carried in a CBOR initial byte.
type MajorType uint8

// CBOR major types.
const (
	MajorUnsigned MajorType = 0
	MajorNegative MajorType = 1
	MajorBytes    MajorType = 2
	MajorText     MajorType = 3
	MajorArray    MajorType = 4
	MajorMap      MajorType = 5
	MajorTag      MajorType = 6
	MajorSimple   MajorType = 7
)

// Simple values accepted by this implementation.
const (
	SimpleFalse byte = 0xf4
	SimpleTrue  byte = 0xf5
	SimpleNull  byte = 0xf6
)

// Initial-byte markers for the non-value encodings this package cares about.
const (
	AdditionalOneByte  byte = 24
	AdditionalTwoBytes byte = 25
	AdditionalFourByte byte = 26
	AdditionalEightBye byte = 27
	AdditionalBreak    byte = 0x1f

	MarkerIndefiniteBytes byte = 0x5f
	MarkerIndefiniteText  byte = 0x7f
	MarkerIndefiniteArray byte = 0x9f
	MarkerIndefiniteMap   byte = 0xbf
	MarkerBreak           byte = 0xff
)

// MaxDepth is the deepest data-item nesting accepted by the manifest format.
// The top-level item is depth 1.
const MaxDepth = 16

// MaxManifestBytes is the largest manifest, in bytes, accepted by the format.
const MaxManifestBytes = 1 << 20

// MajorOf returns the major type encoded in an initial byte.
func MajorOf(initial byte) MajorType {
	return MajorType(initial >> 5)
}

// Error describes a malformed or unsupported input at a byte offset.
type Error struct {
	Offset int
	Msg    string
}

func (e *Error) Error() string {
	return fmt.Sprintf("cbor: %s at offset %d", e.Msg, e.Offset)
}

func errAt(offset int, format string, args ...any) error {
	return &Error{Offset: offset, Msg: fmt.Sprintf(format, args...)}
}
