# Plan 2: Calendar Service

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Calendar Service microservice that manages Google OAuth token storage, polls Google Calendar per user, extracts meeting URLs (Meet/Zoom/Teams), and publishes `meeting.upcoming` and `meeting.cancelled` events to Kafka. Includes Postgres migrations for PG-cal, Dockerfile, and Helm chart.

**Architecture:** Chi-based internal HTTP API with per-user polling goroutines. Google OAuth2 token exchange and refresh via `golang.org/x/oauth2`. AES-256-GCM encryption for refresh tokens at rest. Kafka producer via `segmentio/kafka-go`. Postgres (PG-cal) via `pgx` for calendar connections and tracked meetings.

**Tech Stack:** Go 1.22+, chi router, pgx (Postgres driver), golang-migrate, google.golang.org/api (Calendar v3), golang.org/x/oauth2, segmentio/kafka-go, crypto/aes + crypto/cipher (AES-256-GCM)

**Spec:** `docs/superpowers/specs/2026-04-05-obsidian-meeting-notes-design.md`

**Depends on:** Plan 1 (shared Go module with Kafka helpers, config patterns, PG-cal Helm chart)

---

## File Structure

```
services/calendar-service/
├── cmd/
│   └── calendar/
│       └── main.go                        # Entrypoint: config, DB, Kafka, poller manager, HTTP server
├── internal/
│   ├── config/
│   │   └── config.go                      # Env var loading (Google OAuth, DB, Kafka, encryption key)
│   │   └── config_test.go
│   ├── database/
│   │   ├── database.go                    # pgx pool setup, migration runner
│   │   └── migrations/
│   │       ├── 001_calendar_connections.up.sql
│   │       ├── 001_calendar_connections.down.sql
│   │       ├── 002_tracked_meetings.up.sql
│   │       └── 002_tracked_meetings.down.sql
│   ├── encryption/
│   │   ├── aes.go                         # AES-256-GCM encrypt/decrypt for refresh tokens
│   │   └── aes_test.go
│   ├── model/
│   │   └── model.go                       # CalendarConnection, TrackedMeeting, API request/response types
│   ├── store/
│   │   ├── connection.go                  # CRUD for calendar_connections table
│   │   ├── connection_test.go
│   │   ├── meeting.go                     # CRUD + upsert for tracked_meetings table
│   │   └── meeting_test.go
│   ├── oauth/
│   │   ├── google.go                      # Google OAuth2 config, auth URL generation, token exchange
│   │   └── google_test.go
│   ├── calendar/
│   │   ├── client.go                      # Google Calendar API client (list events, extract meeting URLs)
│   │   ├── client_test.go
│   │   ├── urlextract.go                  # Meeting URL extraction (Meet, Zoom, Teams)
│   │   └── urlextract_test.go
│   ├── poller/
│   │   ├── manager.go                     # Manages per-user polling goroutines (start/stop/restart)
│   │   ├── manager_test.go
│   │   ├── worker.go                      # Single-user poll loop: fetch events, diff, produce Kafka
│   │   └── worker_test.go
│   ├── kafka/
│   │   ├── producer.go                    # Kafka producer for meeting.upcoming and meeting.cancelled
│   │   └── producer_test.go
│   └── handler/
│       ├── authurl.go                     # GET /internal/calendar/auth-url
│       ├── authurl_test.go
│       ├── callback.go                    # POST /internal/calendar/callback
│       ├── callback_test.go
│       ├── disconnect.go                  # DELETE /internal/calendar/disconnect
│       ├── disconnect_test.go
│       ├── meetings.go                    # GET /internal/calendar/meetings
│       ├── meetings_test.go
│       ├── status.go                      # GET /internal/calendar/status
│       └── status_test.go
├── go.mod
├── go.sum
├── Dockerfile
└── docker-compose.yml                     # Calendar service + Postgres for local dev
helm-meeting-calendar/
├── Chart.yaml
├── values.yaml
└── templates/
    ├── deployment.yaml
    ├── service.yaml
    └── secret.yaml
```

---

## Chunk 1: Project Setup, Config, Database & Encryption

### Task 1: Initialize Go module and dependencies

**Files:**
- Create: `services/calendar-service/go.mod`
- Create: `services/calendar-service/cmd/calendar/main.go`
- Create: `services/calendar-service/docker-compose.yml`

- [ ] **Step 1: Create Go module**

```bash
cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin
mkdir -p services/calendar-service/cmd/calendar
cd services/calendar-service
go mod init github.com/devinat1/obsidian-meeting-notes/services/calendar-service
```

- [ ] **Step 2: Add dependencies**

```bash
cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/calendar-service
go get github.com/go-chi/chi/v5
go get github.com/jackc/pgx/v5
go get github.com/golang-migrate/migrate/v4
go get github.com/golang-migrate/migrate/v4/database/postgres
go get github.com/golang-migrate/migrate/v4/source/file
go get golang.org/x/oauth2
go get golang.org/x/oauth2/google
go get google.golang.org/api/calendar/v3
go get google.golang.org/api/option
go get github.com/segmentio/kafka-go
```

- [ ] **Step 3: Create docker-compose.yml**

```yaml
# services/calendar-service/docker-compose.yml
services:
  db:
    image: postgres:16
    environment:
      POSTGRES_USER: calendar_service
      POSTGRES_PASSWORD: localdev
      POSTGRES_DB: calendar_service
    ports:
      - "5433:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data

  kafka:
    image: apache/kafka:4.0.2
    ports:
      - "9092:9092"
    environment:
      KAFKA_NODE_ID: "0"
      KAFKA_PROCESS_ROLES: broker,controller
      KAFKA_CONTROLLER_QUORUM_VOTERS: 0@localhost:9093
      KAFKA_LISTENERS: PLAINTEXT://:9092,CONTROLLER://:9093
      KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT
      KAFKA_CONTROLLER_LISTENER_NAMES: CONTROLLER
      KAFKA_INTER_BROKER_LISTENER_NAME: PLAINTEXT
      KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://localhost:9092
      KAFKA_LOG_DIRS: /var/kafka-logs
      CLUSTER_ID: MkU3OEVBNTcwNTJENDM2Qk

volumes:
  pgdata:
```

- [ ] **Step 4: Create minimal main.go**

```go
// services/calendar-service/cmd/calendar/main.go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	slog.Info("Calendar service starting.")

	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("Listening.", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed.", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down.")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Shutdown error.", "error", err)
	}
}
```

- [ ] **Step 5: Verify it compiles**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/calendar-service && go build ./cmd/calendar`
Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add services/calendar-service/
git commit -m "feat(calendar-service): initialize Go module with deps and minimal entrypoint"
```

---

### Task 2: Config loading from environment variables

**Files:**
- Create: `services/calendar-service/internal/config/config.go`
- Create: `services/calendar-service/internal/config/config_test.go`

- [ ] **Step 1: Write config.go**

```go
// services/calendar-service/internal/config/config.go
package config

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port                  string
	DatabaseURL           string
	KafkaBroker           string
	GoogleClientID        string
	GoogleClientSecret    string
	GoogleRedirectURI     string
	EncryptionKey         []byte // 32 bytes for AES-256
	DefaultPollInterval   int    // minutes
	DefaultBotJoinBefore  int    // minutes
}

func Load() (*Config, error) {
	encKeyHex := os.Getenv("ENCRYPTION_KEY")
	if encKeyHex == "" {
		return nil, fmt.Errorf("ENCRYPTION_KEY is required (64-char hex string for AES-256).")
	}
	encKey, err := hex.DecodeString(encKeyHex)
	if err != nil {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be valid hex: %w.", err)
	}
	if len(encKey) != 32 {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be exactly 32 bytes (64 hex chars), got %d bytes.", len(encKey))
	}

	c := &Config{
		Port:               getEnvOrDefault("PORT", "8081"),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		KafkaBroker:        getEnvOrDefault("KAFKA_BROKER", "kafka.liteagent.svc.cluster.local:9092"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURI:  getEnvOrDefault("GOOGLE_REDIRECT_URI", "https://api.liteagent.net/api/auth/calendar/callback"),
		EncryptionKey:      encKey,
		DefaultPollInterval:  parseIntOrDefault("DEFAULT_POLL_INTERVAL", 3),
		DefaultBotJoinBefore: parseIntOrDefault("DEFAULT_BOT_JOIN_BEFORE", 1),
	}

	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required.")
	}
	if c.GoogleClientID == "" {
		return nil, fmt.Errorf("GOOGLE_CLIENT_ID is required.")
	}
	if c.GoogleClientSecret == "" {
		return nil, fmt.Errorf("GOOGLE_CLIENT_SECRET is required.")
	}

	return c, nil
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseIntOrDefault(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return parsed
}
```

- [ ] **Step 2: Write config_test.go**

```go
// services/calendar-service/internal/config/config_test.go
package config

import (
	"os"
	"testing"
)

func TestLoad_AllRequiredVars(t *testing.T) {
	// 32-byte key as 64 hex chars
	testKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	t.Setenv("DATABASE_URL", "postgres://localhost:5433/calendar_service")
	t.Setenv("GOOGLE_CLIENT_ID", "test-client-id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "test-client-secret")
	t.Setenv("ENCRYPTION_KEY", testKey)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Expected no error, got: %v.", err)
	}

	if cfg.Port != "8081" {
		t.Errorf("Expected default port 8081, got %s.", cfg.Port)
	}
	if len(cfg.EncryptionKey) != 32 {
		t.Errorf("Expected 32-byte encryption key, got %d bytes.", len(cfg.EncryptionKey))
	}
	if cfg.DefaultPollInterval != 3 {
		t.Errorf("Expected default poll interval 3, got %d.", cfg.DefaultPollInterval)
	}
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	testKey := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	t.Setenv("ENCRYPTION_KEY", testKey)
	t.Setenv("GOOGLE_CLIENT_ID", "test")
	t.Setenv("GOOGLE_CLIENT_SECRET", "test")
	os.Unsetenv("DATABASE_URL")

	_, err := Load()
	if err == nil {
		t.Fatal("Expected error for missing DATABASE_URL.")
	}
}

func TestLoad_InvalidEncryptionKey(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("GOOGLE_CLIENT_ID", "test")
	t.Setenv("GOOGLE_CLIENT_SECRET", "test")
	t.Setenv("ENCRYPTION_KEY", "tooshort")

	_, err := Load()
	if err == nil {
		t.Fatal("Expected error for invalid ENCRYPTION_KEY.")
	}
}
```

- [ ] **Step 3: Run tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/calendar-service && go test ./internal/config/`
Expected: all tests pass

- [ ] **Step 4: Commit**

```bash
git add services/calendar-service/internal/config/
git commit -m "feat(calendar-service): add config loading with AES-256 key validation"
```

---

### Task 3: AES-256-GCM encryption module

**Files:**
- Create: `services/calendar-service/internal/encryption/aes.go`
- Create: `services/calendar-service/internal/encryption/aes_test.go`

- [ ] **Step 1: Write aes.go**

```go
// services/calendar-service/internal/encryption/aes.go
package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

type Encryptor struct {
	gcm cipher.AEAD
}

func NewEncryptor(key []byte) (*Encryptor, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("Encryption key must be 32 bytes for AES-256.")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("Failed to create AES cipher: %w.", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("Failed to create GCM: %w.", err)
	}

	return &Encryptor{gcm: gcm}, nil
}

// Encrypt encrypts plaintext and returns a base64-encoded string (nonce + ciphertext).
func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("Failed to generate nonce: %w.", err)
	}

	ciphertext := e.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64-encoded string (nonce + ciphertext) and returns the plaintext.
func (e *Encryptor) Decrypt(encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("Failed to decode base64: %w.", err)
	}

	nonceSize := e.gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("Ciphertext too short.")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := e.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("Failed to decrypt: %w.", err)
	}

	return string(plaintext), nil
}
```

- [ ] **Step 2: Write aes_test.go**

```go
// services/calendar-service/internal/encryption/aes_test.go
package encryption

import (
	"crypto/rand"
	"testing"
)

func testKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("Failed to generate test key: %v.", err)
	}
	return key
}

func TestEncryptDecrypt_Roundtrip(t *testing.T) {
	enc, err := NewEncryptor(testKey(t))
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v.", err)
	}

	original := "ya29.a0AfH6SMBx-some-refresh-token-value"
	encrypted, err := enc.Encrypt(original)
	if err != nil {
		t.Fatalf("Encrypt failed: %v.", err)
	}

	if encrypted == original {
		t.Error("Encrypted value should differ from original.")
	}

	decrypted, err := enc.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt failed: %v.", err)
	}

	if decrypted != original {
		t.Errorf("Roundtrip failed: got %q, want %q.", decrypted, original)
	}
}

func TestEncrypt_ProducesDifferentCiphertexts(t *testing.T) {
	enc, err := NewEncryptor(testKey(t))
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v.", err)
	}

	plaintext := "same-token-value"
	enc1, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("First encrypt failed: %v.", err)
	}
	enc2, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Second encrypt failed: %v.", err)
	}

	if enc1 == enc2 {
		t.Error("Two encryptions of same plaintext should produce different ciphertexts due to random nonce.")
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	enc1, err := NewEncryptor(testKey(t))
	if err != nil {
		t.Fatalf("Failed to create encryptor 1: %v.", err)
	}
	enc2, err := NewEncryptor(testKey(t))
	if err != nil {
		t.Fatalf("Failed to create encryptor 2: %v.", err)
	}

	encrypted, err := enc1.Encrypt("secret-token")
	if err != nil {
		t.Fatalf("Encrypt failed: %v.", err)
	}

	_, err = enc2.Decrypt(encrypted)
	if err == nil {
		t.Error("Decrypt with wrong key should fail.")
	}
}

func TestNewEncryptor_InvalidKeyLength(t *testing.T) {
	_, err := NewEncryptor([]byte("too-short"))
	if err == nil {
		t.Error("Expected error for invalid key length.")
	}
}

func TestDecrypt_InvalidBase64(t *testing.T) {
	enc, err := NewEncryptor(testKey(t))
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v.", err)
	}

	_, err = enc.Decrypt("not-valid-base64!!!")
	if err == nil {
		t.Error("Expected error for invalid base64.")
	}
}
```

- [ ] **Step 3: Run tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/calendar-service && go test ./internal/encryption/`
Expected: all tests pass

- [ ] **Step 4: Commit**

```bash
git add services/calendar-service/internal/encryption/
git commit -m "feat(calendar-service): add AES-256-GCM encryption for refresh token storage"
```

---

### Task 4: Database setup with migrations

**Files:**
- Create: `services/calendar-service/internal/database/database.go`
- Create: `services/calendar-service/internal/database/migrations/001_calendar_connections.up.sql`
- Create: `services/calendar-service/internal/database/migrations/001_calendar_connections.down.sql`
- Create: `services/calendar-service/internal/database/migrations/002_tracked_meetings.up.sql`
- Create: `services/calendar-service/internal/database/migrations/002_tracked_meetings.down.sql`

- [ ] **Step 1: Write migration 001_calendar_connections.up.sql**

```sql
-- services/calendar-service/internal/database/migrations/001_calendar_connections.up.sql
CREATE TABLE calendar_connections (
    id              SERIAL PRIMARY KEY,
    user_id         INTEGER NOT NULL UNIQUE,
    refresh_token   TEXT NOT NULL,
    poll_interval_minutes   INTEGER NOT NULL DEFAULT 3,
    bot_join_before_minutes INTEGER NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_calendar_connections_user_id ON calendar_connections(user_id);
```

- [ ] **Step 2: Write migration 001_calendar_connections.down.sql**

```sql
-- services/calendar-service/internal/database/migrations/001_calendar_connections.down.sql
DROP TABLE IF EXISTS calendar_connections;
```

- [ ] **Step 3: Write migration 002_tracked_meetings.up.sql**

```sql
-- services/calendar-service/internal/database/migrations/002_tracked_meetings.up.sql
CREATE TABLE tracked_meetings (
    id                  SERIAL PRIMARY KEY,
    user_id             INTEGER NOT NULL,
    calendar_event_id   TEXT NOT NULL,
    title               TEXT NOT NULL,
    start_time          TIMESTAMPTZ NOT NULL,
    end_time            TIMESTAMPTZ NOT NULL,
    meeting_url         TEXT NOT NULL,
    attendees           JSONB NOT NULL DEFAULT '[]'::jsonb,
    bot_dispatched      BOOLEAN NOT NULL DEFAULT FALSE,
    cancelled           BOOLEAN NOT NULL DEFAULT FALSE,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, calendar_event_id)
);

CREATE INDEX idx_tracked_meetings_user_id ON tracked_meetings(user_id);
CREATE INDEX idx_tracked_meetings_start_time ON tracked_meetings(start_time);
CREATE INDEX idx_tracked_meetings_user_dispatched ON tracked_meetings(user_id, bot_dispatched) WHERE bot_dispatched = FALSE;
```

- [ ] **Step 4: Write migration 002_tracked_meetings.down.sql**

```sql
-- services/calendar-service/internal/database/migrations/002_tracked_meetings.down.sql
DROP TABLE IF EXISTS tracked_meetings;
```

- [ ] **Step 5: Write database.go**

```go
// services/calendar-service/internal/database/database.go
package database

import (
	"context"
	"embed"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse database URL: %w.", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("Failed to create connection pool: %w.", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("Failed to ping database: %w.", err)
	}

	slog.Info("Database connection established.")
	return pool, nil
}

func RunMigrations(databaseURL string) error {
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("Failed to create migration source: %w.", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, databaseURL)
	if err != nil {
		return fmt.Errorf("Failed to create migrate instance: %w.", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("Migration failed: %w.", err)
	}

	version, dirty, _ := m.Version()
	slog.Info("Migrations complete.", "version", version, "dirty", dirty)
	return nil
}
```

- [ ] **Step 6: Verify it compiles**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/calendar-service && go build ./...`
Expected: no errors

- [ ] **Step 7: Integration test (manual)**

```bash
cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/calendar-service
docker compose up -d db
# Wait for Postgres to be ready
sleep 3
DATABASE_URL="postgres://calendar_service:localdev@localhost:5433/calendar_service?sslmode=disable" go run ./cmd/calendar &
# Health check
curl -s http://localhost:8081/healthz
# Stop
kill %1
docker compose down
```

- [ ] **Step 8: Commit**

```bash
git add services/calendar-service/internal/database/
git commit -m "feat(calendar-service): add database pool setup and PG-cal migrations"
```

---

## Chunk 2: Models, Store Layer & Google OAuth

### Task 5: Data models

**Files:**
- Create: `services/calendar-service/internal/model/model.go`

- [ ] **Step 1: Write model.go**

```go
// services/calendar-service/internal/model/model.go
package model

import (
	"encoding/json"
	"time"
)

// CalendarConnection represents a user's Google Calendar OAuth connection.
type CalendarConnection struct {
	ID                    int       `json:"id"`
	UserID                int       `json:"user_id"`
	RefreshToken          string    `json:"-"` // encrypted, never serialized
	PollIntervalMinutes   int       `json:"poll_interval_minutes"`
	BotJoinBeforeMinutes  int       `json:"bot_join_before_minutes"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// TrackedMeeting represents a calendar event with a video meeting link.
type TrackedMeeting struct {
	ID              int             `json:"id"`
	UserID          int             `json:"user_id"`
	CalendarEventID string          `json:"calendar_event_id"`
	Title           string          `json:"title"`
	StartTime       time.Time       `json:"start_time"`
	EndTime         time.Time       `json:"end_time"`
	MeetingURL      string          `json:"meeting_url"`
	Attendees       json.RawMessage `json:"attendees"`
	BotDispatched   bool            `json:"bot_dispatched"`
	Cancelled       bool            `json:"cancelled"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// MeetingUpcomingEvent is the Kafka payload for meeting.upcoming.
type MeetingUpcomingEvent struct {
	UserID          int       `json:"user_id"`
	CalendarEventID string    `json:"calendar_event_id"`
	MeetingURL      string    `json:"meeting_url"`
	Title           string    `json:"title"`
	StartTime       time.Time `json:"start_time"`
}

// MeetingCancelledEvent is the Kafka payload for meeting.cancelled.
type MeetingCancelledEvent struct {
	UserID          int    `json:"user_id"`
	CalendarEventID string `json:"calendar_event_id"`
	BotID           string `json:"bot_id,omitempty"`
}

// AuthURLResponse is the response from GET /internal/calendar/auth-url.
type AuthURLResponse struct {
	URL string `json:"url"`
}

// CallbackRequest is the request body for POST /internal/calendar/callback.
type CallbackRequest struct {
	Code   string `json:"code"`
	UserID int    `json:"user_id"`
}

// StatusResponse is the response from GET /internal/calendar/status.
type StatusResponse struct {
	Connected bool `json:"connected"`
}

// Attendee represents a meeting participant from Google Calendar.
type Attendee struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name,omitempty"`
	Self        bool   `json:"self,omitempty"`
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/calendar-service && go build ./...`

- [ ] **Step 3: Commit**

```bash
git add services/calendar-service/internal/model/
git commit -m "feat(calendar-service): add data models for connections, meetings, and Kafka events"
```

---

### Task 6: Store layer for calendar_connections

**Files:**
- Create: `services/calendar-service/internal/store/connection.go`
- Create: `services/calendar-service/internal/store/connection_test.go`

- [ ] **Step 1: Write connection.go**

```go
// services/calendar-service/internal/store/connection.go
package store

import (
	"context"
	"fmt"

	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/encryption"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ConnectionStore struct {
	pool      *pgxpool.Pool
	encryptor *encryption.Encryptor
}

func NewConnectionStore(pool *pgxpool.Pool, encryptor *encryption.Encryptor) *ConnectionStore {
	return &ConnectionStore{pool: pool, encryptor: encryptor}
}

func (s *ConnectionStore) Upsert(ctx context.Context, params UpsertConnectionParams) (*model.CalendarConnection, error) {
	encryptedToken, err := s.encryptor.Encrypt(params.RefreshToken)
	if err != nil {
		return nil, fmt.Errorf("Failed to encrypt refresh token: %w.", err)
	}

	var conn model.CalendarConnection
	err = s.pool.QueryRow(ctx, `
		INSERT INTO calendar_connections (user_id, refresh_token, poll_interval_minutes, bot_join_before_minutes)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id) DO UPDATE SET
			refresh_token = EXCLUDED.refresh_token,
			updated_at = NOW()
		RETURNING id, user_id, refresh_token, poll_interval_minutes, bot_join_before_minutes, created_at, updated_at
	`, params.UserID, encryptedToken, params.PollIntervalMinutes, params.BotJoinBeforeMinutes).Scan(
		&conn.ID, &conn.UserID, &conn.RefreshToken,
		&conn.PollIntervalMinutes, &conn.BotJoinBeforeMinutes,
		&conn.CreatedAt, &conn.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("Failed to upsert calendar connection: %w.", err)
	}

	return &conn, nil
}

type UpsertConnectionParams struct {
	UserID               int
	RefreshToken         string
	PollIntervalMinutes  int
	BotJoinBeforeMinutes int
}

func (s *ConnectionStore) GetByUserID(ctx context.Context, userID int) (*model.CalendarConnection, error) {
	var conn model.CalendarConnection
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, refresh_token, poll_interval_minutes, bot_join_before_minutes, created_at, updated_at
		FROM calendar_connections
		WHERE user_id = $1
	`, userID).Scan(
		&conn.ID, &conn.UserID, &conn.RefreshToken,
		&conn.PollIntervalMinutes, &conn.BotJoinBeforeMinutes,
		&conn.CreatedAt, &conn.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("Failed to get calendar connection: %w.", err)
	}

	return &conn, nil
}

// GetDecryptedRefreshToken retrieves and decrypts the refresh token for a user.
func (s *ConnectionStore) GetDecryptedRefreshToken(ctx context.Context, userID int) (string, error) {
	conn, err := s.GetByUserID(ctx, userID)
	if err != nil {
		return "", err
	}
	if conn == nil {
		return "", fmt.Errorf("No calendar connection found for user %d.", userID)
	}

	decrypted, err := s.encryptor.Decrypt(conn.RefreshToken)
	if err != nil {
		return "", fmt.Errorf("Failed to decrypt refresh token: %w.", err)
	}

	return decrypted, nil
}

func (s *ConnectionStore) GetAll(ctx context.Context) ([]model.CalendarConnection, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, refresh_token, poll_interval_minutes, bot_join_before_minutes, created_at, updated_at
		FROM calendar_connections
	`)
	if err != nil {
		return nil, fmt.Errorf("Failed to list calendar connections: %w.", err)
	}
	defer rows.Close()

	connections := []model.CalendarConnection{}
	for rows.Next() {
		var conn model.CalendarConnection
		if err := rows.Scan(
			&conn.ID, &conn.UserID, &conn.RefreshToken,
			&conn.PollIntervalMinutes, &conn.BotJoinBeforeMinutes,
			&conn.CreatedAt, &conn.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("Failed to scan calendar connection: %w.", err)
		}
		connections = append(connections, conn)
	}

	return connections, nil
}

func (s *ConnectionStore) DeleteByUserID(ctx context.Context, userID int) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM calendar_connections WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("Failed to delete calendar connection: %w.", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("No calendar connection found for user %d.", userID)
	}
	return nil
}
```

- [ ] **Step 2: Write connection_test.go (unit tests with interface mocking pattern)**

```go
// services/calendar-service/internal/store/connection_test.go
package store

import (
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/encryption"
)

func TestConnectionStore_EncryptionRoundtrip(t *testing.T) {
	// This test validates the encrypt/decrypt path without a database.
	// Full integration tests require a running Postgres instance.
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	enc, err := encryption.NewEncryptor(key)
	if err != nil {
		t.Fatalf("Failed to create encryptor: %v.", err)
	}

	token := "1//0e-refresh-token-from-google"
	encrypted, err := enc.Encrypt(token)
	if err != nil {
		t.Fatalf("Encrypt failed: %v.", err)
	}

	decrypted, err := enc.Decrypt(encrypted)
	if err != nil {
		t.Fatalf("Decrypt failed: %v.", err)
	}

	if decrypted != token {
		t.Errorf("Roundtrip failed: got %q, want %q.", decrypted, token)
	}
}
```

- [ ] **Step 3: Run tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/calendar-service && go test ./internal/store/`

- [ ] **Step 4: Commit**

```bash
git add services/calendar-service/internal/store/connection.go services/calendar-service/internal/store/connection_test.go
git commit -m "feat(calendar-service): add connection store with encrypted token upsert/get/delete"
```

---

### Task 7: Store layer for tracked_meetings

**Files:**
- Create: `services/calendar-service/internal/store/meeting.go`
- Create: `services/calendar-service/internal/store/meeting_test.go`

- [ ] **Step 1: Write meeting.go**

```go
// services/calendar-service/internal/store/meeting.go
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MeetingStore struct {
	pool *pgxpool.Pool
}

func NewMeetingStore(pool *pgxpool.Pool) *MeetingStore {
	return &MeetingStore{pool: pool}
}

type UpsertMeetingParams struct {
	UserID          int
	CalendarEventID string
	Title           string
	StartTime       time.Time
	EndTime         time.Time
	MeetingURL      string
	Attendees       json.RawMessage
}

// Upsert inserts or updates a tracked meeting. Returns the meeting and whether it was newly inserted.
func (s *MeetingStore) Upsert(ctx context.Context, params UpsertMeetingParams) (*model.TrackedMeeting, bool, error) {
	var meeting model.TrackedMeeting
	var isInsert bool

	err := s.pool.QueryRow(ctx, `
		INSERT INTO tracked_meetings (user_id, calendar_event_id, title, start_time, end_time, meeting_url, attendees)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (user_id, calendar_event_id) DO UPDATE SET
			title = EXCLUDED.title,
			start_time = EXCLUDED.start_time,
			end_time = EXCLUDED.end_time,
			meeting_url = EXCLUDED.meeting_url,
			attendees = EXCLUDED.attendees,
			updated_at = NOW()
		RETURNING id, user_id, calendar_event_id, title, start_time, end_time, meeting_url, attendees,
			bot_dispatched, cancelled, created_at, updated_at,
			(xmax = 0) AS is_insert
	`, params.UserID, params.CalendarEventID, params.Title, params.StartTime, params.EndTime, params.MeetingURL, params.Attendees).Scan(
		&meeting.ID, &meeting.UserID, &meeting.CalendarEventID, &meeting.Title,
		&meeting.StartTime, &meeting.EndTime, &meeting.MeetingURL, &meeting.Attendees,
		&meeting.BotDispatched, &meeting.Cancelled, &meeting.CreatedAt, &meeting.UpdatedAt,
		&isInsert,
	)
	if err != nil {
		return nil, false, fmt.Errorf("Failed to upsert tracked meeting: %w.", err)
	}

	return &meeting, isInsert, nil
}

func (s *MeetingStore) MarkBotDispatched(ctx context.Context, userID int, calendarEventID string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE tracked_meetings
		SET bot_dispatched = TRUE, updated_at = NOW()
		WHERE user_id = $1 AND calendar_event_id = $2
	`, userID, calendarEventID)
	if err != nil {
		return fmt.Errorf("Failed to mark bot dispatched: %w.", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("No tracked meeting found for user %d, event %s.", userID, calendarEventID)
	}
	return nil
}

func (s *MeetingStore) MarkCancelled(ctx context.Context, userID int, calendarEventID string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE tracked_meetings
		SET cancelled = TRUE, updated_at = NOW()
		WHERE user_id = $1 AND calendar_event_id = $2
	`, userID, calendarEventID)
	if err != nil {
		return fmt.Errorf("Failed to mark meeting cancelled: %w.", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("No tracked meeting found for user %d, event %s.", userID, calendarEventID)
	}
	return nil
}

// GetUpcomingByUserID returns non-cancelled meetings for a user starting within the given window.
func (s *MeetingStore) GetUpcomingByUserID(ctx context.Context, userID int, from time.Time, to time.Time) ([]model.TrackedMeeting, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, calendar_event_id, title, start_time, end_time, meeting_url, attendees,
			bot_dispatched, cancelled, created_at, updated_at
		FROM tracked_meetings
		WHERE user_id = $1 AND cancelled = FALSE AND start_time >= $2 AND start_time <= $3
		ORDER BY start_time ASC
	`, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("Failed to get upcoming meetings: %w.", err)
	}
	defer rows.Close()

	meetings := []model.TrackedMeeting{}
	for rows.Next() {
		var m model.TrackedMeeting
		if err := rows.Scan(
			&m.ID, &m.UserID, &m.CalendarEventID, &m.Title,
			&m.StartTime, &m.EndTime, &m.MeetingURL, &m.Attendees,
			&m.BotDispatched, &m.Cancelled, &m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("Failed to scan tracked meeting: %w.", err)
		}
		meetings = append(meetings, m)
	}

	return meetings, nil
}

// GetDispatchedEventIDs returns calendar_event_ids for dispatched, non-cancelled meetings for a user.
func (s *MeetingStore) GetDispatchedEventIDs(ctx context.Context, userID int) (map[string]bool, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT calendar_event_id
		FROM tracked_meetings
		WHERE user_id = $1 AND bot_dispatched = TRUE AND cancelled = FALSE
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("Failed to get dispatched event IDs: %w.", err)
	}
	defer rows.Close()

	result := map[string]bool{}
	for rows.Next() {
		var eventID string
		if err := rows.Scan(&eventID); err != nil {
			return nil, fmt.Errorf("Failed to scan event ID: %w.", err)
		}
		result[eventID] = true
	}

	return result, nil
}

func (s *MeetingStore) DeleteByUserID(ctx context.Context, userID int) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM tracked_meetings WHERE user_id = $1`, userID)
	if err != nil {
		return fmt.Errorf("Failed to delete tracked meetings: %w.", err)
	}
	return nil
}
```

- [ ] **Step 2: Write meeting_test.go**

```go
// services/calendar-service/internal/store/meeting_test.go
package store

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/model"
)

func TestUpsertMeetingParams_AttendeesJSON(t *testing.T) {
	attendees := []model.Attendee{
		{Email: "alice@example.com", DisplayName: "Alice"},
		{Email: "bob@example.com", DisplayName: "Bob"},
	}

	data, err := json.Marshal(attendees)
	if err != nil {
		t.Fatalf("Failed to marshal attendees: %v.", err)
	}

	params := UpsertMeetingParams{
		UserID:          1,
		CalendarEventID: "event-123",
		Title:           "Design Review",
		StartTime:       time.Now().Add(30 * time.Minute),
		EndTime:         time.Now().Add(90 * time.Minute),
		MeetingURL:      "https://meet.google.com/abc-def-ghi",
		Attendees:       json.RawMessage(data),
	}

	if params.CalendarEventID != "event-123" {
		t.Error("CalendarEventID mismatch.")
	}

	var decoded []model.Attendee
	if err := json.Unmarshal(params.Attendees, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal attendees: %v.", err)
	}
	if len(decoded) != 2 {
		t.Errorf("Expected 2 attendees, got %d.", len(decoded))
	}
}
```

- [ ] **Step 3: Run tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/calendar-service && go test ./internal/store/`

- [ ] **Step 4: Commit**

```bash
git add services/calendar-service/internal/store/meeting.go services/calendar-service/internal/store/meeting_test.go
git commit -m "feat(calendar-service): add meeting store with upsert, dispatch tracking, and cancellation"
```

---

### Task 8: Google OAuth module

**Files:**
- Create: `services/calendar-service/internal/oauth/google.go`
- Create: `services/calendar-service/internal/oauth/google_test.go`

- [ ] **Step 1: Write google.go**

```go
// services/calendar-service/internal/oauth/google.go
package oauth

import (
	"context"
	"fmt"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	calendarapi "google.golang.org/api/calendar/v3"
)

type GoogleOAuth struct {
	config *oauth2.Config
}

func NewGoogleOAuth(params NewGoogleOAuthParams) *GoogleOAuth {
	return &GoogleOAuth{
		config: &oauth2.Config{
			ClientID:     params.ClientID,
			ClientSecret: params.ClientSecret,
			RedirectURL:  params.RedirectURI,
			Scopes:       []string{calendarapi.CalendarReadonlyScope},
			Endpoint:     google.Endpoint,
		},
	}
}

type NewGoogleOAuthParams struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
}

// AuthURL generates the Google OAuth consent URL. State parameter should include the user_id.
func (g *GoogleOAuth) AuthURL(state string) string {
	return g.config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
}

// Exchange exchanges an authorization code for tokens.
func (g *GoogleOAuth) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := g.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("Failed to exchange OAuth code: %w.", err)
	}

	if token.RefreshToken == "" {
		return nil, fmt.Errorf("No refresh token received. User may need to re-authorize with consent prompt.")
	}

	return token, nil
}

// TokenSource returns an oauth2.TokenSource that automatically refreshes the access token.
func (g *GoogleOAuth) TokenSource(ctx context.Context, refreshToken string) oauth2.TokenSource {
	token := &oauth2.Token{
		RefreshToken: refreshToken,
	}
	return g.config.TokenSource(ctx, token)
}

// Config returns the underlying oauth2.Config for advanced use.
func (g *GoogleOAuth) Config() *oauth2.Config {
	return g.config
}
```

- [ ] **Step 2: Write google_test.go**

```go
// services/calendar-service/internal/oauth/google_test.go
package oauth

import (
	"strings"
	"testing"
)

func TestAuthURL_ContainsRequiredParams(t *testing.T) {
	g := NewGoogleOAuth(NewGoogleOAuthParams{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURI:  "https://api.liteagent.net/api/auth/calendar/callback",
	})

	url := g.AuthURL("user-state-123")

	if !strings.Contains(url, "accounts.google.com") {
		t.Error("Auth URL should point to Google accounts.")
	}
	if !strings.Contains(url, "test-client-id") {
		t.Error("Auth URL should contain client ID.")
	}
	if !strings.Contains(url, "user-state-123") {
		t.Error("Auth URL should contain state parameter.")
	}
	if !strings.Contains(url, "access_type=offline") {
		t.Error("Auth URL should request offline access for refresh token.")
	}
	if !strings.Contains(url, "calendar.readonly") {
		t.Error("Auth URL should request calendar.readonly scope.")
	}
}

func TestNewGoogleOAuth_SetsConfig(t *testing.T) {
	g := NewGoogleOAuth(NewGoogleOAuthParams{
		ClientID:     "my-id",
		ClientSecret: "my-secret",
		RedirectURI:  "https://example.com/callback",
	})

	cfg := g.Config()
	if cfg.ClientID != "my-id" {
		t.Errorf("Expected ClientID 'my-id', got %q.", cfg.ClientID)
	}
	if cfg.RedirectURL != "https://example.com/callback" {
		t.Errorf("Expected RedirectURL, got %q.", cfg.RedirectURL)
	}
}
```

- [ ] **Step 3: Run tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/calendar-service && go test ./internal/oauth/`

- [ ] **Step 4: Commit**

```bash
git add services/calendar-service/internal/oauth/
git commit -m "feat(calendar-service): add Google OAuth module with auth URL and token exchange"
```

---

## Chunk 3: Calendar Client, URL Extraction & Kafka Producer

### Task 9: Meeting URL extraction

**Files:**
- Create: `services/calendar-service/internal/calendar/urlextract.go`
- Create: `services/calendar-service/internal/calendar/urlextract_test.go`

- [ ] **Step 1: Write urlextract.go**

```go
// services/calendar-service/internal/calendar/urlextract.go
package calendar

import (
	"regexp"
	"strings"

	calendarapi "google.golang.org/api/calendar/v3"
)

var (
	zoomURLPattern  = regexp.MustCompile(`https?://[\w.-]*zoom\.us/j/\d+[^\s"<>]*`)
	teamsURLPattern = regexp.MustCompile(`https?://teams\.microsoft\.com/l/meetup-join/[^\s"<>]+`)
)

// ExtractMeetingURL extracts a video meeting URL from a Google Calendar event.
// Priority: Google Meet conferenceData > Zoom regex > Teams regex.
// Returns empty string if no meeting URL is found.
func ExtractMeetingURL(event *calendarapi.Event) string {
	// Google Meet: check conferenceData entry points.
	if event.ConferenceData != nil {
		for _, ep := range event.ConferenceData.EntryPoints {
			if ep.EntryPointType == "video" && ep.Uri != "" {
				return ep.Uri
			}
		}
	}

	// Zoom and Teams: regex on description and location.
	searchText := strings.Join([]string{event.Description, event.Location}, " ")

	if match := zoomURLPattern.FindString(searchText); match != "" {
		return match
	}

	if match := teamsURLPattern.FindString(searchText); match != "" {
		return match
	}

	return ""
}
```

- [ ] **Step 2: Write urlextract_test.go**

```go
// services/calendar-service/internal/calendar/urlextract_test.go
package calendar

import (
	"testing"

	calendarapi "google.golang.org/api/calendar/v3"
)

func TestExtractMeetingURL_GoogleMeet(t *testing.T) {
	event := &calendarapi.Event{
		ConferenceData: &calendarapi.ConferenceData{
			EntryPoints: []*calendarapi.EntryPoint{
				{EntryPointType: "phone", Uri: "tel:+1234567890"},
				{EntryPointType: "video", Uri: "https://meet.google.com/abc-defg-hij"},
			},
		},
	}

	url := ExtractMeetingURL(event)
	if url != "https://meet.google.com/abc-defg-hij" {
		t.Errorf("Expected Google Meet URL, got %q.", url)
	}
}

func TestExtractMeetingURL_Zoom_InDescription(t *testing.T) {
	event := &calendarapi.Event{
		Description: "Join us at https://zoom.us/j/1234567890?pwd=abc123 for the meeting.",
	}

	url := ExtractMeetingURL(event)
	if url != "https://zoom.us/j/1234567890?pwd=abc123" {
		t.Errorf("Expected Zoom URL, got %q.", url)
	}
}

func TestExtractMeetingURL_Zoom_InLocation(t *testing.T) {
	event := &calendarapi.Event{
		Location: "https://company.zoom.us/j/9876543210",
	}

	url := ExtractMeetingURL(event)
	if url != "https://company.zoom.us/j/9876543210" {
		t.Errorf("Expected Zoom URL, got %q.", url)
	}
}

func TestExtractMeetingURL_Teams(t *testing.T) {
	event := &calendarapi.Event{
		Description: "Click here to join: https://teams.microsoft.com/l/meetup-join/19%3ameeting_abc123 and dial in.",
	}

	url := ExtractMeetingURL(event)
	expected := "https://teams.microsoft.com/l/meetup-join/19%3ameeting_abc123"
	if url != expected {
		t.Errorf("Expected Teams URL %q, got %q.", expected, url)
	}
}

func TestExtractMeetingURL_NoMeetingURL(t *testing.T) {
	event := &calendarapi.Event{
		Description: "Regular meeting with no video link.",
		Location:    "Conference Room B",
	}

	url := ExtractMeetingURL(event)
	if url != "" {
		t.Errorf("Expected empty string for no meeting URL, got %q.", url)
	}
}

func TestExtractMeetingURL_GoogleMeet_Priority(t *testing.T) {
	// When both Meet conferenceData and Zoom in description exist, prefer Meet.
	event := &calendarapi.Event{
		ConferenceData: &calendarapi.ConferenceData{
			EntryPoints: []*calendarapi.EntryPoint{
				{EntryPointType: "video", Uri: "https://meet.google.com/abc-defg-hij"},
			},
		},
		Description: "Also available at https://zoom.us/j/1234567890",
	}

	url := ExtractMeetingURL(event)
	if url != "https://meet.google.com/abc-defg-hij" {
		t.Errorf("Expected Google Meet to take priority, got %q.", url)
	}
}

func TestExtractMeetingURL_EmptyEvent(t *testing.T) {
	event := &calendarapi.Event{}
	url := ExtractMeetingURL(event)
	if url != "" {
		t.Errorf("Expected empty string for empty event, got %q.", url)
	}
}
```

- [ ] **Step 3: Run tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/calendar-service && go test ./internal/calendar/`

- [ ] **Step 4: Commit**

```bash
git add services/calendar-service/internal/calendar/urlextract.go services/calendar-service/internal/calendar/urlextract_test.go
git commit -m "feat(calendar-service): add meeting URL extraction for Meet, Zoom, and Teams"
```

---

### Task 10: Google Calendar API client

**Files:**
- Create: `services/calendar-service/internal/calendar/client.go`
- Create: `services/calendar-service/internal/calendar/client_test.go`

- [ ] **Step 1: Write client.go**

```go
// services/calendar-service/internal/calendar/client.go
package calendar

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/model"
	"golang.org/x/oauth2"
	calendarapi "google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

type FetchEventsParams struct {
	TokenSource oauth2.TokenSource
	TimeMin     time.Time
	TimeMax     time.Time
}

type CalendarEvent struct {
	CalendarEventID string
	Title           string
	StartTime       time.Time
	EndTime         time.Time
	MeetingURL      string
	Attendees       json.RawMessage
}

// FetchUpcomingEvents fetches Google Calendar events with video meeting links within the given time range.
func (c *Client) FetchUpcomingEvents(ctx context.Context, params FetchEventsParams) ([]CalendarEvent, error) {
	svc, err := calendarapi.NewService(ctx, option.WithTokenSource(params.TokenSource))
	if err != nil {
		return nil, fmt.Errorf("Failed to create calendar service: %w.", err)
	}

	events, err := svc.Events.List("primary").
		TimeMin(params.TimeMin.Format(time.RFC3339)).
		TimeMax(params.TimeMax.Format(time.RFC3339)).
		SingleEvents(true).
		OrderBy("startTime").
		ShowDeleted(true).
		Fields("items(id,summary,start,end,conferenceData,description,location,attendees,status)").
		Do()
	if err != nil {
		return nil, fmt.Errorf("Failed to list calendar events: %w.", err)
	}

	result := []CalendarEvent{}
	for _, event := range events.Items {
		// Skip cancelled events (handled separately by poller for cancellation detection).
		if event.Status == "cancelled" {
			continue
		}

		meetingURL := ExtractMeetingURL(event)
		if meetingURL == "" {
			continue
		}

		startTime, err := parseEventTime(event.Start)
		if err != nil {
			slog.Warn("Failed to parse event start time, skipping.", "event_id", event.Id, "error", err)
			continue
		}

		endTime, err := parseEventTime(event.End)
		if err != nil {
			slog.Warn("Failed to parse event end time, skipping.", "event_id", event.Id, "error", err)
			continue
		}

		attendees := extractAttendees(event.Attendees)
		attendeesJSON, err := json.Marshal(attendees)
		if err != nil {
			slog.Warn("Failed to marshal attendees.", "event_id", event.Id, "error", err)
			attendeesJSON = []byte("[]")
		}

		result = append(result, CalendarEvent{
			CalendarEventID: event.Id,
			Title:           event.Summary,
			StartTime:       startTime,
			EndTime:         endTime,
			MeetingURL:      meetingURL,
			Attendees:       json.RawMessage(attendeesJSON),
		})
	}

	return result, nil
}

// FetchDeletedEventIDs returns event IDs that have been cancelled since we last polled.
func (c *Client) FetchDeletedEventIDs(ctx context.Context, params FetchEventsParams) ([]string, error) {
	svc, err := calendarapi.NewService(ctx, option.WithTokenSource(params.TokenSource))
	if err != nil {
		return nil, fmt.Errorf("Failed to create calendar service: %w.", err)
	}

	events, err := svc.Events.List("primary").
		TimeMin(params.TimeMin.Format(time.RFC3339)).
		TimeMax(params.TimeMax.Format(time.RFC3339)).
		SingleEvents(true).
		ShowDeleted(true).
		Fields("items(id,status)").
		Do()
	if err != nil {
		return nil, fmt.Errorf("Failed to list calendar events for deletion check: %w.", err)
	}

	deletedIDs := []string{}
	for _, event := range events.Items {
		if event.Status == "cancelled" {
			deletedIDs = append(deletedIDs, event.Id)
		}
	}

	return deletedIDs, nil
}

func parseEventTime(eventDateTime *calendarapi.EventDateTime) (time.Time, error) {
	if eventDateTime == nil {
		return time.Time{}, fmt.Errorf("Event date/time is nil.")
	}

	// Prefer DateTime (specific time) over Date (all-day event).
	if eventDateTime.DateTime != "" {
		return time.Parse(time.RFC3339, eventDateTime.DateTime)
	}

	if eventDateTime.Date != "" {
		return time.Parse("2006-01-02", eventDateTime.Date)
	}

	return time.Time{}, fmt.Errorf("Event has no date or datetime.")
}

func extractAttendees(gcalAttendees []*calendarapi.EventAttendee) []model.Attendee {
	attendees := []model.Attendee{}
	for _, a := range gcalAttendees {
		attendees = append(attendees, model.Attendee{
			Email:       a.Email,
			DisplayName: a.DisplayName,
			Self:        a.Self,
		})
	}
	return attendees
}
```

- [ ] **Step 2: Write client_test.go**

```go
// services/calendar-service/internal/calendar/client_test.go
package calendar

import (
	"testing"
	"time"

	calendarapi "google.golang.org/api/calendar/v3"
)

func TestParseEventTime_DateTime(t *testing.T) {
	edt := &calendarapi.EventDateTime{
		DateTime: "2026-04-05T10:00:00-05:00",
	}

	parsed, err := parseEventTime(edt)
	if err != nil {
		t.Fatalf("Unexpected error: %v.", err)
	}

	if parsed.Hour() != 10 {
		t.Errorf("Expected hour 10, got %d.", parsed.Hour())
	}
}

func TestParseEventTime_DateOnly(t *testing.T) {
	edt := &calendarapi.EventDateTime{
		Date: "2026-04-05",
	}

	parsed, err := parseEventTime(edt)
	if err != nil {
		t.Fatalf("Unexpected error: %v.", err)
	}

	if parsed.Day() != 5 {
		t.Errorf("Expected day 5, got %d.", parsed.Day())
	}
}

func TestParseEventTime_Nil(t *testing.T) {
	_, err := parseEventTime(nil)
	if err == nil {
		t.Error("Expected error for nil EventDateTime.")
	}
}

func TestExtractAttendees(t *testing.T) {
	gcalAttendees := []*calendarapi.EventAttendee{
		{Email: "alice@example.com", DisplayName: "Alice Smith", Self: false},
		{Email: "bob@example.com", DisplayName: "Bob Jones", Self: true},
	}

	attendees := extractAttendees(gcalAttendees)
	if len(attendees) != 2 {
		t.Fatalf("Expected 2 attendees, got %d.", len(attendees))
	}

	if attendees[0].Email != "alice@example.com" {
		t.Errorf("Expected alice email, got %q.", attendees[0].Email)
	}
	if attendees[1].Self != true {
		t.Error("Expected bob to be marked as self.")
	}
}

func TestExtractAttendees_Empty(t *testing.T) {
	attendees := extractAttendees(nil)
	if len(attendees) != 0 {
		t.Errorf("Expected 0 attendees for nil input, got %d.", len(attendees))
	}
}

func TestFetchEventsParams_TimeRange(t *testing.T) {
	now := time.Now()
	params := FetchEventsParams{
		TimeMin: now,
		TimeMax: now.Add(2 * time.Hour),
	}

	duration := params.TimeMax.Sub(params.TimeMin)
	if duration != 2*time.Hour {
		t.Errorf("Expected 2h range, got %v.", duration)
	}
}
```

- [ ] **Step 3: Run tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/calendar-service && go test ./internal/calendar/`

- [ ] **Step 4: Commit**

```bash
git add services/calendar-service/internal/calendar/client.go services/calendar-service/internal/calendar/client_test.go
git commit -m "feat(calendar-service): add Google Calendar API client with event fetching and attendee extraction"
```

---

### Task 11: Kafka producer

**Files:**
- Create: `services/calendar-service/internal/kafka/producer.go`
- Create: `services/calendar-service/internal/kafka/producer_test.go`

- [ ] **Step 1: Write producer.go**

```go
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
```

- [ ] **Step 2: Write producer_test.go**

```go
// services/calendar-service/internal/kafka/producer_test.go
package kafka

import (
	"testing"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/model"
)

func TestMeetingUpcomingEvent_JSON(t *testing.T) {
	event := model.MeetingUpcomingEvent{
		UserID:          42,
		CalendarEventID: "event-abc-123",
		MeetingURL:      "https://meet.google.com/abc-defg-hij",
		Title:           "Design Review",
		StartTime:       time.Date(2026, 4, 5, 10, 0, 0, 0, time.UTC),
	}

	if event.UserID != 42 {
		t.Errorf("Expected UserID 42, got %d.", event.UserID)
	}
	if event.Title != "Design Review" {
		t.Errorf("Expected title 'Design Review', got %q.", event.Title)
	}
}

func TestMeetingCancelledEvent_JSON(t *testing.T) {
	event := model.MeetingCancelledEvent{
		UserID:          42,
		CalendarEventID: "event-abc-123",
		BotID:           "bot-xyz-789",
	}

	if event.BotID != "bot-xyz-789" {
		t.Errorf("Expected BotID, got %q.", event.BotID)
	}
}

func TestTopicConstants(t *testing.T) {
	if TopicMeetingUpcoming != "meeting.upcoming" {
		t.Errorf("Unexpected topic name: %q.", TopicMeetingUpcoming)
	}
	if TopicMeetingCancelled != "meeting.cancelled" {
		t.Errorf("Unexpected topic name: %q.", TopicMeetingCancelled)
	}
}
```

- [ ] **Step 3: Run tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/calendar-service && go test ./internal/kafka/`

- [ ] **Step 4: Commit**

```bash
git add services/calendar-service/internal/kafka/
git commit -m "feat(calendar-service): add Kafka producer for meeting.upcoming and meeting.cancelled topics"
```

---

## Chunk 4: Poller Manager & Worker

### Task 12: Poller worker (single-user poll loop)

**Files:**
- Create: `services/calendar-service/internal/poller/worker.go`
- Create: `services/calendar-service/internal/poller/worker_test.go`

- [ ] **Step 1: Write worker.go**

```go
// services/calendar-service/internal/poller/worker.go
package poller

import (
	"context"
	"log/slog"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/calendar"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/kafka"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/model"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/store"
	"golang.org/x/oauth2"
)

type Worker struct {
	userID              int
	pollInterval        time.Duration
	botJoinBefore       time.Duration
	tokenSource         oauth2.TokenSource
	calendarClient      *calendar.Client
	meetingStore        *store.MeetingStore
	producer            *kafka.Producer
}

type NewWorkerParams struct {
	UserID              int
	PollIntervalMinutes int
	BotJoinBeforeMinutes int
	TokenSource         oauth2.TokenSource
	CalendarClient      *calendar.Client
	MeetingStore        *store.MeetingStore
	Producer            *kafka.Producer
}

func NewWorker(params NewWorkerParams) *Worker {
	return &Worker{
		userID:         params.UserID,
		pollInterval:   time.Duration(params.PollIntervalMinutes) * time.Minute,
		botJoinBefore:  time.Duration(params.BotJoinBeforeMinutes) * time.Minute,
		tokenSource:    params.TokenSource,
		calendarClient: params.CalendarClient,
		meetingStore:   params.MeetingStore,
		producer:       params.Producer,
	}
}

// Run starts the poll loop. Blocks until ctx is cancelled.
func (w *Worker) Run(ctx context.Context) {
	slog.Info("Poller worker started.", "user_id", w.userID, "interval", w.pollInterval)

	// Poll immediately on start, then on interval.
	w.poll(ctx)

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Poller worker stopped.", "user_id", w.userID)
			return
		case <-ticker.C:
			w.poll(ctx)
		}
	}
}

func (w *Worker) poll(ctx context.Context) {
	slog.Debug("Polling calendar.", "user_id", w.userID)

	now := time.Now()
	timeMin := now
	timeMax := now.Add(2 * time.Hour)

	// Fetch upcoming events from Google Calendar.
	events, err := w.calendarClient.FetchUpcomingEvents(ctx, calendar.FetchEventsParams{
		TokenSource: w.tokenSource,
		TimeMin:     timeMin,
		TimeMax:     timeMax,
	})
	if err != nil {
		slog.Error("Failed to fetch calendar events.", "user_id", w.userID, "error", err)
		return
	}

	// Track which event IDs we see in this poll cycle.
	seenEventIDs := map[string]bool{}

	for _, event := range events {
		seenEventIDs[event.CalendarEventID] = true

		// Upsert the meeting (updates title/time/url/attendees if changed).
		meeting, _, err := w.meetingStore.Upsert(ctx, store.UpsertMeetingParams{
			UserID:          w.userID,
			CalendarEventID: event.CalendarEventID,
			Title:           event.Title,
			StartTime:       event.StartTime,
			EndTime:         event.EndTime,
			MeetingURL:      event.MeetingURL,
			Attendees:       event.Attendees,
		})
		if err != nil {
			slog.Error("Failed to upsert tracked meeting.", "user_id", w.userID, "event_id", event.CalendarEventID, "error", err)
			continue
		}

		// Check if we should dispatch the bot.
		if meeting.BotDispatched || meeting.Cancelled {
			continue
		}

		dispatchThreshold := event.StartTime.Add(-w.botJoinBefore)
		if now.Before(dispatchThreshold) {
			continue
		}

		// Publish meeting.upcoming to Kafka.
		publishErr := w.producer.PublishMeetingUpcoming(ctx, model.MeetingUpcomingEvent{
			UserID:          w.userID,
			CalendarEventID: event.CalendarEventID,
			MeetingURL:      event.MeetingURL,
			Title:           event.Title,
			StartTime:       event.StartTime,
		})
		if publishErr != nil {
			slog.Error("Failed to publish meeting.upcoming.", "user_id", w.userID, "event_id", event.CalendarEventID, "error", publishErr)
			continue
		}

		// Mark as dispatched.
		if err := w.meetingStore.MarkBotDispatched(ctx, w.userID, event.CalendarEventID); err != nil {
			slog.Error("Failed to mark bot dispatched.", "user_id", w.userID, "event_id", event.CalendarEventID, "error", err)
		}
	}

	// Detect cancelled/deleted events.
	w.detectCancellations(ctx, seenEventIDs, timeMin, timeMax)
}

func (w *Worker) detectCancellations(ctx context.Context, seenEventIDs map[string]bool, timeMin time.Time, timeMax time.Time) {
	// Get dispatched event IDs from DB.
	dispatchedIDs, err := w.meetingStore.GetDispatchedEventIDs(ctx, w.userID)
	if err != nil {
		slog.Error("Failed to get dispatched event IDs.", "user_id", w.userID, "error", err)
		return
	}

	// Also check the Google Calendar API for explicitly cancelled events.
	deletedIDs, err := w.calendarClient.FetchDeletedEventIDs(ctx, calendar.FetchEventsParams{
		TokenSource: w.tokenSource,
		TimeMin:     timeMin,
		TimeMax:     timeMax,
	})
	if err != nil {
		slog.Warn("Failed to fetch deleted event IDs.", "user_id", w.userID, "error", err)
	}

	deletedSet := map[string]bool{}
	for _, id := range deletedIDs {
		deletedSet[id] = true
	}

	// A dispatched event is considered cancelled if:
	// 1. It no longer appears in the non-cancelled events list AND
	// 2. It appears in the deleted events list from Google
	for eventID := range dispatchedIDs {
		if seenEventIDs[eventID] {
			continue
		}
		if !deletedSet[eventID] {
			continue
		}

		slog.Info("Detected cancelled meeting.", "user_id", w.userID, "event_id", eventID)

		publishErr := w.producer.PublishMeetingCancelled(ctx, model.MeetingCancelledEvent{
			UserID:          w.userID,
			CalendarEventID: eventID,
		})
		if publishErr != nil {
			slog.Error("Failed to publish meeting.cancelled.", "user_id", w.userID, "event_id", eventID, "error", publishErr)
			continue
		}

		if err := w.meetingStore.MarkCancelled(ctx, w.userID, eventID); err != nil {
			slog.Error("Failed to mark meeting cancelled.", "user_id", w.userID, "event_id", eventID, "error", err)
		}
	}
}
```

- [ ] **Step 2: Write worker_test.go**

```go
// services/calendar-service/internal/poller/worker_test.go
package poller

import (
	"testing"
	"time"
)

func TestNewWorker_SetsIntervals(t *testing.T) {
	w := NewWorker(NewWorkerParams{
		UserID:               42,
		PollIntervalMinutes:  5,
		BotJoinBeforeMinutes: 2,
		// TokenSource, CalendarClient, MeetingStore, Producer left nil for unit test.
	})

	if w.userID != 42 {
		t.Errorf("Expected userID 42, got %d.", w.userID)
	}
	if w.pollInterval != 5*time.Minute {
		t.Errorf("Expected 5m poll interval, got %v.", w.pollInterval)
	}
	if w.botJoinBefore != 2*time.Minute {
		t.Errorf("Expected 2m bot join before, got %v.", w.botJoinBefore)
	}
}

func TestDispatchThreshold_Logic(t *testing.T) {
	botJoinBefore := 1 * time.Minute
	startTime := time.Date(2026, 4, 5, 10, 0, 0, 0, time.UTC)
	threshold := startTime.Add(-botJoinBefore)

	// 2 minutes before start: should NOT dispatch yet.
	twoMinBefore := startTime.Add(-2 * time.Minute)
	if !twoMinBefore.Before(threshold) {
		t.Error("2 minutes before start should be before the dispatch threshold.")
	}

	// 30 seconds before start: should dispatch.
	thirtySecBefore := startTime.Add(-30 * time.Second)
	if thirtySecBefore.Before(threshold) {
		t.Error("30 seconds before start should be after the dispatch threshold.")
	}

	// At start time: should dispatch.
	if startTime.Before(threshold) {
		t.Error("Start time should be after the dispatch threshold.")
	}
}
```

- [ ] **Step 3: Run tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/calendar-service && go test ./internal/poller/`

- [ ] **Step 4: Commit**

```bash
git add services/calendar-service/internal/poller/worker.go services/calendar-service/internal/poller/worker_test.go
git commit -m "feat(calendar-service): add poller worker with calendar polling, dispatch, and cancellation detection"
```

---

### Task 13: Poller manager (manages per-user goroutines)

**Files:**
- Create: `services/calendar-service/internal/poller/manager.go`
- Create: `services/calendar-service/internal/poller/manager_test.go`

- [ ] **Step 1: Write manager.go**

```go
// services/calendar-service/internal/poller/manager.go
package poller

import (
	"context"
	"log/slog"
	"sync"

	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/calendar"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/encryption"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/kafka"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/model"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/oauth"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/store"
)

type Manager struct {
	mu              sync.Mutex
	workers         map[int]context.CancelFunc // user_id -> cancel func
	calendarClient  *calendar.Client
	meetingStore    *store.MeetingStore
	connectionStore *store.ConnectionStore
	producer        *kafka.Producer
	googleOAuth     *oauth.GoogleOAuth
	encryptor       *encryption.Encryptor
}

type NewManagerParams struct {
	CalendarClient  *calendar.Client
	MeetingStore    *store.MeetingStore
	ConnectionStore *store.ConnectionStore
	Producer        *kafka.Producer
	GoogleOAuth     *oauth.GoogleOAuth
	Encryptor       *encryption.Encryptor
}

func NewManager(params NewManagerParams) *Manager {
	return &Manager{
		workers:         map[int]context.CancelFunc{},
		calendarClient:  params.CalendarClient,
		meetingStore:    params.MeetingStore,
		connectionStore: params.ConnectionStore,
		producer:        params.Producer,
		googleOAuth:     params.GoogleOAuth,
		encryptor:       params.Encryptor,
	}
}

// StartAll loads all calendar connections from DB and starts a poller for each.
func (m *Manager) StartAll(ctx context.Context) error {
	connections, err := m.connectionStore.GetAll(ctx)
	if err != nil {
		return err
	}

	for _, conn := range connections {
		m.startWorker(ctx, conn)
	}

	slog.Info("Poller manager started all workers.", "count", len(connections))
	return nil
}

// StartForUser starts (or restarts) the poller for a specific user.
func (m *Manager) StartForUser(ctx context.Context, conn model.CalendarConnection) {
	m.StopForUser(conn.UserID)
	m.startWorker(ctx, conn)
}

// StopForUser stops the poller for a specific user.
func (m *Manager) StopForUser(userID int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cancel, exists := m.workers[userID]; exists {
		cancel()
		delete(m.workers, userID)
		slog.Info("Stopped poller for user.", "user_id", userID)
	}
}

// StopAll stops all active pollers.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for userID, cancel := range m.workers {
		cancel()
		slog.Info("Stopped poller for user.", "user_id", userID)
	}
	m.workers = map[int]context.CancelFunc{}
}

// ActiveUserCount returns the number of active poller goroutines.
func (m *Manager) ActiveUserCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.workers)
}

// IsActive returns whether a poller is running for the given user.
func (m *Manager) IsActive(userID int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, exists := m.workers[userID]
	return exists
}

func (m *Manager) startWorker(ctx context.Context, conn model.CalendarConnection) {
	// Decrypt the refresh token.
	refreshToken, err := m.encryptor.Decrypt(conn.RefreshToken)
	if err != nil {
		slog.Error("Failed to decrypt refresh token for user, skipping.", "user_id", conn.UserID, "error", err)
		return
	}

	tokenSource := m.googleOAuth.TokenSource(ctx, refreshToken)

	workerCtx, cancel := context.WithCancel(ctx)

	m.mu.Lock()
	m.workers[conn.UserID] = cancel
	m.mu.Unlock()

	worker := NewWorker(NewWorkerParams{
		UserID:               conn.UserID,
		PollIntervalMinutes:  conn.PollIntervalMinutes,
		BotJoinBeforeMinutes: conn.BotJoinBeforeMinutes,
		TokenSource:          tokenSource,
		CalendarClient:       m.calendarClient,
		MeetingStore:         m.meetingStore,
		Producer:             m.producer,
	})

	go worker.Run(workerCtx)
}
```

- [ ] **Step 2: Write manager_test.go**

```go
// services/calendar-service/internal/poller/manager_test.go
package poller

import (
	"testing"
)

func TestManager_ActiveUserCount_StartsAtZero(t *testing.T) {
	m := NewManager(NewManagerParams{})

	if m.ActiveUserCount() != 0 {
		t.Errorf("Expected 0 active users, got %d.", m.ActiveUserCount())
	}
}

func TestManager_IsActive_ReturnsFalseForUnknownUser(t *testing.T) {
	m := NewManager(NewManagerParams{})

	if m.IsActive(999) {
		t.Error("Expected IsActive to return false for unknown user.")
	}
}

func TestManager_StopForUser_NoopForUnknownUser(t *testing.T) {
	m := NewManager(NewManagerParams{})

	// Should not panic.
	m.StopForUser(999)

	if m.ActiveUserCount() != 0 {
		t.Errorf("Expected 0 active users after noop stop, got %d.", m.ActiveUserCount())
	}
}

func TestManager_StopAll_EmptyManager(t *testing.T) {
	m := NewManager(NewManagerParams{})

	// Should not panic.
	m.StopAll()

	if m.ActiveUserCount() != 0 {
		t.Errorf("Expected 0 active users after StopAll, got %d.", m.ActiveUserCount())
	}
}
```

- [ ] **Step 3: Run tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/calendar-service && go test ./internal/poller/`

- [ ] **Step 4: Commit**

```bash
git add services/calendar-service/internal/poller/manager.go services/calendar-service/internal/poller/manager_test.go
git commit -m "feat(calendar-service): add poller manager for per-user goroutine lifecycle"
```

---

## Chunk 5: HTTP Handlers

### Task 14: Auth URL handler

**Files:**
- Create: `services/calendar-service/internal/handler/authurl.go`
- Create: `services/calendar-service/internal/handler/authurl_test.go`

- [ ] **Step 1: Write authurl.go**

```go
// services/calendar-service/internal/handler/authurl.go
package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/model"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/oauth"
)

type AuthURLHandler struct {
	googleOAuth *oauth.GoogleOAuth
}

func NewAuthURLHandler(googleOAuth *oauth.GoogleOAuth) *AuthURLHandler {
	return &AuthURLHandler{googleOAuth: googleOAuth}
}

// ServeHTTP handles GET /internal/calendar/auth-url?user_id=<id>
func (h *AuthURLHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		http.Error(w, `{"error":"user_id query parameter is required."}`, http.StatusBadRequest)
		return
	}

	_, err := strconv.Atoi(userIDStr)
	if err != nil {
		http.Error(w, `{"error":"user_id must be an integer."}`, http.StatusBadRequest)
		return
	}

	// Use user_id as OAuth state parameter for callback correlation.
	authURL := h.googleOAuth.AuthURL(userIDStr)

	slog.Info("Generated OAuth auth URL.", "user_id", userIDStr)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(model.AuthURLResponse{URL: authURL})
}
```

- [ ] **Step 2: Write authurl_test.go**

```go
// services/calendar-service/internal/handler/authurl_test.go
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/model"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/oauth"
)

func testOAuth() *oauth.GoogleOAuth {
	return oauth.NewGoogleOAuth(oauth.NewGoogleOAuthParams{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		RedirectURI:  "https://api.liteagent.net/api/auth/calendar/callback",
	})
}

func TestAuthURLHandler_Success(t *testing.T) {
	h := NewAuthURLHandler(testOAuth())

	req := httptest.NewRequest(http.MethodGet, "/internal/calendar/auth-url?user_id=42", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d.", rec.Code)
	}

	var resp model.AuthURLResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v.", err)
	}

	if !strings.Contains(resp.URL, "accounts.google.com") {
		t.Error("URL should contain Google accounts domain.")
	}
	if !strings.Contains(resp.URL, "42") {
		t.Error("URL state should contain user_id.")
	}
}

func TestAuthURLHandler_MissingUserID(t *testing.T) {
	h := NewAuthURLHandler(testOAuth())

	req := httptest.NewRequest(http.MethodGet, "/internal/calendar/auth-url", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d.", rec.Code)
	}
}

func TestAuthURLHandler_InvalidUserID(t *testing.T) {
	h := NewAuthURLHandler(testOAuth())

	req := httptest.NewRequest(http.MethodGet, "/internal/calendar/auth-url?user_id=abc", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d.", rec.Code)
	}
}
```

- [ ] **Step 3: Run tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/calendar-service && go test ./internal/handler/`

- [ ] **Step 4: Commit**

```bash
git add services/calendar-service/internal/handler/authurl.go services/calendar-service/internal/handler/authurl_test.go
git commit -m "feat(calendar-service): add auth URL handler for Google OAuth flow initiation"
```

---

### Task 15: Callback handler

**Files:**
- Create: `services/calendar-service/internal/handler/callback.go`
- Create: `services/calendar-service/internal/handler/callback_test.go`

- [ ] **Step 1: Write callback.go**

```go
// services/calendar-service/internal/handler/callback.go
package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/oauth"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/model"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/poller"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/store"
)

type CallbackHandler struct {
	googleOAuth     *oauth.GoogleOAuth
	connectionStore *store.ConnectionStore
	pollerManager   *poller.Manager
	defaultPollInterval  int
	defaultBotJoinBefore int
}

type NewCallbackHandlerParams struct {
	GoogleOAuth          *oauth.GoogleOAuth
	ConnectionStore      *store.ConnectionStore
	PollerManager        *poller.Manager
	DefaultPollInterval  int
	DefaultBotJoinBefore int
}

func NewCallbackHandler(params NewCallbackHandlerParams) *CallbackHandler {
	return &CallbackHandler{
		googleOAuth:          params.GoogleOAuth,
		connectionStore:      params.ConnectionStore,
		pollerManager:        params.PollerManager,
		defaultPollInterval:  params.DefaultPollInterval,
		defaultBotJoinBefore: params.DefaultBotJoinBefore,
	}
}

// ServeHTTP handles POST /internal/calendar/callback with JSON body {code, user_id}.
func (h *CallbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req model.CallbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request body."}`, http.StatusBadRequest)
		return
	}

	if req.Code == "" {
		http.Error(w, `{"error":"code is required."}`, http.StatusBadRequest)
		return
	}
	if req.UserID == 0 {
		http.Error(w, `{"error":"user_id is required."}`, http.StatusBadRequest)
		return
	}

	// Exchange the authorization code for tokens.
	token, err := h.googleOAuth.Exchange(r.Context(), req.Code)
	if err != nil {
		slog.Error("Failed to exchange OAuth code.", "user_id", req.UserID, "error", err)
		http.Error(w, `{"error":"Failed to exchange authorization code."}`, http.StatusBadGateway)
		return
	}

	// Store the encrypted refresh token.
	conn, err := h.connectionStore.Upsert(r.Context(), store.UpsertConnectionParams{
		UserID:               req.UserID,
		RefreshToken:         token.RefreshToken,
		PollIntervalMinutes:  h.defaultPollInterval,
		BotJoinBeforeMinutes: h.defaultBotJoinBefore,
	})
	if err != nil {
		slog.Error("Failed to store calendar connection.", "user_id", req.UserID, "error", err)
		http.Error(w, `{"error":"Failed to store calendar connection."}`, http.StatusInternalServerError)
		return
	}

	// Start the poller for this user.
	h.pollerManager.StartForUser(r.Context(), *conn)

	slog.Info("Calendar connected and poller started.", "user_id", req.UserID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "connected"})
}
```

- [ ] **Step 2: Write callback_test.go**

```go
// services/calendar-service/internal/handler/callback_test.go
package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCallbackHandler_MissingCode(t *testing.T) {
	h := &CallbackHandler{}

	body, _ := json.Marshal(map[string]interface{}{"user_id": 42})
	req := httptest.NewRequest(http.MethodPost, "/internal/calendar/callback", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d.", rec.Code)
	}
}

func TestCallbackHandler_MissingUserID(t *testing.T) {
	h := &CallbackHandler{}

	body, _ := json.Marshal(map[string]interface{}{"code": "auth-code-123"})
	req := httptest.NewRequest(http.MethodPost, "/internal/calendar/callback", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d.", rec.Code)
	}
}

func TestCallbackHandler_InvalidBody(t *testing.T) {
	h := &CallbackHandler{}

	req := httptest.NewRequest(http.MethodPost, "/internal/calendar/callback", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d.", rec.Code)
	}
}
```

- [ ] **Step 3: Run tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/calendar-service && go test ./internal/handler/`

- [ ] **Step 4: Commit**

```bash
git add services/calendar-service/internal/handler/callback.go services/calendar-service/internal/handler/callback_test.go
git commit -m "feat(calendar-service): add callback handler for OAuth token exchange and poller start"
```

---

### Task 16: Disconnect, meetings, and status handlers

**Files:**
- Create: `services/calendar-service/internal/handler/disconnect.go`
- Create: `services/calendar-service/internal/handler/disconnect_test.go`
- Create: `services/calendar-service/internal/handler/meetings.go`
- Create: `services/calendar-service/internal/handler/meetings_test.go`
- Create: `services/calendar-service/internal/handler/status.go`
- Create: `services/calendar-service/internal/handler/status_test.go`

- [ ] **Step 1: Write disconnect.go**

```go
// services/calendar-service/internal/handler/disconnect.go
package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/poller"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/store"
)

type DisconnectHandler struct {
	connectionStore *store.ConnectionStore
	meetingStore    *store.MeetingStore
	pollerManager   *poller.Manager
}

type NewDisconnectHandlerParams struct {
	ConnectionStore *store.ConnectionStore
	MeetingStore    *store.MeetingStore
	PollerManager   *poller.Manager
}

func NewDisconnectHandler(params NewDisconnectHandlerParams) *DisconnectHandler {
	return &DisconnectHandler{
		connectionStore: params.ConnectionStore,
		meetingStore:    params.MeetingStore,
		pollerManager:   params.PollerManager,
	}
}

// ServeHTTP handles DELETE /internal/calendar/disconnect?user_id=<id>
func (h *DisconnectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		http.Error(w, `{"error":"user_id query parameter is required."}`, http.StatusBadRequest)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		http.Error(w, `{"error":"user_id must be an integer."}`, http.StatusBadRequest)
		return
	}

	// Stop the poller first.
	h.pollerManager.StopForUser(userID)

	// Delete tracked meetings.
	if err := h.meetingStore.DeleteByUserID(r.Context(), userID); err != nil {
		slog.Warn("Failed to delete tracked meetings during disconnect.", "user_id", userID, "error", err)
	}

	// Delete the calendar connection (and encrypted token).
	if err := h.connectionStore.DeleteByUserID(r.Context(), userID); err != nil {
		slog.Error("Failed to delete calendar connection.", "user_id", userID, "error", err)
		http.Error(w, `{"error":"Failed to disconnect calendar."}`, http.StatusInternalServerError)
		return
	}

	slog.Info("Calendar disconnected.", "user_id", userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "disconnected"})
}
```

- [ ] **Step 2: Write disconnect_test.go**

```go
// services/calendar-service/internal/handler/disconnect_test.go
package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDisconnectHandler_MissingUserID(t *testing.T) {
	h := &DisconnectHandler{}

	req := httptest.NewRequest(http.MethodDelete, "/internal/calendar/disconnect", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d.", rec.Code)
	}
}

func TestDisconnectHandler_InvalidUserID(t *testing.T) {
	h := &DisconnectHandler{}

	req := httptest.NewRequest(http.MethodDelete, "/internal/calendar/disconnect?user_id=abc", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d.", rec.Code)
	}
}
```

- [ ] **Step 3: Write meetings.go**

```go
// services/calendar-service/internal/handler/meetings.go
package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/store"
)

type MeetingsHandler struct {
	meetingStore *store.MeetingStore
}

func NewMeetingsHandler(meetingStore *store.MeetingStore) *MeetingsHandler {
	return &MeetingsHandler{meetingStore: meetingStore}
}

// ServeHTTP handles GET /internal/calendar/meetings?user_id=<id>
func (h *MeetingsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		http.Error(w, `{"error":"user_id query parameter is required."}`, http.StatusBadRequest)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		http.Error(w, `{"error":"user_id must be an integer."}`, http.StatusBadRequest)
		return
	}

	now := time.Now()
	meetings, err := h.meetingStore.GetUpcomingByUserID(r.Context(), userID, now, now.Add(24*time.Hour))
	if err != nil {
		slog.Error("Failed to get upcoming meetings.", "user_id", userID, "error", err)
		http.Error(w, `{"error":"Failed to retrieve meetings."}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"meetings": meetings})
}
```

- [ ] **Step 4: Write meetings_test.go**

```go
// services/calendar-service/internal/handler/meetings_test.go
package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMeetingsHandler_MissingUserID(t *testing.T) {
	h := &MeetingsHandler{}

	req := httptest.NewRequest(http.MethodGet, "/internal/calendar/meetings", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d.", rec.Code)
	}
}

func TestMeetingsHandler_InvalidUserID(t *testing.T) {
	h := &MeetingsHandler{}

	req := httptest.NewRequest(http.MethodGet, "/internal/calendar/meetings?user_id=xyz", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d.", rec.Code)
	}
}
```

- [ ] **Step 5: Write status.go**

```go
// services/calendar-service/internal/handler/status.go
package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/model"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/store"
)

type StatusHandler struct {
	connectionStore *store.ConnectionStore
}

func NewStatusHandler(connectionStore *store.ConnectionStore) *StatusHandler {
	return &StatusHandler{connectionStore: connectionStore}
}

// ServeHTTP handles GET /internal/calendar/status?user_id=<id>
func (h *StatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		http.Error(w, `{"error":"user_id query parameter is required."}`, http.StatusBadRequest)
		return
	}

	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		http.Error(w, `{"error":"user_id must be an integer."}`, http.StatusBadRequest)
		return
	}

	conn, err := h.connectionStore.GetByUserID(r.Context(), userID)
	if err != nil {
		slog.Error("Failed to check calendar status.", "user_id", userID, "error", err)
		http.Error(w, `{"error":"Failed to check status."}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(model.StatusResponse{Connected: conn != nil})
}
```

- [ ] **Step 6: Write status_test.go**

```go
// services/calendar-service/internal/handler/status_test.go
package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStatusHandler_MissingUserID(t *testing.T) {
	h := &StatusHandler{}

	req := httptest.NewRequest(http.MethodGet, "/internal/calendar/status", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d.", rec.Code)
	}
}

func TestStatusHandler_InvalidUserID(t *testing.T) {
	h := &StatusHandler{}

	req := httptest.NewRequest(http.MethodGet, "/internal/calendar/status?user_id=abc", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400, got %d.", rec.Code)
	}
}
```

- [ ] **Step 7: Run all handler tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/calendar-service && go test ./internal/handler/`

- [ ] **Step 8: Commit**

```bash
git add services/calendar-service/internal/handler/
git commit -m "feat(calendar-service): add disconnect, meetings, and status HTTP handlers"
```

---

## Chunk 6: Main Entrypoint Wiring & Full Test Suite

### Task 17: Wire everything in main.go

**Files:**
- Modify: `services/calendar-service/cmd/calendar/main.go`

- [ ] **Step 1: Update main.go with full wiring**

```go
// services/calendar-service/cmd/calendar/main.go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/calendar"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/config"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/database"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/encryption"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/handler"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/kafka"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/oauth"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/poller"
	"github.com/devinat1/obsidian-meeting-notes/services/calendar-service/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	slog.Info("Calendar service starting.")

	// Load config.
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load config.", "error", err)
		os.Exit(1)
	}

	// Initialize database.
	ctx := context.Background()

	if err := database.RunMigrations(cfg.DatabaseURL); err != nil {
		slog.Error("Failed to run migrations.", "error", err)
		os.Exit(1)
	}

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("Failed to connect to database.", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Initialize encryption.
	encryptor, err := encryption.NewEncryptor(cfg.EncryptionKey)
	if err != nil {
		slog.Error("Failed to initialize encryptor.", "error", err)
		os.Exit(1)
	}

	// Initialize stores.
	connectionStore := store.NewConnectionStore(pool, encryptor)
	meetingStore := store.NewMeetingStore(pool)

	// Initialize Google OAuth.
	googleOAuth := oauth.NewGoogleOAuth(oauth.NewGoogleOAuthParams{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		RedirectURI:  cfg.GoogleRedirectURI,
	})

	// Initialize Kafka producer.
	producer := kafka.NewProducer(cfg.KafkaBroker)
	defer producer.Close()

	// Initialize calendar client.
	calendarClient := calendar.NewClient()

	// Initialize poller manager.
	pollerManager := poller.NewManager(poller.NewManagerParams{
		CalendarClient:  calendarClient,
		MeetingStore:    meetingStore,
		ConnectionStore: connectionStore,
		Producer:        producer,
		GoogleOAuth:     googleOAuth,
		Encryptor:       encryptor,
	})

	// Start all existing pollers.
	if err := pollerManager.StartAll(ctx); err != nil {
		slog.Error("Failed to start pollers.", "error", err)
		os.Exit(1)
	}
	defer pollerManager.StopAll()

	// Initialize handlers.
	authURLHandler := handler.NewAuthURLHandler(googleOAuth)
	callbackHandler := handler.NewCallbackHandler(handler.NewCallbackHandlerParams{
		GoogleOAuth:          googleOAuth,
		ConnectionStore:      connectionStore,
		PollerManager:        pollerManager,
		DefaultPollInterval:  cfg.DefaultPollInterval,
		DefaultBotJoinBefore: cfg.DefaultBotJoinBefore,
	})
	disconnectHandler := handler.NewDisconnectHandler(handler.NewDisconnectHandlerParams{
		ConnectionStore: connectionStore,
		MeetingStore:    meetingStore,
		PollerManager:   pollerManager,
	})
	meetingsHandler := handler.NewMeetingsHandler(meetingStore)
	statusHandler := handler.NewStatusHandler(connectionStore)

	// Set up router.
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	r.Route("/internal/calendar", func(r chi.Router) {
		r.Get("/auth-url", authURLHandler.ServeHTTP)
		r.Post("/callback", callbackHandler.ServeHTTP)
		r.Delete("/disconnect", disconnectHandler.ServeHTTP)
		r.Get("/meetings", meetingsHandler.ServeHTTP)
		r.Get("/status", statusHandler.ServeHTTP)
	})

	// Start server.
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("Listening.", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed.", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutting down.")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("Shutdown error.", "error", err)
	}

	slog.Info("Calendar service stopped.")
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/calendar-service && go build ./cmd/calendar`
Expected: no errors

- [ ] **Step 3: Run the full test suite**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/calendar-service && go test ./...`
Expected: all tests pass

- [ ] **Step 4: Commit**

```bash
git add services/calendar-service/cmd/calendar/main.go
git commit -m "feat(calendar-service): wire all components in main entrypoint with chi router"
```

---

## Chunk 7: Dockerfile & Helm Chart

### Task 18: Dockerfile

**Files:**
- Create: `services/calendar-service/Dockerfile`

- [ ] **Step 1: Write Dockerfile**

```dockerfile
# services/calendar-service/Dockerfile
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /calendar-service ./cmd/calendar

FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /calendar-service /calendar-service

EXPOSE 8081

ENTRYPOINT ["/calendar-service"]
```

- [ ] **Step 2: Verify Docker build**

```bash
cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/calendar-service
docker build -t meeting-calendar-service:dev .
```
Expected: successful build

- [ ] **Step 3: Commit**

```bash
git add services/calendar-service/Dockerfile
git commit -m "feat(calendar-service): add multi-stage Dockerfile"
```

---

### Task 19: Helm chart

**Files:**
- Create: `helm-meeting-calendar/Chart.yaml`
- Create: `helm-meeting-calendar/values.yaml`
- Create: `helm-meeting-calendar/templates/deployment.yaml`
- Create: `helm-meeting-calendar/templates/service.yaml`
- Create: `helm-meeting-calendar/templates/secret.yaml`

- [ ] **Step 1: Write Chart.yaml**

```yaml
# helm-meeting-calendar/Chart.yaml
apiVersion: v2
name: meeting-calendar
description: Calendar Service - Google Calendar polling and meeting event producer
type: application
version: 0.1.0
appVersion: "latest"
```

- [ ] **Step 2: Write values.yaml**

```yaml
# helm-meeting-calendar/values.yaml
namespace: liteagent

calendarService:
  image:
    repository: meeting-calendar-service
    tag: "latest"
  port: 8081
  resources:
    requests:
      memory: "128Mi"
      cpu: "100m"
    limits:
      memory: "256Mi"
      cpu: "500m"

env:
  DATABASE_URL: ""
  KAFKA_BROKER: "kafka.liteagent.svc.cluster.local:9092"
  GOOGLE_CLIENT_ID: ""
  GOOGLE_CLIENT_SECRET: ""
  GOOGLE_REDIRECT_URI: "https://api.liteagent.net/api/auth/calendar/callback"
  ENCRYPTION_KEY: ""
  DEFAULT_POLL_INTERVAL: "3"
  DEFAULT_BOT_JOIN_BEFORE: "1"

nodeSelector: {}
```

- [ ] **Step 3: Write templates/deployment.yaml**

```yaml
# helm-meeting-calendar/templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: meeting-calendar
  namespace: {{ .Values.namespace }}
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: meeting-calendar
  template:
    metadata:
      labels:
        app: meeting-calendar
    spec:
      {{- with .Values.nodeSelector }}
      nodeSelector:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      containers:
        - name: calendar-service
          image: {{ .Values.calendarService.image.repository }}:{{ .Values.calendarService.image.tag }}
          ports:
            - containerPort: {{ .Values.calendarService.port }}
              name: http
          env:
            - name: PORT
              value: {{ .Values.calendarService.port | quote }}
            - name: DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: meeting-calendar-secret
                  key: DATABASE_URL
            - name: KAFKA_BROKER
              value: {{ .Values.env.KAFKA_BROKER | quote }}
            - name: GOOGLE_CLIENT_ID
              valueFrom:
                secretKeyRef:
                  name: meeting-calendar-secret
                  key: GOOGLE_CLIENT_ID
            - name: GOOGLE_CLIENT_SECRET
              valueFrom:
                secretKeyRef:
                  name: meeting-calendar-secret
                  key: GOOGLE_CLIENT_SECRET
            - name: GOOGLE_REDIRECT_URI
              value: {{ .Values.env.GOOGLE_REDIRECT_URI | quote }}
            - name: ENCRYPTION_KEY
              valueFrom:
                secretKeyRef:
                  name: meeting-calendar-secret
                  key: ENCRYPTION_KEY
            - name: DEFAULT_POLL_INTERVAL
              value: {{ .Values.env.DEFAULT_POLL_INTERVAL | quote }}
            - name: DEFAULT_BOT_JOIN_BEFORE
              value: {{ .Values.env.DEFAULT_BOT_JOIN_BEFORE | quote }}
          resources:
            {{- toYaml .Values.calendarService.resources | nindent 12 }}
          readinessProbe:
            httpGet:
              path: /healthz
              port: {{ .Values.calendarService.port }}
            initialDelaySeconds: 5
            periodSeconds: 10
            failureThreshold: 3
          livenessProbe:
            httpGet:
              path: /healthz
              port: {{ .Values.calendarService.port }}
            initialDelaySeconds: 10
            periodSeconds: 30
            failureThreshold: 5
```

- [ ] **Step 4: Write templates/service.yaml**

```yaml
# helm-meeting-calendar/templates/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: meeting-calendar
  namespace: {{ .Values.namespace }}
spec:
  selector:
    app: meeting-calendar
  ports:
    - port: {{ .Values.calendarService.port }}
      targetPort: {{ .Values.calendarService.port }}
      protocol: TCP
      name: http
```

- [ ] **Step 5: Write templates/secret.yaml**

```yaml
# helm-meeting-calendar/templates/secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: meeting-calendar-secret
  namespace: {{ .Values.namespace }}
type: Opaque
stringData:
  DATABASE_URL: {{ .Values.env.DATABASE_URL | quote }}
  GOOGLE_CLIENT_ID: {{ .Values.env.GOOGLE_CLIENT_ID | quote }}
  GOOGLE_CLIENT_SECRET: {{ .Values.env.GOOGLE_CLIENT_SECRET | quote }}
  ENCRYPTION_KEY: {{ .Values.env.ENCRYPTION_KEY | quote }}
```

- [ ] **Step 6: Validate Helm chart**

```bash
cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin
helm lint helm-meeting-calendar/
```
Expected: no errors

- [ ] **Step 7: Dry-run template rendering**

```bash
cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin
helm template meeting-calendar helm-meeting-calendar/ \
  --set env.DATABASE_URL="postgres://cal:pass@pg-cal:5432/calendar?sslmode=disable" \
  --set env.GOOGLE_CLIENT_ID="test-id" \
  --set env.GOOGLE_CLIENT_SECRET="test-secret" \
  --set env.ENCRYPTION_KEY="0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
```
Expected: valid YAML output with all templates rendered

- [ ] **Step 8: Commit**

```bash
git add helm-meeting-calendar/
git commit -m "feat(calendar-service): add Helm chart for Kubernetes deployment"
```

---

### Task 20: Deploy to liteagent namespace

- [ ] **Step 1: Build and push Docker image**

```bash
cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services/calendar-service
docker build -t meeting-calendar-service:latest .
# Tag and push to your registry (adjust for your registry):
# docker tag meeting-calendar-service:latest <registry>/meeting-calendar-service:latest
# docker push <registry>/meeting-calendar-service:latest
```

- [ ] **Step 2: Create values override for production**

Create `helm-meeting-calendar/values-prod.yaml` with actual secrets (do NOT commit this file):

```yaml
# helm-meeting-calendar/values-prod.yaml (DO NOT COMMIT)
namespace: liteagent

calendarService:
  image:
    repository: meeting-calendar-service
    tag: "latest"

env:
  DATABASE_URL: "postgres://calendar_service:<password>@pg-cal.liteagent.svc.cluster.local:5432/calendar_service?sslmode=disable"
  KAFKA_BROKER: "kafka.liteagent.svc.cluster.local:9092"
  GOOGLE_CLIENT_ID: "<your-google-client-id>"
  GOOGLE_CLIENT_SECRET: "<your-google-client-secret>"
  GOOGLE_REDIRECT_URI: "https://api.liteagent.net/api/auth/calendar/callback"
  ENCRYPTION_KEY: "<64-char-hex-key>"
```

- [ ] **Step 3: Deploy with Helm**

```bash
cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin
helm upgrade --install meeting-calendar helm-meeting-calendar/ \
  -n liteagent \
  -f helm-meeting-calendar/values-prod.yaml
```

- [ ] **Step 4: Verify deployment**

```bash
kubectl -n liteagent get pods -l app=meeting-calendar
kubectl -n liteagent logs -l app=meeting-calendar --tail=20
kubectl -n liteagent get svc meeting-calendar
```
Expected: pod running, logs show "Calendar service starting." and "Migrations complete.", service exists on port 8081

- [ ] **Step 5: Smoke test internal API**

```bash
# Port-forward to test internally
kubectl -n liteagent port-forward svc/meeting-calendar 8081:8081 &
curl -s http://localhost:8081/healthz
# Expected: "ok"
curl -s http://localhost:8081/internal/calendar/status?user_id=1
# Expected: {"connected":false}
kill %1
```

- [ ] **Step 6: Commit .gitignore update (if needed)**

```bash
echo "helm-meeting-calendar/values-prod.yaml" >> .gitignore
git add .gitignore
git commit -m "chore: ignore production Helm values with secrets"
```

---

## Verification Checklist

After all chunks are complete, verify end-to-end:

- [ ] `go test ./...` passes from `services/calendar-service/`
- [ ] `go build ./cmd/calendar` compiles cleanly
- [ ] `docker build` succeeds
- [ ] `helm lint helm-meeting-calendar/` passes
- [ ] `helm template` renders valid YAML
- [ ] Pod is running in `liteagent` namespace
- [ ] `/healthz` returns 200
- [ ] `/internal/calendar/status?user_id=1` returns `{"connected":false}`
- [ ] `/internal/calendar/auth-url?user_id=1` returns a Google OAuth URL
- [ ] Kafka topics `meeting.upcoming` and `meeting.cancelled` are created on first publish
- [ ] Encrypted refresh tokens in `calendar_connections` table are not readable as plaintext
