// Package caldav 는 CalDAV(RFC 4791) 프로토콜 클라이언트를 구현한다.
// CalDAV 서버에서 캘린더 목록, 일정 조회/생성/수정/삭제를 수행한다.
package caldav

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"working/internal/config"
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

	// authType은 Basic 또는 OAuth2 인증 방식이다.
	authType account.AuthType

	// httpClient는 요청에 사용하는 HTTP 클라이언트.
	httpClient *http.Client
}

// NewClient는 CalDAV 클라이언트를 생성한다.
func NewClient(acc *account.Account, credential string, onTokenRefresh ...func(string) error) (*Client, error) {
	c := &Client{
		url:        strings.TrimRight(acc.CalDAVURL, "/"),
		username:   acc.Username,
		password:   credential,
		authType:   acc.AuthType,
		httpClient: &http.Client{},
	}
	if acc.AuthType == account.AuthOAuth2 {
		if strings.TrimSpace(config.GoogleClientID()) == "" {
			return nil, fmt.Errorf("GOOGLE_CLIENT_ID가 설정되지 않았습니다")
		}
		var token oauth2.Token
		if err := json.Unmarshal([]byte(credential), &token); err != nil {
			return nil, fmt.Errorf("OAuth 토큰 파싱 실패: %w", err)
		}
		oauthConfig := &oauth2.Config{ClientID: config.GoogleClientID(), ClientSecret: config.GoogleClientSecret(), Endpoint: google.Endpoint}
		source := &savingTokenSource{
			source: oauthConfig.TokenSource(context.Background(), &token),
			save: func(token *oauth2.Token) error {
				if len(onTokenRefresh) == 0 || onTokenRefresh[0] == nil {
					return nil
				}
				data, err := json.Marshal(token)
				if err != nil {
					return err
				}
				return onTokenRefresh[0](string(data))
			},
		}
		c.httpClient = oauth2.NewClient(context.Background(), source)
		c.url = googleCalDAVURL(c.url, acc.Username)
	}
	return c, nil
}

// savingTokenSource는 OAuth 토큰이 갱신되었을 때 키체인 저장 콜백을 실행한다.
type savingTokenSource struct {
	source oauth2.TokenSource
	save   func(*oauth2.Token) error
	mu     sync.Mutex
	last   string
}

func (s *savingTokenSource) Token() (*oauth2.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	token, err := s.source.Token()
	if err != nil {
		return nil, err
	}
	if token.AccessToken != s.last {
		s.last = token.AccessToken
		if err := s.save(token); err != nil {
			return nil, err
		}
	}
	return token, nil
}

// googleCalDAVURL은 Google CalDAV의 principal 시작점으로 URL을 보정한다.
// Google은 /caldav/v2 아래에 캘린더 ID와 /user 경로를 요구한다.
func googleCalDAVURL(base, username string) string {
	if !strings.Contains(base, "apidata.googleusercontent.com/caldav/v2") {
		return base
	}
	if strings.HasSuffix(base, "/user") || strings.HasSuffix(base, "/events") {
		return base
	}
	return strings.TrimRight(base, "/") + "/" + url.PathEscape(username) + "/user"
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
	resp, err := c.requestWithDepth("PROPFIND", c.url, []byte(body), "application/xml; charset=utf-8", "0")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 207 {
		return "", responseError(fmt.Sprintf("principal 조회 실패 (URL: %s)", c.url), resp)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	href := findPropertyHref(data, "current-user-principal")
	if href == "" {
		// principal을 못 찾으면 URL 자체를 principal로 간주.
		return c.url, nil
	}
	return c.resolveURL(href), nil
}

// findCalendarHomeSet은 PROPFIND로 calendar-home-set URL을 찾는다.
func (c *Client) findCalendarHomeSet(principalURL string) (string, error) {
	body := `<?xml version="1.0" encoding="utf-8"?>
<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav">
  <D:prop>
    <C:calendar-home-set/>
  </D:prop>
</D:propfind>`
	resp, err := c.requestWithDepth("PROPFIND", principalURL, []byte(body), "application/xml; charset=utf-8", "0")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 207 {
		return "", responseError("calendar-home-set 조회 실패", resp)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return principalURL, nil
	}
	href := findPropertyHref(data, "calendar-home-set")
	if href == "" {
		return principalURL, nil
	}
	return c.resolveURL(href), nil
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
	calendars := parseCalendarList(data, homeSetURL)
	if len(calendars) == 0 {
		return nil, fmt.Errorf("캘린더 목록 응답에서 캘린더를 찾지 못했습니다")
	}
	return calendars, nil
}

// parseCalendarList는 PROPFIND 응답에서 캘린더 목록을 추출한다.
func parseCalendarList(data []byte, baseURL string) []types.CalendarInfo {
	var multistatus davMultistatus
	if err := xml.Unmarshal(data, &multistatus); err != nil {
		return nil
	}

	var out []types.CalendarInfo
	for _, response := range multistatus.Responses {
		var prop davProp
		for _, propstat := range response.Propstats {
			prop = propstat.Prop
			break
		}
		href := strings.TrimSpace(response.Href)
		if href == "" {
			continue
		}
		// 캘린더 리소스 타입인지 확인.
		if prop.ResourceType.Calendar == nil {
			continue
		}
		out = append(out, types.CalendarInfo{
			Href:        href,
			Name:        strings.TrimSpace(prop.DisplayName),
			Color:       strings.TrimSpace(prop.CalendarColor),
			Description: strings.TrimSpace(prop.CalendarDescription),
		})
	}
	_ = baseURL
	return out
}

// parseReportResponse는 REPORT 응답에서 일정 목록을 추출한다.
func parseReportResponse(data []byte, calendarID string) ([]types.Event, error) {
	var multistatus davMultistatus
	if err := xml.Unmarshal(data, &multistatus); err != nil {
		return nil, fmt.Errorf("CalDAV REPORT XML 파싱 실패: %w", err)
	}

	var out []types.Event
	for _, response := range multistatus.Responses {
		var prop davProp
		for _, propstat := range response.Propstats {
			prop = propstat.Prop
			break
		}
		href := strings.TrimSpace(response.Href)
		etag := strings.TrimSpace(prop.ETag)
		calData := strings.TrimSpace(prop.CalendarData)
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

// davMultistatus는 CalDAV/DAV 응답의 네임스페이스 접두사와 무관하게
// response 요소를 읽기 위한 최소 XML 구조체다. encoding/xml은 태그의
// 접두사(D:, d:, C:)가 아니라 Local 이름을 기준으로 매칭한다.
type davMultistatus struct {
	Responses []davResponse `xml:"response"`
}

type davResponse struct {
	Href      string        `xml:"href"`
	Propstats []davPropstat `xml:"propstat"`
}

type davPropstat struct {
	Prop davProp `xml:"prop"`
}

type davProp struct {
	DisplayName         string          `xml:"displayname"`
	CalendarDescription string          `xml:"calendar-description"`
	CalendarColor       string          `xml:"calendar-color"`
	ResourceType        davResourceType `xml:"resourcetype"`
	ETag                string          `xml:"getetag"`
	CalendarData        string          `xml:"calendar-data"`
}

type davResourceType struct {
	Calendar *struct{} `xml:"calendar"`
}

// findPropertyHref는 지정한 DAV/CalDAV 속성 내부의 href를 반환한다.
// 응답 자체의 href와 속성 내부 href가 모두 존재하므로 문서 전체의 첫 href를
// 사용하면 principal URL 또는 캘린더 홈 URL을 잘못 선택할 수 있다.
func findPropertyHref(data []byte, property string) string {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := decoder.Token()
		if err != nil {
			return ""
		}
		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != property {
			continue
		}
		var value struct {
			Href string `xml:"href"`
		}
		if err := decoder.DecodeElement(&value, &start); err != nil {
			return ""
		}
		return strings.TrimSpace(value.Href)
	}
}

// request는 지정한 메서드/URL로 HTTP 요청을 보낸다.
func (c *Client) request(method, url string, body []byte, contentType string) (*http.Response, error) {
	return c.requestWithDepth(method, url, body, contentType, "")
}

// requestWithDepth는 DAV 요청에 필요한 Depth 헤더를 설정해 HTTP 요청을 보낸다.
func (c *Client) requestWithDepth(method, url string, body []byte, contentType, depth string) (*http.Response, error) {
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	c.setAuth(req)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if depth != "" {
		req.Header.Set("Depth", depth)
	}
	return c.httpClient.Do(req)
}

// responseError는 외부 DAV 서버가 반환한 상태 코드와 오류 본문을 함께 보존한다.
// Google CalDAV는 403 응답 본문에 scope/권한 관련 원인을 포함할 수 있다.
func responseError(prefix string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	detail := strings.TrimSpace(string(body))
	if detail == "" {
		return fmt.Errorf("%s: HTTP %d", prefix, resp.StatusCode)
	}
	return fmt.Errorf("%s: HTTP %d: %s", prefix, resp.StatusCode, detail)
}

// setAuth는 Basic 인증 헤더를 설정한다.
func (c *Client) setAuth(req *http.Request) {
	if c.authType == account.AuthOAuth2 {
		return
	}
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
