# Go Backend Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the Go backend that owns the Recall AI integration and exposes a REST API for bot creation, status polling, transcript retrieval, and user auth.

**Architecture:** Chi-based HTTP server with PostgreSQL for state, Recall AI HTTP client for bot management, Svix HMAC webhook verification, and SHA-256 API key auth. Background goroutines handle stale bot cleanup and unverified user cleanup.

**Tech Stack:** Go 1.22+, chi router, pgx (Postgres driver), golang-migrate, crypto/sha256, net/http client for Recall API

**Spec:** `docs/superpowers/specs/2026-04-05-obsidian-meeting-notes-design.md` (Component 1)

---

## File Structure

```
backend/
├── cmd/
│   └── server/
│       └── main.go                  # Entrypoint: config loading, DB connect, router setup, start server
├── internal/
│   ├── config/
│   │   └── config.go                # Env var loading (RECALL_REGION, RECALL_API_KEY, etc.)
│   ├── database/
│   │   ├── database.go              # pgx pool setup, migration runner
│   │   └── migrations/
│   │       └── 001_initial.up.sql   # Users, verification_tokens, bots tables
│   ├── auth/
│   │   ├── auth.go                  # SHA-256 hashing, key generation, middleware
│   │   └── auth_test.go
│   ├── handler/
│   │   ├── register.go              # POST /api/auth/register
│   │   ├── register_test.go
│   │   ├── verify.go                # GET /api/auth/verify
│   │   ├── verify_test.go
│   │   ├── rotate_key.go            # POST /api/auth/rotate-key
│   │   ├── rotate_key_test.go
│   │   ├── create_bot.go            # POST /api/bots
│   │   ├── create_bot_test.go
│   │   ├── bot_status.go            # GET /api/bots/:botId/status
│   │   ├── bot_status_test.go
│   │   ├── bot_transcript.go        # GET /api/bots/:botId/transcript
│   │   ├── bot_transcript_test.go
│   │   ├── delete_bot.go            # DELETE /api/bots/:botId
│   │   ├── delete_bot_test.go
│   │   └── webhook.go               # POST /api/webhooks/recallai
│   │   └── webhook_test.go
│   ├── recall/
│   │   ├── client.go                # Recall AI HTTP client (create bot, delete bot, fetch transcript)
│   │   ├── client_test.go
│   │   ├── transcript.go            # Transcript fetching + readable conversion
│   │   └── transcript_test.go
│   ├── ratelimit/
│   │   ├── ratelimit.go             # In-memory token bucket middleware
│   │   └── ratelimit_test.go
│   ├── background/
│   │   ├── stale_bots.go            # Stale bot cleanup goroutine
│   │   ├── stale_bots_test.go
│   │   ├── unverified_users.go      # Unverified user cleanup goroutine
│   │   └── unverified_users_test.go
│   └── model/
│       └── model.go                 # Structs: User, Bot, VerificationToken, API request/response types
├── static/
│   └── verify.html                  # Static page that reads API key from URL fragment
├── go.mod
├── go.sum
├── Dockerfile
└── docker-compose.yml               # Backend + Postgres for local dev
```

---

## Chunk 1: Project Setup & Database

### Task 1: Initialize Go module and dependencies

**Files:**
- Create: `backend/go.mod`
- Create: `backend/cmd/server/main.go`
- Create: `backend/docker-compose.yml`

- [ ] **Step 1: Create Go module**

```bash
cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin
mkdir -p backend/cmd/server
cd backend
go mod init github.com/devinat1/obsidian-meeting-notes/backend
```

- [ ] **Step 2: Add dependencies**

```bash
cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/backend
go get github.com/go-chi/chi/v5
go get github.com/jackc/pgx/v5
go get github.com/golang-migrate/migrate/v4
go get github.com/golang-migrate/migrate/v4/database/postgres
go get github.com/golang-migrate/migrate/v4/source/file
```

- [ ] **Step 3: Create docker-compose.yml**

```yaml
# backend/docker-compose.yml
services:
  db:
    image: postgres:16
    environment:
      POSTGRES_USER: meeting_notes
      POSTGRES_PASSWORD: localdev
      POSTGRES_DB: meeting_notes
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data

volumes:
  pgdata:
```

- [ ] **Step 4: Create minimal main.go that starts and exits**

```go
// backend/cmd/server/main.go
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("obsidian-meeting-notes backend starting")
	os.Exit(0)
}
```

- [ ] **Step 5: Verify it compiles**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/backend && go build ./cmd/server`
Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add backend/
git commit -m "feat(backend): initialize Go module with chi, pgx, migrate deps"
```

---

### Task 2: Config loading from environment variables

**Files:**
- Create: `backend/internal/config/config.go`

- [ ] **Step 1: Write config.go**

```go
// backend/internal/config/config.go
package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port                              string
	DatabaseURL                       string
	RecallRegion                      string
	RecallAPIKey                      string
	RecallWorkspaceVerificationSecret string
	SMTPHost                          string
	SMTPPort                          string
	SMTPUser                          string
	SMTPPass                          string
	SMTPFrom                          string
	BaseURL                           string
}

func Load() (*Config, error) {
	c := &Config{
		Port:                              getEnvOrDefault("PORT", "8080"),
		DatabaseURL:                       os.Getenv("DATABASE_URL"),
		RecallRegion:                      os.Getenv("RECALL_REGION"),
		RecallAPIKey:                      os.Getenv("RECALL_API_KEY"),
		RecallWorkspaceVerificationSecret: os.Getenv("RECALL_WORKSPACE_VERIFICATION_SECRET"),
		SMTPHost:                          os.Getenv("SMTP_HOST"),
		SMTPPort:                          getEnvOrDefault("SMTP_PORT", "587"),
		SMTPUser:                          os.Getenv("SMTP_USER"),
		SMTPPass:                          os.Getenv("SMTP_PASS"),
		SMTPFrom:                          os.Getenv("SMTP_FROM"),
		BaseURL:                           os.Getenv("BASE_URL"),
	}

	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if c.RecallRegion == "" {
		return nil, fmt.Errorf("RECALL_REGION is required")
	}
	if c.RecallAPIKey == "" {
		return nil, fmt.Errorf("RECALL_API_KEY is required")
	}
	if c.RecallWorkspaceVerificationSecret == "" {
		return nil, fmt.Errorf("RECALL_WORKSPACE_VERIFICATION_SECRET is required")
	}
	if c.BaseURL == "" {
		return nil, fmt.Errorf("BASE_URL is required")
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

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/backend && go build ./...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add backend/internal/config/
git commit -m "feat(backend): add config loading from env vars"
```

---

### Task 3: Database setup with migrations

**Files:**
- Create: `backend/internal/database/database.go`
- Create: `backend/internal/database/migrations/001_initial.up.sql`
- Create: `backend/internal/database/migrations/001_initial.down.sql`

- [ ] **Step 1: Write the initial migration SQL**

```sql
-- backend/internal/database/migrations/001_initial.up.sql
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    api_key_hash TEXT UNIQUE,
    verified BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_api_key_hash ON users (api_key_hash) WHERE api_key_hash IS NOT NULL;

CREATE TABLE verification_tokens (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE bots (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    bot_id TEXT NOT NULL UNIQUE,
    meeting_url TEXT NOT NULL,
    bot_status TEXT NOT NULL DEFAULT 'scheduled',
    bot_webhook_updated_at TIMESTAMPTZ,
    recording_id TEXT,
    recording_status TEXT,
    recording_webhook_updated_at TIMESTAMPTZ,
    transcript_id TEXT,
    transcript_status TEXT,
    transcript_failure_sub_code TEXT,
    transcript_webhook_updated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_bots_user_id ON bots (user_id);
CREATE INDEX idx_bots_bot_id ON bots (bot_id);
```

- [ ] **Step 2: Write the down migration**

```sql
-- backend/internal/database/migrations/001_initial.down.sql
DROP TABLE IF EXISTS bots;
DROP TABLE IF EXISTS verification_tokens;
DROP TABLE IF EXISTS users;
```

- [ ] **Step 3: Write database.go**

```go
// backend/internal/database/database.go
package database

import (
	"context"
	"embed"
	"fmt"

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
		return nil, fmt.Errorf("connecting to database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return pool, nil
}

func RunMigrations(databaseURL string) error {
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("creating migration source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, databaseURL)
	if err != nil {
		return fmt.Errorf("creating migrator: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("running migrations: %w", err)
	}

	return nil
}
```

- [ ] **Step 4: Verify it compiles**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/backend && go build ./...`
Expected: no errors

- [ ] **Step 5: Start Postgres and test migration**

```bash
cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/backend
docker compose up -d
```

- [ ] **Step 6: Commit**

```bash
git add backend/internal/database/ backend/internal/model/
git commit -m "feat(backend): add database setup with initial migration"
```

---

### Task 4: Model types

**Files:**
- Create: `backend/internal/model/model.go`

- [ ] **Step 1: Write model.go**

```go
// backend/internal/model/model.go
package model

import "time"

type User struct {
	ID         int        `json:"id"`
	Email      string     `json:"email"`
	APIKeyHash *string    `json:"-"`
	Verified   bool       `json:"verified"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type VerificationToken struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	TokenHash string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type Bot struct {
	ID                        int        `json:"id"`
	UserID                    int        `json:"user_id"`
	BotID                     string     `json:"bot_id"`
	MeetingURL                string     `json:"meeting_url"`
	BotStatus                 string     `json:"bot_status"`
	BotWebhookUpdatedAt       *time.Time `json:"-"`
	RecordingID               *string    `json:"recording_id"`
	RecordingStatus           *string    `json:"recording_status"`
	RecordingWebhookUpdatedAt *time.Time `json:"-"`
	TranscriptID              *string    `json:"transcript_id"`
	TranscriptStatus          *string    `json:"transcript_status"`
	TranscriptFailureSubCode  *string    `json:"transcript_failure_sub_code"`
	TranscriptWebhookUpdatedAt *time.Time `json:"-"`
	CreatedAt                 time.Time  `json:"created_at"`
	UpdatedAt                 time.Time  `json:"updated_at"`
}

// API request/response types

type RegisterRequest struct {
	Email string `json:"email"`
}

type CreateBotRequest struct {
	MeetingURL string `json:"meeting_url"`
}

type CreateBotResponse struct {
	BotID string `json:"bot_id"`
}

type BotStatusResponse struct {
	BotID                    string  `json:"bot_id"`
	BotStatus                string  `json:"bot_status"`
	RecordingStatus          *string `json:"recording_status"`
	TranscriptStatus         *string `json:"transcript_status"`
	TranscriptFailureSubCode *string `json:"transcript_failure_sub_code"`
	UpdatedAt                string  `json:"updated_at"`
}

type TranscriptResponse struct {
	Status             TranscriptStatus `json:"status"`
	RawTranscript      any              `json:"raw_transcript"`
	ReadableTranscript any              `json:"readable_transcript"`
}

type TranscriptStatus struct {
	Code    string  `json:"code"`
	SubCode *string `json:"sub_code"`
	Message *string `json:"message"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/backend && go build ./...`

- [ ] **Step 3: Commit**

```bash
git add backend/internal/model/
git commit -m "feat(backend): add model types for users, bots, API requests/responses"
```

---

## Chunk 2: Auth System

### Task 5: Auth utilities (SHA-256 hashing, key generation, middleware)

**Files:**
- Create: `backend/internal/auth/auth.go`
- Create: `backend/internal/auth/auth_test.go`

- [ ] **Step 1: Write the failing test**

```go
// backend/internal/auth/auth_test.go
package auth_test

import (
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/backend/internal/auth"
)

func TestGenerateAPIKey(t *testing.T) {
	key1, err := auth.GenerateAPIKey()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(key1) == 0 {
		t.Fatal("expected non-empty key")
	}

	key2, err := auth.GenerateAPIKey()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key1 == key2 {
		t.Fatal("expected unique keys")
	}
}

func TestHashAPIKey(t *testing.T) {
	key := "test-api-key-12345"
	hash1 := auth.HashAPIKey(key)
	hash2 := auth.HashAPIKey(key)

	if hash1 != hash2 {
		t.Fatal("same input should produce same hash")
	}
	if hash1 == key {
		t.Fatal("hash should differ from input")
	}
}

func TestGenerateVerificationToken(t *testing.T) {
	token1, err := auth.GenerateVerificationToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	token2, err := auth.GenerateVerificationToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token1 == token2 {
		t.Fatal("expected unique tokens")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/backend && go test ./internal/auth/ -v`
Expected: FAIL (package doesn't exist yet)

- [ ] **Step 3: Write auth.go**

```go
// backend/internal/auth/auth.go
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type contextKey string

const userIDKey contextKey = "userID"

func GenerateAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generating random bytes: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

func GenerateVerificationToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generating random bytes: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

func HashAPIKey(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:])
}

func Middleware(db *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				http.Error(w, `{"error":"missing or invalid Authorization header"}`, http.StatusUnauthorized)
				return
			}

			apiKey := strings.TrimPrefix(header, "Bearer ")
			keyHash := HashAPIKey(apiKey)

			var userID int
			err := db.QueryRow(r.Context(),
				"SELECT id FROM users WHERE api_key_hash = $1 AND verified = true",
				keyHash,
			).Scan(&userID)
			if err != nil {
				http.Error(w, `{"error":"invalid API key"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserIDFromContext(ctx context.Context) (int, bool) {
	userID, ok := ctx.Value(userIDKey).(int)
	return userID, ok
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/backend && go test ./internal/auth/ -v`
Expected: PASS (3 tests)

- [ ] **Step 5: Commit**

```bash
git add backend/internal/auth/
git commit -m "feat(backend): add auth utilities with SHA-256 hashing and middleware"
```

---

### Task 6: Registration handler

**Files:**
- Create: `backend/internal/handler/register.go`
- Create: `backend/internal/handler/register_test.go`

- [ ] **Step 1: Write the failing test**

```go
// backend/internal/handler/register_test.go
package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/backend/internal/handler"
)

func TestRegisterHandler_ValidEmail(t *testing.T) {
	// Uses a mock DB and mock email sender
	h := handler.NewRegisterHandler(nil, nil, "http://localhost:8080")

	body, _ := json.Marshal(map[string]string{"email": "test@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	// With nil DB, we expect an internal error, but the handler should parse the request correctly
	// Full integration test requires a real DB
	if rec.Code == http.StatusBadRequest {
		t.Fatal("valid email should not return 400")
	}
}

func TestRegisterHandler_MissingEmail(t *testing.T) {
	h := handler.NewRegisterHandler(nil, nil, "http://localhost:8080")

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestRegisterHandler_InvalidEmail(t *testing.T) {
	h := handler.NewRegisterHandler(nil, nil, "http://localhost:8080")

	body, _ := json.Marshal(map[string]string{"email": "notanemail"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/backend && go test ./internal/handler/ -v -run TestRegister`
Expected: FAIL

- [ ] **Step 3: Write register.go**

```go
// backend/internal/handler/register.go
package handler

import (
	"encoding/json"
	"net/http"
	"net/mail"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/backend/internal/auth"
	"github.com/devinat1/obsidian-meeting-notes/backend/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type EmailSender interface {
	SendVerificationEmail(to, verifyURL string) error
}

type RegisterHandler struct {
	db      *pgxpool.Pool
	email   EmailSender
	baseURL string
}

func NewRegisterHandler(db *pgxpool.Pool, email EmailSender, baseURL string) *RegisterHandler {
	return &RegisterHandler{db: db, email: email, baseURL: baseURL}
}

func (h *RegisterHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req model.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{Error: "invalid request body"})
		return
	}

	if req.Email == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{Error: "email is required"})
		return
	}

	if _, err := mail.ParseAddress(req.Email); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{Error: "invalid email address"})
		return
	}

	token, err := auth.GenerateVerificationToken()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Error: "failed to generate token"})
		return
	}

	tokenHash := auth.HashAPIKey(token)
	expiresAt := time.Now().Add(24 * time.Hour)

	tx, err := h.db.Begin(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Error: "database error"})
		return
	}
	defer tx.Rollback(r.Context())

	var userID int
	err = tx.QueryRow(r.Context(),
		`INSERT INTO users (email) VALUES ($1)
		 ON CONFLICT (email) DO UPDATE SET updated_at = NOW()
		 RETURNING id`,
		req.Email,
	).Scan(&userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Error: "database error"})
		return
	}

	_, err = tx.Exec(r.Context(),
		`INSERT INTO verification_tokens (user_id, token_hash, expires_at)
		 VALUES ($1, $2, $3)`,
		userID, tokenHash, expiresAt,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Error: "database error"})
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Error: "database error"})
		return
	}

	verifyURL := h.baseURL + "/api/auth/verify?token=" + token
	if h.email != nil {
		if err := h.email.SendVerificationEmail(req.Email, verifyURL); err != nil {
			writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Error: "failed to send email"})
			return
		}
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, http.StatusCreated, map[string]string{"message": "verification email sent"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/backend && go test ./internal/handler/ -v -run TestRegister`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handler/register.go backend/internal/handler/register_test.go
git commit -m "feat(backend): add registration handler with email validation"
```

---

### Task 7: Verification handler

**Files:**
- Create: `backend/internal/handler/verify.go`
- Create: `backend/internal/handler/verify_test.go`
- Create: `backend/static/verify.html`

- [ ] **Step 1: Write verify.html (static page that reads key from URL fragment)**

```html
<!-- backend/static/verify.html -->
<!DOCTYPE html>
<html>
<head><title>API Key</title></head>
<body>
  <h1>Your API Key</h1>
  <p>Copy this key and store it securely. It will not be shown again.</p>
  <code id="key"></code>
  <script>
    const hash = window.location.hash;
    if (hash && hash.startsWith('#key=')) {
      document.getElementById('key').textContent = hash.substring(5);
    } else {
      document.getElementById('key').textContent = 'No key found. The link may have expired.';
    }
  </script>
</body>
</html>
```

- [ ] **Step 2: Write verify.go**

```go
// backend/internal/handler/verify.go
package handler

import (
	"net/http"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/backend/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

type VerifyHandler struct {
	db      *pgxpool.Pool
	baseURL string
}

func NewVerifyHandler(db *pgxpool.Pool, baseURL string) *VerifyHandler {
	return &VerifyHandler{db: db, baseURL: baseURL}
}

func (h *VerifyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "missing token", http.StatusBadRequest)
		return
	}

	tokenHash := auth.HashAPIKey(token)

	var userID int
	var expiresAt time.Time
	err := h.db.QueryRow(r.Context(),
		`SELECT user_id, expires_at FROM verification_tokens WHERE token_hash = $1`,
		tokenHash,
	).Scan(&userID, &expiresAt)
	if err != nil {
		http.Error(w, "invalid or expired token", http.StatusBadRequest)
		return
	}

	if time.Now().After(expiresAt) {
		http.Error(w, "token expired", http.StatusBadRequest)
		return
	}

	apiKey, err := auth.GenerateAPIKey()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	apiKeyHash := auth.HashAPIKey(apiKey)

	tx, err := h.db.Begin(r.Context())
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(r.Context())

	_, err = tx.Exec(r.Context(),
		`UPDATE users SET api_key_hash = $1, verified = true, updated_at = NOW() WHERE id = $2`,
		apiKeyHash, userID,
	)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	_, err = tx.Exec(r.Context(),
		`DELETE FROM verification_tokens WHERE user_id = $1`,
		userID,
	)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/static/verify.html#key="+apiKey, http.StatusFound)
}
```

- [ ] **Step 3: Verify it compiles**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/backend && go build ./...`
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add backend/internal/handler/verify.go backend/static/verify.html
git commit -m "feat(backend): add email verification handler with static key display page"
```

---

### Task 8: Key rotation handler

**Files:**
- Create: `backend/internal/handler/rotate_key.go`
- Create: `backend/internal/handler/rotate_key_test.go`

- [ ] **Step 1: Write rotate_key.go**

```go
// backend/internal/handler/rotate_key.go
package handler

import (
	"net/http"

	"github.com/devinat1/obsidian-meeting-notes/backend/internal/auth"
	"github.com/devinat1/obsidian-meeting-notes/backend/internal/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RotateKeyHandler struct {
	db *pgxpool.Pool
}

func NewRotateKeyHandler(db *pgxpool.Pool) *RotateKeyHandler {
	return &RotateKeyHandler{db: db}
}

func (h *RotateKeyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, model.ErrorResponse{Error: "unauthorized"})
		return
	}

	newKey, err := auth.GenerateAPIKey()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Error: "failed to generate key"})
		return
	}

	newKeyHash := auth.HashAPIKey(newKey)

	_, err = h.db.Exec(r.Context(),
		`UPDATE users SET api_key_hash = $1, updated_at = NOW() WHERE id = $2`,
		newKeyHash, userID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Error: "database error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"api_key": newKey})
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/backend && go build ./...`

- [ ] **Step 3: Commit**

```bash
git add backend/internal/handler/rotate_key.go
git commit -m "feat(backend): add API key rotation handler"
```

---

## Chunk 3: Rate Limiting & Recall Client

### Task 9: Rate limiting middleware

**Files:**
- Create: `backend/internal/ratelimit/ratelimit.go`
- Create: `backend/internal/ratelimit/ratelimit_test.go`

- [ ] **Step 1: Write the failing test**

```go
// backend/internal/ratelimit/ratelimit_test.go
package ratelimit_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/backend/internal/ratelimit"
)

func TestTokenBucket_AllowsUnderLimit(t *testing.T) {
	limiter := ratelimit.NewIPLimiter(10, 10) // 10 requests per interval
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, rec.Code)
		}
	}
}

func TestTokenBucket_BlocksOverLimit(t *testing.T) {
	limiter := ratelimit.NewIPLimiter(2, 2)
	handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/backend && go test ./internal/ratelimit/ -v`
Expected: FAIL

- [ ] **Step 3: Write ratelimit.go**

```go
// backend/internal/ratelimit/ratelimit.go
package ratelimit

import (
	"net"
	"net/http"
	"sync"
	"time"
)

type bucket struct {
	tokens    float64
	lastCheck time.Time
}

type IPLimiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     float64
	capacity float64
}

func NewIPLimiter(ratePerInterval float64, capacity float64) *IPLimiter {
	return &IPLimiter{
		buckets:  make(map[string]*bucket),
		rate:     ratePerInterval / 3600, // convert to per-second
		capacity: capacity,
	}
}

func (l *IPLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	b, exists := l.buckets[key]
	if !exists {
		l.buckets[key] = &bucket{tokens: l.capacity - 1, lastCheck: now}
		return true
	}

	elapsed := now.Sub(b.lastCheck).Seconds()
	b.tokens += elapsed * l.rate
	if b.tokens > l.capacity {
		b.tokens = l.capacity
	}
	b.lastCheck = now

	if b.tokens < 1 {
		return false
	}

	b.tokens--
	return true
}

func (l *IPLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		if ip == "" {
			ip = r.RemoteAddr
		}

		if !l.allow(ip) {
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/backend && go test ./internal/ratelimit/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/ratelimit/
git commit -m "feat(backend): add in-memory token bucket rate limiter"
```

---

### Task 10: Recall AI client

**Files:**
- Create: `backend/internal/recall/client.go`
- Create: `backend/internal/recall/client_test.go`

- [ ] **Step 1: Write the failing test (using httptest server as mock Recall API)**

```go
// backend/internal/recall/client_test.go
package recall_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/backend/internal/recall"
)

func TestCreateBot_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Token test-key" {
			t.Fatalf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}

		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["meeting_url"] != "https://zoom.us/j/123" {
			t.Fatalf("unexpected meeting_url: %v", body["meeting_url"])
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"id": "bot-abc-123"})
	}))
	defer server.Close()

	client := recall.NewClient(server.URL, "test-key")
	botID, err := client.CreateBot(t.Context(), "https://zoom.us/j/123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if botID != "bot-abc-123" {
		t.Fatalf("expected bot-abc-123, got %s", botID)
	}
}

func TestCreateBot_507Retry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusInsufficientStorage)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"id": "bot-retry-ok"})
	}))
	defer server.Close()

	client := recall.NewClient(server.URL, "test-key")
	client.RetryDelay = 0 // no delay in tests
	botID, err := client.CreateBot(t.Context(), "https://zoom.us/j/123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if botID != "bot-retry-ok" {
		t.Fatalf("expected bot-retry-ok, got %s", botID)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/backend && go test ./internal/recall/ -v`
Expected: FAIL

- [ ] **Step 3: Write client.go**

```go
// backend/internal/recall/client.go
package recall

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	RetryDelay time.Duration
	MaxRetries int
}

func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL:    baseURL,
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		RetryDelay: 30 * time.Second,
		MaxRetries: 10,
	}
}

type createBotRequest struct {
	MeetingURL      string          `json:"meeting_url"`
	RecordingConfig recordingConfig `json:"recording_config"`
}

type recordingConfig struct {
	Transcript transcriptConfig `json:"transcript"`
}

type transcriptConfig struct {
	Provider    providerConfig    `json:"provider"`
	Diarization diarizationConfig `json:"diarization"`
}

type providerConfig struct {
	RecallAIStreaming recallAIStreamingConfig `json:"recallai_streaming"`
}

type recallAIStreamingConfig struct {
	Mode         string `json:"mode"`
	LanguageCode string `json:"language_code"`
}

type diarizationConfig struct {
	UseSeparateStreamsWhenAvailable bool `json:"use_separate_streams_when_available"`
}

func (c *Client) CreateBot(ctx context.Context, meetingURL string) (string, error) {
	reqBody := createBotRequest{
		MeetingURL: meetingURL,
		RecordingConfig: recordingConfig{
			Transcript: transcriptConfig{
				Provider: providerConfig{
					RecallAIStreaming: recallAIStreamingConfig{
						Mode:         "prioritize_accuracy",
						LanguageCode: "auto",
					},
				},
				Diarization: diarizationConfig{
					UseSeparateStreamsWhenAvailable: true,
				},
			},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	for attempt := range c.MaxRetries {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/bot/", bytes.NewReader(bodyBytes))
		if err != nil {
			return "", fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("Authorization", "Token "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return "", fmt.Errorf("sending request: %w", err)
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("reading response: %w", err)
		}

		switch {
		case resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK:
			var result struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(respBody, &result); err != nil {
				return "", fmt.Errorf("parsing response: %w", err)
			}
			return result.ID, nil

		case resp.StatusCode == http.StatusInsufficientStorage:
			if attempt == c.MaxRetries-1 {
				return "", fmt.Errorf("bot capacity unavailable after %d retries", c.MaxRetries)
			}
			if c.RetryDelay > 0 {
				time.Sleep(c.RetryDelay)
			}
			continue

		case resp.StatusCode == http.StatusTooManyRequests:
			retryAfter := resp.Header.Get("Retry-After")
			delay := 5 * time.Second
			if seconds, err := strconv.Atoi(retryAfter); err == nil {
				jitter := time.Duration(rand.IntN(1000)) * time.Millisecond
				delay = time.Duration(seconds)*time.Second + jitter
			}
			time.Sleep(delay)
			continue

		default:
			return "", fmt.Errorf("recall API error (status %d): %s", resp.StatusCode, string(respBody))
		}
	}

	return "", fmt.Errorf("exceeded max retries")
}

func (c *Client) DeleteBot(ctx context.Context, botID string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+"/api/v1/bot/"+botID+"/", nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("recall API error (status %d): %s", resp.StatusCode, string(body))
	}

	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/backend && go test ./internal/recall/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/recall/
git commit -m "feat(backend): add Recall AI client with 507 retry and 429 backoff"
```

---

### Task 11: Transcript fetching and readable conversion

**Files:**
- Create: `backend/internal/recall/transcript.go`
- Create: `backend/internal/recall/transcript_test.go`

- [ ] **Step 1: Write the failing test**

```go
// backend/internal/recall/transcript_test.go
package recall_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/backend/internal/recall"
)

func TestConvertToReadableTranscript(t *testing.T) {
	raw := []recall.RawTranscriptEntry{
		{
			Words: []recall.Word{
				{Text: "Hello.", StartTimestamp: recall.Timestamp{Relative: 0.0}, EndTimestamp: recall.Timestamp{Relative: 0.5}},
				{Text: "How", StartTimestamp: recall.Timestamp{Relative: 0.6}, EndTimestamp: recall.Timestamp{Relative: 0.8}},
				{Text: "are", StartTimestamp: recall.Timestamp{Relative: 0.9}, EndTimestamp: recall.Timestamp{Relative: 1.0}},
				{Text: "you?", StartTimestamp: recall.Timestamp{Relative: 1.1}, EndTimestamp: recall.Timestamp{Relative: 1.3}},
			},
			Participant: recall.Participant{Name: "Alice"},
		},
	}

	readable := recall.ConvertToReadableTranscript(raw)
	if len(readable) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(readable))
	}
	if readable[0].Speaker != "Alice" {
		t.Fatalf("expected Alice, got %s", readable[0].Speaker)
	}
	if readable[0].Paragraph != "Hello. How are you?" {
		t.Fatalf("unexpected paragraph: %s", readable[0].Paragraph)
	}
}

func TestFetchTranscript_Success(t *testing.T) {
	rawTranscript := []recall.RawTranscriptEntry{
		{
			Words: []recall.Word{
				{Text: "Test.", StartTimestamp: recall.Timestamp{Relative: 0.0}, EndTimestamp: recall.Timestamp{Relative: 0.5}},
			},
			Participant: recall.Participant{Name: "Bob"},
		},
	}
	rawBytes, _ := json.Marshal(rawTranscript)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/transcript/t-123/", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"id": "t-123",
			"data": map[string]string{
				"download_url": "/download/transcript",
			},
		})
	})
	mux.HandleFunc("/download/transcript", func(w http.ResponseWriter, r *http.Request) {
		w.Write(rawBytes)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := recall.NewClient(server.URL, "test-key")
	result, err := client.FetchTranscript(t.Context(), "t-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.RawTranscript) != 1 {
		t.Fatalf("expected 1 raw entry, got %d", len(result.RawTranscript))
	}
	if len(result.ReadableTranscript) != 1 {
		t.Fatalf("expected 1 readable entry, got %d", len(result.ReadableTranscript))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/backend && go test ./internal/recall/ -v -run TestConvert`
Expected: FAIL

- [ ] **Step 3: Write transcript.go**

```go
// backend/internal/recall/transcript.go
package recall

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Timestamp struct {
	Relative float64 `json:"relative"`
}

type Word struct {
	Text           string    `json:"text"`
	StartTimestamp Timestamp `json:"start_timestamp"`
	EndTimestamp   Timestamp `json:"end_timestamp"`
}

type Participant struct {
	ID        int     `json:"id"`
	Name      string  `json:"name"`
	IsHost    bool    `json:"is_host"`
	Platform  string  `json:"platform"`
	ExtraData any     `json:"extra_data"`
	Email     *string `json:"email"`
}

type RawTranscriptEntry struct {
	Words        []Word      `json:"words"`
	LanguageCode string      `json:"language_code"`
	Participant  Participant `json:"participant"`
}

type ReadableTranscriptEntry struct {
	Speaker        string    `json:"speaker"`
	Paragraph      string    `json:"paragraph"`
	StartTimestamp Timestamp `json:"start_timestamp"`
	EndTimestamp   Timestamp `json:"end_timestamp"`
}

type TranscriptResult struct {
	RawTranscript      []RawTranscriptEntry      `json:"raw_transcript"`
	ReadableTranscript []ReadableTranscriptEntry  `json:"readable_transcript"`
}

func ConvertToReadableTranscript(raw []RawTranscriptEntry) []ReadableTranscriptEntry {
	readable := make([]ReadableTranscriptEntry, 0, len(raw))

	for _, entry := range raw {
		if len(entry.Words) == 0 {
			continue
		}

		words := make([]string, 0, len(entry.Words))
		for _, w := range entry.Words {
			words = append(words, w.Text)
		}

		readable = append(readable, ReadableTranscriptEntry{
			Speaker:        entry.Participant.Name,
			Paragraph:      strings.Join(words, " "),
			StartTimestamp: entry.Words[0].StartTimestamp,
			EndTimestamp:   entry.Words[len(entry.Words)-1].EndTimestamp,
		})
	}

	return readable
}

func (c *Client) FetchTranscript(ctx context.Context, transcriptID string) (*TranscriptResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/v1/transcript/"+transcriptID+"/", nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching transcript metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("recall API error (status %d): %s", resp.StatusCode, string(body))
	}

	var metadata struct {
		Data struct {
			DownloadURL string `json:"download_url"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("parsing transcript metadata: %w", err)
	}

	downloadURL := metadata.Data.DownloadURL
	if !strings.HasPrefix(downloadURL, "http") {
		downloadURL = c.baseURL + downloadURL
	}

	dlReq, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating download request: %w", err)
	}

	dlResp, err := c.httpClient.Do(dlReq)
	if err != nil {
		return nil, fmt.Errorf("downloading transcript: %w", err)
	}
	defer dlResp.Body.Close()

	var raw []RawTranscriptEntry
	if err := json.NewDecoder(dlResp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("parsing transcript data: %w", err)
	}

	return &TranscriptResult{
		RawTranscript:      raw,
		ReadableTranscript: ConvertToReadableTranscript(raw),
	}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/backend && go test ./internal/recall/ -v`
Expected: PASS (all 4 tests)

- [ ] **Step 5: Commit**

```bash
git add backend/internal/recall/transcript.go backend/internal/recall/transcript_test.go
git commit -m "feat(backend): add transcript fetching and readable conversion"
```

---

## Chunk 4: Bot Handlers & Webhooks

### Task 12: Create bot handler

**Files:**
- Create: `backend/internal/handler/create_bot.go`
- Create: `backend/internal/handler/create_bot_test.go`

- [ ] **Step 1: Write create_bot.go**

```go
// backend/internal/handler/create_bot.go
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/devinat1/obsidian-meeting-notes/backend/internal/auth"
	"github.com/devinat1/obsidian-meeting-notes/backend/internal/model"
	"github.com/devinat1/obsidian-meeting-notes/backend/internal/recall"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CreateBotHandler struct {
	db     *pgxpool.Pool
	recall *recall.Client
}

func NewCreateBotHandler(db *pgxpool.Pool, recall *recall.Client) *CreateBotHandler {
	return &CreateBotHandler{db: db, recall: recall}
}

func (h *CreateBotHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, model.ErrorResponse{Error: "unauthorized"})
		return
	}

	var req model.CreateBotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{Error: "invalid request body"})
		return
	}

	if req.MeetingURL == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{Error: "meeting_url is required"})
		return
	}

	botID, err := h.recall.CreateBot(r.Context(), req.MeetingURL)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, model.ErrorResponse{Error: err.Error()})
		return
	}

	_, err = h.db.Exec(r.Context(),
		`INSERT INTO bots (user_id, bot_id, meeting_url, bot_status) VALUES ($1, $2, $3, 'scheduled')`,
		userID, botID, req.MeetingURL,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Error: "failed to save bot record"})
		return
	}

	writeJSON(w, http.StatusCreated, model.CreateBotResponse{BotID: botID})
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/backend && go build ./...`

- [ ] **Step 3: Commit**

```bash
git add backend/internal/handler/create_bot.go
git commit -m "feat(backend): add create bot handler with Recall integration"
```

---

### Task 13: Bot status handler

**Files:**
- Create: `backend/internal/handler/bot_status.go`

- [ ] **Step 1: Write bot_status.go**

```go
// backend/internal/handler/bot_status.go
package handler

import (
	"net/http"

	"github.com/devinat1/obsidian-meeting-notes/backend/internal/auth"
	"github.com/devinat1/obsidian-meeting-notes/backend/internal/model"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BotStatusHandler struct {
	db *pgxpool.Pool
}

func NewBotStatusHandler(db *pgxpool.Pool) *BotStatusHandler {
	return &BotStatusHandler{db: db}
}

func (h *BotStatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, model.ErrorResponse{Error: "unauthorized"})
		return
	}

	botID := chi.URLParam(r, "botId")
	if botID == "" {
		writeJSON(w, http.StatusBadRequest, model.ErrorResponse{Error: "bot ID is required"})
		return
	}

	var resp model.BotStatusResponse
	var updatedAt string
	err := h.db.QueryRow(r.Context(),
		`SELECT bot_id, bot_status, recording_status, transcript_status, transcript_failure_sub_code, updated_at::text
		 FROM bots WHERE bot_id = $1 AND user_id = $2`,
		botID, userID,
	).Scan(&resp.BotID, &resp.BotStatus, &resp.RecordingStatus, &resp.TranscriptStatus, &resp.TranscriptFailureSubCode, &updatedAt)
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{Error: "bot not found"})
		return
	}

	resp.UpdatedAt = updatedAt
	writeJSON(w, http.StatusOK, resp)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/backend && go build ./...`

- [ ] **Step 3: Commit**

```bash
git add backend/internal/handler/bot_status.go
git commit -m "feat(backend): add bot status handler"
```

---

### Task 14: Bot transcript handler

**Files:**
- Create: `backend/internal/handler/bot_transcript.go`

- [ ] **Step 1: Write bot_transcript.go**

```go
// backend/internal/handler/bot_transcript.go
package handler

import (
	"net/http"

	"github.com/devinat1/obsidian-meeting-notes/backend/internal/auth"
	"github.com/devinat1/obsidian-meeting-notes/backend/internal/model"
	"github.com/devinat1/obsidian-meeting-notes/backend/internal/recall"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BotTranscriptHandler struct {
	db     *pgxpool.Pool
	recall *recall.Client
}

func NewBotTranscriptHandler(db *pgxpool.Pool, recall *recall.Client) *BotTranscriptHandler {
	return &BotTranscriptHandler{db: db, recall: recall}
}

func (h *BotTranscriptHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, model.ErrorResponse{Error: "unauthorized"})
		return
	}

	botID := chi.URLParam(r, "botId")

	var transcriptID *string
	var transcriptStatus *string
	var transcriptFailureSubCode *string
	err := h.db.QueryRow(r.Context(),
		`SELECT transcript_id, transcript_status, transcript_failure_sub_code
		 FROM bots WHERE bot_id = $1 AND user_id = $2`,
		botID, userID,
	).Scan(&transcriptID, &transcriptStatus, &transcriptFailureSubCode)
	if err != nil {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{Error: "bot not found"})
		return
	}

	if transcriptStatus == nil || *transcriptStatus == "" {
		writeJSON(w, http.StatusOK, model.TranscriptResponse{
			Status: model.TranscriptStatus{Code: "pending"},
		})
		return
	}

	if *transcriptStatus == "transcript.failed" {
		msg := "Transcript generation failed"
		writeJSON(w, http.StatusOK, model.TranscriptResponse{
			Status: model.TranscriptStatus{Code: "failed", SubCode: transcriptFailureSubCode, Message: &msg},
		})
		return
	}

	if *transcriptStatus == "transcript.done" && transcriptID != nil {
		result, err := h.recall.FetchTranscript(r.Context(), *transcriptID)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, model.ErrorResponse{Error: "failed to fetch transcript from Recall"})
			return
		}

		writeJSON(w, http.StatusOK, model.TranscriptResponse{
			Status:             model.TranscriptStatus{Code: "done"},
			RawTranscript:      result.RawTranscript,
			ReadableTranscript: result.ReadableTranscript,
		})
		return
	}

	writeJSON(w, http.StatusOK, model.TranscriptResponse{
		Status: model.TranscriptStatus{Code: "pending"},
	})
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/backend && go build ./...`

- [ ] **Step 3: Commit**

```bash
git add backend/internal/handler/bot_transcript.go
git commit -m "feat(backend): add bot transcript handler with Recall fetch"
```

---

### Task 15: Delete bot handler

**Files:**
- Create: `backend/internal/handler/delete_bot.go`

- [ ] **Step 1: Write delete_bot.go**

```go
// backend/internal/handler/delete_bot.go
package handler

import (
	"net/http"

	"github.com/devinat1/obsidian-meeting-notes/backend/internal/auth"
	"github.com/devinat1/obsidian-meeting-notes/backend/internal/model"
	"github.com/devinat1/obsidian-meeting-notes/backend/internal/recall"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DeleteBotHandler struct {
	db     *pgxpool.Pool
	recall *recall.Client
}

func NewDeleteBotHandler(db *pgxpool.Pool, recall *recall.Client) *DeleteBotHandler {
	return &DeleteBotHandler{db: db, recall: recall}
}

func (h *DeleteBotHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, model.ErrorResponse{Error: "unauthorized"})
		return
	}

	botID := chi.URLParam(r, "botId")

	var exists bool
	err := h.db.QueryRow(r.Context(),
		`SELECT EXISTS(SELECT 1 FROM bots WHERE bot_id = $1 AND user_id = $2)`,
		botID, userID,
	).Scan(&exists)
	if err != nil || !exists {
		writeJSON(w, http.StatusNotFound, model.ErrorResponse{Error: "bot not found"})
		return
	}

	if err := h.recall.DeleteBot(r.Context(), botID); err != nil {
		writeJSON(w, http.StatusBadGateway, model.ErrorResponse{Error: "failed to remove bot: " + err.Error()})
		return
	}

	_, err = h.db.Exec(r.Context(),
		`UPDATE bots SET bot_status = 'bot.done', updated_at = NOW() WHERE bot_id = $1`,
		botID,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{Error: "failed to update bot status"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/backend && go build ./...`

- [ ] **Step 3: Commit**

```bash
git add backend/internal/handler/delete_bot.go
git commit -m "feat(backend): add delete bot handler"
```

---

### Task 16: Webhook handler

**Files:**
- Create: `backend/internal/handler/webhook.go`
- Create: `backend/internal/handler/webhook_test.go`

- [ ] **Step 1: Write webhook.go**

```go
// backend/internal/handler/webhook.go
package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type WebhookHandler struct {
	db                *pgxpool.Pool
	verificationSecret string
}

func NewWebhookHandler(db *pgxpool.Pool, verificationSecret string) *WebhookHandler {
	return &WebhookHandler{db: db, verificationSecret: verificationSecret}
}

type webhookPayload struct {
	Event string `json:"event"`
	Data  struct {
		Data struct {
			Code      string  `json:"code"`
			SubCode   *string `json:"sub_code"`
			UpdatedAt string  `json:"updated_at"`
		} `json:"data"`
		Bot struct {
			ID string `json:"id"`
		} `json:"bot"`
		Recording *struct {
			ID string `json:"id"`
		} `json:"recording"`
		Transcript *struct {
			ID string `json:"id"`
		} `json:"transcript"`
	} `json:"data"`
}

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if !h.verifySignature(r, body) {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var payload webhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	webhookTime, err := time.Parse(time.RFC3339Nano, payload.Data.Data.UpdatedAt)
	if err != nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	botID := payload.Data.Bot.ID
	if botID == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	switch {
	case len(payload.Event) >= 4 && payload.Event[:4] == "bot.":
		h.handleBotEvent(r, botID, payload.Event, webhookTime)

	case payload.Event == "recording.done" || payload.Event == "recording.failed":
		recordingID := ""
		if payload.Data.Recording != nil {
			recordingID = payload.Data.Recording.ID
		}
		h.handleRecordingEvent(r, botID, recordingID, payload.Event, webhookTime)

	case payload.Event == "transcript.done" || payload.Event == "transcript.failed":
		transcriptID := ""
		if payload.Data.Transcript != nil {
			transcriptID = payload.Data.Transcript.ID
		}
		h.handleTranscriptEvent(r, botID, transcriptID, payload.Event, payload.Data.Data.SubCode, webhookTime)
	}

	w.WriteHeader(http.StatusOK)
}

func (h *WebhookHandler) handleBotEvent(r *http.Request, botID, event string, webhookTime time.Time) {
	h.db.Exec(r.Context(),
		`UPDATE bots SET bot_status = $1, bot_webhook_updated_at = $2, updated_at = NOW()
		 WHERE bot_id = $3 AND (bot_webhook_updated_at IS NULL OR bot_webhook_updated_at < $2)`,
		event, webhookTime, botID,
	)
}

func (h *WebhookHandler) handleRecordingEvent(r *http.Request, botID, recordingID, event string, webhookTime time.Time) {
	h.db.Exec(r.Context(),
		`UPDATE bots SET recording_id = $1, recording_status = $2, recording_webhook_updated_at = $3, updated_at = NOW()
		 WHERE bot_id = $4 AND (recording_webhook_updated_at IS NULL OR recording_webhook_updated_at < $3)`,
		recordingID, event, webhookTime, botID,
	)
}

func (h *WebhookHandler) handleTranscriptEvent(r *http.Request, botID, transcriptID, event string, subCode *string, webhookTime time.Time) {
	h.db.Exec(r.Context(),
		`UPDATE bots SET transcript_id = $1, transcript_status = $2, transcript_failure_sub_code = $3,
		 transcript_webhook_updated_at = $4, updated_at = NOW()
		 WHERE bot_id = $5 AND (transcript_webhook_updated_at IS NULL OR transcript_webhook_updated_at < $4)`,
		transcriptID, event, subCode, webhookTime, botID,
	)
}

func (h *WebhookHandler) verifySignature(r *http.Request, body []byte) bool {
	signature := r.Header.Get("webhook-signature")
	if signature == "" {
		signature = r.Header.Get("Webhook-Signature")
	}
	if signature == "" || h.verificationSecret == "" {
		return false
	}

	mac := hmac.New(sha256.New, []byte(h.verificationSecret))
	mac.Write(body)
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expectedSig))
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/backend && go build ./...`

- [ ] **Step 3: Commit**

```bash
git add backend/internal/handler/webhook.go
git commit -m "feat(backend): add webhook handler with HMAC verification and idempotent updates"
```

---

## Chunk 5: Background Jobs, Router & Main

### Task 17: Background goroutines (stale bots, unverified users)

**Files:**
- Create: `backend/internal/background/stale_bots.go`
- Create: `backend/internal/background/unverified_users.go`

- [ ] **Step 1: Write stale_bots.go**

```go
// backend/internal/background/stale_bots.go
package background

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func StartStaleBotCleanup(ctx context.Context, db *pgxpool.Pool) {
	ticker := time.NewTicker(15 * time.Minute)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				cleanStaleBots(ctx, db)
			}
		}
	}()
}

func cleanStaleBots(ctx context.Context, db *pgxpool.Pool) {
	terminalStatuses := []string{"bot.done", "bot.fatal", "recording.failed", "transcript.done", "transcript.failed"}

	result, err := db.Exec(ctx,
		`UPDATE bots SET bot_status = 'bot.fatal', transcript_failure_sub_code = 'stale_timeout', updated_at = NOW()
		 WHERE created_at < NOW() - INTERVAL '3 hours'
		 AND bot_status NOT IN ($1, $2, $3, $4, $5)
		 AND (transcript_status IS NULL OR transcript_status NOT IN ($4, $5))`,
		terminalStatuses[0], terminalStatuses[1], terminalStatuses[2], terminalStatuses[3], terminalStatuses[4],
	)
	if err != nil {
		log.Printf("stale bot cleanup error: %v", err)
		return
	}

	if result.RowsAffected() > 0 {
		log.Printf("cleaned up %d stale bots.", result.RowsAffected())
	}
}
```

- [ ] **Step 2: Write unverified_users.go**

```go
// backend/internal/background/unverified_users.go
package background

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func StartUnverifiedUserCleanup(ctx context.Context, db *pgxpool.Pool) {
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				cleanUnverifiedUsers(ctx, db)
			}
		}
	}()
}

func cleanUnverifiedUsers(ctx context.Context, db *pgxpool.Pool) {
	result, err := db.Exec(ctx,
		`DELETE FROM users WHERE verified = false AND created_at < NOW() - INTERVAL '7 days'`,
	)
	if err != nil {
		log.Printf("unverified user cleanup error: %v", err)
		return
	}

	if result.RowsAffected() > 0 {
		log.Printf("cleaned up %d unverified users.", result.RowsAffected())
	}
}
```

- [ ] **Step 3: Verify it compiles**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/backend && go build ./...`

- [ ] **Step 4: Commit**

```bash
git add backend/internal/background/
git commit -m "feat(backend): add background goroutines for stale bot and unverified user cleanup"
```

---

### Task 18: Wire up router and main.go

**Files:**
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Write the full main.go with router**

```go
// backend/cmd/server/main.go
package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/backend/internal/auth"
	"github.com/devinat1/obsidian-meeting-notes/backend/internal/background"
	"github.com/devinat1/obsidian-meeting-notes/backend/internal/config"
	"github.com/devinat1/obsidian-meeting-notes/backend/internal/database"
	"github.com/devinat1/obsidian-meeting-notes/backend/internal/handler"
	"github.com/devinat1/obsidian-meeting-notes/backend/internal/ratelimit"
	"github.com/devinat1/obsidian-meeting-notes/backend/internal/recall"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

//go:embed ../../static
var staticFS embed.FS

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if err := database.RunMigrations(cfg.DatabaseURL); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	recallClient := recall.NewClient(
		"https://"+cfg.RecallRegion+".recall.ai",
		cfg.RecallAPIKey,
	)

	registerLimiter := ratelimit.NewIPLimiter(5, 5)
	botCreateLimiter := ratelimit.NewIPLimiter(20, 20)

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	staticSub, _ := fs.Sub(staticFS, "static")
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))

	r.Route("/api", func(r chi.Router) {
		r.Route("/auth", func(r chi.Router) {
			r.With(registerLimiter.Middleware).Post("/register", handler.NewRegisterHandler(db, nil, cfg.BaseURL).ServeHTTP)
			r.Get("/verify", handler.NewVerifyHandler(db, cfg.BaseURL).ServeHTTP)

			r.Group(func(r chi.Router) {
				r.Use(auth.Middleware(db))
				r.Post("/rotate-key", handler.NewRotateKeyHandler(db).ServeHTTP)
			})
		})

		r.Route("/bots", func(r chi.Router) {
			r.Use(auth.Middleware(db))
			r.With(botCreateLimiter.Middleware).Post("/", handler.NewCreateBotHandler(db, recallClient).ServeHTTP)
			r.Get("/{botId}/status", handler.NewBotStatusHandler(db).ServeHTTP)
			r.Get("/{botId}/transcript", handler.NewBotTranscriptHandler(db, recallClient).ServeHTTP)
			r.Delete("/{botId}", handler.NewDeleteBotHandler(db, recallClient).ServeHTTP)
		})

		r.Post("/webhooks/recallai", handler.NewWebhookHandler(db, cfg.RecallWorkspaceVerificationSecret).ServeHTTP)
	})

	background.StartStaleBotCleanup(ctx, db)
	background.StartUnverifiedUserCleanup(ctx, db)

	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: r,
	}

	go func() {
		log.Printf("server starting on :%s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down server.")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	server.Shutdown(shutdownCtx)
	cancel()
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/backend && go build ./cmd/server`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add backend/cmd/server/main.go
git commit -m "feat(backend): wire up chi router with all handlers, auth, rate limiting, and background jobs"
```

---

### Task 19: Dockerfile

**Files:**
- Create: `backend/Dockerfile`

- [ ] **Step 1: Write Dockerfile**

```dockerfile
# backend/Dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o server ./cmd/server

FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/server .
COPY --from=builder /app/static ./static
EXPOSE 8080
CMD ["./server"]
```

- [ ] **Step 2: Update docker-compose.yml to include the backend service**

```yaml
# backend/docker-compose.yml
services:
  db:
    image: postgres:16
    environment:
      POSTGRES_USER: meeting_notes
      POSTGRES_PASSWORD: localdev
      POSTGRES_DB: meeting_notes
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data

  backend:
    build: .
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: postgres://meeting_notes:localdev@db:5432/meeting_notes?sslmode=disable
      RECALL_REGION: us-west-2
      RECALL_API_KEY: ${RECALL_API_KEY}
      RECALL_WORKSPACE_VERIFICATION_SECRET: ${RECALL_WORKSPACE_VERIFICATION_SECRET}
      BASE_URL: ${BASE_URL:-http://localhost:8080}
      PORT: "8080"
    depends_on:
      - db

volumes:
  pgdata:
```

- [ ] **Step 3: Commit**

```bash
git add backend/Dockerfile backend/docker-compose.yml
git commit -m "feat(backend): add Dockerfile and docker-compose for local dev"
```

---

### Task 20: Create .env.example

**Files:**
- Create: `backend/.env.example`

- [ ] **Step 1: Write .env.example**

```bash
# backend/.env.example
DATABASE_URL=postgres://meeting_notes:localdev@localhost:5432/meeting_notes?sslmode=disable
RECALL_REGION=us-west-2
RECALL_API_KEY=your-recall-api-key
RECALL_WORKSPACE_VERIFICATION_SECRET=your-webhook-secret
BASE_URL=http://localhost:8080
PORT=8080
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USER=your-smtp-user
SMTP_PASS=your-smtp-pass
SMTP_FROM=noreply@example.com
```

- [ ] **Step 2: Create .gitignore**

```
# backend/.gitignore
.env
server
```

- [ ] **Step 3: Commit**

```bash
git add backend/.env.example backend/.gitignore
git commit -m "feat(backend): add env example and gitignore"
```

---

This completes the Go backend plan. The Electron tray app plan will be written as a separate document.
