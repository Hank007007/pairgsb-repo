package policy

import (
	"errors"
	"net"
	"net/url"
	"testing"
)

type recordingResolver struct {
	ips   map[string][]net.IP
	err   error
	calls int
}

func (r *recordingResolver) LookupIP(host string) ([]net.IP, error) {
	r.calls++
	if r.err != nil {
		return nil, r.err
	}
	if ips, ok := r.ips[host]; ok {
		return ips, nil
	}
	return []net.IP{net.ParseIP("93.184.216.34")}, nil
}

func TestAllowlistExactEntryDoesNotMatchSuffix(t *testing.T) {
	p := New([]string{"shop.example.com"}, nil, nil, publicResolver{})
	for _, raw := range []string{
		"http://notshop.example.com/",
		"http://attacker-shop.example.com/",
		"http://shop.example.com.attacker.tld/",
	} {
		if err := p.Check(mustURL(t, raw)); !errors.Is(err, ErrHostDenied) {
			t.Errorf("%s: want ErrHostDenied, got %v", raw, err)
		}
	}
	if err := p.Check(mustURL(t, "http://shop.example.com/")); err != nil {
		t.Fatalf("exact entry should match: %v", err)
	}
}

func TestAllowlistWildcardDoesNotMatchApexOrLookalike(t *testing.T) {
	p := New([]string{"*.example.com"}, nil, nil, publicResolver{})
	for _, raw := range []string{
		"http://example.com/",
		"http://notexample.com/",
		"http://.example.com/",
	} {
		if err := p.Check(mustURL(t, raw)); !errors.Is(err, ErrHostDenied) {
			t.Errorf("%s: want ErrHostDenied, got %v", raw, err)
		}
	}
	for _, raw := range []string{"http://a.example.com/", "http://a.b.example.com/"} {
		if err := p.Check(mustURL(t, raw)); err != nil {
			t.Errorf("%s: wildcard should match: %v", raw, err)
		}
	}
}

func TestBlockedHostsWinOverAllowlist(t *testing.T) {
	p := New([]string{"*.example.com"}, []string{"evil.example.com"}, nil, publicResolver{})
	if err := p.Check(mustURL(t, "http://evil.example.com/")); !errors.Is(err, ErrHostDenied) {
		t.Fatalf("block_hosts should win: %v", err)
	}
	p = New([]string{"*.example.com"}, []string{"*.blocked.example.com"}, nil, publicResolver{})
	if err := p.Check(mustURL(t, "http://a.blocked.example.com/")); !errors.Is(err, ErrHostDenied) {
		t.Fatalf("wildcard block_hosts should match: %v", err)
	}
}

func TestBlocksEveryDocumentedAddressRange(t *testing.T) {
	p := New(nil, nil, nil, publicResolver{})
	for _, raw := range []string{
		"http://127.0.0.1/",
		"http://127.255.255.255/",
		"http://[::1]/",
		"http://10.0.0.1/",
		"http://10.255.255.255/",
		"http://172.16.0.1/",
		"http://172.31.255.255/",
		"http://192.168.0.1/",
		"http://169.254.169.254/latest/meta-data/",
		"http://[fe80::1]/",
		"http://0.0.0.0/",
		"http://[::]/",
		"http://100.64.0.1/",
		"http://100.127.255.255/",
		"http://[::ffff:127.0.0.1]/",
	} {
		if err := p.Check(mustURL(t, raw)); !errors.Is(err, ErrBlockedIP) {
			t.Errorf("%s: want ErrBlockedIP, got %v", raw, err)
		}
	}
}

func TestAllowsPublicBoundaryAddresses(t *testing.T) {
	p := New(nil, nil, nil, publicResolver{})
	for _, raw := range []string{
		"http://100.63.255.255/",
		"http://100.128.0.1/",
		"http://172.15.255.255/",
		"http://172.32.0.1/",
		"http://169.253.255.255/",
		"http://169.255.0.1/",
	} {
		if err := p.Check(mustURL(t, raw)); err != nil {
			t.Errorf("%s: should be allowed, got %v", raw, err)
		}
	}
}

func TestPrivateHostnameIsRejectedWithoutAllowlist(t *testing.T) {
	r := &recordingResolver{ips: map[string][]net.IP{
		"metadata.internal": {net.ParseIP("10.12.34.56")},
	}}
	p := New(nil, nil, nil, r)
	if err := p.Check(mustURL(t, "http://metadata.internal/latest/meta-data/")); !errors.Is(err, ErrBlockedIP) {
		t.Fatalf("private hostname should be rejected: %v", err)
	}
}

func TestOneBadResolvedAddressRejectsWholeHost(t *testing.T) {
	r := &recordingResolver{ips: map[string][]net.IP{
		"mixed.example": {net.ParseIP("93.184.216.34"), net.ParseIP("10.0.0.8")},
	}}
	p := New(nil, nil, nil, r)
	if err := p.Check(mustURL(t, "http://mixed.example/")); !errors.Is(err, ErrBlockedIP) {
		t.Fatalf("one blocked address must reject the host: %v", err)
	}
}

func TestMetadataAddressRejectedEvenWhenExplicitlyAllowlisted(t *testing.T) {
	p := New([]string{"169.254.169.254"}, nil, nil, publicResolver{})
	if err := p.Check(mustURL(t, "http://169.254.169.254/latest/meta-data/")); !errors.Is(err, ErrBlockedIP) {
		t.Fatalf("blocked address must not be rescued by allow_hosts: %v", err)
	}
}

func TestCheckEnforcesPortAllowlist(t *testing.T) {
	p := New([]string{"shop.example.com"}, nil, []int{80, 443}, publicResolver{})
	cases := map[string]error{
		"http://shop.example.com/":       nil,
		"https://shop.example.com/":      nil,
		"http://shop.example.com:80/":    nil,
		"https://shop.example.com:443/":  nil,
		"http://shop.example.com:2222/":  ErrPortDenied,
		"https://shop.example.com:8443/": ErrPortDenied,
	}
	for raw, want := range cases {
		err := p.Check(mustURL(t, raw))
		if want == nil && err != nil {
			t.Errorf("%s: want allowed, got %v", raw, err)
		}
		if want != nil && !errors.Is(err, want) {
			t.Errorf("%s: want %v, got %v", raw, want, err)
		}
	}
}

func TestCheckAndResolveUsesResolverOnce(t *testing.T) {
	r := &recordingResolver{ips: map[string][]net.IP{
		"shop.example.com": {net.ParseIP("93.184.216.34")},
	}}
	p := New([]string{"shop.example.com"}, nil, []int{80}, r)
	ips, err := p.CheckAndResolve(mustURL(t, "http://shop.example.com/"))
	if err != nil {
		t.Fatalf("CheckAndResolve: %v", err)
	}
	if len(ips) != 1 || !ips[0].Equal(net.ParseIP("93.184.216.34")) {
		t.Fatalf("unexpected addresses: %v", ips)
	}
	if r.calls != 1 {
		t.Fatalf("resolver should be called once, got %d", r.calls)
	}
}

func TestResolveFailureIsReported(t *testing.T) {
	r := &recordingResolver{err: errors.New("no such host")}
	p := New(nil, nil, nil, r)
	if err := p.Check(mustURL(t, "http://missing.example/")); !errors.Is(err, ErrResolve) {
		t.Fatalf("want ErrResolve, got %v", err)
	}
}

func TestHostnameComparisonIsCaseInsensitiveWithTrailingDot(t *testing.T) {
	p := New([]string{"Shop.Example.COM."}, nil, nil, publicResolver{})
	if err := p.Check(mustURL(t, "http://SHOP.example.com./")); err != nil {
		t.Fatalf("trailing dot and case should not bypass exact matching: %v", err)
	}
}

func TestInvalidURLDoesNotPanic(t *testing.T) {
	p := New(nil, nil, nil, publicResolver{})
	var u *url.URL
	if err := p.Check(u); !errors.Is(err, ErrNoHost) {
		t.Fatalf("nil url: want ErrNoHost, got %v", err)
	}
}
