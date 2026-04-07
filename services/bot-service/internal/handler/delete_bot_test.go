package handler

import (
	"testing"
)

func TestDeleteBotHandler_NewCreatesHandler(t *testing.T) {
	handler := NewDeleteBotHandler(NewDeleteBotHandlerParams{Pool: nil})
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}
