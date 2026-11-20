package header

import (
	"strings"
)

// Field 是头块中的一个逻辑字段。值必须已经完成编码。
type Field struct {
	Name  string
	Value string
}

// Render 把若干字段渲染成头块，每个字段后跟 CRLF。
//
// 值会经过一次折行；字段之间不插入空行——空行由调用方在头块结束时补上。
func Render(fields []Field) string {
	var b strings.Builder
	for _, f := range fields {
		b.WriteString(FoldHeader(f.Name, f.Value))
		b.WriteString("\r\n")
	}
	return b.String()
}
