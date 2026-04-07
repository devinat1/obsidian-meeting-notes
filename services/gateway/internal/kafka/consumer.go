// services/gateway/internal/kafka/consumer.go
package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/sse"
	"github.com/devinat1/obsidian-meeting-notes/services/shared/kafkautil"
)

type ConsumerGroupParams struct {
	Brokers []string
	Hub     *sse.Hub
}

func StartConsumers(ctx context.Context, params ConsumerGroupParams) {
	topics := []string{
		"bot.status",
		"transcript.ready",
		"transcript.failed",
		"meeting.upcoming",
	}

	for _, topic := range topics {
		topicCopy := topic
		consumer := kafkautil.NewConsumer(kafkautil.NewConsumerParams{
			Brokers: params.Brokers,
			Topic:   topicCopy,
			GroupID: "gateway-group",
			Handler: func(ctx context.Context, msg kafkago.Message) error {
				return handleMessage(params.Hub, topicCopy, msg)
			},
		})

		go func() {
			log.Printf("Starting Kafka consumer for topic %s.", topicCopy)
			if err := consumer.Run(ctx); err != nil {
				log.Printf("Kafka consumer for topic %s stopped: %v", topicCopy, err)
			}
		}()

		go func() {
			<-ctx.Done()
			consumer.Close()
		}()
	}
}

func handleMessage(hub *sse.Hub, topic string, msg kafkago.Message) error {
	userID, err := ExtractUserID(msg.Value)
	if err != nil {
		log.Printf("Skipping Kafka message on topic %s: %v", topic, err)
		return nil
	}

	DispatchToHub(hub, userID, topic, string(msg.Value))
	return nil
}

func ExtractUserID(data []byte) (int, error) {
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return 0, fmt.Errorf("unmarshaling Kafka message: %w", err)
	}

	userIDRaw, ok := payload["user_id"]
	if !ok {
		return 0, fmt.Errorf("missing user_id in Kafka message")
	}

	userIDFloat, ok := userIDRaw.(float64)
	if !ok {
		return 0, fmt.Errorf("user_id is not a number")
	}

	return int(userIDFloat), nil
}

func DispatchToHub(hub *sse.Hub, userID int, topic string, data string) {
	hub.Send(userID, sse.Event{
		Type: topic,
		Data: data,
	})
}

func BrokersFromString(brokers string) []string {
	return strings.Split(brokers, ",")
}
