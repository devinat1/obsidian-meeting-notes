// services/calendar-service/internal/calendar/client_test.go
package calendar

import (
	"testing"
	"time"

	calendarapi "google.golang.org/api/calendar/v3"
)

func TestParseEventTime_DateTime(t *testing.T) {
	edt := &calendarapi.EventDateTime{
		DateTime: "2026-04-05T10:00:00-05:00",
	}

	parsed, err := parseEventTime(edt)
	if err != nil {
		t.Fatalf("Unexpected error: %v.", err)
	}

	if parsed.Hour() != 10 {
		t.Errorf("Expected hour 10, got %d.", parsed.Hour())
	}
}

func TestParseEventTime_DateOnly(t *testing.T) {
	edt := &calendarapi.EventDateTime{
		Date: "2026-04-05",
	}

	parsed, err := parseEventTime(edt)
	if err != nil {
		t.Fatalf("Unexpected error: %v.", err)
	}

	if parsed.Day() != 5 {
		t.Errorf("Expected day 5, got %d.", parsed.Day())
	}
}

func TestParseEventTime_Nil(t *testing.T) {
	_, err := parseEventTime(nil)
	if err == nil {
		t.Error("Expected error for nil EventDateTime.")
	}
}

func TestExtractAttendees(t *testing.T) {
	gcalAttendees := []*calendarapi.EventAttendee{
		{Email: "alice@example.com", DisplayName: "Alice Smith", Self: false},
		{Email: "bob@example.com", DisplayName: "Bob Jones", Self: true},
	}

	attendees := extractAttendees(gcalAttendees)
	if len(attendees) != 2 {
		t.Fatalf("Expected 2 attendees, got %d.", len(attendees))
	}

	if attendees[0].Email != "alice@example.com" {
		t.Errorf("Expected alice email, got %q.", attendees[0].Email)
	}
	if attendees[1].Self != true {
		t.Error("Expected bob to be marked as self.")
	}
}

func TestExtractAttendees_Empty(t *testing.T) {
	attendees := extractAttendees(nil)
	if len(attendees) != 0 {
		t.Errorf("Expected 0 attendees for nil input, got %d.", len(attendees))
	}
}

func TestFetchEventsParams_TimeRange(t *testing.T) {
	now := time.Now()
	params := FetchEventsParams{
		TimeMin: now,
		TimeMax: now.Add(2 * time.Hour),
	}

	duration := params.TimeMax.Sub(params.TimeMin)
	if duration != 2*time.Hour {
		t.Errorf("Expected 2h range, got %v.", duration)
	}
}
