// Package provider 는 대표적인 이메일 서비스 제공자의 서버 설정을
// 사전 정의한다. 이메일 주소의 도메인으로부터 SMTP/IMAP 서버 설정을
// 자동으로 채울 수 있다.
package provider

import (
	"strings"

	"working/internal/modules/email/account"
)

// Provider는 단일 이메일 서비스 제공자의 메타데이터와 서버 설정을 나타낸다.
type Provider struct {
	// ID는 제공자 식별자(예: "naver", "gmail").
	ID string `json:"id"`

	// Name은 표시 이름(예: "Naver Mail", "Gmail").
	Name string `json:"name"`

	// Domains은 이 제공자에 해당하는 이메일 도메인 목록.
	// 사용자 이메일 주소의 도메인이 여기에 포함되면 이 제공자로 인식한다.
	Domains []string `json:"domains"`

	// HelpURL은 앱 비밀번호/2단계 인증 설정 도움말 URL.
	// 사용자에게 안내 링크로 표시한다.
	HelpURL string `json:"helpUrl,omitempty"`

	// Note는 사용자 안내 문구(예: "앱 비밀번호가 필요합니다").
	Note string `json:"note,omitempty"`

	// SMTP는 발송 서버 설정. nil이면 발송 미지원 제공자.
	SMTP *account.ServerConfig `json:"smtp,omitempty"`

	// IMAP은 수신 서버 설정. nil이면 수신 미지원 제공자.
	IMAP *account.ServerConfig `json:"imap,omitempty"`
}

// providers는 사전 정의된 제공자 목록이다.
// 순서가 중요하다: 더 구체적인 도메인이 먼저 와야 한다
// (예: 회사 도메인이 gmail.com이 아닌 경우 hosts 파일 기반 매칭이 우선).
var providers = []Provider{
	{
		ID:       "naver",
		Name:     "Naver Mail",
		Domains:  []string{"naver.com", "nate.com"},
		HelpURL:  "https://help.naver.com/service/30026/contents/18025",
		Note:     "IMAP/SMTP 사용 허용 및 앱 비밀번호(2단계 인증)가 필요합니다.",
		SMTP:     &account.ServerConfig{Host: "smtp.naver.com", Port: 587, Encryption: "starttls"},
		IMAP:     &account.ServerConfig{Host: "imap.naver.com", Port: 993, Encryption: "tls"},
	},
	{
		ID:       "daum",
		Name:     "Daum Mail (Kakao)",
		Domains:  []string{"daum.net", "hanmail.net", "kakao.com"},
		HelpURL:  "https://cs.daum.net/faq/36/00001993",
		Note:     "IMAP/SMTP 사용 허용 및 앱 비밀번호(2단계 인증)가 필요합니다.",
		SMTP:     &account.ServerConfig{Host: "smtp.daum.net", Port: 465, Encryption: "tls"},
		IMAP:     &account.ServerConfig{Host: "imap.daum.net", Port: 993, Encryption: "tls"},
	},
	{
		ID:       "gmail",
		Name:     "Gmail",
		Domains:  []string{"gmail.com", "googlemail.com"},
		HelpURL:  "https://support.google.com/accounts/answer/185833",
		Note:     "구글 계정 2단계 인증 후 앱 비밀번호가 필요합니다.",
		SMTP:     &account.ServerConfig{Host: "smtp.gmail.com", Port: 587, Encryption: "starttls"},
		IMAP:     &account.ServerConfig{Host: "imap.gmail.com", Port: 993, Encryption: "tls"},
	},
	{
		ID:       "outlook",
		Name:     "Outlook / Hotmail",
		Domains:  []string{"outlook.com", "hotmail.com", "live.com", "msn.com"},
		HelpURL:  "https://support.microsoft.com/account-billing/how-to-get-and-use-app-passwords-58b6469b-4a2d-8e0a-93d3-aa5c52e1dc5e",
		Note:     "2단계 인증 사용 시 앱 비밀번호가 필요합니다.",
		SMTP:     &account.ServerConfig{Host: "smtp.office365.com", Port: 587, Encryption: "starttls"},
		IMAP:     &account.ServerConfig{Host: "outlook.office365.com", Port: 993, Encryption: "tls"},
	},
	{
		ID:       "yahoo",
		Name:     "Yahoo Mail",
		Domains:  []string{"yahoo.com", "yahoo.co.jp", "yahoo.co.kr"},
		HelpURL:  "https://help.yahoo.com/kb/account/sln15241.html",
		Note:     "계정 보안에서 앱 비밀번호 생성이 필요합니다.",
		SMTP:     &account.ServerConfig{Host: "smtp.mail.yahoo.com", Port: 587, Encryption: "starttls"},
		IMAP:     &account.ServerConfig{Host: "imap.mail.yahoo.com", Port: 993, Encryption: "tls"},
	},
	{
		ID:       "icloud",
		Name:     "iCloud Mail",
		Domains:  []string{"icloud.com", "me.com", "mac.com"},
		HelpURL:  "https://support.apple.com/HT204397",
		Note:     "Apple ID 2단계 인증 후 앱 전용 비밀번호가 필요합니다.",
		SMTP:     &account.ServerConfig{Host: "smtp.mail.me.com", Port: 587, Encryption: "starttls"},
		IMAP:     &account.ServerConfig{Host: "imap.mail.me.com", Port: 993, Encryption: "tls"},
	},
}

// All은 사전 정의된 모든 제공자 목록을 반환한다.
// 프론트엔드에서 사용자가 제공자를 직접 선택할 때 사용한다.
func All() []Provider {
	out := make([]Provider, len(providers))
	copy(out, providers)
	return out
}

// LookupByDomain은 이메일 도메인으로 제공자를 찾는다.
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