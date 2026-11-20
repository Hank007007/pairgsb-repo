// Package addr 负责地址列表的切分以及显示名、地址规格的格式化。
package addr

import (
	"strings"
)

// Address 是一个已经切分出来的地址条目。
type Address struct {
	// Display 是显示名部分，可能为空。
	Display string
	// Local 是地址规格中 @ 之前的部分。
	Local string
	// Domain 是地址规格中 @ 之后的部分。
	Domain string
}

// Spec 把 Local 与 Domain 重新拼成地址规格。
func (a Address) Spec() string {
	if a.Domain == "" {
		return a.Local
	}
	return a.Local + "@" + a.Domain
}

// SplitList 把逗号分隔的地址列表切分成条目。
func SplitList(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

// ParseOne 把一个地址条目拆成显示名与地址规格。
//
// 形如 `Name <local@domain>` 的条目取尖括号内的部分作为规格；
// 不带尖括号时整个条目作为规格。
func ParseOne(s string) Address {
	s = strings.TrimSpace(s)

	var display, spec string
	if i := strings.LastIndex(s, "<"); i >= 0 && strings.HasSuffix(s, ">") {
		display = strings.TrimSpace(s[:i])
		spec = strings.TrimSpace(s[i+1 : len(s)-1])
	} else {
		spec = s
	}

	local, domain, _ := SplitSpec(spec)
	return Address{Display: display, Local: local, Domain: domain}
}

// SplitSpec 把地址规格切成 local part 与域。
func SplitSpec(spec string) (local, domain string, ok bool) {
	i := strings.Index(spec, "@")
	if i <= 0 || i == len(spec)-1 {
		return "", "", false
	}
	return spec[:i], spec[i+1:], true
}

// DisplayName 把显示名格式化成可以直接放进头块的形式。
func DisplayName(name string) string {
	return name
}

// Format 组合显示名与地址规格。
func Format(a Address) string {
	name := DisplayName(a.Display)
	if name == "" {
		return a.Spec()
	}
	return name + " <" + a.Spec() + ">"
}
