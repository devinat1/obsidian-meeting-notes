package kafkautil

import (
	"context"
	"fmt"
	"log"

	"github.com/segmentio/kafka-go"
)

type MessageHandler func(ctx context.Context, msg kafka.Message) error

type Consumer struct {
	reader  *kafka.Reader
	handler MessageHandler
}

func NewConsumer(params NewConsumerParams) *Consumer {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  params.Brokers,
		Topic:    params.Topic,
		GroupID:  params.GroupID,
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	return &Consumer{
		reader:  r,
		handler: params.Handler,
	}
}

type NewConsumerParams struct {
	Brokers []string
	Topic   string
	GroupID string
	Handler MessageHandler
}

func (c *Consumer) Run(ctx context.Context) error {
	for {
		msg, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("fetching kafka message: %w", err)
		}
		if err := c.handler(ctx, msg); err != nil {
			log.Printf("Error handling kafka message (topic=%s, offset=%d): %v", msg.Topic, msg.Offset, err)
			continue
		}
		if err := c.reader.CommitMessages(ctx, msg); err != nil {
			log.Printf("Error committing kafka message (topic=%s, offset=%d): %v", msg.Topic, msg.Offset, err)
		}
	}
}

func (c *Consumer) Close() error {
	if err := c.reader.Close(); err != nil {
		log.Printf("Error closing kafka consumer: %v", err)
		return err
	}
	return nil
}
