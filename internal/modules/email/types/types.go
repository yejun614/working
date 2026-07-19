package types

// Message는 단일 이메일 메시지를 나타낸다.
// IMAP 수신 결과와 SMTP 전송 입력 양쪽에서 재사용된다.
type Message struct {
	// UID는 IMAP 메시지 식별자(수신 시에만 유효).
	UID uint32 `json:"uid,omitempty"`

	// From은 발신자 주소.
	From string `json:"from"`

	// To는 수신자 주소 목록(콤마로 조인된 문자열).
	To string `json:"to"`

	// Cc는 참조 주소 목록(콤마로 조인된 문자열).
	Cc string `json:"cc,omitempty"`

	// Subject는 메일 제목.
	Subject string `json:"subject"`

	// Body는 메일 본문(plaintext).
	Body string `json:"body"`

	// Date는 RFC3339 포맷의 메일 날짜(수신 시).
	Date string `json:"date,omitempty"`

	// Unread는 읽지 않은 메일 여부(수신 시).
	Unread bool `json:"unread,omitempty"`

	// Attachments는 첨부파일 목록(전송 시 로컬 파일 경로, 수신 시 파일명만).
	Attachments []Attachment `json:"attachments,omitempty"`
}

// Attachment는 이메일 첨부파일을 나타낸다.
type Attachment struct {
	// Name은 파일명.
	Name string `json:"name"`

	// Path는 전송 시 로컬 파일 경로.
	Path string `json:"path,omitempty"`

	// Size는 바이트 단위 크기(수신 시).
	Size int `json:"size,omitempty"`
}