// Package smtp 是报文组装的入口。它把上游的 Message 交给 header、addr、mime
// 三个包处理，产出一份可以直接交给 net/smtp 的字节流。
package smtp

import (
	"errors"
	"strings"

	"headlace/internal/addr"
	"headlace/internal/header"
	"headlace/internal/mime"
)

// Attachment 是一个待发送的附件。
type Attachment struct {
	Filename string
	Data     []byte
}

// Message 是上游交给组装层的结构化通知。
//
// 所有字段都来自运营后台或终端用户，按不可信输入对待。
type Message struct {
	From            string
	To              string
	ReplyTo         string
	ListUnsubscribe string
	Subject         string
	Text            string
	HTML            string
	Attachments     []Attachment
}

// Compose 把 Message 组装成一份完整的 RFC 5322 报文。
func Compose(msg Message) ([]byte, error) {
	if strings.TrimSpace(msg.From) == "" {
		return nil, errors.New("smtp: message has no From address")
	}

	from := addr.Format(addr.ParseOne(msg.From))

	recipients := addr.SplitList(msg.To)
	formatted := make([]string, 0, len(recipients))
	for _, r := range recipients {
		formatted = append(formatted, addr.Format(addr.ParseOne(r)))
	}

	fields := []header.Field{
		{Name: "From", Value: from},
		{Name: "To", Value: strings.Join(formatted, ", ")},
	}
	if msg.ReplyTo != "" {
		fields = append(fields, header.Field{Name: "Reply-To", Value: msg.ReplyTo})
	}
	if msg.ListUnsubscribe != "" {
		fields = append(fields, header.Field{Name: "List-Unsubscribe", Value: msg.ListUnsubscribe})
	}
	fields = append(fields,
		header.Field{Name: "Subject", Value: header.EncodeValue(msg.Subject)},
		header.Field{Name: "MIME-Version", Value: "1.0"},
	)

	var body strings.Builder
	if len(msg.Attachments) == 0 && msg.HTML == "" {
		fields = append(fields, header.Field{
			Name:  "Content-Type",
			Value: mime.ContentTypeFor("text/plain"),
		})
		fields = append(fields,
			header.Field{Name: "Content-Transfer-Encoding", Value: "7bit"},
		)
		body.WriteString(msg.Text)
		body.WriteString("\r\n")
	} else {
		boundary := mime.NewBoundary()
		fields = append(fields, header.Field{
			Name:  "Content-Type",
			Value: `multipart/mixed; boundary="` + boundary + `"`,
		})

		parts := make([]mime.Part, 0, 2+len(msg.Attachments))
		parts = append(parts, mime.Part{
			ContentType: mime.ContentTypeFor("text/plain"),
			Body:        []byte(msg.Text),
		})
		if msg.HTML != "" {
			parts = append(parts, mime.Part{
				ContentType: mime.ContentTypeFor("text/html"),
				Body:        []byte(msg.HTML),
			})
		}
		for _, a := range msg.Attachments {
			parts = append(parts, mime.Part{
				ContentType: "application/octet-stream",
				Disposition: mime.ContentDisposition(a.Filename),
				Body:        a.Data,
				Binary:      true,
			})
		}
		body.Write(mime.RenderBody(boundary, parts))
	}

	var out strings.Builder
	out.WriteString(header.Render(fields))
	out.WriteString("\r\n")
	out.WriteString(body.String())
	return []byte(out.String()), nil
}
