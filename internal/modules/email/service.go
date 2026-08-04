// Package email 은 "working" 앱의 이메일 모듈 진입점이다.
//
// 이 모듈은 SMTP 발송과 IMAP/Gmail 수신 기능을 Wails Service 형태로
// 프론트엔드에 노출한다. 계정과 자격증명은 통합 계정 모듈
// (internal/modules/account)이 관리하며, 이 모듈은 메일 기능이 켜진
// 계정만 읽어 사용하고 메일 캐시만 직접 보관한다.
// 다른 모듈과 마찬가지로 main.go에서 Service를 등록하면 앱에 포함된다.
package email

import (
	"encoding/base64"
	"fmt"
	"strings"

	accountstore "working/internal/modules/account/store"
	account "working/internal/modules/account/types"
	"working/internal/modules/email/gmail"
	"working/internal/modules/email/imap"
	"working/internal/modules/email/smtp"
	"working/internal/modules/email/store"
	"working/internal/modules/email/types"
)

// Service는 프론트엔드에 바인딩되는 이메일 모듈 서비스이다.
// 모든 메서드는 Wails를 통해 JS/TS에서 호출 가능하다.
type Service struct {
	store    *store.Store
	accounts *accountstore.Store
	sender   *smtp.Sender
	receiver *imap.Receiver
}

// NewService는 이메일 모듈 Service를 생성한다.
// 내부 저장소 초기화 실패 시 에러를 반환한다.
func NewService() (*Service, error) {
	st, err := store.New()
	if err != nil {
		return nil, err
	}
	accounts, err := accountstore.New()
	if err != nil {
		return nil, err
	}
	return &Service{
		store:    st,
		accounts: accounts,
		sender:   smtp.New(),
		receiver: imap.New(),
	}, nil
}

// ServiceShutdown은 Wails 종료 시 호출되는 훅이다.
// 현재는 유지할 상태가 없으므로 아무 작업도 하지 않는다.
func (s *Service) ServiceShutdown() {}

// AccountList는 메일 기능이 켜진 계정만 반환한다(자격증명 제외).
// 계정 등록·수정·삭제는 통합 계정 모듈이 담당한다.
func (s *Service) AccountList() ([]account.Account, error) {
	all, err := s.accounts.List()
	if err != nil {
		return nil, err
	}
	out := make([]account.Account, 0, len(all))
	for _, acc := range all {
		if acc.UsesMail() {
			out = append(out, acc)
		}
	}
	return out, nil
}

// AccountGet은 ID로 계정을 조회한다.
func (s *Service) AccountGet(id string) (*account.Account, error) {
	return s.mailAccount(id)
}

// mailAccount는 메일 기능이 켜진 계정을 조회한다.
func (s *Service) mailAccount(id string) (*account.Account, error) {
	acc, err := s.accounts.Get(id)
	if err != nil {
		return nil, err
	}
	if !acc.UsesMail() {
		return nil, fmt.Errorf("메일을 사용하지 않는 계정입니다: %s", acc.Name)
	}
	return acc, nil
}

// credentials는 계정과 키체인 자격증명을 함께 조회한다.
func (s *Service) credentials(accID string) (*account.Account, string, error) {
	acc, err := s.mailAccount(accID)
	if err != nil {
		return nil, "", err
	}
	cred, err := s.accounts.Credential(accID)
	if err != nil {
		return nil, "", err
	}
	return acc, cred, nil
}

// newGmailClient는 OAuth 계정용 Gmail 클라이언트를 만든다.
// 갱신된 access token은 통합 계정의 키체인 자격증명에 다시 저장한다.
func (s *Service) newGmailClient(acc *account.Account, cred string) (*gmail.Client, error) {
	return gmail.New(cred, func(updated string) error {
		return s.accounts.SaveCredential(acc.ID, updated)
	})
}

// Send는 지정한 계정으로 메일을 발송한다.
// accID로 계정을 조회하고, 키체인에서 자격증명을 꺼내 SMTP 전송에 사용한다.
func (s *Service) Send(accID string, msg *types.Message) error {
	acc, cred, err := s.credentials(accID)
	if err != nil {
		return err
	}
	if acc.AuthType == account.AuthOAuth2 {
		client, err := s.newGmailClient(acc, cred)
		if err != nil {
			return err
		}
		return client.Send(acc, msg)
	}
	return s.sender.Send(acc, cred, msg)
}

// Folders는 지정한 계정의 IMAP 폴더 목록을 반환한다.
func (s *Service) Folders(accID string) ([]string, error) {
	acc, cred, err := s.credentials(accID)
	if err != nil {
		return nil, err
	}
	if acc.AuthType == account.AuthOAuth2 {
		client, err := s.newGmailClient(acc, cred)
		if err != nil {
			return nil, err
		}
		return client.Labels()
	}
	return s.receiver.Folders(acc, cred)
}

// List는 네트워크를 호출하지 않고 SQLite 캐시에서 목록과 본문을 읽는다.
func (s *Service) List(accID, folder string) ([]types.Message, error) {
	list, _, err := s.store.Cached(accID, folder)
	return list, err
}

// Page는 네트워크를 호출하지 않고 캐시된 목록과 다음 페이지 커서를 반환한다.
func (s *Service) Page(accID, folder string) (types.MessagePage, error) {
	page, _, err := s.store.CachedPage(accID, folder)
	return page, err
}

// ListRefresh는 사용자가 명시적으로 새로고침했을 때만 메일 서버를 조회하고,
// 반환된 목록과 본문을 SQLite 캐시에 저장한다.
func (s *Service) ListRefresh(accID, folder string) ([]types.Message, error) {
	page, err := s.ListRefreshPage(accID, folder, "")
	return page.Messages, err
}

// ListRefreshPage는 서버에서 지정한 페이지를 조회한다.
// 첫 페이지는 캐시를 교체하고, 이후 페이지는 ListMore가 기존 캐시에 합친다.
func (s *Service) ListRefreshPage(accID, folder, pageToken string) (types.MessagePage, error) {
	acc, cred, err := s.credentials(accID)
	if err != nil {
		return types.MessagePage{}, err
	}
	var page types.MessagePage
	if acc.AuthType == account.AuthOAuth2 {
		client, clientErr := s.newGmailClient(acc, cred)
		if clientErr != nil {
			return types.MessagePage{}, clientErr
		}
		page, err = client.ListPage(folder, pageToken)
	} else {
		page, err = s.receiver.ListPage(acc, cred, folder, pageToken)
	}
	if err != nil {
		return types.MessagePage{}, err
	}
	if pageToken == "" {
		if err := s.store.CachePage(accID, folder, page); err != nil {
			return types.MessagePage{}, err
		}
	}
	return page, nil
}

// ListMore는 현재 캐시에 서버의 다음 페이지를 추가하고 중복 메시지는 제거한다.
func (s *Service) ListMore(accID, folder, pageToken string) (types.MessagePage, error) {
	if pageToken == "" {
		return types.MessagePage{}, fmt.Errorf("다음 이메일 페이지가 없습니다")
	}
	page, err := s.ListRefreshPage(accID, folder, pageToken)
	if err != nil {
		return types.MessagePage{}, err
	}
	current, _, err := s.store.CachedPage(accID, folder)
	if err != nil {
		return types.MessagePage{}, err
	}
	merged := append(current.Messages, page.Messages...)
	seen := make(map[string]bool, len(merged))
	unique := merged[:0]
	for _, msg := range merged {
		key := msg.ID
		if key == "" {
			key = fmt.Sprintf("uid:%d", msg.UID)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		unique = append(unique, msg)
	}
	if err := s.store.CachePage(accID, folder, types.MessagePage{Messages: unique, NextPageToken: page.NextPageToken}); err != nil {
		return types.MessagePage{}, err
	}
	page.Messages = unique
	return page, nil
}

// MessageMarkRead는 메일 서버의 읽음 상태를 변경하고 SQLite 캐시에도 반영한다.
// Gmail 계정은 UNREAD 라벨을, IMAP 계정은 \Seen 플래그를 사용한다.
// messageID는 Gmail 원격 메시지 ID이며 IMAP 계정에서는 비어 있고 uid로 식별한다.
func (s *Service) MessageMarkRead(accID, folder, messageID string, uid uint32, read bool) error {
	acc, cred, err := s.credentials(accID)
	if err != nil {
		return err
	}
	if acc.AuthType == account.AuthOAuth2 {
		client, err := s.messageGmailClient(acc, cred, messageID)
		if err != nil {
			return err
		}
		if err := client.SetUnread(messageID, !read); err != nil {
			return err
		}
	} else if err := s.receiver.SetSeen(acc, cred, folder, uid, read); err != nil {
		return err
	}
	return s.store.SetCachedUnread(accID, folder, messageID, uid, !read)
}

// MessageDelete는 메일 서버에서 메일을 삭제하고 캐시에서도 제거한다.
// Gmail 계정은 휴지통으로 이동하고, IMAP 계정은 \Deleted 플래그 후 EXPUNGE로 폴더에서 지운다.
func (s *Service) MessageDelete(accID, folder, messageID string, uid uint32) error {
	acc, cred, err := s.credentials(accID)
	if err != nil {
		return err
	}
	if acc.AuthType == account.AuthOAuth2 {
		client, err := s.messageGmailClient(acc, cred, messageID)
		if err != nil {
			return err
		}
		if err := client.Trash(messageID); err != nil {
			return err
		}
	} else if err := s.receiver.Delete(acc, cred, folder, uid); err != nil {
		return err
	}
	return s.store.RemoveCached(accID, folder, messageID, uid)
}

// messageGmailClient는 단일 메시지를 다루는 OAuth 계정용 Gmail 클라이언트를 만든다.
// 이전 버전이 캐시한 메시지에는 원격 ID가 없으므로, 그 경우 새로고침을 안내한다.
func (s *Service) messageGmailClient(acc *account.Account, cred, messageID string) (*gmail.Client, error) {
	if strings.TrimSpace(messageID) == "" {
		return nil, fmt.Errorf("Gmail 메시지 ID가 없습니다. 목록을 새로고침한 뒤 다시 시도하세요")
	}
	return s.newGmailClient(acc, cred)
}

// AttachmentData는 원문 MIME 메시지에서 첨부파일 바이트를 추출해 Base64로 반환한다.
// 프론트엔드는 List 응답에 포함된 원문과 첨부파일 인덱스를 사용하므로,
// 다운로드나 미리보기 때 메일 서버를 다시 조회하지 않는다.
func (s *Service) AttachmentData(raw string, index int) (string, error) {
	data, err := imap.ExtractRawAttachment([]byte(raw), index)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}
