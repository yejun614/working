// Package provider 는 대표적인 캘린더 서비스(CalDAV) 제공자의
// 서버 설정을 사전 정의한다. 사용자가 제공자를 선택하면
// CalDAV URL 필드를 자동으로 채울 수 있다.
package provider

import "strings"

// Provider는 단일 캘린더 서비스 제공자의 메타데이터와
// CalDAV 서버 URL을 나타낸다.
type Provider struct {
	// ID는 제공자 식별자(예: "google", "apple").
	ID string `json:"id"`

	// Name은 표시 이름(예: "Google Calendar", "Apple iCloud").
	Name string `json:"name"`

	// Domains은 이 제공자에 해당하는 이메일/계정 도메인 목록.
	// 사용자 이름 입력 시 자동 감지에 사용된다.
	Domains []string `json:"domains"`

	// CalDAVURL은 CalDAV 서버 캘린더 홈 URL.
	CalDAVURL string `json:"caldavUrl"`

	// HelpURL은 제공자 인증 설정 도움말 URL.
	HelpURL string `json:"helpUrl,omitempty"`

	// Note는 사용자 안내 문구.
	Note string `json:"note,omitempty"`
}

// providers는 사전 정의된 제공자 목록이다.
var providers = []Provider{
	{
		ID:        "google",
		Name:      "Google Calendar",
		Domains:   []string{"gmail.com", "googlemail.com"},
		CalDAVURL: "https://apidata.googleusercontent.com/caldav/v2",
		HelpURL:   "https://developers.google.com/identity/protocols/oauth2",
		Note:      "Google OAuth 인증으로 연결합니다. 비밀번호를 입력하지 마세요.",
	},
	{
		ID:        "apple",
		Name:      "Apple iCloud",
		Domains:   []string{"icloud.com", "me.com", "mac.com"},
		CalDAVURL: "https://caldav.icloud.com",
		HelpURL:   "https://support.apple.com/HT204397",
		Note:      "Apple ID 2단계 인증 후 앱 전용 비밀번호가 필요합니다.",
	},
	{
		ID:        "outlook",
		Name:      "Outlook / Office 365",
		Domains:   []string{"outlook.com", "hotmail.com", "live.com", "office365.com"},
		CalDAVURL: "https://outlook.office365.com/caldav",
		HelpURL:   "https://support.microsoft.com/account-billing/how-to-get-and-use-app-passwords-58b6469b-4a2d-8e0a-93d3-aa5c52e1dc5e",
		Note:      "2단계 인증 사용 시 앱 비밀번호가 필요합니다.",
	},
	{
		ID:        "yahoo",
		Name:      "Yahoo Calendar",
		Domains:   []string{"yahoo.com", "yahoo.co.jp", "yahoo.co.kr"},
		CalDAVURL: "https://caldav.calendar.yahoo.com",
		HelpURL:   "https://help.yahoo.com/kb/account/sln15241.html",
		Note:      "계정 보안에서 앱 비밀번호 생성이 필요합니다.",
	},
	{
		ID:        "nextcloud",
		Name:      "Nextcloud / ownCloud",
		Domains:   []string{},
		CalDAVURL: "",
		HelpURL:   "",
		Note:      "Nextcloud 서버 주소 + /remote.php/dav/principals/users/<사용자명>/ 형식으로 입력하세요.",
	},
}

// All은 사전 정의된 모든 제공자 목록을 반환한다.
// 프론트엔드에서 사용자가 제공자를 직접 선택할 때 사용한다.
func All() []Provider {
	out := make([]Provider, len(providers))
	copy(out, providers)
	return out
}

// LookupByDomain은 계정 도메인으로 제공자를 찾는다.
// 일치하는 제공자가 없으면 nil을 반환한다.
func LookupByDomain(domain string) *Provider {
	domain = strings.ToLower(strings.TrimSpace(domain))
	for i := range providers {
		for _, d := range providers[i].Domains {
			if d == domain {
				return &providers[i]
			}
		}
	}
	return nil
}

// LookupByEmail은 이메일 주소에서 도메인을 추출해 제공자를 찾는다.
// 이메일 형식이 유효하지 않거나 알려진 제공자가 아니면 nil을 반환한다.
func LookupByEmail(email string) *Provider {
	email = strings.ToLower(strings.TrimSpace(email))
	at := strings.LastIndex(email, "@")
	if at < 0 || at == len(email)-1 {
		return nil
	}
	return LookupByDomain(email[at+1:])
}
