package consumer

import (
	"encoding/json"
	"testing"
)

func TestRecallBotWebhook_Unmarshal(t *testing.T) {
	raw := `{"event":"bot.status_change","data":{"bot_id":"bot_123","status":"in_call_recording","updated_at":"2026-04-05T10:05:00Z","data":{"updated_at":"2026-04-05T10:05:00Z"}}}`

	var webhook recallBotWebhook
	if err := json.Unmarshal([]byte(raw), &webhook); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if webhook.Event != "bot.status_change" {
		t.Errorf("expected event bot.status_change, got %s", webhook.Event)
	}
	if webhook.Data.BotID != "bot_123" {
		t.Errorf("expected bot_id bot_123, got %s", webhook.Data.BotID)
	}
	if webhook.Data.Status != "in_call_recording" {
		t.Errorf("expected status in_call_recording, got %s", webhook.Data.Status)
	}
}
