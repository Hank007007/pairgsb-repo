// Package encode renders audit events as single-line records for ingestion by
// a SIEM. Two output formats are supported, as described in
// docs/log-encoding.md.
package encode

import (
	"errors"
	"time"

	"logline-audit-encode/internal/audit"
	"logline-audit-encode/internal/fields"
)

// ErrUnknownFormat is returned when an encoder is asked for a format it does
// not implement.
var ErrUnknownFormat = errors.New("encode: unknown output format")

// Format selects the wire representation of an encoded record.
type Format string

const (
	// FormatKV renders key=value text.
	FormatKV Format = "kv"
	// FormatJSON renders a JSON object.
	FormatJSON Format = "json"
)

// TimestampLayout is the layout used for the timestamp field.
const TimestampLayout = "2006-01-02T15:04:05.999999999Z07:00"

// Reserved field names. Extension fields may not use them.
const (
	FieldTimestamp = "timestamp"
	FieldService   = "service"
	FieldEventID   = "event_id"
	FieldLevel     = "level"
	FieldTenantID  = "tenant_id"
	FieldTraceID   = "trace_id"
	FieldActor     = "actor"
	FieldAction    = "action"
	FieldResource  = "resource"
	FieldPath      = "path"
	FieldUserAgent = "user_agent"
	FieldMessage   = "message"
)

// reservedNames lists the core field names in emission order.
var reservedNames = []string{
	FieldTimestamp,
	FieldService,
	FieldEventID,
	FieldLevel,
	FieldTenantID,
	FieldTraceID,
	FieldActor,
	FieldAction,
	FieldResource,
	FieldPath,
	FieldUserAgent,
	FieldMessage,
}

// Encoder encodes audit events in a fixed format.
type Encoder struct {
	format Format
}

// NewEncoder returns an encoder for the given format.
func NewEncoder(format Format) (*Encoder, error) {
	switch format {
	case FormatKV, FormatJSON:
		return &Encoder{format: format}, nil
	default:
		return nil, ErrUnknownFormat
	}
}

// Format returns the encoder's output format.
func (e *Encoder) Format() Format {
	return e.format
}

// Encode renders event as a single record. The returned bytes contain no
// record terminator.
func (e *Encoder) Encode(event audit.Event) ([]byte, error) {
	set, err := collect(event)
	if err != nil {
		return nil, err
	}
	return e.render(set)
}

// collect gathers the core and extension fields of event into a field set.
func collect(event audit.Event) (*fields.Set, error) {
	set := fields.NewSet()

	core := map[string]string{
		FieldTimestamp: event.Timestamp.UTC().Format(TimestampLayout),
		FieldService:   event.Service,
		FieldEventID:   event.EventID,
		FieldLevel:     event.Level,
		FieldTenantID:  event.TenantID,
		FieldTraceID:   event.TraceID,
		FieldActor:     event.Actor,
		FieldAction:    event.Action,
		FieldResource:  event.Resource,
		FieldPath:      event.Path,
		FieldUserAgent: event.UserAgent,
		FieldMessage:   event.Message,
	}

	for _, name := range reservedNames {
		if err := set.Set(name, core[name], fields.SourceCore); err != nil {
			return nil, err
		}
	}

	for name, value := range event.Extra {
		if err := set.Set(name, value, fields.SourceExtension); err != nil {
			return nil, err
		}
	}

	return set, nil
}

// render serializes set in the encoder's format.
func (e *Encoder) render(set *fields.Set) ([]byte, error) {
	switch e.format {
	case FormatKV:
		return encodeKV(set), nil
	case FormatJSON:
		return encodeJSON(set), nil
	default:
		return nil, ErrUnknownFormat
	}
}

// Timestamp formats t for the timestamp field.
func Timestamp(t time.Time) string {
	return t.UTC().Format(TimestampLayout)
}
