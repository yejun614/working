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
	"encoding/hex"
	"fmt"
	"strings"

	"working/internal/modules/email/account"
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
		acc.AuthType = "password"
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
	return s.receiver.Folders(acc, cred)
}

// List는 지정한 폴더의 최신 메시지를 조회한다.
// folder가 빈 문자열이면 INBOX를 사용한다.
func (s *Service) List(accID, folder string) ([]types.Message, error) {
	acc, err := s.store.Get(accID)
	if err != nil {
		return nil, err
	}
	cred, err := s.store.Credential(accID)
	if err != nil {
		return nil, err
	}
	return s.receiver.List(acc, cred, folder)
}

// newID는 16바이트 난수 기반의 계정 ID를 생성한다.
func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}