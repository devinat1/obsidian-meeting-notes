package transcript

import (
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/transcript-service/internal/model"
)

func TestConvertToReadable_BasicSegments(t *testing.T) {
	segments := []model.RawSegment{
		{
			Speaker: "Alice",
			Words: []model.RawWord{
				{Text: "Hello", StartTime: 0.0, EndTime: 0.5},
				{Text: "everyone", StartTime: 0.5, EndTime: 1.0},
			},
		},
		{
			Speaker: "Bob",
			Words: []model.RawWord{
				{Text: "Hey", StartTime: 2.0, EndTime: 2.3},
				{Text: "Alice", StartTime: 2.3, EndTime: 2.6},
			},
		},
	}

	result := ConvertToReadable(ConvertToReadableParams{Segments: segments})

	if len(result) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(result))
	}
	if result[0].Speaker != "Alice" {
		t.Errorf("expected speaker Alice, got %s", result[0].Speaker)
	}
	if result[0].Timestamp != "0:00" {
		t.Errorf("expected timestamp 0:00, got %s", result[0].Timestamp)
	}
	if result[0].Text != "Hello everyone" {
		t.Errorf("expected text 'Hello everyone', got '%s'", result[0].Text)
	}
	if result[1].Speaker != "Bob" {
		t.Errorf("expected speaker Bob, got %s", result[1].Speaker)
	}
	if result[1].Timestamp != "0:02" {
		t.Errorf("expected timestamp 0:02, got %s", result[1].Timestamp)
	}
}

func TestConvertToReadable_MergeConsecutiveSpeakers(t *testing.T) {
	segments := []model.RawSegment{
		{
			Speaker: "Alice",
			Words:   []model.RawWord{{Text: "First", StartTime: 0.0, EndTime: 0.5}},
		},
		{
			Speaker: "Alice",
			Words:   []model.RawWord{{Text: "sentence", StartTime: 1.0, EndTime: 1.5}},
		},
		{
			Speaker: "Bob",
			Words:   []model.RawWord{{Text: "Response", StartTime: 3.0, EndTime: 3.5}},
		},
	}

	result := ConvertToReadable(ConvertToReadableParams{Segments: segments})

	if len(result) != 2 {
		t.Fatalf("expected 2 blocks (merged), got %d", len(result))
	}
	if result[0].Text != "First sentence" {
		t.Errorf("expected merged text 'First sentence', got '%s'", result[0].Text)
	}
}

func TestConvertToReadable_EmptySegments(t *testing.T) {
	segments := []model.RawSegment{
		{Speaker: "Alice", Words: []model.RawWord{}},
	}

	result := ConvertToReadable(ConvertToReadableParams{Segments: segments})

	if len(result) != 0 {
		t.Errorf("expected 0 blocks for empty words, got %d", len(result))
	}
}

func TestConvertToReadable_UnknownSpeaker(t *testing.T) {
	segments := []model.RawSegment{
		{
			Speaker: "",
			Words:   []model.RawWord{{Text: "Hello", StartTime: 0.0, EndTime: 0.5}},
		},
	}

	result := ConvertToReadable(ConvertToReadableParams{Segments: segments})

	if len(result) != 1 {
		t.Fatalf("expected 1 block, got %d", len(result))
	}
	if result[0].Speaker != "Unknown Speaker" {
		t.Errorf("expected 'Unknown Speaker', got '%s'", result[0].Speaker)
	}
}

func TestFormatTimestamp(t *testing.T) {
	tests := []struct {
		seconds  float64
		expected string
	}{
		{0.0, "0:00"},
		{5.5, "0:05"},
		{65.0, "1:05"},
		{3661.0, "61:01"},
	}

	for _, tt := range tests {
		got := formatTimestamp(tt.seconds)
		if got != tt.expected {
			t.Errorf("formatTimestamp(%v) = %q, want %q", tt.seconds, got, tt.expected)
		}
	}
}
