package mime

// ContentDisposition 生成附件的 Content-Disposition 头值。
func ContentDisposition(filename string) string {
	return `attachment; filename="` + filename + `"`
}
