package sendEmail

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/http"
	"net/smtp"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
)

var coder = base64.StdEncoding

type Message struct {
	from        string
	to          string
	Subject     string
	plainText   []byte
	html        []byte
	attachments []string
}

func NewMessage() *Message {
	m := &Message{
		attachments: []string{},
	}

	return m
}

func (m *Message) SetFrom(from string) *Message {
	m.from = from
	return m
}

func (m *Message) SetTo(to string) *Message {
	m.to = to
	return m
}

func (m *Message) SetSubject(subject string) *Message {
	m.Subject = fmt.Sprintf("=?UTF-8?B?%s?=", coder.EncodeToString([]byte(subject)))
	return m
}

func (m *Message) SetPlainText(text string) *Message {
	m.plainText = []byte(text)
	return m
}

func (m *Message) SetHtml(text string) *Message {
	m.html = []byte(text)
	return m
}

func (m *Message) AddAttachment(path string) *Message {
	m.attachments = append(m.attachments, path)
	return m
}

func (m Message) Send(smtpClient *smtp.Client) error {
	if strings.TrimSpace(m.to) == "" {
		return fmt.Errorf(`"To" field is empty`)
	}

	if strings.TrimSpace(m.from) == "" {
		return fmt.Errorf(`"From" field is empty`)
	}

	if strings.TrimSpace(m.Subject) == "" {
		return fmt.Errorf(`"Subject" field is empty`)
	}

	if m.plainText != nil && m.html != nil {
		return fmt.Errorf("message can't have both plain text and html text")
	}

	if err := smtpClient.Mail(m.from); err != nil {
		return err
	}

	for _, dest := range strings.Split(m.to, ",") {
		if err := smtpClient.Rcpt(dest); err != nil {
			return err
		}
	}

	wc, err := smtpClient.Data()
	if err != nil {
		return err
	}
	defer func() {
		if err = wc.Close(); err != nil {
			fmt.Println(err)
		}
	}()

	_ = m.Write(wc)

	return nil
}

func (m Message) Write(wc io.WriteCloser) error {
	_, _ = fmt.Fprint(wc, "MIME-Version: 1.0\r\n")
	_, _ = fmt.Fprintf(wc, "From: %s\r\n", m.from)
	_, _ = fmt.Fprintf(wc, "To: %s\r\n", m.to)
	_, _ = fmt.Fprintf(wc, "Subject: %s\r\n", m.Subject)

	pw := multipart.NewWriter(wc)
	_, _ = fmt.Fprintf(wc, "Content-Type: multipart/mixed; boundary=%s\r\n", pw.Boundary())
	_, _ = wc.Write([]byte("\r\n"))

	buf := new(bytes.Buffer)
	bw := multipart.NewWriter(buf)

	if len(m.plainText) > 0 {
		header := textproto.MIMEHeader{
			"Content-Type":              []string{"text/plain; charset=utf-8"},
			"Content-Transfer-Encoding": []string{"quoted-printable"},
		}
		plainWriter, _ := bw.CreatePart(header)
		quotedWriter := quotedprintable.NewWriter(plainWriter)
		_, _ = quotedWriter.Write(m.plainText)
		_ = quotedWriter.Close()
	}

	if len(m.html) > 0 {
		header := textproto.MIMEHeader{
			"Content-Type":              []string{"text/html; charset=utf-8"},
			"Content-Transfer-Encoding": []string{"quoted-printable"},
		}
		htmlWriter, _ := bw.CreatePart(header)
		quotedWriter := quotedprintable.NewWriter(htmlWriter)
		_, _ = quotedWriter.Write(m.html)
		_ = quotedWriter.Close()
	}

	_ = bw.Close()

	header := textproto.MIMEHeader{"Content-Type": []string{"multipart/alternative; boundary=" + bw.Boundary()}}
	textWriter, _ := pw.CreatePart(header)
	_, _ = textWriter.Write(buf.Bytes())

	if err := m.writeAttachments(pw); err != nil {
		return err
	}

	return pw.Close()
}

func (m Message) writeAttachments(w *multipart.Writer) error {
	for _, attachment := range m.attachments {
		bs, err := os.ReadFile(filepath.Clean(attachment))
		if err != nil {
			return err
		}
		contentType := http.DetectContentType(bs)
		if contentType == "application/octet-stream" {
			if ext := mime.TypeByExtension("." + filepath.Ext(attachment)); ext != "" {
				contentType = ext
			}
		}
		header := textproto.MIMEHeader{
			"Content-Type":              []string{contentType},
			"Content-Transfer-Encoding": []string{"base64"},
			"Content-Disposition":       []string{fmt.Sprintf("attachment; filename=%q", filepath.Base(attachment))},
		}
		pw, _ := w.CreatePart(header)
		encoded := make([]byte, coder.EncodedLen(len(bs)))
		coder.Encode(encoded, bs)

		for i := 0; i*76 <= len(encoded); i++ {
			if (i+1)*76 >= len(encoded) {
				_, _ = pw.Write(encoded[i*76:])
				break
			}
			_, _ = pw.Write(encoded[i*76 : (i+1)*76])
			_, _ = pw.Write([]byte("\r\n"))
		}
	}

	return nil
}
