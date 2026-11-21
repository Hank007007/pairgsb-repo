// Package token validates access tokens against the authorization server's
// introspection endpoint and caches the results.
package token

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
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
}

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
	c.store(accessToken, info)
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
		log.Printf("token: introspection reported inactive token")
	}

	info := &Info{Subject: r.Sub, Scope: r.Scope}
	if r.CNF != nil {
		info.CNFJKT = r.CNF.JKT
	}
	return info, nil
}

func (c *Client) cacheKey(accessToken string) string {
	return accessToken
}

func (c *Client) fromCache(accessToken string) (*Info, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.cache[c.cacheKey(accessToken)]
	if !ok {
		return nil, false
	}
	info := entry.info
	return &info, true
}

func (c *Client) store(accessToken string, info *Info) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache[c.cacheKey(accessToken)] = cacheEntry{
		info:      *info,
		expiresAt: c.now().Add(c.TTL),
	}
}

func (c *Client) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}
