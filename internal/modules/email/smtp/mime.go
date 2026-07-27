package smtp

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"math/rand"
	"mime"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"time"

	"working/internal/modules/email/types"
)

// buildMIME는 Message를 RFC 2822 MIME 메시지로 인코딩한다.
// 첨부파일이 있으면 multipart/mixed, 없으면 text/plain 단일 파트.
func buildMIME(from string, msg *types.Message) ([]byte, error) {
	headers := textproto.MIMEHeader{}
	headers.Set("From", from)
	headers.Set("To", msg.To)
	if msg.Cc != "" {
		headers.Set("Cc", msg.Cc)
	}
	headers.Set("Subject", encodeHeader(msg.Subject))
	headers.Set("MIME-Version", "1.0")
	headers.Set("Date", time.Now().Format(time.RFC1123Z))

	var buf bytes.Buffer
	if len(msg.Attachments) == 0 {
		headers.Set("Content-Type", "text/plain; charset=UTF-8")
		headers.Set("Content-Transfer-Encoding", "base64")
		writeHeaders(&buf, headers)
		enc := base64.StdEncoding.EncodeToString([]byte(msg.Body))
		buf.WriteString(enc)
		buf.WriteString("\r\n")
		return buf.Bytes(), nil
	}

	boundary := fmt.Sprintf("WORKING-%d-%d", time.Now().UnixNano(), rand.Int63())
	headers.Set("Content-Type", "multipart/mixed; boundary=\""+boundary+"\"")
	writeHeaders(&buf, headers)

	// 본문 파트
	buf.WriteString("--" + boundary + "\r\n")
	buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	buf.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
	buf.WriteString(base64.StdEncoding.EncodeToString([]byte(msg.Body)))
	buf.WriteString("\r\n")

	// 첨부파일 파트
	for _, att := range msg.Attachments {
		data, err := os.ReadFile(att.Path)
		if err != nil {
			return nil, fmt.Errorf("첨부파일 읽기 실패(%s): %w", att.Path, err)
		}
		name := att.Name
		if name == "" {
			name = filepath.Base(att.Path)
		}
		buf.WriteString("--" + boundary + "\r\n")
		buf.WriteString("Content-Type: application/octet-stream\r\n")
		buf.WriteString("Content-Transfer-Encoding: base64\r\n")
		buf.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n\r\n", encodeHeader(name)))
		buf.WriteString(base64.StdEncoding.EncodeToString(data))
		buf.WriteString("\r\n")
	}
	buf.WriteString("--" + boundary + "--\r\n")
	return buf.Bytes(), nil
}

func writeHeaders(buf *bytes.Buffer, h textproto.MIMEHeader) {
	for k, v := range h {
		fmt.Fprintf(buf, "%s: %s\r\n", k, strings.Join(v, "; "))
	}
	buf.WriteString("\r\n")
}

// encodeHeader는 RFC 2047 B 인코딩으로 비ASCII 헤더를 인코딩한다.
// ASCII 전용이면 그대로 반환한다.
func encodeHeader(s string) string {
	if isASCII(s) {
		return s
	}
	return mime.BEncoding.Encode("UTF-8", s)
}

func isASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return true
}
