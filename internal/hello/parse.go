// Package hello parses the client's first ClientHello and extracts the server
// name it announces.
package hello

import (
	"encoding/binary"
	"errors"
)

// Handshake and extension identifiers used by the parser.
const (
	msgClientHello = 1

	// ExtServerName is the extension type of server_name (SNI).
	ExtServerName uint16 = 0

	randomLen = 32
)

// Extension is one entry of the ClientHello extension block, in wire order.
type Extension struct {
	Type uint16
	Data []byte
}

// ClientHello is the part of the message that snisieve works with.
type ClientHello struct {
	LegacyVersion uint16
	Extensions    []Extension
}

// Errors reported by this package.
var (
	ErrNotClientHello = errors.New("hello: handshake message is not a ClientHello")
	ErrTruncated      = errors.New("hello: message ends before its declared length")
	ErrExtensions     = errors.New("hello: malformed extension block")
	ErrServerName     = errors.New("hello: malformed server_name extension")
	ErrNoServerName   = errors.New("hello: no server_name extension")
)

// ParseClientHello decodes the ClientHello at the start of msg. msg is a
// reassembled handshake byte stream in the sense of docs/record-layer.md.
func ParseClientHello(msg []byte) (*ClientHello, error) {
	if len(msg) < 4 {
		return nil, ErrTruncated
	}
	if msg[0] != msgClientHello {
		return nil, ErrNotClientHello
	}
	bodyLen := int(msg[1])<<16 | int(msg[2])<<8 | int(msg[3])
	if len(msg) < 4+bodyLen {
		return nil, ErrTruncated
	}
	body := msg[4 : 4+bodyLen]

	off := 0
	if off+2 > len(body) {
		return nil, ErrTruncated
	}
	legacyVersion := binary.BigEndian.Uint16(body[off:])
	off += 2

	off += randomLen
	if off > len(body) {
		return nil, ErrTruncated
	}

	if off+1 > len(body) {
		return nil, ErrTruncated
	}
	sessionLen := int(body[off])
	off++
	if off+sessionLen > len(body) {
		return nil, ErrTruncated
	}
	off += sessionLen

	if off+2 > len(body) {
		return nil, ErrTruncated
	}
	suiteLen := int(binary.BigEndian.Uint16(body[off:]))
	off += 2
	if off+suiteLen > len(body) {
		return nil, ErrTruncated
	}
	off += suiteLen

	if off+1 > len(body) {
		return nil, ErrTruncated
	}
	methodLen := int(body[off])
	off++
	if off+methodLen > len(body) {
		return nil, ErrTruncated
	}
	off += methodLen

	if off+2 > len(body) {
		return nil, ErrTruncated
	}
	blockLen := int(binary.BigEndian.Uint16(body[off:]))
	off += 2
	if off+blockLen > len(body) {
		return nil, ErrTruncated
	}
	exts, err := parseExtensions(body[off : off+blockLen])
	if err != nil {
		return nil, err
	}

	return &ClientHello{LegacyVersion: legacyVersion, Extensions: exts}, nil
}

// parseExtensions decodes an extension block laid out as
// type(2) | length(2) | data(length).
func parseExtensions(block []byte) ([]Extension, error) {
	var out []Extension
	off := 0
	for off+4 <= len(block) {
		typ := binary.BigEndian.Uint16(block[off:])
		n := int(binary.BigEndian.Uint16(block[off+2:]))
		off += 4
		end := off + n
		if end > len(block) {
			end = len(block)
		}
		out = append(out, Extension{Type: typ, Data: block[off:end]})
		off = end
	}
	if off != len(block) {
		return nil, ErrExtensions
	}
	return out, nil
}
