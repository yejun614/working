package store

import (
	"testing"

	"working/internal/config"
	"working/internal/modules/account/types"
)

// staticLookup은 "<서비스>/<계정ID>" 키로 자격증명을 돌려주는 테스트용 조회 함수이다.
func staticLookup(values map[string]string) credentialLookup {
	return func(service, id string) string { return values[service+"/"+id] }
}

func mailService() string     { return config.AppName + ".email" }
func calendarService() string { return config.AppName + ".calendar" }

func TestMergeLegacyMergesGoogleAccountByAddress(t *testing.T) {
	mails := []legacyMailAccount{{ID: "mail-1", Name: "지메일", Email: "user@gmail.com", AuthType: "oauth2"}}
	cals := []legacyCalendarAccount{{
		ID: "cal-1", Name: "Google 캘린더", Source: "caldav", Username: "user@gmail.com",
		AuthType: "oauth2", CalDAVURL: "https://apidata.googleusercontent.com/caldav/v2",
		SyncEnabled: true, AuthError: "oauth2: \"invalid_grant\"",
	}}
	lookup := staticLookup(map[string]string{
		mailService() + "/mail-1":    `{"access_token":"mail"}`,
		calendarService() + "/cal-1": `{"access_token":"calendar"}`,
	})

	accounts, credentials, remapped := mergeLegacy(mails, cals, lookup)

	if len(accounts) != 1 {
		t.Fatalf("계정 수 = %d, want 1 (병합되어야 함)", len(accounts))
	}
	merged := accounts[0]
	if merged.ID != "mail-1" {
		t.Fatalf("병합 계정 ID = %q, want mail-1 (이메일 계정 ID를 유지해야 함)", merged.ID)
	}
	if !merged.UsesMail() || !merged.UsesCalendar() {
		t.Fatalf("병합 계정이 메일/캘린더를 모두 갖지 않음: %+v", merged)
	}
	if merged.Calendar.URL != cals[0].CalDAVURL || !merged.Calendar.SyncEnabled {
		t.Fatalf("캘린더 설정이 옮겨지지 않음: %+v", merged.Calendar)
	}
	// 살아남은 이메일 쪽 토큰을 쓰므로 캘린더 쪽 재인증 안내는 따라오지 않는다.
	if merged.AuthError != "" {
		t.Fatalf("AuthError = %q, want 빈 문자열", merged.AuthError)
	}
	if got := credentials["mail-1"]; got != `{"access_token":"mail"}` {
		t.Fatalf("자격증명 = %q, want 이메일 계정 토큰", got)
	}
	if remapped["cal-1"] != "mail-1" {
		t.Fatalf("일정 캐시 재매핑 = %v, want cal-1 -> mail-1", remapped)
	}
}

func TestMergeLegacyKeepsAccountsSeparate(t *testing.T) {
	tests := []struct {
		name        string
		mail        legacyMailAccount
		cal         legacyCalendarAccount
		credentials map[string]string
	}{
		{
			name: "주소가 다르면 병합하지 않음",
			mail: legacyMailAccount{ID: "mail-1", Name: "회사", Email: "me@corp.com", AuthType: "password"},
			cal:  legacyCalendarAccount{ID: "cal-1", Name: "개인", Source: "caldav", Username: "me@icloud.com", AuthType: "basic", CalDAVURL: "https://caldav.icloud.com"},
		},
		{
			name: "인증 방식이 다르면 병합하지 않음",
			mail: legacyMailAccount{ID: "mail-1", Name: "지메일", Email: "user@gmail.com", AuthType: "password"},
			cal:  legacyCalendarAccount{ID: "cal-1", Name: "구글", Source: "caldav", Username: "user@gmail.com", AuthType: "oauth2", CalDAVURL: "https://apidata.googleusercontent.com/caldav/v2"},
		},
		{
			name: "비밀번호가 다르면 병합하지 않음",
			mail: legacyMailAccount{ID: "mail-1", Name: "아이클라우드", Email: "me@icloud.com", AuthType: "password"},
			cal:  legacyCalendarAccount{ID: "cal-1", Name: "아이클라우드 캘린더", Source: "caldav", Username: "me@icloud.com", AuthType: "basic", CalDAVURL: "https://caldav.icloud.com"},
			credentials: map[string]string{
				config.AppName + ".email/mail-1":   "mail-app-password",
				config.AppName + ".calendar/cal-1": "caldav-app-password",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			accounts, _, remapped := mergeLegacy([]legacyMailAccount{tt.mail}, []legacyCalendarAccount{tt.cal}, staticLookup(tt.credentials))
			if len(accounts) != 2 {
				t.Fatalf("계정 수 = %d, want 2 (분리 유지)", len(accounts))
			}
			if len(remapped) != 0 {
				t.Fatalf("재매핑 = %v, want 없음 (ID가 그대로여야 함)", remapped)
			}
		})
	}
}

func TestMergeLegacyMergesIdenticalPasswordCredential(t *testing.T) {
	mails := []legacyMailAccount{{ID: "mail-1", Name: "넥스트클라우드", Email: "me@example.com", AuthType: "password"}}
	cals := []legacyCalendarAccount{{ID: "cal-1", Name: "넥스트클라우드 캘린더", Source: "caldav", Username: "me@example.com", AuthType: "basic", CalDAVURL: "https://cloud.example.com/dav"}}
	lookup := staticLookup(map[string]string{
		mailService() + "/mail-1":    "same-password",
		calendarService() + "/cal-1": "same-password",
	})

	accounts, credentials, remapped := mergeLegacy(mails, cals, lookup)

	if len(accounts) != 1 {
		t.Fatalf("계정 수 = %d, want 1 (자격증명이 같으면 병합)", len(accounts))
	}
	if credentials["mail-1"] != "same-password" {
		t.Fatalf("자격증명 = %q, want same-password", credentials["mail-1"])
	}
	if remapped["cal-1"] != "mail-1" {
		t.Fatalf("재매핑 = %v, want cal-1 -> mail-1", remapped)
	}
}

func TestMergeLegacyKeepsLocalCalendarStandalone(t *testing.T) {
	cals := []legacyCalendarAccount{{ID: "cal-local", Name: "개인 일정", Source: "local", Color: "#4f7cff"}}

	accounts, _, remapped := mergeLegacy(nil, cals, staticLookup(nil))

	if len(accounts) != 1 {
		t.Fatalf("계정 수 = %d, want 1", len(accounts))
	}
	if accounts[0].ID != "cal-local" {
		t.Fatalf("ID = %q, want cal-local (일정 캐시 키를 유지해야 함)", accounts[0].ID)
	}
	if accounts[0].UsesMail() {
		t.Fatalf("로컬 캘린더 계정에 메일 설정이 생김: %+v", accounts[0].Mail)
	}
	if accounts[0].Calendar.Source != types.CalendarSourceLocal {
		t.Fatalf("Source = %q, want local", accounts[0].Calendar.Source)
	}
	if len(remapped) != 0 {
		t.Fatalf("재매핑 = %v, want 없음", remapped)
	}
}

func TestMergeLegacyUsesCalendarTokenWhenMailTokenMissing(t *testing.T) {
	mails := []legacyMailAccount{{ID: "mail-1", Name: "지메일", Email: "user@gmail.com", AuthType: "oauth2"}}
	cals := []legacyCalendarAccount{{ID: "cal-1", Name: "구글", Source: "caldav", Username: "user@gmail.com", AuthType: "oauth2", CalDAVURL: "https://apidata.googleusercontent.com/caldav/v2", AuthError: "만료됨"}}
	lookup := staticLookup(map[string]string{calendarService() + "/cal-1": `{"access_token":"calendar"}`})

	accounts, credentials, _ := mergeLegacy(mails, cals, lookup)

	if len(accounts) != 1 {
		t.Fatalf("계정 수 = %d, want 1", len(accounts))
	}
	if credentials["mail-1"] != `{"access_token":"calendar"}` {
		t.Fatalf("자격증명 = %q, want 캘린더 토큰", credentials["mail-1"])
	}
	// 캘린더 토큰을 그대로 물려받았으므로 그쪽 재인증 안내도 함께 옮긴다.
	if accounts[0].AuthError != "만료됨" {
		t.Fatalf("AuthError = %q, want 만료됨", accounts[0].AuthError)
	}
}
