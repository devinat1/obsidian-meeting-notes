// services/calendar-service/internal/handler/status.go
package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/model"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/store"
)

type StatusHandler struct {
	connectionStore *store.ConnectionStore
}

func NewStatusHandler(connectionStore *store.ConnectionStore) *StatusHandler {
	return &StatusHandler{connectionStore: connectionStore}
}

// ServeHTTP handles GET /internal/calendar/status?user_id=<id>
func (h *StatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	conn, err := h.connectionStore.GetByUserID(r.Context(), userID)
	if err != nil {
		slog.Error("Failed to check calendar status.", "user_id", userID, "error", err)
		http.Error(w, `{"error":"Failed to check status."}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(model.StatusResponse{Connected: conn != nil})
}
