package kafkautil

import (
	"context"
	"fmt"
	"log"

	"github.com/segmentio/kafka-go"
)

type Producer struct {
	writer *kafka.Writer
}

func NewProducer(params NewProducerParams) *Producer {
	w := &kafka.Writer{
		Addr:         kafka.TCP(params.Brokers...),
		Topic:        params.Topic,
		Balancer:     &kafka.LeastBytes{},
		RequiredAcks: kafka.RequireOne,
		Async:        false,
	}
	return &Producer{writer: w}
}

type NewProducerParams struct {
	Brokers []string
	Topic   string
}

func (p *Producer) Publish(ctx context.Context, params PublishParams) error {
	msg := kafka.Message{
		Key:   []byte(params.Key),
		Value: params.Value,
	}
	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("publishing to kafka topic %s: %w", p.writer.Topic, err)
	}
	return nil
}

type PublishParams struct {
	Key   string
	Value []byte
}

func (p *Producer) Close() error {
	if err := p.writer.Close(); err != nil {
		log.Printf("Error closing kafka producer: %v", err)
		return err
	}
	return nil
}
