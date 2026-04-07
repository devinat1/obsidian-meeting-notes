package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	kafkago "github.com/segmentio/kafka-go"

	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/model"
)

type BotEventsConsumer struct {
	reader       *kafkago.Reader
	pool         *pgxpool.Pool
	statusWriter *kafkago.Writer
}

type NewBotEventsConsumerParams struct {
	Reader       *kafkago.Reader
	Pool         *pgxpool.Pool
	StatusWriter *kafkago.Writer
}

func NewBotEventsConsumer(params NewBotEventsConsumerParams) *BotEventsConsumer {
	return &BotEventsConsumer{
		reader:       params.Reader,
		pool:         params.Pool,
		statusWriter: params.StatusWriter,
	}
}

func (c *BotEventsConsumer) Start(ctx context.Context) {
	log.Println("Starting bot.events consumer.")
	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Println("Bot events consumer shutting down.")
				return
			}
			log.Printf("Error reading bot.events message: %v.", err)
			continue
		}

		if err := c.handleMessage(ctx, msg); err != nil {
			log.Printf("Error handling bot.events message: %v.", err)
		}
	}
}

type recallBotWebhook struct {
	Event string `json:"event"`
	Data  struct {
		BotID     string `json:"bot_id"`
		Status    string `json:"status"`
		UpdatedAt string `json:"updated_at"`
		Data      struct {
			UpdatedAt string `json:"updated_at"`
		} `json:"data"`
	} `json:"data"`
}

func (c *BotEventsConsumer) handleMessage(ctx context.Context, msg kafkago.Message) error {
	var webhook recallBotWebhook
	if err := json.Unmarshal(msg.Value, &webhook); err != nil {
		return fmt.Errorf("failed to unmarshal bot webhook: %w", err)
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

	var bot model.Bot
	var existingWebhookUpdatedAt *time.Time
	err := c.pool.QueryRow(ctx,
		"SELECT id, user_id, meeting_title, bot_status, bot_webhook_updated_at FROM bots WHERE bot_id = $1",
		botID,
	).Scan(&bot.ID, &bot.UserID, &bot.MeetingTitle, &bot.BotStatus, &existingWebhookUpdatedAt)
	if err != nil {
		return fmt.Errorf("bot not found for bot_id %s: %w", botID, err)
	}

	if existingWebhookUpdatedAt != nil && !webhookUpdatedAt.IsZero() && !webhookUpdatedAt.After(*existingWebhookUpdatedAt) {
		log.Printf("Stale bot event for bot %s, skipping (webhook %v <= stored %v).", botID, webhookUpdatedAt, existingWebhookUpdatedAt)
		return nil
	}

	newStatus := webhook.Data.Status
	if newStatus == "" {
		newStatus = webhook.Event
	}

	_, err = c.pool.Exec(ctx,
		"UPDATE bots SET bot_status = $1, bot_webhook_updated_at = $2, updated_at = NOW() WHERE id = $3",
		newStatus, webhookUpdatedAt, bot.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update bot status: %w", err)
	}

	statusEvent := model.BotStatusEvent{
		UserID:       bot.UserID,
		BotID:        botID,
		BotStatus:    newStatus,
		MeetingTitle: bot.MeetingTitle,
	}
	statusBytes, err := json.Marshal(statusEvent)
	if err != nil {
		return fmt.Errorf("failed to marshal bot.status event: %w", err)
	}

	err = c.statusWriter.WriteMessages(ctx, kafkago.Message{
		Key:   []byte(strconv.Itoa(bot.UserID)),
		Value: statusBytes,
	})
	if err != nil {
		return fmt.Errorf("failed to publish bot.status: %w", err)
	}

	log.Printf("Updated bot %s to status %s, published bot.status.", botID, newStatus)
	return nil
}
