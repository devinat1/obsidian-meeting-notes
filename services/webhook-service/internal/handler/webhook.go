package handler

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/segmentio/kafka-go"

	"github.com/devinat1/obsidian-meeting-notes/services/webhook-service/internal/verify"
)

type WebhookHandler struct {
	secret    string
	producers map[string]*kafka.Writer
}

type NewWebhookHandlerParams struct {
	Secret    string
	Producers map[string]*kafka.Writer
}

func NewWebhookHandler(params NewWebhookHandlerParams) *WebhookHandler {
	return &WebhookHandler{
		secret:    params.Secret,
		producers: params.Producers,
	}
}

type webhookPayload struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

func (h *WebhookHandler) HandleRecallWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	headers := verify.SvixHeaders{
		ID:        r.Header.Get("svix-id"),
		Timestamp: r.Header.Get("svix-timestamp"),
		Signature: r.Header.Get("svix-signature"),
	}

	if err := verify.VerifySvixSignature(verify.VerifySvixSignatureParams{
		Headers: headers,
		Body:    body,
		Secret:  h.secret,
	}); err != nil {
		log.Printf("Webhook signature verification failed: %v.", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var payload webhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("Failed to parse webhook payload: %v.", err)
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	topic := resolveTopicFromEvent(payload.Event)
	if topic == "" {
		log.Printf("Unknown event type: %s.", payload.Event)
		w.WriteHeader(http.StatusOK)
		return
	}

	botID := extractBotID(payload.Data)

	producer, exists := h.producers[topic]
	if !exists {
		log.Printf("No producer configured for topic: %s.", topic)
		w.WriteHeader(http.StatusOK)
		return
	}

	err = producer.WriteMessages(context.Background(), kafka.Message{
		Key:   []byte(botID),
		Value: body,
	})
	if err != nil {
		log.Printf("Failed to publish to Kafka topic %s: %v.", topic, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	log.Printf("Published %s event to topic %s with bot_id %s.", payload.Event, topic, botID)
	w.WriteHeader(http.StatusOK)
}

func resolveTopicFromEvent(event string) string {
	switch {
	case len(event) >= 4 && event[:4] == "bot.":
		return "bot.events"
	case len(event) >= 10 && event[:10] == "recording.":
		return "recording.events"
	case len(event) >= 11 && event[:11] == "transcript.":
		return "transcript.events"
	default:
		return ""
	}
}

func extractBotID(data json.RawMessage) string {
	var parsed struct {
		BotID string `json:"bot_id"`
		Data  struct {
			BotID string `json:"bot_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &parsed); err == nil {
		if parsed.BotID != "" {
			return parsed.BotID
		}
		if parsed.Data.BotID != "" {
			return parsed.Data.BotID
		}
	}
	return "unknown"
}
