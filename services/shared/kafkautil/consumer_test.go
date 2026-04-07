package kafkautil_test

import (
	"context"
	"testing"

	"github.com/segmentio/kafka-go"

	"github.com/devinat1/obsidian-meeting-notes/services/shared/kafkautil"
)

func TestNewConsumer(t *testing.T) {
	c := kafkautil.NewConsumer(kafkautil.NewConsumerParams{
		Brokers: []string{"localhost:9092"},
		Topic:   "test.topic",
		GroupID: "test-group",
		Handler: func(ctx context.Context, msg kafka.Message) error {
			return nil
		},
	})
	if c == nil {
		t.Fatal("expected non-nil consumer")
	}
	if err := c.Close(); err != nil {
		t.Fatalf("unexpected error closing consumer: %v", err)
	}
}
