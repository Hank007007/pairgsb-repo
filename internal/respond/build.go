// Package respond assembles the headers that describe a download response.
//
// Build produces the status code and the header block. For a multipart
// response it also produces the boundary and the per-part headers; the bytes
// of each part are supplied by the caller.
package respond

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/textproto"
	"strconv"
	"strings"

	"rangecarve/internal/rangehdr"
)

// Object is the metadata a response needs in order to describe what it serves.
//
// Filename is the name the object was uploaded under, kept verbatim by the
// store; turning it into a Content-Disposition value is this package's job.
type Object struct {
	ETag         string
	LastModified string
	ContentType  string
	Filename     string
	CacheControl string
	Size         int64
}

// Part is one segment of a multipart response.
type Part struct {
	Span   rangehdr.Span
	Header textproto.MIMEHeader
}

// Response is a status code, a header block and, for a multipart body, the
// boundary and the ordered part headers.
type Response struct {
	Status   int
	Header   map[string][]string
	Boundary string
	Parts    []Part
}

// HeaderMap exposes the header block in the shape net/http expects.
func (r *Response) HeaderMap() map[string][]string {
	if r.Header == nil {
		r.Header = make(map[string][]string)
	}
	return r.Header
}

// Build assembles the response for obj given the spans that were selected.
//
// An empty spans slice means the whole object is served. One span yields a
// single part response, several spans yield a multipart/byteranges response.
func Build(obj Object, spans []rangehdr.Span) *Response {
	if len(spans) == 0 {
		return buildWhole(obj)
	}
	if len(spans) == 1 {
		return buildSingle(obj, spans[0])
	}
	return buildMulti(obj, spans)
}

func newResponse(status int) *Response {
	return &Response{Status: status, Header: make(map[string][]string)}
}

func (r *Response) set(key, value string) {
	r.Header[key] = []string{value}
}

func buildWhole(obj Object) *Response {
	resp := newResponse(200)
	resp.set("Content-Type", contentType(obj))
	resp.set("Accept-Ranges", "bytes")
	resp.set("Content-Length", strconv.FormatInt(obj.Size, 10))
	applyMetadata(resp, obj)
	applyDisposition(resp, obj)
	return resp
}

func buildSingle(obj Object, span rangehdr.Span) *Response {
	resp := newResponse(206)
	resp.set("Content-Type", contentType(obj))
	resp.set("Accept-Ranges", "bytes")
	resp.set("Content-Range", contentRange(span, obj.Size))
	resp.set("Content-Length", strconv.FormatInt(span.Len(), 10))
	applyMetadata(resp, obj)
	applyDisposition(resp, obj)
	return resp
}

func buildMulti(obj Object, spans []rangehdr.Span) *Response {
	resp := newResponse(206)

	partType := contentType(obj)
	parts := make([]Part, 0, len(spans))
	for _, span := range spans {
		head := make(textproto.MIMEHeader)
		head.Set("Content-Range", contentRange(span, obj.Size))
		head.Set("Content-Type", partType)
		parts = append(parts, Part{Span: span, Header: head})
	}

	writer := multipart.NewWriter(&bytes.Buffer{})
	resp.Boundary = writer.Boundary()
	resp.Parts = parts
	resp.set("Content-Type", "multipart/byteranges; boundary="+resp.Boundary)
	resp.set("Accept-Ranges", "bytes")
	applyMetadata(resp, obj)
	return resp
}

func applyMetadata(resp *Response, obj Object) {
	if obj.ETag != "" {
		resp.set("ETag", obj.ETag)
	}
	if obj.LastModified != "" {
		resp.set("Last-Modified", obj.LastModified)
	}
	if obj.CacheControl != "" {
		resp.set("Cache-Control", obj.CacheControl)
	}
}

func applyDisposition(resp *Response, obj Object) {
	if obj.Filename == "" {
		return
	}
	resp.set("Content-Disposition", dispositionValue(obj.Filename))
}

// dispositionValue builds the Content-Disposition value for one upload name.
// Names that stay printable ASCII ride in a quoted string; anything else gets
// an ASCII fallback plus an RFC 5987 filename* parameter, so the original
// name survives without ever putting a control character or a raw non-ASCII
// byte into the header.
func dispositionValue(filename string) string {
	if isPrintableASCII(filename) {
		return `attachment; filename="` + escapeQuoted(filename) + `"`
	}
	return `attachment; filename="` + escapeQuoted(asciiFallback(filename)) +
		`"; filename*=UTF-8''` + encodeRFC5987(filename)
}

// isPrintableASCII reports whether s holds only printable ASCII bytes.
func isPrintableASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 0x20 || s[i] > 0x7e {
			return false
		}
	}
	return true
}

// escapeQuoted renders s for a quoted string: `"` and `\` are backslash
// escaped and control characters are dropped, since they must never appear
// in a header value.
func escapeQuoted(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c < 0x20 || c == 0x7f:
			continue
		case c == '"' || c == '\\':
			b.WriteByte('\\')
		}
		b.WriteByte(c)
	}
	return b.String()
}

// asciiFallback maps s to printable ASCII, replacing every non-ASCII rune
// with _ and dropping control characters.
func asciiFallback(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 0x20 && r <= 0x7e:
			b.WriteRune(r)
		case r > 0x7f:
			b.WriteByte('_')
		}
	}
	return b.String()
}

// encodeRFC5987 percent-encodes s as an RFC 5987 ext-value, keeping only
// attr-chars verbatim.
func encodeRFC5987(s string) string {
	const attrChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!#$&+-.^_`|~"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if strings.IndexByte(attrChars, c) >= 0 {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}

func contentType(obj Object) string {
	if obj.ContentType != "" {
		return obj.ContentType
	}
	return "application/octet-stream"
}

func contentRange(span rangehdr.Span, size int64) string {
	return "bytes " + strconv.FormatInt(span.Start, 10) + "-" +
		strconv.FormatInt(span.End, 10) + "/" + strconv.FormatInt(size, 10)
}
