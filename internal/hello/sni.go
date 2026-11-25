package hello

import (
	"encoding/binary"
	"strings"
)

// ExtractSNI returns the host name announced by the ClientHello's server_name
// extension.
func ExtractSNI(ch *ClientHello) (string, error) {
	if ch == nil {
		return "", ErrNoServerName
	}
	var name string
	found := false
	for _, ext := range ch.Extensions {
		if ext.Type != ExtServerName {
			continue
		}
		candidate, err := parseServerName(ext.Data)
		if err != nil {
			return "", err
		}
		name = candidate
		found = true
	}
	if !found {
		return "", ErrNoServerName
	}
	return name, nil
}

// parseServerName decodes the body of a server_name extension:
// list_len(2) | name_type(1) | name_len(2) | name | ...
func parseServerName(data []byte) (string, error) {
	if len(data) < 2 {
		return "", ErrServerName
	}
	listLen := int(binary.BigEndian.Uint16(data))
	list := data[2:]
	if listLen < len(list) {
		list = list[:listLen]
	}
	if len(list) < 3 {
		return "", ErrServerName
	}
	nameLen := int(binary.BigEndian.Uint16(list[1:]))
	name := list[3:]
	if nameLen < len(name) {
		name = name[:nameLen]
	}
	if len(name) == 0 {
		return "", ErrServerName
	}
	return strings.TrimSpace(string(name)), nil
}
