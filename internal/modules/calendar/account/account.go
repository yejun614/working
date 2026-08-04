package account

// AuthType은 캘린더 계정의 인증 방식 구분값이다.
type AuthType string

const (
	// AuthBasic은 사용자 이름 + 비밀번호 기반 인증.
	AuthBasic AuthType = "basic"

	// AuthOAuth2는 OAuth2 access/refresh token 기반 인증.
	AuthOAuth2 AuthType = "oauth2"
)

// Source는 계정의 일정 저장소 종류를 나타낸다.
type Source string

const (
	// SourceLocal은 로컬 전용 계정. 사용자 데이터 디렉토리의 JSON 파일에 저장.
	SourceLocal Source = "local"

	// SourceCalDAV는 외부 CalDAV 서버 연동 계정.
	SourceCalDAV Source = "caldav"
)

// Account는 등록된 캘린더 계정을 나타낸다.
// 비밀번호/토큰은 이 구조체에 저장되지 않으며 OS 키체인에 별도 보관된다.
type Account struct {
	// ID는 계정 식별자(자동 생성). 변경되지 않는 안정적 키.
	ID string `json:"id"`

	// Name은 사용자 지정 계정 표시 이름(예: "회사 캘린더").
	Name string `json:"name"`

	// Source는 일정 저장소 종류 ("local" 또는 "caldav").
	Source Source `json:"source"`

	// Color는 캘린더 표시 색상(HEX). 일정 구분에 사용.
	Color string `json:"color,omitempty"`

	// CalDAVURL은 CalDAV 서버 캘린더 홈 URL.
	// Source가 "caldav"일 때만 사용된다.
	CalDAVURL string `json:"caldavUrl,omitempty"`

	// Username은 CalDAV 로그인 사용자 이름.
	// Source가 "caldav"일 때만 사용된다.
	Username string `json:"username,omitempty"`

	// AuthType은 인증 방식 ("basic" 또는 "oauth2").
	AuthType AuthType `json:"authType,omitempty"`

	// SyncEnabled는 외부 캘린더 동기화 활성화 여부.
	// Source가 "caldav"일 때만 의미가 있다.
	SyncEnabled bool `json:"syncEnabled,omitempty"`

	// LastSyncAt은 마지막 동기화 시각(RFC3339).
	LastSyncAt string `json:"lastSyncAt,omitempty"`

	// AuthError는 재인증이 필요한 마지막 인증 실패 사유이다.
	// OAuth 토큰이 만료·철회되면 기록되고, 재인증이나 정상 호출 성공 시 비워진다.
	// 값이 비어 있지 않으면 프론트엔드가 재인증 안내와 버튼을 표시한다.
	AuthError string `json:"authError,omitempty"`
}
