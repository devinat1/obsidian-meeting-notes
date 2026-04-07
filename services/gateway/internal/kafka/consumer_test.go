// services/gateway/internal/kafka/consumer_test.go
package kafka_test

import (
	"encoding/json"
	"testing"

	gatewaykafka "github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/kafka"
	"github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/sse"
)

func TestExtractUserID(t *testing.T) {
	tests := []struct {
		name     string
		payload  map[string]any
		expected int
		hasError bool
	}{
		{
			name:     "valid user_id",
			payload:  map[string]any{"user_id": float64(42)},
			expected: 42,
			hasError: false,
		},
		{
			name:     "missing user_id",
			payload:  map[string]any{"bot_id": "abc"},
			expected: 0,
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, _ := json.Marshal(tt.payload)
			userID, err := gatewaykafka.ExtractUserID(data)
			if tt.hasError && err == nil {
				t.Fatal("expected error")
			}
			if !tt.hasError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if userID != tt.expected {
				t.Fatalf("expected user_id %d, got %d", tt.expected, userID)
			}
		})
	}
}

func TestDispatchToHub(t *testing.T) {
	hub := sse.NewHub()
	ch := hub.Register(42)

	gatewaykafka.DispatchToHub(hub, 42, "bot.status", `{"bot_id":"abc","status":"recording"}`)

	select {
	case event := <-ch:
		if event.Type != "bot.status" {
			t.Fatalf("expected event type bot.status, got %s", event.Type)
		}
	default:
		t.Fatal("expected event on channel")
	}

	hub.Unregister(42)
}
