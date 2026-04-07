package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/devinat1/obsidian-meeting-notes/services/transcript-service/internal/model"
)

type GetTranscriptHandler struct {
	pool *pgxpool.Pool
}

type NewGetTranscriptHandlerParams struct {
	Pool *pgxpool.Pool
}

func NewGetTranscriptHandler(params NewGetTranscriptHandlerParams) *GetTranscriptHandler {
	return &GetTranscriptHandler{pool: params.Pool}
}

func (h *GetTranscriptHandler) GetTranscript(w http.ResponseWriter, r *http.Request) {
	botID := chi.URLParam(r, "botId")
	if botID == "" {
		http.Error(w, "botId is required", http.StatusBadRequest)
		return
	}

	var t model.Transcript
	var rawJSON, readableJSON []byte
	err := h.pool.QueryRow(r.Context(),
		`SELECT id, bot_id, user_id, meeting_title, transcript_id, status, failure_sub_code, raw_transcript, readable_transcript, created_at
		 FROM transcripts WHERE bot_id = $1`,
		botID,
	).Scan(&t.ID, &t.BotID, &t.UserID, &t.MeetingTitle, &t.TranscriptID, &t.Status, &t.FailureSubCode, &rawJSON, &readableJSON, &t.CreatedAt)
	if err != nil {
		log.Printf("Transcript not found for bot_id %s: %v.", botID, err)
		http.Error(w, "transcript not found", http.StatusNotFound)
		return
	}

	if rawJSON != nil {
		var raw []model.RawSegment
		if err := json.Unmarshal(rawJSON, &raw); err == nil {
			t.RawTranscript = &raw
		}
	}
	if readableJSON != nil {
		var readable []model.ReadableBlock
		if err := json.Unmarshal(readableJSON, &readable); err == nil {
			t.ReadableTranscript = &readable
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}
