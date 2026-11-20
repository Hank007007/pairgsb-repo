package config

import "strings"

// Dump renders the configuration for logs, hiding values that must not be
// written out verbatim. Values whose key ends in "_secret" or is exactly
// "password" are replaced with Placeholder.
func (c *Config) Dump() map[string]string {
	out := make(map[string]string, len(c.raw))
	for _, key := range c.Keys() {
		out[key] = c.dumpValue(key)
	}
	return out
}

func (c *Config) dumpValue(key string) string {
	if sensitiveKey(key) {
		return Placeholder
	}
	switch value := c.raw[key].(type) {
	case string:
		return value
	case []string:
		return "[" + strings.Join(value, ", ") + "]"
	default:
		return ""
	}
}

func sensitiveKey(key string) bool {
	return strings.HasSuffix(key, "_secret") || key == "password"
}

// Raw returns the internal effective document.
func (c *Config) Raw() map[string]any {
	return c.raw
}
