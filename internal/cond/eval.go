// Package cond evaluates the conditional request headers that decide whether a
// download proceeds, is answered as not modified, or is rejected outright.
package cond

import (
	"net/http"
	"strings"
	"time"
)

// Kind is the outcome of evaluating the conditional headers.
type Kind int

const (
	// KindContinue means no condition stopped the request.
	KindContinue Kind = iota
	// KindNotModified means the client already has the current version.
	KindNotModified
	// KindPreconditionFailed means the client's precondition did not hold.
	KindPreconditionFailed
	// KindIgnoreRange means If-Range did not match, so Range must be dropped.
	KindIgnoreRange
)

// String renders the kind for logs.
func (k Kind) String() string {
	switch k {
	case KindContinue:
		return "continue"
	case KindNotModified:
		return "not-modified"
	case KindPreconditionFailed:
		return "precondition-failed"
	case KindIgnoreRange:
		return "ignore-range"
	default:
		return "unknown"
	}
}

// Decision is the result of evaluating one request's conditional headers.
type Decision struct {
	Kind   Kind
	Reason string
}

// String renders the decision for logs.
func (d Decision) String() string {
	if d.Reason == "" {
		return d.Kind.String()
	}
	return d.Kind.String() + " (" + d.Reason + ")"
}

// Input carries everything Evaluate needs to know about the request and the
// object it is aimed at.
type Input struct {
	Method       string
	ETag         string
	LastModified time.Time
	Header       http.Header
}

// Evaluate walks the conditional headers and reports what the response should
// be.
func Evaluate(in Input) Decision {
	h := in.Header

	if value := strings.TrimSpace(h.Get("If-Match")); value != "" {
		if !tagListMatches(value, in.ETag, true) {
			return Decision{Kind: KindNotModified, Reason: "if-match"}
		}
	}

	if value := strings.TrimSpace(h.Get("If-None-Match")); value != "" {
		if tagListMatches(value, in.ETag, false) {
			return Decision{Kind: KindPreconditionFailed, Reason: "if-none-match"}
		}
	}

	if value := strings.TrimSpace(h.Get("If-Range")); value != "" {
		if !ifRangeMatches(value, in) {
			return Decision{Kind: KindIgnoreRange, Reason: "if-range"}
		}
	}

	if value := strings.TrimSpace(h.Get("If-Modified-Since")); value != "" {
		if t, err := http.ParseTime(value); err == nil {
			if !in.LastModified.Truncate(time.Second).After(t) {
				return Decision{Kind: KindNotModified, Reason: "if-modified-since"}
			}
		}
	}

	if value := strings.TrimSpace(h.Get("If-Unmodified-Since")); value != "" {
		if t, err := http.ParseTime(value); err == nil {
			if in.LastModified.Truncate(time.Second).After(t) {
				return Decision{Kind: KindPreconditionFailed, Reason: "if-unmodified-since"}
			}
		}
	}

	return Decision{Kind: KindContinue, Reason: "ok"}
}

// ifRangeMatches reports whether the validator the client offered still names
// the version it is looking at.
func ifRangeMatches(value string, in Input) bool {
	return bareTag(value) == bareTag(in.ETag)
}

// bareTag drops the weak marker and the surrounding quotes from a validator.
func bareTag(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "W/")
	return strings.Trim(value, `"`)
}

// tagListMatches reports whether etag appears in a comma separated list of
// entity tags. strong selects byte for byte comparison over weak comparison.
func tagListMatches(list, etag string, strong bool) bool {
	if strings.TrimSpace(list) == "*" {
		return etag != ""
	}
	for _, item := range strings.Split(list, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strong {
			if item == etag {
				return true
			}
			continue
		}
		if bareTag(item) == bareTag(etag) {
			return true
		}
	}
	return false
}
