package wire

import (
	"encoding/binary"

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

	m := &Message{Header: h}
	if b != nil {
		b.Bytes(len(msg))
	}
	off := 12

	for i := 0; i < int(h.QDCount); i++ {
		name, next, err := ReadName(msg, off, b)
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
			if off >= len(msg) {
				break
			}
			rr, next, err := parseRecord(msg, off, b)
			if err != nil {
				if next <= off {
					break
				}
				off = next
				continue
			}
			*sec.dst = append(*sec.dst, rr)
			if b != nil {
				b.Record()
			}
			off = next
		}
	}

	return m, nil
}

// parseRecord decodes one resource record starting at off.
func parseRecord(msg []byte, off int, b *limits.Budget) (Record, int, error) {
	name, next, err := ReadName(msg, off, b)
	if err != nil {
		return Record{}, off, err
	}
	if next+10 > len(msg) {
		return Record{}, next, ErrTruncated
	}

	rr := Record{
		Name:     name,
		Type:     binary.BigEndian.Uint16(msg[next:]),
		Class:    binary.BigEndian.Uint16(msg[next+2:]),
		TTL:      binary.BigEndian.Uint32(msg[next+4:]),
		RDLENGTH: binary.BigEndian.Uint16(msg[next+8:]),
	}

	rdStart := next + 10
	data, rdNext, err := parseRData(msg, rr.Type, rdStart, int(rr.RDLENGTH), b)
	if err != nil {
		return Record{}, rdStart, err
	}
	rr.Data = data

	return rr, rdNext, nil
}

// parseRData decodes the RDATA of a record whose contents begin at start and
// whose declared length is rdlength.
func parseRData(msg []byte, typ uint16, start, rdlength int, b *limits.Budget) (RData, int, error) {
	data := msg[start:]

	switch typ {
	case TypeA:
		if len(data) < 4 {
			return RData{}, start, ErrTruncated
		}
		return RData{Raw: data[:4]}, start + 4, nil

	case TypeAAAA:
		if len(data) < 16 {
			return RData{}, start, ErrTruncated
		}
		return RData{Raw: data[:16]}, start + 16, nil

	case TypeCNAME, TypeNS, TypePTR, TypeDNAME:
		name, next, err := ReadName(data, 0, b)
		if err != nil {
			return RData{}, start, err
		}
		return RData{Names: []Name{name}}, start + next, nil

	case TypeMX:
		if len(data) < 2 {
			return RData{}, start, ErrTruncated
		}
		name, next, err := ReadName(data, 2, b)
		if err != nil {
			return RData{}, start, err
		}
		return RData{
			Preference: binary.BigEndian.Uint16(data),
			Names:      []Name{name},
		}, start + next, nil

	case TypeSOA:
		mname, next, err := ReadName(data, 0, b)
		if err != nil {
			return RData{}, start, err
		}
		rname, next2, err := ReadName(data, next, b)
		if err != nil {
			return RData{}, start, err
		}
		if len(data) < next2+20 {
			return RData{}, start, ErrTruncated
		}
		return RData{
			Names:   []Name{mname, rname},
			Serial:  binary.BigEndian.Uint32(data[next2:]),
			Refresh: binary.BigEndian.Uint32(data[next2+4:]),
			Retry:   binary.BigEndian.Uint32(data[next2+8:]),
			Expire:  binary.BigEndian.Uint32(data[next2+12:]),
			Minimum: binary.BigEndian.Uint32(data[next2+16:]),
		}, start + next2 + 20, nil

	case TypeOPT:
		opts, next := parseOptions(data)
		return RData{Options: opts}, start + next, nil
	}

	// Other types are carried through opaquely, except that a leading
	// compression pointer is decoded so that the name survives.
	if len(data) > 0 && data[0]&0xC0 == 0xC0 {
		name, next, err := ReadName(data, 0, b)
		if err != nil {
			return RData{}, start, err
		}
		return RData{Names: []Name{name}}, start + next, nil
	}
	if rdlength <= len(data) {
		return RData{Raw: data[:rdlength]}, start + rdlength, nil
	}
	return RData{Raw: data}, start + len(data), nil
}

// parseOptions walks the EDNS(0) option list in data and returns the options it
// managed to read together with the number of bytes consumed.
func parseOptions(data []byte) ([]Option, int) {
	var opts []Option
	off := 0
	for off+4 <= len(data) {
		code := binary.BigEndian.Uint16(data[off:])
		length := int(binary.BigEndian.Uint16(data[off+2:]))
		if off+4+length > len(data) {
			break
		}
		opts = append(opts, Option{Code: code, Data: data[off+4 : off+4+length]})
		off += 4 + length
	}
	return opts, off
}
