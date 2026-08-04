// Package clock 은 "working" 앱의 시계 모듈 진입점이다.
//
// 현재 시간·세계 시간·스톱워치·타이머·뽀모도로를 제공한다. 시간 계산과
// 진행 상태는 화면에서 다루고, 이 모듈은 앱을 다시 켜도 유지해야 하는
// 설정(세계 시간 목록, 뽀모도로 길이)만 저장한다.
// 다른 모듈과 마찬가지로 main.go에서 Service를 등록하면 앱에 포함된다.
package clock

import (
	"strings"

	"working/internal/modules/clock/store"
	"working/internal/modules/clock/types"
)

// 뽀모도로 길이의 허용 범위. 화면에서 잘못된 값이 들어와도 여기서 잡는다.
const (
	minMinutes = 1
	maxMinutes = 180
	minRounds  = 1
	maxRounds  = 12
)

// maxWorldClocks는 세계 시간에 담을 수 있는 최대 시간대 수이다.
const maxWorldClocks = 20

// Service는 프론트엔드에 바인딩되는 시계 모듈 서비스이다.
type Service struct {
	store *store.Store
}

// NewService는 시계 모듈 Service를 생성한다.
func NewService() (*Service, error) {
	st, err := store.New()
	if err != nil {
		return nil, err
	}
	return &Service{store: st}, nil
}

// ServiceShutdown은 Wails 종료 시 호출되는 훅이다.
func (s *Service) ServiceShutdown() {}

// Settings는 저장된 설정을 반환한다. 저장된 값이 없으면 기본값을 준다.
func (s *Service) Settings() (*types.Settings, error) {
	settings, found, err := s.store.Load()
	if err != nil {
		return nil, err
	}
	if !found {
		defaults := DefaultSettings()
		return &defaults, nil
	}
	normalized := Normalize(settings)
	return &normalized, nil
}

// SaveSettings는 설정을 검증해 저장하고, 실제로 저장된 값을 반환한다.
func (s *Service) SaveSettings(settings *types.Settings) (*types.Settings, error) {
	if settings == nil {
		defaults := DefaultSettings()
		settings = &defaults
	}
	normalized := Normalize(*settings)
	if err := s.store.Save(normalized); err != nil {
		return nil, err
	}
	return &normalized, nil
}

// DefaultSettings는 처음 실행했을 때 보여 줄 기본 설정이다.
func DefaultSettings() types.Settings {
	return types.Settings{
		WorldClocks: []string{"America/New_York", "Europe/London", "Asia/Tokyo"},
		Pomodoro: types.PomodoroSettings{
			FocusMinutes:          25,
			ShortBreakMinutes:     5,
			LongBreakMinutes:      15,
			RoundsBeforeLongBreak: 4,
		},
	}
}

// Normalize는 설정값을 허용 범위로 맞추고 시간대 목록을 정리한다.
// 빈 값이나 범위를 벗어난 값은 기본값으로 되돌려, 잘못된 설정이 저장되지 않게 한다.
func Normalize(settings types.Settings) types.Settings {
	defaults := DefaultSettings()

	zones := make([]string, 0, len(settings.WorldClocks))
	seen := make(map[string]bool, len(settings.WorldClocks))
	for _, zone := range settings.WorldClocks {
		zone = strings.TrimSpace(zone)
		if zone == "" || seen[zone] || len(zones) >= maxWorldClocks {
			continue
		}
		seen[zone] = true
		zones = append(zones, zone)
	}

	return types.Settings{
		WorldClocks: zones,
		Pomodoro: types.PomodoroSettings{
			FocusMinutes:          clamp(settings.Pomodoro.FocusMinutes, minMinutes, maxMinutes, defaults.Pomodoro.FocusMinutes),
			ShortBreakMinutes:     clamp(settings.Pomodoro.ShortBreakMinutes, minMinutes, maxMinutes, defaults.Pomodoro.ShortBreakMinutes),
			LongBreakMinutes:      clamp(settings.Pomodoro.LongBreakMinutes, minMinutes, maxMinutes, defaults.Pomodoro.LongBreakMinutes),
			RoundsBeforeLongBreak: clamp(settings.Pomodoro.RoundsBeforeLongBreak, minRounds, maxRounds, defaults.Pomodoro.RoundsBeforeLongBreak),
		},
	}
}

// clamp는 값을 범위 안으로 맞춘다. 0 이하이면 설정하지 않은 것으로 보고 기본값을 쓴다.
func clamp(value, min, max, fallback int) int {
	if value <= 0 {
		return fallback
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
