package imap

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
)

// bytesReader는 data를 감싼 *bytes.Reader를 반환한다.
// io.ReadAll 결과([]byte)를 mail.ReadMessage가 요구하는 io.Reader로 변환.
func bytesReader(data []byte) *bytes.Reader {
	return bytes.NewReader(data)
}

// extractBody는 mail.Message에서 plaintext 본문을 추출한다.
// multipart/alternative 또는 multipart/mixed인 경우 text/plain 파트를 선호하고,
// 없으면 text/html 파트(태그 제거)를 사용한다.
func extractBody(msg *mail.Message) (string, error) {
	ctype := msg.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(ctype)
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		body, rerr := io.ReadAll(msg.Body)
		if rerr != nil {
			return "", rerr
		}
		return string(body), nil
	}

	mpr := multipart.NewReader(msg.Body, params["boundary"])
	var html string
	for {
		part, perr := mpr.NextPart()
		if perr == io.EOF {
			break
		}
		if perr != nil {
			break
		}
		pctype := part.Header.Get("Content-Type")
		pmedia, _, _ := mime.ParseMediaType(pctype)
		data, _ := io.ReadAll(part)
		switch {
		case strings.HasPrefix(pmedia, "text/plain"):
			return string(data), nil
		case strings.HasPrefix(pmedia, "text/html") && html == "":
			html = string(data)
		}
	}
	if html != "" {
		return stripHTML(html), nil
	}
	return "", nil
}

// stripHTML는 HTML에서 태그를 제거하고 대략적인 텍스트를 반환한다.
// 단순 클라이언트 표시용이므로 정확한 변환은 목표로 하지 않는다.
func stripHTML(s string) string {
	var out strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			out.WriteRune(r)
		}
	}
	return strings.TrimSpace(out.String())
}