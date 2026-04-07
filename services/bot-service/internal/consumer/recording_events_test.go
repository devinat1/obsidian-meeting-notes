package consumer

import (
	"encoding/json"
	"testing"
)

func TestRecallRecordingWebhook_Unmarshal(t *testing.T) {
	raw := `{"event":"recording.done","data":{"bot_id":"bot_123","recording_id":"rec_456","status":"done","updated_at":"2026-04-05T10:30:00Z","data":{"updated_at":"2026-04-05T10:30:00Z"}}}`

	var webhook recallRecordingWebhook
	if err := json.Unmarshal([]byte(raw), &webhook); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if webhook.Data.RecordingID != "rec_456" {
		t.Errorf("expected recording_id rec_456, got %s", webhook.Data.RecordingID)
	}
	if webhook.Data.Status != "done" {
		t.Errorf("expected status done, got %s", webhook.Data.Status)
	}
}
