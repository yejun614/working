// Package types는 메일과 캘린더가 공유하는 통합 계정 모델을 정의한다.
//
// 이전에는 이메일 모듈과 캘린더 모듈이 각각 자기 계정 목록과 키체인 자격증명을
// 따로 관리했다. 같은 Google 계정을 두 번 등록하고 OAuth 인증도 두 번 받아야 했고,
// 한쪽 토큰만 만료되는 문제가 있었다. 이 패키지의 Account 하나가 두 모듈의
// 계정을 대신하며, 사용할 기능만 Mail/Calendar 설정으로 켠다.
package types

// AuthType은 계정 인증 방식이다.
type AuthType string

const (
	// AuthPassword는 비밀번호 또는 앱 비밀번호 인증이다.
	AuthPassword AuthType = "password"

	// AuthOAuth2는 Google OAuth access/refresh token 인증이다.
	AuthOAuth2 AuthType = "oauth2"
)

// CalendarSource는 일정 저장소 종류이다.
type CalendarSource string

const (
	// CalendarSourceLocal은 외부 서버 없이 앱에만 저장하는 캘린더이다.
	CalendarSourceLocal CalendarSource = "local"

	// CalendarSourceCalDAV는 외부 CalDAV 서버 연동 캘린더이다.
	CalendarSourceCalDAV CalendarSource = "caldav"
)

// ServerConfig는 단일 프로토콜의 서버 연결 정보이다.
type ServerConfig struct {
	// Host는 서버 호스트명.
	Host string `json:"host"`

	// Port는 서버 포트.
	Port int `json:"port"`

	// Encryption은 암호화 방식 ("none", "starttls", "tls").
	Encryption string `json:"encryption"`
}

// MailConfig는 계정의 메일 기능 설정이다.
// nil이면 이 계정은 이메일 모듈에서 사용되지 않는다.
type MailConfig struct {
	// SMTP는 발송 서버 설정. 비어 있으면 발송 기능 비활성화.
	SMTP *ServerConfig `json:"smtp,omitempty"`

	// IMAP은 수신 서버 설정. 비어 있으면 수신 기능 비활성화.
	// OAuth 계정은 Gmail API를 사용하므로 비어 있어도 된다.
	IMAP *ServerConfig `json:"imap,omitempty"`
}

// CalendarConfig는 계정의 캘린더 기능 설정이다.
// nil이면 이 계정은 캘린더 모듈에서 사용되지 않는다.
type CalendarConfig struct {
	// Source는 일정 저장소 종류 ("local" 또는 "caldav").
	Source CalendarSource `json:"source"`

	// URL은 CalDAV 서버 캘린더 홈 URL. Source가 "caldav"일 때만 사용된다.
	URL string `json:"url,omitempty"`

	// Username은 CalDAV 로그인 사용자 이름.
	// 비어 있으면 계정의 Email을 사용한다.
	Username string `json:"username,omitempty"`

	// Color는 캘린더 표시 색상(HEX).
	Color string `json:"color,omitempty"`

	// SyncEnabled는 외부 캘린더 동기화 활성화 여부.
	SyncEnabled bool `json:"syncEnabled,omitempty"`

	// LastSyncAt은 마지막 동기화 시각(RFC3339).
	LastSyncAt string `json:"lastSyncAt,omitempty"`
}

// Account는 메일과 캘린더가 공유하는 단일 계정이다.
// 비밀번호와 OAuth 토큰은 이 구조체에 담기지 않으며 OS 키체인에 별도 보관된다.
type Account struct {
	// ID는 계정 식별자(자동 생성). 메일 캐시와 일정 캐시의 키로도 쓰이므로 변경되지 않는다.
	ID string `json:"id"`

	// Name은 사용자 지정 표시 이름(예: "회사 계정").
	Name string `json:"name"`

	// Email은 계정 이메일 주소이자 로그인 아이디.
	Email string `json:"email,omitempty"`

	// DisplayName은 발송 메일 From 표시에 사용할 이름.
	DisplayName string `json:"displayName,omitempty"`

	// AuthType은 인증 방식 ("password" 또는 "oauth2").
	// OAuth 계정은 토큰 하나로 Gmail과 Google 캘린더를 함께 사용한다.
	AuthType AuthType `json:"authType,omitempty"`

	// Mail은 메일 기능 설정. nil이면 메일 모듈에 노출되지 않는다.
	Mail *MailConfig `json:"mail,omitempty"`

	// Calendar는 캘린더 기능 설정. nil이면 캘린더 모듈에 노출되지 않는다.
	Calendar *CalendarConfig `json:"calendar,omitempty"`

	// AuthError는 재인증이 필요한 마지막 인증 실패 사유이다.
	// 값이 비어 있지 않으면 프론트엔드가 재인증 안내와 버튼을 표시한다.
	AuthError string `json:"authError,omitempty"`
}

// UsesMail은 이 계정이 메일 모듈에서 사용되는지 여부이다.
func (a Account) UsesMail() bool { return a.Mail != nil }

// UsesCalendar는 이 계정이 캘린더 모듈에서 사용되는지 여부이다.
func (a Account) UsesCalendar() bool { return a.Calendar != nil }

// CalendarUsername은 CalDAV 로그인에 사용할 사용자 이름을 반환한다.
// 캘린더 설정에 값이 없으면 계정 이메일로 대체한다.
func (a Account) CalendarUsername() string {
	if a.Calendar != nil && a.Calendar.Username != "" {
		return a.Calendar.Username
	}
	return a.Email
}
