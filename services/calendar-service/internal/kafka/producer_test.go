// services/calendar-service/internal/kafka/producer_test.go
package kafka

import (
	"testing"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/model"
)

func TestMeetingUpcomingEvent_JSON(t *testing.T) {
	event := model.MeetingUpcomingEvent{
		UserID:          42,
		CalendarEventID: "event-abc-123",
		MeetingURL:      "https://meet.google.com/abc-defg-hij",
		Title:           "Design Review",
		StartTime:       time.Date(2026, 4, 5, 10, 0, 0, 0, time.UTC),
	}

	if event.UserID != 42 {
		t.Errorf("Expected UserID 42, got %d.", event.UserID)
	}
	if event.Title != "Design Review" {
		t.Errorf("Expected title 'Design Review', got %q.", event.Title)
	}
}

func TestMeetingCancelledEvent_JSON(t *testing.T) {
	event := model.MeetingCancelledEvent{
		UserID:          42,
		CalendarEventID: "event-abc-123",
		BotID:           "bot-xyz-789",
	}

	if event.BotID != "bot-xyz-789" {
		t.Errorf("Expected BotID, got %q.", event.BotID)
	}
}

func TestTopicConstants(t *testing.T) {
	if TopicMeetingUpcoming != "meeting.upcoming" {
		t.Errorf("Unexpected topic name: %q.", TopicMeetingUpcoming)
	}
	if TopicMeetingCancelled != "meeting.cancelled" {
		t.Errorf("Unexpected topic name: %q.", TopicMeetingCancelled)
	}
}
