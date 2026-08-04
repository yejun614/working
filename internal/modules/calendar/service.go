// Package calendar 은 "working" 앱의 캘린더 모듈 진입점이다.
//
// 이 모듈은 로컬 일정 관리와 외부 CalDAV 서버 연동 기능을
// Wails Service 형태로 프론트엔드에 노출한다. 계정과 자격증명은 통합 계정
// 모듈(internal/modules/account)이 관리하며, 이 모듈은 캘린더 기능이 켜진
// 계정만 읽어 사용하고 일정 캐시만 직접 보관한다.
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

	accountstore "working/internal/modules/account/store"
	account "working/internal/modules/account/types"
	"working/internal/modules/calendar/caldav"
	"working/internal/modules/calendar/store"
	"working/internal/modules/calendar/types"
	kanbanstore "working/internal/modules/kanban/store"
)

// Service는 프론트엔드에 바인딩되는 캘린더 모듈 서비스이다.
// 모든 메서드는 Wails를 통해 JS/TS에서 호출 가능하다.
type Service struct {
	store    *store.Store
	accounts *accountstore.Store
}

// NewService는 캘린더 모듈 Service를 생성한다.
// 내부 저장소 초기화 실패 시 에러를 반환한다.
func NewService() (*Service, error) {
	st, err := store.New()
	if err != nil {
		return nil, err
	}
	accounts, err := accountstore.New()
	if err != nil {
		return nil, err
	}
	return &Service{store: st, accounts: accounts}, nil
}

// ServiceShutdown은 Wails 종료 시 호출되는 훅이다.
// 현재는 유지할 상태가 없으므로 아무 작업도 하지 않는다.
func (s *Service) ServiceShutdown() {}

// newCalDAVClient는 계정 자격증명을 키체인에서 읽어 CalDAV 클라이언트를 만든다.
// OAuth access token이 갱신되면 동일 계정의 키체인 credential도 갱신한다.
func (s *Service) newCalDAVClient(acc *account.Account) (*caldav.Client, error) {
	credential, err := s.accounts.Credential(acc.ID)
	if err != nil {
		return nil, err
	}
	return caldav.NewClient(acc, credential, func(updated string) error {
		return s.accounts.SaveCredential(acc.ID, updated)
	})
}

// AccountList는 캘린더 기능이 켜진 계정만 반환한다(자격증명 제외).
// 계정 등록·수정·삭제와 Google 재인증은 통합 계정 모듈이 담당한다.
func (s *Service) AccountList() ([]account.Account, error) {
	all, err := s.accounts.List()
	if err != nil {
		return nil, err
	}
	out := make([]account.Account, 0, len(all))
	for _, acc := range all {
		if acc.UsesCalendar() {
			out = append(out, acc)
		}
	}
	return out, nil
}

// AccountGet은 ID로 계정을 조회한다.
func (s *Service) AccountGet(id string) (*account.Account, error) {
	return s.calendarAccount(id)
}

// calendarAccount는 캘린더 기능이 켜진 계정을 조회한다.
func (s *Service) calendarAccount(id string) (*account.Account, error) {
	acc, err := s.accounts.Get(id)
	if err != nil {
		return nil, err
	}
	if !acc.UsesCalendar() {
		return nil, fmt.Errorf("캘린더를 사용하지 않는 계정입니다: %s", acc.Name)
	}
	return acc, nil
}

// EventList는 SQLite에 캐시된 접근 가능한 모든 일정을 반환한다.
// 외부 서버 호출은 SyncNow에서만 수행한다. 화면 진입/월 이동은
// 네트워크를 사용하지 않고 마지막 동기화 스냅샷을 보여준다.
// from/to가 빈 문자열이면 전체 범위를 조회한다.
func (s *Service) EventList(from, to string) ([]types.Event, error) {
	accs, err := s.AccountList()
	if err != nil {
		return nil, err
	}
	var all []types.Event
	var firstCalDAVError error

	for _, acc := range accs {
		if acc.Calendar.Source == account.CalendarSourceLocal {
			events, err := s.store.EventsByCalendar(acc.ID)
			if err != nil {
				return nil, err
			}
			all = append(all, events...)
		} else if acc.Calendar.Source == account.CalendarSourceCalDAV && acc.Calendar.SyncEnabled {
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
	if _, err := s.calendarAccount(accID); err != nil {
		return nil, err
	}
	return s.store.EventsByCalendar(accID)
}

// Calendars는 CalDAV 계정의 캘린더(폴더) 목록을 반환한다.
// 로컬 계정인 경우 기본 캘린더 하나를 반환한다.
func (s *Service) Calendars(accID string) ([]types.CalendarInfo, error) {
	acc, err := s.calendarAccount(accID)
	if err != nil {
		return nil, err
	}
	if acc.Calendar.Source == account.CalendarSourceLocal {
		return []types.CalendarInfo{{Name: acc.Name, Color: acc.Calendar.Color}}, nil
	}
	c, err := s.newCalDAVClient(acc)
	if err != nil {
		return nil, err
	}
	cals, err := c.Calendars()
	return cals, s.noteCalDAVError(acc, err)
}

// noteCalDAVError는 CalDAV 호출 결과를 계정의 인증 상태에 반영한다.
// 재인증이 필요한 오류이면 계정에 사유를 기록해 프론트엔드가 안내와 재인증 버튼을
// 띄울 수 있게 하고, 호출이 성공하면 남아 있던 안내를 해제한다.
func (s *Service) noteCalDAVError(acc *account.Account, err error) error {
	if err == nil {
		if acc.AuthError != "" {
			acc.AuthError = ""
			_ = s.accounts.Save(acc, "")
		}
		return nil
	}
	if !caldav.IsAuthError(err) {
		return err
	}
	if acc.AuthError != err.Error() {
		acc.AuthError = err.Error()
		_ = s.accounts.Save(acc, "")
	}
	return fmt.Errorf("Google 계정 인증이 만료되어 재인증이 필요합니다: %w", err)
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
	acc, err := s.calendarAccount(ev.CalendarID)
	if err != nil {
		return nil, err
	}
	if acc.Calendar.Source == account.CalendarSourceCalDAV {
		c, err := s.newCalDAVClient(acc)
		if err != nil {
			return nil, err
		}
		// 기본 캘린더 URL을 사용(첫 번째 캘린더).
		cals, _ := c.Calendars()
		calURL := acc.Calendar.URL
		if len(cals) > 0 {
			calURL = cals[0].Href
		}
		created, err := c.CreateEvent(calURL, ev)
		return created, s.noteCalDAVError(acc, err)
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
	acc, err := s.calendarAccount(ev.CalendarID)
	if err != nil {
		return nil, err
	}
	if acc.Calendar.Source == account.CalendarSourceCalDAV {
		c, err := s.newCalDAVClient(acc)
		if err != nil {
			return nil, err
		}
		cals, _ := c.Calendars()
		calURL := acc.Calendar.URL
		if len(cals) > 0 {
			calURL = cals[0].Href
		}
		updated, err := c.UpdateEvent(calURL, ev)
		return updated, s.noteCalDAVError(acc, err)
	}
	if err := s.store.SaveEvent(ev); err != nil {
		return nil, err
	}
	return ev, nil
}

// EventDelete는 일정을 삭제한다.
func (s *Service) EventDelete(calendarID, uid string) error {
	acc, err := s.calendarAccount(calendarID)
	if err != nil {
		return err
	}
	if acc.Calendar.Source == account.CalendarSourceCalDAV {
		c, err := s.newCalDAVClient(acc)
		if err != nil {
			return err
		}
		cals, _ := c.Calendars()
		calURL := acc.Calendar.URL
		if len(cals) > 0 {
			calURL = cals[0].Href
		}
		ev := &types.Event{UID: uid, CalendarID: calendarID}
		return s.noteCalDAVError(acc, c.DeleteEvent(calURL, ev))
	}
	return s.store.DeleteEvent(calendarID, uid)
}

// SyncNow는 CalDAV 계정의 일정을 즉시 동기화한다.
// 동기화 성공 시 계정의 LastSyncAt을 갱신한다.
func (s *Service) SyncNow(accID string) error {
	acc, err := s.calendarAccount(accID)
	if err != nil {
		return err
	}
	if acc.Calendar.Source != account.CalendarSourceCalDAV {
		return fmt.Errorf("로컬 계정은 동기화 대상이 아닙니다")
	}
	events, err := s.caldavEvents(acc)
	if err != nil {
		return s.noteCalDAVError(acc, err)
	}
	if err := s.store.ReplaceEvents(acc.ID, events); err != nil {
		return err
	}
	acc.Calendar.LastSyncAt = time.Now().UTC().Format(time.RFC3339)
	// 동기화에 성공했으므로 남아 있던 재인증 안내를 해제한다.
	acc.AuthError = ""
	return s.accounts.Save(acc, "")
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
