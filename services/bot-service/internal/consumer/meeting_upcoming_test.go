package consumer

import (
	"encoding/json"
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/model"
)

func TestMeetingUpcomingEvent_Unmarshal(t *testing.T) {
	raw := `{"user_id":1,"calendar_event_id":"evt_abc","meeting_url":"https://meet.google.com/abc","title":"Standup","start_time":"2026-04-05T10:00:00Z"}`

	var event model.MeetingUpcomingEvent
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if event.UserID != 1 {
		t.Errorf("expected user_id 1, got %d", event.UserID)
	}
	if event.CalendarEventID != "evt_abc" {
		t.Errorf("expected calendar_event_id evt_abc, got %s", event.CalendarEventID)
	}
	if event.MeetingURL != "https://meet.google.com/abc" {
		t.Errorf("expected meeting_url https://meet.google.com/abc, got %s", event.MeetingURL)
	}
	if event.Title != "Standup" {
		t.Errorf("expected title Standup, got %s", event.Title)
	}
}
