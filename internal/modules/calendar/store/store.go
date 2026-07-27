package store

import (
	"database/sql"
	"fmt"
	"sort"

	"github.com/zalando/go-keyring"
	"working/internal/config"
	"working/internal/modules/calendar/account"
	"working/internal/modules/calendar/types"
	"working/internal/storage"
)

const accountsKey = "calendar.accounts"
const eventsKey = "calendar.events"

type Store struct {
	db             *sql.DB
	keyringService string
}

// New는 앱 전체가 공유하는 SQLite 저장소를 사용한다.
func New() (*Store, error) {
	db, err := storage.Open()
	if err != nil {
		return nil, err
	}
	return &Store{db: db, keyringService: config.AppName + ".calendar"}, nil
}

func (s *Store) loadAccounts() ([]account.Account, error) {
	var out []account.Account
	found, err := storage.GetJSON(s.db, accountsKey, &out)
	if err != nil {
		return nil, err
	}
	if !found {
		// 기존 설치의 JSON 계정을 첫 실행 때 SQLite로 이전한다.
		var legacy []account.Account
		if ok, e := storage.LegacyJSON(mustConfigDir(), "calendar_accounts.json", &legacy); e != nil {
			return nil, fmt.Errorf("계정 마이그레이션 실패: %w", e)
		} else if ok {
			out = legacy
			if err := s.saveAccounts(out); err != nil {
				return nil, err
			}
		}
	}
	if out == nil {
		out = []account.Account{}
	}
	return out, nil
}

func mustConfigDir() string                             { dir, _ := config.Dir(); return dir }
func (s *Store) saveAccounts(v []account.Account) error { return storage.PutJSON(s.db, accountsKey, v) }

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

func (s *Store) ListAccounts() ([]account.Account, error) {
	v, e := s.loadAccounts()
	sort.Slice(v, func(i, j int) bool { return v[i].ID < v[j].ID })
	return v, e
}
func (s *Store) GetAccount(id string) (*account.Account, error) {
	v, e := s.loadAccounts()
	if e != nil {
		return nil, e
	}
	for i := range v {
		if v[i].ID == id {
			return &v[i], nil
		}
	}
	return nil, fmt.Errorf("계정을 찾을 수 없습니다: %s", id)
}
func (s *Store) SaveAccount(a *account.Account, credential string) error {
	v, e := s.loadAccounts()
	if e != nil {
		return e
	}
	found := false
	for i := range v {
		if v[i].ID == a.ID {
			v[i] = *a
			found = true
		}
	}
	if !found {
		v = append(v, *a)
	}
	if credential != "" {
		if e := keyring.Set(s.keyringService, a.ID, credential); e != nil {
			return fmt.Errorf("키체인 저장 실패: %w", e)
		}
	}
	return s.saveAccounts(v)
}
func (s *Store) DeleteAccount(id string) error {
	v, e := s.loadAccounts()
	if e != nil {
		return e
	}
	kept := v[:0]
	for _, a := range v {
		if a.ID != id {
			kept = append(kept, a)
		}
	}
	if e = s.saveAccounts(kept); e != nil {
		return e
	}
	ev, e := s.loadEvents()
	if e != nil {
		return e
	}
	keptEv := ev[:0]
	for _, x := range ev {
		if x.CalendarID != id {
			keptEv = append(keptEv, x)
		}
	}
	if e = s.saveEvents(keptEv); e != nil {
		return e
	}
	_ = keyring.Delete(s.keyringService, id)
	return nil
}
func (s *Store) Credential(id string) (string, error) {
	v, e := keyring.Get(s.keyringService, id)
	if e == keyring.ErrNotFound {
		return "", fmt.Errorf("자격증명이 키체인에 없습니다: %s", id)
	}
	if e != nil {
		return "", fmt.Errorf("키체인 조회 실패: %w", e)
	}
	return v, nil
}
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
