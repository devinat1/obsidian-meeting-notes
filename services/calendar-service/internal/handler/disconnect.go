// services/calendar-service/internal/handler/disconnect.go
package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/poller"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/store"
)

type DisconnectHandler struct {
	connectionStore *store.ConnectionStore
	meetingStore    *store.MeetingStore
	pollerManager   *poller.Manager
}

type NewDisconnectHandlerParams struct {
	ConnectionStore *store.ConnectionStore
	MeetingStore    *store.MeetingStore
	PollerManager   *poller.Manager
}

func NewDisconnectHandler(params NewDisconnectHandlerParams) *DisconnectHandler {
	return &DisconnectHandler{
		connectionStore: params.ConnectionStore,
		meetingStore:    params.MeetingStore,
		pollerManager:   params.PollerManager,
	}
}

// ServeHTTP handles DELETE /internal/calendar/disconnect?user_id=<id>
func (h *DisconnectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	// Stop the poller first.
	h.pollerManager.StopForUser(userID)

	// Delete tracked meetings.
	if err := h.meetingStore.DeleteByUserID(r.Context(), userID); err != nil {
		slog.Warn("Failed to delete tracked meetings during disconnect.", "user_id", userID, "error", err)
	}

	// Delete the calendar connection (and encrypted token).
	if err := h.connectionStore.DeleteByUserID(r.Context(), userID); err != nil {
		slog.Error("Failed to delete calendar connection.", "user_id", userID, "error", err)
		http.Error(w, `{"error":"Failed to disconnect calendar."}`, http.StatusInternalServerError)
		return
	}

	slog.Info("Calendar disconnected.", "user_id", userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "disconnected"})
}
