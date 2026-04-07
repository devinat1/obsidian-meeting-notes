// services/calendar-service/internal/calendar/urlextract_test.go
package calendar

import (
	"testing"

	calendarapi "google.golang.org/api/calendar/v3"
)

func TestExtractMeetingURL_GoogleMeet(t *testing.T) {
	event := &calendarapi.Event{
		ConferenceData: &calendarapi.ConferenceData{
			EntryPoints: []*calendarapi.EntryPoint{
				{EntryPointType: "phone", Uri: "tel:+1234567890"},
				{EntryPointType: "video", Uri: "https://meet.google.com/abc-defg-hij"},
			},
		},
	}

	url := ExtractMeetingURL(event)
	if url != "https://meet.google.com/abc-defg-hij" {
		t.Errorf("Expected Google Meet URL, got %q.", url)
	}
}

func TestExtractMeetingURL_Zoom_InDescription(t *testing.T) {
	event := &calendarapi.Event{
		Description: "Join us at https://zoom.us/j/1234567890?pwd=abc123 for the meeting.",
	}

	url := ExtractMeetingURL(event)
	if url != "https://zoom.us/j/1234567890?pwd=abc123" {
		t.Errorf("Expected Zoom URL, got %q.", url)
	}
}

func TestExtractMeetingURL_Zoom_InLocation(t *testing.T) {
	event := &calendarapi.Event{
		Location: "https://company.zoom.us/j/9876543210",
	}

	url := ExtractMeetingURL(event)
	if url != "https://company.zoom.us/j/9876543210" {
		t.Errorf("Expected Zoom URL, got %q.", url)
	}
}

func TestExtractMeetingURL_Teams(t *testing.T) {
	event := &calendarapi.Event{
		Description: "Click here to join: https://teams.microsoft.com/l/meetup-join/19%3ameeting_abc123 and dial in.",
	}

	url := ExtractMeetingURL(event)
	expected := "https://teams.microsoft.com/l/meetup-join/19%3ameeting_abc123"
	if url != expected {
		t.Errorf("Expected Teams URL %q, got %q.", expected, url)
	}
}

func TestExtractMeetingURL_NoMeetingURL(t *testing.T) {
	event := &calendarapi.Event{
		Description: "Regular meeting with no video link.",
		Location:    "Conference Room B",
	}

	url := ExtractMeetingURL(event)
	if url != "" {
		t.Errorf("Expected empty string for no meeting URL, got %q.", url)
	}
}

func TestExtractMeetingURL_GoogleMeet_Priority(t *testing.T) {
	// When both Meet conferenceData and Zoom in description exist, prefer Meet.
	event := &calendarapi.Event{
		ConferenceData: &calendarapi.ConferenceData{
			EntryPoints: []*calendarapi.EntryPoint{
				{EntryPointType: "video", Uri: "https://meet.google.com/abc-defg-hij"},
			},
		},
		Description: "Also available at https://zoom.us/j/1234567890",
	}

	url := ExtractMeetingURL(event)
	if url != "https://meet.google.com/abc-defg-hij" {
		t.Errorf("Expected Google Meet to take priority, got %q.", url)
	}
}

func TestExtractMeetingURL_EmptyEvent(t *testing.T) {
	event := &calendarapi.Event{}
	url := ExtractMeetingURL(event)
	if url != "" {
		t.Errorf("Expected empty string for empty event, got %q.", url)
	}
}
