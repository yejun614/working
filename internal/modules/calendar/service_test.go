package calendar

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zalando/go-keyring"

	accountstore "working/internal/modules/account/store"
	account "working/internal/modules/account/types"
	"working/internal/modules/calendar/types"
	"working/internal/storage"
)

// newCalDAVTestServer는 캘린더 하나를 노출하고 PUT/DELETE를 받아 주는 최소 CalDAV 서버이다.
func newCalDAVTestServer(t *testing.T) (*httptest.Server, *int) {
	t.Helper()
	puts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "PROPFIND" && r.URL.Path == "/":
			w.WriteHeader(http.StatusMultiStatus)
			_, _ = io.WriteString(w, `<?xml version="1.0"?><D:multistatus xmlns:D="DAV:"><D:response><D:propstat><D:prop><D:current-user-principal><D:href>/principal/</D:href></D:current-user-principal></D:prop></D:propstat></D:response></D:multistatus>`)
		case r.Method == "PROPFIND" && r.URL.Path == "/principal/":
			w.WriteHeader(http.StatusMultiStatus)
			_, _ = io.WriteString(w, `<?xml version="1.0"?><D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:response><D:propstat><D:prop><C:calendar-home-set><D:href>/calendars/</D:href></C:calendar-home-set></D:prop></D:propstat></D:response></D:multistatus>`)
		case r.Method == "PROPFIND" && r.URL.Path == "/calendars/":
			w.WriteHeader(http.StatusMultiStatus)
			_, _ = io.WriteString(w, `<?xml version="1.0"?><D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:response><D:href>/calendars/personal/</D:href><D:propstat><D:prop><D:displayname>개인</D:displayname><D:resourcetype><D:collection/><C:calendar/></D:resourcetype></D:prop></D:propstat></D:response></D:multistatus>`)
		case r.Method == "PUT" && strings.HasPrefix(r.URL.Path, "/calendars/personal/"):
			puts++
			w.Header().Set("ETag", `"etag-1"`)
			w.WriteHeader(http.StatusCreated)
		case r.Method == "DELETE" && strings.HasPrefix(r.URL.Path, "/calendars/personal/"):
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("예상하지 못한 요청: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return server, &puts
}

// newTestService는 임시 데이터 디렉터리와 mock 키체인을 쓰는 캘린더 서비스를 만든다.
func newTestService(t *testing.T, calDAVURL string) (*Service, string) {
	t.Helper()
	keyring.MockInit()
	// config.Dir이 os.UserConfigDir을 쓰므로 앱 DB를 테스트마다 격리한다.
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	// 공유 SQLite 연결이 열려 있으면 Windows에서 임시 디렉터리를 지울 수 없다.
	t.Cleanup(func() { _ = storage.Close() })

	accounts, err := accountstore.New()
	if err != nil {
		t.Fatalf("계정 저장소 생성 실패: %v", err)
	}
	acc := &account.Account{
		ID:       "acc-1",
		Name:     "테스트 캘린더",
		Email:    "user@example.com",
		AuthType: account.AuthPassword,
		Calendar: &account.CalendarConfig{
			Source:      account.CalendarSourceCalDAV,
			URL:         calDAVURL,
			Username:    "user@example.com",
			SyncEnabled: true,
		},
	}
	if err := accounts.Save(acc, "app-password"); err != nil {
		t.Fatalf("계정 저장 실패: %v", err)
	}

	service, err := NewService()
	if err != nil {
		t.Fatalf("캘린더 서비스 생성 실패: %v", err)
	}
	return service, acc.ID
}

func TestEventCreateOnCalDAVAppearsInEventList(t *testing.T) {
	server, puts := newCalDAVTestServer(t)
	service, accID := newTestService(t, server.URL)

	created, err := service.EventCreate(&types.Event{
		CalendarID: accID,
		Title:      "팀 회의",
		Start:      "2026-08-04T01:00:00Z",
		End:        "2026-08-04T02:00:00Z",
	})
	if err != nil {
		t.Fatalf("일정 생성 실패: %v", err)
	}
	if *puts != 1 {
		t.Fatalf("서버 PUT 횟수 = %d, want 1", *puts)
	}
	if created.CalendarID != accID {
		t.Fatalf("CalendarID = %q, want %q", created.CalendarID, accID)
	}

	// EventList는 캐시만 읽으므로, 생성 결과가 캐시에 저장되어야 SyncNow 없이 화면에 보인다.
	events, err := service.EventList("", "")
	if err != nil {
		t.Fatalf("일정 조회 실패: %v", err)
	}
	found := false
	for _, ev := range events {
		if ev.UID == created.UID {
			found = true
			if ev.Title != "팀 회의" {
				t.Fatalf("제목 = %q, want 팀 회의", ev.Title)
			}
			if ev.CalendarHref == "" {
				t.Fatalf("CalendarHref가 비어 있어 캘린더 색상 매칭이 되지 않습니다")
			}
		}
	}
	if !found {
		t.Fatalf("생성한 일정이 목록에 없습니다: %+v", events)
	}
}

func TestEventDeleteOnCalDAVRemovesFromEventList(t *testing.T) {
	server, _ := newCalDAVTestServer(t)
	service, accID := newTestService(t, server.URL)

	created, err := service.EventCreate(&types.Event{
		CalendarID: accID,
		Title:      "삭제할 일정",
		Start:      "2026-08-04T01:00:00Z",
		End:        "2026-08-04T02:00:00Z",
	})
	if err != nil {
		t.Fatalf("일정 생성 실패: %v", err)
	}
	if err := service.EventDelete(accID, created.UID); err != nil {
		t.Fatalf("일정 삭제 실패: %v", err)
	}

	events, err := service.EventList("", "")
	if err != nil {
		t.Fatalf("일정 조회 실패: %v", err)
	}
	for _, ev := range events {
		if ev.UID == created.UID {
			t.Fatalf("삭제한 일정이 목록에 남아 있습니다: %+v", ev)
		}
	}
}
