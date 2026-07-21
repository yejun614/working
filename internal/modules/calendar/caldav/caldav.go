// Package caldav 는 CalDAV(RFC 4791) 프로토콜 클라이언트를 구현한다.
// CalDAV 서버에서 캘린더 목록, 일정 조회/생성/수정/삭제를 수행한다.
package caldav

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"working/internal/modules/calendar/account"
	"working/internal/modules/calendar/ical"
	"working/internal/modules/calendar/types"
)

// Client는 단일 CalDAV 서버와 통신하는 클라이언트이다.
type Client struct {
	// url은 CalDAV 캘린더 홈 URL.
	url string

	// username은 인증용 사용자 이름.
	username string

	// password는 비밀번호 또는 앱 비밀번호.
	password string

	// httpClient는 요청에 사용하는 HTTP 클라이언트.
	httpClient *http.Client
}

// NewClient는 CalDAV 클라이언트를 생성한다.
func NewClient(acc *account.Account, credential string) *Client {
	return &Client{
		url:        strings.TrimRight(acc.CalDAVURL, "/"),
		username:   acc.Username,
		password:   credential,
		httpClient: &http.Client{},
	}
}

// Calendars는 사용자의 캘린더(폴더) 목록을 조회한다.
// CalDAV 서버에 PROPFIND calendar-home-set 요청 후 각 캘린더를 조회한다.
func (c *Client) Calendars() ([]types.CalendarInfo, error) {
	principalURL, err := c.findPrincipalURL()
	if err != nil {
		return nil, err
	}
	homeSet, err := c.findCalendarHomeSet(principalURL)
	if err != nil {
		return nil, err
	}
	return c.listCalendars(homeSet)
}

// Events는 지정한 캘린더 URL의 일정을 조회한다.
// from부터 to까지의 시간 범위에 겹치는 일정을 반환한다.
func (c *Client) Events(calendarURL string) ([]types.Event, error) {
	full := c.resolveURL(calendarURL)
	body := `<?xml version="1.0" encoding="utf-8"?>
<C:calendar-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <D:getetag/>
    <C:calendar-data/>
  </D:prop>
  <C:filter>
    <C:comp-filter name="VCALENDAR">
      <C:comp-filter name="VEVENT"/>
    </C:comp-filter>
  </C:filter>
</C:calendar-query>`
	resp, err := c.request("REPORT", full, []byte(body), "application/calendar; charset=utf-8")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 207 {
		return nil, fmt.Errorf("일정 조회 실패: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("응답 읽기 실패: %w", err)
	}
	return parseReportResponse(data, calendarURL)
}

// CreateEvent는 새 일정을 생성한다.
// CalDAV 서버에 iCalendar 데이터를 PUT 한다.
func (c *Client) CreateEvent(calendarURL string, ev *types.Event) (*types.Event, error) {
	full := c.resolveURL(calendarURL)
	objectURL := strings.TrimRight(full, "/") + "/" + ev.UID + ".ics"

	icalData, err := ical.SerializeEvent(ev)
	if err != nil {
		return nil, err
	}
	resp, err := c.request("PUT", objectURL, []byte(icalData), "text/calendar; charset=utf-8")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 && resp.StatusCode != 204 && resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("일정 생성 실패: HTTP %d: %s", resp.StatusCode, string(body))
	}
	ev.Href = objectURL
	ev.ETag = resp.Header.Get("ETag")
	return ev, nil
}

// UpdateEvent는 기존 일정을 갱신한다.
// If-Match 헤더로 ETag 기반 동시성 제어를 수행한다.
func (c *Client) UpdateEvent(calendarURL string, ev *types.Event) (*types.Event, error) {
	objectURL := c.resolveURL(ev.Href)
	if objectURL == "" {
		// Href가 없으면 캘린더 URL + UID.ics로 구성.
		objectURL = strings.TrimRight(c.resolveURL(calendarURL), "/") + "/" + ev.UID + ".ics"
	}

	icalData, err := ical.SerializeEvent(ev)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("PUT", objectURL, bytes.NewReader([]byte(icalData)))
	if err != nil {
		return nil, err
	}
	c.setAuth(req)
	req.Header.Set("Content-Type", "text/calendar; charset=utf-8")
	if ev.ETag != "" {
		req.Header.Set("If-Match", ev.ETag)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("일정 수정 요청 실패: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("일정 수정 실패: HTTP %d: %s", resp.StatusCode, string(body))
	}
	ev.ETag = resp.Header.Get("ETag")
	return ev, nil
}

// DeleteEvent는 일정을 삭제한다.
func (c *Client) DeleteEvent(calendarURL string, ev *types.Event) error {
	objectURL := c.resolveURL(ev.Href)
	if objectURL == "" {
		objectURL = strings.TrimRight(c.resolveURL(calendarURL), "/") + "/" + ev.UID + ".ics"
	}
	req, err := http.NewRequest("DELETE", objectURL, nil)
	if err != nil {
		return err
	}
	c.setAuth(req)
	if ev.ETag != "" {
		req.Header.Set("If-Match", ev.ETag)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("일정 삭제 요청 실패: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 204 && resp.StatusCode != 200 && resp.StatusCode != 404 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("일정 삭제 실패: HTTP %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// findPrincipalURL은 PROPFIND로 current-user-principal URL을 찾는다.
func (c *Client) findPrincipalURL() (string, error) {
	body := `<?xml version="1.0" encoding="utf-8"?>
<D:propfind xmlns:D="DAV:">
  <D:prop>
    <D:current-user-principal/>
  </D:prop>
</D:propfind>`
	resp, err := c.request("PROPFIND", c.url, []byte(body), "application/xml; charset=utf-8")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 207 {
		return "", fmt.Errorf("principal 조회 실패: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	href := extractTag(data, []byte("current-user-principal"))
	if href == "" {
		// principal을 못 찾으면 URL 자체를 principal로 간주.
		return c.url, nil
	}
	h := extractTag([]byte(href), []byte("href"))
	if h == "" {
		return c.url, nil
	}
	return c.resolveURL(h), nil
}

// findCalendarHomeSet은 PROPFIND로 calendar-home-set URL을 찾는다.
func (c *Client) findCalendarHomeSet(principalURL string) (string, error) {
	body := `<?xml version="1.0" encoding="utf-8"?>
<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <C:calendar-home-set/>
  </D:prop>
</D:propfind>`
	resp, err := c.request("PROPFIND", principalURL, []byte(body), "application/xml; charset=utf-8")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 207 {
		return principalURL, nil
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return principalURL, nil
	}
	hs := extractTag(data, []byte("calendar-home-set"))
	if hs == "" {
		return principalURL, nil
	}
	h := extractTag([]byte(hs), []byte("href"))
	if h == "" {
		return principalURL, nil
	}
	return c.resolveURL(h), nil
}

// listCalendars는 calendar-home-set 하위 캘린더 목록을 조회한다.
func (c *Client) listCalendars(homeSetURL string) ([]types.CalendarInfo, error) {
	body := `<?xml version="1.0" encoding="utf-8"?>
<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:I="http://apple.com/ns/ical/">
  <D:prop>
    <D:displayname/>
    <D:resourcetype/>
    <C:calendar-description/>
    <I:calendar-color/>
  </D:prop>
</D:propfind>`
	req, err := http.NewRequest("PROPFIND", homeSetURL, bytes.NewReader([]byte(body)))
	if err != nil {
		return nil, err
	}
	c.setAuth(req)
	req.Header.Set("Content-Type", "application/xml; charset=utf-8")
	req.Header.Set("Depth", "1")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("캘린더 목록 조회 실패: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 207 {
		return nil, fmt.Errorf("캘린더 목록 조회 실패: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return parseCalendarList(data, homeSetURL), nil
}

// parseCalendarList는 PROPFIND 응답에서 캘린더 목록을 추출한다.
func parseCalendarList(data []byte, baseURL string) []types.CalendarInfo {
	var out []types.CalendarInfo
	responses := splitAll(data, []byte("<D:response>"))
	if len(responses) == 0 {
		responses = splitAll(data, []byte("<response>"))
	}
	for _, respChunk := range responses {
		href := extractTag(respChunk, []byte("href"))
		if href == "" {
			continue
		}
		// 캘린더 리소스 타입인지 확인.
		if !bytes.Contains(respChunk, []byte("calendar")) {
			continue
		}
		name := textOf(respChunk, []byte("displayname"))
		color := textOf(respChunk, []byte("calendar-color"))
		desc := textOf(respChunk, []byte("calendar-description"))
		out = append(out, types.CalendarInfo{
			Href:        href,
			Name:        name,
			Color:       color,
			Description: desc,
		})
	}
	return out
}

// parseReportResponse는 REPORT 응답에서 일정 목록을 추출한다.
func parseReportResponse(data []byte, calendarID string) ([]types.Event, error) {
	var out []types.Event
	responses := splitAll(data, []byte("<D:response>"))
	if len(responses) == 0 {
		responses = splitAll(data, []byte("<response>"))
	}
	for _, respChunk := range responses {
		href := extractTag(respChunk, []byte("href"))
		etag := textOf(respChunk, []byte("getetag"))
		calData := textOf(respChunk, []byte("calendar-data"))
		if calData == "" {
			continue
		}
		events, err := ical.ParseCalendar([]byte(calData), calendarID)
		if err != nil {
			continue
		}
		for i := range events {
			events[i].Href = href
			events[i].ETag = etag
			out = append(out, events[i])
		}
	}
	return out, nil
}

// request는 지정한 메서드/URL로 HTTP 요청을 보낸다.
func (c *Client) request(method, url string, body []byte, contentType string) (*http.Response, error) {
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	c.setAuth(req)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return c.httpClient.Do(req)
}

// setAuth는 Basic 인증 헤더를 설정한다.
func (c *Client) setAuth(req *http.Request) {
	req.SetBasicAuth(c.username, c.password)
}

// resolveURL은 상대 경로를 절대 URL로 변환한다.
func (c *Client) resolveURL(href string) string {
	if href == "" {
		return ""
	}
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
		return href
	}
	if strings.HasPrefix(href, "/") {
		idx := strings.Index(c.url, "://")
		slash := strings.IndexByte(c.url[idx+3:], '/')
		if slash < 0 {
			return c.url + href
		}
		return c.url[:idx+3+slash] + href
	}
	return c.url + "/" + href
}

// extractTag는 XML 조각에서 첫 번째 <tag>...</tag>(또는 <ns:tag> 같이
// 네임스페이스 접두사가 붙은 형태)의 내용을 반환한다.
// 속성이 있는 열린 태그(<tag foo="bar">)도 처리한다.
func extractTag(data []byte, tag []byte) string {
	local := string(tag)
	open := "<" + local
	closeTag := "</" + local

	start := bytes.Index(data, []byte(open))
	if start < 0 {
		return ""
	}
	gt := bytes.IndexByte(data[start:], '>')
	if gt < 0 {
		return ""
	}
	contentStart := start + gt + 1
	end := bytes.Index(data[contentStart:], []byte(closeTag))
	if end < 0 {
		return ""
	}
	return string(data[contentStart : contentStart+end])
}

// textOf는 XML 조각에서 <tag>...</tag>의 텍스트 내용을 반환한다.
func textOf(data []byte, tag []byte) string {
	return strings.TrimSpace(extractTag(data, tag))
}

// splitAll는 data를 sep 기준으로 분할한다(sep 자체는 제거).
func splitAll(data, sep []byte) [][]byte {
	parts := bytes.Split(data, sep)
	if len(parts) <= 1 {
		return nil
	}
	return parts[1:]
}
