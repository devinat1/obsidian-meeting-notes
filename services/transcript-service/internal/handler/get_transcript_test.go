package handler

import (
	"testing"
)

func TestGetTranscriptHandler_NewCreatesHandler(t *testing.T) {
	handler := NewGetTranscriptHandler(NewGetTranscriptHandlerParams{Pool: nil})
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}
