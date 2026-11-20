package mime

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// Part 是一份多部分报文的组成部分。
type Part struct {
	// ContentType 是完整的内容类型，例如 `text/plain; charset=utf-8`。
	ContentType string
	// ContentID 为空时不输出 Content-ID。
	ContentID string
	// Disposition 是 Content-Disposition 的值；为空时输出 inline。
	Disposition string
	// Body 是已经完成传输编码之前的内容。
	Body []byte
	// Binary 为真时用 base64 传输编码，否则按 7bit 直传。
	Binary bool
}

// NewBoundary 生成一个随机 boundary。
func NewBoundary() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// 随机源不可用时退回固定长度的时间无关字符串。
		return "=_headlace_boundary_0000000000000000"
	}
	return "=_headlace_" + hex.EncodeToString(buf[:])
}

// RenderBody 按给定的 boundary 组装正文及其各个 part。
//
// 返回的字节不含头块，以一个空行开始后的第一段正文起始。
func RenderBody(boundary string, body []Part) []byte {
	var b strings.Builder
	for _, p := range body {
		b.WriteString("--")
		b.WriteString(boundary)
		b.WriteString("\r\n")
		b.WriteString("Content-Type: ")
		b.WriteString(p.ContentType)
		b.WriteString("\r\n")
		if p.ContentID != "" {
			b.WriteString("Content-ID: <")
			b.WriteString(p.ContentID)
			b.WriteString(">\r\n")
		}
		disp := p.Disposition
		if disp == "" {
			disp = "inline"
		}
		b.WriteString("Content-Disposition: ")
		b.WriteString(disp)
		b.WriteString("\r\n")
		if p.Binary {
			b.WriteString("Content-Transfer-Encoding: base64\r\n")
		} else {
			b.WriteString("Content-Transfer-Encoding: 7bit\r\n")
		}
		b.WriteString("\r\n")
		if p.Binary {
			b.WriteString(base64.StdEncoding.EncodeToString(p.Body))
		} else {
			b.Write(p.Body)
		}
		b.WriteString("\r\n")
	}
	b.WriteString("--")
	b.WriteString(boundary)
	b.WriteString("--\r\n")
	return []byte(b.String())
}

// ContentTypeFor 返回某个 part 的 Content-Type，正文部分统一声明 UTF-8。
func ContentTypeFor(kind string) string {
	return fmt.Sprintf("%s; charset=utf-8", kind)
}
