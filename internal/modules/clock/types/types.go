// Package types는 시계 모듈의 데이터 모델을 정의한다.
package types

// PomodoroSettings는 뽀모도로 한 주기의 길이 설정이다.
type PomodoroSettings struct {
	// FocusMinutes는 집중 시간(분).
	FocusMinutes int `json:"focusMinutes"`

	// ShortBreakMinutes는 짧은 휴식 시간(분).
	ShortBreakMinutes int `json:"shortBreakMinutes"`

	// LongBreakMinutes는 긴 휴식 시간(분).
	LongBreakMinutes int `json:"longBreakMinutes"`

	// RoundsBeforeLongBreak는 긴 휴식까지의 집중 횟수.
	RoundsBeforeLongBreak int `json:"roundsBeforeLongBreak"`
}

// Settings는 앱을 다시 켜도 유지해야 하는 시계 모듈 설정이다.
// 스톱워치와 타이머의 진행 상태는 화면에서만 관리하므로 저장하지 않는다.
type Settings struct {
	// WorldClocks는 세계 시간에 표시할 IANA 시간대 이름 목록이다(예: "Asia/Tokyo").
	WorldClocks []string `json:"worldClocks"`

	// Pomodoro는 뽀모도로 길이 설정이다.
	Pomodoro PomodoroSettings `json:"pomodoro"`
}
