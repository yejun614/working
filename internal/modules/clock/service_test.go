package clock

import (
	"reflect"
	"testing"

	"working/internal/modules/clock/types"
)

func TestNormalizeWorldClocks(t *testing.T) {
	tests := []struct {
		name  string
		zones []string
		want  []string
	}{
		{name: "빈 목록은 그대로", zones: nil, want: []string{}},
		{name: "앞뒤 공백 제거", zones: []string{"  Asia/Seoul  "}, want: []string{"Asia/Seoul"}},
		{name: "빈 문자열 제외", zones: []string{"Asia/Seoul", "", "   "}, want: []string{"Asia/Seoul"}},
		{
			name:  "중복 제거하고 순서 유지",
			zones: []string{"Asia/Tokyo", "Europe/London", "Asia/Tokyo"},
			want:  []string{"Asia/Tokyo", "Europe/London"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Normalize(types.Settings{WorldClocks: tt.zones}).WorldClocks
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("WorldClocks = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestNormalizeWorldClocksCapsCount(t *testing.T) {
	zones := make([]string, 0, maxWorldClocks+5)
	for i := 0; i < maxWorldClocks+5; i++ {
		zones = append(zones, string(rune('A'+i))+"/Zone")
	}

	got := Normalize(types.Settings{WorldClocks: zones}).WorldClocks

	if len(got) != maxWorldClocks {
		t.Fatalf("시간대 수 = %d, want %d", len(got), maxWorldClocks)
	}
}

func TestNormalizePomodoro(t *testing.T) {
	defaults := DefaultSettings().Pomodoro

	tests := []struct {
		name  string
		given types.PomodoroSettings
		want  types.PomodoroSettings
	}{
		{
			name:  "값이 없으면 기본값",
			given: types.PomodoroSettings{},
			want:  defaults,
		},
		{
			name:  "음수도 기본값",
			given: types.PomodoroSettings{FocusMinutes: -5, ShortBreakMinutes: -1, LongBreakMinutes: -1, RoundsBeforeLongBreak: -1},
			want:  defaults,
		},
		{
			name:  "정상 값은 그대로",
			given: types.PomodoroSettings{FocusMinutes: 50, ShortBreakMinutes: 10, LongBreakMinutes: 20, RoundsBeforeLongBreak: 3},
			want:  types.PomodoroSettings{FocusMinutes: 50, ShortBreakMinutes: 10, LongBreakMinutes: 20, RoundsBeforeLongBreak: 3},
		},
		{
			name:  "너무 큰 값은 상한으로",
			given: types.PomodoroSettings{FocusMinutes: 9999, ShortBreakMinutes: 9999, LongBreakMinutes: 9999, RoundsBeforeLongBreak: 99},
			want:  types.PomodoroSettings{FocusMinutes: maxMinutes, ShortBreakMinutes: maxMinutes, LongBreakMinutes: maxMinutes, RoundsBeforeLongBreak: maxRounds},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Normalize(types.Settings{Pomodoro: tt.given}).Pomodoro
			if got != tt.want {
				t.Fatalf("Pomodoro = %+v, want %+v", got, tt.want)
			}
		})
	}
}
