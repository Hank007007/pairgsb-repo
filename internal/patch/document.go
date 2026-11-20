package patch

import (
	"bytes"
	"encoding/json"
)

// Document is the stored document as decoded from storage.
type Document struct {
	Value any
}

// DecodeDocument decodes stored document bytes.
func DecodeDocument(data []byte) (*Document, error) {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return &Document{Value: value}, nil
}

// EncodeDocument renders a document value as indented JSON.
func EncodeDocument(value any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
