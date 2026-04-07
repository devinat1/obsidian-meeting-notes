// services/gateway/internal/sse/handler.go
package sse

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

type Handler struct {
	hub               *Hub
	userIDFromContext  func(ctx context.Context) int
	heartbeatInterval time.Duration
}

type NewHandlerParams struct {
	Hub               *Hub
	UserIDFromContext  func(ctx context.Context) int
	HeartbeatInterval time.Duration
}

func NewHandler(params NewHandlerParams) *Handler {
	interval := params.HeartbeatInterval
	if interval == 0 {
		interval = 30 * time.Second
	}
	return &Handler{
		hub:               params.Hub,
		userIDFromContext:  params.UserIDFromContext,
		heartbeatInterval: interval,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID := h.userIDFromContext(r.Context())
	if userID == 0 {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":"streaming not supported"}`, http.StatusInternalServerError)
		return
	}

	if h.hub.HasConnection(userID) {
		http.Error(w, `{"error":"concurrent SSE connection limit reached"}`, http.StatusTooManyRequests)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch := h.hub.Register(userID)
	defer h.hub.Unregister(userID)

	lastEventIDStr := r.Header.Get("Last-Event-ID")
	if lastEventIDStr == "" {
		lastEventIDStr = r.URL.Query().Get("last_event_id")
	}

	if lastEventIDStr != "" {
		lastEventID, err := strconv.ParseUint(lastEventIDStr, 10, 64)
		if err == nil {
			catchUpEvents := h.hub.CatchUp(userID, lastEventID)
			for _, event := range catchUpEvents {
				writeSSEEvent(w, event)
			}
			flusher.Flush()
		}
	}

	ticker := time.NewTicker(h.heartbeatInterval)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			log.Printf("SSE connection closed for user %d.", userID)
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			writeSSEEvent(w, event)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, "event: ping\ndata: {}\n\n")
			flusher.Flush()
		}
	}
}

func writeSSEEvent(w http.ResponseWriter, event Event) {
	fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", event.ID, event.Type, event.Data)
}
