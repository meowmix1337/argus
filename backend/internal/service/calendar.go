package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"

	"github.com/meowmix1337/argus/backend/internal/httpclient"
	"github.com/meowmix1337/argus/backend/internal/model"
)

const calendarCacheTTL = 15 * time.Minute

var calendarColors = []string{
	"#6366f1", "#f59e0b", "#10b981", "#ec4899", "#8b5cf6", "#14b8a6",
}

// CalendarService fetches and parses an ICS/iCal feed URL.
type CalendarService struct {
	httpClient          httpclient.HTTPClient
	cache               *CacheService
	loc                 *time.Location
	userSettingsService *UserSettingsService
}

// NewCalendarService creates a new CalendarService.
func NewCalendarService(httpClient httpclient.HTTPClient, cache *CacheService, loc *time.Location, userSettingService *UserSettingsService) *CalendarService {
	if loc == nil {
		loc = time.Local
	}
	return &CalendarService{
		httpClient:          httpClient,
		cache:               cache,
		loc:                 loc,
		userSettingsService: userSettingService,
	}
}

// Fetch returns today's calendar events sorted by start time.
// Returns an error (card shows unavailable state) if no ICS URL is configured.
func (s *CalendarService) Fetch(ctx context.Context, userID string) ([]model.CalendarEvent, error) {
	cacheKey := fmt.Sprintf("calendar-%s", userID)
	if v, ok := s.cache.Get(cacheKey); ok {
		return v.([]model.CalendarEvent), nil
	}

	userSettings, err := s.userSettingsService.Get(ctx, userID)
	if err != nil {
		return nil, err
	}

	events, err := s.fetchAndParse(ctx, userSettings.CalendarICSURL)
	if err != nil {
		return nil, err
	}
	s.cache.Set(cacheKey, events, calendarCacheTTL)
	return events, nil
}

func (s *CalendarService) fetchAndParse(ctx context.Context, icsURL *string) ([]model.CalendarEvent, error) {
	if icsURL == nil {
		return []model.CalendarEvent{}, nil
	}
	body, err := s.httpClient.GetBytes(ctx, *icsURL, httpclient.WithHeader("User-Agent", "Argus/1.0"))
	if err != nil {
		return nil, fmt.Errorf("calendar: fetch ICS: %w", err)
	}

	cal, err := ics.ParseCalendar(strings.NewReader(string(body)))
	if err != nil {
		return nil, fmt.Errorf("calendar: parse ICS: %w", err)
	}

	return s.filterToday(cal), nil
}

// filterToday extracts events whose DTSTART falls on today in local time,
// skips cancelled events, and sorts by start time (all-day events first).
func (s *CalendarService) filterToday(cal *ics.Calendar) []model.CalendarEvent {
	now := time.Now().In(s.loc)
	todayYear, todayMonth, todayDay := now.Date()
	loc := s.loc

	type timedEvent struct {
		start  time.Time
		allDay bool
		event  model.CalendarEvent
	}

	var results []timedEvent
	colorIdx := 0

	for _, vevent := range cal.Events() {
		// Skip cancelled events.
		if p := vevent.GetProperty(ics.ComponentPropertyStatus); p != nil {
			if strings.EqualFold(p.Value, string(ics.ObjectStatusCancelled)) {
				continue
			}
		}

		title := "(No title)"
		if p := vevent.GetProperty(ics.ComponentPropertySummary); p != nil && p.Value != "" {
			title = p.Value
		}

		color := calendarColors[colorIdx%len(calendarColors)]

		dtStartProp := vevent.GetProperty(ics.ComponentPropertyDtStart)
		if dtStartProp == nil {
			continue
		}

		// Detect all-day event: DTSTART;VALUE=DATE has no time component.
		isAllDay := false
		if vals, ok := dtStartProp.ICalParameters["VALUE"]; ok {
			for _, v := range vals {
				if strings.EqualFold(v, "DATE") {
					isAllDay = true
					break
				}
			}
		}

		if isAllDay {
			startDate, err := vevent.GetAllDayStartAt()
			if err != nil {
				continue
			}
			y, m, d := startDate.Date()
			if y != todayYear || m != todayMonth || d != todayDay {
				continue
			}
			results = append(results, timedEvent{
				start:  startDate,
				allDay: true,
				event:  model.CalendarEvent{Time: "All Day", Title: title, Color: color, Duration: "all day"},
			})
		} else {
			// GetStartAt handles TZID params and UTC Z-suffix via embedded VTIMEZONE.
			startTime, err := vevent.GetStartAt()
			if err != nil {
				continue
			}
			startLocal := startTime.In(loc)
			y, m, d := startLocal.Date()
			if y != todayYear || m != todayMonth || d != todayDay {
				continue
			}
			duration := "?"
			if endTime, err := vevent.GetEndAt(); err == nil {
				duration = formatDuration(endTime.Sub(startTime))
			}
			results = append(results, timedEvent{
				start: startLocal,
				event: model.CalendarEvent{
					Time:     startTime.UTC().Format(time.RFC3339),
					Title:    title,
					Color:    color,
					Duration: duration,
				},
			})
		}
		colorIdx++
	}

	// All-day events first, then chronological.
	sort.Slice(results, func(i, j int) bool {
		if results[i].allDay != results[j].allDay {
			return results[i].allDay
		}
		return results[i].start.Before(results[j].start)
	})

	out := make([]model.CalendarEvent, len(results))
	for i, r := range results {
		out[i] = r.event
	}
	return out
}

// formatDuration converts a duration to a human-readable string: "45m", "2h", "1h 30m".
func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "?"
	}
	total := int(d.Minutes())
	h, m := total/60, total%60
	switch {
	case h == 0:
		return fmt.Sprintf("%dm", m)
	case m == 0:
		return fmt.Sprintf("%dh", h)
	default:
		return fmt.Sprintf("%dh %dm", h, m)
	}
}
