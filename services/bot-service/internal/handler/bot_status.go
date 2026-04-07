package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/model"
)

type BotStatusHandler struct {
	pool *pgxpool.Pool
}

type NewBotStatusHandlerParams struct {
	Pool *pgxpool.Pool
}

func NewBotStatusHandler(params NewBotStatusHandlerParams) *BotStatusHandler {
	return &BotStatusHandler{pool: params.Pool}
}

func (h *BotStatusHandler) GetBotStatus(w http.ResponseWriter, r *http.Request) {
	botID := chi.URLParam(r, "botId")
	if botID == "" {
		http.Error(w, "botId is required", http.StatusBadRequest)
		return
	}

	var resp model.BotStatusResponse
	err := h.pool.QueryRow(r.Context(),
		"SELECT user_id, bot_id, bot_status, meeting_title, meeting_url FROM bots WHERE bot_id = $1",
		botID,
	).Scan(&resp.UserID, &resp.BotID, &resp.BotStatus, &resp.MeetingTitle, &resp.MeetingURL)
	if err != nil {
		log.Printf("Bot not found for bot_id %s: %v.", botID, err)
		http.Error(w, "bot not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
