package imap

import (
	"crypto/tls"
	"fmt"
	"io"
	"mime"
	"net/mail"
	"sort"
	"strings"
	"time"

	"working/internal/modules/email/account"
	"working/internal/modules/email/types"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

// Encryption은 IMAP 서버 암호화 방식을 나타낸다.
type Encryption string

const (
	EncryptionNone     Encryption = "none"
	EncryptionStartTLS Encryption = "starttls"
	EncryptionTLS      Encryption = "tls"
)

// FetchLimit는 한 번에 가져오는 최대 메시지 수이다.
const FetchLimit = 50

// Receiver는 IMAP 서버에서 메일을 조회한다.
type Receiver struct{}

// New는 기본 Receiver를 반환한다.
func New() *Receiver { return &Receiver{} }

// Folders는 계정의 모든 폴더(사서함) 이름 목록을 반환한다.
func (r *Receiver) Folders(acc *account.Account, credential string) ([]string, error) {
	if acc.IMAP == nil {
		return nil, fmt.Errorf("계정에 IMAP 설정이 없습니다: %s", acc.ID)
	}
	c, err := r.connect(acc, credential)
	if err != nil {
		return nil, err
	}
	defer c.Logout()

	mailboxes := make(chan *imap.MailboxInfo, 100)
	err = c.List("", "*", mailboxes)
	if err != nil {
		return nil, fmt.Errorf("폴더 목록 조회 실패: %w", err)
	}
	var out []string
	for m := range mailboxes {
		out = append(out, m.Name)
	}
	sort.Strings(out)
	return out, nil
}

// List는 지정한 폴더의 최신 메시지를 FetchLimit개까지 조회한다.
// folder가 빈 문자열이면 "INBOX"를 사용한다.
// 반환되는 Message.Body는 plaintext 본문(HTML을 단순 변환한 결과)이다.
func (r *Receiver) List(acc *account.Account, credential, folder string) ([]types.Message, error) {
	if acc.IMAP == nil {
		return nil, fmt.Errorf("계정에 IMAP 설정이 없습니다: %s", acc.ID)
	}
	if folder == "" {
		folder = "INBOX"
	}
	c, err := r.connect(acc, credential)
	if err != nil {
		return nil, err
	}
	defer c.Logout()

	mbox, err := c.Select(folder, false)
	if err != nil {
		return nil, fmt.Errorf("폴더 선택 실패(%s): %w", folder, err)
	}
	if mbox.Messages == 0 {
		return []types.Message{}, nil
	}

	from := uint32(1)
	to := mbox.Messages
	if mbox.Messages > FetchLimit {
		from = mbox.Messages - FetchLimit + 1
	}
	seqset := new(imap.SeqSet)
	seqset.AddRange(from, to)

	// 헤더/봉투/플래그/본문 섹션을 요청한다.
	bodySection := &imap.BodySectionName{Peek: true}
	items := []imap.FetchItem{imap.FetchUid, imap.FetchEnvelope, imap.FetchFlags, imap.FetchInternalDate, bodySection.FetchItem()}

	messages := make(chan *imap.Message, FetchLimit)
	go func() {
		_ = c.Fetch(seqset, items, messages)
	}()

	var out []types.Message
	for m := range messages {
		out = append(out, toMessage(m, bodySection))
	}
	// 최신순(UID desc) 정렬
	sort.Slice(out, func(i, j int) bool { return out[i].UID > out[j].UID })
	return out, nil
}

// connect는 IMAP 서버에 연결 후 로그인한다.
func (r *Receiver) connect(acc *account.Account, credential string) (*client.Client, error) {
	addr := fmt.Sprintf("%s:%d", acc.IMAP.Host, acc.IMAP.Port)
	enc := Encryption(strings.ToLower(acc.IMAP.Encryption))

	var c *client.Client
	var err error
	switch enc {
	case EncryptionTLS:
		c, err = client.DialTLS(addr, &tls.Config{ServerName: acc.IMAP.Host})
	case EncryptionStartTLS:
		c, err = client.Dial(addr)
		if err == nil {
			if err = c.StartTLS(&tls.Config{ServerName: acc.IMAP.Host}); err != nil {
				c.Close()
			}
		}
	default:
		c, err = client.Dial(addr)
	}
	if err != nil {
		return nil, fmt.Errorf("IMAP 연결 실패: %w", err)
	}

	if err := c.Login(acc.Email, credential); err != nil {
		c.Close()
		return nil, fmt.Errorf("IMAP 로그인 실패: %w", err)
	}
	return c, nil
}

// toMessage는 imap.Message를 types.Message로 변환한다.
func toMessage(m *imap.Message, section *imap.BodySectionName) types.Message {
	msg := types.Message{
		UID:    m.Uid,
		Unread: hasFlag(m, imap.SeenFlag),
	}
	if m.Envelope != nil {
		msg.Subject = decodeRFC2047(m.Envelope.Subject)
		if m.Envelope.From != nil && len(m.Envelope.From) > 0 {
			msg.From = m.Envelope.From[0].Address()
		}
		if m.Envelope.To != nil {
			msg.To = joinAddresses(m.Envelope.To)
		}
		if m.Envelope.Cc != nil {
			msg.Cc = joinAddresses(m.Envelope.Cc)
		}
		if !m.Envelope.Date.IsZero() {
			msg.Date = m.Envelope.Date.Format(time.RFC3339)
		}
	}
	// BODY[] 추출
	var buf strings.Builder
	if literal := m.GetBody(section); literal != nil {
		_ = readBody(literal, &buf)
	}
	msg.Body = buf.String()
	return msg
}

func joinAddresses(addrs []*imap.Address) string {
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		if a != nil {
			out = append(out, a.Address())
		}
	}
	return strings.Join(out, ", ")
}

func hasFlag(m *imap.Message, f string) bool {
	for _, fl := range m.Flags {
		if fl == f {
			return true
		}
	}
	return false
}

// readBody는 IMAP 리터럴을 읽어 plaintext 본문을 추출한다.
// 메시지가 multipart인 경우 text/plain 또는 text/html 파트를 찾는다.
func readBody(r io.Reader, buf *strings.Builder) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	msg, err := mail.ReadMessage(bytesReader(data))
	if err != nil {
		// 파싱 실패 시 원문을 그대로 담는다.
		buf.Write(data)
		return nil
	}
	body, err := extractBody(msg)
	if err != nil {
		buf.Write(data)
		return nil
	}
	buf.WriteString(body)
	return nil
}

// decodeRFC2047는 RFC 2047 인코딩된 헤더 값을 디코딩한다.
// 디코딩 실패 시 원본을 반환한다.
func decodeRFC2047(s string) string {
	dec := new(mime.WordDecoder)
	out, err := dec.DecodeHeader(s)
	if err != nil {
		return s
	}
	return out
}