package types

// Event는 단일 캘린더 일정을 나타낸다.
// 로컬 저장소와 외부 CalDAV 서버 양쪽에서 재사용된다.
type Event struct {
	// UID는 일정 식별자. CalDAV 서버에서는 VEVENT UID와 매핑되고,
	// 로컬 일정은 자동 생성된 고유 문자열을 사용한다.
	UID string `json:"uid"`

	// CalendarID는 이 일정이 속한 캘린더(계정)의 ID.
	CalendarID string `json:"calendarId"`

	// CalendarHref는 외부 CalDAV 계정 안에서 일정이 속한 캘린더의 경로.
	// 로컬 일정과 칸반 일정은 비어 있다.
	CalendarHref string `json:"calendarHref,omitempty"`

	// Title은 일정 제목(요약).
	Title string `json:"title"`

	// Description은 상세 설명.
	Description string `json:"description,omitempty"`

	// Location은 장소.
	Location string `json:"location,omitempty"`

	// Start은 일정 시작 시각(RFC3339).
	Start string `json:"start"`

	// End은 일정 종료 시각(RFC3339).
	End string `json:"end"`

	// AllDay가 true면 종일 일정. Start/End는 날짜만 의미한다.
	AllDay bool `json:"allDay,omitempty"`

	// RecurrenceRule은 반복 규칙(RFC 5545 RRULE 문자열, 예: "FREQ=WEEKLY;BYDAY=MO").
	RecurrenceRule string `json:"recurrenceRule,omitempty"`

	// Attendees는 참석자 이메일 목록.
	Attendees []string `json:"attendees,omitempty"`

	// Organizer는 주최자 이메일.
	Organizer string `json:"organizer,omitempty"`

	// ETag는 CalDAV 서버에서 사용하는 동시성 제어용 태그.
	// 로컬 일정은 비어 있다.
	ETag string `json:"etag,omitempty"`

	// Href는 CalDAV 서버 상의 객체 경로.
	// 로컬 일정은 비어 있다.
	Href string `json:"href,omitempty"`
}

// CalendarInfo는 캘린더(폴더) 메타데이터를 나타낸다.
// CalDAV 서버의 calendar-home-set 하위 캘린더와
// 로컬 기본 캘린더 모두 이 형태로 표현된다.
type CalendarInfo struct {
	// Href는 CalDAV 서버 상의 캘린더 경로.
	Href string `json:"href,omitempty"`

	// Name은 캘린더 표시 이름.
	Name string `json:"name"`

	// Color는 캘린더 색상(HEX, 예: "#4f7cff").
	Color string `json:"color,omitempty"`

	// Description은 캘린더 설명.
	Description string `json:"description,omitempty"`
}
