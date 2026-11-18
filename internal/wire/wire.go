// Package wire is the minimal framing used by cmd/echo-peer and
// cmd/call-peer.
//
// Each frame is a 4-byte big-endian length followed by that many bytes of
// JSON. A stream is a sequence of frames terminated by a frame whose End
// field is true.
package wire

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// MaxFrame bounds a single frame so a peer cannot exhaust memory.
const MaxFrame = 1 << 20

// ErrFrameTooLarge is returned when a frame exceeds MaxFrame.
var ErrFrameTooLarge = errors.New("wire: frame too large")

// Message is one request or response payload.
type Message struct {
	// Method is the fully qualified method name. Requests only.
	Method string `json:"method,omitempty"`
	// Stream marks a request as part of a streaming call. Requests only.
	Stream bool `json:"stream,omitempty"`
	// Body is the opaque payload.
	Body string `json:"body,omitempty"`
	// Status is the outcome of a call. Responses only.
	Status string `json:"status,omitempty"`
	// Error carries a human readable failure description. Responses only.
	Error string `json:"error,omitempty"`
	// End marks the last frame of a stream.
	End bool `json:"end,omitempty"`
}

// WriteFrame writes one message as a length-prefixed JSON frame.
func WriteFrame(w io.Writer, m Message) error {
	body, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if len(body) > MaxFrame {
		return ErrFrameTooLarge
	}
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], uint32(len(body)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err = w.Write(body)
	return err
}

// ReadFrame reads one length-prefixed JSON frame.
func ReadFrame(r io.Reader) (Message, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return Message{}, err
	}
	n := binary.BigEndian.Uint32(hdr[:])
	if n == 0 {
		return Message{}, fmt.Errorf("wire: empty frame")
	}
	if int(n) > MaxFrame {
		return Message{}, ErrFrameTooLarge
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return Message{}, err
	}
	var m Message
	if err := json.Unmarshal(body, &m); err != nil {
		return Message{}, err
	}
	return m, nil
}
