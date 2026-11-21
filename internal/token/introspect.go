// Package token validates access tokens against the authorization server's
// introspection endpoint and caches the results.
package token

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Info is the subset of the introspection response the resource server keeps.
type Info struct {
	Subject string
	Scope   string
	CNFJKT  string

	exp *int64
}

// ErrUnauthenticated is returned when the introspection endpoint answered
// but the token is not authenticated (inactive, expired, or unbound).
var ErrUnauthenticated = errors.New("token: not authenticated")

// Client talks to the introspection endpoint.
type Client struct {
	Endpoint     string
	ClientID     string
	ClientSecret string
	HTTP         *http.Client
	TTL          time.Duration
	Now          func() time.Time

	mu    sync.Mutex
	cache map[string]cacheEntry
}

type cacheEntry struct {
	info      Info
	expiresAt time.Time
}

type response struct {
	Active bool   `json:"active"`
	Sub    string `json:"sub"`
	Scope  string `json:"scope"`
	Exp    *int64 `json:"exp"`
	CNF    *struct {
		JKT string `json:"jkt"`
	} `json:"cnf"`
}

// NewClient returns a client with sensible HTTP defaults.
func NewClient(endpoint, id, secret string) *Client {
	return &Client{
		Endpoint:     endpoint,
		ClientID:     id,
		ClientSecret: secret,
		HTTP:         &http.Client{Timeout: 5 * time.Second},
		TTL:          10 * time.Minute,
		Now:          time.Now,
		cache:        make(map[string]cacheEntry),
	}
}

// Introspect validates the access token and returns its metadata.
func (c *Client) Introspect(ctx context.Context, accessToken string) (*Info, error) {
	if info, ok := c.fromCache(accessToken); ok {
		return info, nil
	}

	info, err := c.fetch(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	lifetime := c.TTL
	if info.exp != nil {
		if remaining := time.Unix(*info.exp, 0).Sub(c.now()); remaining < lifetime {
			lifetime = remaining
		}
	}
	if lifetime > 0 {
		c.store(accessToken, info, lifetime)
	}
	return info, nil
}

func (c *Client) fetch(ctx context.Context, accessToken string) (*Info, error) {
	form := url.Values{}
	form.Set("token", accessToken)
	form.Set("token_type_hint", "access_token")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint,
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if c.ClientID != "" || c.ClientSecret != "" {
		basic := base64.StdEncoding.EncodeToString([]byte(c.ClientID + ":" + c.ClientSecret))
		req.Header.Set("Authorization", "Basic "+basic)
	}

	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(io.LimitReader(res.Body, 64<<10))
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token: introspection status %d", res.StatusCode)
	}

	var r response
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("token: introspection response: %w", err)
	}
	if !r.Active {
		return nil, fmt.Errorf("%w: introspection reported inactive", ErrUnauthenticated)
	}
	if r.Exp != nil && *r.Exp <= c.now().Unix() {
		return nil, fmt.Errorf("%w: introspection reported expired", ErrUnauthenticated)
	}
	if r.CNF == nil || r.CNF.JKT == "" {
		return nil, fmt.Errorf("%w: introspection response missing cnf.jkt", ErrUnauthenticated)
	}

	info := &Info{Subject: r.Sub, Scope: r.Scope, CNFJKT: r.CNF.JKT, exp: r.Exp}
	return info, nil
}

// cacheKey derives the cache key from the access token. The raw token is a
// bearer secret and is never used as a map key.
func (c *Client) cacheKey(accessToken string) string {
	sum := sha256.Sum256([]byte(accessToken))
	return hex.EncodeToString(sum[:])
}

func (c *Client) fromCache(accessToken string) (*Info, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := c.cacheKey(accessToken)
	entry, ok := c.cache[key]
	if !ok {
		return nil, false
	}
	if !entry.expiresAt.After(c.now()) {
		delete(c.cache, key)
		return nil, false
	}
	info := entry.info
	return &info, true
}

func (c *Client) store(accessToken string, info *Info, lifetime time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache[c.cacheKey(accessToken)] = cacheEntry{
		info:      *info,
		expiresAt: c.now().Add(lifetime),
	}
}

func (c *Client) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}
