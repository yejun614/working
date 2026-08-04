// Package email 은 "working" 앱의 이메일 모듈 진입점이다.
//
// 이 모듈은 SMTP 발송, IMAP 수신, 계정 관리(키체인 자격증명 포함)
// 기능을 Wails Service 형태로 프론트엔드에 노출한다.
// 다른 모듈과 마찬가지로 internal/modules/<이름> 패키지로 격리되며,
// main.go에서 원하는 모듈의 Service만 application.NewService로 등록하면
// 해당 모듈만 앱에 포함된다.
package email

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"working/internal/config"
	"working/internal/googleoauth"
	"working/internal/modules/email/account"
	"working/internal/modules/email/gmail"
	"working/internal/modules/email/imap"
	"working/internal/modules/email/provider"
	"working/internal/modules/email/smtp"
	"working/internal/modules/email/store"
	"working/internal/modules/email/types"
)

// Service는 프론트엔드에 바인딩되는 이메일 모듈 서비스이다.
// 모든 메서드는 Wails를 통해 JS/TS에서 호출 가능하다.
type Service struct {
	store    *store.Store
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
	return &Service{
		store:    st,
		sender:   smtp.New(),
		receiver: imap.New(),
	}, nil
}

// ServiceShutdown은 Wails 종료 시 호출되는 훅이다.
// 현재는 유지할 상태가 없으므로 아무 작업도 하지 않는다.
func (s *Service) ServiceShutdown() {}

// ProviderList는 사전 정의된 이메일 제공자 목록을 반환한다.
// 프론트엔드에서 계정 추가 시 드롭다운으로 표시하거나,
// 이메일 도메인 자동 인식에 사용한다.
func (s *Service) ProviderList() []provider.Provider {
	return provider.All()
}

// ProviderLookupByEmail은 이메일 주소의 도메인으로 제공자를 찾는다.
// 일치하는 사전 정의 제공자가 없으면 nil을 반환한다.
// 프론트엔드에서 이메일 입력 중 서버 필드 자동 채우기에 사용한다.
func (s *Service) ProviderLookupByEmail(email string) *provider.Provider {
	return provider.LookupByEmail(email)
}

// AccountList는 등록된 모든 계정을 반환한다(자격증명 제외).
func (s *Service) AccountList() ([]account.Account, error) {
	return s.store.List()
}

// AccountGet은 ID로 계정을 조회한다.
func (s *Service) AccountGet(id string) (*account.Account, error) {
	return s.store.Get(id)
}

// AccountCreate는 새 계정을 등록하고 자격증명을 키체인에 저장한다.
// id는 자동 생성되어 반환된다. credential은 빈 문자열이면 안 된다.
func (s *Service) AccountCreate(acc *account.Account, credential string) (string, error) {
	if acc == nil {
		return "", fmt.Errorf("계정 정보가 없습니다")
	}
	if strings.TrimSpace(acc.Email) == "" {
		return "", fmt.Errorf("이메일 주소는 필수입니다")
	}
	if credential == "" {
		return "", fmt.Errorf("자격증명은 필수입니다")
	}
	acc.ID = newID()
	if acc.AuthType == "" {
		acc.AuthType = account.AuthPassword
	}
	if err := s.store.Save(acc, credential); err != nil {
		return "", err
	}
	return acc.ID, nil
}

// AccountUpdate는 기존 계정 메타데이터를 갱신한다.
// credential이 빈 문자열이 아니면 키체인 자격증명도 함께 갱신하고,
// 빈 문자열이면 기존 자격증명을 유지한다.
func (s *Service) AccountUpdate(acc *account.Account, credential string) error {
	if acc == nil || acc.ID == "" {
		return fmt.Errorf("계정 ID가 필요합니다")
	}
	existing, err := s.store.Get(acc.ID)
	if err != nil {
		return err
	}
	if acc.Email == "" {
		acc.Email = existing.Email
	}
	if acc.AuthType == "" {
		acc.AuthType = existing.AuthType
	}
	return s.store.Save(acc, credential)
}

// AccountDelete는 계정과 키체인 자격증명을 함께 삭제한다.
// 메타데이터가 이미 없더라도 키체인 잔여물은 제거를 시도한다.
func (s *Service) AccountDelete(id string) error {
	if id == "" {
		return fmt.Errorf("계정 ID가 필요합니다")
	}
	return s.store.Delete(id)
}

// GoogleOAuthConnect는 Gmail API 사용을 위한 Google OAuth 인증을 수행하고 계정을 저장한다.
// Calendar와 동일한 Google Client ID를 사용하며, Gmail 읽기/수정/발송 scope를 요청한다.
func (s *Service) GoogleOAuthConnect(acc *account.Account) (string, error) {
	if acc == nil {
		return "", fmt.Errorf("계정 정보가 없습니다")
	}
	if strings.TrimSpace(acc.Email) == "" {
		return "", fmt.Errorf("Google 계정 이메일은 필수입니다")
	}
	clientID := config.GoogleClientID()
	if clientID == "" {
		return "", fmt.Errorf("GOOGLE_CLIENT_ID가 설정되지 않았습니다")
	}
	token, err := googleoauth.Authenticate(clientID, config.GoogleClientSecret(), []string{
		"https://www.googleapis.com/auth/gmail.modify",
		"https://www.googleapis.com/auth/gmail.send",
		"https://www.googleapis.com/auth/calendar",
	})
	if err != nil {
		return "", err
	}
	credential, err := json.Marshal(token)
	if err != nil {
		return "", fmt.Errorf("Google OAuth 토큰 저장 형식 변환 실패: %w", err)
	}
	if acc.ID == "" {
		acc.ID = newID()
	}
	acc.AuthType = account.AuthOAuth2
	if err := s.store.Save(acc, string(credential)); err != nil {
		return "", err
	}
	return acc.ID, nil
}

// Send는 지정한 계정으로 메일을 발송한다.
// accID로 계정을 조회하고, 키체인에서 자격증명을 꺼내 SMTP 전송에 사용한다.
func (s *Service) Send(accID string, msg *types.Message) error {
	acc, err := s.store.Get(accID)
	if err != nil {
		return err
	}
	cred, err := s.store.Credential(accID)
	if err != nil {
		return err
	}
	if acc.AuthType == account.AuthOAuth2 {
		client, err := gmail.New(cred, func(updated string) error {
			return s.store.Save(acc, updated)
		})
		if err != nil {
			return err
		}
		return client.Send(acc, msg)
	}
	return s.sender.Send(acc, cred, msg)
}

// Folders는 지정한 계정의 IMAP 폴더 목록을 반환한다.
func (s *Service) Folders(accID string) ([]string, error) {
	acc, err := s.store.Get(accID)
	if err != nil {
		return nil, err
	}
	cred, err := s.store.Credential(accID)
	if err != nil {
		return nil, err
	}
	if acc.AuthType == account.AuthOAuth2 {
		client, err := gmail.New(cred, func(updated string) error {
			return s.store.Save(acc, updated)
		})
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
	acc, err := s.store.Get(accID)
	if err != nil {
		return types.MessagePage{}, err
	}
	cred, err := s.store.Credential(accID)
	if err != nil {
		return types.MessagePage{}, err
	}
	var page types.MessagePage
	if acc.AuthType == account.AuthOAuth2 {
		client, clientErr := gmail.New(cred, func(updated string) error { return s.store.Save(acc, updated) })
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
		client, err := s.gmailClient(acc, cred, messageID)
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

// credentials는 계정 메타데이터와 키체인 자격증명을 함께 조회한다.
func (s *Service) credentials(accID string) (*account.Account, string, error) {
	acc, err := s.store.Get(accID)
	if err != nil {
		return nil, "", err
	}
	cred, err := s.store.Credential(accID)
	if err != nil {
		return nil, "", err
	}
	return acc, cred, nil
}

// gmailClient는 OAuth 계정용 Gmail 클라이언트를 만든다.
// 이전 버전이 캐시한 메시지에는 원격 ID가 없으므로, 그 경우 새로고침을 안내한다.
func (s *Service) gmailClient(acc *account.Account, cred, messageID string) (*gmail.Client, error) {
	if strings.TrimSpace(messageID) == "" {
		return nil, fmt.Errorf("Gmail 메시지 ID가 없습니다. 목록을 새로고침한 뒤 다시 시도하세요")
	}
	return gmail.New(cred, func(updated string) error { return s.store.Save(acc, updated) })
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

// newID는 16바이트 난수 기반의 계정 ID를 생성한다.
func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
