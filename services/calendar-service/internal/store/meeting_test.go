// services/calendar-service/internal/store/meeting_test.go
package store

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/model"
)

func TestUpsertMeetingParams_AttendeesJSON(t *testing.T) {
	attendees := []model.Attendee{
		{Email: "alice@example.com", DisplayName: "Alice"},
		{Email: "bob@example.com", DisplayName: "Bob"},
	}

	data, err := json.Marshal(attendees)
	if err != nil {
		t.Fatalf("Failed to marshal attendees: %v.", err)
	}

	params := UpsertMeetingParams{
		UserID:          1,
		CalendarEventID: "event-123",
		Title:           "Design Review",
		StartTime:       time.Now().Add(30 * time.Minute),
		EndTime:         time.Now().Add(90 * time.Minute),
		MeetingURL:      "https://meet.google.com/abc-def-ghi",
		Attendees:       json.RawMessage(data),
	}

	if params.CalendarEventID != "event-123" {
		t.Error("CalendarEventID mismatch.")
	}

	var decoded []model.Attendee
	if err := json.Unmarshal(params.Attendees, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal attendees: %v.", err)
	}
	if len(decoded) != 2 {
		t.Errorf("Expected 2 attendees, got %d.", len(decoded))
	}
}
