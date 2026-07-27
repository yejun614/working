package caldav

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFindPrincipalURLUsesZeroDepth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PROPFIND" {
			t.Fatalf("method = %q, want PROPFIND", r.Method)
		}
		if got := r.Header.Get("Depth"); got != "0" {
			t.Fatalf("Depth = %q, want 0", got)
		}
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = io.WriteString(w, `<?xml version="1.0"?><D:multistatus xmlns:D="DAV:"><D:response><D:propstat><D:prop><D:current-user-principal><D:href>/principal/</D:href></D:current-user-principal></D:prop></D:propstat></D:response></D:multistatus>`)
	}))
	defer server.Close()

	client := &Client{url: server.URL, httpClient: server.Client()}
	got, err := client.findPrincipalURL()
	if err != nil {
		t.Fatalf("findPrincipalURL() error = %v", err)
	}
	if !strings.HasSuffix(got, "/principal/") {
		t.Fatalf("principal URL = %q", got)
	}
}

func TestParseCalendarListSupportsNamespacePrefixes(t *testing.T) {
	data := []byte(`<?xml version="1.0"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:response>
    <D:href>/calendars/user/work/</D:href>
    <D:propstat>
      <D:prop>
        <D:displayname>업무</D:displayname>
        <D:resourcetype><C:calendar/></D:resourcetype>
        <C:calendar-description>업무 일정</C:calendar-description>
      </D:prop>
    </D:propstat>
  </D:response>
</D:multistatus>`)

	got := parseCalendarList(data, "https://example.test/calendars/user/")
	if len(got) != 1 {
		t.Fatalf("캘린더 수: got %d, want 1", len(got))
	}
	if got[0].Href != "/calendars/user/work/" {
		t.Errorf("href: got %q", got[0].Href)
	}
	if got[0].Name != "업무" {
		t.Errorf("name: got %q", got[0].Name)
	}
}

func TestParseReportResponseSupportsNamespacePrefixes(t *testing.T) {
	data := []byte(`<?xml version="1.0"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:response>
    <D:href>/calendars/user/work/event.ics</D:href>
    <D:propstat>
      <D:prop>
        <D:getetag>"abc"</D:getetag>
        <C:calendar-data>BEGIN:VCALENDAR
VERSION:2.0
BEGIN:VEVENT
UID:event-1
SUMMARY:회의
DTSTART:20260727T090000Z
DTEND:20260727T100000Z
END:VEVENT
END:VCALENDAR</C:calendar-data>
      </D:prop>
    </D:propstat>
  </D:response>
</D:multistatus>`)

	got, err := parseReportResponse(data, "account-1")
	if err != nil {
		t.Fatalf("parseReportResponse() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("일정 수: got %d, want 1", len(got))
	}
	if got[0].UID != "event-1" || got[0].Title != "회의" {
		t.Errorf("event: got UID=%q title=%q", got[0].UID, got[0].Title)
	}
	if got[0].Href != "/calendars/user/work/event.ics" {
		t.Errorf("href: got %q", got[0].Href)
	}
	if got[0].ETag != `"abc"` {
		t.Errorf("etag: got %q", got[0].ETag)
	}
}

func TestFindPropertyHrefIgnoresResponseHref(t *testing.T) {
	data := []byte(`<?xml version="1.0"?>
<D:multistatus xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:response>
    <D:href>/principal/resource</D:href>
    <D:propstat>
      <D:prop>
        <C:calendar-home-set><D:href>/calendars/user/</D:href></C:calendar-home-set>
      </D:prop>
    </D:propstat>
  </D:response>
</D:multistatus>`)

	got := findPropertyHref(data, "calendar-home-set")
	if got != "/calendars/user/" {
		t.Fatalf("findPropertyHref() = %q, want /calendars/user/", got)
	}
}

func TestGoogleCalDAVURL(t *testing.T) {
	base := "https://apidata.googleusercontent.com/caldav/v2"
	got := googleCalDAVURL(base, "user@example.com")
	want := "https://apidata.googleusercontent.com/caldav/v2/user@example.com/user"
	if got != want {
		t.Fatalf("googleCalDAVURL() = %q, want %q", got, want)
	}

	if got := googleCalDAVURL(base+"/user", "user@example.com"); got != base+"/user" {
		t.Fatalf("기존 principal URL이 변경됨: %q", got)
	}
}
