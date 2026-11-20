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
			b.WriteByte(data[i])
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

// sgrLen reports the length of an SGR-shaped run at the start of data, or 0
// when data does not open with one. An SGR run opens with ESC [, carries
// parameter and intermediate bytes, and is closed by m.
func sgrLen(data []byte) int {
	if len(data) < 3 || data[0] != scan.ESC || data[1] != '[' {
		return 0
	}
	i := 2
	for i < len(data) && data[i] >= 0x20 && data[i] <= 0x3f {
		i++
	}
	if i < len(data) && data[i] == 'm' {
		return i + 1
	}
	return 0
}
