package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"working/internal/config"
	"working/internal/modules/calendar/account"
	"working/internal/modules/calendar/types"

	"github.com/zalando/go-keyring"
)

// accountsFile은 계정 메타데이터 파일명이다.
const accountsFile = "calendar_accounts.json"

// eventsFile은 로컬 일정 메타데이터 파일명이다.
// CalDAV 계정의 일정은 서버에 저장되므로 로컬 파일에는 보관하지 않고,
// 로컬 계정(SourceLocal)의 일정만 이 파일에 저장된다.
const eventsFile = "calendar_events.json"

// Store는 캘린더 계정 메타데이터, 자격증명, 로컬 일정을 관리한다.
// 계정 메타데이터와 로컬 일정은 사용자 데이터 디렉토리의 JSON 파일에
// 저장되고, 비밀번호/토큰은 OS 키체인에 저장된다.
type Store struct {
	// dataDir은 메타데이터 파일이 위치하는 디렉토리.
	dataDir string

	// keyringService는 키체인 서비스 식별자.
	keyringService string
}

// New는 기본 사용자 데이터 디렉토리를 사용하는 Store를 생성한다.
func New() (*Store, error) {
	dir, err := config.Dir()
	if err != nil {
		return nil, fmt.Errorf("사용자 데이터 디렉토리 조회 실패: %w", err)
	}
	return &Store{
		dataDir:        dir,
		keyringService: config.AppName + ".calendar",
	}, nil
}

// ListAccounts는 등록된 모든 계정을 ID순으로 반환한다.
// 비밀번호는 절대 포함되지 않는다.
func (s *Store) ListAccounts() ([]account.Account, error) {
	accs, err := s.loadAccounts()
	if err != nil {
		return nil, err
	}
	sort.Slice(accs, func(i, j int) bool { return accs[i].ID < accs[j].ID })
	return accs, nil
}

// GetAccount는 ID로 계정을 조회한다.
func (s *Store) GetAccount(id string) (*account.Account, error) {
	accs, err := s.loadAccounts()
	if err != nil {
		return nil, err
	}
	for i := range accs {
		if accs[i].ID == id {
			return &accs[i], nil
		}
	}
	return nil, fmt.Errorf("계정을 찾을 수 없습니다: %s", id)
}

// SaveAccount는 계정 메타데이터를 저장하고, credential이 비어있지 않으면
// 키체인에도 자격증명을 저장한다. credential이 빈 문자열이면
// 기존 키체인 항목을 그대로 유지한다.
func (s *Store) SaveAccount(acc *account.Account, credential string) error {
	accs, err := s.loadAccounts()
	if err != nil {
		return err
	}
	found := false
	for i := range accs {
		if accs[i].ID == acc.ID {
			accs[i] = *acc
			found = true
			break
		}
	}
	if !found {
		accs = append(accs, *acc)
	}

	if credential != "" {
		if err := keyring.Set(s.keyringService, acc.ID, credential); err != nil {
			return fmt.Errorf("키체인 저장 실패: %w", err)
		}
	}
	return s.saveAccounts(accs)
}

// DeleteAccount는 계정 메타데이터와 키체인 자격증명을 함께 삭제한다.
// 해당 계정의 로컬 일정도 함께 삭제한다.
func (s *Store) DeleteAccount(id string) error {
	accs, err := s.loadAccounts()
	if err != nil {
		return err
	}
	kept := accs[:0]
	for _, a := range accs {
		if a.ID != id {
			kept = append(kept, a)
		}
	}
	if err := s.saveAccounts(kept); err != nil {
		return err
	}

	// 로컬 일정 삭제
	events, err := s.loadEvents()
	if err != nil {
		return err
	}
	keptEvents := events[:0]
	for _, e := range events {
		if e.CalendarID != id {
			keptEvents = append(keptEvents, e)
		}
	}
	if err := s.saveEvents(keptEvents); err != nil {
		return err
	}

	// 키체인 항목 삭제: 존재하지 않아도 에러는 무시.
	_ = keyring.Delete(s.keyringService, id)
	return nil
}

// Credential은 계정의 자격증명(비밀번호/토큰)을 키체인에서 조회한다.
func (s *Store) Credential(id string) (string, error) {
	cred, err := keyring.Get(s.keyringService, id)
	if err != nil {
		if err == keyring.ErrNotFound {
			return "", fmt.Errorf("자격증명이 키체인에 없습니다: %s", id)
		}
		return "", fmt.Errorf("키체인 조회 실패: %w", err)
	}
	return cred, nil
}

// ListEvents는 로컬 계정에 속한 모든 일정을 반환한다.
// CalDAV 계정의 일정은 서버에 저장되므로 이 메서드에서 제외된다.
func (s *Store) ListEvents() ([]types.Event, error) {
	events, err := s.loadEvents()
	if err != nil {
		return nil, err
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Start < events[j].Start })
	return events, nil
}

// EventsByCalendar는 지정한 캘린더(계정) ID의 로컬 일정을 반환한다.
func (s *Store) EventsByCalendar(calendarID string) ([]types.Event, error) {
	events, err := s.loadEvents()
	if err != nil {
		return nil, err
	}
	var out []types.Event
	for _, e := range events {
		if e.CalendarID == calendarID {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out, nil
}

// SaveEvent는 로컬 일정을 저장한다(신규 또는 갱신).
func (s *Store) SaveEvent(event *types.Event) error {
	events, err := s.loadEvents()
	if err != nil {
		return err
	}
	found := false
	for i := range events {
		if events[i].UID == event.UID && events[i].CalendarID == event.CalendarID {
			events[i] = *event
			found = true
			break
		}
	}
	if !found {
		events = append(events, *event)
	}
	return s.saveEvents(events)
}

// DeleteEvent는 로컬 일정을 삭제한다.
func (s *Store) DeleteEvent(calendarID, uid string) error {
	events, err := s.loadEvents()
	if err != nil {
		return err
	}
	kept := events[:0]
	for _, e := range events {
		if e.CalendarID == calendarID && e.UID == uid {
			continue
		}
		kept = append(kept, e)
	}
	return s.saveEvents(kept)
}

// loadAccounts는 계정 메타데이터 파일을 읽어 계정 목록을 반환한다.
// 파일이 없으면 빈 목록을 반환한다.
func (s *Store) loadAccounts() ([]account.Account, error) {
	path := filepath.Join(s.dataDir, accountsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []account.Account{}, nil
		}
		return nil, fmt.Errorf("계정 파일 읽기 실패: %w", err)
	}
	var accs []account.Account
	if err := json.Unmarshal(data, &accs); err != nil {
		return nil, fmt.Errorf("계정 파일 파싱 실패: %w", err)
	}
	if accs == nil {
		accs = []account.Account{}
	}
	return accs, nil
}

// saveAccounts는 계정 목록을 메타데이터 파일에 쓴다.
func (s *Store) saveAccounts(accs []account.Account) error {
	path := filepath.Join(s.dataDir, accountsFile)
	data, err := json.MarshalIndent(accs, "", "  ")
	if err != nil {
		return fmt.Errorf("계정 직렬화 실패: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("계정 파일 쓰기 실패: %w", err)
	}
	return nil
}

// loadEvents는 로컬 일정 파일을 읽어 일정 목록을 반환한다.
// 파일이 없으면 빈 목록을 반환한다.
func (s *Store) loadEvents() ([]types.Event, error) {
	path := filepath.Join(s.dataDir, eventsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []types.Event{}, nil
		}
		return nil, fmt.Errorf("일정 파일 읽기 실패: %w", err)
	}
	var events []types.Event
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, fmt.Errorf("일정 파일 파싱 실패: %w", err)
	}
	if events == nil {
		events = []types.Event{}
	}
	return events, nil
}

// saveEvents는 로컬 일정 목록을 파일에 쓴다.
func (s *Store) saveEvents(events []types.Event) error {
	path := filepath.Join(s.dataDir, eventsFile)
	data, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		return fmt.Errorf("일정 직렬화 실패: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("일정 파일 쓰기 실패: %w", err)
	}
	return nil
}
