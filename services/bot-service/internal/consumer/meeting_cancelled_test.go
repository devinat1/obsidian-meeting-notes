package consumer

import (
	"encoding/json"
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/model"
)

func TestMeetingCancelledEvent_Unmarshal(t *testing.T) {
	raw := `{"user_id":1,"calendar_event_id":"evt_abc","bot_id":"bot_123"}`

	var event model.MeetingCancelledEvent
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if event.UserID != 1 {
		t.Errorf("expected user_id 1, got %d", event.UserID)
	}
	if event.BotID != "bot_123" {
		t.Errorf("expected bot_id bot_123, got %s", event.BotID)
	}
}

func TestTerminalBotStatuses(t *testing.T) {
	if !model.TerminalBotStatuses["done"] {
		t.Error("expected done to be terminal")
	}
	if !model.TerminalBotStatuses["fatal"] {
		t.Error("expected fatal to be terminal")
	}
	if !model.TerminalBotStatuses["cancelled"] {
		t.Error("expected cancelled to be terminal")
	}
	if model.TerminalBotStatuses["in_call"] {
		t.Error("expected in_call to not be terminal")
	}
}
