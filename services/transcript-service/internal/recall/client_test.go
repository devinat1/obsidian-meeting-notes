package recall

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/services/transcript-service/internal/model"
)

func TestFetchTranscriptMetadata_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/transcript/tr_123/" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Token test-key" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(TranscriptMetadata{
			ID:          "tr_123",
			Status:      "done",
			DownloadURL: "https://example.com/transcript.json",
		})
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		apiKey:     "test-key",
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	metadata, err := client.FetchTranscriptMetadata(context.Background(), FetchTranscriptMetadataParams{
		TranscriptID: "tr_123",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if metadata.ID != "tr_123" {
		t.Errorf("expected transcript ID tr_123, got %s", metadata.ID)
	}
	if metadata.DownloadURL != "https://example.com/transcript.json" {
		t.Errorf("unexpected download URL: %s", metadata.DownloadURL)
	}
}

func TestDownloadTranscript_Success(t *testing.T) {
	segments := []model.RawSegment{
		{
			Speaker: "Alice",
			Words: []model.RawWord{
				{Text: "Hello", StartTime: 0.0, EndTime: 0.5},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(segments)
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		apiKey:     "test-key",
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	result, err := client.DownloadTranscript(context.Background(), DownloadTranscriptParams{
		DownloadURL: server.URL + "/download",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(result))
	}
	if result[0].Speaker != "Alice" {
		t.Errorf("expected speaker Alice, got %s", result[0].Speaker)
	}
}
