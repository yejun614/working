package calendar

import (
	"encoding/json"
	"fmt"
	"strings"

	"working/internal/config"
	"working/internal/googleoauth"
	"working/internal/modules/calendar/account"
)

// GoogleOAuthConnect는 Google OAuth 인증을 완료한 뒤 캘린더 계정을 저장한다.
// 인증 화면은 기본 브라우저에서 열고, authorization code 수신과 토큰 교환은
// 로컬 callback 서버를 통해 Go에서 수행한다.
func (s *Service) GoogleOAuthConnect(acc *account.Account) (string, error) {
	if acc == nil {
		return "", fmt.Errorf("계정 정보가 없습니다")
	}
	if strings.TrimSpace(acc.Name) == "" {
		return "", fmt.Errorf("계정 이름은 필수입니다")
	}
	if strings.TrimSpace(acc.Username) == "" {
		return "", fmt.Errorf("Google 계정 이메일은 필수입니다")
	}
	credential, err := authenticateGoogle()
	if err != nil {
		return "", err
	}

	acc.Source = account.SourceCalDAV
	acc.AuthType = account.AuthOAuth2
	if strings.TrimSpace(acc.CalDAVURL) == "" {
		acc.CalDAVURL = "https://apidata.googleusercontent.com/caldav/v2"
	}
	if acc.ID == "" {
		acc.ID = newID()
	}
	acc.AuthError = ""
	if err := s.store.SaveAccount(acc, credential); err != nil {
		return "", err
	}
	return acc.ID, nil
}

// GoogleOAuthReconnect는 토큰이 만료·철회된 기존 Google 계정을 다시 인증한다.
// 계정 설정은 그대로 두고 키체인의 토큰만 새로 발급받아 교체하며,
// 성공하면 계정에 기록된 재인증 안내를 해제한다.
func (s *Service) GoogleOAuthReconnect(accID string) error {
	acc, err := s.store.GetAccount(accID)
	if err != nil {
		return err
	}
	if acc.AuthType != account.AuthOAuth2 {
		return fmt.Errorf("Google OAuth 계정이 아닙니다: %s", acc.Name)
	}
	credential, err := authenticateGoogle()
	if err != nil {
		return err
	}
	acc.AuthError = ""
	return s.store.SaveAccount(acc, credential)
}

// googleScopes는 캘린더·Gmail 연동에 필요한 OAuth scope 목록이다.
var googleScopes = []string{
	"https://www.googleapis.com/auth/calendar",
	"https://www.googleapis.com/auth/gmail.modify",
	"https://www.googleapis.com/auth/gmail.send",
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
