# Plan 3: Recall AI Pipeline (Bot, Webhook, Transcript)

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the three Go microservices that form the Recall AI pipeline: Webhook Service (ingress bridge), Bot Service (bot lifecycle management), and Transcript Service (transcript fetching and formatting). Together they receive Recall AI webhooks, manage bot state, and produce ready-to-consume transcripts.

**Architecture:** Three independent Go services communicating via Kafka. Webhook Service is a stateless HTTP-to-Kafka bridge. Bot Service is a Kafka consumer/producer with Postgres state and a Recall AI HTTP client. Transcript Service is a Kafka consumer/producer with Postgres storage and Recall AI HTTP client. Each service has an internal HTTP API for cross-service queries. All deploy as single-replica Deployments in the `liteagent` namespace.

**Tech Stack:** Go 1.22+, chi router, segmentio/kafka-go (Kafka client), jackc/pgx/v5 (Postgres driver), golang-migrate, net/http (Recall AI client), crypto/hmac + crypto/sha256 (Svix HMAC verification)

**Spec:** `docs/superpowers/specs/2026-04-05-obsidian-meeting-notes-design.md`

**Depends on:** Plan 1 (shared Go module with Kafka helpers, config loader, Postgres helpers, Helm chart patterns for Postgres instances)

---

## File Structure

```
services/
├── webhook-service/
│   ├── cmd/
│   │   └── webhook/
│   │       └── main.go                      # Entrypoint: config, Kafka producer, HTTP server
│   ├── internal/
│   │   ├── config/
│   │   │   └── config.go                    # RECALL_WORKSPACE_VERIFICATION_SECRET, KAFKA_BROKERS, PORT
│   │   ├── handler/
│   │   │   ├── webhook.go                   # POST /webhooks/recallai handler
│   │   │   └── webhook_test.go
│   │   ├── verify/
│   │   │   ├── svix.go                      # Svix HMAC-SHA256 verification
│   │   │   └── svix_test.go
│   │   └── router/
│   │       └── router.go                    # Chi router setup + health check
│   ├── go.mod
│   ├── go.sum
│   └── Dockerfile
│
├── bot-service/
│   ├── cmd/
│   │   └── bot/
│   │       └── main.go                      # Entrypoint: config, DB, Kafka, HTTP server, consumers, cleanup
│   ├── internal/
│   │   ├── config/
│   │   │   └── config.go                    # RECALL_REGION, RECALL_API_KEY, KAFKA_BROKERS, DATABASE_URL, etc.
│   │   ├── database/
│   │   │   ├── database.go                  # pgx pool setup + migration runner
│   │   │   └── migrations/
│   │   │       ├── 001_create_bots.up.sql
│   │   │       └── 001_create_bots.down.sql
│   │   ├── model/
│   │   │   └── model.go                     # Bot struct, Kafka message types, API response types
│   │   ├── recall/
│   │   │   ├── client.go                    # Recall AI HTTP client (create bot, delete bot)
│   │   │   └── client_test.go
│   │   ├── consumer/
│   │   │   ├── meeting_upcoming.go          # Kafka consumer for meeting.upcoming
│   │   │   ├── meeting_upcoming_test.go
│   │   │   ├── meeting_cancelled.go         # Kafka consumer for meeting.cancelled
│   │   │   ├── meeting_cancelled_test.go
│   │   │   ├── bot_events.go               # Kafka consumer for bot.events
│   │   │   ├── bot_events_test.go
│   │   │   ├── recording_events.go         # Kafka consumer for recording.events
│   │   │   └── recording_events_test.go
│   │   ├── handler/
│   │   │   ├── delete_bot.go               # DELETE /internal/bots/:botId
│   │   │   ├── delete_bot_test.go
│   │   │   ├── bot_status.go               # GET /internal/bots/:botId/status
│   │   │   └── bot_status_test.go
│   │   ├── background/
│   │   │   ├── stale_bots.go               # Stale bot cleanup goroutine
│   │   │   └── stale_bots_test.go
│   │   └── router/
│   │       └── router.go                    # Chi router for internal API + health check
│   ├── go.mod
│   ├── go.sum
│   └── Dockerfile
│
├── transcript-service/
│   ├── cmd/
│   │   └── transcript/
│   │       └── main.go                      # Entrypoint: config, DB, Kafka, HTTP server, consumer
│   ├── internal/
│   │   ├── config/
│   │   │   └── config.go                    # RECALL_REGION, RECALL_API_KEY, KAFKA_BROKERS, DATABASE_URL, etc.
│   │   ├── database/
│   │   │   ├── database.go                  # pgx pool setup + migration runner
│   │   │   └── migrations/
│   │   │       ├── 001_create_transcripts.up.sql
│   │   │       └── 001_create_transcripts.down.sql
│   │   ├── model/
│   │   │   └── model.go                     # Transcript struct, Kafka message types, readable format types
│   │   ├── recall/
│   │   │   ├── client.go                    # Recall AI HTTP client (fetch transcript metadata + download)
│   │   │   └── client_test.go
│   │   ├── consumer/
│   │   │   ├── transcript_events.go         # Kafka consumer for transcript.events
│   │   │   └── transcript_events_test.go
│   │   ├── transcript/
│   │   │   ├── convert.go                   # Raw transcript to readable format conversion
│   │   │   └── convert_test.go
│   │   ├── handler/
│   │   │   ├── get_transcript.go            # GET /internal/transcripts/:botId
│   │   │   └── get_transcript_test.go
│   │   └── router/
│   │       └── router.go                    # Chi router for internal API + health check
│   ├── go.mod
│   ├── go.sum
│   └── Dockerfile
│
helm-meeting-webhook/
├── Chart.yaml
├── values.yaml
└── templates/
    ├── deployment.yaml
    ├── service.yaml
    └── secret.yaml
│
helm-meeting-bot/
├── Chart.yaml
├── values.yaml
└── templates/
    ├── deployment.yaml
    ├── service.yaml
    ├── secret.yaml
    └── pvc.yaml
│
helm-meeting-bot-db/
├── Chart.yaml
├── values.yaml
└── templates/
    ├── deployment.yaml
    ├── service.yaml
    ├── secret.yaml
    └── pvc.yaml
│
helm-meeting-transcript/
├── Chart.yaml
├── values.yaml
└── templates/
    ├── deployment.yaml
    ├── service.yaml
    ├── secret.yaml
    └── pvc.yaml
│
helm-meeting-transcript-db/
├── Chart.yaml
├── values.yaml
└── templates/
    ├── deployment.yaml
    ├── service.yaml
    ├── secret.yaml
    └── pvc.yaml
```

---

## Chunk 1: Webhook Service (Stateless HTTP-to-Kafka Bridge)

### Task 1: Webhook Service project setup and config

**Files:**
- Create: `services/webhook-service/go.mod`
- Create: `services/webhook-service/cmd/webhook/main.go`
- Create: `services/webhook-service/internal/config/config.go`
- Create: `services/webhook-service/internal/router/router.go`

- [ ] **Step 1: Initialize Go module**

```bash
cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin
mkdir -p services/webhook-service/cmd/webhook
cd services/webhook-service
go mod init github.com/devinat1/obsidian-meeting-notes/services/webhook-service
go get github.com/go-chi/chi/v5
go get github.com/segmentio/kafka-go
```

- [ ] **Step 2: Write config.go**

```go
// services/webhook-service/internal/config/config.go
package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port                              string
	KafkaBrokers                      string
	RecallWorkspaceVerificationSecret string
}

func Load() (*Config, error) {
	c := &Config{
		Port:                              getEnvOrDefault("PORT", "8080"),
		KafkaBrokers:                      os.Getenv("KAFKA_BROKERS"),
		RecallWorkspaceVerificationSecret: os.Getenv("RECALL_WORKSPACE_VERIFICATION_SECRET"),
	}

	if c.KafkaBrokers == "" {
		return nil, fmt.Errorf("KAFKA_BROKERS is required")
	}
	if c.RecallWorkspaceVerificationSecret == "" {
		return nil, fmt.Errorf("RECALL_WORKSPACE_VERIFICATION_SECRET is required")
	}

	return c, nil
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 3: Write router.go with health check**

```go
// services/webhook-service/internal/router/router.go
package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func New() *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	return r
}
```

- [ ] **Step 4: Write minimal main.go**

```go
// services/webhook-service/cmd/webhook/main.go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/services/webhook-service/internal/config"
	"github.com/devinat1/obsidian-meeting-notes/services/webhook-service/internal/router"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	r := router.New()

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		log.Printf("Webhook service starting on port %s.", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down webhook service.")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}
}
```

- [ ] **Step 5: Verify compilation**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/webhook-service && go build ./cmd/webhook`
Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add services/webhook-service/
git commit -m "feat(webhook-service): initialize project with config, router, and main entrypoint"
```

---

### Task 2: Svix HMAC-SHA256 verification

**Files:**
- Create: `services/webhook-service/internal/verify/svix.go`
- Create: `services/webhook-service/internal/verify/svix_test.go`

- [ ] **Step 1: Write svix.go**

Svix webhook format sends three headers:
- `svix-id`: unique message ID
- `svix-timestamp`: Unix timestamp string
- `svix-signature`: comma-separated list of `v1,<base64-signature>` values

The signed content is `{svix-id}.{svix-timestamp}.{body}`. The secret is base64-encoded with a `whsec_` prefix.

```go
// services/webhook-service/internal/verify/svix.go
package verify

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	toleranceSeconds = 300
	secretPrefix     = "whsec_"
)

type SvixHeaders struct {
	ID        string
	Timestamp string
	Signature string
}

func VerifySvixSignature(params VerifySvixSignatureParams) error {
	if params.Headers.ID == "" || params.Headers.Timestamp == "" || params.Headers.Signature == "" {
		return fmt.Errorf("missing required Svix headers")
	}

	timestampInt, err := strconv.ParseInt(params.Headers.Timestamp, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid svix-timestamp: %w", err)
	}

	now := time.Now().Unix()
	if math.Abs(float64(now-timestampInt)) > toleranceSeconds {
		return fmt.Errorf("timestamp outside tolerance window")
	}

	secretBytes, err := decodeSecret(params.Secret)
	if err != nil {
		return fmt.Errorf("failed to decode secret: %w", err)
	}

	signedContent := fmt.Sprintf("%s.%s.%s", params.Headers.ID, params.Headers.Timestamp, string(params.Body))

	mac := hmac.New(sha256.New, secretBytes)
	mac.Write([]byte(signedContent))
	expectedSignature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	expectedWithPrefix := "v1," + expectedSignature

	signatures := strings.Split(params.Headers.Signature, " ")
	for _, sig := range signatures {
		if hmac.Equal([]byte(strings.TrimSpace(sig)), []byte(expectedWithPrefix)) {
			return nil
		}
	}

	return fmt.Errorf("no matching signature found")
}

type VerifySvixSignatureParams struct {
	Headers SvixHeaders
	Body    []byte
	Secret  string
}

func decodeSecret(secret string) ([]byte, error) {
	trimmed := strings.TrimPrefix(secret, secretPrefix)
	return base64.StdEncoding.DecodeString(trimmed)
}
```

- [ ] **Step 2: Write svix_test.go (TDD)**

```go
// services/webhook-service/internal/verify/svix_test.go
package verify

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strconv"
	"testing"
	"time"
)

func generateTestSignature(params generateTestSignatureParams) string {
	secretBytes, _ := base64.StdEncoding.DecodeString(params.SecretRaw)
	signedContent := fmt.Sprintf("%s.%s.%s", params.ID, params.Timestamp, string(params.Body))
	mac := hmac.New(sha256.New, secretBytes)
	mac.Write([]byte(signedContent))
	return "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

type generateTestSignatureParams struct {
	SecretRaw string
	ID        string
	Timestamp string
	Body      []byte
}

func TestVerifySvixSignature_ValidSignature(t *testing.T) {
	secretRaw := base64.StdEncoding.EncodeToString([]byte("test-secret-key-1234"))
	secret := "whsec_" + secretRaw
	body := []byte(`{"event":"bot.status_change","data":{"bot_id":"abc123"}}`)
	msgID := "msg_abc123"
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signature := generateTestSignature(generateTestSignatureParams{
		SecretRaw: secretRaw,
		ID:        msgID,
		Timestamp: timestamp,
		Body:      body,
	})

	err := VerifySvixSignature(VerifySvixSignatureParams{
		Headers: SvixHeaders{
			ID:        msgID,
			Timestamp: timestamp,
			Signature: signature,
		},
		Body:   body,
		Secret: secret,
	})
	if err != nil {
		t.Fatalf("expected valid signature, got error: %v", err)
	}
}

func TestVerifySvixSignature_InvalidSignature(t *testing.T) {
	secret := "whsec_" + base64.StdEncoding.EncodeToString([]byte("test-secret-key-1234"))
	body := []byte(`{"event":"bot.status_change"}`)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	err := VerifySvixSignature(VerifySvixSignatureParams{
		Headers: SvixHeaders{
			ID:        "msg_abc123",
			Timestamp: timestamp,
			Signature: "v1,invalidsignaturehere",
		},
		Body:   body,
		Secret: secret,
	})
	if err == nil {
		t.Fatal("expected error for invalid signature, got nil")
	}
}

func TestVerifySvixSignature_ExpiredTimestamp(t *testing.T) {
	secretRaw := base64.StdEncoding.EncodeToString([]byte("test-secret-key-1234"))
	secret := "whsec_" + secretRaw
	body := []byte(`{"event":"bot.status_change"}`)
	msgID := "msg_abc123"
	expiredTimestamp := strconv.FormatInt(time.Now().Add(-10*time.Minute).Unix(), 10)
	signature := generateTestSignature(generateTestSignatureParams{
		SecretRaw: secretRaw,
		ID:        msgID,
		Timestamp: expiredTimestamp,
		Body:      body,
	})

	err := VerifySvixSignature(VerifySvixSignatureParams{
		Headers: SvixHeaders{
			ID:        msgID,
			Timestamp: expiredTimestamp,
			Signature: signature,
		},
		Body:   body,
		Secret: secret,
	})
	if err == nil {
		t.Fatal("expected error for expired timestamp, got nil")
	}
}

func TestVerifySvixSignature_MissingHeaders(t *testing.T) {
	err := VerifySvixSignature(VerifySvixSignatureParams{
		Headers: SvixHeaders{},
		Body:    []byte("test"),
		Secret:  "whsec_dGVzdA==",
	})
	if err == nil {
		t.Fatal("expected error for missing headers, got nil")
	}
}

func TestVerifySvixSignature_MultipleSignatures(t *testing.T) {
	secretRaw := base64.StdEncoding.EncodeToString([]byte("test-secret-key-1234"))
	secret := "whsec_" + secretRaw
	body := []byte(`{"event":"bot.status_change"}`)
	msgID := "msg_abc123"
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	validSig := generateTestSignature(generateTestSignatureParams{
		SecretRaw: secretRaw,
		ID:        msgID,
		Timestamp: timestamp,
		Body:      body,
	})
	multiSig := "v1,oldsignature " + validSig

	err := VerifySvixSignature(VerifySvixSignatureParams{
		Headers: SvixHeaders{
			ID:        msgID,
			Timestamp: timestamp,
			Signature: multiSig,
		},
		Body:   body,
		Secret: secret,
	})
	if err != nil {
		t.Fatalf("expected valid with multiple signatures, got error: %v", err)
	}
}
```

- [ ] **Step 3: Run tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/webhook-service && go test ./internal/verify/...`
Expected: all 5 tests pass

- [ ] **Step 4: Commit**

```bash
git add services/webhook-service/internal/verify/
git commit -m "feat(webhook-service): add Svix HMAC-SHA256 signature verification with tests"
```

---

### Task 3: Webhook handler and Kafka producer

**Files:**
- Create: `services/webhook-service/internal/handler/webhook.go`
- Create: `services/webhook-service/internal/handler/webhook_test.go`
- Update: `services/webhook-service/internal/router/router.go`
- Update: `services/webhook-service/cmd/webhook/main.go`

- [ ] **Step 1: Write webhook.go handler**

The handler reads the raw body, verifies the Svix signature, parses the event type from the JSON, routes to the correct Kafka topic based on event prefix, and returns 200 immediately.

```go
// services/webhook-service/internal/handler/webhook.go
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
```

- [ ] **Step 2: Write webhook_test.go**

```go
// services/webhook-service/internal/handler/webhook_test.go
package handler

import (
	"encoding/json"
	"testing"
)

func TestResolveTopicFromEvent(t *testing.T) {
	tests := []struct {
		event    string
		expected string
	}{
		{"bot.status_change", "bot.events"},
		{"bot.leave_call", "bot.events"},
		{"recording.started", "recording.events"},
		{"recording.done", "recording.events"},
		{"transcript.done", "transcript.events"},
		{"transcript.failed", "transcript.events"},
		{"unknown.event", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			got := resolveTopicFromEvent(tt.event)
			if got != tt.expected {
				t.Errorf("resolveTopicFromEvent(%q) = %q, want %q", tt.event, got, tt.expected)
			}
		})
	}
}

func TestExtractBotID(t *testing.T) {
	tests := []struct {
		name     string
		data     string
		expected string
	}{
		{
			name:     "top-level bot_id",
			data:     `{"bot_id":"abc123","status":"in_call"}`,
			expected: "abc123",
		},
		{
			name:     "nested bot_id",
			data:     `{"data":{"bot_id":"def456"}}`,
			expected: "def456",
		},
		{
			name:     "no bot_id found",
			data:     `{"other":"field"}`,
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractBotID(json.RawMessage(tt.data))
			if got != tt.expected {
				t.Errorf("extractBotID(%s) = %q, want %q", tt.data, got, tt.expected)
			}
		})
	}
}
```

- [ ] **Step 3: Update router.go to accept the webhook handler**

```go
// services/webhook-service/internal/router/router.go
package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/devinat1/obsidian-meeting-notes/services/webhook-service/internal/handler"
)

type NewParams struct {
	WebhookHandler *handler.WebhookHandler
}

func New(params NewParams) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Post("/webhooks/recallai", params.WebhookHandler.HandleRecallWebhook)

	return r
}
```

- [ ] **Step 4: Update main.go with Kafka producers**

```go
// services/webhook-service/cmd/webhook/main.go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/devinat1/obsidian-meeting-notes/services/webhook-service/internal/config"
	"github.com/devinat1/obsidian-meeting-notes/services/webhook-service/internal/handler"
	"github.com/devinat1/obsidian-meeting-notes/services/webhook-service/internal/router"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	brokers := strings.Split(cfg.KafkaBrokers, ",")
	topics := []string{"bot.events", "recording.events", "transcript.events"}
	producers := make(map[string]*kafkago.Writer)

	for _, topic := range topics {
		producers[topic] = &kafkago.Writer{
			Addr:         kafkago.TCP(brokers...),
			Topic:        topic,
			Balancer:     &kafkago.LeastBytes{},
			RequiredAcks: kafkago.RequireOne,
			BatchTimeout: 10 * time.Millisecond,
		}
	}

	webhookHandler := handler.NewWebhookHandler(handler.NewWebhookHandlerParams{
		Secret:    cfg.RecallWorkspaceVerificationSecret,
		Producers: producers,
	})

	r := router.New(router.NewParams{
		WebhookHandler: webhookHandler,
	})

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		log.Printf("Webhook service starting on port %s.", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down webhook service.")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	for topic, producer := range producers {
		if err := producer.Close(); err != nil {
			log.Printf("Failed to close producer for topic %s: %v.", topic, err)
		}
	}

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}
}
```

- [ ] **Step 5: Run all tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/webhook-service && go test ./...`
Expected: all tests pass

- [ ] **Step 6: Commit**

```bash
git add services/webhook-service/
git commit -m "feat(webhook-service): add webhook handler with Kafka routing and Svix verification"
```

---

### Task 4: Webhook Service Dockerfile and Helm chart

**Files:**
- Create: `services/webhook-service/Dockerfile`
- Create: `helm-meeting-webhook/Chart.yaml`
- Create: `helm-meeting-webhook/values.yaml`
- Create: `helm-meeting-webhook/templates/deployment.yaml`
- Create: `helm-meeting-webhook/templates/service.yaml`
- Create: `helm-meeting-webhook/templates/secret.yaml`

- [ ] **Step 1: Write Dockerfile**

```dockerfile
# services/webhook-service/Dockerfile
FROM golang:1.22-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /webhook-service ./cmd/webhook

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /webhook-service /webhook-service
EXPOSE 8080
ENTRYPOINT ["/webhook-service"]
```

- [ ] **Step 2: Write Chart.yaml**

```yaml
# helm-meeting-webhook/Chart.yaml
apiVersion: v2
name: meeting-webhook
description: Recall AI webhook-to-Kafka bridge (stateless)
type: application
version: 0.1.0
appVersion: "latest"
```

- [ ] **Step 3: Write values.yaml**

```yaml
# helm-meeting-webhook/values.yaml
namespace: liteagent

webhook:
  image:
    repository: ghcr.io/devinat1/meeting-webhook-service
    tag: "latest"
  port: 8080
  resources:
    requests:
      memory: "64Mi"
      cpu: "50m"
    limits:
      memory: "128Mi"
      cpu: "200m"

env:
  KAFKA_BROKERS: "kafka.liteagent.svc.cluster.local:9092"
  RECALL_WORKSPACE_VERIFICATION_SECRET: "CHANGEME"

nodeSelector: {}
```

- [ ] **Step 4: Write deployment.yaml**

```yaml
# helm-meeting-webhook/templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: meeting-webhook
  namespace: {{ .Values.namespace }}
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: meeting-webhook
  template:
    metadata:
      labels:
        app: meeting-webhook
    spec:
      {{- with .Values.nodeSelector }}
      nodeSelector:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      containers:
        - name: meeting-webhook
          image: {{ .Values.webhook.image.repository }}:{{ .Values.webhook.image.tag }}
          ports:
            - containerPort: {{ .Values.webhook.port }}
              name: http
          env:
            - name: PORT
              value: {{ .Values.webhook.port | quote }}
            - name: KAFKA_BROKERS
              valueFrom:
                secretKeyRef:
                  name: meeting-webhook-secret
                  key: KAFKA_BROKERS
            - name: RECALL_WORKSPACE_VERIFICATION_SECRET
              valueFrom:
                secretKeyRef:
                  name: meeting-webhook-secret
                  key: RECALL_WORKSPACE_VERIFICATION_SECRET
          resources:
            {{- toYaml .Values.webhook.resources | nindent 12 }}
          readinessProbe:
            httpGet:
              path: /healthz
              port: {{ .Values.webhook.port }}
            initialDelaySeconds: 5
            periodSeconds: 10
          livenessProbe:
            httpGet:
              path: /healthz
              port: {{ .Values.webhook.port }}
            initialDelaySeconds: 10
            periodSeconds: 30
```

- [ ] **Step 5: Write service.yaml**

```yaml
# helm-meeting-webhook/templates/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: meeting-webhook
  namespace: {{ .Values.namespace }}
spec:
  selector:
    app: meeting-webhook
  ports:
    - port: {{ .Values.webhook.port }}
      targetPort: {{ .Values.webhook.port }}
      protocol: TCP
      name: http
```

- [ ] **Step 6: Write secret.yaml**

```yaml
# helm-meeting-webhook/templates/secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: meeting-webhook-secret
  namespace: {{ .Values.namespace }}
type: Opaque
stringData:
  KAFKA_BROKERS: {{ .Values.env.KAFKA_BROKERS | quote }}
  RECALL_WORKSPACE_VERIFICATION_SECRET: {{ .Values.env.RECALL_WORKSPACE_VERIFICATION_SECRET | quote }}
```

- [ ] **Step 7: Validate Helm template**

Run: `helm template meeting-webhook helm-meeting-webhook/`
Expected: valid YAML output, no errors

- [ ] **Step 8: Commit**

```bash
git add services/webhook-service/Dockerfile helm-meeting-webhook/
git commit -m "feat(webhook-service): add Dockerfile and Helm chart for liteagent namespace"
```

---

## Chunk 2: Bot Service Core (Database, Recall Client, Models)

### Task 5: Bot Service project setup with database and migrations

**Files:**
- Create: `services/bot-service/go.mod`
- Create: `services/bot-service/cmd/bot/main.go`
- Create: `services/bot-service/internal/config/config.go`
- Create: `services/bot-service/internal/database/database.go`
- Create: `services/bot-service/internal/database/migrations/001_create_bots.up.sql`
- Create: `services/bot-service/internal/database/migrations/001_create_bots.down.sql`
- Create: `services/bot-service/internal/model/model.go`

- [ ] **Step 1: Initialize Go module**

```bash
cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin
mkdir -p services/bot-service/cmd/bot
cd services/bot-service
go mod init github.com/devinat1/obsidian-meeting-notes/services/bot-service
go get github.com/go-chi/chi/v5
go get github.com/segmentio/kafka-go
go get github.com/jackc/pgx/v5
go get github.com/golang-migrate/migrate/v4
go get github.com/golang-migrate/migrate/v4/database/postgres
go get github.com/golang-migrate/migrate/v4/source/file
```

- [ ] **Step 2: Write config.go**

```go
// services/bot-service/internal/config/config.go
package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port                string
	DatabaseURL         string
	KafkaBrokers        string
	RecallRegion        string
	RecallAPIKey        string
	BotServiceInternalURL string
}

func Load() (*Config, error) {
	c := &Config{
		Port:                  getEnvOrDefault("PORT", "8081"),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		KafkaBrokers:          os.Getenv("KAFKA_BROKERS"),
		RecallRegion:          os.Getenv("RECALL_REGION"),
		RecallAPIKey:          os.Getenv("RECALL_API_KEY"),
		BotServiceInternalURL: getEnvOrDefault("BOT_SERVICE_INTERNAL_URL", "http://meeting-bot.liteagent.svc.cluster.local:8081"),
	}

	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if c.KafkaBrokers == "" {
		return nil, fmt.Errorf("KAFKA_BROKERS is required")
	}
	if c.RecallRegion == "" {
		return nil, fmt.Errorf("RECALL_REGION is required")
	}
	if c.RecallAPIKey == "" {
		return nil, fmt.Errorf("RECALL_API_KEY is required")
	}

	return c, nil
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 3: Write migration SQL**

```sql
-- services/bot-service/internal/database/migrations/001_create_bots.up.sql
CREATE TABLE bots (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    bot_id TEXT NOT NULL UNIQUE,
    calendar_event_id TEXT NOT NULL,
    meeting_url TEXT NOT NULL,
    meeting_title TEXT NOT NULL DEFAULT '',
    bot_status TEXT NOT NULL DEFAULT 'scheduled',
    bot_webhook_updated_at TIMESTAMPTZ,
    recording_id TEXT,
    recording_status TEXT,
    recording_webhook_updated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_bots_user_id ON bots (user_id);
CREATE INDEX idx_bots_bot_id ON bots (bot_id);
CREATE INDEX idx_bots_calendar_event_id ON bots (calendar_event_id);
CREATE INDEX idx_bots_bot_status ON bots (bot_status);
```

```sql
-- services/bot-service/internal/database/migrations/001_create_bots.down.sql
DROP TABLE IF EXISTS bots;
```

- [ ] **Step 4: Write database.go**

```go
// services/bot-service/internal/database/database.go
package database

import (
	"context"
	"embed"
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}

func RunMigrations(databaseURL string) error {
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, databaseURL)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Println("Database migrations completed successfully.")
	return nil
}
```

- [ ] **Step 5: Write model.go**

```go
// services/bot-service/internal/model/model.go
package model

import "time"

type Bot struct {
	ID                        int        `json:"id"`
	UserID                    int        `json:"user_id"`
	BotID                     string     `json:"bot_id"`
	CalendarEventID           string     `json:"calendar_event_id"`
	MeetingURL                string     `json:"meeting_url"`
	MeetingTitle              string     `json:"meeting_title"`
	BotStatus                 string     `json:"bot_status"`
	BotWebhookUpdatedAt       *time.Time `json:"bot_webhook_updated_at,omitempty"`
	RecordingID               *string    `json:"recording_id,omitempty"`
	RecordingStatus           *string    `json:"recording_status,omitempty"`
	RecordingWebhookUpdatedAt *time.Time `json:"recording_webhook_updated_at,omitempty"`
	CreatedAt                 time.Time  `json:"created_at"`
	UpdatedAt                 time.Time  `json:"updated_at"`
}

type MeetingUpcomingEvent struct {
	UserID          int    `json:"user_id"`
	CalendarEventID string `json:"calendar_event_id"`
	MeetingURL      string `json:"meeting_url"`
	Title           string `json:"title"`
	StartTime       string `json:"start_time"`
}

type MeetingCancelledEvent struct {
	UserID          int    `json:"user_id"`
	CalendarEventID string `json:"calendar_event_id"`
	BotID           string `json:"bot_id"`
}

type BotStatusEvent struct {
	UserID       int    `json:"user_id"`
	BotID        string `json:"bot_id"`
	BotStatus    string `json:"bot_status"`
	MeetingTitle string `json:"meeting_title"`
}

type BotStatusResponse struct {
	UserID       int    `json:"user_id"`
	BotID        string `json:"bot_id"`
	BotStatus    string `json:"bot_status"`
	MeetingTitle string `json:"meeting_title"`
	MeetingURL   string `json:"meeting_url"`
}

type RecallWebhookPayload struct {
	Event string                 `json:"event"`
	Data  map[string]interface{} `json:"data"`
}

var TerminalBotStatuses = map[string]bool{
	"done":      true,
	"fatal":     true,
	"cancelled": true,
}
```

- [ ] **Step 6: Write minimal main.go**

```go
// services/bot-service/cmd/bot/main.go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/config"
	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/database"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := database.RunMigrations(cfg.DatabaseURL); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	srv := &http.Server{
		Addr: ":" + cfg.Port,
	}

	go func() {
		log.Printf("Bot service starting on port %s.", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down bot service.")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}
}
```

- [ ] **Step 7: Verify compilation**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/bot-service && go build ./cmd/bot`
Expected: no errors

- [ ] **Step 8: Commit**

```bash
git add services/bot-service/
git commit -m "feat(bot-service): initialize project with database, migrations, config, and models"
```

---

### Task 6: Recall AI HTTP client for Bot Service

**Files:**
- Create: `services/bot-service/internal/recall/client.go`
- Create: `services/bot-service/internal/recall/client_test.go`

- [ ] **Step 1: Write client.go**

```go
// services/bot-service/internal/recall/client.go
package recall

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"time"
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

type NewClientParams struct {
	Region string
	APIKey string
}

func NewClient(params NewClientParams) *Client {
	return &Client{
		baseURL: fmt.Sprintf("https://%s.recall.ai/api/v1", params.Region),
		apiKey:  params.APIKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

type CreateBotRequest struct {
	MeetingURL      string              `json:"meeting_url"`
	BotName         string              `json:"bot_name,omitempty"`
	TranscriptConfig *TranscriptConfig  `json:"transcript"`
}

type TranscriptConfig struct {
	Provider           string `json:"provider"`
	PrioritizeAccuracy bool   `json:"prioritize_accuracy"`
	Language           string `json:"language"`
	Diarization        bool   `json:"diarization"`
}

type CreateBotResponse struct {
	ID     string `json:"id"`
	Status string `json:"status_changes"`
}

type CreateBotParams struct {
	MeetingURL string
	BotName    string
}

const (
	maxRetries507        = 10
	retryInterval507     = 30 * time.Second
	defaultRetryAfter429 = 5 * time.Second
)

func (c *Client) CreateBot(ctx context.Context, params CreateBotParams) (*CreateBotResponse, error) {
	reqBody := CreateBotRequest{
		MeetingURL: params.MeetingURL,
		BotName:    params.BotName,
		TranscriptConfig: &TranscriptConfig{
			Provider:           "recallai_streaming",
			PrioritizeAccuracy: true,
			Language:           "auto",
			Diarization:        true,
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal create bot request: %w", err)
	}

	for attempt := 0; attempt <= maxRetries507; attempt++ {
		if attempt > 0 {
			log.Printf("Retrying create bot (attempt %d/%d) after 507.", attempt, maxRetries507)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryInterval507):
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/bot/", bytes.NewReader(bodyBytes))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Token "+c.apiKey)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("create bot request failed: %w", err)
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()

		if err != nil {
			return nil, fmt.Errorf("failed to read create bot response: %w", err)
		}

		switch resp.StatusCode {
		case http.StatusCreated, http.StatusOK:
			var result CreateBotResponse
			if err := json.Unmarshal(respBody, &result); err != nil {
				return nil, fmt.Errorf("failed to parse create bot response: %w", err)
			}
			return &result, nil

		case http.StatusTooManyRequests:
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
			jitter := time.Duration(rand.Intn(1000)) * time.Millisecond
			waitTime := retryAfter + jitter
			log.Printf("Rate limited (429), waiting %v before retry.", waitTime)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(waitTime):
			}
			continue

		case http.StatusInsufficientStorage:
			log.Printf("Recall returned 507 (insufficient capacity), will retry.")
			continue

		default:
			return nil, fmt.Errorf("create bot failed with status %d: %s", resp.StatusCode, string(respBody))
		}
	}

	return nil, fmt.Errorf("create bot failed after %d retries due to 507", maxRetries507)
}

type DeleteBotParams struct {
	BotID string
}

func (c *Client) DeleteBot(ctx context.Context, params DeleteBotParams) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/bot/"+params.BotID+"/", nil)
	if err != nil {
		return fmt.Errorf("failed to create delete request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete bot request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("delete bot failed with status %d: %s", resp.StatusCode, string(body))
}

func parseRetryAfter(header string) time.Duration {
	if header == "" {
		return defaultRetryAfter429
	}
	seconds, err := strconv.Atoi(header)
	if err != nil {
		return defaultRetryAfter429
	}
	return time.Duration(seconds) * time.Second
}
```

- [ ] **Step 2: Write client_test.go**

```go
// services/bot-service/internal/recall/client_test.go
package recall

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCreateBot_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/bot/" {
			t.Errorf("expected /bot/, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Token test-key" {
			t.Errorf("unexpected Authorization header: %s", r.Header.Get("Authorization"))
		}

		var reqBody CreateBotRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		if reqBody.MeetingURL != "https://meet.google.com/abc-defg-hij" {
			t.Errorf("unexpected meeting_url: %s", reqBody.MeetingURL)
		}
		if reqBody.TranscriptConfig.Provider != "recallai_streaming" {
			t.Errorf("unexpected transcript provider: %s", reqBody.TranscriptConfig.Provider)
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(CreateBotResponse{ID: "bot_123"})
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		apiKey:     "test-key",
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	resp, err := client.CreateBot(context.Background(), CreateBotParams{
		MeetingURL: "https://meet.google.com/abc-defg-hij",
		BotName:    "Meeting Notes Bot",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if resp.ID != "bot_123" {
		t.Errorf("expected bot ID bot_123, got %s", resp.ID)
	}
}

func TestCreateBot_429Retry(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(CreateBotResponse{ID: "bot_456"})
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		apiKey:     "test-key",
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	resp, err := client.CreateBot(context.Background(), CreateBotParams{
		MeetingURL: "https://meet.google.com/test",
	})
	if err != nil {
		t.Fatalf("expected no error after retry, got: %v", err)
	}
	if resp.ID != "bot_456" {
		t.Errorf("expected bot ID bot_456, got %s", resp.ID)
	}
	if callCount != 2 {
		t.Errorf("expected 2 calls (1 retry), got %d", callCount)
	}
}

func TestDeleteBot_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		if r.URL.Path != "/bot/bot_789/" {
			t.Errorf("expected /bot/bot_789/, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		apiKey:     "test-key",
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	err := client.DeleteBot(context.Background(), DeleteBotParams{BotID: "bot_789"})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		header   string
		expected time.Duration
	}{
		{"5", 5 * time.Second},
		{"", defaultRetryAfter429},
		{"invalid", defaultRetryAfter429},
		{"0", 0},
	}

	for _, tt := range tests {
		got := parseRetryAfter(tt.header)
		if got != tt.expected {
			t.Errorf("parseRetryAfter(%q) = %v, want %v", tt.header, got, tt.expected)
		}
	}
}
```

- [ ] **Step 3: Run tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/bot-service && go test ./internal/recall/...`
Expected: all tests pass

- [ ] **Step 4: Commit**

```bash
git add services/bot-service/internal/recall/
git commit -m "feat(bot-service): add Recall AI HTTP client with 507 retry and 429 backoff"
```

---

## Chunk 3: Bot Service Kafka Consumers and Internal API

### Task 7: Bot Service Kafka consumers

**Files:**
- Create: `services/bot-service/internal/consumer/meeting_upcoming.go`
- Create: `services/bot-service/internal/consumer/meeting_upcoming_test.go`
- Create: `services/bot-service/internal/consumer/meeting_cancelled.go`
- Create: `services/bot-service/internal/consumer/meeting_cancelled_test.go`
- Create: `services/bot-service/internal/consumer/bot_events.go`
- Create: `services/bot-service/internal/consumer/bot_events_test.go`
- Create: `services/bot-service/internal/consumer/recording_events.go`
- Create: `services/bot-service/internal/consumer/recording_events_test.go`

- [ ] **Step 1: Write meeting_upcoming.go**

Consumes from `meeting.upcoming`. On each message: create a Recall bot, insert into PG-bot, publish `bot.status` (scheduled).

```go
// services/bot-service/internal/consumer/meeting_upcoming.go
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
```

- [ ] **Step 2: Write meeting_cancelled.go**

```go
// services/bot-service/internal/consumer/meeting_cancelled.go
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
```

- [ ] **Step 3: Write bot_events.go**

Consumes Recall `bot.*` webhook events. Compares `data.data.updated_at` against `bot_webhook_updated_at` for idempotency. Updates bot_status, publishes `bot.status`.

```go
// services/bot-service/internal/consumer/bot_events.go
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
```

- [ ] **Step 4: Write recording_events.go**

```go
// services/bot-service/internal/consumer/recording_events.go
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
```

- [ ] **Step 5: Write meeting_upcoming_test.go (unit test for handleMessage with mocked deps)**

```go
// services/bot-service/internal/consumer/meeting_upcoming_test.go
package consumer

import (
	"encoding/json"
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/model"
)

func TestMeetingUpcomingEvent_Unmarshal(t *testing.T) {
	raw := `{"user_id":1,"calendar_event_id":"evt_abc","meeting_url":"https://meet.google.com/abc","title":"Standup","start_time":"2026-04-05T10:00:00Z"}`

	var event model.MeetingUpcomingEvent
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if event.UserID != 1 {
		t.Errorf("expected user_id 1, got %d", event.UserID)
	}
	if event.CalendarEventID != "evt_abc" {
		t.Errorf("expected calendar_event_id evt_abc, got %s", event.CalendarEventID)
	}
	if event.MeetingURL != "https://meet.google.com/abc" {
		t.Errorf("expected meeting_url https://meet.google.com/abc, got %s", event.MeetingURL)
	}
	if event.Title != "Standup" {
		t.Errorf("expected title Standup, got %s", event.Title)
	}
}
```

- [ ] **Step 6: Write bot_events_test.go and recording_events_test.go**

```go
// services/bot-service/internal/consumer/bot_events_test.go
package consumer

import (
	"encoding/json"
	"testing"
)

func TestRecallBotWebhook_Unmarshal(t *testing.T) {
	raw := `{"event":"bot.status_change","data":{"bot_id":"bot_123","status":"in_call_recording","updated_at":"2026-04-05T10:05:00Z","data":{"updated_at":"2026-04-05T10:05:00Z"}}}`

	var webhook recallBotWebhook
	if err := json.Unmarshal([]byte(raw), &webhook); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if webhook.Event != "bot.status_change" {
		t.Errorf("expected event bot.status_change, got %s", webhook.Event)
	}
	if webhook.Data.BotID != "bot_123" {
		t.Errorf("expected bot_id bot_123, got %s", webhook.Data.BotID)
	}
	if webhook.Data.Status != "in_call_recording" {
		t.Errorf("expected status in_call_recording, got %s", webhook.Data.Status)
	}
}
```

```go
// services/bot-service/internal/consumer/recording_events_test.go
package consumer

import (
	"encoding/json"
	"testing"
)

func TestRecallRecordingWebhook_Unmarshal(t *testing.T) {
	raw := `{"event":"recording.done","data":{"bot_id":"bot_123","recording_id":"rec_456","status":"done","updated_at":"2026-04-05T10:30:00Z","data":{"updated_at":"2026-04-05T10:30:00Z"}}}`

	var webhook recallRecordingWebhook
	if err := json.Unmarshal([]byte(raw), &webhook); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if webhook.Data.RecordingID != "rec_456" {
		t.Errorf("expected recording_id rec_456, got %s", webhook.Data.RecordingID)
	}
	if webhook.Data.Status != "done" {
		t.Errorf("expected status done, got %s", webhook.Data.Status)
	}
}
```

- [ ] **Step 7: Write meeting_cancelled_test.go**

```go
// services/bot-service/internal/consumer/meeting_cancelled_test.go
package consumer

import (
	"encoding/json"
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/model"
)

func TestMeetingCancelledEvent_Unmarshal(t *testing.T) {
	raw := `{"user_id":1,"calendar_event_id":"evt_abc","bot_id":"bot_123"}`

	var event model.MeetingCancelledEvent
	if err := json.Unmarshal([]byte(raw), &event); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if event.UserID != 1 {
		t.Errorf("expected user_id 1, got %d", event.UserID)
	}
	if event.BotID != "bot_123" {
		t.Errorf("expected bot_id bot_123, got %s", event.BotID)
	}
}

func TestTerminalBotStatuses(t *testing.T) {
	if !model.TerminalBotStatuses["done"] {
		t.Error("expected done to be terminal")
	}
	if !model.TerminalBotStatuses["fatal"] {
		t.Error("expected fatal to be terminal")
	}
	if !model.TerminalBotStatuses["cancelled"] {
		t.Error("expected cancelled to be terminal")
	}
	if model.TerminalBotStatuses["in_call"] {
		t.Error("expected in_call to not be terminal")
	}
}
```

- [ ] **Step 8: Run all tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/bot-service && go test ./...`
Expected: all tests pass

- [ ] **Step 9: Commit**

```bash
git add services/bot-service/internal/consumer/
git commit -m "feat(bot-service): add Kafka consumers for meeting.upcoming, meeting.cancelled, bot.events, recording.events"
```

---

### Task 8: Bot Service internal HTTP API and stale bot cleanup

**Files:**
- Create: `services/bot-service/internal/handler/delete_bot.go`
- Create: `services/bot-service/internal/handler/delete_bot_test.go`
- Create: `services/bot-service/internal/handler/bot_status.go`
- Create: `services/bot-service/internal/handler/bot_status_test.go`
- Create: `services/bot-service/internal/background/stale_bots.go`
- Create: `services/bot-service/internal/background/stale_bots_test.go`
- Create: `services/bot-service/internal/router/router.go`

- [ ] **Step 1: Write bot_status.go**

```go
// services/bot-service/internal/handler/bot_status.go
package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/model"
)

type BotStatusHandler struct {
	pool *pgxpool.Pool
}

type NewBotStatusHandlerParams struct {
	Pool *pgxpool.Pool
}

func NewBotStatusHandler(params NewBotStatusHandlerParams) *BotStatusHandler {
	return &BotStatusHandler{pool: params.Pool}
}

func (h *BotStatusHandler) GetBotStatus(w http.ResponseWriter, r *http.Request) {
	botID := chi.URLParam(r, "botId")
	if botID == "" {
		http.Error(w, "botId is required", http.StatusBadRequest)
		return
	}

	var resp model.BotStatusResponse
	err := h.pool.QueryRow(r.Context(),
		"SELECT user_id, bot_id, bot_status, meeting_title, meeting_url FROM bots WHERE bot_id = $1",
		botID,
	).Scan(&resp.UserID, &resp.BotID, &resp.BotStatus, &resp.MeetingTitle, &resp.MeetingURL)
	if err != nil {
		log.Printf("Bot not found for bot_id %s: %v.", botID, err)
		http.Error(w, "bot not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
```

- [ ] **Step 2: Write delete_bot.go**

```go
// services/bot-service/internal/handler/delete_bot.go
package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	kafkago "github.com/segmentio/kafka-go"

	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/model"
	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/recall"
)

type DeleteBotHandler struct {
	pool         *pgxpool.Pool
	recallClient *recall.Client
	statusWriter *kafkago.Writer
}

type NewDeleteBotHandlerParams struct {
	Pool         *pgxpool.Pool
	RecallClient *recall.Client
	StatusWriter *kafkago.Writer
}

func NewDeleteBotHandler(params NewDeleteBotHandlerParams) *DeleteBotHandler {
	return &DeleteBotHandler{
		pool:         params.Pool,
		recallClient: params.RecallClient,
		statusWriter: params.StatusWriter,
	}
}

func (h *DeleteBotHandler) DeleteBot(w http.ResponseWriter, r *http.Request) {
	botID := chi.URLParam(r, "botId")
	if botID == "" {
		http.Error(w, "botId is required", http.StatusBadRequest)
		return
	}

	var bot model.Bot
	err := h.pool.QueryRow(r.Context(),
		"SELECT id, user_id, bot_id, meeting_title, bot_status FROM bots WHERE bot_id = $1",
		botID,
	).Scan(&bot.ID, &bot.UserID, &bot.BotID, &bot.MeetingTitle, &bot.BotStatus)
	if err != nil {
		log.Printf("Bot not found for bot_id %s: %v.", botID, err)
		http.Error(w, "bot not found", http.StatusNotFound)
		return
	}

	if model.TerminalBotStatuses[bot.BotStatus] {
		http.Error(w, "bot already in terminal state", http.StatusConflict)
		return
	}

	if err := h.recallClient.DeleteBot(r.Context(), recall.DeleteBotParams{BotID: botID}); err != nil {
		log.Printf("Failed to delete Recall bot %s: %v.", botID, err)
	}

	_, err = h.pool.Exec(r.Context(),
		"UPDATE bots SET bot_status = 'cancelled', updated_at = NOW() WHERE id = $1",
		bot.ID,
	)
	if err != nil {
		log.Printf("Failed to update bot %s to cancelled: %v.", botID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	statusEvent := model.BotStatusEvent{
		UserID:       bot.UserID,
		BotID:        botID,
		BotStatus:    "cancelled",
		MeetingTitle: bot.MeetingTitle,
	}
	statusBytes, _ := json.Marshal(statusEvent)
	h.statusWriter.WriteMessages(context.Background(), kafkago.Message{
		Key:   []byte(strconv.Itoa(bot.UserID)),
		Value: statusBytes,
	})

	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 3: Write stale_bots.go**

```go
// services/bot-service/internal/background/stale_bots.go
package background

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	kafkago "github.com/segmentio/kafka-go"

	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/model"
)

const (
	staleCheckInterval = 15 * time.Minute
	staleBotThreshold  = 3 * time.Hour
)

type StaleBotCleanup struct {
	pool         *pgxpool.Pool
	statusWriter *kafkago.Writer
}

type NewStaleBotCleanupParams struct {
	Pool         *pgxpool.Pool
	StatusWriter *kafkago.Writer
}

func NewStaleBotCleanup(params NewStaleBotCleanupParams) *StaleBotCleanup {
	return &StaleBotCleanup{
		pool:         params.Pool,
		statusWriter: params.StatusWriter,
	}
}

func (s *StaleBotCleanup) Start(ctx context.Context) {
	log.Println("Starting stale bot cleanup goroutine (every 15 minutes).")
	ticker := time.NewTicker(staleCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Stale bot cleanup shutting down.")
			return
		case <-ticker.C:
			s.cleanup(ctx)
		}
	}
}

func (s *StaleBotCleanup) cleanup(ctx context.Context) {
	threshold := time.Now().Add(-staleBotThreshold)

	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, bot_id, meeting_title FROM bots
		 WHERE bot_status NOT IN ('done', 'fatal', 'cancelled')
		 AND created_at < $1`,
		threshold,
	)
	if err != nil {
		log.Printf("Failed to query stale bots: %v.", err)
		return
	}
	defer rows.Close()

	staleCount := 0
	for rows.Next() {
		var bot model.Bot
		if err := rows.Scan(&bot.ID, &bot.UserID, &bot.BotID, &bot.MeetingTitle); err != nil {
			log.Printf("Failed to scan stale bot row: %v.", err)
			continue
		}

		_, err := s.pool.Exec(ctx,
			"UPDATE bots SET bot_status = 'fatal', updated_at = NOW() WHERE id = $1",
			bot.ID,
		)
		if err != nil {
			log.Printf("Failed to mark bot %s as fatal: %v.", bot.BotID, err)
			continue
		}

		statusEvent := model.BotStatusEvent{
			UserID:       bot.UserID,
			BotID:        bot.BotID,
			BotStatus:    "fatal",
			MeetingTitle: bot.MeetingTitle,
		}
		statusBytes, _ := json.Marshal(statusEvent)
		s.statusWriter.WriteMessages(ctx, kafkago.Message{
			Key:   []byte(strconv.Itoa(bot.UserID)),
			Value: statusBytes,
		})

		staleCount++
		log.Printf("Marked bot %s as fatal (stale for >3 hours).", bot.BotID)
	}

	if staleCount > 0 {
		log.Printf("Cleaned up %d stale bots.", staleCount)
	}
}
```

- [ ] **Step 4: Write stale_bots_test.go**

```go
// services/bot-service/internal/background/stale_bots_test.go
package background

import (
	"testing"
	"time"
)

func TestStaleBotThreshold(t *testing.T) {
	if staleBotThreshold != 3*time.Hour {
		t.Errorf("expected stale bot threshold of 3 hours, got %v", staleBotThreshold)
	}
}

func TestStaleCheckInterval(t *testing.T) {
	if staleCheckInterval != 15*time.Minute {
		t.Errorf("expected stale check interval of 15 minutes, got %v", staleCheckInterval)
	}
}
```

- [ ] **Step 5: Write bot_status_test.go and delete_bot_test.go**

```go
// services/bot-service/internal/handler/bot_status_test.go
package handler

import (
	"testing"
)

func TestBotStatusHandler_NewCreatesHandler(t *testing.T) {
	handler := NewBotStatusHandler(NewBotStatusHandlerParams{Pool: nil})
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}
```

```go
// services/bot-service/internal/handler/delete_bot_test.go
package handler

import (
	"testing"
)

func TestDeleteBotHandler_NewCreatesHandler(t *testing.T) {
	handler := NewDeleteBotHandler(NewDeleteBotHandlerParams{Pool: nil})
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}
```

- [ ] **Step 6: Write router.go**

```go
// services/bot-service/internal/router/router.go
package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/handler"
)

type NewParams struct {
	BotStatusHandler *handler.BotStatusHandler
	DeleteBotHandler *handler.DeleteBotHandler
}

func New(params NewParams) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Route("/internal/bots/{botId}", func(r chi.Router) {
		r.Get("/status", params.BotStatusHandler.GetBotStatus)
		r.Delete("/", params.DeleteBotHandler.DeleteBot)
	})

	return r
}
```

- [ ] **Step 7: Update main.go to wire everything together**

```go
// services/bot-service/cmd/bot/main.go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/background"
	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/config"
	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/consumer"
	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/database"
	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/handler"
	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/recall"
	"github.com/devinat1/obsidian-meeting-notes/services/bot-service/internal/router"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := database.RunMigrations(cfg.DatabaseURL); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	recallClient := recall.NewClient(recall.NewClientParams{
		Region: cfg.RecallRegion,
		APIKey: cfg.RecallAPIKey,
	})

	brokers := strings.Split(cfg.KafkaBrokers, ",")

	statusWriter := &kafkago.Writer{
		Addr:         kafkago.TCP(brokers...),
		Topic:        "bot.status",
		Balancer:     &kafkago.LeastBytes{},
		RequiredAcks: kafkago.RequireOne,
		BatchTimeout: 10 * time.Millisecond,
	}
	defer statusWriter.Close()

	meetingUpcomingReader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:  brokers,
		Topic:    "meeting.upcoming",
		GroupID:  "bot-service-group",
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	defer meetingUpcomingReader.Close()

	meetingCancelledReader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:  brokers,
		Topic:    "meeting.cancelled",
		GroupID:  "bot-service-group",
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	defer meetingCancelledReader.Close()

	botEventsReader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:  brokers,
		Topic:    "bot.events",
		GroupID:  "bot-service-group",
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	defer botEventsReader.Close()

	recordingEventsReader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:  brokers,
		Topic:    "recording.events",
		GroupID:  "bot-service-group",
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	defer recordingEventsReader.Close()

	meetingUpcomingConsumer := consumer.NewMeetingUpcomingConsumer(consumer.NewMeetingUpcomingConsumerParams{
		Reader:       meetingUpcomingReader,
		Pool:         pool,
		RecallClient: recallClient,
		StatusWriter: statusWriter,
	})

	meetingCancelledConsumer := consumer.NewMeetingCancelledConsumer(consumer.NewMeetingCancelledConsumerParams{
		Reader:       meetingCancelledReader,
		Pool:         pool,
		RecallClient: recallClient,
		StatusWriter: statusWriter,
	})

	botEventsConsumer := consumer.NewBotEventsConsumer(consumer.NewBotEventsConsumerParams{
		Reader:       botEventsReader,
		Pool:         pool,
		StatusWriter: statusWriter,
	})

	recordingEventsConsumer := consumer.NewRecordingEventsConsumer(consumer.NewRecordingEventsConsumerParams{
		Reader: recordingEventsReader,
		Pool:   pool,
	})

	staleBotCleanup := background.NewStaleBotCleanup(background.NewStaleBotCleanupParams{
		Pool:         pool,
		StatusWriter: statusWriter,
	})

	go meetingUpcomingConsumer.Start(ctx)
	go meetingCancelledConsumer.Start(ctx)
	go botEventsConsumer.Start(ctx)
	go recordingEventsConsumer.Start(ctx)
	go staleBotCleanup.Start(ctx)

	botStatusHandler := handler.NewBotStatusHandler(handler.NewBotStatusHandlerParams{Pool: pool})
	deleteBotHandler := handler.NewDeleteBotHandler(handler.NewDeleteBotHandlerParams{
		Pool:         pool,
		RecallClient: recallClient,
		StatusWriter: statusWriter,
	})

	r := router.New(router.NewParams{
		BotStatusHandler: botStatusHandler,
		DeleteBotHandler: deleteBotHandler,
	})

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		log.Printf("Bot service starting on port %s.", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down bot service.")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}
}
```

- [ ] **Step 8: Run all tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/bot-service && go test ./...`
Expected: all tests pass

- [ ] **Step 9: Commit**

```bash
git add services/bot-service/
git commit -m "feat(bot-service): add internal HTTP API, Kafka consumers, stale bot cleanup, and full main wiring"
```

---

### Task 9: Bot Service Dockerfile and Helm charts

**Files:**
- Create: `services/bot-service/Dockerfile`
- Create: `helm-meeting-bot/Chart.yaml`
- Create: `helm-meeting-bot/values.yaml`
- Create: `helm-meeting-bot/templates/deployment.yaml`
- Create: `helm-meeting-bot/templates/service.yaml`
- Create: `helm-meeting-bot/templates/secret.yaml`
- Create: `helm-meeting-bot-db/Chart.yaml`
- Create: `helm-meeting-bot-db/values.yaml`
- Create: `helm-meeting-bot-db/templates/deployment.yaml`
- Create: `helm-meeting-bot-db/templates/service.yaml`
- Create: `helm-meeting-bot-db/templates/secret.yaml`
- Create: `helm-meeting-bot-db/templates/pvc.yaml`

- [ ] **Step 1: Write Bot Service Dockerfile**

```dockerfile
# services/bot-service/Dockerfile
FROM golang:1.22-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bot-service ./cmd/bot

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /bot-service /bot-service
EXPOSE 8081
ENTRYPOINT ["/bot-service"]
```

- [ ] **Step 2: Write Bot DB Helm chart**

```yaml
# helm-meeting-bot-db/Chart.yaml
apiVersion: v2
name: meeting-bot-db
description: PostgreSQL for Bot Service
type: application
version: 0.1.0
appVersion: "16"
```

```yaml
# helm-meeting-bot-db/values.yaml
namespace: liteagent

postgres:
  image:
    repository: postgres
    tag: "16"
  port: 5432
  database: bot_service
  user: bot_service
  password: "CHANGEME"
  persistence:
    enabled: true
    size: 2Gi
    storageClass: local-path
  resources:
    requests:
      memory: "128Mi"
      cpu: "100m"
    limits:
      memory: "256Mi"
      cpu: "500m"

nodeSelector: {}
```

```yaml
# helm-meeting-bot-db/templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: meeting-bot-db
  namespace: {{ .Values.namespace }}
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: meeting-bot-db
  template:
    metadata:
      labels:
        app: meeting-bot-db
    spec:
      {{- with .Values.nodeSelector }}
      nodeSelector:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      containers:
        - name: postgres
          image: {{ .Values.postgres.image.repository }}:{{ .Values.postgres.image.tag }}
          ports:
            - containerPort: {{ .Values.postgres.port }}
              name: postgres
          env:
            - name: POSTGRES_DB
              valueFrom:
                secretKeyRef:
                  name: meeting-bot-db-secret
                  key: POSTGRES_DB
            - name: POSTGRES_USER
              valueFrom:
                secretKeyRef:
                  name: meeting-bot-db-secret
                  key: POSTGRES_USER
            - name: POSTGRES_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: meeting-bot-db-secret
                  key: POSTGRES_PASSWORD
          resources:
            {{- toYaml .Values.postgres.resources | nindent 12 }}
          volumeMounts:
            - name: pgdata
              mountPath: /var/lib/postgresql/data
          readinessProbe:
            tcpSocket:
              port: {{ .Values.postgres.port }}
            initialDelaySeconds: 10
            periodSeconds: 10
          livenessProbe:
            tcpSocket:
              port: {{ .Values.postgres.port }}
            initialDelaySeconds: 30
            periodSeconds: 30
      volumes:
        {{- if .Values.postgres.persistence.enabled }}
        - name: pgdata
          persistentVolumeClaim:
            claimName: meeting-bot-db-data
        {{- else }}
        - name: pgdata
          emptyDir: {}
        {{- end }}
```

```yaml
# helm-meeting-bot-db/templates/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: meeting-bot-db
  namespace: {{ .Values.namespace }}
spec:
  selector:
    app: meeting-bot-db
  ports:
    - port: {{ .Values.postgres.port }}
      targetPort: {{ .Values.postgres.port }}
      protocol: TCP
      name: postgres
```

```yaml
# helm-meeting-bot-db/templates/secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: meeting-bot-db-secret
  namespace: {{ .Values.namespace }}
type: Opaque
stringData:
  POSTGRES_DB: {{ .Values.postgres.database | quote }}
  POSTGRES_USER: {{ .Values.postgres.user | quote }}
  POSTGRES_PASSWORD: {{ .Values.postgres.password | quote }}
```

```yaml
# helm-meeting-bot-db/templates/pvc.yaml
{{- if .Values.postgres.persistence.enabled }}
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: meeting-bot-db-data
  namespace: {{ .Values.namespace }}
spec:
  storageClassName: {{ .Values.postgres.persistence.storageClass }}
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: {{ .Values.postgres.persistence.size }}
{{- end }}
```

- [ ] **Step 3: Write Bot Service Helm chart**

```yaml
# helm-meeting-bot/Chart.yaml
apiVersion: v2
name: meeting-bot
description: Bot Service - Recall AI bot lifecycle management
type: application
version: 0.1.0
appVersion: "latest"
```

```yaml
# helm-meeting-bot/values.yaml
namespace: liteagent

bot:
  image:
    repository: ghcr.io/devinat1/meeting-bot-service
    tag: "latest"
  port: 8081
  resources:
    requests:
      memory: "64Mi"
      cpu: "50m"
    limits:
      memory: "256Mi"
      cpu: "500m"

env:
  DATABASE_URL: "postgres://bot_service:CHANGEME@meeting-bot-db.liteagent.svc.cluster.local:5432/bot_service?sslmode=disable"
  KAFKA_BROKERS: "kafka.liteagent.svc.cluster.local:9092"
  RECALL_REGION: "us-west-2"
  RECALL_API_KEY: "CHANGEME"

nodeSelector: {}
```

```yaml
# helm-meeting-bot/templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: meeting-bot
  namespace: {{ .Values.namespace }}
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: meeting-bot
  template:
    metadata:
      labels:
        app: meeting-bot
    spec:
      {{- with .Values.nodeSelector }}
      nodeSelector:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      containers:
        - name: meeting-bot
          image: {{ .Values.bot.image.repository }}:{{ .Values.bot.image.tag }}
          ports:
            - containerPort: {{ .Values.bot.port }}
              name: http
          env:
            - name: PORT
              value: {{ .Values.bot.port | quote }}
            - name: DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: meeting-bot-secret
                  key: DATABASE_URL
            - name: KAFKA_BROKERS
              valueFrom:
                secretKeyRef:
                  name: meeting-bot-secret
                  key: KAFKA_BROKERS
            - name: RECALL_REGION
              valueFrom:
                secretKeyRef:
                  name: meeting-bot-secret
                  key: RECALL_REGION
            - name: RECALL_API_KEY
              valueFrom:
                secretKeyRef:
                  name: meeting-bot-secret
                  key: RECALL_API_KEY
          resources:
            {{- toYaml .Values.bot.resources | nindent 12 }}
          readinessProbe:
            httpGet:
              path: /healthz
              port: {{ .Values.bot.port }}
            initialDelaySeconds: 10
            periodSeconds: 10
          livenessProbe:
            httpGet:
              path: /healthz
              port: {{ .Values.bot.port }}
            initialDelaySeconds: 15
            periodSeconds: 30
      volumes: []
```

```yaml
# helm-meeting-bot/templates/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: meeting-bot
  namespace: {{ .Values.namespace }}
spec:
  selector:
    app: meeting-bot
  ports:
    - port: {{ .Values.bot.port }}
      targetPort: {{ .Values.bot.port }}
      protocol: TCP
      name: http
```

```yaml
# helm-meeting-bot/templates/secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: meeting-bot-secret
  namespace: {{ .Values.namespace }}
type: Opaque
stringData:
  DATABASE_URL: {{ .Values.env.DATABASE_URL | quote }}
  KAFKA_BROKERS: {{ .Values.env.KAFKA_BROKERS | quote }}
  RECALL_REGION: {{ .Values.env.RECALL_REGION | quote }}
  RECALL_API_KEY: {{ .Values.env.RECALL_API_KEY | quote }}
```

- [ ] **Step 4: Validate Helm templates**

Run: `helm template meeting-bot-db helm-meeting-bot-db/ && helm template meeting-bot helm-meeting-bot/`
Expected: valid YAML output, no errors

- [ ] **Step 5: Commit**

```bash
git add services/bot-service/Dockerfile helm-meeting-bot/ helm-meeting-bot-db/
git commit -m "feat(bot-service): add Dockerfile, Helm charts for bot service and bot database"
```

---

## Chunk 4: Transcript Service

### Task 10: Transcript Service project setup with database and migrations

**Files:**
- Create: `services/transcript-service/go.mod`
- Create: `services/transcript-service/cmd/transcript/main.go`
- Create: `services/transcript-service/internal/config/config.go`
- Create: `services/transcript-service/internal/database/database.go`
- Create: `services/transcript-service/internal/database/migrations/001_create_transcripts.up.sql`
- Create: `services/transcript-service/internal/database/migrations/001_create_transcripts.down.sql`
- Create: `services/transcript-service/internal/model/model.go`

- [ ] **Step 1: Initialize Go module**

```bash
cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin
mkdir -p services/transcript-service/cmd/transcript
cd services/transcript-service
go mod init github.com/devinat1/obsidian-meeting-notes/services/transcript-service
go get github.com/go-chi/chi/v5
go get github.com/segmentio/kafka-go
go get github.com/jackc/pgx/v5
go get github.com/golang-migrate/migrate/v4
go get github.com/golang-migrate/migrate/v4/database/postgres
go get github.com/golang-migrate/migrate/v4/source/file
```

- [ ] **Step 2: Write config.go**

```go
// services/transcript-service/internal/config/config.go
package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port                  string
	DatabaseURL           string
	KafkaBrokers          string
	RecallRegion          string
	RecallAPIKey          string
	BotServiceInternalURL string
}

func Load() (*Config, error) {
	c := &Config{
		Port:                  getEnvOrDefault("PORT", "8082"),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		KafkaBrokers:          os.Getenv("KAFKA_BROKERS"),
		RecallRegion:          os.Getenv("RECALL_REGION"),
		RecallAPIKey:          os.Getenv("RECALL_API_KEY"),
		BotServiceInternalURL: getEnvOrDefault("BOT_SERVICE_INTERNAL_URL", "http://meeting-bot.liteagent.svc.cluster.local:8081"),
	}

	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if c.KafkaBrokers == "" {
		return nil, fmt.Errorf("KAFKA_BROKERS is required")
	}
	if c.RecallRegion == "" {
		return nil, fmt.Errorf("RECALL_REGION is required")
	}
	if c.RecallAPIKey == "" {
		return nil, fmt.Errorf("RECALL_API_KEY is required")
	}

	return c, nil
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
```

- [ ] **Step 3: Write migration SQL**

```sql
-- services/transcript-service/internal/database/migrations/001_create_transcripts.up.sql
CREATE TABLE transcripts (
    id SERIAL PRIMARY KEY,
    bot_id TEXT NOT NULL UNIQUE,
    user_id INTEGER NOT NULL,
    meeting_title TEXT NOT NULL DEFAULT '',
    transcript_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending',
    failure_sub_code TEXT,
    raw_transcript JSONB,
    readable_transcript JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_transcripts_bot_id ON transcripts (bot_id);
CREATE INDEX idx_transcripts_user_id ON transcripts (user_id);
```

```sql
-- services/transcript-service/internal/database/migrations/001_create_transcripts.down.sql
DROP TABLE IF EXISTS transcripts;
```

- [ ] **Step 4: Write database.go**

```go
// services/transcript-service/internal/database/database.go
package database

import (
	"context"
	"embed"
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}

func RunMigrations(databaseURL string) error {
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, databaseURL)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Println("Database migrations completed successfully.")
	return nil
}
```

- [ ] **Step 5: Write model.go**

```go
// services/transcript-service/internal/model/model.go
package model

import "time"

type Transcript struct {
	ID                  int              `json:"id"`
	BotID               string           `json:"bot_id"`
	UserID              int              `json:"user_id"`
	MeetingTitle        string           `json:"meeting_title"`
	TranscriptID        string           `json:"transcript_id"`
	Status              string           `json:"status"`
	FailureSubCode      *string          `json:"failure_sub_code,omitempty"`
	RawTranscript       *[]RawSegment    `json:"raw_transcript,omitempty"`
	ReadableTranscript  *[]ReadableBlock `json:"readable_transcript,omitempty"`
	CreatedAt           time.Time        `json:"created_at"`
}

type RawSegment struct {
	Speaker string    `json:"speaker"`
	Words   []RawWord `json:"words"`
}

type RawWord struct {
	Text       string  `json:"text"`
	StartTime  float64 `json:"start_time"`
	EndTime    float64 `json:"end_time"`
	Confidence float64 `json:"confidence"`
}

type ReadableBlock struct {
	Speaker   string `json:"speaker"`
	Timestamp string `json:"timestamp"`
	Text      string `json:"text"`
}

type TranscriptReadyEvent struct {
	UserID              int              `json:"user_id"`
	BotID               string           `json:"bot_id"`
	MeetingTitle        string           `json:"meeting_title"`
	RawTranscript       []RawSegment     `json:"raw_transcript"`
	ReadableTranscript  []ReadableBlock  `json:"readable_transcript"`
}

type TranscriptFailedEvent struct {
	UserID         int    `json:"user_id"`
	BotID          string `json:"bot_id"`
	MeetingTitle   string `json:"meeting_title"`
	FailureSubCode string `json:"failure_sub_code"`
}

type BotStatusResponse struct {
	UserID       int    `json:"user_id"`
	BotID        string `json:"bot_id"`
	BotStatus    string `json:"bot_status"`
	MeetingTitle string `json:"meeting_title"`
	MeetingURL   string `json:"meeting_url"`
}

type RecallWebhookPayload struct {
	Event string                 `json:"event"`
	Data  map[string]interface{} `json:"data"`
}
```

- [ ] **Step 6: Write minimal main.go**

```go
// services/transcript-service/cmd/transcript/main.go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/services/transcript-service/internal/config"
	"github.com/devinat1/obsidian-meeting-notes/services/transcript-service/internal/database"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := database.RunMigrations(cfg.DatabaseURL); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	srv := &http.Server{
		Addr: ":" + cfg.Port,
	}

	go func() {
		log.Printf("Transcript service starting on port %s.", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down transcript service.")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}
}
```

- [ ] **Step 7: Verify compilation**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/transcript-service && go build ./cmd/transcript`
Expected: no errors

- [ ] **Step 8: Commit**

```bash
git add services/transcript-service/
git commit -m "feat(transcript-service): initialize project with database, migrations, config, and models"
```

---

### Task 11: Transcript conversion logic and Recall client

**Files:**
- Create: `services/transcript-service/internal/transcript/convert.go`
- Create: `services/transcript-service/internal/transcript/convert_test.go`
- Create: `services/transcript-service/internal/recall/client.go`
- Create: `services/transcript-service/internal/recall/client_test.go`

- [ ] **Step 1: Write convert.go**

Converts raw Recall transcript segments (speaker + words with timestamps) into readable blocks grouped by speaker with formatted timestamps.

```go
// services/transcript-service/internal/transcript/convert.go
package transcript

import (
	"fmt"
	"strings"

	"github.com/devinat1/obsidian-meeting-notes/services/transcript-service/internal/model"
)

func ConvertToReadable(params ConvertToReadableParams) []model.ReadableBlock {
	blocks := make([]model.ReadableBlock, 0, len(params.Segments))

	for _, segment := range params.Segments {
		if len(segment.Words) == 0 {
			continue
		}

		words := make([]string, 0, len(segment.Words))
		for _, w := range segment.Words {
			words = append(words, w.Text)
		}

		startSeconds := segment.Words[0].StartTime
		timestamp := formatTimestamp(startSeconds)

		text := strings.Join(words, " ")
		text = cleanTranscriptText(text)

		speaker := segment.Speaker
		if speaker == "" {
			speaker = "Unknown Speaker"
		}

		blocks = append(blocks, model.ReadableBlock{
			Speaker:   speaker,
			Timestamp: timestamp,
			Text:      text,
		})
	}

	return mergeConsecutiveSpeakerBlocks(blocks)
}

type ConvertToReadableParams struct {
	Segments []model.RawSegment
}

func mergeConsecutiveSpeakerBlocks(blocks []model.ReadableBlock) []model.ReadableBlock {
	if len(blocks) <= 1 {
		return blocks
	}

	merged := make([]model.ReadableBlock, 0, len(blocks))
	current := blocks[0]

	for i := 1; i < len(blocks); i++ {
		if blocks[i].Speaker == current.Speaker {
			current.Text = current.Text + " " + blocks[i].Text
		} else {
			merged = append(merged, current)
			current = blocks[i]
		}
	}
	merged = append(merged, current)

	return merged
}

func formatTimestamp(totalSeconds float64) string {
	minutes := int(totalSeconds) / 60
	seconds := int(totalSeconds) % 60
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}

func cleanTranscriptText(text string) string {
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "  ", " ")
	return text
}
```

- [ ] **Step 2: Write convert_test.go (TDD)**

```go
// services/transcript-service/internal/transcript/convert_test.go
package transcript

import (
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/transcript-service/internal/model"
)

func TestConvertToReadable_BasicSegments(t *testing.T) {
	segments := []model.RawSegment{
		{
			Speaker: "Alice",
			Words: []model.RawWord{
				{Text: "Hello", StartTime: 0.0, EndTime: 0.5},
				{Text: "everyone", StartTime: 0.5, EndTime: 1.0},
			},
		},
		{
			Speaker: "Bob",
			Words: []model.RawWord{
				{Text: "Hey", StartTime: 2.0, EndTime: 2.3},
				{Text: "Alice", StartTime: 2.3, EndTime: 2.6},
			},
		},
	}

	result := ConvertToReadable(ConvertToReadableParams{Segments: segments})

	if len(result) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(result))
	}
	if result[0].Speaker != "Alice" {
		t.Errorf("expected speaker Alice, got %s", result[0].Speaker)
	}
	if result[0].Timestamp != "0:00" {
		t.Errorf("expected timestamp 0:00, got %s", result[0].Timestamp)
	}
	if result[0].Text != "Hello everyone" {
		t.Errorf("expected text 'Hello everyone', got '%s'", result[0].Text)
	}
	if result[1].Speaker != "Bob" {
		t.Errorf("expected speaker Bob, got %s", result[1].Speaker)
	}
	if result[1].Timestamp != "0:02" {
		t.Errorf("expected timestamp 0:02, got %s", result[1].Timestamp)
	}
}

func TestConvertToReadable_MergeConsecutiveSpeakers(t *testing.T) {
	segments := []model.RawSegment{
		{
			Speaker: "Alice",
			Words:   []model.RawWord{{Text: "First", StartTime: 0.0, EndTime: 0.5}},
		},
		{
			Speaker: "Alice",
			Words:   []model.RawWord{{Text: "sentence", StartTime: 1.0, EndTime: 1.5}},
		},
		{
			Speaker: "Bob",
			Words:   []model.RawWord{{Text: "Response", StartTime: 3.0, EndTime: 3.5}},
		},
	}

	result := ConvertToReadable(ConvertToReadableParams{Segments: segments})

	if len(result) != 2 {
		t.Fatalf("expected 2 blocks (merged), got %d", len(result))
	}
	if result[0].Text != "First sentence" {
		t.Errorf("expected merged text 'First sentence', got '%s'", result[0].Text)
	}
}

func TestConvertToReadable_EmptySegments(t *testing.T) {
	segments := []model.RawSegment{
		{Speaker: "Alice", Words: []model.RawWord{}},
	}

	result := ConvertToReadable(ConvertToReadableParams{Segments: segments})

	if len(result) != 0 {
		t.Errorf("expected 0 blocks for empty words, got %d", len(result))
	}
}

func TestConvertToReadable_UnknownSpeaker(t *testing.T) {
	segments := []model.RawSegment{
		{
			Speaker: "",
			Words:   []model.RawWord{{Text: "Hello", StartTime: 0.0, EndTime: 0.5}},
		},
	}

	result := ConvertToReadable(ConvertToReadableParams{Segments: segments})

	if len(result) != 1 {
		t.Fatalf("expected 1 block, got %d", len(result))
	}
	if result[0].Speaker != "Unknown Speaker" {
		t.Errorf("expected 'Unknown Speaker', got '%s'", result[0].Speaker)
	}
}

func TestFormatTimestamp(t *testing.T) {
	tests := []struct {
		seconds  float64
		expected string
	}{
		{0.0, "0:00"},
		{5.5, "0:05"},
		{65.0, "1:05"},
		{3661.0, "61:01"},
	}

	for _, tt := range tests {
		got := formatTimestamp(tt.seconds)
		if got != tt.expected {
			t.Errorf("formatTimestamp(%v) = %q, want %q", tt.seconds, got, tt.expected)
		}
	}
}
```

- [ ] **Step 3: Write recall/client.go for fetching transcripts**

```go
// services/transcript-service/internal/recall/client.go
package recall

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/services/transcript-service/internal/model"
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

type NewClientParams struct {
	Region string
	APIKey string
}

func NewClient(params NewClientParams) *Client {
	return &Client{
		baseURL: fmt.Sprintf("https://%s.recall.ai/api/v1", params.Region),
		apiKey:  params.APIKey,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

type TranscriptMetadata struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	DownloadURL string `json:"download_url"`
}

type FetchTranscriptMetadataParams struct {
	TranscriptID string
}

func (c *Client) FetchTranscriptMetadata(ctx context.Context, params FetchTranscriptMetadataParams) (*TranscriptMetadata, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/transcript/"+params.TranscriptID+"/", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create transcript metadata request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("transcript metadata request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("transcript metadata request returned %d: %s", resp.StatusCode, string(body))
	}

	var metadata TranscriptMetadata
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to decode transcript metadata: %w", err)
	}

	return &metadata, nil
}

type DownloadTranscriptParams struct {
	DownloadURL string
}

func (c *Client) DownloadTranscript(ctx context.Context, params DownloadTranscriptParams) ([]model.RawSegment, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, params.DownloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create transcript download request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("transcript download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("transcript download returned %d: %s", resp.StatusCode, string(body))
	}

	var segments []model.RawSegment
	if err := json.NewDecoder(resp.Body).Decode(&segments); err != nil {
		return nil, fmt.Errorf("failed to decode transcript data: %w", err)
	}

	return segments, nil
}
```

- [ ] **Step 4: Write recall/client_test.go**

```go
// services/transcript-service/internal/recall/client_test.go
package recall

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/services/transcript-service/internal/model"
)

func TestFetchTranscriptMetadata_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/transcript/tr_123/" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Token test-key" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(TranscriptMetadata{
			ID:          "tr_123",
			Status:      "done",
			DownloadURL: "https://example.com/transcript.json",
		})
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		apiKey:     "test-key",
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	metadata, err := client.FetchTranscriptMetadata(context.Background(), FetchTranscriptMetadataParams{
		TranscriptID: "tr_123",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if metadata.ID != "tr_123" {
		t.Errorf("expected transcript ID tr_123, got %s", metadata.ID)
	}
	if metadata.DownloadURL != "https://example.com/transcript.json" {
		t.Errorf("unexpected download URL: %s", metadata.DownloadURL)
	}
}

func TestDownloadTranscript_Success(t *testing.T) {
	segments := []model.RawSegment{
		{
			Speaker: "Alice",
			Words: []model.RawWord{
				{Text: "Hello", StartTime: 0.0, EndTime: 0.5},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(segments)
	}))
	defer server.Close()

	client := &Client{
		baseURL:    server.URL,
		apiKey:     "test-key",
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	result, err := client.DownloadTranscript(context.Background(), DownloadTranscriptParams{
		DownloadURL: server.URL + "/download",
	})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(result))
	}
	if result[0].Speaker != "Alice" {
		t.Errorf("expected speaker Alice, got %s", result[0].Speaker)
	}
}
```

- [ ] **Step 5: Run all tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/transcript-service && go test ./...`
Expected: all tests pass

- [ ] **Step 6: Commit**

```bash
git add services/transcript-service/internal/transcript/ services/transcript-service/internal/recall/
git commit -m "feat(transcript-service): add transcript conversion logic and Recall API client with tests"
```

---

### Task 12: Transcript Service Kafka consumer, internal API, and full main wiring

**Files:**
- Create: `services/transcript-service/internal/consumer/transcript_events.go`
- Create: `services/transcript-service/internal/consumer/transcript_events_test.go`
- Create: `services/transcript-service/internal/handler/get_transcript.go`
- Create: `services/transcript-service/internal/handler/get_transcript_test.go`
- Create: `services/transcript-service/internal/router/router.go`
- Update: `services/transcript-service/cmd/transcript/main.go`

- [ ] **Step 1: Write transcript_events.go**

On `transcript.done`: resolve user_id + meeting_title from Bot Service internal API, fetch transcript from Recall, convert to readable, store in PG, publish `transcript.ready`.
On `transcript.failed`: resolve user context, store failure, publish `transcript.failed`.

```go
// services/transcript-service/internal/consumer/transcript_events.go
package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	kafkago "github.com/segmentio/kafka-go"

	"github.com/devinat1/obsidian-meeting-notes/services/transcript-service/internal/model"
	"github.com/devinat1/obsidian-meeting-notes/services/transcript-service/internal/recall"
	"github.com/devinat1/obsidian-meeting-notes/services/transcript-service/internal/transcript"
)

type TranscriptEventsConsumer struct {
	reader              *kafkago.Reader
	pool                *pgxpool.Pool
	recallClient        *recall.Client
	readyWriter         *kafkago.Writer
	failedWriter        *kafkago.Writer
	botServiceURL       string
	httpClient          *http.Client
}

type NewTranscriptEventsConsumerParams struct {
	Reader        *kafkago.Reader
	Pool          *pgxpool.Pool
	RecallClient  *recall.Client
	ReadyWriter   *kafkago.Writer
	FailedWriter  *kafkago.Writer
	BotServiceURL string
}

func NewTranscriptEventsConsumer(params NewTranscriptEventsConsumerParams) *TranscriptEventsConsumer {
	return &TranscriptEventsConsumer{
		reader:        params.Reader,
		pool:          params.Pool,
		recallClient:  params.RecallClient,
		readyWriter:   params.ReadyWriter,
		failedWriter:  params.FailedWriter,
		botServiceURL: params.BotServiceURL,
		httpClient:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *TranscriptEventsConsumer) Start(ctx context.Context) {
	log.Println("Starting transcript.events consumer.")
	for {
		msg, err := c.reader.ReadMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Println("Transcript events consumer shutting down.")
				return
			}
			log.Printf("Error reading transcript.events message: %v.", err)
			continue
		}

		if err := c.handleMessage(ctx, msg); err != nil {
			log.Printf("Error handling transcript.events message: %v.", err)
		}
	}
}

type recallTranscriptWebhook struct {
	Event string `json:"event"`
	Data  struct {
		BotID          string `json:"bot_id"`
		TranscriptID   string `json:"transcript_id"`
		FailureSubCode string `json:"failure_sub_code"`
	} `json:"data"`
}

func (c *TranscriptEventsConsumer) handleMessage(ctx context.Context, msg kafkago.Message) error {
	var webhook recallTranscriptWebhook
	if err := json.Unmarshal(msg.Value, &webhook); err != nil {
		return fmt.Errorf("failed to unmarshal transcript webhook: %w", err)
	}

	botID := string(msg.Key)
	if botID == "" || botID == "unknown" {
		botID = webhook.Data.BotID
	}

	switch webhook.Event {
	case "transcript.done":
		return c.handleTranscriptDone(ctx, botID, webhook.Data.TranscriptID)
	case "transcript.failed":
		return c.handleTranscriptFailed(ctx, botID, webhook.Data.FailureSubCode)
	default:
		log.Printf("Ignoring transcript event type: %s.", webhook.Event)
		return nil
	}
}

func (c *TranscriptEventsConsumer) handleTranscriptDone(ctx context.Context, botID string, transcriptID string) error {
	log.Printf("Processing transcript.done for bot %s, transcript %s.", botID, transcriptID)

	botStatus, err := c.fetchBotStatus(ctx, botID)
	if err != nil {
		return fmt.Errorf("failed to resolve bot info for bot %s: %w", botID, err)
	}

	metadata, err := c.recallClient.FetchTranscriptMetadata(ctx, recall.FetchTranscriptMetadataParams{
		TranscriptID: transcriptID,
	})
	if err != nil {
		return fmt.Errorf("failed to fetch transcript metadata: %w", err)
	}

	rawSegments, err := c.recallClient.DownloadTranscript(ctx, recall.DownloadTranscriptParams{
		DownloadURL: metadata.DownloadURL,
	})
	if err != nil {
		return fmt.Errorf("failed to download transcript: %w", err)
	}

	readableBlocks := transcript.ConvertToReadable(transcript.ConvertToReadableParams{
		Segments: rawSegments,
	})

	rawJSON, err := json.Marshal(rawSegments)
	if err != nil {
		return fmt.Errorf("failed to marshal raw transcript: %w", err)
	}
	readableJSON, err := json.Marshal(readableBlocks)
	if err != nil {
		return fmt.Errorf("failed to marshal readable transcript: %w", err)
	}

	_, err = c.pool.Exec(ctx,
		`INSERT INTO transcripts (bot_id, user_id, meeting_title, transcript_id, status, raw_transcript, readable_transcript)
		 VALUES ($1, $2, $3, $4, 'done', $5, $6)
		 ON CONFLICT (bot_id) DO UPDATE SET
		   status = 'done',
		   transcript_id = $4,
		   raw_transcript = $5,
		   readable_transcript = $6`,
		botID, botStatus.UserID, botStatus.MeetingTitle, transcriptID, rawJSON, readableJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to store transcript: %w", err)
	}

	readyEvent := model.TranscriptReadyEvent{
		UserID:             botStatus.UserID,
		BotID:              botID,
		MeetingTitle:       botStatus.MeetingTitle,
		RawTranscript:      rawSegments,
		ReadableTranscript: readableBlocks,
	}
	readyBytes, err := json.Marshal(readyEvent)
	if err != nil {
		return fmt.Errorf("failed to marshal transcript.ready event: %w", err)
	}

	err = c.readyWriter.WriteMessages(ctx, kafkago.Message{
		Key:   []byte(strconv.Itoa(botStatus.UserID)),
		Value: readyBytes,
	})
	if err != nil {
		return fmt.Errorf("failed to publish transcript.ready: %w", err)
	}

	log.Printf("Published transcript.ready for bot %s (user %d, meeting '%s').", botID, botStatus.UserID, botStatus.MeetingTitle)
	return nil
}

func (c *TranscriptEventsConsumer) handleTranscriptFailed(ctx context.Context, botID string, failureSubCode string) error {
	log.Printf("Processing transcript.failed for bot %s, failure: %s.", botID, failureSubCode)

	botStatus, err := c.fetchBotStatus(ctx, botID)
	if err != nil {
		return fmt.Errorf("failed to resolve bot info for bot %s: %w", botID, err)
	}

	_, err = c.pool.Exec(ctx,
		`INSERT INTO transcripts (bot_id, user_id, meeting_title, status, failure_sub_code)
		 VALUES ($1, $2, $3, 'failed', $4)
		 ON CONFLICT (bot_id) DO UPDATE SET
		   status = 'failed',
		   failure_sub_code = $4`,
		botID, botStatus.UserID, botStatus.MeetingTitle, failureSubCode,
	)
	if err != nil {
		return fmt.Errorf("failed to store transcript failure: %w", err)
	}

	failedEvent := model.TranscriptFailedEvent{
		UserID:         botStatus.UserID,
		BotID:          botID,
		MeetingTitle:   botStatus.MeetingTitle,
		FailureSubCode: failureSubCode,
	}
	failedBytes, err := json.Marshal(failedEvent)
	if err != nil {
		return fmt.Errorf("failed to marshal transcript.failed event: %w", err)
	}

	err = c.failedWriter.WriteMessages(ctx, kafkago.Message{
		Key:   []byte(strconv.Itoa(botStatus.UserID)),
		Value: failedBytes,
	})
	if err != nil {
		return fmt.Errorf("failed to publish transcript.failed: %w", err)
	}

	log.Printf("Published transcript.failed for bot %s (user %d).", botID, botStatus.UserID)
	return nil
}

func (c *TranscriptEventsConsumer) fetchBotStatus(ctx context.Context, botID string) (*model.BotStatusResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.botServiceURL+"/internal/bots/"+botID+"/status", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot status request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bot status request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bot status request returned %d: %s", resp.StatusCode, string(body))
	}

	var botStatus model.BotStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&botStatus); err != nil {
		return nil, fmt.Errorf("failed to decode bot status response: %w", err)
	}

	return &botStatus, nil
}
```

- [ ] **Step 2: Write transcript_events_test.go**

```go
// services/transcript-service/internal/consumer/transcript_events_test.go
package consumer

import (
	"encoding/json"
	"testing"
)

func TestRecallTranscriptWebhook_UnmarshalDone(t *testing.T) {
	raw := `{"event":"transcript.done","data":{"bot_id":"bot_123","transcript_id":"tr_456"}}`

	var webhook recallTranscriptWebhook
	if err := json.Unmarshal([]byte(raw), &webhook); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if webhook.Event != "transcript.done" {
		t.Errorf("expected event transcript.done, got %s", webhook.Event)
	}
	if webhook.Data.BotID != "bot_123" {
		t.Errorf("expected bot_id bot_123, got %s", webhook.Data.BotID)
	}
	if webhook.Data.TranscriptID != "tr_456" {
		t.Errorf("expected transcript_id tr_456, got %s", webhook.Data.TranscriptID)
	}
}

func TestRecallTranscriptWebhook_UnmarshalFailed(t *testing.T) {
	raw := `{"event":"transcript.failed","data":{"bot_id":"bot_789","failure_sub_code":"language_not_supported"}}`

	var webhook recallTranscriptWebhook
	if err := json.Unmarshal([]byte(raw), &webhook); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if webhook.Event != "transcript.failed" {
		t.Errorf("expected event transcript.failed, got %s", webhook.Event)
	}
	if webhook.Data.FailureSubCode != "language_not_supported" {
		t.Errorf("expected failure_sub_code language_not_supported, got %s", webhook.Data.FailureSubCode)
	}
}
```

- [ ] **Step 3: Write get_transcript.go**

```go
// services/transcript-service/internal/handler/get_transcript.go
package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/devinat1/obsidian-meeting-notes/services/transcript-service/internal/model"
)

type GetTranscriptHandler struct {
	pool *pgxpool.Pool
}

type NewGetTranscriptHandlerParams struct {
	Pool *pgxpool.Pool
}

func NewGetTranscriptHandler(params NewGetTranscriptHandlerParams) *GetTranscriptHandler {
	return &GetTranscriptHandler{pool: params.Pool}
}

func (h *GetTranscriptHandler) GetTranscript(w http.ResponseWriter, r *http.Request) {
	botID := chi.URLParam(r, "botId")
	if botID == "" {
		http.Error(w, "botId is required", http.StatusBadRequest)
		return
	}

	var t model.Transcript
	var rawJSON, readableJSON []byte
	err := h.pool.QueryRow(r.Context(),
		`SELECT id, bot_id, user_id, meeting_title, transcript_id, status, failure_sub_code, raw_transcript, readable_transcript, created_at
		 FROM transcripts WHERE bot_id = $1`,
		botID,
	).Scan(&t.ID, &t.BotID, &t.UserID, &t.MeetingTitle, &t.TranscriptID, &t.Status, &t.FailureSubCode, &rawJSON, &readableJSON, &t.CreatedAt)
	if err != nil {
		log.Printf("Transcript not found for bot_id %s: %v.", botID, err)
		http.Error(w, "transcript not found", http.StatusNotFound)
		return
	}

	if rawJSON != nil {
		var raw []model.RawSegment
		if err := json.Unmarshal(rawJSON, &raw); err == nil {
			t.RawTranscript = &raw
		}
	}
	if readableJSON != nil {
		var readable []model.ReadableBlock
		if err := json.Unmarshal(readableJSON, &readable); err == nil {
			t.ReadableTranscript = &readable
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(t)
}
```

- [ ] **Step 4: Write get_transcript_test.go**

```go
// services/transcript-service/internal/handler/get_transcript_test.go
package handler

import (
	"testing"
)

func TestGetTranscriptHandler_NewCreatesHandler(t *testing.T) {
	handler := NewGetTranscriptHandler(NewGetTranscriptHandlerParams{Pool: nil})
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}
```

- [ ] **Step 5: Write router.go**

```go
// services/transcript-service/internal/router/router.go
package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/devinat1/obsidian-meeting-notes/services/transcript-service/internal/handler"
)

type NewParams struct {
	GetTranscriptHandler *handler.GetTranscriptHandler
}

func New(params NewParams) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Get("/internal/transcripts/{botId}", params.GetTranscriptHandler.GetTranscript)

	return r
}
```

- [ ] **Step 6: Update main.go to wire everything together**

```go
// services/transcript-service/cmd/transcript/main.go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/devinat1/obsidian-meeting-notes/services/transcript-service/internal/config"
	"github.com/devinat1/obsidian-meeting-notes/services/transcript-service/internal/consumer"
	"github.com/devinat1/obsidian-meeting-notes/services/transcript-service/internal/database"
	"github.com/devinat1/obsidian-meeting-notes/services/transcript-service/internal/handler"
	"github.com/devinat1/obsidian-meeting-notes/services/transcript-service/internal/recall"
	"github.com/devinat1/obsidian-meeting-notes/services/transcript-service/internal/router"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := database.RunMigrations(cfg.DatabaseURL); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	recallClient := recall.NewClient(recall.NewClientParams{
		Region: cfg.RecallRegion,
		APIKey: cfg.RecallAPIKey,
	})

	brokers := strings.Split(cfg.KafkaBrokers, ",")

	readyWriter := &kafkago.Writer{
		Addr:         kafkago.TCP(brokers...),
		Topic:        "transcript.ready",
		Balancer:     &kafkago.LeastBytes{},
		RequiredAcks: kafkago.RequireOne,
		BatchTimeout: 10 * time.Millisecond,
	}
	defer readyWriter.Close()

	failedWriter := &kafkago.Writer{
		Addr:         kafkago.TCP(brokers...),
		Topic:        "transcript.failed",
		Balancer:     &kafkago.LeastBytes{},
		RequiredAcks: kafkago.RequireOne,
		BatchTimeout: 10 * time.Millisecond,
	}
	defer failedWriter.Close()

	transcriptEventsReader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:  brokers,
		Topic:    "transcript.events",
		GroupID:  "transcript-service-group",
		MinBytes: 1,
		MaxBytes: 10e6,
	})
	defer transcriptEventsReader.Close()

	transcriptEventsConsumer := consumer.NewTranscriptEventsConsumer(consumer.NewTranscriptEventsConsumerParams{
		Reader:        transcriptEventsReader,
		Pool:          pool,
		RecallClient:  recallClient,
		ReadyWriter:   readyWriter,
		FailedWriter:  failedWriter,
		BotServiceURL: cfg.BotServiceInternalURL,
	})

	go transcriptEventsConsumer.Start(ctx)

	getTranscriptHandler := handler.NewGetTranscriptHandler(handler.NewGetTranscriptHandlerParams{Pool: pool})

	r := router.New(router.NewParams{
		GetTranscriptHandler: getTranscriptHandler,
	})

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		log.Printf("Transcript service starting on port %s.", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down transcript service.")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}
}
```

- [ ] **Step 7: Run all tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/transcript-service && go test ./...`
Expected: all tests pass

- [ ] **Step 8: Commit**

```bash
git add services/transcript-service/
git commit -m "feat(transcript-service): add Kafka consumer, internal API, router, and full main wiring"
```

---

## Chunk 5: Transcript Service Dockerfile, Helm Charts, and Deploy Verification

### Task 13: Transcript Service Dockerfile and Helm charts

**Files:**
- Create: `services/transcript-service/Dockerfile`
- Create: `helm-meeting-transcript/Chart.yaml`
- Create: `helm-meeting-transcript/values.yaml`
- Create: `helm-meeting-transcript/templates/deployment.yaml`
- Create: `helm-meeting-transcript/templates/service.yaml`
- Create: `helm-meeting-transcript/templates/secret.yaml`
- Create: `helm-meeting-transcript-db/Chart.yaml`
- Create: `helm-meeting-transcript-db/values.yaml`
- Create: `helm-meeting-transcript-db/templates/deployment.yaml`
- Create: `helm-meeting-transcript-db/templates/service.yaml`
- Create: `helm-meeting-transcript-db/templates/secret.yaml`
- Create: `helm-meeting-transcript-db/templates/pvc.yaml`

- [ ] **Step 1: Write Transcript Service Dockerfile**

```dockerfile
# services/transcript-service/Dockerfile
FROM golang:1.22-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /transcript-service ./cmd/transcript

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /transcript-service /transcript-service
EXPOSE 8082
ENTRYPOINT ["/transcript-service"]
```

- [ ] **Step 2: Write Transcript DB Helm chart**

```yaml
# helm-meeting-transcript-db/Chart.yaml
apiVersion: v2
name: meeting-transcript-db
description: PostgreSQL for Transcript Service
type: application
version: 0.1.0
appVersion: "16"
```

```yaml
# helm-meeting-transcript-db/values.yaml
namespace: liteagent

postgres:
  image:
    repository: postgres
    tag: "16"
  port: 5432
  database: transcript_service
  user: transcript_service
  password: "CHANGEME"
  persistence:
    enabled: true
    size: 5Gi
    storageClass: local-path
  resources:
    requests:
      memory: "128Mi"
      cpu: "100m"
    limits:
      memory: "256Mi"
      cpu: "500m"

nodeSelector: {}
```

```yaml
# helm-meeting-transcript-db/templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: meeting-transcript-db
  namespace: {{ .Values.namespace }}
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: meeting-transcript-db
  template:
    metadata:
      labels:
        app: meeting-transcript-db
    spec:
      {{- with .Values.nodeSelector }}
      nodeSelector:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      containers:
        - name: postgres
          image: {{ .Values.postgres.image.repository }}:{{ .Values.postgres.image.tag }}
          ports:
            - containerPort: {{ .Values.postgres.port }}
              name: postgres
          env:
            - name: POSTGRES_DB
              valueFrom:
                secretKeyRef:
                  name: meeting-transcript-db-secret
                  key: POSTGRES_DB
            - name: POSTGRES_USER
              valueFrom:
                secretKeyRef:
                  name: meeting-transcript-db-secret
                  key: POSTGRES_USER
            - name: POSTGRES_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: meeting-transcript-db-secret
                  key: POSTGRES_PASSWORD
          resources:
            {{- toYaml .Values.postgres.resources | nindent 12 }}
          volumeMounts:
            - name: pgdata
              mountPath: /var/lib/postgresql/data
          readinessProbe:
            tcpSocket:
              port: {{ .Values.postgres.port }}
            initialDelaySeconds: 10
            periodSeconds: 10
          livenessProbe:
            tcpSocket:
              port: {{ .Values.postgres.port }}
            initialDelaySeconds: 30
            periodSeconds: 30
      volumes:
        {{- if .Values.postgres.persistence.enabled }}
        - name: pgdata
          persistentVolumeClaim:
            claimName: meeting-transcript-db-data
        {{- else }}
        - name: pgdata
          emptyDir: {}
        {{- end }}
```

```yaml
# helm-meeting-transcript-db/templates/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: meeting-transcript-db
  namespace: {{ .Values.namespace }}
spec:
  selector:
    app: meeting-transcript-db
  ports:
    - port: {{ .Values.postgres.port }}
      targetPort: {{ .Values.postgres.port }}
      protocol: TCP
      name: postgres
```

```yaml
# helm-meeting-transcript-db/templates/secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: meeting-transcript-db-secret
  namespace: {{ .Values.namespace }}
type: Opaque
stringData:
  POSTGRES_DB: {{ .Values.postgres.database | quote }}
  POSTGRES_USER: {{ .Values.postgres.user | quote }}
  POSTGRES_PASSWORD: {{ .Values.postgres.password | quote }}
```

```yaml
# helm-meeting-transcript-db/templates/pvc.yaml
{{- if .Values.postgres.persistence.enabled }}
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: meeting-transcript-db-data
  namespace: {{ .Values.namespace }}
spec:
  storageClassName: {{ .Values.postgres.persistence.storageClass }}
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: {{ .Values.postgres.persistence.size }}
{{- end }}
```

- [ ] **Step 3: Write Transcript Service Helm chart**

```yaml
# helm-meeting-transcript/Chart.yaml
apiVersion: v2
name: meeting-transcript
description: Transcript Service - transcript processing and storage
type: application
version: 0.1.0
appVersion: "latest"
```

```yaml
# helm-meeting-transcript/values.yaml
namespace: liteagent

transcript:
  image:
    repository: ghcr.io/devinat1/meeting-transcript-service
    tag: "latest"
  port: 8082
  resources:
    requests:
      memory: "64Mi"
      cpu: "50m"
    limits:
      memory: "256Mi"
      cpu: "500m"

env:
  DATABASE_URL: "postgres://transcript_service:CHANGEME@meeting-transcript-db.liteagent.svc.cluster.local:5432/transcript_service?sslmode=disable"
  KAFKA_BROKERS: "kafka.liteagent.svc.cluster.local:9092"
  RECALL_REGION: "us-west-2"
  RECALL_API_KEY: "CHANGEME"
  BOT_SERVICE_INTERNAL_URL: "http://meeting-bot.liteagent.svc.cluster.local:8081"

nodeSelector: {}
```

```yaml
# helm-meeting-transcript/templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: meeting-transcript
  namespace: {{ .Values.namespace }}
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: meeting-transcript
  template:
    metadata:
      labels:
        app: meeting-transcript
    spec:
      {{- with .Values.nodeSelector }}
      nodeSelector:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      containers:
        - name: meeting-transcript
          image: {{ .Values.transcript.image.repository }}:{{ .Values.transcript.image.tag }}
          ports:
            - containerPort: {{ .Values.transcript.port }}
              name: http
          env:
            - name: PORT
              value: {{ .Values.transcript.port | quote }}
            - name: DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: meeting-transcript-secret
                  key: DATABASE_URL
            - name: KAFKA_BROKERS
              valueFrom:
                secretKeyRef:
                  name: meeting-transcript-secret
                  key: KAFKA_BROKERS
            - name: RECALL_REGION
              valueFrom:
                secretKeyRef:
                  name: meeting-transcript-secret
                  key: RECALL_REGION
            - name: RECALL_API_KEY
              valueFrom:
                secretKeyRef:
                  name: meeting-transcript-secret
                  key: RECALL_API_KEY
            - name: BOT_SERVICE_INTERNAL_URL
              valueFrom:
                secretKeyRef:
                  name: meeting-transcript-secret
                  key: BOT_SERVICE_INTERNAL_URL
          resources:
            {{- toYaml .Values.transcript.resources | nindent 12 }}
          readinessProbe:
            httpGet:
              path: /healthz
              port: {{ .Values.transcript.port }}
            initialDelaySeconds: 10
            periodSeconds: 10
          livenessProbe:
            httpGet:
              path: /healthz
              port: {{ .Values.transcript.port }}
            initialDelaySeconds: 15
            periodSeconds: 30
      volumes: []
```

```yaml
# helm-meeting-transcript/templates/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: meeting-transcript
  namespace: {{ .Values.namespace }}
spec:
  selector:
    app: meeting-transcript
  ports:
    - port: {{ .Values.transcript.port }}
      targetPort: {{ .Values.transcript.port }}
      protocol: TCP
      name: http
```

```yaml
# helm-meeting-transcript/templates/secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: meeting-transcript-secret
  namespace: {{ .Values.namespace }}
type: Opaque
stringData:
  DATABASE_URL: {{ .Values.env.DATABASE_URL | quote }}
  KAFKA_BROKERS: {{ .Values.env.KAFKA_BROKERS | quote }}
  RECALL_REGION: {{ .Values.env.RECALL_REGION | quote }}
  RECALL_API_KEY: {{ .Values.env.RECALL_API_KEY | quote }}
  BOT_SERVICE_INTERNAL_URL: {{ .Values.env.BOT_SERVICE_INTERNAL_URL | quote }}
```

- [ ] **Step 4: Validate all Helm templates**

Run: `helm template meeting-transcript-db helm-meeting-transcript-db/ && helm template meeting-transcript helm-meeting-transcript/`
Expected: valid YAML output, no errors

- [ ] **Step 5: Commit**

```bash
git add services/transcript-service/Dockerfile helm-meeting-transcript/ helm-meeting-transcript-db/
git commit -m "feat(transcript-service): add Dockerfile, Helm charts for transcript service and transcript database"
```

---

### Task 14: Deploy all services to liteagent namespace

- [ ] **Step 1: Build and push Docker images**

```bash
cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin

docker build -t ghcr.io/devinat1/meeting-webhook-service:latest services/webhook-service/
docker build -t ghcr.io/devinat1/meeting-bot-service:latest services/bot-service/
docker build -t ghcr.io/devinat1/meeting-transcript-service:latest services/transcript-service/

docker push ghcr.io/devinat1/meeting-webhook-service:latest
docker push ghcr.io/devinat1/meeting-bot-service:latest
docker push ghcr.io/devinat1/meeting-transcript-service:latest
```

- [ ] **Step 2: Deploy databases first**

```bash
helm upgrade --install meeting-bot-db helm-meeting-bot-db/ -n liteagent
helm upgrade --install meeting-transcript-db helm-meeting-transcript-db/ -n liteagent
```

- [ ] **Step 3: Wait for databases to be ready**

```bash
kubectl -n liteagent wait --for=condition=Ready pod -l app=meeting-bot-db --timeout=120s
kubectl -n liteagent wait --for=condition=Ready pod -l app=meeting-transcript-db --timeout=120s
```

- [ ] **Step 4: Deploy application services**

```bash
helm upgrade --install meeting-webhook helm-meeting-webhook/ -n liteagent
helm upgrade --install meeting-bot helm-meeting-bot/ -n liteagent
helm upgrade --install meeting-transcript helm-meeting-transcript/ -n liteagent
```

- [ ] **Step 5: Verify all pods are running**

```bash
kubectl -n liteagent get pods -l 'app in (meeting-webhook,meeting-bot,meeting-transcript,meeting-bot-db,meeting-transcript-db)'
```

Expected: all 5 pods in `Running` state with `1/1` ready.

- [ ] **Step 6: Verify health endpoints**

```bash
kubectl -n liteagent port-forward svc/meeting-webhook 8080:8080 &
curl -s http://localhost:8080/healthz
kill %1

kubectl -n liteagent port-forward svc/meeting-bot 8081:8081 &
curl -s http://localhost:8081/healthz
kill %1

kubectl -n liteagent port-forward svc/meeting-transcript 8082:8082 &
curl -s http://localhost:8082/healthz
kill %1
```

Expected: all return `ok`

- [ ] **Step 7: Verify Kafka topic connectivity**

```bash
kubectl -n liteagent exec -it deploy/kafka -- /opt/kafka/bin/kafka-topics.sh --bootstrap-server localhost:9092 --list
```

Expected: should show `bot.events`, `recording.events`, `transcript.events`, `bot.status`, `transcript.ready`, `transcript.failed` topics (auto-created by producers/consumers).

- [ ] **Step 8: Commit any last adjustments**

```bash
git add -A
git commit -m "chore: finalize Plan 3 deployment configuration"
```

---

### Task 15: End-to-end smoke test

- [ ] **Step 1: Verify webhook-to-Kafka flow**

Send a test webhook to the Webhook Service and verify it arrives in the correct Kafka topic.

```bash
kubectl -n liteagent port-forward svc/meeting-webhook 8080:8080 &

curl -X POST http://localhost:8080/webhooks/recallai \
  -H "Content-Type: application/json" \
  -H "svix-id: msg_test_001" \
  -H "svix-timestamp: $(date +%s)" \
  -H "svix-signature: v1,TEST_SIGNATURE" \
  -d '{"event":"bot.status_change","data":{"bot_id":"test_bot_001","status":"in_call"}}'

kill %1
```

Expected: 401 (signature invalid) -- this confirms the verification logic rejects bad signatures. To test full flow, generate a valid signature using the configured secret.

- [ ] **Step 2: Verify Bot Service internal API**

```bash
kubectl -n liteagent port-forward svc/meeting-bot 8081:8081 &

curl -s http://localhost:8081/internal/bots/nonexistent/status
# Expected: 404 "bot not found"

kill %1
```

- [ ] **Step 3: Verify Transcript Service internal API**

```bash
kubectl -n liteagent port-forward svc/meeting-transcript 8082:8082 &

curl -s http://localhost:8082/internal/transcripts/nonexistent
# Expected: 404 "transcript not found"

kill %1
```

- [ ] **Step 4: Verify cross-service connectivity**

From the Transcript Service pod, verify it can reach the Bot Service internal API.

```bash
kubectl -n liteagent exec -it deploy/meeting-transcript -- wget -qO- http://meeting-bot.liteagent.svc.cluster.local:8081/healthz
```

Expected: `ok`

- [ ] **Step 5: Final commit**

```bash
git add -A
git commit -m "chore: complete Plan 3 smoke tests and verification"
```

---

## Summary

| Chunk | Tasks | Description |
|-------|-------|-------------|
| 1 | 1-4 | Webhook Service: project setup, Svix verification, webhook handler + Kafka routing, Dockerfile + Helm |
| 2 | 5-6 | Bot Service core: project + DB + migrations + models, Recall AI HTTP client |
| 3 | 7-9 | Bot Service consumers + API: 4 Kafka consumers, internal HTTP handlers, stale cleanup, Dockerfile + Helm |
| 4 | 10-12 | Transcript Service: project + DB, conversion logic + Recall client, Kafka consumer + internal API |
| 5 | 13-15 | Transcript Dockerfile + Helm, deploy all to liteagent, end-to-end smoke test |

**Total: 15 tasks across 5 chunks**

**Kafka topics produced:** `bot.events`, `recording.events`, `transcript.events` (Webhook Service), `bot.status` (Bot Service), `transcript.ready`, `transcript.failed` (Transcript Service)

**Kafka topics consumed:** `meeting.upcoming`, `meeting.cancelled`, `bot.events`, `recording.events` (Bot Service), `transcript.events` (Transcript Service)

**Internal APIs:**
- Bot Service: `DELETE /internal/bots/:botId`, `GET /internal/bots/:botId/status`
- Transcript Service: `GET /internal/transcripts/:botId`

**Databases:**
- PG-bot: `meeting-bot-db.liteagent.svc.cluster.local:5432/bot_service`
- PG-transcript: `meeting-transcript-db.liteagent.svc.cluster.local:5432/transcript_service`
