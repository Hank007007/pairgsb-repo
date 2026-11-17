package policy

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
)

var (
	ErrScheme     = errors.New("policy: scheme not allowed")
	ErrNoHost     = errors.New("policy: url has no host")
	ErrBlockedIP  = errors.New("policy: address is not routable")
	ErrHostDenied = errors.New("policy: host not allowed")
	ErrPortDenied = errors.New("policy: port not allowed")
	ErrResolve    = errors.New("policy: cannot resolve host")
)

// Resolver 把主机名解析成 IP 列表。线上用系统解析，单测里换成固定的表。
type Resolver interface {
	LookupIP(host string) ([]net.IP, error)
}

type systemResolver struct{}

func (systemResolver) LookupIP(host string) ([]net.IP, error) {
	return net.LookupIP(host)
}

// Policy 是一次出方向请求的准入规则。字段公开，方便直接从配置填。
type Policy struct {
	AllowedHosts []string
	BlockedHosts []string
	AllowedPorts []int
	Resolver     Resolver
}

func New(allowed, blocked []string, ports []int, r Resolver) *Policy {
	if r == nil {
		r = systemResolver{}
	}
	return &Policy{
		AllowedHosts: allowed,
		BlockedHosts: blocked,
		AllowedPorts: ports,
		Resolver:     r,
	}
}

// Check 判断这个 URL 能不能发出去。不通过时返回上面几个哨兵错误之一。
func (p *Policy) Check(u *url.URL) error {
	_, err := p.CheckAndResolve(u)
	return err
}

// CheckAndResolve 在 Check 的基础上返回本次校验解析出的地址。
// fetcher 必须用这批地址建连，不能自己再解析一次，否则会留下 DNS rebinding 窗口。
func (p *Policy) CheckAndResolve(u *url.URL) ([]net.IP, error) {
	if u == nil {
		return nil, ErrNoHost
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, ErrScheme
	}
	host := u.Hostname()
	if host == "" {
		return nil, ErrNoHost
	}

	for _, b := range p.BlockedHosts {
		if hostMatches(host, b) {
			return nil, ErrHostDenied
		}
	}

	if len(p.AllowedHosts) > 0 {
		if !p.hostAllowed(host) {
			return nil, ErrHostDenied
		}
	}

	if len(p.AllowedPorts) > 0 && !p.IsPortAllowed(u) {
		return nil, ErrPortDenied
	}

	ips, err := p.resolve(host)
	if err != nil {
		return nil, ErrResolve
	}
	if len(ips) == 0 {
		return nil, ErrResolve
	}
	for _, ip := range ips {
		if isBlocked(ip) {
			return nil, ErrBlockedIP
		}
	}
	return ips, nil
}

// IsPortAllowed 判断端口在不在白名单里。URL 没写端口时按 scheme 取默认值。
func (p *Policy) IsPortAllowed(u *url.URL) bool {
	if u == nil {
		return false
	}
	port := u.Port()
	if port == "" {
		switch u.Scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		}
	}
	for _, allowed := range p.AllowedPorts {
		if port == strconv.Itoa(allowed) {
			return true
		}
	}
	return false
}

func (p *Policy) hostAllowed(host string) bool {
	for _, a := range p.AllowedHosts {
		if hostMatches(host, a) {
			return true
		}
	}
	return false
}

func (p *Policy) resolve(host string) ([]net.IP, error) {
	ip := net.ParseIP(host)
	if ip != nil {
		return []net.IP{ip}, nil
	}
	r := p.Resolver
	if r == nil {
		r = systemResolver{}
	}
	return r.LookupIP(host)
}

// hostMatches 实现规范里的两种写法：example.com 只匹配自身，
// *.example.com 只匹配子域，不匹配 example.com 本身。
func hostMatches(host, entry string) bool {
	h := normalizeHost(host)
	a := normalizeHost(entry)
	if h == "" || a == "" {
		return false
	}
	if strings.HasPrefix(a, "*.") {
		suffix := a[1:]
		return len(suffix) > 1 && len(h) > len(suffix) && strings.HasSuffix(h, suffix)
	}
	return h == a
}

func normalizeHost(host string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
}

func isBlocked(ip net.IP) bool {
	if ip == nil {
		return true
	}
	v4 := ip.To4()
	if v4 != nil {
		ip = v4
	}
	if ip.IsUnspecified() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return true
	}
	if v4 != nil {
		switch {
		case v4[0] == 10:
			return true
		case v4[0] == 192 && v4[1] == 168:
			return true
		case v4[0] == 172 && v4[1]&0xf0 == 16:
			return true
		case v4[0] == 100 && v4[1]&0xc0 == 64:
			return true
		}
		return false
	}
	return ip.IsPrivate()
}
