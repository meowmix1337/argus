package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	ics "github.com/arran4/golang-ical"

	"github.com/meowmix1337/argus/backend/internal/model"
)

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{45 * time.Minute, "45m"},
		{2 * time.Hour, "2h"},
		{90 * time.Minute, "1h 30m"},
		{0, "?"},
		{-time.Minute, "?"},
		{30 * time.Minute, "30m"},
		{61 * time.Minute, "1h 1m"},
	}
	for _, tc := range cases {
		t.Run(tc.d.String(), func(t *testing.T) {
			if got := formatDuration(tc.d); got != tc.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tc.d, got, tc.want)
			}
		})
	}
}

func parseTestCalendar(t *testing.T, icsBody string) *ics.Calendar {
	t.Helper()
	cal, err := ics.ParseCalendar(strings.NewReader(icsBody))
	if err != nil {
		t.Fatalf("ParseCalendar: %v", err)
	}
	return cal
}

func todayUTC() time.Time {
	return time.Now().UTC()
}

func TestFilterToday_AllDayEvent(t *testing.T) {
	today := todayUTC().Format("20060102")
	icsBody := fmt.Sprintf(`BEGIN:VCALENDAR
BEGIN:VEVENT
SUMMARY:Team Standup
DTSTART;VALUE=DATE:%s
END:VEVENT
END:VCALENDAR`, today)

	svc := &CalendarService{loc: time.UTC}
	events := svc.filterToday(parseTestCalendar(t, icsBody))

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Time != "All Day" {
		t.Errorf("Time = %q, want %q", events[0].Time, "All Day")
	}
	if events[0].Title != "Team Standup" {
		t.Errorf("Title = %q, want %q", events[0].Title, "Team Standup")
	}
	if events[0].Duration != "all day" {
		t.Errorf("Duration = %q, want %q", events[0].Duration, "all day")
	}
}

func TestFilterToday_WrongDay_Excluded(t *testing.T) {
	yesterday := todayUTC().AddDate(0, 0, -1).Format("20060102")
	icsBody := fmt.Sprintf(`BEGIN:VCALENDAR
BEGIN:VEVENT
SUMMARY:Yesterday Event
DTSTART;VALUE=DATE:%s
END:VEVENT
END:VCALENDAR`, yesterday)

	svc := &CalendarService{loc: time.UTC}
	events := svc.filterToday(parseTestCalendar(t, icsBody))

	if len(events) != 0 {
		t.Errorf("expected 0 events for yesterday's date, got %d", len(events))
	}
}

func TestFilterToday_CancelledEvent_Excluded(t *testing.T) {
	today := todayUTC().Format("20060102")
	icsBody := fmt.Sprintf(`BEGIN:VCALENDAR
BEGIN:VEVENT
SUMMARY:Cancelled Meeting
STATUS:CANCELLED
DTSTART;VALUE=DATE:%s
END:VEVENT
END:VCALENDAR`, today)

	svc := &CalendarService{loc: time.UTC}
	events := svc.filterToday(parseTestCalendar(t, icsBody))

	if len(events) != 0 {
		t.Errorf("expected 0 events for cancelled event, got %d", len(events))
	}
}

func TestFilterToday_TimedEvent(t *testing.T) {
	now := todayUTC()
	start := time.Date(now.Year(), now.Month(), now.Day(), 10, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)

	icsBody := fmt.Sprintf(`BEGIN:VCALENDAR
BEGIN:VEVENT
SUMMARY:Morning Meeting
DTSTART:%s
DTEND:%s
END:VEVENT
END:VCALENDAR`, start.Format("20060102T150405Z"), end.Format("20060102T150405Z"))

	svc := &CalendarService{loc: time.UTC}
	events := svc.filterToday(parseTestCalendar(t, icsBody))

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Title != "Morning Meeting" {
		t.Errorf("Title = %q, want %q", events[0].Title, "Morning Meeting")
	}
	if events[0].Duration != "1h" {
		t.Errorf("Duration = %q, want %q", events[0].Duration, "1h")
	}
}

func TestFilterToday_NoTitle_DefaultsToNoTitle(t *testing.T) {
	today := todayUTC().Format("20060102")
	icsBody := fmt.Sprintf(`BEGIN:VCALENDAR
BEGIN:VEVENT
DTSTART;VALUE=DATE:%s
END:VEVENT
END:VCALENDAR`, today)

	svc := &CalendarService{loc: time.UTC}
	events := svc.filterToday(parseTestCalendar(t, icsBody))

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Title != "(No title)" {
		t.Errorf("Title = %q, want %q", events[0].Title, "(No title)")
	}
}

func TestFilterToday_SortOrder_AllDayFirst(t *testing.T) {
	now := todayUTC()
	// Timed event at 9am
	start := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	today := now.Format("20060102")

	icsBody := fmt.Sprintf(`BEGIN:VCALENDAR
BEGIN:VEVENT
SUMMARY:Morning Meeting
DTSTART:%s
DTEND:%s
END:VEVENT
BEGIN:VEVENT
SUMMARY:All Day Task
DTSTART;VALUE=DATE:%s
END:VEVENT
END:VCALENDAR`, start.Format("20060102T150405Z"), end.Format("20060102T150405Z"), today)

	svc := &CalendarService{loc: time.UTC}
	events := svc.filterToday(parseTestCalendar(t, icsBody))

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Time != "All Day" {
		t.Errorf("expected first event to be All Day, got %q", events[0].Time)
	}
	if events[1].Title != "Morning Meeting" {
		t.Errorf("expected second event to be Morning Meeting, got %q", events[1].Title)
	}
}

// ---- CalendarService.Fetch ----

func TestCalendarService_Fetch_CacheHit(t *testing.T) {
	cache := NewCacheService()
	cache.Set("calendar", []model.CalendarEvent{{Title: "Standup"}}, time.Minute)

	svc := NewCalendarService(
		&fakeHTTPClient{err: fmt.Errorf("HTTP must not be called on cache hit")},
		"https://example.com/cal.ics", cache, time.UTC,
	)
	events, err := svc.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(events) != 1 || events[0].Title != "Standup" {
		t.Errorf("expected cached event, got: %+v", events)
	}
}

func TestCalendarService_Fetch_NoURL_ReturnsError(t *testing.T) {
	svc := NewCalendarService(&fakeHTTPClient{}, "", NewCacheService(), time.UTC)
	if _, err := svc.Fetch(context.Background()); err == nil {
		t.Error("expected error when ICS URL is empty")
	}
}

func TestCalendarService_Fetch_HTTPError_Propagates(t *testing.T) {
	svc := NewCalendarService(
		&fakeHTTPClient{err: fmt.Errorf("connection refused")},
		"https://example.com/cal.ics", NewCacheService(), time.UTC,
	)
	if _, err := svc.Fetch(context.Background()); err == nil {
		t.Error("expected error on HTTP failure")
	}
}

func TestCalendarService_Fetch_PopulatesCache(t *testing.T) {
	today := time.Now().UTC().Format("20060102")
	icsContent := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nBEGIN:VEVENT\r\nSUMMARY:Test\r\nDTSTART;VALUE=DATE:" + today + "\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"

	cache := NewCacheService()
	svc := NewCalendarService(
		&fakeHTTPClient{rawBytes: []byte(icsContent)},
		"https://example.com/cal.ics", cache, time.UTC,
	)
	if _, err := svc.Fetch(context.Background()); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if _, ok := cache.Get("calendar"); !ok {
		t.Error("expected result to be cached after successful fetch")
	}
}

func TestFilterToday_MultipleTimedEvents_ChronologicalOrder(t *testing.T) {
	now := todayUTC()
	early := time.Date(now.Year(), now.Month(), now.Day(), 8, 0, 0, 0, time.UTC)
	late := time.Date(now.Year(), now.Month(), now.Day(), 14, 0, 0, 0, time.UTC)

	icsBody := fmt.Sprintf(`BEGIN:VCALENDAR
BEGIN:VEVENT
SUMMARY:Late Meeting
DTSTART:%s
DTEND:%s
END:VEVENT
BEGIN:VEVENT
SUMMARY:Early Meeting
DTSTART:%s
DTEND:%s
END:VEVENT
END:VCALENDAR`,
		late.Format("20060102T150405Z"), late.Add(time.Hour).Format("20060102T150405Z"),
		early.Format("20060102T150405Z"), early.Add(time.Hour).Format("20060102T150405Z"),
	)

	svc := &CalendarService{loc: time.UTC}
	events := svc.filterToday(parseTestCalendar(t, icsBody))

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Title != "Early Meeting" {
		t.Errorf("expected first event to be 'Early Meeting', got %q", events[0].Title)
	}
	if events[1].Title != "Late Meeting" {
		t.Errorf("expected second event to be 'Late Meeting', got %q", events[1].Title)
	}
}

// TestNewCalendarService_NilLoc verifies that a nil location falls back to time.Local.
func TestNewCalendarService_NilLoc(t *testing.T) {
	svc := NewCalendarService(&fakeHTTPClient{}, "https://example.com/cal.ics", NewCacheService(), nil)
	if svc == nil {
		t.Fatal("expected non-nil CalendarService when loc is nil")
	}
}
