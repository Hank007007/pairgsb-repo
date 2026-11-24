package acl

import "strings"

// Match reports whether topic matches filter. The rules are described in
// docs/matching.md: literals compare exactly, "+" stands for one level, and
// "#" stands for the remaining levels.
func Match(filter, topic string) bool {
	filter = strings.Trim(filter, "/")
	topic = strings.Trim(topic, "/")

	f := strings.Split(filter, "/")
	t := strings.Split(topic, "/")

	fi, ti := 0, 0
	for fi < len(f) {
		if f[fi] == "#" {
			return ti < len(t)
		}
		if ti >= len(t) {
			return false
		}
		if f[fi] == "+" {
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

// probeLevel is the literal used when expanding a wildcard level.
const probeLevel = "x"

// probeDepth is how many trailing levels an expanded "#" may contribute.
const probeDepth = 3

// Expand returns a representative set of concrete topics described by filter.
// A "+" level is filled in with a literal level. A "#" level contributes the
// filter prefix itself and prefixes with one to probeDepth extra levels.
func Expand(filter string) []string {
	levels := strings.Split(filter, "/")

	var out []string
	var walk func(int, []string)
	walk = func(i int, acc []string) {
		if i == len(levels) {
			out = append(out, strings.Join(acc, "/"))
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
