package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	kafkago "github.com/segmentio/kafka-go"
)

type RecordingEventsConsumer struct {
	reader *kafkago.Reader
	pool   *pgxpool.Pool
}

type NewRecordingEventsConsumerParams struct {
	Reader *kafkago.Reader
	Pool   *pgxpool.Pool
}

func NewRecordingEventsConsumer(params NewRecordingEventsConsumerParams) *RecordingEventsConsumer {
	return &RecordingEventsConsumer{
		reader: params.Reader,
		pool:   params.Pool,
	}
}

func (c *RecordingEventsConsumer) Start(ctx context.Context) {
	log.Println("Starting recording.events consumer.")
	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Println("Recording events consumer shutting down.")
				return
			}
			log.Printf("Error reading recording.events message: %v.", err)
			continue
		}

		if err := c.handleMessage(ctx, msg); err != nil {
			log.Printf("Error handling recording.events message: %v.", err)
		}
	}
}

type recallRecordingWebhook struct {
	Event string `json:"event"`
	Data  struct {
		BotID       string `json:"bot_id"`
		RecordingID string `json:"recording_id"`
		Status      string `json:"status"`
		UpdatedAt   string `json:"updated_at"`
		Data        struct {
			UpdatedAt string `json:"updated_at"`
		} `json:"data"`
	} `json:"data"`
}

func (c *RecordingEventsConsumer) handleMessage(ctx context.Context, msg kafkago.Message) error {
	var webhook recallRecordingWebhook
	if err := json.Unmarshal(msg.Value, &webhook); err != nil {
		return fmt.Errorf("failed to unmarshal recording webhook: %w", err)
	}

	botID := string(msg.Key)
	if botID == "" || botID == "unknown" {
		botID = webhook.Data.BotID
	}

	webhookUpdatedAtStr := webhook.Data.Data.UpdatedAt
	if webhookUpdatedAtStr == "" {
		webhookUpdatedAtStr = webhook.Data.UpdatedAt
	}

	var webhookUpdatedAt time.Time
	if webhookUpdatedAtStr != "" {
		parsed, err := time.Parse(time.RFC3339Nano, webhookUpdatedAtStr)
		if err == nil {
			webhookUpdatedAt = parsed
		}
	}

	var botDBID int
	var existingRecordingWebhookUpdatedAt *time.Time
	err := c.pool.QueryRow(ctx,
		"SELECT id, recording_webhook_updated_at FROM bots WHERE bot_id = $1",
		botID,
	).Scan(&botDBID, &existingRecordingWebhookUpdatedAt)
	if err != nil {
		return fmt.Errorf("bot not found for bot_id %s: %w", botID, err)
	}

	if existingRecordingWebhookUpdatedAt != nil && !webhookUpdatedAt.IsZero() && !webhookUpdatedAt.After(*existingRecordingWebhookUpdatedAt) {
		log.Printf("Stale recording event for bot %s, skipping.", botID)
		return nil
	}

	recordingID := webhook.Data.RecordingID
	recordingStatus := webhook.Data.Status
	if recordingStatus == "" {
		recordingStatus = webhook.Event
	}

	_, err = c.pool.Exec(ctx,
		"UPDATE bots SET recording_id = $1, recording_status = $2, recording_webhook_updated_at = $3, updated_at = NOW() WHERE id = $4",
		recordingID, recordingStatus, webhookUpdatedAt, botDBID,
	)
	if err != nil {
		return fmt.Errorf("failed to update recording status: %w", err)
	}

	log.Printf("Updated bot %s recording to status %s (recording_id: %s).", botID, recordingStatus, recordingID)
	return nil
}
