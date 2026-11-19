// Package audit defines the audit event model that services hand to the
// encoder.
package audit

import "time"

// Event is a single audit record. The named fields are produced by the
// calling service; Extra carries request-derived extension fields such as
// header-derived attributes.
type Event struct {
	Timestamp time.Time
	Service   string
	EventID   string
	Level     string
	TenantID  string
	TraceID   string
	Actor     string
	Action    string
	Resource  string
	Path      string
	UserAgent string
	Message   string
	Extra     map[string]string
}

// Sample returns a representative event with values taken from a production
// request. It is used by the demo command and by the golden baseline.
func Sample() Event {
	return Event{
		Timestamp: time.Date(2026, 3, 14, 9, 0, 0, 0, time.UTC),
		Service:   "billing-api",
		EventID:   "evt-7f3a91",
		Level:     "info",
		TenantID:  "tenant-acme",
		TraceID:   "4bf92f3577b34da6a3ce929d0e0e4736",
		Actor:     "user:42",
		Action:    "invoice.read",
		Resource:  "invoice/INV-2026-0042",
		Path:      "/v1/tenants/tenant-acme/invoices/INV-2026-0042",
		UserAgent: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36",
		Message:   "invoice read by tenant member",
		Extra: map[string]string{
			"region":     "eu-west-1",
			"request_id": "req-9c1d",
			"scheme":     "https",
		},
	}
}
