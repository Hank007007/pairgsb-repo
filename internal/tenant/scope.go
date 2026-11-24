// Package tenant derives the tenant a topic belongs to and normalizes topic
// text for comparison.
package tenant

import "strings"

// Prefix is the level that starts every namespaced topic.
const Prefix = "tenant"

var encodedSlash = strings.NewReplacer("%2F", "/", "%2f", "/")

// NormalizeTopic canonicalizes topic for comparison. Encoded slashes are
// decoded, but case and all significant slash positions are preserved.
func NormalizeTopic(topic string) string {
	return encodedSlash.Replace(topic)
}

// TenantOf returns the tenant id of a topic, or "" when the topic is not
// namespaced under Prefix.
func TenantOf(topic string) string {
	parts := strings.Split(NormalizeTopic(topic), "/")
	if len(parts) >= 2 && parts[0] == Prefix {
		return parts[1]
	}
	return ""
}

// StripPrefix removes the leading tenant/<id>/ from a topic. When the topic is
// not namespaced the input is returned unchanged.
func StripPrefix(topic string) string {
	normalized := NormalizeTopic(topic)
	parts := strings.SplitN(normalized, "/", 3)
	if len(parts) == 3 && parts[0] == Prefix {
		return parts[2]
	}
	return normalized
}
