// services/calendar-service/internal/handler/callback.go
package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/model"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/oauth"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/poller"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/store"
)

type CallbackHandler struct {
	googleOAuth          *oauth.GoogleOAuth
	connectionStore      *store.ConnectionStore
	pollerManager        *poller.Manager
	defaultPollInterval  int
	defaultBotJoinBefore int
}

type NewCallbackHandlerParams struct {
	GoogleOAuth          *oauth.GoogleOAuth
	ConnectionStore      *store.ConnectionStore
	PollerManager        *poller.Manager
	DefaultPollInterval  int
	DefaultBotJoinBefore int
}

func NewCallbackHandler(params NewCallbackHandlerParams) *CallbackHandler {
	return &CallbackHandler{
		googleOAuth:          params.GoogleOAuth,
		connectionStore:      params.ConnectionStore,
		pollerManager:        params.PollerManager,
		defaultPollInterval:  params.DefaultPollInterval,
		defaultBotJoinBefore: params.DefaultBotJoinBefore,
	}
}

// ServeHTTP handles POST /internal/calendar/callback with JSON body {code, user_id}.
func (h *CallbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req model.CallbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request body."}`, http.StatusBadRequest)
		return
	}

	if req.Code == "" {
		http.Error(w, `{"error":"code is required."}`, http.StatusBadRequest)
		return
	}
	if req.UserID == 0 {
		http.Error(w, `{"error":"user_id is required."}`, http.StatusBadRequest)
		return
	}

	// Exchange the authorization code for tokens.
	token, err := h.googleOAuth.Exchange(r.Context(), req.Code)
	if err != nil {
		slog.Error("Failed to exchange OAuth code.", "user_id", req.UserID, "error", err)
		http.Error(w, `{"error":"Failed to exchange authorization code."}`, http.StatusBadGateway)
		return
	}

	// Store the encrypted refresh token.
	conn, err := h.connectionStore.Upsert(r.Context(), store.UpsertConnectionParams{
		UserID:               req.UserID,
		RefreshToken:         token.RefreshToken,
		PollIntervalMinutes:  h.defaultPollInterval,
		BotJoinBeforeMinutes: h.defaultBotJoinBefore,
	})
	if err != nil {
		slog.Error("Failed to store calendar connection.", "user_id", req.UserID, "error", err)
		http.Error(w, `{"error":"Failed to store calendar connection."}`, http.StatusInternalServerError)
		return
	}

	// Start the poller for this user.
	h.pollerManager.StartForUser(r.Context(), *conn)

	slog.Info("Calendar connected and poller started.", "user_id", req.UserID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "connected"})
}
