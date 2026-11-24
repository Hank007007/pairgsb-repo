package acl

import (
	"strings"

	"topicwarden/internal/tenant"
)

// Match reports whether topic matches filter. The rules are described in
// docs/matching.md: literals compare exactly, "+" stands for one level, and
// "#" stands for the remaining levels.
func Match(filter, topic string) bool {
	f := strings.Split(tenant.NormalizeTopic(filter), "/")
	t := strings.Split(tenant.NormalizeTopic(topic), "/")
	if !validFilterLevels(f) {
		return false
	}

	fi, ti := 0, 0
	for fi < len(f) {
		if f[fi] == "#" {
			return true
		}
		if ti >= len(t) {
			return false
		}
		if f[fi] == "+" {
			if t[ti] == "" {
				return false
			}
			fi++
			ti++
			continue
		}
		if f[fi] != t[ti] {
			return false
		}
		fi++
		ti++
	}
	return ti == len(t)
}

// ValidFilter reports whether filter follows the MQTT wildcard grammar used
// by this package. In particular, "#" may only occupy the final level.
func ValidFilter(filter string) bool {
	return validFilterLevels(strings.Split(tenant.NormalizeTopic(filter), "/"))
}

func validFilterLevels(levels []string) bool {
	for i, level := range levels {
		if level == "#" && i != len(levels)-1 {
			return false
		}
	}
	return true
}

// probeLevel is the literal used when expanding a wildcard level.
const probeLevel = "x"

// probeDepth is how many trailing levels an expanded "#" may contribute.
const probeDepth = 3

// Expand returns a representative set of concrete topics described by filter.
// A "+" level is filled in with a literal level. A "#" level contributes the
// filter prefix itself and prefixes with one to probeDepth extra levels.
// Malformed filters have no expansion.
func Expand(filter string) []string {
	filter = tenant.NormalizeTopic(filter)
	if !ValidFilter(filter) {
		return nil
	}
	levels := strings.Split(filter, "/")

	var out []string
	var walk func(int, []string)
	walk = func(i int, acc []string) {
		if i == len(levels) {
			topic := strings.Join(acc, "/")
			if topic != "" {
				out = append(out, topic)
			}
			return
		}
		switch levels[i] {
		case "+":
			walk(i+1, append(acc, probeLevel))
		case "#":
			walk(i+1, acc)
			for n := 1; n <= probeDepth; n++ {
				next := make([]string, len(acc), len(acc)+n)
				copy(next, acc)
				for k := 0; k < n; k++ {
					next = append(next, probeLevel)
				}
				walk(i+1, next)
			}
		default:
			walk(i+1, append(acc, levels[i]))
		}
	}
	walk(0, nil)
	return out
}
