// Package calendar 은 "working" 앱의 캘린더 모듈 진입점이다.
//
// 이 모듈은 로컬 일정 관리와 외부 CalDAV 서버 연동 기능을
// Wails Service 형태로 프론트엔드에 노출한다.
// 다른 모듈과 마찬가지로 internal/modules/<이름> 패키지로 격리되며,
// main.go에서 원하는 모듈의 Service만 application.NewService로 등록하면
// 해당 모듈만 앱에 포함된다.
package calendar

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"working/internal/modules/calendar/account"
	"working/internal/modules/calendar/caldav"
	"working/internal/modules/calendar/provider"
	"working/internal/modules/calendar/store"
	"working/internal/modules/calendar/types"
	kanbanstore "working/internal/modules/kanban/store"
)

// Service는 프론트엔드에 바인딩되는 캘린더 모듈 서비스이다.
// 모든 메서드는 Wails를 통해 JS/TS에서 호출 가능하다.
type Service struct {
	store *store.Store
}

// NewService는 캘린더 모듈 Service를 생성한다.
// 내부 저장소 초기화 실패 시 에러를 반환한다.
func NewService() (*Service, error) {
	st, err := store.New()
	if err != nil {
		return nil, err
	}
	return &Service{store: st}, nil
}

// ServiceShutdown은 Wails 종료 시 호출되는 훅이다.
// 현재는 유지할 상태가 없으므로 아무 작업도 하지 않는다.
func (s *Service) ServiceShutdown() {}

// ProviderList는 사전 정의된 캘린더 서비스 제공자 목록을 반환한다.
// 프론트엔드에서 계정 추가 시 드롭다운으로 표시하거나,
// 계정 도메인 자동 인식에 사용한다.
func (s *Service) ProviderList() []provider.Provider {
	return provider.All()
}

// ProviderLookupByEmail은 이메일 주소의 도메인으로 제공자를 찾는다.
// 일치하는 사전 정의 제공자가 없으면 nil을 반환한다.
// 프론트엔드에서 사용자 이름 입력 중 CalDAV URL 자동 채우기에 사용한다.
func (s *Service) ProviderLookupByEmail(email string) *provider.Provider {
	return provider.LookupByEmail(email)
}

// newCalDAVClient는 계정 자격증명을 키체인에서 읽어 CalDAV 클라이언트를 만든다.
// OAuth access token이 갱신되면 동일 계정의 키체인 credential도 갱신한다.
func (s *Service) newCalDAVClient(acc *account.Account) (*caldav.Client, error) {
	credential, err := s.store.Credential(acc.ID)
	if err != nil {
		return nil, err
	}
	return caldav.NewClient(acc, credential, func(updated string) error {
		return s.store.SaveAccount(acc, updated)
	})
}

// AccountList는 등록된 모든 캘린더 계정을 반환한다(자격증명 제외).
func (s *Service) AccountList() ([]account.Account, error) {
	return s.store.ListAccounts()
}

// AccountGet은 ID로 계정을 조회한다.
func (s *Service) AccountGet(id string) (*account.Account, error) {
	return s.store.GetAccount(id)
}

// AccountCreate는 새 계정을 등록하고 자격증명을 키체인에 저장한다.
// id는 자동 생성되어 반환된다. CalDAV 계정은 credential(비밀번호/토큰)이 필수.
// 로컬 계정(SourceLocal)은 credential을 사용하지 않는다.
func (s *Service) AccountCreate(acc *account.Account, credential string) (string, error) {
	if acc == nil {
		return "", fmt.Errorf("계정 정보가 없습니다")
	}
	if strings.TrimSpace(acc.Name) == "" {
		return "", fmt.Errorf("계정 이름은 필수입니다")
	}
	if acc.Source == "" {
		acc.Source = account.SourceLocal
	}
	if acc.Source == account.SourceCalDAV {
		if strings.TrimSpace(acc.CalDAVURL) == "" {
			return "", fmt.Errorf("CalDAV 서버 URL은 필수입니다")
		}
		if strings.TrimSpace(acc.Username) == "" {
			return "", fmt.Errorf("사용자 이름은 필수입니다")
		}
		if credential == "" {
			return "", fmt.Errorf("비밀번호/토큰은 필수입니다")
		}
	}
	acc.ID = newID()
	if acc.AuthType == "" {
		acc.AuthType = account.AuthBasic
	}
	if err := s.store.SaveAccount(acc, credential); err != nil {
		return "", err
	}
	return acc.ID, nil
}

// AccountUpdate는 기존 계정 메타데이터를 갱신한다.
// credential이 빈 문자열이 아니면 키체인 자격증명도 함께 갱신하고,
// 빈 문자열이면 기존 자격증명을 유지한다.
func (s *Service) AccountUpdate(acc *account.Account, credential string) error {
	if acc == nil || acc.ID == "" {
		return fmt.Errorf("계정 ID가 필요합니다")
	}
	existing, err := s.store.GetAccount(acc.ID)
	if err != nil {
		return err
	}
	if acc.Name == "" {
		acc.Name = existing.Name
	}
	if acc.Source == "" {
		acc.Source = existing.Source
	}
	if acc.AuthType == "" {
		acc.AuthType = existing.AuthType
	}
	return s.store.SaveAccount(acc, credential)
}

// AccountDelete는 계정과 키체인 자격증명, 로컬 일정을 함께 삭제한다.
func (s *Service) AccountDelete(id string) error {
	if id == "" {
		return fmt.Errorf("계정 ID가 필요합니다")
	}
	return s.store.DeleteAccount(id)
}

// EventList는 SQLite에 캐시된 접근 가능한 모든 일정을 반환한다.
// 외부 서버 호출은 SyncNow에서만 수행한다. 화면 진입/월 이동은
// 네트워크를 사용하지 않고 마지막 동기화 스냅샷을 보여준다.
// from/to가 빈 문자열이면 전체 범위를 조회한다.
func (s *Service) EventList(from, to string) ([]types.Event, error) {
	accs, err := s.store.ListAccounts()
	if err != nil {
		return nil, err
	}
	var all []types.Event
	var firstCalDAVError error

	for _, acc := range accs {
		if acc.Source == account.SourceLocal {
			events, err := s.store.EventsByCalendar(acc.ID)
			if err != nil {
				return nil, err
			}
			all = append(all, events...)
		} else if acc.Source == account.SourceCalDAV && acc.SyncEnabled {
			// CalDAV는 새로고침 버튼의 SyncNow가 갱신한 SQLite 캐시만 읽는다.
			events, err := s.store.EventsByCalendar(acc.ID)
			if err != nil {
				if firstCalDAVError == nil {
					firstCalDAVError = fmt.Errorf("캐시 조회 실패: %w", err)
				}
				continue
			}
			all = append(all, events...)
		}
	}
	if len(all) == 0 && firstCalDAVError != nil {
		return nil, firstCalDAVError
	}
	// 칸반 카드 마감일을 기존 캘린더에서 읽기 전용 종일 일정으로 표시한다.
	if kanbanEvents, err := kanbanDueEvents(); err == nil {
		all = append(all, kanbanEvents...)
	}
	if from != "" {
		all = filterFrom(all, from)
	}
	if to != "" {
		all = filterTo(all, to)
	}
	return all, nil
}

func kanbanDueEvents() ([]types.Event, error) {
	st, err := kanbanstore.New()
	if err != nil {
		return nil, err
	}
	d, err := st.Load()
	if err != nil {
		return nil, err
	}
	var events []types.Event
	for _, card := range d.Cards {
		if card.Archived || card.DueDate == "" {
			continue
		}
		start := card.DueDate + "T00:00:00Z"
		endDate, err := time.Parse("2006-01-02", card.DueDate)
		if err != nil {
			continue
		}
		end := endDate.AddDate(0, 0, 1).Format("2006-01-02") + "T00:00:00Z"
		events = append(events, types.Event{
			UID: "kanban:" + card.ID, CalendarID: "kanban", Title: card.Title,
			Start: start, End: end, AllDay: true, Description: "칸반 카드 마감일",
		})
	}
	return events, nil
}

// EventsByAccount는 단일 계정의 일정만 반환한다.
// CalDAV 계정은 서버에서, 로컬 계정은 저장소에서 조회한다.
func (s *Service) EventsByAccount(accID string) ([]types.Event, error) {
	acc, err := s.store.GetAccount(accID)
	if err != nil {
		return nil, err
	}
	if acc.Source == account.SourceCalDAV {
		return s.store.EventsByCalendar(accID)
	}
	return s.store.EventsByCalendar(accID)
}

// Calendars는 CalDAV 계정의 캘린더(폴더) 목록을 반환한다.
// 로컬 계정인 경우 기본 캘린더 하나를 반환한다.
func (s *Service) Calendars(accID string) ([]types.CalendarInfo, error) {
	acc, err := s.store.GetAccount(accID)
	if err != nil {
		return nil, err
	}
	if acc.Source == account.SourceLocal {
		return []types.CalendarInfo{{Name: acc.Name, Color: acc.Color}}, nil
	}
	c, err := s.newCalDAVClient(acc)
	if err != nil {
		return nil, err
	}
	return c.Calendars()
}

// EventCreate은 새 일정을 생성한다.
// 로컬 계정은 저장소에 저장하고, CalDAV 계정은 서버에 PUT 한다.
func (s *Service) EventCreate(ev *types.Event) (*types.Event, error) {
	if ev == nil {
		return nil, fmt.Errorf("일정 정보가 없습니다")
	}
	if strings.TrimSpace(ev.Title) == "" {
		return nil, fmt.Errorf("일정 제목은 필수입니다")
	}
	if ev.Start == "" || ev.End == "" {
		return nil, fmt.Errorf("시작/종료 시각은 필수입니다")
	}
	if ev.UID == "" {
		ev.UID = newID()
	}
	acc, err := s.store.GetAccount(ev.CalendarID)
	if err != nil {
		return nil, err
	}
	if acc.Source == account.SourceCalDAV {
		c, err := s.newCalDAVClient(acc)
		if err != nil {
			return nil, err
		}
		// 기본 캘린더 URL을 사용(첫 번째 캘린더).
		cals, _ := c.Calendars()
		calURL := acc.CalDAVURL
		if len(cals) > 0 {
			calURL = cals[0].Href
		}
		return c.CreateEvent(calURL, ev)
	}
	if err := s.store.SaveEvent(ev); err != nil {
		return nil, err
	}
	return ev, nil
}

// EventUpdate는 기존 일정을 갱신한다.
// CalDAV 계정은 ETag 기반 동시성 제어를 수행한다.
func (s *Service) EventUpdate(ev *types.Event) (*types.Event, error) {
	if ev == nil || ev.UID == "" {
		return nil, fmt.Errorf("일정 UID가 필요합니다")
	}
	acc, err := s.store.GetAccount(ev.CalendarID)
	if err != nil {
		return nil, err
	}
	if acc.Source == account.SourceCalDAV {
		c, err := s.newCalDAVClient(acc)
		if err != nil {
			return nil, err
		}
		cals, _ := c.Calendars()
		calURL := acc.CalDAVURL
		if len(cals) > 0 {
			calURL = cals[0].Href
		}
		return c.UpdateEvent(calURL, ev)
	}
	if err := s.store.SaveEvent(ev); err != nil {
		return nil, err
	}
	return ev, nil
}

// EventDelete는 일정을 삭제한다.
func (s *Service) EventDelete(calendarID, uid string) error {
	acc, err := s.store.GetAccount(calendarID)
	if err != nil {
		return err
	}
	if acc.Source == account.SourceCalDAV {
		c, err := s.newCalDAVClient(acc)
		if err != nil {
			return err
		}
		cals, _ := c.Calendars()
		calURL := acc.CalDAVURL
		if len(cals) > 0 {
			calURL = cals[0].Href
		}
		ev := &types.Event{UID: uid, CalendarID: calendarID}
		return c.DeleteEvent(calURL, ev)
	}
	return s.store.DeleteEvent(calendarID, uid)
}

// SyncNow는 CalDAV 계정의 일정을 즉시 동기화한다.
// 동기화 성공 시 계정의 LastSyncAt을 갱신한다.
func (s *Service) SyncNow(accID string) error {
	acc, err := s.store.GetAccount(accID)
	if err != nil {
		return err
	}
	if acc.Source != account.SourceCalDAV {
		return fmt.Errorf("로컬 계정은 동기화 대상이 아닙니다")
	}
	events, err := s.caldavEvents(acc)
	if err != nil {
		return err
	}
	if err := s.store.ReplaceEvents(acc.ID, events); err != nil {
		return err
	}
	acc.LastSyncAt = time.Now().UTC().Format(time.RFC3339)
	return s.store.SaveAccount(acc, "")
}

// caldavEvents는 CalDAV 서버에서 모든 캘린더의 일정을 조회해 합친다.
func (s *Service) caldavEvents(acc *account.Account) ([]types.Event, error) {
	c, err := s.newCalDAVClient(acc)
	if err != nil {
		return nil, err
	}
	cals, err := c.Calendars()
	if err != nil {
		return nil, err
	}
	if len(cals) == 0 {
		return nil, fmt.Errorf("CalDAV 서버에서 캘린더를 찾지 못했습니다")
	}
	var all []types.Event
	var firstCalendarError error
	for _, cal := range cals {
		events, err := c.Events(cal.Href)
		if err != nil {
			if firstCalendarError == nil {
				firstCalendarError = fmt.Errorf("캘린더 %q 일정 조회 실패: %w", cal.Name, err)
			}
			continue
		}
		for i := range events {
			events[i].CalendarID = acc.ID
			events[i].CalendarHref = cal.Href
		}
		all = append(all, events...)
	}
	if len(all) == 0 && firstCalendarError != nil {
		return nil, firstCalendarError
	}
	return all, nil
}

// filterFrom은 시작 시각이 from 이후인 일정만 남긴다.
func filterFrom(events []types.Event, from string) []types.Event {
	var out []types.Event
	for _, e := range events {
		if e.Start >= from {
			out = append(out, e)
		}
	}
	return out
}

// filterTo는 시작 시각이 to 이전인 일정만 남긴다.
func filterTo(events []types.Event, to string) []types.Event {
	var out []types.Event
	for _, e := range events {
		if e.Start <= to {
			out = append(out, e)
		}
	}
	return out
}

// newID는 16바이트 난수 기반의 고유 ID를 생성한다.
func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
