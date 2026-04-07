package consumer

import (
	"encoding/json"
	"testing"
)

func TestRecallTranscriptWebhook_UnmarshalDone(t *testing.T) {
	raw := `{"event":"transcript.done","data":{"bot_id":"bot_123","transcript_id":"tr_456"}}`

	var webhook recallTranscriptWebhook
	if err := json.Unmarshal([]byte(raw), &webhook); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if webhook.Event != "transcript.done" {
		t.Errorf("expected event transcript.done, got %s", webhook.Event)
	}
	if webhook.Data.BotID != "bot_123" {
		t.Errorf("expected bot_id bot_123, got %s", webhook.Data.BotID)
	}
	if webhook.Data.TranscriptID != "tr_456" {
		t.Errorf("expected transcript_id tr_456, got %s", webhook.Data.TranscriptID)
	}
}

func TestRecallTranscriptWebhook_UnmarshalFailed(t *testing.T) {
	raw := `{"event":"transcript.failed","data":{"bot_id":"bot_789","failure_sub_code":"language_not_supported"}}`

	var webhook recallTranscriptWebhook
	if err := json.Unmarshal([]byte(raw), &webhook); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if webhook.Event != "transcript.failed" {
		t.Errorf("expected event transcript.failed, got %s", webhook.Event)
	}
	if webhook.Data.FailureSubCode != "language_not_supported" {
		t.Errorf("expected failure_sub_code language_not_supported, got %s", webhook.Data.FailureSubCode)
	}
}
