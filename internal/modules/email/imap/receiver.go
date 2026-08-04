package imap

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"mime"
	"net/mail"
	"sort"
	"strconv"
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
	page, err := r.ListPage(acc, credential, folder, "")
	return page.Messages, err
}

// ListPage는 지정한 폴더의 메시지를 한 페이지 조회한다.
// pageToken은 다음 페이지를 조회할 때 사용할 IMAP sequence number 상한이다.
func (r *Receiver) ListPage(acc *account.Account, credential, folder, pageToken string) (types.MessagePage, error) {
	if acc.IMAP == nil {
		return types.MessagePage{}, fmt.Errorf("계정에 IMAP 설정이 없습니다: %s", acc.ID)
	}
	if folder == "" {
		folder = "INBOX"
	}
	c, err := r.connect(acc, credential)
	if err != nil {
		return types.MessagePage{}, err
	}
	defer c.Logout()

	mbox, err := c.Select(folder, false)
	if err != nil {
		return types.MessagePage{}, fmt.Errorf("폴더 선택 실패(%s): %w", folder, err)
	}
	if mbox.Messages == 0 {
		return types.MessagePage{Messages: []types.Message{}}, nil
	}

	to := mbox.Messages
	if pageToken != "" {
		parsed, parseErr := strconv.ParseUint(pageToken, 10, 32)
		if parseErr != nil {
			return types.MessagePage{}, fmt.Errorf("잘못된 이메일 페이지 커서: %w", parseErr)
		}
		if parsed == 0 {
			return types.MessagePage{Messages: []types.Message{}}, nil
		}
		if uint32(parsed) < to {
			to = uint32(parsed)
		}
	}
	from := uint32(1)
	if to > FetchLimit {
		from = to - FetchLimit + 1
	}
	seqset := new(imap.SeqSet)
	seqset.AddRange(from, to)

	bodySection := &imap.BodySectionName{Peek: true}
	items := []imap.FetchItem{imap.FetchUid, imap.FetchEnvelope, imap.FetchFlags, imap.FetchInternalDate, bodySection.FetchItem()}
	itemsCh := make(chan *imap.Message, FetchLimit)
	go func() { _ = c.Fetch(seqset, items, itemsCh) }()

	var out []types.Message
	for m := range itemsCh {
		out = append(out, toMessage(m, bodySection))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UID > out[j].UID })
	next := ""
	if from > 1 {
		next = strconv.FormatUint(uint64(from-1), 10)
	}
	return types.MessagePage{Messages: out, NextPageToken: next}, nil
}

// SetSeen은 지정한 메시지의 \Seen 플래그를 설정하거나 해제한다.
// seen이 true면 읽음, false면 읽지 않음으로 서버 상태를 바꾼다.
func (r *Receiver) SetSeen(acc *account.Account, credential, folder string, uid uint32, seen bool) error {
	c, err := r.openFolder(acc, credential, folder, uid)
	if err != nil {
		return err
	}
	defer c.Logout()

	var op imap.FlagsOp = imap.AddFlags
	if !seen {
		op = imap.RemoveFlags
	}
	if err := storeFlag(c, uid, op, imap.SeenFlag); err != nil {
		return fmt.Errorf("읽음 상태 변경 실패: %w", err)
	}
	return nil
}

// Delete는 메시지에 \Deleted 플래그를 설정한 뒤 EXPUNGE로 폴더에서 제거한다.
// EXPUNGE는 IMAP 표준상 해당 폴더에서 \Deleted가 붙은 메시지를 모두 정리하므로,
// 다른 클라이언트가 이미 삭제 표시해 둔 메시지도 함께 사라질 수 있다.
func (r *Receiver) Delete(acc *account.Account, credential, folder string, uid uint32) error {
	c, err := r.openFolder(acc, credential, folder, uid)
	if err != nil {
		return err
	}
	defer c.Logout()

	if err := storeFlag(c, uid, imap.AddFlags, imap.DeletedFlag); err != nil {
		return fmt.Errorf("메일 삭제 표시 실패: %w", err)
	}
	if err := c.Expunge(nil); err != nil {
		return fmt.Errorf("메일 삭제 반영 실패: %w", err)
	}
	return nil
}

// openFolder는 IMAP 서버에 연결하고 폴더를 쓰기 가능 모드로 선택한다.
// 플래그 변경은 읽기 전용 선택에서는 거부되므로 Select의 readOnly를 false로 둔다.
func (r *Receiver) openFolder(acc *account.Account, credential, folder string, uid uint32) (*client.Client, error) {
	if acc.IMAP == nil {
		return nil, fmt.Errorf("계정에 IMAP 설정이 없습니다: %s", acc.ID)
	}
	if uid == 0 {
		return nil, fmt.Errorf("메시지 UID가 필요합니다")
	}
	if folder == "" {
		folder = "INBOX"
	}
	c, err := r.connect(acc, credential)
	if err != nil {
		return nil, err
	}
	if _, err := c.Select(folder, false); err != nil {
		_ = c.Logout()
		return nil, fmt.Errorf("폴더 선택 실패(%s): %w", folder, err)
	}
	return c, nil
}

// storeFlag는 UID로 지정한 단일 메시지의 플래그를 추가하거나 제거한다.
func storeFlag(c *client.Client, uid uint32, op imap.FlagsOp, flag string) error {
	seqset := new(imap.SeqSet)
	seqset.AddNum(uid)
	return c.UidStore(seqset, imap.FormatFlagsOp(op, true), []any{flag}, nil)
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
		UID: m.Uid,
		// IMAP의 \Seen 플래그는 "읽음"을 뜻하므로 읽지 않음 여부는 그 부정이다.
		Unread: !hasFlag(m, imap.SeenFlag),
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
	var html string
	var raw string
	var attachments []types.Attachment
	if literal := m.GetBody(section); literal != nil {
		data, err := io.ReadAll(literal)
		if err == nil {
			raw = string(data)
			content, bodyErr := readBodyContent(bytes.NewReader(data))
			if bodyErr == nil {
				buf.WriteString(content.Plain)
				html = content.HTML
				attachments = content.Attachments
			}
		}
	}
	msg.Body = buf.String()
	msg.HTML = html
	msg.Attachments = attachments
	msg.Raw = raw
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

// readBodyContent는 IMAP 리터럴을 읽어 plaintext와 HTML 본문을 추출한다.
func readBodyContent(r io.Reader) (bodyContent, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return bodyContent{}, err
	}
	msg, err := mail.ReadMessage(bytesReader(data))
	if err != nil {
		// 파싱 실패 시 원문을 plaintext로 담는다.
		return bodyContent{Plain: string(data)}, nil
	}
	return extractBodyContent(msg)
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
