// services/gateway/internal/sse/handler_test.go
package sse_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/sse"
)

type mockUserIDContextKey string

const testUserIDKey mockUserIDContextKey = "userID"

func TestSSEHandler_StreamsEvents(t *testing.T) {
	hub := sse.NewHub()
	handler := sse.NewHandler(sse.NewHandlerParams{
		Hub:              hub,
		UserIDFromContext: func(ctx context.Context) int { return 42 },
		HeartbeatInterval: 100 * time.Millisecond,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/events/stream", nil)
	w := httptest.NewRecorder()

	ctx, cancel := context.WithTimeout(req.Context(), 300*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	go func() {
		time.Sleep(50 * time.Millisecond)
		hub.Send(42, sse.Event{Type: "bot.status", Data: `{"status":"recording"}`})
	}()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "event: bot.status") {
		t.Fatalf("expected SSE event in body, got: %s", body)
	}
	if !strings.Contains(body, `data: {"status":"recording"}`) {
		t.Fatalf("expected data in body, got: %s", body)
	}
}

func TestSSEHandler_SendsHeartbeat(t *testing.T) {
	hub := sse.NewHub()
	handler := sse.NewHandler(sse.NewHandlerParams{
		Hub:              hub,
		UserIDFromContext: func(ctx context.Context) int { return 42 },
		HeartbeatInterval: 50 * time.Millisecond,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/events/stream", nil)
	w := httptest.NewRecorder()

	ctx, cancel := context.WithTimeout(req.Context(), 200*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "event: ping") {
		t.Fatalf("expected heartbeat ping in body, got: %s", body)
	}
}
