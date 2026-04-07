// services/gateway/internal/sse/hub_test.go
package sse_test

import (
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/sse"
)

func TestHub_RegisterAndSend(t *testing.T) {
	hub := sse.NewHub()

	ch := hub.Register(42)
	defer hub.Unregister(42)

	hub.Send(42, sse.Event{
		Type: "bot.status",
		Data: `{"bot_id":"abc","bot_status":"recording"}`,
	})

	select {
	case event := <-ch:
		if event.Type != "bot.status" {
			t.Fatalf("expected event type bot.status, got %s", event.Type)
		}
		if event.ID == 0 {
			t.Fatal("expected non-zero event ID")
		}
	default:
		t.Fatal("expected to receive event")
	}
}

func TestHub_BufferCatchUp(t *testing.T) {
	hub := sse.NewHub()

	ch := hub.Register(42)
	hub.Unregister(42)

	hub.Send(42, sse.Event{Type: "bot.status", Data: "event1"})
	hub.Send(42, sse.Event{Type: "bot.status", Data: "event2"})
	hub.Send(42, sse.Event{Type: "bot.status", Data: "event3"})

	events := hub.CatchUp(42, 1)
	if len(events) != 2 {
		t.Fatalf("expected 2 catch-up events, got %d", len(events))
	}
	if events[0].Data != "event2" {
		t.Fatalf("expected event2, got %s", events[0].Data)
	}
	if events[1].Data != "event3" {
		t.Fatalf("expected event3, got %s", events[1].Data)
	}

	_ = ch
}

func TestHub_UnregisterRemovesChannel(t *testing.T) {
	hub := sse.NewHub()

	hub.Register(42)
	hub.Unregister(42)

	if hub.HasConnection(42) {
		t.Fatal("expected no connection after unregister")
	}
}

func TestHub_BufferMaxSize(t *testing.T) {
	hub := sse.NewHub()

	for i := 0; i < 120; i++ {
		hub.Send(99, sse.Event{Type: "test", Data: "data"})
	}

	events := hub.CatchUp(99, 0)
	if len(events) != 100 {
		t.Fatalf("expected buffer capped at 100, got %d", len(events))
	}
}
