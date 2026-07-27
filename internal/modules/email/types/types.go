package types

// Message는 단일 이메일 메시지를 나타낸다.
// IMAP 수신 결과와 SMTP 전송 입력 양쪽에서 재사용된다.
type Message struct {
	// ID는 Gmail 등 외부 메일 서비스가 제공하는 원격 메시지 식별자이다.
	// IMAP 메시지는 UID를 사용하므로 이 값이 비어 있을 수 있다.
	ID string `json:"id,omitempty"`

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

	// HTML은 수신 메일의 HTML 본문이다.
	// 프론트엔드에서 허용된 태그만 필터링한 뒤 렌더링한다.
	HTML string `json:"html,omitempty"`

	// Raw는 수신한 이메일의 원문 MIME 데이터이다.
	// 헤더와 multipart 경계를 포함하며, 원문 보기와 복사에 사용한다.
	Raw string `json:"raw,omitempty"`

	// Date는 RFC3339 포맷의 메일 날짜(수신 시).
	Date string `json:"date,omitempty"`

	// Unread는 읽지 않은 메일 여부(수신 시).
	Unread bool `json:"unread,omitempty"`

	// Favorite는 메일 서버에 저장된 즐겨찾기(별표) 여부이다.
	Favorite bool `json:"favorite,omitempty"`

	// Attachments는 첨부파일 목록(전송 시 로컬 파일 경로, 수신 시 파일명만).
	Attachments []Attachment `json:"attachments,omitempty"`
}

// MessagePage는 이메일 목록의 한 페이지와 다음 페이지 커서이다.
// 커서가 비어 있으면 현재 폴더의 모든 메시지를 불러온 것이다.
type MessagePage struct {
	Messages      []Message `json:"messages"`
	NextPageToken string    `json:"nextPageToken,omitempty"`
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
