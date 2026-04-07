// services/calendar-service/internal/handler/authurl.go
package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/model"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/oauth"
)

type AuthURLHandler struct {
	googleOAuth *oauth.GoogleOAuth
}

func NewAuthURLHandler(googleOAuth *oauth.GoogleOAuth) *AuthURLHandler {
	return &AuthURLHandler{googleOAuth: googleOAuth}
}

// ServeHTTP handles GET /internal/calendar/auth-url?user_id=<id>
func (h *AuthURLHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		http.Error(w, `{"error":"user_id query parameter is required."}`, http.StatusBadRequest)
		return
	}

	_, err := strconv.Atoi(userIDStr)
	if err != nil {
		http.Error(w, `{"error":"user_id must be an integer."}`, http.StatusBadRequest)
		return
	}

	// Use user_id as OAuth state parameter for callback correlation.
	authURL := h.googleOAuth.AuthURL(userIDStr)

	slog.Info("Generated OAuth auth URL.", "user_id", userIDStr)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(model.AuthURLResponse{URL: authURL})
}
