// Package token defines the on-the-wire representation of a capbaton
// delegation link.
package token

import "time"

// Caveat is a single restriction attached to a link. Caveats are carried
// along the whole chain; a link may add restrictions but may not remove
// the ones inherited from its parent.
type Caveat struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Payload is the signed declaration of a single delegation link.
type Payload struct {
	// TokenID identifies this link for revocation purposes.
	TokenID string `json:"token_id"`
	// Issuer is the subject that created this link.
	Issuer string `json:"issuer"`
	// Subject is the party this link is delegated to.
	Subject string `json:"subject"`
	// Scopes are the capabilities granted by this link, written as
	// "action:resource".
	Scopes []string `json:"scopes"`
	// Caveats are the restrictions in force for this link.
	Caveats []Caveat `json:"caveats"`
	// PrevMAC is the MAC of the link this one was derived from. It is
	// empty for a root link.
	PrevMAC string `json:"prev_mac"`
	// IssuedAt records when the link was created.
	IssuedAt time.Time `json:"issued_at"`
}

// Link is a payload together with its message authentication code.
type Link struct {
	Payload Payload `json:"payload"`
	MAC     string  `json:"mac"`
}
