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
	clientID := config.GoogleClientID()
	if clientID == "" {
		return "", fmt.Errorf("GOOGLE_CLIENT_ID가 설정되지 않았습니다")
	}
	clientSecret := config.GoogleClientSecret()
	token, err := googleoauth.Authenticate(clientID, clientSecret, []string{
		"https://www.googleapis.com/auth/calendar",
		"https://www.googleapis.com/auth/gmail.modify",
		"https://www.googleapis.com/auth/gmail.send",
	})
	if err != nil {
		return "", err
	}
	credential, err := json.Marshal(token)
	if err != nil {
		return "", fmt.Errorf("OAuth 토큰 저장 형식 변환 실패: %w", err)
	}

	acc.Source = account.SourceCalDAV
	acc.AuthType = account.AuthOAuth2
	if strings.TrimSpace(acc.CalDAVURL) == "" {
		acc.CalDAVURL = "https://apidata.googleusercontent.com/caldav/v2"
	}
	if acc.ID == "" {
		acc.ID = newID()
	}
	if err := s.store.SaveAccount(acc, string(credential)); err != nil {
		return "", err
	}
	return acc.ID, nil
}
