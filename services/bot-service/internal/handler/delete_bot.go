package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	kafkago "github.com/segmentio/kafka-go"

	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/model"
	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/recall"
)

type DeleteBotHandler struct {
	pool         *pgxpool.Pool
	recallClient *recall.Client
	statusWriter *kafkago.Writer
}

type NewDeleteBotHandlerParams struct {
	Pool         *pgxpool.Pool
	RecallClient *recall.Client
	StatusWriter *kafkago.Writer
}

func NewDeleteBotHandler(params NewDeleteBotHandlerParams) *DeleteBotHandler {
	return &DeleteBotHandler{
		pool:         params.Pool,
		recallClient: params.RecallClient,
		statusWriter: params.StatusWriter,
	}
}

func (h *DeleteBotHandler) DeleteBot(w http.ResponseWriter, r *http.Request) {
	botID := chi.URLParam(r, "botId")
	if botID == "" {
		http.Error(w, "botId is required", http.StatusBadRequest)
		return
	}

	var bot model.Bot
	err := h.pool.QueryRow(r.Context(),
		"SELECT id, user_id, bot_id, meeting_title, bot_status FROM bots WHERE bot_id = $1",
		botID,
	).Scan(&bot.ID, &bot.UserID, &bot.BotID, &bot.MeetingTitle, &bot.BotStatus)
	if err != nil {
		log.Printf("Bot not found for bot_id %s: %v.", botID, err)
		http.Error(w, "bot not found", http.StatusNotFound)
		return
	}

	if model.TerminalBotStatuses[bot.BotStatus] {
		http.Error(w, "bot already in terminal state", http.StatusConflict)
		return
	}

	if err := h.recallClient.DeleteBot(r.Context(), recall.DeleteBotParams{BotID: botID}); err != nil {
		log.Printf("Failed to delete Recall bot %s: %v.", botID, err)
	}

	_, err = h.pool.Exec(r.Context(),
		"UPDATE bots SET bot_status = 'cancelled', updated_at = NOW() WHERE id = $1",
		bot.ID,
	)
	if err != nil {
		log.Printf("Failed to update bot %s to cancelled: %v.", botID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	statusEvent := model.BotStatusEvent{
		UserID:       bot.UserID,
		BotID:        botID,
		BotStatus:    "cancelled",
		MeetingTitle: bot.MeetingTitle,
	}
	statusBytes, _ := json.Marshal(statusEvent)
	h.statusWriter.WriteMessages(context.Background(), kafkago.Message{
		Key:   []byte(strconv.Itoa(bot.UserID)),
		Value: statusBytes,
	})

	w.WriteHeader(http.StatusNoContent)
}
