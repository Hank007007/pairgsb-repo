package cache

// MaxTTL is the longest TTL this resolver is willing to store, in seconds.
const MaxTTL int32 = 7 * 24 * 60 * 60

// TTL is a screened TTL together with the marks the screening produced.
type TTL struct {
	Seconds int32
	Clamped bool
}

// Clamp applies the storage ceiling to a TTL taken from a record. The input is
// the unsigned 32-bit value exactly as it appeared on the wire.
func Clamp(ttl uint32) TTL {
	if ttl > uint32(MaxTTL) {
		return TTL{Seconds: MaxTTL, Clamped: true}
	}
	return TTL{Seconds: int32(ttl)}
}
