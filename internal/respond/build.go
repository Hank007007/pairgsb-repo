// Package respond assembles the headers that describe a download response.
//
// Build produces the status code and the header block. For a multipart
// response it also produces the boundary and the per-part headers; the bytes
// of each part are supplied by the caller.
package respond

import (
	"bytes"
	"mime/multipart"
	"net/textproto"
	"strconv"

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
		if obj.Filename != "" {
			head.Set("Content-Disposition", dispositionValue(obj.Filename))
		}
		parts = append(parts, Part{Span: span, Header: head})
	}

	writer := multipart.NewWriter(&bytes.Buffer{})
	resp.Boundary = writer.Boundary()
	resp.Parts = parts
	resp.set("Content-Type", "multipart/byteranges; boundary="+resp.Boundary)
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

// dispositionValue builds the attachment header for one upload name.
func dispositionValue(filename string) string {
	return `attachment; filename="` + filename + `"`
}

func contentType(obj Object) string {
	if obj.ContentType != "" {
		return obj.ContentType
	}
	return "application/octet-stream"
}

func contentRange(span rangehdr.Span, size int64) string {
	return "bytes " + strconv.FormatInt(span.Start, 10) + "-" +
		strconv.FormatInt(span.End+1, 10) + "/" + strconv.FormatInt(size, 10)
}
