// Package account 는 메일과 캘린더가 공유하는 통합 계정 모듈이다.
//
// 계정 등록·수정·삭제와 Google OAuth 인증을 한곳에서 처리하고,
// 이메일/캘린더 모듈은 이 모듈의 저장소를 읽어 자기 기능에 필요한 계정만 사용한다.
// 다른 모듈과 마찬가지로 main.go에서 Service를 등록하면 앱에 포함된다.
package account

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"working/internal/config"
	"working/internal/googleoauth"
	"working/internal/modules/account/store"
	"working/internal/modules/account/types"
	calendarprovider "working/internal/modules/calendar/provider"
	mailprovider "working/internal/modules/email/provider"
)

// googleScopes는 Gmail과 Google 캘린더를 함께 사용하기 위한 OAuth scope이다.
// 토큰 하나로 두 모듈을 모두 사용하므로 인증도 한 번만 하면 된다.
var googleScopes = []string{
	"https://www.googleapis.com/auth/calendar",
	"https://www.googleapis.com/auth/gmail.modify",
	"https://www.googleapis.com/auth/gmail.send",
}

// Service는 프론트엔드에 바인딩되는 통합 계정 서비스이다.
type Service struct {
	store *store.Store
}

// NewService는 통합 계정 모듈 Service를 생성한다.
func NewService() (*Service, error) {
	st, err := store.New()
	if err != nil {
		return nil, err
	}
	return &Service{store: st}, nil
}

// ServiceShutdown은 Wails 종료 시 호출되는 훅이다.
func (s *Service) ServiceShutdown() {}

// List는 등록된 모든 계정을 반환한다(자격증명 제외).
func (s *Service) List() ([]types.Account, error) {
	return s.store.List()
}

// Get은 ID로 계정을 조회한다.
func (s *Service) Get(id string) (*types.Account, error) {
	return s.store.Get(id)
}

// Create는 새 계정을 등록하고 자격증명을 키체인에 저장한다.
// 메일과 캘린더 중 최소 하나는 사용하도록 설정해야 한다.
func (s *Service) Create(acc *types.Account, credential string) (string, error) {
	if err := validate(acc); err != nil {
		return "", err
	}
	if needsCredential(acc) && credential == "" {
		return "", fmt.Errorf("비밀번호/토큰은 필수입니다")
	}
	acc.ID = newID()
	if acc.AuthType == "" {
		acc.AuthType = types.AuthPassword
	}
	if err := s.store.Save(acc, credential); err != nil {
		return "", err
	}
	return acc.ID, nil
}

// Update는 기존 계정을 갱신한다.
// credential이 빈 문자열이면 키체인의 기존 자격증명을 유지한다.
func (s *Service) Update(acc *types.Account, credential string) error {
	if acc == nil || acc.ID == "" {
		return fmt.Errorf("계정 ID가 필요합니다")
	}
	existing, err := s.store.Get(acc.ID)
	if err != nil {
		return err
	}
	if err := validate(acc); err != nil {
		return err
	}
	if acc.AuthType == "" {
		acc.AuthType = existing.AuthType
	}
	// 계정 정보 수정은 인증 상태와 무관하므로 재인증 안내를 그대로 유지한다.
	acc.AuthError = existing.AuthError
	return s.store.Save(acc, credential)
}

// Delete는 계정과 키체인 자격증명을 함께 삭제한다.
// 해당 계정의 메일 캐시와 일정 캐시는 각 모듈이 계정을 찾지 못하면 사용하지 않는다.
func (s *Service) Delete(id string) error {
	if id == "" {
		return fmt.Errorf("계정 ID가 필요합니다")
	}
	return s.store.Delete(id)
}

// GoogleConnect는 Google OAuth 인증을 수행하고 계정을 저장한다.
// 발급된 토큰 하나를 Gmail과 Google 캘린더가 함께 사용한다.
func (s *Service) GoogleConnect(acc *types.Account) (string, error) {
	if acc == nil {
		return "", fmt.Errorf("계정 정보가 없습니다")
	}
	if strings.TrimSpace(acc.Email) == "" {
		return "", fmt.Errorf("Google 계정 이메일은 필수입니다")
	}
	if strings.TrimSpace(acc.Name) == "" {
		acc.Name = acc.Email
	}
	acc.AuthType = types.AuthOAuth2
	applyGoogleDefaults(acc)
	if err := validate(acc); err != nil {
		return "", err
	}

	credential, err := authenticateGoogle()
	if err != nil {
		return "", err
	}
	if acc.ID == "" {
		acc.ID = newID()
	}
	acc.AuthError = ""
	if err := s.store.Save(acc, credential); err != nil {
		return "", err
	}
	return acc.ID, nil
}

// GoogleReconnect는 토큰이 만료·철회된 기존 Google 계정을 다시 인증한다.
// 계정 설정은 그대로 두고 키체인의 토큰만 새로 발급받아 교체하며,
// 성공하면 메일과 캘린더의 재인증 안내가 함께 해제된다.
func (s *Service) GoogleReconnect(id string) error {
	acc, err := s.store.Get(id)
	if err != nil {
		return err
	}
	if acc.AuthType != types.AuthOAuth2 {
		return fmt.Errorf("Google OAuth 계정이 아닙니다: %s", acc.Name)
	}
	credential, err := authenticateGoogle()
	if err != nil {
		return err
	}
	acc.AuthError = ""
	return s.store.Save(acc, credential)
}

// MailProviderLookup은 이메일 도메인으로 SMTP/IMAP 서버 설정을 찾는다.
// 일치하는 사전 정의 제공자가 없으면 nil을 반환한다.
func (s *Service) MailProviderLookup(email string) *mailprovider.Provider {
	return mailprovider.LookupByEmail(email)
}

// MailProviderList는 사전 정의된 메일 제공자 목록을 반환한다.
func (s *Service) MailProviderList() []mailprovider.Provider {
	return mailprovider.All()
}

// CalendarProviderLookup은 이메일 도메인으로 CalDAV 서버 설정을 찾는다.
func (s *Service) CalendarProviderLookup(email string) *calendarprovider.Provider {
	return calendarprovider.LookupByEmail(email)
}

// CalendarProviderList는 사전 정의된 캘린더 제공자 목록을 반환한다.
func (s *Service) CalendarProviderList() []calendarprovider.Provider {
	return calendarprovider.All()
}

// applyGoogleDefaults는 Google 계정에 필요한 기본 서버 설정을 채운다.
// Gmail은 API로 송수신하므로 IMAP/SMTP 설정 없이도 동작한다.
func applyGoogleDefaults(acc *types.Account) {
	if acc.Calendar != nil {
		acc.Calendar.Source = types.CalendarSourceCalDAV
		if strings.TrimSpace(acc.Calendar.URL) == "" {
			acc.Calendar.URL = "https://apidata.googleusercontent.com/caldav/v2"
		}
		if strings.TrimSpace(acc.Calendar.Username) == "" {
			acc.Calendar.Username = acc.Email
		}
	}
}

// validate는 계정 설정의 필수 값을 검사한다.
func validate(acc *types.Account) error {
	if acc == nil {
		return fmt.Errorf("계정 정보가 없습니다")
	}
	if strings.TrimSpace(acc.Name) == "" {
		return fmt.Errorf("계정 이름은 필수입니다")
	}
	if !acc.UsesMail() && !acc.UsesCalendar() {
		return fmt.Errorf("메일과 캘린더 중 최소 하나는 사용해야 합니다")
	}
	if acc.UsesMail() && strings.TrimSpace(acc.Email) == "" {
		return fmt.Errorf("메일을 사용하려면 이메일 주소가 필요합니다")
	}
	if acc.UsesCalendar() && acc.Calendar.Source == types.CalendarSourceCalDAV {
		if strings.TrimSpace(acc.Calendar.URL) == "" {
			return fmt.Errorf("CalDAV 서버 URL은 필수입니다")
		}
		if strings.TrimSpace(acc.CalendarUsername()) == "" {
			return fmt.Errorf("CalDAV 사용자 이름은 필수입니다")
		}
	}
	return nil
}

// needsCredential은 계정 등록에 자격증명이 필요한지 여부이다.
// 로컬 캘린더만 사용하는 계정은 외부 서버에 접속하지 않으므로 필요 없다.
func needsCredential(acc *types.Account) bool {
	if acc.UsesMail() {
		return true
	}
	return acc.UsesCalendar() && acc.Calendar.Source == types.CalendarSourceCalDAV
}

// authenticateGoogle은 Google OAuth 인증을 수행하고 키체인에 저장할 토큰 JSON을 반환한다.
func authenticateGoogle() (string, error) {
	clientID := config.GoogleClientID()
	if clientID == "" {
		return "", fmt.Errorf("GOOGLE_CLIENT_ID가 설정되지 않았습니다")
	}
	token, err := googleoauth.Authenticate(clientID, config.GoogleClientSecret(), googleScopes)
	if err != nil {
		return "", err
	}
	credential, err := json.Marshal(token)
	if err != nil {
		return "", fmt.Errorf("OAuth 토큰 저장 형식 변환 실패: %w", err)
	}
	return string(credential), nil
}

// newID는 16바이트 난수 기반의 계정 ID를 생성한다.
func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
