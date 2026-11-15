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
	if u == nil {
		return ErrNoHost
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ErrScheme
	}
	host := u.Hostname()
	if host == "" {
		return ErrNoHost
	}

	for _, b := range p.BlockedHosts {
		if strings.EqualFold(host, strings.TrimSpace(b)) {
			return ErrHostDenied
		}
	}

	if len(p.AllowedHosts) > 0 {
		return p.checkAllowlist(host)
	}
	return p.checkLiteralAddress(host)
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

func (p *Policy) checkAllowlist(host string) error {
	h := strings.ToLower(host)
	for _, a := range p.AllowedHosts {
		a = strings.ToLower(strings.TrimSpace(a))
		if a == "" {
			continue
		}
		if strings.HasPrefix(a, "*.") {
			if strings.HasSuffix(h, a[1:]) {
				return nil
			}
			continue
		}
		if strings.HasSuffix(h, a) {
			return nil
		}
	}
	return ErrHostDenied
}

func (p *Policy) checkLiteralAddress(host string) error {
	ip := net.ParseIP(host)
	if ip == nil {
		return nil
	}
	if isBlocked(ip) {
		return ErrBlockedIP
	}
	return nil
}

func isBlocked(ip net.IP) bool {
	v4 := ip.To4()
	if v4 == nil {
		return false
	}
	switch {
	case v4[0] == 127:
		return true
	case v4[0] == 10:
		return true
	case v4[0] == 192 && v4[1] == 168:
		return true
	case v4[0] == 172 && v4[1]&0xf0 == 16:
		return true
	}
	return false
}
