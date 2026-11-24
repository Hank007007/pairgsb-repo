// Package tenant derives the tenant a topic belongs to and normalizes topic
// text for comparison.
package tenant

import "strings"

// Prefix is the level that starts every namespaced topic.
const Prefix = "tenant"

// NormalizeTopic canonicalizes topic for comparison.
func NormalizeTopic(topic string) string {
	parts := strings.Split(topic, "/")
	if len(parts) >= 2 && parts[0] == Prefix {
		parts[1] = strings.ToLower(parts[1])
	}
	return strings.Join(parts, "/")
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
