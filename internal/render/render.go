// Package render turns untrusted log text into a form that is safe to hand
// to a terminal under one of the render policies.
package render

import (
	"strings"

	"termveil/internal/scan"
)

// Apply renders data according to p.
func Apply(p Policy, data []byte) string {
	out := filter(data, p.keepSGR)
	if p.Name == Width {
		out = scan.Cut(out, p.MaxWidth)
	}
	return out
}

// filter walks data once. Bytes are copied through unless they are control
// content. When keepColor is set, an SGR-shaped run is copied verbatim;
// otherwise a sequence the scanner recognises is rewritten as visible text.
// Classified control characters are spelled as backslash escapes.
func filter(data []byte, keepColor bool) string {
	var b strings.Builder
	b.Grow(len(data))
	for i := 0; i < len(data); {
		if data[i] == scan.ESC {
			if keepColor {
				if n := sgrLen(data[i:]); n > 0 {
					b.Write(data[i : i+n])
					i += n
					continue
				}
			}
			if seq, ok := scan.Next(data[i:]); ok {
				b.WriteString(scan.Visible(seq.Raw))
				i += len(seq.Raw)
				continue
			}
			b.WriteString(scan.Token(scan.ESC))
			i++
			continue
		}
		d := scan.DecodeRune(data[i:])
		if d.Class == scan.Text {
			b.WriteRune(d.Rune)
		} else {
			b.WriteString(scan.Token(d.Rune))
		}
		i += d.Size
	}
	return b.String()
}

// sgrLen reports the length of a white-listed SGR run at the start of data,
// or 0 when data does not open with one. Only digits and ';' are accepted in
// the parameter area, every parameter must be on the whitelist, and the run
// must be closed by 'm'; any intermediate byte or other final byte rejects it.
func sgrLen(data []byte) int {
	if len(data) < 3 || data[0] != scan.ESC || data[1] != '[' {
		return 0
	}
	i := 2
	for {
		value := 0
		start := i
		for i < len(data) && data[i] >= '0' && data[i] <= '9' {
			value = value*10 + int(data[i]-'0')
			if value > 1<<30 {
				return 0
			}
			i++
		}
		if i == start || !allowedSGR(value) {
			return 0
		}
		if i >= len(data) {
			return 0
		}
		if data[i] == 'm' {
			return i + 1
		}
		if data[i] != ';' {
			return 0
		}
		i++
	}
}

// allowedSGR reports whether parameter is an SGR value the color policy
// keeps: full reset, weight, foreground or background colors, including the
// bright variants.
func allowedSGR(value int) bool {
	switch {
	case value == 0 || value == 1 || value == 22:
		return true
	case value >= 30 && value <= 37 || value == 39:
		return true
	case value >= 40 && value <= 47 || value == 49:
		return true
	case value >= 90 && value <= 97:
		return true
	case value >= 100 && value <= 107:
		return true
	}
	return false
}
