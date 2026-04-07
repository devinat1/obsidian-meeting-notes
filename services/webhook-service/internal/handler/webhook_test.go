package handler

import (
	"encoding/json"
	"testing"
)

func TestResolveTopicFromEvent(t *testing.T) {
	tests := []struct {
		event    string
		expected string
	}{
		{"bot.status_change", "bot.events"},
		{"bot.leave_call", "bot.events"},
		{"recording.started", "recording.events"},
		{"recording.done", "recording.events"},
		{"transcript.done", "transcript.events"},
		{"transcript.failed", "transcript.events"},
		{"unknown.event", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			got := resolveTopicFromEvent(tt.event)
			if got != tt.expected {
				t.Errorf("resolveTopicFromEvent(%q) = %q, want %q", tt.event, got, tt.expected)
			}
		})
	}
}

func TestExtractBotID(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		expected string
	}{
		{
			name:     "top-level bot_id",
			data:     `{"bot_id":"abc123","status":"in_call"}`,
			expected: "abc123",
		},
		{
			name:     "nested bot_id",
			data:     `{"data":{"bot_id":"def456"}}`,
			expected: "def456",
		},
		{
			name:     "no bot_id found",
			data:     `{"other":"field"}`,
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractBotID(json.RawMessage(tt.data))
			if got != tt.expected {
				t.Errorf("extractBotID(%s) = %q, want %q", tt.data, got, tt.expected)
			}
		})
	}
}
