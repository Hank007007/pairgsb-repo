package fetcher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"example.com/outfetch/internal/policy"
)

// ErrUpstream 表示请求已经发出去，但上游失败或超时。
var ErrUpstream = errors.New("fetcher: upstream request failed")

// Result 是一次抓取的摘要。正文不落盘、不回传，只留哈希和字节数。
type Result struct {
	Status    int    `json:"status"`
	Bytes     int    `json:"bytes"`
	Digest    string `json:"digest"`
	Truncated bool   `json:"truncated"`
}

// Fetcher 负责真正把请求发出去。
type Fetcher struct {
	Client       *http.Client
	Policy       *policy.Policy
	MaxBytes     int64
	MaxRedirects int
}

func New(p *policy.Policy) *Fetcher {
	return &Fetcher{
		Client:       &http.Client{Timeout: 10 * time.Second},
		Policy:       p,
		MaxBytes:     1 << 20,
		MaxRedirects: 3,
	}
}

// Get 抓取 rawURL，返回状态码、正文大小和 SHA-256。
func (f *Fetcher) Get(ctx context.Context, rawURL string) (*Result, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("%w: bad url", ErrUpstream)
	}
	if f.Policy != nil {
		if err := f.Policy.Check(u); err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("%w: build request", ErrUpstream)
	}
	req.Header.Set("User-Agent", "outfetch/1.4")

	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: read body", ErrUpstream)
	}

	sum := sha256.Sum256(body)
	return &Result{
		Status:    resp.StatusCode,
		Bytes:     len(body),
		Digest:    hex.EncodeToString(sum[:]),
		Truncated: false,
	}, nil
}
