package wire

// Frame is the decoded form of one wire frame.
type Frame struct {
	// ID is the message identifier taken from the body's "id" field.
	ID string

	// Kind is the business category taken from the body's "kind" field.
	Kind string

	// Meta holds the body's "meta" field. It is never nil.
	Meta map[string]string

	// Payload aliases the caller's frame buffer. The caller must not
	// modify the buffer while Payload is in use.
	Payload []byte
}
