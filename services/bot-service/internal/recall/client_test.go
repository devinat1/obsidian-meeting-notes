package recall

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCreateBot_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/bot/" {
			t.Errorf("expected /bot/, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Token test-key" {
			t.Errorf("unexpected Authorization header: %s", r.Header.Get("Authorization"))
		}

		var reqBody CreateBotRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if reqBody.MeetingURL != "https://meet.google.com/abc-defg-hij" {
			t.Errorf("unexpected meeting_url: %s", reqBody.MeetingURL)
		}
		if reqBody.TranscriptConfig.Provider != "recallai_streaming" {
			t.Errorf("unexpected transcript provider: %s", reqBody.TranscriptConfig.Provider)
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(CreateBotResponse{ID: "bot_123"})
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		apiKey:     "test-key",
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	resp, err := client.CreateBot(context.Background(), CreateBotParams{
		MeetingURL: "https://meet.google.com/abc-defg-hij",
		BotName:    "Meeting Notes Bot",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resp.ID != "bot_123" {
		t.Errorf("expected bot ID bot_123, got %s", resp.ID)
	}
}

func TestCreateBot_429Retry(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(CreateBotResponse{ID: "bot_456"})
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		apiKey:     "test-key",
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	resp, err := client.CreateBot(context.Background(), CreateBotParams{
		MeetingURL: "https://meet.google.com/test",
	})
	if err != nil {
		t.Fatalf("expected no error after retry, got: %v", err)
	}
	if resp.ID != "bot_456" {
		t.Errorf("expected bot ID bot_456, got %s", resp.ID)
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls (1 retry), got %d", callCount)
	}
}

func TestDeleteBot_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/bot/bot_789/" {
			t.Errorf("expected /bot/bot_789/, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		apiKey:     "test-key",
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	err := client.DeleteBot(context.Background(), DeleteBotParams{BotID: "bot_789"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		header   string
		expected time.Duration
	}{
		{"5", 5 * time.Second},
		{"", defaultRetryAfter429},
		{"invalid", defaultRetryAfter429},
		{"0", 0},
	}

	for _, tt := range tests {
		got := parseRetryAfter(tt.header)
		if got != tt.expected {
			t.Errorf("parseRetryAfter(%q) = %v, want %v", tt.header, got, tt.expected)
		}
	}
}
