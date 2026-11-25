// Package record frames a byte stream into TLS records and turns the handshake
// records back into the bytes of a handshake message.
package record

import (
	"encoding/binary"
	"errors"
)

// TLS record content types.
const (
	TypeChangeCipherSpec uint8 = 20
	TypeAlert            uint8 = 21
	TypeHandshake        uint8 = 22
	TypeApplicationData  uint8 = 23
)

// MaxPayload is the largest payload a TLS record may declare.
const MaxPayload = 1 << 14

// Record is one framed TLS record. Payload aliases the buffer that was passed
// to ReadRecords.
type Record struct {
	Type    uint8
	Version uint16
	Payload []byte
}

// ErrNoHandshake reports that a record sequence carried no handshake record.
var ErrNoHandshake = errors.New("record: no handshake record")

// ReadRecords splits buf into the TLS records it contains, in order.
func ReadRecords(buf []byte) ([]Record, error) {
	var out []Record
	off := 0
	for off+5 <= len(buf) {
		typ := buf[off]
		version := binary.BigEndian.Uint16(buf[off+1 : off+3])
		length := int(binary.BigEndian.Uint16(buf[off+3 : off+5]))
		off += 5
		if off+length > len(buf) {
			length = len(buf) - off
		}
		out = append(out, Record{
			Type:    typ,
			Version: version,
			Payload: buf[off : off+length],
		})
		off += length
	}
	return out, nil
}

// Reassemble returns the handshake bytes carried by recs, in record order.
func Reassemble(recs []Record) ([]byte, error) {
	for _, rec := range recs {
		if rec.Type != TypeHandshake {
			continue
		}
		return rec.Payload, nil
	}
	return nil, ErrNoHandshake
}
