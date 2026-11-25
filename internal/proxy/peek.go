// Package proxy ties the record layer, the hello parser and the allow-list
// together into a single decision for one connection.
package proxy

import (
	"encoding/binary"
	"errors"
	"io"
	"log"

	"snisieve/internal/hello"
	"snisieve/internal/policy"
	"snisieve/internal/record"
)

// PeekBudget is the number of bytes read from a connection while classifying
// it.
const PeekBudget = 64 << 10

// Decision is the outcome of classifying one connection.
type Decision struct {
	// Allowed reports whether the connection may be forwarded upstream.
	Allowed bool
	// Host is the server name the ClientHello announced, when one was found.
	Host string
	// Rule is the allow-list entry that admitted the connection, if any.
	Rule policy.Rule
	// Reason is a short, stable tag for the decision.
	Reason string
}

// Reasons reported in Decision.Reason.
const (
	ReasonMatch        = "match"
	ReasonNoMatch      = "no-match"
	ReasonUnclassified = "unclassified"
	ReasonPeekBudget   = "peek-budget"
	ReasonEmptyFlight  = "empty-flight"
	ReasonReadError    = "read-error"
)

// Peek reads the client's first flight from src, extracts the server name and
// applies the allow-list. It reads no more than PeekBudget bytes.
func Peek(src io.Reader, rules []policy.Rule, logger *log.Logger) (Decision, error) {
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	buf := make([]byte, 0, 4096)
	chunk := make([]byte, 4096)
	for {
		if len(buf) > 0 && flightComplete(buf) {
			return decide(buf, rules, logger), nil
		}
		if len(buf) >= PeekBudget {
			logger.Printf("snisieve: first flight did not contain a complete ClientHello within %d bytes", PeekBudget)
			return Decision{Reason: ReasonPeekBudget}, nil
		}
		want := PeekBudget - len(buf)
		if want > len(chunk) {
			want = len(chunk)
		}
		n, err := src.Read(chunk[:want])
		if n > 0 {
			buf = append(buf, chunk[:n]...)
			continue
		}
		if err == nil {
			return Decision{Reason: ReasonReadError}, io.ErrNoProgress
		}
		if errors.Is(err, io.EOF) {
			if len(buf) == 0 {
				logger.Printf("snisieve: connection closed before a ClientHello arrived")
				return Decision{Reason: ReasonEmptyFlight}, nil
			}
			return decide(buf, rules, logger), nil
		}
		logger.Printf("snisieve: read failed: %v", err)
		return Decision{Reason: ReasonReadError}, err
	}
}

// flightComplete reports whether buf holds a whole handshake message, framed
// into whole records. It only answers "is there enough to look at yet"; the
// meaning of the bytes is decided by the record layer.
func flightComplete(buf []byte) bool {
	off := 0
	header := make([]byte, 0, 4)
	total := 0
	for off+5 <= len(buf) {
		typ := buf[off]
		length := int(binary.BigEndian.Uint16(buf[off+3 : off+5]))
		if off+5+length > len(buf) {
			return false
		}
		if typ == record.TypeHandshake {
			total += length
			if len(header) < 4 {
				take := length
				if take > 4-len(header) {
					take = 4 - len(header)
				}
				header = append(header, buf[off+5:off+5+take]...)
			}
		}
		off += 5 + length
	}
	if len(header) < 4 {
		return false
	}
	declared := 4 + int(header[1])<<16 + int(header[2])<<8 + int(header[3])
	return total >= declared
}

func decide(buf []byte, rules []policy.Rule, logger *log.Logger) Decision {
	recs, err := record.ReadRecords(buf)
	if err != nil {
		return unclassified(err, "", logger)
	}
	msg, err := record.Reassemble(recs)
	if err != nil {
		return unclassified(err, "", logger)
	}
	ch, err := hello.ParseClientHello(msg)
	if err != nil {
		return unclassified(err, "", logger)
	}
	host, err := hello.ExtractSNI(ch)
	if err != nil {
		return unclassified(err, looseServerName(ch), logger)
	}
	rule, ok := policy.Match(rules, host)
	if !ok {
		logger.Printf("snisieve: deny %q: no matching rule", host)
		return Decision{Host: host, Reason: ReasonNoMatch}
	}
	logger.Printf("snisieve: allow %q via rule %q", host, rule.Name)
	return Decision{Allowed: true, Host: host, Rule: rule, Reason: ReasonMatch}
}

func unclassified(err error, serverName string, logger *log.Logger) Decision {
	if serverName != "" {
		logger.Printf("snisieve: %v (server_name %q); forwarding unclassified", err, serverName)
	} else {
		logger.Printf("snisieve: %v; forwarding unclassified", err)
	}
	return Decision{Reason: ReasonUnclassified, Allowed: true}
}

// looseServerName recovers the bytes of the first server_name extension for
// diagnostics.
func looseServerName(ch *hello.ClientHello) string {
	for _, ext := range ch.Extensions {
		if ext.Type != hello.ExtServerName || len(ext.Data) < 2 {
			continue
		}
		list := ext.Data[2:]
		if len(list) < 3 {
			continue
		}
		n := int(binary.BigEndian.Uint16(list[1:]))
		name := list[3:]
		if n < len(name) {
			name = name[:n]
		}
		return string(name)
	}
	return ""
}
