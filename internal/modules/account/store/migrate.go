package store

import (
	"fmt"
	"strings"

	"github.com/zalando/go-keyring"

	"working/internal/config"
	"working/internal/modules/account/types"
	"working/internal/storage"
)

// 이관 전 모듈별 계정이 저장돼 있던 SQLite 키와 키체인 서비스 이름이다.
// 이관 후에도 원본은 지우지 않고 남겨 둔다. 통합 계정 목록만 삭제하면
// 다음 실행 때 같은 자리에서 다시 이관되므로 되돌리기가 쉽다.
const (
	legacyMailAccountsKey     = "email.accounts"
	legacyCalendarAccountsKey = "calendar.accounts"
	legacyCalendarEventsKey   = "calendar.events"
)

// legacyMailAccount는 이관 전 이메일 모듈의 계정 JSON 구조이다.
type legacyMailAccount struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Email       string              `json:"email"`
	DisplayName string              `json:"displayName,omitempty"`
	SMTP        *types.ServerConfig `json:"smtp,omitempty"`
	IMAP        *types.ServerConfig `json:"imap,omitempty"`
	AuthType    string              `json:"authType,omitempty"`
}

// legacyCalendarAccount는 이관 전 캘린더 모듈의 계정 JSON 구조이다.
type legacyCalendarAccount struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Source      string `json:"source"`
	Color       string `json:"color,omitempty"`
	CalDAVURL   string `json:"caldavUrl,omitempty"`
	Username    string `json:"username,omitempty"`
	AuthType    string `json:"authType,omitempty"`
	SyncEnabled bool   `json:"syncEnabled,omitempty"`
	LastSyncAt  string `json:"lastSyncAt,omitempty"`
	AuthError   string `json:"authError,omitempty"`
}

// legacyCalendarEvent는 일정 캐시에서 계정 ID만 다시 매핑하기 위한 최소 구조이다.
// 나머지 필드는 손대지 않도록 원본 JSON을 그대로 담아 둔다.
type legacyCalendarEvent map[string]any

// migrateLegacyAccounts는 모듈별로 나뉘어 있던 계정을 통합 계정 목록으로 한 번 이관한다.
// 통합 목록이 이미 있으면 아무 것도 하지 않으므로 여러 모듈이 저장소를 만들어도 안전하다.
func (s *Store) migrateLegacyAccounts() error {
	var existing []types.Account
	found, err := storage.GetJSON(s.db, accountsKey, &existing)
	if err != nil {
		return err
	}
	if found {
		return nil
	}

	mails, err := s.legacyMailAccounts()
	if err != nil {
		return err
	}
	cals, err := s.legacyCalendarAccounts()
	if err != nil {
		return err
	}
	if len(mails) == 0 && len(cals) == 0 {
		return s.save([]types.Account{})
	}

	accounts, credentials, remapped := mergeLegacy(mails, cals, legacyCredential)
	for id, credential := range credentials {
		if credential == "" {
			continue
		}
		// 이관 실패는 계정 목록 자체를 막지 않는다. 자격증명이 없으면
		// 해당 계정만 재인증하거나 비밀번호를 다시 입력하면 된다.
		_ = keyring.Set(s.keyringService, id, credential)
	}
	// 병합으로 캘린더 계정 ID가 바뀌면 일정 캐시가 고아가 되므로 새 ID로 다시 매핑한다.
	if err := s.remapCalendarEvents(remapped); err != nil {
		return err
	}
	return s.save(accounts)
}

// legacyMailAccounts는 이메일 모듈이 쓰던 계정 목록을 읽는다.
// SQLite로 옮기기 전 버전의 JSON 파일도 함께 확인한다.
func (s *Store) legacyMailAccounts() ([]legacyMailAccount, error) {
	var out []legacyMailAccount
	found, err := storage.GetJSON(s.db, legacyMailAccountsKey, &out)
	if err != nil {
		return nil, err
	}
	if found {
		return out, nil
	}
	if _, err := storage.LegacyJSON(configDir(), "email_accounts.json", &out); err != nil {
		return nil, fmt.Errorf("이메일 계정 이관 실패: %w", err)
	}
	return out, nil
}

// legacyCalendarAccounts는 캘린더 모듈이 쓰던 계정 목록을 읽는다.
func (s *Store) legacyCalendarAccounts() ([]legacyCalendarAccount, error) {
	var out []legacyCalendarAccount
	found, err := storage.GetJSON(s.db, legacyCalendarAccountsKey, &out)
	if err != nil {
		return nil, err
	}
	if found {
		return out, nil
	}
	if _, err := storage.LegacyJSON(configDir(), "calendar_accounts.json", &out); err != nil {
		return nil, fmt.Errorf("캘린더 계정 이관 실패: %w", err)
	}
	return out, nil
}

func configDir() string { dir, _ := config.Dir(); return dir }

// mergeLegacy는 이메일 계정과 캘린더 계정을 하나의 통합 계정 목록으로 합친다.
//
// 이메일 주소가 같아도 자격증명이 서로 다르면 계정 하나에 담을 수 없으므로 분리해 둔다.
// OAuth 계정은 두 모듈이 캘린더와 Gmail scope를 똑같이 요청해 왔기 때문에 토큰 하나로
// 양쪽을 모두 쓸 수 있어 항상 병합한다. 비밀번호 계정은 메일과 CalDAV의 앱 비밀번호가
// 다를 수 있으므로 값이 완전히 같을 때만 병합한다.
//
// 병합 시에는 이메일 계정의 ID를 남긴다. 메일 캐시는 폴더마다 키가 나뉘어 있어
// 다시 매핑하기 어렵지만, 일정 캐시는 한 덩어리라 새 ID로 옮기기 쉽기 때문이다.
// 반환값은 통합 계정 목록, 계정 ID별로 옮겨 담을 자격증명,
// 그리고 사라진 캘린더 계정 ID에서 새 계정 ID로의 대응이다.
func mergeLegacy(mails []legacyMailAccount, cals []legacyCalendarAccount, lookup credentialLookup) ([]types.Account, map[string]string, map[string]string) {
	accounts := make([]types.Account, 0, len(mails)+len(cals))
	credentials := make(map[string]string, len(mails)+len(cals))
	remapped := make(map[string]string)

	for _, mail := range mails {
		account := types.Account{
			ID:          mail.ID,
			Name:        mail.Name,
			Email:       mail.Email,
			DisplayName: mail.DisplayName,
			AuthType:    normalizeAuthType(mail.AuthType),
			Mail:        &types.MailConfig{SMTP: mail.SMTP, IMAP: mail.IMAP},
		}
		accounts = append(accounts, account)
		credentials[account.ID] = lookup(config.AppName+".email", mail.ID)
	}

	for _, cal := range cals {
		calendar := &types.CalendarConfig{
			Source:      calendarSource(cal.Source),
			URL:         cal.CalDAVURL,
			Username:    cal.Username,
			Color:       cal.Color,
			SyncEnabled: cal.SyncEnabled,
			LastSyncAt:  cal.LastSyncAt,
		}
		calAuth := normalizeAuthType(cal.AuthType)
		calCredential := lookup(config.AppName+".calendar", cal.ID)

		if target := findMergeTarget(accounts, credentials, cal, calAuth, calCredential); target >= 0 {
			accounts[target].Calendar = calendar
			// OAuth 계정에서 이메일 쪽 토큰이 없으면 캘린더 토큰을 대신 쓴다.
			if credentials[accounts[target].ID] == "" {
				credentials[accounts[target].ID] = calCredential
				accounts[target].AuthError = cal.AuthError
			}
			remapped[cal.ID] = accounts[target].ID
			continue
		}

		account := types.Account{
			ID:        cal.ID,
			Name:      cal.Name,
			Email:     cal.Username,
			AuthType:  calAuth,
			Calendar:  calendar,
			AuthError: cal.AuthError,
		}
		accounts = append(accounts, account)
		credentials[account.ID] = calCredential
	}
	return accounts, credentials, remapped
}

// findMergeTarget은 캘린더 계정을 합칠 수 있는 이메일 계정의 인덱스를 찾는다.
// 합칠 수 없으면 -1을 반환한다.
func findMergeTarget(accounts []types.Account, credentials map[string]string, cal legacyCalendarAccount, calAuth types.AuthType, calCredential string) int {
	address := strings.TrimSpace(cal.Username)
	if address == "" || calendarSource(cal.Source) != types.CalendarSourceCalDAV {
		return -1
	}
	for i := range accounts {
		if accounts[i].Calendar != nil || !strings.EqualFold(accounts[i].Email, address) {
			continue
		}
		if accounts[i].AuthType != calAuth {
			continue
		}
		if calAuth != types.AuthOAuth2 && credentials[accounts[i].ID] != calCredential {
			continue
		}
		return i
	}
	return -1
}

// remapCalendarEvents는 병합으로 사라진 계정 ID를 가진 일정 캐시를 새 계정 ID로 옮긴다.
func (s *Store) remapCalendarEvents(remapped map[string]string) error {
	if len(remapped) == 0 {
		return nil
	}
	var events []legacyCalendarEvent
	found, err := storage.GetJSON(s.db, legacyCalendarEventsKey, &events)
	if err != nil {
		return err
	}
	if !found || len(events) == 0 {
		return nil
	}
	changed := false
	for _, event := range events {
		current, ok := event["calendarId"].(string)
		if !ok {
			continue
		}
		if next, ok := remapped[current]; ok {
			event["calendarId"] = next
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return storage.PutJSON(s.db, legacyCalendarEventsKey, events)
}

// credentialLookup은 모듈별 키체인에서 자격증명을 읽는 함수이다.
// 테스트에서 키체인 없이 병합 규칙만 검증할 수 있도록 분리했다.
type credentialLookup func(service, id string) string

// legacyCredential은 모듈별 키체인에서 자격증명을 읽는다.
// 값이 없으면 빈 문자열을 반환해 나머지 계정 이관을 계속한다.
func legacyCredential(service, id string) string {
	value, err := keyring.Get(service, id)
	if err != nil {
		return ""
	}
	return value
}

// normalizeAuthType은 모듈마다 달랐던 인증 방식 문자열을 통합 값으로 맞춘다.
// 캘린더 모듈의 "basic"은 이메일 모듈의 "password"와 같은 의미이다.
func normalizeAuthType(value string) types.AuthType {
	if strings.EqualFold(value, string(types.AuthOAuth2)) {
		return types.AuthOAuth2
	}
	return types.AuthPassword
}

func calendarSource(value string) types.CalendarSource {
	if strings.EqualFold(value, string(types.CalendarSourceCalDAV)) {
		return types.CalendarSourceCalDAV
	}
	return types.CalendarSourceLocal
}
