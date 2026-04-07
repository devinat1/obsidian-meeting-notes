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

type MeetingUpcomingConsumer struct {
	reader       *kafkago.Reader
	pool         *pgxpool.Pool
	recallClient *recall.Client
	statusWriter *kafkago.Writer
}

type NewMeetingUpcomingConsumerParams struct {
	Reader       *kafkago.Reader
	Pool         *pgxpool.Pool
	RecallClient *recall.Client
	StatusWriter *kafkago.Writer
}

func NewMeetingUpcomingConsumer(params NewMeetingUpcomingConsumerParams) *MeetingUpcomingConsumer {
	return &MeetingUpcomingConsumer{
		reader:       params.Reader,
		pool:         params.Pool,
		recallClient: params.RecallClient,
		statusWriter: params.StatusWriter,
	}
}

func (c *MeetingUpcomingConsumer) Start(ctx context.Context) {
	log.Println("Starting meeting.upcoming consumer.")
	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Println("Meeting upcoming consumer shutting down.")
				return
			}
			log.Printf("Error reading meeting.upcoming message: %v.", err)
			continue
		}

		if err := c.handleMessage(ctx, msg); err != nil {
			log.Printf("Error handling meeting.upcoming message: %v.", err)
		}
	}
}

func (c *MeetingUpcomingConsumer) handleMessage(ctx context.Context, msg kafkago.Message) error {
	var event model.MeetingUpcomingEvent
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return fmt.Errorf("failed to unmarshal meeting.upcoming event: %w", err)
	}

	log.Printf("Processing meeting.upcoming for user %d, event %s, url %s.", event.UserID, event.CalendarEventID, event.MeetingURL)

	var existingBotID string
	err := c.pool.QueryRow(ctx,
		"SELECT bot_id FROM bots WHERE user_id = $1 AND calendar_event_id = $2 AND bot_status NOT IN ('done', 'fatal', 'cancelled')",
		event.UserID, event.CalendarEventID,
	).Scan(&existingBotID)
	if err == nil {
		log.Printf("Bot %s already exists for calendar event %s, skipping.", existingBotID, event.CalendarEventID)
		return nil
	}

	resp, err := c.recallClient.CreateBot(ctx, recall.CreateBotParams{
		MeetingURL: event.MeetingURL,
		BotName:    "Meeting Notes",
	})
	if err != nil {
		return fmt.Errorf("failed to create Recall bot: %w", err)
	}

	_, err = c.pool.Exec(ctx,
		`INSERT INTO bots (user_id, bot_id, calendar_event_id, meeting_url, meeting_title, bot_status)
		 VALUES ($1, $2, $3, $4, $5, 'scheduled')
		 ON CONFLICT (bot_id) DO NOTHING`,
		event.UserID, resp.ID, event.CalendarEventID, event.MeetingURL, event.Title,
	)
	if err != nil {
		return fmt.Errorf("failed to insert bot record: %w", err)
	}

	statusEvent := model.BotStatusEvent{
		UserID:       event.UserID,
		BotID:        resp.ID,
		BotStatus:    "scheduled",
		MeetingTitle: event.Title,
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
		return fmt.Errorf("failed to publish bot.status: %w", err)
	}

	log.Printf("Created bot %s for meeting %s (user %d), published bot.status scheduled.", resp.ID, event.Title, event.UserID)
	return nil
}
