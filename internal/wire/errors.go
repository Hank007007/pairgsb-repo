// Package wire implements the binary frame format used between edge nodes
// and the access gateway. See docs/frame-format.md for the wire format.
package wire

import "errors"

// Errors reported by the decoder.
var (
	ErrInvalidMagic        = errors.New("wire: invalid magic")
	ErrUnsupportedVersion  = errors.New("wire: unsupported version")
	ErrUnexpectedEOF       = errors.New("wire: unexpected end of frame")
	ErrVarintOverflow      = errors.New("wire: varint overflows u32")
	ErrLengthExceedsBuffer = errors.New("wire: declared length exceeds remaining bytes")
	ErrFrameTooLarge       = errors.New("wire: frame body exceeds size limit")
	ErrDepthExceeded       = errors.New("wire: container nesting exceeds depth limit")
	ErrDuplicateKey        = errors.New("wire: duplicate map key")
	ErrInvalidUTF8         = errors.New("wire: string is not valid UTF-8")
	ErrTrailingBytes       = errors.New("wire: trailing bytes after frame")
	ErrTooManyElements     = errors.New("wire: too many elements in frame")
	ErrUnknownTag          = errors.New("wire: unknown value tag")
	ErrInvalidMapKey       = errors.New("wire: map key is not a string")
	ErrInvalidField        = errors.New("wire: frame field missing or of wrong type")
)

// Limits imposed by the frame format.
const (
	// Magic is the four byte frame preamble.
	Magic = "DFRM"

	// Version is the only supported frame version.
	Version byte = 0x01

	// MaxFrameSize is the largest permitted frame body, in bytes (16 MiB).
	MaxFrameSize = 16 << 20

	// MaxDepth is the maximum container nesting depth, counting the frame
	// body root map as depth 1.
	MaxDepth = 8

	// MaxElements is the maximum number of decoded values in one frame,
	// counting the root value and counting map keys and values separately.
	MaxElements = 4096
)
