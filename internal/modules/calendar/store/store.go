package store

import (
	"database/sql"
	"fmt"
	"sort"

	"working/internal/config"
	"working/internal/modules/calendar/types"
	"working/internal/storage"
)

const eventsKey = "calendar.events"

// Store는 캘린더 모듈의 일정 캐시를 보관한다.
// 계정과 키체인 자격증명은 통합 계정 모듈(internal/modules/account)이 관리한다.
type Store struct {
	db *sql.DB
}

// New는 앱 전체가 공유하는 SQLite 저장소를 사용한다.
func New() (*Store, error) {
	db, err := storage.Open()
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func mustConfigDir() string { dir, _ := config.Dir(); return dir }

func (s *Store) loadEvents() ([]types.Event, error) {
	var out []types.Event
	found, err := storage.GetJSON(s.db, eventsKey, &out)
	if err != nil {
		return nil, err
	}
	if !found {
		var legacy []types.Event
		if ok, e := storage.LegacyJSON(mustConfigDir(), "calendar_events.json", &legacy); e != nil {
			return nil, fmt.Errorf("일정 마이그레이션 실패: %w", e)
		} else if ok {
			out = legacy
			if err := s.saveEvents(out); err != nil {
				return nil, err
			}
		}
	}
	if out == nil {
		out = []types.Event{}
	}
	return out, nil
}
func (s *Store) saveEvents(v []types.Event) error { return storage.PutJSON(s.db, eventsKey, v) }

func (s *Store) ListEvents() ([]types.Event, error) {
	v, e := s.loadEvents()
	sort.Slice(v, func(i, j int) bool { return v[i].Start < v[j].Start })
	return v, e
}
func (s *Store) EventsByCalendar(id string) ([]types.Event, error) {
	v, e := s.loadEvents()
	if e != nil {
		return nil, e
	}
	out := make([]types.Event, 0)
	for _, x := range v {
		if x.CalendarID == id {
			out = append(out, x)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out, nil
}

// DeleteCalendarEvents는 계정이 삭제될 때 해당 계정의 일정 캐시를 제거한다.
func (s *Store) DeleteCalendarEvents(calendarID string) error {
	return s.ReplaceEvents(calendarID, nil)
}

func (s *Store) SaveEvent(x *types.Event) error {
	v, e := s.loadEvents()
	if e != nil {
		return e
	}
	found := false
	for i := range v {
		if v[i].UID == x.UID && v[i].CalendarID == x.CalendarID {
			v[i] = *x
			found = true
		}
	}
	if !found {
		v = append(v, *x)
	}
	return s.saveEvents(v)
}
func (s *Store) DeleteEvent(cal, uid string) error {
	v, e := s.loadEvents()
	if e != nil {
		return e
	}
	kept := v[:0]
	for _, x := range v {
		if !(x.CalendarID == cal && x.UID == uid) {
			kept = append(kept, x)
		}
	}
	return s.saveEvents(kept)
}

// ReplaceEvents는 외부 캘린더의 최신 스냅샷을 SQLite 캐시에 반영한다.
func (s *Store) ReplaceEvents(calendarID string, events []types.Event) error {
	v, e := s.loadEvents()
	if e != nil {
		return e
	}
	kept := v[:0]
	for _, x := range v {
		if x.CalendarID != calendarID {
			kept = append(kept, x)
		}
	}
	v = append(kept, events...)
	return s.saveEvents(v)
}
