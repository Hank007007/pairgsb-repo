package fetcher

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"example.com/outfetch/internal/policy"
)

// ErrUpstream 表示请求已经发出去，但上游失败或超时。
var ErrUpstream = errors.New("fetcher: upstream request failed")

// ErrTooManyRedirects 表示重定向跳数超过 MaxRedirects。
var ErrTooManyRedirects = errors.New("fetcher: too many redirects")

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
	dial         func(context.Context, string, string) (net.Conn, error)
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

	if f.Client != nil && f.Client.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, f.Client.Timeout)
		defer cancel()
	}

	maxRedirects := f.MaxRedirects
	if maxRedirects < 0 {
		maxRedirects = 0
	}
	followed := 0

	for {
		ips, err := f.check(u)
		if err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("%w: build request", ErrUpstream)
		}
		req.Header.Set("User-Agent", "outfetch/1.4")

		resp, err := f.clientFor(ips).Do(req)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrUpstream, err)
		}

		if isRedirect(resp.StatusCode) && resp.Header.Get("Location") != "" {
			next, locErr := resp.Location()
			resp.Body.Close()
			if locErr != nil {
				return nil, fmt.Errorf("%w: bad redirect location", ErrUpstream)
			}
			if followed >= maxRedirects {
				return nil, ErrTooManyRedirects
			}
			followed++
			u = next
			continue
		}

		n, digest, truncated, readErr := readBody(resp.Body, f.MaxBytes)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("%w: read body", ErrUpstream)
		}
		return &Result{
			Status:    resp.StatusCode,
			Bytes:     int(n),
			Digest:    digest,
			Truncated: truncated,
		}, nil
	}
}

func (f *Fetcher) check(u *url.URL) ([]net.IP, error) {
	if f.Policy != nil {
		return f.Policy.CheckAndResolve(u)
	}
	if u == nil || u.Hostname() == "" {
		return nil, policy.ErrNoHost
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, policy.ErrScheme
	}
	host := u.Hostname()
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return nil, policy.ErrResolve
	}
	if len(ips) == 0 {
		return nil, policy.ErrResolve
	}
	return ips, nil
}

func (f *Fetcher) clientFor(ips []net.IP) *http.Client {
	client := &http.Client{}
	if f.Client != nil {
		c := *f.Client
		client = &c
	}
	client.Transport = f.pinnedTransport(ips)
	client.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return client
}

func (f *Fetcher) pinnedTransport(ips []net.IP) *http.Transport {
	dial := f.dial
	if dial == nil {
		dial = (&net.Dialer{}).DialContext
	}

	base := http.DefaultTransport.(*http.Transport)
	if f.Client != nil {
		if tr, ok := f.Client.Transport.(*http.Transport); ok && tr != nil {
			base = tr
		}
	}
	t := base.Clone()
	t.Proxy = nil
	t.DisableKeepAlives = true
	t.DialTLS = nil
	t.DialTLSContext = nil
	t.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		_, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		var lastErr error
		for _, ip := range ips {
			conn, err := dial(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}
		if lastErr == nil {
			lastErr = errors.New("no approved address")
		}
		return nil, lastErr
	}
	return t
}

func isRedirect(code int) bool {
	switch code {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther,
		http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return true
	default:
		return false
	}
}

func readBody(r io.Reader, max int64) (int64, string, bool, error) {
	if max < 0 {
		max = 0
	}
	h := sha256.New()
	n, err := io.CopyN(h, r, max)
	if err != nil && !errors.Is(err, io.EOF) {
		return 0, "", false, err
	}

	truncated := false
	if err == nil {
		var probe [1]byte
		got, probeErr := r.Read(probe[:])
		if got > 0 {
			truncated = true
		} else if probeErr != nil && !errors.Is(probeErr, io.EOF) {
			return 0, "", false, probeErr
		}
	}
	return n, hex.EncodeToString(h.Sum(nil)), truncated, nil
}
