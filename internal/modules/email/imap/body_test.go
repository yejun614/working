package imap

import (
	"bytes"
	"encoding/base64"
	"net/mail"
	"strings"
	"testing"
)

func TestExtractBodyDecodesBase64(t *testing.T) {
	raw := "Content-Type: text/plain; charset=UTF-8\r\n" +
		"Content-Transfer-Encoding: base64\r\n\r\n" +
		"SGVsbG8sIGVtYWlsIQ==\r\n"

	got := extractTestBody(t, raw)
	if got != "Hello, email!" {
		t.Fatalf("본문이 디코딩되지 않았습니다: %q", got)
	}
}

func TestExtractBodyDecodesQuotedPrintable(t *testing.T) {
	raw := "Content-Type: text/plain; charset=UTF-8\r\n" +
		"Content-Transfer-Encoding: quoted-printable\r\n\r\n" +
		"Hello=2C=20email=21\r\n"

	got := extractTestBody(t, raw)
	if got != "Hello, email!\r\n" {
		t.Fatalf("본문이 디코딩되지 않았습니다: %q", got)
	}
}

func TestExtractBodyDecodesMultipartBase64PlainText(t *testing.T) {
	raw := "Content-Type: multipart/alternative; boundary=mail-boundary\r\n" +
		"MIME-Version: 1.0\r\n\r\n" +
		"--mail-boundary\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"Content-Transfer-Encoding: base64\r\n\r\n" +
		"SGVsbG8sIG11bHRpcGFydCE=\r\n" +
		"--mail-boundary--\r\n"

	got := extractTestBody(t, raw)
	if got != "Hello, multipart!" {
		t.Fatalf("multipart 본문이 디코딩되지 않았습니다: %q", got)
	}
}

func TestExtractBodyPreservesHTMLPart(t *testing.T) {
	raw := "Content-Type: multipart/alternative; boundary=html-boundary\r\n" +
		"MIME-Version: 1.0\r\n\r\n" +
		"--html-boundary\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n\r\n" +
		"Plain body\r\n" +
		"--html-boundary\r\n" +
		"Content-Type: text/html; charset=UTF-8\r\n\r\n" +
		"<p><strong>HTML body</strong></p>\r\n" +
		"--html-boundary--\r\n"

	msg, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("테스트 메일 파싱 실패: %v", err)
	}
	content, err := extractBodyContent(msg)
	if err != nil {
		t.Fatalf("본문 추출 실패: %v", err)
	}
	if content.Plain != "Plain body" {
		t.Fatalf("plaintext 본문이 잘못되었습니다: %q", content.Plain)
	}
	if content.HTML != "<p><strong>HTML body</strong></p>" {
		t.Fatalf("HTML 본문이 보존되지 않았습니다: %q", content.HTML)
	}
}

func TestExtractBodyRecursesIntoMultipartRelated(t *testing.T) {
	html := base64.StdEncoding.EncodeToString([]byte("<html><body><h1>네이버 웹툰 개인정보 안내</h1></body></html>"))
	raw := "Content-Type: multipart/alternative; boundary=outer-boundary\r\n" +
		"MIME-Version: 1.0\r\n\r\n" +
		"--outer-boundary\r\n" +
		"Content-Type: multipart/related; boundary=inner-boundary\r\n\r\n" +
		"--inner-boundary\r\n" +
		"Content-Type: text/html; charset=UTF-8\r\n" +
		"Content-Transfer-Encoding: base64\r\n\r\n" +
		html + "\r\n" +
		"--inner-boundary--\r\n" +
		"--outer-boundary--\r\n"

	msg, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("중첩 multipart 테스트 메일 파싱 실패: %v", err)
	}
	content, err := extractBodyContent(msg)
	if err != nil {
		t.Fatalf("중첩 multipart 본문 추출 실패: %v", err)
	}
	if content.HTML != "<html><body><h1>네이버 웹툰 개인정보 안내</h1></body></html>" {
		t.Fatalf("중첩 multipart HTML 본문이 추출되지 않았습니다: %q", content.HTML)
	}
}

func TestParseRawMessageListsDecodedAttachments(t *testing.T) {
	raw := "From: sender@example.com\r\n" +
		"Content-Type: multipart/mixed; boundary=mixed-boundary\r\n" +
		"MIME-Version: 1.0\r\n\r\n" +
		"--mixed-boundary\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n\r\n" +
		"본문\r\n" +
		"--mixed-boundary\r\n" +
		"Content-Type: application/pdf\r\n" +
		"Content-Transfer-Encoding: base64\r\n" +
		"Content-Disposition: attachment; filename=\"=?UTF-8?B?7LKo67aALnBkZg==?=\"\r\n\r\n" +
		"AQIDBAU=\r\n" +
		"--mixed-boundary--\r\n"

	message, err := ParseRawMessage([]byte(raw))
	if err != nil {
		t.Fatalf("원문 메일 파싱 실패: %v", err)
	}
	if len(message.Attachments) != 1 {
		t.Fatalf("첨부파일 개수가 잘못되었습니다: %d", len(message.Attachments))
	}
	attachment := message.Attachments[0]
	if attachment.Name != "첨부.pdf" {
		t.Fatalf("첨부파일명이 디코딩되지 않았습니다: %q", attachment.Name)
	}
	if attachment.Size != 5 {
		t.Fatalf("첨부파일 용량이 잘못되었습니다: %d", attachment.Size)
	}
	decoded, err := ExtractRawAttachment([]byte(raw), 0)
	if err != nil {
		t.Fatalf("첨부파일 데이터 추출 실패: %v", err)
	}
	if !bytes.Equal(decoded, []byte{1, 2, 3, 4, 5}) {
		t.Fatalf("첨부파일 데이터가 디코딩되지 않았습니다: %v", decoded)
	}
}

func TestParseRawMessageListsFoldedRFC2047Attachment(t *testing.T) {
	raw := "From: sender@example.com\r\n" +
		"Content-Type: multipart/mixed; boundary=outer\r\n" +
		"MIME-Version: 1.0\r\n\r\n" +
		"--outer\r\n" +
		"Content-Type: application/octet-stream\r\n" +
		"Content-Disposition: attachment;\r\n" +
		"\tfilename=\"=?UTF-8?B?7LKo67aALnBkZg==?=\r\n" +
		" =?UTF-8?B?LnBkZg==?=\"\r\n" +
		"Content-Transfer-Encoding: base64\r\n\r\n" +
		"AQIDBAU=\r\n" +
		"--outer--\r\n"

	message, err := ParseRawMessage([]byte(raw))
	if err != nil {
		t.Fatalf("원문 메일 파싱 실패: %v", err)
	}
	if len(message.Attachments) != 1 {
		t.Fatalf("첨부파일 개수 불일치: got %d, want 1", len(message.Attachments))
	}
	if message.Attachments[0].Name != "첨부.pdf.pdf" {
		t.Fatalf("파일명 디코딩 불일치: got %q", message.Attachments[0].Name)
	}
	if message.Attachments[0].Size != 5 {
		t.Fatalf("디코딩된 첨부파일 용량 불일치: got %d, want 5", message.Attachments[0].Size)
	}
}

func TestParseRawMessageListsAttachmentWhenContentTypeParametersAreMalformed(t *testing.T) {
	raw := "From: sender@example.com\r\n" +
		"Content-Type: multipart/mixed; boundary=outer\r\n" +
		"MIME-Version: 1.0\r\n\r\n" +
		"--outer\r\n" +
		"Content-Type: application/octet-stream; broken=\"unterminated\r\n" +
		"Content-Disposition: attachment; filename=\"report.pdf\"\r\n\r\n" +
		"12345\r\n" +
		"--outer--\r\n"

	message, err := ParseRawMessage([]byte(raw))
	if err != nil {
		t.Fatalf("원문 메일 파싱 실패: %v", err)
	}
	if len(message.Attachments) != 1 {
		t.Fatalf("첨부파일 개수 불일치: got %d, want 1", len(message.Attachments))
	}
	if message.Attachments[0].Name != "report.pdf" || message.Attachments[0].Size != 5 {
		t.Fatalf("첨부파일 정보 불일치: %#v", message.Attachments[0])
	}
}

func extractTestBody(t *testing.T, raw string) string {
	t.Helper()
	msg, err := mail.ReadMessage(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("테스트 메일 파싱 실패: %v", err)
	}
	got, err := extractBody(msg)
	if err != nil {
		t.Fatalf("본문 추출 실패: %v", err)
	}
	return got
}
