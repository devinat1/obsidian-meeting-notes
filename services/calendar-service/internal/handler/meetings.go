// services/calendar-service/internal/handler/meetings.go
package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/store"
)

type MeetingsHandler struct {
	meetingStore *store.MeetingStore
}

func NewMeetingsHandler(meetingStore *store.MeetingStore) *MeetingsHandler {
	return &MeetingsHandler{meetingStore: meetingStore}
}

// ServeHTTP handles GET /internal/calendar/meetings?user_id=<id>
func (h *MeetingsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		http.Error(w, `{"error":"user_id query parameter is required."}`, http.StatusBadRequest)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		http.Error(w, `{"error":"user_id must be an integer."}`, http.StatusBadRequest)
		return
	}

	now := time.Now()
	meetings, err := h.meetingStore.GetUpcomingByUserID(r.Context(), userID, now, now.Add(24*time.Hour))
	if err != nil {
		slog.Error("Failed to get upcoming meetings.", "user_id", userID, "error", err)
		http.Error(w, `{"error":"Failed to retrieve meetings."}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"meetings": meetings})
}
