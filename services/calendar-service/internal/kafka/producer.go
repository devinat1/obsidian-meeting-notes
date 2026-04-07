// services/calendar-service/internal/kafka/producer.go
package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/model"
	kafkago "github.com/segmentio/kafka-go"
)

const (
	TopicMeetingUpcoming  = "meeting.upcoming"
	TopicMeetingCancelled = "meeting.cancelled"
)

type Producer struct {
	writer *kafkago.Writer
}

func NewProducer(broker string) *Producer {
	return &Producer{
		writer: &kafkago.Writer{
			Addr:         kafkago.TCP(broker),
			Balancer:     &kafkago.LeastBytes{},
			BatchTimeout: 10 * time.Millisecond,
			RequiredAcks: kafkago.RequireOne,
		},
	}
}

// PublishMeetingUpcoming publishes a meeting.upcoming event to Kafka.
// Key is user_id for per-user ordering.
func (p *Producer) PublishMeetingUpcoming(ctx context.Context, event model.MeetingUpcomingEvent) error {
	value, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("Failed to marshal meeting.upcoming event: %w.", err)
	}

	msg := kafkago.Message{
		Topic: TopicMeetingUpcoming,
		Key:   []byte(strconv.Itoa(event.UserID)),
		Value: value,
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("Failed to publish meeting.upcoming: %w.", err)
	}

	slog.Info("Published meeting.upcoming.",
		"user_id", event.UserID,
		"calendar_event_id", event.CalendarEventID,
		"title", event.Title,
	)
	return nil
}

// PublishMeetingCancelled publishes a meeting.cancelled event to Kafka.
// Key is user_id for per-user ordering.
func (p *Producer) PublishMeetingCancelled(ctx context.Context, event model.MeetingCancelledEvent) error {
	value, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("Failed to marshal meeting.cancelled event: %w.", err)
	}

	msg := kafkago.Message{
		Topic: TopicMeetingCancelled,
		Key:   []byte(strconv.Itoa(event.UserID)),
		Value: value,
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("Failed to publish meeting.cancelled: %w.", err)
	}

	slog.Info("Published meeting.cancelled.",
		"user_id", event.UserID,
		"calendar_event_id", event.CalendarEventID,
	)
	return nil
}

// Close shuts down the Kafka writer.
func (p *Producer) Close() error {
	return p.writer.Close()
}
