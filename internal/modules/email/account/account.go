package account

// Protocol은 메일 프로토콜 구분값이다.
type Protocol string

const (
	// ProtocolSMTP는 발송용 SMTP를 의미한다.
	ProtocolSMTP Protocol = "smtp"
	// ProtocolIMAP는 수신용 IMAP를 의미한다.
	ProtocolIMAP Protocol = "imap"

	// AuthPassword는 서버 비밀번호 또는 앱 비밀번호 인증이다.
	AuthPassword = "password"
	// AuthOAuth2는 Google OAuth access/refresh token 인증이다.
	AuthOAuth2 = "oauth2"
)

// ServerConfig는 단일 프로토콜의 서버 연결 정보를 나타낸다.
type ServerConfig struct {
	// Host는 서버 호스트명.
	Host string `json:"host"`

	// Port는 서버 포트.
	Port int `json:"port"`

	// Encryption은 암호화 방식 ("none", "starttls", "tls").
	Encryption string `json:"encryption"`
}

// Account는 등록된 이메일 계정을 나타낸다.
// 비밀번호/토큰은 이 구조체에 저장되지 않으며 OS 키체인에 별도 보관된다.
type Account struct {
	// ID는 계정 식별자(자동 생성). 변경되지 않는 안정적 키.
	ID string `json:"id"`

	// Name은 사용자 지정 계정 표시 이름(예: "회사 메일").
	Name string `json:"name"`

	// Email은 계정 이메일 주소이자 로그인 아이디.
	Email string `json:"email"`

	// DisplayName은 발송 메일 From 표시에 사용할 이름.
	DisplayName string `json:"displayName,omitempty"`

	// SMTP는 발송 서버 설정. 빈 값이면 발송 기능 비활성화.
	SMTP *ServerConfig `json:"smtp,omitempty"`

	// IMAP은 수신 서버 설정. 빈 값이면 수신 기능 비활성화.
	IMAP *ServerConfig `json:"imap,omitempty"`

	// AuthType은 인증 방식 ("password" 또는 "oauth2").
	// 현재 첫 버전은 password만 지원한다.
	AuthType string `json:"authType,omitempty"`
}
