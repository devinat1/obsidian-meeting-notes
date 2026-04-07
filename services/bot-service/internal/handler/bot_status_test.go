package handler

import (
	"testing"
)

func TestBotStatusHandler_NewCreatesHandler(t *testing.T) {
	handler := NewBotStatusHandler(NewBotStatusHandlerParams{Pool: nil})
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}
