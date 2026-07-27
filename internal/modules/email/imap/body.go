package imap

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"time"

	"working/internal/modules/email/types"
)

// ParseRawMessage는 원시 MIME 메시지를 공통 이메일 모델로 변환한다.
// Gmail API와 IMAP이 같은 본문 파서와 헤더 디코딩 규칙을 사용하도록 공개한다.
func ParseRawMessage(data []byte) (types.Message, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(data))
	if err != nil {
		return types.Message{}, err
	}
	content, err := extractBodyContent(msg)
	if err != nil {
		return types.Message{}, err
	}
	date := ""
	if value, err := msg.Header.Date(); err == nil {
		date = value.Format(time.RFC3339)
	}
	return types.Message{
		From:        msg.Header.Get("From"),
		To:          msg.Header.Get("To"),
		Cc:          msg.Header.Get("Cc"),
		Subject:     decodeRFC2047(msg.Header.Get("Subject")),
		Body:        content.Plain,
		HTML:        content.HTML,
		Attachments: content.Attachments,
		Raw:         string(data),
		Date:        date,
	}, nil
}

// bytesReader는 data를 감싼 *bytes.Reader를 반환한다.
// io.ReadAll 결과([]byte)를 mail.ReadMessage가 요구하는 io.Reader로 변환.
func bytesReader(data []byte) *bytes.Reader {
	return bytes.NewReader(data)
}

type bodyContent struct {
	Plain          string
	HTML           string
	Attachments    []types.Attachment
	attachmentData []attachmentContent
}

type attachmentContent struct {
	metadata types.Attachment
	data     []byte
}

// ExtractRawAttachment는 원문 MIME 메시지에서 지정한 첨부파일의 디코딩된
// 바이트를 반환한다. 프론트엔드가 원문을 이미 보유하고 있으므로, 다운로드나
// 미리보기 때 서버에 다시 메일을 조회하지 않고 동일한 원문을 재사용한다.
func ExtractRawAttachment(data []byte, index int) ([]byte, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	content, err := extractBodyContent(msg)
	if err != nil {
		return nil, err
	}
	if index < 0 || index >= len(content.attachmentData) {
		return nil, fmt.Errorf("첨부파일 인덱스가 범위를 벗어났습니다: %d", index)
	}
	return append([]byte(nil), content.attachmentData[index].data...), nil
}

// extractBody는 하위 호환을 위해 plaintext 본문만 반환한다.
func extractBody(msg *mail.Message) (string, error) {
	content, err := extractBodyContent(msg)
	return content.Plain, err
}

// extractBodyContent는 MIME 메시지에서 plaintext와 HTML 본문을 각각 추출한다.
// multipart/alternative에서는 두 본문을 모두 보존하고, 표시 우선순위는 호출자가 결정한다.
func extractBodyContent(msg *mail.Message) (bodyContent, error) {
	ctype := msg.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(ctype)
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		body, rerr := io.ReadAll(msg.Body)
		if rerr != nil {
			return bodyContent{}, rerr
		}
		body, derr := decodeTransferEncoding(body, msg.Header.Get("Content-Transfer-Encoding"))
		if derr != nil {
			return bodyContent{}, derr
		}
		if strings.HasPrefix(mediaType, "text/html") {
			return bodyContent{Plain: stripHTML(string(body)), HTML: string(body)}, nil
		}
		return bodyContent{Plain: string(body)}, nil
	}
	return extractMIMEPart(msg.Body, mediaType, params, msg.Header.Get("Content-Transfer-Encoding"))
}

// extractMIMEPart는 MIME 파트를 재귀적으로 순회하며 본문을 추출한다.
// multipart/alternative 안에 multipart/related가 중첩되는 메일처럼
// 여러 단계로 감싸진 본문도 최종 text/plain 또는 text/html까지 내려간다.
func extractMIMEPart(r io.Reader, mediaType string, params map[string]string, encoding string) (bodyContent, error) {
	if strings.HasPrefix(mediaType, "multipart/") {
		return extractMultipartMIMEPart(r, params["boundary"])
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return bodyContent{}, err
	}
	data, err = decodeTransferEncoding(data, encoding)
	if err != nil {
		return bodyContent{}, err
	}
	if strings.HasPrefix(mediaType, "text/html") {
		return bodyContent{Plain: stripHTML(string(data)), HTML: string(data)}, nil
	}
	if strings.HasPrefix(mediaType, "text/plain") {
		return bodyContent{Plain: string(data)}, nil
	}
	return bodyContent{}, nil
}

func extractMultipartMIMEPart(r io.Reader, boundary string) (bodyContent, error) {
	mpr := multipart.NewReader(r, boundary)
	var plain string
	var html string
	var attachments []types.Attachment
	var attachmentData []attachmentContent
	for {
		part, perr := mpr.NextPart()
		if perr == io.EOF {
			break
		}
		if perr != nil {
			return bodyContent{}, perr
		}
		pctype := part.Header.Get("Content-Type")
		pmedia, pparams, perr := mime.ParseMediaType(pctype)
		if perr != nil {
			// 일부 메일은 Content-Type 파라미터가 비표준이어도
			// Content-Disposition의 첨부파일 정보는 정상적으로 제공한다.
			// 첨부파일 판정 기회를 잃지 않도록 미디어 타입만 보수적으로 복구한다.
			pmedia = strings.ToLower(strings.TrimSpace(strings.SplitN(pctype, ";", 2)[0]))
			pparams = map[string]string{}
		}
		encoding := part.Header.Get("Content-Transfer-Encoding")
		if name := attachmentName(part.Header, pparams); name != "" {
			data, derr := readMIMEPartData(part, encoding)
			if derr != nil {
				return bodyContent{}, derr
			}
			metadata := types.Attachment{Name: name, Size: len(data)}
			attachments = append(attachments, metadata)
			attachmentData = append(attachmentData, attachmentContent{metadata: metadata, data: data})
			continue
		}
		content, cerr := extractMIMEPart(part, pmedia, pparams, encoding)
		if cerr != nil {
			return bodyContent{}, cerr
		}
		if plain == "" && content.Plain != "" {
			plain = content.Plain
		}
		if html == "" && content.HTML != "" {
			html = content.HTML
		}
		attachments = append(attachments, content.Attachments...)
		attachmentData = append(attachmentData, content.attachmentData...)
	}
	return bodyContent{Plain: plain, HTML: html, Attachments: attachments, attachmentData: attachmentData}, nil
}

func attachmentName(header interface{ Get(string) string }, contentParams map[string]string) string {
	disposition, params, err := mime.ParseMediaType(header.Get("Content-Disposition"))
	if err == nil && strings.EqualFold(disposition, "attachment") {
		if name := decodeRFC2047(params["filename"]); name != "" {
			return name
		}
	}
	if name := decodeRFC2047(contentParams["name"]); name != "" {
		return name
	}
	return ""
}

func readMIMEPartData(r io.Reader, encoding string) ([]byte, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	// multipart.Reader.NextPart는 quoted-printable 파트를 읽을 때
	// 전송 인코딩을 이미 제거하므로 다시 디코딩하지 않는다.
	if strings.EqualFold(strings.TrimSpace(encoding), "quoted-printable") {
		return data, nil
	}
	return decodeTransferEncoding(data, encoding)
}

// decodeTransferEncoding은 MIME 본문의 전송 인코딩을 원래 바이트로 복원한다.
// multipart.Reader.NextPart는 quoted-printable을 읽는 과정에서 이미 디코딩하므로,
// 해당 헤더가 숨겨진 경우에는 데이터를 다시 디코딩하지 않는다.
func decodeTransferEncoding(data []byte, encoding string) ([]byte, error) {
	switch strings.ToLower(strings.TrimSpace(encoding)) {
	case "", "7bit", "8bit", "binary":
		return data, nil
	case "base64":
		return base64.StdEncoding.DecodeString(string(data))
	case "quoted-printable":
		return io.ReadAll(quotedprintable.NewReader(bytes.NewReader(data)))
	default:
		return data, nil
	}
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
