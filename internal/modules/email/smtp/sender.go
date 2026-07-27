package smtp

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"

	"working/internal/modules/email/account"
	"working/internal/modules/email/types"
)

// Encryption은 SMTP 서버 암호화 방식을 나타낸다.
type Encryption string

const (
	// EncryptionNone는 평문 연결(권장하지 않음).
	EncryptionNone Encryption = "none"
	// EncryptionStartTLS는 STARTTLS 업그레이드.
	EncryptionStartTLS Encryption = "starttls"
	// EncryptionTLS는 암시적 TLS(포트 465에서 주로 사용).
	EncryptionTLS Encryption = "tls"
)

// Sender는 SMTP 서버를 통해 메일을 발송한다.
type Sender struct{}

// New는 기본 Sender를 반환한다.
func New() *Sender { return &Sender{} }

// Send는 계정의 SMTP 설정과 자격증명을 사용해 메일을 발송한다.
// msg.Attachments의 Path가 가리키는 로컬 파일을 MIME 첨부파일로 인코딩한다.
func (s *Sender) Send(acc *account.Account, credential string, msg *types.Message) error {
	if acc.SMTP == nil {
		return fmt.Errorf("계정에 SMTP 설정이 없습니다: %s", acc.ID)
	}
	if credential == "" {
		return fmt.Errorf("자격증명이 비어 있습니다: %s", acc.ID)
	}
	if msg == nil {
		return fmt.Errorf("메시지가 nil입니다")
	}

	addr := fmt.Sprintf("%s:%d", acc.SMTP.Host, acc.SMTP.Port)
	auth := smtp.PlainAuth("", acc.Email, credential, acc.SMTP.Host)

	from := acc.Email
	if acc.DisplayName != "" {
		from = fmt.Sprintf("%s <%s>", encodeHeader(acc.DisplayName), acc.Email)
	}

	raw, err := BuildMIME(from, msg)
	if err != nil {
		return err
	}

	enc := Encryption(strings.ToLower(acc.SMTP.Encryption))
	recipients := recipientsOf(msg)
	switch enc {
	case EncryptionTLS:
		return sendImplicitTLS(addr, auth, acc.Email, recipients, raw)
	case EncryptionStartTLS:
		return sendStartTLS(addr, auth, acc.Email, recipients, raw, acc.SMTP.Host)
	default:
		return sendPlain(addr, auth, acc.Email, recipients, raw)
	}
}

// BuildMIME는 메시지를 RFC 2045 MIME 바이트로 변환한다.
// SMTP와 Gmail API 발송이 동일한 본문/첨부파일 포맷을 사용하도록 공개한다.
func BuildMIME(from string, msg *types.Message) ([]byte, error) {
	return buildMIME(from, msg)
}

// recipientsOf는 To + Cc 주소를 콤마로 분리해 반환한다.
func recipientsOf(m *types.Message) []string {
	out := splitAddresses(m.To)
	out = append(out, splitAddresses(m.Cc)...)
	return out
}

func splitAddresses(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	res := make([]string, 0, len(parts))
	for _, p := range parts {
		if a := strings.TrimSpace(p); a != "" {
			res = append(res, a)
		}
	}
	return res
}

func sendPlain(addr string, auth smtp.Auth, from string, to []string, raw []byte) error {
	return smtp.SendMail(addr, auth, from, to, raw)
}

func sendStartTLS(addr string, auth smtp.Auth, from string, to []string, raw []byte, host string) error {
	conn, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("SMTP 연결 실패: %w", err)
	}
	defer conn.Close()
	if err := conn.StartTLS(&tls.Config{ServerName: host}); err != nil {
		return fmt.Errorf("STARTTLS 실패: %w", err)
	}
	if err := conn.Auth(auth); err != nil {
		return fmt.Errorf("SMTP 인증 실패: %w", err)
	}
	if err := conn.Mail(from); err != nil {
		return err
	}
	for _, rcpt := range to {
		if err := conn.Rcpt(rcpt); err != nil {
			return err
		}
	}
	w, err := conn.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(raw); err != nil {
		return err
	}
	return w.Close()
}

func sendImplicitTLS(addr string, auth smtp.Auth, from string, to []string, raw []byte) error {
	tlsConf := &tls.Config{
		ServerName: strings.Split(addr, ":")[0],
	}
	conn, err := tls.Dial("tcp", addr, tlsConf)
	if err != nil {
		return fmt.Errorf("TLS 연결 실패: %w", err)
	}
	defer conn.Close()

	c, err := smtp.NewClient(conn, strings.Split(addr, ":")[0])
	if err != nil {
		return fmt.Errorf("SMTP 클라이언트 생성 실패: %w", err)
	}
	defer c.Close()
	if err := c.Auth(auth); err != nil {
		return fmt.Errorf("SMTP 인증 실패: %w", err)
	}
	if err := c.Mail(from); err != nil {
		return err
	}
	for _, rcpt := range to {
		if err := c.Rcpt(rcpt); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(raw); err != nil {
		return err
	}
	return w.Close()
}
