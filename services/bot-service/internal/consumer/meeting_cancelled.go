package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
	kafkago "github.com/segmentio/kafka-go"

	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/model"
	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/recall"
)

type MeetingCancelledConsumer struct {
	reader       *kafkago.Reader
	pool         *pgxpool.Pool
	recallClient *recall.Client
	statusWriter *kafkago.Writer
}

type NewMeetingCancelledConsumerParams struct {
	Reader       *kafkago.Reader
	Pool         *pgxpool.Pool
	RecallClient *recall.Client
	StatusWriter *kafkago.Writer
}

func NewMeetingCancelledConsumer(params NewMeetingCancelledConsumerParams) *MeetingCancelledConsumer {
	return &MeetingCancelledConsumer{
		reader:       params.Reader,
		pool:         params.Pool,
		recallClient: params.RecallClient,
		statusWriter: params.StatusWriter,
	}
}

func (c *MeetingCancelledConsumer) Start(ctx context.Context) {
	log.Println("Starting meeting.cancelled consumer.")
	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Println("Meeting cancelled consumer shutting down.")
				return
			}
			log.Printf("Error reading meeting.cancelled message: %v.", err)
			continue
		}

		if err := c.handleMessage(ctx, msg); err != nil {
			log.Printf("Error handling meeting.cancelled message: %v.", err)
		}
	}
}

func (c *MeetingCancelledConsumer) handleMessage(ctx context.Context, msg kafkago.Message) error {
	var event model.MeetingCancelledEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("failed to unmarshal meeting.cancelled event: %w", err)
	}

	log.Printf("Processing meeting.cancelled for user %d, calendar event %s.", event.UserID, event.CalendarEventID)

	var bot model.Bot
	err := c.pool.QueryRow(ctx,
		"SELECT id, bot_id, meeting_title, bot_status FROM bots WHERE user_id = $1 AND calendar_event_id = $2 ORDER BY created_at DESC LIMIT 1",
		event.UserID, event.CalendarEventID,
	).Scan(&bot.ID, &bot.BotID, &bot.MeetingTitle, &bot.BotStatus)
	if err != nil {
		log.Printf("No bot found for cancelled calendar event %s, skipping.", event.CalendarEventID)
		return nil
	}

	if model.TerminalBotStatuses[bot.BotStatus] {
		log.Printf("Bot %s already in terminal state %s, skipping cancellation.", bot.BotID, bot.BotStatus)
		return nil
	}

	if err := c.recallClient.DeleteBot(ctx, recall.DeleteBotParams{BotID: bot.BotID}); err != nil {
		log.Printf("Failed to delete Recall bot %s: %v.", bot.BotID, err)
	}

	_, err = c.pool.Exec(ctx,
		"UPDATE bots SET bot_status = 'cancelled', updated_at = NOW() WHERE id = $1",
		bot.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update bot status to cancelled: %w", err)
	}

	statusEvent := model.BotStatusEvent{
		UserID:       event.UserID,
		BotID:        bot.BotID,
		BotStatus:    "cancelled",
		MeetingTitle: bot.MeetingTitle,
	}
	statusBytes, err := json.Marshal(statusEvent)
	if err != nil {
		return fmt.Errorf("failed to marshal bot.status event: %w", err)
	}

	err = c.statusWriter.WriteMessages(ctx, kafkago.Message{
		Key:   []byte(strconv.Itoa(event.UserID)),
		Value: statusBytes,
	})
	if err != nil {
		return fmt.Errorf("failed to publish bot.status cancelled: %w", err)
	}

	log.Printf("Cancelled bot %s for user %d.", bot.BotID, event.UserID)
	return nil
}
