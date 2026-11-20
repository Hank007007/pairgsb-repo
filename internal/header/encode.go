// Package header 负责 RFC 5322 头值的编码、折行与头块渲染。
package header

import (
	"encoding/base64"
	"strings"
)

const (
	// lineLimit 是可打印行的折行上限（不含 CRLF）。
	lineLimit = 76
	// encodedWordPrefix 与 encodedWordSuffix 构成一个 UTF-8 base64 encoded-word 的外壳。
	encodedWordPrefix = "=?UTF-8?B?"
	encodedWordSuffix = "?="
)

// NeedsEncoding 报告头值是否需要按 RFC 2047 编码。
func NeedsEncoding(s string) bool {
	for _, r := range s {
		if r > 127 {
			return true
		}
	}
	return false
}

// EncodeValue 把任意头值转换成可以安全放进头块的形式。
//
// 纯 ASCII 值原样返回；含非 ASCII 字符时整体转为 base64 encoded-word。
func EncodeValue(s string) string {
	if !NeedsEncoding(s) {
		return s
	}
	return encodedWordPrefix + base64.StdEncoding.EncodeToString([]byte(s)) + encodedWordSuffix
}

// Fold 在超长的行上插入 CRLF + 空格，使其不超过可打印行上限。
func Fold(line string) string {
	if len(line) <= lineLimit {
		return line
	}
	var b strings.Builder
	for len(line) > lineLimit {
		b.WriteString(line[:lineLimit])
		b.WriteString("\r\n ")
		line = line[lineLimit:]
	}
	b.WriteString(line)
	return b.String()
}

// FoldHeader 把一个「名称: 值」形式的头行按需折叠。
func FoldHeader(name, value string) string {
	return Fold(name + ": " + value)
}
