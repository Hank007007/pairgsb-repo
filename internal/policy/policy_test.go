package policy

import (
	"net"
	"net/url"
	"testing"
)

// publicResolver 把所有主机名都解析成一个公网地址，让依赖解析的用例结果稳定。
type publicResolver struct {
	ips map[string][]net.IP
}

func (r publicResolver) LookupIP(host string) ([]net.IP, error) {
	if ips, ok := r.ips[host]; ok {
		return ips, nil
	}
	return []net.IP{net.ParseIP("93.184.216.34")}, nil
}

func mustURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return u
}

func TestRejectsNonHTTPScheme(t *testing.T) {
	p := New(nil, nil, nil, publicResolver{})
	for _, raw := range []string{"ftp://example.com/a", "file:///etc/passwd", "gopher://example.com/"} {
		if err := p.Check(mustURL(t, raw)); err != ErrScheme {
			t.Errorf("%s: want ErrScheme, got %v", raw, err)
		}
	}
}

func TestRejectsEmptyHost(t *testing.T) {
	p := New(nil, nil, nil, publicResolver{})
	if err := p.Check(mustURL(t, "http:///just-a-path")); err != ErrNoHost {
		t.Fatalf("want ErrNoHost, got %v", err)
	}
	if err := p.Check(nil); err != ErrNoHost {
		t.Fatalf("nil url: want ErrNoHost, got %v", err)
	}
}

func TestBlocksLoopbackAndRFC1918(t *testing.T) {
	p := New(nil, nil, nil, publicResolver{})
	for _, raw := range []string{
		"http://127.0.0.1/",
		"http://127.9.9.9/x",
		"http://10.0.0.7/",
		"http://10.255.255.254/",
		"http://192.168.1.1/",
		"http://172.16.4.4/",
		"http://172.31.255.255/",
	} {
		if err := p.Check(mustURL(t, raw)); err == nil {
			t.Errorf("%s: 这个地址应当被拒", raw)
		}
	}
}

func TestAllowsPublicAddress(t *testing.T) {
	p := New(nil, nil, nil, publicResolver{})
	for _, raw := range []string{
		"http://93.184.216.34/",
		"https://1.1.1.1/path",
		"http://172.32.0.1/",
		"http://11.0.0.1/",
	} {
		if err := p.Check(mustURL(t, raw)); err != nil {
			t.Errorf("%s: 应当放行，实际 %v", raw, err)
		}
	}
}

func TestBlockedHostIsRejected(t *testing.T) {
	p := New(nil, []string{"metadata.internal"}, nil, publicResolver{})
	if err := p.Check(mustURL(t, "http://metadata.internal/latest")); err == nil {
		t.Fatal("block_hosts 命中时应当拒绝")
	}
}

func TestAllowlistExactEntry(t *testing.T) {
	p := New([]string{"shop.example.com"}, nil, nil, publicResolver{})
	if err := p.Check(mustURL(t, "http://shop.example.com/")); err != nil {
		t.Fatalf("名单内的主机应当放行，实际 %v", err)
	}
	if err := p.Check(mustURL(t, "http://other.org/")); err == nil {
		t.Fatal("名单外的主机应当拒绝")
	}
}

func TestAllowlistWildcardEntry(t *testing.T) {
	p := New([]string{"*.example.com"}, nil, nil, publicResolver{})
	for _, raw := range []string{"http://a.example.com/", "http://a.b.example.com/"} {
		if err := p.Check(mustURL(t, raw)); err != nil {
			t.Errorf("%s: 通配项应当匹配，实际 %v", raw, err)
		}
	}
}

func TestAllowlistIsCaseInsensitive(t *testing.T) {
	p := New([]string{"Shop.Example.COM"}, nil, nil, publicResolver{})
	if err := p.Check(mustURL(t, "http://SHOP.example.com/")); err != nil {
		t.Fatalf("大小写不同应当视为同一主机，实际 %v", err)
	}
}

func TestResolverIsInjectable(t *testing.T) {
	r := publicResolver{}
	p := New(nil, nil, nil, r)
	if p.Resolver == nil {
		t.Fatal("Resolver 字段不应为空")
	}
	got, err := p.Resolver.LookupIP("example.com")
	if err != nil || len(got) != 1 {
		t.Fatalf("LookupIP: %v %v", got, err)
	}
}

func TestIsPortAllowed(t *testing.T) {
	p := New(nil, nil, []int{80, 443, 8080}, publicResolver{})
	cases := map[string]bool{
		"http://shop.example.com/a":      true,
		"https://shop.example.com:443/a": true,
		"http://shop.example.com:8080/a": true,
		"http://shop.example.com:22/a":   false,
	}
	for raw, want := range cases {
		if got := p.IsPortAllowed(mustURL(t, raw)); got != want {
			t.Errorf("%s: want %v, got %v", raw, want, got)
		}
	}
}
