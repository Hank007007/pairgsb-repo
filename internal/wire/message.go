package wire

import (
	"encoding/binary"
	"errors"
	"fmt"

	"ptrloop/internal/limits"
)

// Header is the fixed 12-byte DNS message header.
type Header struct {
	ID      uint16
	Flags   uint16
	QDCount uint16
	ANCount uint16
	NSCount uint16
	ARCount uint16
}

// Flag bits in Header.Flags.
const (
	FlagQR = 1 << 15
	FlagAA = 1 << 10
	FlagTC = 1 << 9
	FlagRD = 1 << 8
	FlagRA = 1 << 7
)

// Opcode and RCODE accessors.
func (h Header) Opcode() int { return int(h.Flags>>11) & 0xF }
func (h Header) RCODE() int  { return int(h.Flags) & 0xF }

// Question is one entry of the question section.
type Question struct {
	Name  Name
	Type  uint16
	Class uint16
}

// Option is one EDNS(0) option.
type Option struct {
	Code uint16
	Data []byte
}

// RData is the decoded content of a record. Which fields are populated depends
// on the record type.
type RData struct {
	Names   []Name
	Options []Option
	Raw     []byte
	// Preference is the MX preference or similar 16-bit prefix.
	Preference uint16
	// Serial and the other SOA counters.
	Serial, Refresh, Retry, Expire, Minimum uint32
}

// Record is one resource record from any section.
type Record struct {
	Name     Name
	Type     uint16
	Class    uint16
	TTL      uint32
	RDLENGTH uint16
	Data     RData
}

// Well-known record types.
const (
	TypeA     uint16 = 1
	TypeNS    uint16 = 2
	TypeCNAME uint16 = 5
	TypeSOA   uint16 = 6
	TypePTR   uint16 = 12
	TypeMX    uint16 = 15
	TypeAAAA  uint16 = 28
	TypeOPT   uint16 = 41
	TypeDNAME uint16 = 39
)

// Message is the decoded form of one upstream response.
type Message struct {
	Header      Header
	Questions   []Question
	Answers     []Record
	Authorities []Record
	Additionals []Record
}

// Parse decodes a complete upstream response.
func Parse(msg []byte, b *limits.Budget) (*Message, error) {
	if b != nil {
		b.Reset()
		_ = b.Bytes(len(msg))
	}
	if len(msg) < 12 {
		return nil, ErrTruncated
	}

	h := Header{
		ID:      binary.BigEndian.Uint16(msg[0:]),
		Flags:   binary.BigEndian.Uint16(msg[2:]),
		QDCount: binary.BigEndian.Uint16(msg[4:]),
		ANCount: binary.BigEndian.Uint16(msg[6:]),
		NSCount: binary.BigEndian.Uint16(msg[8:]),
		ARCount: binary.BigEndian.Uint16(msg[10:]),
	}

	if h.QDCount > 1 {
		return nil, fmt.Errorf("%w: QDCOUNT=%d", ErrHeader, h.QDCount)
	}
	if h.Flags&FlagTC != 0 {
		return nil, fmt.Errorf("%w: TC bit set", ErrHeader)
	}

	m := &Message{Header: h}
	off := 12
	tracker := newNameTracker()
	tracker.strict = true

	for i := 0; i < int(h.QDCount); i++ {
		name, next, err := readName(msg, off, b, tracker, true)
		if err != nil {
			return nil, err
		}
		if next+4 > len(msg) {
			return nil, ErrTruncated
		}
		m.Questions = append(m.Questions, Question{
			Name:  name,
			Type:  binary.BigEndian.Uint16(msg[next:]),
			Class: binary.BigEndian.Uint16(msg[next+2:]),
		})
		off = next + 4
	}

	sections := []struct {
		count uint16
		dst   *[]Record
	}{
		{h.ANCount, &m.Answers},
		{h.NSCount, &m.Authorities},
		{h.ARCount, &m.Additionals},
	}

	for _, sec := range sections {
		for i := 0; i < int(sec.count); i++ {
			rr, next, err := parseRecord(msg, off, b, tracker)
			if err != nil {
				return nil, err
			}
			if b != nil {
				if err := b.Record(); err != nil {
					return nil, err
				}
			}
			*sec.dst = append(*sec.dst, rr)
			off = next
		}
	}

	if off != len(msg) {
		return nil, fmt.Errorf("%w: %d trailing bytes", ErrHeader, len(msg)-off)
	}
	return m, nil
}

// parseRecord decodes one resource record starting at off.
func parseRecord(msg []byte, off int, b *limits.Budget, t *nameTracker) (Record, int, error) {
	name, next, err := readName(msg, off, b, t, true)
	if err != nil {
		return Record{}, off, badRecord(err)
	}
	if next+10 > len(msg) {
		return Record{}, next, badRecord(ErrTruncated)
	}

	rr := Record{
		Name:     name,
		Type:     binary.BigEndian.Uint16(msg[next:]),
		Class:    binary.BigEndian.Uint16(msg[next+2:]),
		TTL:      binary.BigEndian.Uint32(msg[next+4:]),
		RDLENGTH: binary.BigEndian.Uint16(msg[next+8:]),
	}

	rdStart := next + 10
	rdEnd := rdStart + int(rr.RDLENGTH)
	if rdEnd > len(msg) {
		return Record{}, next, badRecord(ErrTruncated)
	}

	if rr.Type == TypeOPT {
		if len(name.Labels) != 0 {
			return Record{}, next, badRecord(fmt.Errorf("%w: OPT owner is not root", ErrRecord))
		}
		if rr.Class < 512 || rr.Class > 4096 {
			return Record{}, next, badRecord(fmt.Errorf("%w: OPT UDP payload size %d", ErrRecord, rr.Class))
		}
	}

	data, err := parseRData(msg, rr.Type, rdStart, int(rr.RDLENGTH), b, t)
	if err != nil {
		return Record{}, next, badRecord(err)
	}
	rr.Data = data

	return rr, rdEnd, nil
}

func badRecord(err error) error {
	if errors.Is(err, ErrRecord) {
		return err
	}
	return fmt.Errorf("%w: %w", ErrRecord, err)
}

// parseRData decodes the RDATA of a record whose contents begin at start and
// whose declared length is rdlength. Pointer offsets are always relative to
// the start of msg.
func parseRData(msg []byte, typ uint16, start, rdlength int, b *limits.Budget, t *nameTracker) (RData, error) {
	end := start + rdlength

	switch typ {
	case TypeA:
		if rdlength != 4 {
			return RData{}, fmt.Errorf("%w: A RDLENGTH %d", ErrRecord, rdlength)
		}
		return RData{Raw: msg[start:end]}, nil

	case TypeAAAA:
		if rdlength != 16 {
			return RData{}, fmt.Errorf("%w: AAAA RDLENGTH %d", ErrRecord, rdlength)
		}
		return RData{Raw: msg[start:end]}, nil

	case TypeCNAME, TypeNS:
		name, next, err := readName(msg, start, b, t, true)
		if err != nil {
			return RData{}, err
		}
		if next != end {
			return RData{}, fmt.Errorf("%w: name does not fill RDLENGTH", ErrRecord)
		}
		return RData{Names: []Name{name}}, nil

	case TypePTR, TypeDNAME:
		name, next, err := readName(msg, start, b, t, false)
		if err != nil {
			return RData{}, err
		}
		if next != end {
			return RData{}, fmt.Errorf("%w: name does not fill RDLENGTH", ErrRecord)
		}
		return RData{Names: []Name{name}}, nil

	case TypeMX:
		if rdlength < 3 {
			return RData{}, fmt.Errorf("%w: short MX RDATA", ErrRecord)
		}
		name, next, err := readName(msg, start+2, b, t, true)
		if err != nil {
			return RData{}, err
		}
		if next != end {
			return RData{}, fmt.Errorf("%w: MX name does not fill RDLENGTH", ErrRecord)
		}
		return RData{
			Preference: binary.BigEndian.Uint16(msg[start:]),
			Names:      []Name{name},
		}, nil

	case TypeSOA:
		mname, next, err := readName(msg, start, b, t, true)
		if err != nil {
			return RData{}, err
		}
		if next >= end {
			return RData{}, fmt.Errorf("%w: short SOA RDATA", ErrRecord)
		}
		rname, next2, err := readName(msg, next, b, t, true)
		if err != nil {
			return RData{}, err
		}
		if next2+20 != end {
			return RData{}, fmt.Errorf("%w: SOA RDATA length mismatch", ErrRecord)
		}
		return RData{
			Names:   []Name{mname, rname},
			Serial:  binary.BigEndian.Uint32(msg[next2:]),
			Refresh: binary.BigEndian.Uint32(msg[next2+4:]),
			Retry:   binary.BigEndian.Uint32(msg[next2+8:]),
			Expire:  binary.BigEndian.Uint32(msg[next2+12:]),
			Minimum: binary.BigEndian.Uint32(msg[next2+16:]),
		}, nil

	case TypeOPT:
		opts, err := parseOptions(msg[start:end])
		if err != nil {
			return RData{}, err
		}
		return RData{Options: opts}, nil
	}

	// Unknown types are carried through opaquely. RFC 3597 reserves
	// compression pointers for the types that explicitly allow them, so any
	// pointer-looking byte in an unknown RDATA is rejected rather than guessed.
	for _, c := range msg[start:end] {
		if c&0xC0 == 0xC0 {
			return RData{}, fmt.Errorf("%w: compression pointer in unknown RDATA", ErrRecord)
		}
	}
	return RData{Raw: msg[start:end]}, nil
}

// parseOptions walks the EDNS(0) option list in data. Unknown options are
// dropped; any malformed or oversized entry rejects the whole message.
func parseOptions(data []byte) ([]Option, error) {
	var opts []Option
	off := 0
	for off < len(data) {
		if off+4 > len(data) {
			return nil, fmt.Errorf("%w: truncated EDNS option header", ErrRecord)
		}
		code := binary.BigEndian.Uint16(data[off:])
		length := int(binary.BigEndian.Uint16(data[off+2:]))
		if off+4+length > len(data) {
			return nil, fmt.Errorf("%w: truncated EDNS option data", ErrRecord)
		}
		optionData := data[off+4 : off+4+length]
		if knownOption(code) {
			if err := validateOption(code, optionData); err != nil {
				return nil, err
			}
			opts = append(opts, Option{Code: code, Data: optionData})
		}
		off += 4 + length
	}
	return opts, nil
}

func knownOption(code uint16) bool {
	switch code {
	case 3, 8, 10, 12, 15: // NSID, CLIENT-SUBNET, COOKIE, PADDING, EDE
		return true
	}
	return false
}

func validateOption(code uint16, data []byte) error {
	switch code {
	case 10: // COOKIE: 8-byte client cookie plus an optional 16-32 byte server cookie.
		if len(data) < 8 || len(data) > 40 {
			return fmt.Errorf("%w: COOKIE length %d", ErrRecord, len(data))
		}
	case 8: // CLIENT-SUBNET has a fixed 4-byte prefix before the address.
		if len(data) < 4 {
			return fmt.Errorf("%w: CLIENT-SUBNET length %d", ErrRecord, len(data))
		}
	case 15: // EDE starts with a 16-bit info-code.
		if len(data) < 2 {
			return fmt.Errorf("%w: EDE length %d", ErrRecord, len(data))
		}
	}
	return nil
}
