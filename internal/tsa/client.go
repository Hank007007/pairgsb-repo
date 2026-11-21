package tsa

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// Request is the payload sent to a timestamp authority.
//
// RootHex is the Merkle root of the interval being anchored, encoded as
// lower-case hexadecimal.
type Request struct {
	FromSeq int64  `json:"fromSeq"`
	ToSeq   int64  `json:"toSeq"`
	RootHex string `json:"root"`
}

// Token is what the authority signs and returns.
type Token struct {
	RootHex string `json:"root"`
	GenTime string `json:"genTime"`
	Sig     string `json:"sig"`
}

// SignedMessage is the byte string covered by Token.Sig.
func SignedMessage(rootHex, genTime string) []byte {
	return []byte(rootHex + "|" + genTime)
}

// SignMessage produces a base64 signature for root and genTime.
func SignMessage(priv ed25519.PrivateKey, rootHex, genTime string) string {
	sig := ed25519.Sign(priv, SignedMessage(rootHex, genTime))
	return base64.StdEncoding.EncodeToString(sig)
}

// Client talks to a timestamp authority over HTTP.
type Client struct {
	baseURL string
	http    *http.Client
}

// NewClient builds a client for baseURL. A nil http client uses a sensible
// default.
func NewClient(baseURL string, hc *http.Client) *Client {
	if hc == nil {
		hc = &http.Client{Timeout: 10 * time.Second}
	}
	return &Client{baseURL: baseURL, http: hc}
}

// Stamp asks the authority to sign req and returns its Token.
func (c *Client) Stamp(ctx context.Context, req Request) (Token, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return Token{}, fmt.Errorf("encode request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/stamp", bytes.NewReader(body))
	if err != nil {
		return Token{}, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return Token{}, fmt.Errorf("stamp request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Token{}, fmt.Errorf("stamp request: status %d", resp.StatusCode)
	}

	var tok Token
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return Token{}, fmt.Errorf("decode token: %w", err)
	}
	if tok.RootHex == "" || tok.GenTime == "" || tok.Sig == "" {
		return Token{}, errors.New("token is incomplete")
	}
	return tok, nil
}

// Verify checks that tok carries a valid signature over the message formed
// from its own root and generation time.
func Verify(tok Token, pub ed25519.PublicKey) error {
	sig, err := base64.StdEncoding.DecodeString(tok.Sig)
	if err != nil {
		return fmt.Errorf("decode signature: %w", err)
	}
	if !ed25519.Verify(pub, SignedMessage(tok.RootHex, tok.GenTime), sig) {
		return errors.New("signature does not verify")
	}
	return nil
}

// IssuedAt parses the token's generation time.
func (t Token) IssuedAt() (time.Time, error) {
	return time.Parse(time.RFC3339, t.GenTime)
}
