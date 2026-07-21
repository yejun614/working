// Package ical 은 iCalendar(RFC 5545) 데이터와 types.Event 사이의
// 변환을 담당한다. CalDAV 서버와의 통신 시 VEVENT 블록을 파싱/직렬화한다.
package ical

import (
	"fmt"
	"strings"
	"time"

	"github.com/emersion/go-ical"

	"working/internal/modules/calendar/types"
)

// ParseCalendar는 iCalendar 데이터를 파싱해 일정 목록을 반환한다.
// VEVENT 컴포넌트만 추출하며, VTIMEZONE 등은 무시한다.
func ParseCalendar(data []byte, calendarID string) ([]types.Event, error) {
	cal, err := ical.NewDecoder(strings.NewReader(string(data))).Decode()
	if err != nil {
		return nil, fmt.Errorf("iCalendar 파싱 실패: %w", err)
	}
	return eventsFromCalendar(cal, calendarID), nil
}

// ParseObject는 단일 iCalendar 객체를 파싱해 일정 하나를 반환한다.
func ParseObject(data []byte, calendarID, href, etag string) (*types.Event, error) {
	events, err := ParseCalendar(data, calendarID)
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, fmt.Errorf("VEVENT가 없습니다")
	}
	ev := events[0]
	ev.Href = href
	ev.ETag = etag
	return &ev, nil
}

// eventsFromCalendar는 ical.Calendar에서 VEVENT를 추출해
// types.Event 슬라이스로 변환한다.
func eventsFromCalendar(cal *ical.Calendar, calendarID string) []types.Event {
	var out []types.Event
	for _, child := range cal.Children {
		if child.Name != "VEVENT" {
			continue
		}
		ev, err := eventFromComponent(child, calendarID)
		if err != nil {
			continue
		}
		out = append(out, *ev)
	}
	return out
}

// eventFromComponent는 VEVENT 컴포넌트를 types.Event로 변환한다.
func eventFromComponent(c *ical.Component, calendarID string) (*types.Event, error) {
	ev := &types.Event{CalendarID: calendarID}

	if v, err := c.Props.Text("UID"); err == nil {
		ev.UID = v
	}
	if v, err := c.Props.Text("SUMMARY"); err == nil {
		ev.Title = v
	}
	if v, err := c.Props.Text("DESCRIPTION"); err == nil {
		ev.Description = v
	}
	if v, err := c.Props.Text("LOCATION"); err == nil {
		ev.Location = v
	}
	if prop := c.Props.Get("ORGANIZER"); prop != nil {
		ev.Organizer = stripMailto(prop.Value)
	}
	if prop := c.Props.Get("RRULE"); prop != nil {
		ev.RecurrenceRule = prop.Value
	}

	if prop := c.Props.Get("DTSTART"); prop != nil {
		start, allDay, err := parseTime(prop)
		if err != nil {
			return nil, err
		}
		ev.Start = start
		ev.AllDay = allDay
	}
	if prop := c.Props.Get("DTEND"); prop != nil {
		end, _, err := parseTime(prop)
		if err != nil {
			return nil, err
		}
		ev.End = end
	}

	// 참석자: Props.Values("ATTENDEE")로 여러 개 가져온다.
	for _, prop := range c.Props.Values("ATTENDEE") {
		ev.Attendees = append(ev.Attendees, stripMailto(prop.Value))
	}
	return ev, nil
}

// parseTime은 iCalendar DATETIME/DATE 속성을 파싱해
// RFC3339 문자열과 종일 여부를 반환한다.
func parseTime(prop *ical.Prop) (string, bool, error) {
	val := prop.Value
	// VALUE=DATE 인 경우(YYYYMMDD 형식)는 종일 일정.
	if len(val) == 8 && strings.IndexByte(val, 'T') < 0 {
		t, err := time.Parse("20060102", val)
		if err != nil {
			return "", false, err
		}
		return t.Format(time.RFC3339), true, nil
	}
	// 일반 DATETIME: Prop.DateTime 메서드 사용.
	// 단, Prop.DateTime은 tzid 파라미터가 있을 때 로케이션이 필요하므로
	// 먼저 tzid가 없는지 확인하고, 있으면 로컬 시간으로 해석한다.
	loc := time.UTC
	if tzid := prop.Params.Get("TZID"); tzid != "" {
		if l, err := time.LoadLocation(tzid); err == nil {
			loc = l
		}
	}
	t, err := prop.DateTime(loc)
	if err != nil {
		return "", false, fmt.Errorf("시각 파싱 실패(%q): %w", val, err)
	}
	return t.Format(time.RFC3339), false, nil
}

// stripMailto는 "mailto:foo@bar.com" 형식의 값을 순수 주소로 변환한다.
func stripMailto(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "mailto:")
	v = strings.TrimPrefix(v, "MAILTO:")
	return v
}

// SerializeEvent는 단일 일정을 iCalendar 문자열로 직렬화한다.
// CalDAV PUT 요청 본문으로 사용된다.
func SerializeEvent(ev *types.Event) (string, error) {
	cal := ical.NewCalendar()
	// PRODID/VERSION 필수.
	cal.Props.SetText("PRODID", "-//working//calendar module//KR")
	cal.Props.SetText("VERSION", "2.0")

	vevent := ical.NewComponent("VEVENT")
	vevent.Props.SetText("UID", ev.UID)
	vevent.Props.SetText("SUMMARY", ev.Title)
	if ev.Description != "" {
		vevent.Props.SetText("DESCRIPTION", ev.Description)
	}
	if ev.Location != "" {
		vevent.Props.SetText("LOCATION", ev.Location)
	}
	if ev.Organizer != "" {
		vevent.Props.SetText("ORGANIZER", "mailto:"+ev.Organizer)
	}
	if ev.RecurrenceRule != "" {
		prop := ical.NewProp("RRULE")
		prop.Value = ev.RecurrenceRule
		vevent.Props.Set(prop)
	}

	start, err := time.Parse(time.RFC3339, ev.Start)
	if err != nil {
		return "", fmt.Errorf("시작 시각 파싱 실패: %w", err)
	}
	end, err := time.Parse(time.RFC3339, ev.End)
	if err != nil {
		return "", fmt.Errorf("종료 시각 파싱 실패: %w", err)
	}

	if ev.AllDay {
		ds := start.Format("20060102")
		de := end.Format("20060102")
		dsProp := ical.NewProp("DTSTART")
		dsProp.Value = ds
		dsProp.Params = ical.Params{"VALUE": []string{"DATE"}}
		vevent.Props.Set(dsProp)
		deProp := ical.NewProp("DTEND")
		deProp.Value = de
		deProp.Params = ical.Params{"VALUE": []string{"DATE"}}
		vevent.Props.Set(deProp)
	} else {
		vevent.Props.SetDateTime("DTSTART", start.UTC())
		vevent.Props.SetDateTime("DTEND", end.UTC())
	}
	vevent.Props.SetDateTime("DTSTAMP", time.Now().UTC())

	for _, a := range ev.Attendees {
		vevent.Props.SetText("ATTENDEE", "mailto:"+a)
	}

	cal.Children = append(cal.Children, vevent)

	var sb strings.Builder
	enc := ical.NewEncoder(&sb)
	if err := enc.Encode(cal); err != nil {
		return "", fmt.Errorf("iCalendar 직렬화 실패: %w", err)
	}
	return sb.String(), nil
}
