# Plan 1: Infrastructure, Auth & Gateway

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up the shared Go module, four Postgres Helm charts, and two microservices (Auth Service + Gateway Service) so a tray app can register, authenticate, and receive real-time SSE events from Kafka topics.

**Architecture:** Microservice architecture on homelab Kubernetes (`liteagent` namespace). Auth Service owns user registration, email verification, and API key management against PG-auth. Gateway Service is the single public entry point (`api.liteagent.net`), authenticates every request via Auth Service internal API, proxies to downstream services, and pushes Kafka events to connected tray apps via SSE. Kafka at `kafka.liteagent.svc.cluster.local:9092` for async event delivery.

**Tech Stack:** Go 1.22+, chi router, pgx (Postgres driver), golang-migrate, segmentio/kafka-go, crypto/sha256, net/smtp, Helm 3, Postgres 16, NGINX Ingress with cert-manager (letsencrypt-prod)

**Spec:** `docs/superpowers/specs/2026-04-05-obsidian-meeting-notes-design.md`

---

## File Structure

```
services/
├── go.mod                                    # Shared Go module root
├── go.sum
├── shared/
│   ├── kafkautil/
│   │   ├── producer.go                       # Kafka producer wrapper
│   │   ├── producer_test.go
│   │   ├── consumer.go                       # Kafka consumer wrapper
│   │   └── consumer_test.go
│   ├── httputil/
│   │   ├── respond.go                        # JSON response helpers
│   │   ├── respond_test.go
│   │   ├── parse.go                          # JSON request parsing helpers
│   │   └── parse_test.go
│   └── migrate/
│       └── migrate.go                        # Shared migration runner
├── auth/
│   ├── cmd/
│   │   └── main.go                           # Auth service entrypoint
│   ├── internal/
│   │   ├── config/
│   │   │   └── config.go                     # Env var loading
│   │   ├── database/
│   │   │   ├── database.go                   # pgx pool setup
│   │   │   └── migrations/
│   │   │       ├── 001_create_users.up.sql
│   │   │       └── 001_create_users.down.sql
│   │   ├── handler/
│   │   │   ├── register.go                   # POST /internal/auth/register
│   │   │   ├── register_test.go
│   │   │   ├── verify.go                     # GET /internal/auth/verify
│   │   │   ├── verify_test.go
│   │   │   ├── rotate_key.go                 # POST /internal/auth/rotate-key
│   │   │   ├── rotate_key_test.go
│   │   │   ├── validate.go                   # GET /internal/auth/validate
│   │   │   └── validate_test.go
│   │   ├── model/
│   │   │   └── model.go                      # User, VerificationToken, request/response types
│   │   ├── email/
│   │   │   ├── smtp.go                       # SMTP email sender
│   │   │   └── smtp_test.go
│   │   └── background/
│   │       ├── cleanup.go                    # Unverified user cleanup goroutine
│   │       └── cleanup_test.go
│   ├── Dockerfile
│   └── static/
│       └── verify.html                       # Static page showing API key via URL fragment
├── gateway/
│   ├── cmd/
│   │   └── main.go                           # Gateway service entrypoint
│   ├── internal/
│   │   ├── config/
│   │   │   └── config.go                     # Env var loading
│   │   ├── authclient/
│   │   │   ├── client.go                     # HTTP client for Auth Service /internal/auth/validate
│   │   │   └── client_test.go
│   │   ├── middleware/
│   │   │   ├── auth.go                       # Auth middleware (calls authclient)
│   │   │   └── auth_test.go
│   │   ├── proxy/
│   │   │   ├── proxy.go                      # Reverse proxy to internal services
│   │   │   └── proxy_test.go
│   │   ├── sse/
│   │   │   ├── hub.go                        # SSE connection hub (map user_id -> writer)
│   │   │   ├── hub_test.go
│   │   │   ├── handler.go                    # GET /api/events/stream handler
│   │   │   └── handler_test.go
│   │   ├── kafka/
│   │   │   ├── consumer.go                   # Kafka consumer → SSE push
│   │   │   └── consumer_test.go
│   │   └── ratelimit/
│   │       ├── ratelimit.go                  # In-memory token bucket
│   │       └── ratelimit_test.go
│   └── Dockerfile
└── docker-compose.yml                        # Local dev: 4 Postgres instances + Kafka

helm/
├── helm-pg-auth/
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
│       ├── deployment.yaml
│       ├── service.yaml
│       ├── pvc.yaml
│       └── secret.yaml
├── helm-pg-cal/
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
│       ├── deployment.yaml
│       ├── service.yaml
│       ├── pvc.yaml
│       └── secret.yaml
├── helm-pg-bot/
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
│       ├── deployment.yaml
│       ├── service.yaml
│       ├── pvc.yaml
│       └── secret.yaml
├── helm-pg-transcript/
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
│       ├── deployment.yaml
│       ├── service.yaml
│       ├── pvc.yaml
│       └── secret.yaml
├── helm-meeting-auth/
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
│       ├── deployment.yaml
│       ├── service.yaml
│       └── secret.yaml
└── helm-meeting-gateway/
    ├── Chart.yaml
    ├── values.yaml
    └── templates/
        ├── deployment.yaml
        ├── service.yaml
        ├── secret.yaml
        └── ingress.yaml
```

---

## Chunk 1: Shared Go Module & Utilities

### Task 1: Initialize shared Go module and dependencies

**Files:**
- Create: `services/go.mod`
- Create: `services/docker-compose.yml`

- [ ] **Step 1: Create Go module**

```bash
cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin
mkdir -p services
cd services
go mod init github.com/devinat1/obsidian-meeting-notes/services
```

- [ ] **Step 2: Add dependencies**

```bash
cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services
go get github.com/go-chi/chi/v5
go get github.com/jackc/pgx/v5
go get github.com/golang-migrate/migrate/v4
go get github.com/golang-migrate/migrate/v4/database/postgres
go get github.com/golang-migrate/migrate/v4/source/iofs
go get github.com/segmentio/kafka-go
```

- [ ] **Step 3: Create docker-compose.yml for local development**

```yaml
# services/docker-compose.yml
services:
  pg-auth:
    image: postgres:16
    environment:
      POSTGRES_USER: auth
      POSTGRES_PASSWORD: localdev
      POSTGRES_DB: auth
    ports:
      - "5501:5432"
    volumes:
      - pgauth:/var/lib/postgresql/data

  pg-cal:
    image: postgres:16
    environment:
      POSTGRES_USER: calendar
      POSTGRES_PASSWORD: localdev
      POSTGRES_DB: calendar
    ports:
      - "5502:5432"
    volumes:
      - pgcal:/var/lib/postgresql/data

  pg-bot:
    image: postgres:16
    environment:
      POSTGRES_USER: bot
      POSTGRES_PASSWORD: localdev
      POSTGRES_DB: bot
    ports:
      - "5503:5432"
    volumes:
      - pgbot:/var/lib/postgresql/data

  pg-transcript:
    image: postgres:16
    environment:
      POSTGRES_USER: transcript
      POSTGRES_PASSWORD: localdev
      POSTGRES_DB: transcript
    ports:
      - "5504:5432"
    volumes:
      - pgtranscript:/var/lib/postgresql/data

  kafka:
    image: apache/kafka:4.0.2
    environment:
      KAFKA_NODE_ID: "0"
      KAFKA_PROCESS_ROLES: "broker,controller"
      KAFKA_CONTROLLER_QUORUM_VOTERS: "0@localhost:9093"
      KAFKA_LISTENERS: "PLAINTEXT://:9092,CONTROLLER://:9093"
      KAFKA_LISTENER_SECURITY_PROTOCOL_MAP: "CONTROLLER:PLAINTEXT,PLAINTEXT:PLAINTEXT"
      KAFKA_CONTROLLER_LISTENER_NAMES: "CONTROLLER"
      KAFKA_INTER_BROKER_LISTENER_NAME: "PLAINTEXT"
      KAFKA_ADVERTISED_LISTENERS: "PLAINTEXT://localhost:9092"
      KAFKA_LOG_RETENTION_HOURS: "72"
      KAFKA_HEAP_OPTS: "-Xmx512m -Xms512m"
      KAFKA_LOG_DIRS: "/var/kafka-logs"
      CLUSTER_ID: "MkU3OEVBNTcwNTJENDM2Qk"
    ports:
      - "9092:9092"

volumes:
  pgauth:
  pgcal:
  pgbot:
  pgtranscript:
```

- [ ] **Step 4: Verify module initializes**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services && go mod tidy`
Expected: no errors, `go.mod` and `go.sum` present

- [ ] **Step 5: Commit**

```bash
git add services/go.mod services/go.sum services/docker-compose.yml
git commit -m "feat(services): initialize shared Go module with chi, pgx, kafka-go deps"
```

---

### Task 2: Shared HTTP utilities

**Files:**
- Create: `services/shared/httputil/respond.go`
- Create: `services/shared/httputil/respond_test.go`
- Create: `services/shared/httputil/parse.go`
- Create: `services/shared/httputil/parse_test.go`

- [ ] **Step 1: Write failing tests for respond.go**

```go
// services/shared/httputil/respond_test.go
package httputil_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/shared/httputil"
)

func TestRespondJSON(t *testing.T) {
	w := httptest.NewRecorder()
	payload := map[string]string{"message": "hello"}

	httputil.RespondJSON(w, http.StatusOK, payload)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %s", ct)
	}
	expected := `{"message":"hello"}` + "\n"
	if w.Body.String() != expected {
		t.Fatalf("expected body %q, got %q", expected, w.Body.String())
	}
}

func TestRespondError(t *testing.T) {
	w := httptest.NewRecorder()

	httputil.RespondError(w, http.StatusBadRequest, "bad input")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
	expected := `{"error":"bad input"}` + "\n"
	if w.Body.String() != expected {
		t.Fatalf("expected body %q, got %q", expected, w.Body.String())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services && go test ./shared/httputil/ -v`
Expected: FAIL (package doesn't compile)

- [ ] **Step 3: Write respond.go**

```go
// services/shared/httputil/respond.go
package httputil

import (
	"encoding/json"
	"net/http"
)

type errorBody struct {
	Error string `json:"error"`
}

func RespondJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(payload)
}

func RespondError(w http.ResponseWriter, statusCode int, message string) {
	RespondJSON(w, statusCode, errorBody{Error: message})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services && go test ./shared/httputil/ -v`
Expected: PASS

- [ ] **Step 5: Write failing tests for parse.go**

```go
// services/shared/httputil/parse_test.go
package httputil_test

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/shared/httputil"
)

func TestParseJSON(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	body := bytes.NewBufferString(`{"name":"alice"}`)
	r := httptest.NewRequest("POST", "/", body)
	r.Header.Set("Content-Type", "application/json")

	var p payload
	err := httputil.ParseJSON(r, &p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name != "alice" {
		t.Fatalf("expected name 'alice', got %q", p.Name)
	}
}

func TestParseJSONEmptyBody(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	r := httptest.NewRequest("POST", "/", nil)
	r.Header.Set("Content-Type", "application/json")

	var p payload
	err := httputil.ParseJSON(r, &p)
	if err == nil {
		t.Fatal("expected error for empty body")
	}
}
```

- [ ] **Step 6: Write parse.go**

```go
// services/shared/httputil/parse.go
package httputil

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const maxBodySize = 1 << 20 // 1 MB

func ParseJSON(r *http.Request, dest any) error {
	if r.Body == nil {
		return fmt.Errorf("request body is empty")
	}
	limited := io.LimitReader(r.Body, maxBodySize)
	defer r.Body.Close()

	decoder := json.NewDecoder(limited)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return fmt.Errorf("decoding JSON: %w", err)
	}
	return nil
}
```

- [ ] **Step 7: Run all httputil tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services && go test ./shared/httputil/ -v`
Expected: PASS (all 4 tests)

- [ ] **Step 8: Commit**

```bash
git add services/shared/httputil/
git commit -m "feat(shared): add HTTP response and JSON parsing utilities"
```

---

### Task 3: Shared Kafka producer and consumer wrappers

**Files:**
- Create: `services/shared/kafkautil/producer.go`
- Create: `services/shared/kafkautil/producer_test.go`
- Create: `services/shared/kafkautil/consumer.go`
- Create: `services/shared/kafkautil/consumer_test.go`

- [ ] **Step 1: Write producer.go**

```go
// services/shared/kafkautil/producer.go
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
```

- [ ] **Step 2: Write producer_test.go (unit test for struct construction)**

```go
// services/shared/kafkautil/producer_test.go
package kafkautil_test

import (
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/shared/kafkautil"
)

func TestNewProducer(t *testing.T) {
	p := kafkautil.NewProducer(kafkautil.NewProducerParams{
		Brokers: []string{"localhost:9092"},
		Topic:   "test.topic",
	})
	if p == nil {
		t.Fatal("expected non-nil producer")
	}
	if err := p.Close(); err != nil {
		t.Fatalf("unexpected error closing producer: %v", err)
	}
}
```

- [ ] **Step 3: Write consumer.go**

```go
// services/shared/kafkautil/consumer.go
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
```

- [ ] **Step 4: Write consumer_test.go (unit test for struct construction)**

```go
// services/shared/kafkautil/consumer_test.go
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
```

- [ ] **Step 5: Run all kafkautil tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services && go test ./shared/kafkautil/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add services/shared/kafkautil/
git commit -m "feat(shared): add Kafka producer and consumer wrappers"
```

---

### Task 4: Shared migration runner

**Files:**
- Create: `services/shared/migrate/migrate.go`

- [ ] **Step 1: Write migrate.go**

```go
// services/shared/migrate/migrate.go
package migrate

import (
	"fmt"
	"io/fs"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

func Run(params RunParams) error {
	source, err := iofs.New(params.MigrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("creating migration source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, params.DatabaseURL)
	if err != nil {
		return fmt.Errorf("creating migrator: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("running migrations: %w", err)
	}

	return nil
}

type RunParams struct {
	MigrationsFS fs.FS
	DatabaseURL  string
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services && go build ./shared/...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add services/shared/migrate/
git commit -m "feat(shared): add reusable database migration runner"
```

---

## Chunk 2: Postgres Helm Charts

### Task 5: Create Helm chart for PG-auth

**Files:**
- Create: `helm/helm-pg-auth/Chart.yaml`
- Create: `helm/helm-pg-auth/values.yaml`
- Create: `helm/helm-pg-auth/templates/deployment.yaml`
- Create: `helm/helm-pg-auth/templates/service.yaml`
- Create: `helm/helm-pg-auth/templates/pvc.yaml`
- Create: `helm/helm-pg-auth/templates/secret.yaml`

- [ ] **Step 1: Create Chart.yaml**

```yaml
# helm/helm-pg-auth/Chart.yaml
apiVersion: v2
name: pg-auth
description: Postgres instance for Auth Service
type: application
version: 0.1.0
appVersion: "16"
```

- [ ] **Step 2: Create values.yaml**

```yaml
# helm/helm-pg-auth/values.yaml
namespace: liteagent

postgres:
  image:
    repository: postgres
    tag: "16"
  port: 5432
  database: auth
  user: auth
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

- [ ] **Step 3: Create templates/secret.yaml**

```yaml
# helm/helm-pg-auth/templates/secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: pg-auth-secret
  namespace: {{ .Values.namespace }}
type: Opaque
stringData:
  POSTGRES_USER: {{ .Values.postgres.user }}
  POSTGRES_DB: {{ .Values.postgres.database }}
  POSTGRES_PASSWORD: {{ required "postgres.password is required" .Values.postgres.password }}
```

- [ ] **Step 4: Create templates/pvc.yaml**

```yaml
# helm/helm-pg-auth/templates/pvc.yaml
{{- if .Values.postgres.persistence.enabled }}
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: pg-auth-data
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

- [ ] **Step 5: Create templates/deployment.yaml**

```yaml
# helm/helm-pg-auth/templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: pg-auth
  namespace: {{ .Values.namespace }}
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: pg-auth
  template:
    metadata:
      labels:
        app: pg-auth
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
          envFrom:
            - secretRef:
                name: pg-auth-secret
          resources:
            {{- toYaml .Values.postgres.resources | nindent 12 }}
          volumeMounts:
            - name: pg-data
              mountPath: /var/lib/postgresql/data
          readinessProbe:
            tcpSocket:
              port: {{ .Values.postgres.port }}
            initialDelaySeconds: 10
            periodSeconds: 10
            failureThreshold: 5
          livenessProbe:
            tcpSocket:
              port: {{ .Values.postgres.port }}
            initialDelaySeconds: 30
            periodSeconds: 30
            failureThreshold: 3
      volumes:
        {{- if .Values.postgres.persistence.enabled }}
        - name: pg-data
          persistentVolumeClaim:
            claimName: pg-auth-data
        {{- else }}
        - name: pg-data
          emptyDir: {}
        {{- end }}
```

- [ ] **Step 6: Create templates/service.yaml**

```yaml
# helm/helm-pg-auth/templates/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: pg-auth
  namespace: {{ .Values.namespace }}
spec:
  selector:
    app: pg-auth
  ports:
    - port: {{ .Values.postgres.port }}
      targetPort: {{ .Values.postgres.port }}
      protocol: TCP
      name: postgres
```

- [ ] **Step 7: Validate Helm template renders**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin && helm template pg-auth helm/helm-pg-auth/ --set postgres.password=testpass`
Expected: renders all 4 YAML documents with no errors

- [ ] **Step 8: Commit**

```bash
git add helm/helm-pg-auth/
git commit -m "feat(helm): add Postgres Helm chart for Auth Service (PG-auth)"
```

---

### Task 6: Create Helm charts for PG-cal, PG-bot, PG-transcript

**Files:**
- Create: `helm/helm-pg-cal/` (all chart files)
- Create: `helm/helm-pg-bot/` (all chart files)
- Create: `helm/helm-pg-transcript/` (all chart files)

These three charts are structurally identical to PG-auth, differing only in names and labels. Each step below creates one chart by copying the PG-auth pattern and replacing identifiers.

- [ ] **Step 1: Create helm-pg-cal Chart.yaml**

```yaml
# helm/helm-pg-cal/Chart.yaml
apiVersion: v2
name: pg-cal
description: Postgres instance for Calendar Service
type: application
version: 0.1.0
appVersion: "16"
```

- [ ] **Step 2: Create helm-pg-cal values.yaml**

```yaml
# helm/helm-pg-cal/values.yaml
namespace: liteagent

postgres:
  image:
    repository: postgres
    tag: "16"
  port: 5432
  database: calendar
  user: calendar
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

- [ ] **Step 3: Create helm-pg-cal templates (secret, pvc, deployment, service)**

```yaml
# helm/helm-pg-cal/templates/secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: pg-cal-secret
  namespace: {{ .Values.namespace }}
type: Opaque
stringData:
  POSTGRES_USER: {{ .Values.postgres.user }}
  POSTGRES_DB: {{ .Values.postgres.database }}
  POSTGRES_PASSWORD: {{ required "postgres.password is required" .Values.postgres.password }}
```

```yaml
# helm/helm-pg-cal/templates/pvc.yaml
{{- if .Values.postgres.persistence.enabled }}
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: pg-cal-data
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

```yaml
# helm/helm-pg-cal/templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: pg-cal
  namespace: {{ .Values.namespace }}
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: pg-cal
  template:
    metadata:
      labels:
        app: pg-cal
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
          envFrom:
            - secretRef:
                name: pg-cal-secret
          resources:
            {{- toYaml .Values.postgres.resources | nindent 12 }}
          volumeMounts:
            - name: pg-data
              mountPath: /var/lib/postgresql/data
          readinessProbe:
            tcpSocket:
              port: {{ .Values.postgres.port }}
            initialDelaySeconds: 10
            periodSeconds: 10
            failureThreshold: 5
          livenessProbe:
            tcpSocket:
              port: {{ .Values.postgres.port }}
            initialDelaySeconds: 30
            periodSeconds: 30
            failureThreshold: 3
      volumes:
        {{- if .Values.postgres.persistence.enabled }}
        - name: pg-data
          persistentVolumeClaim:
            claimName: pg-cal-data
        {{- else }}
        - name: pg-data
          emptyDir: {}
        {{- end }}
```

```yaml
# helm/helm-pg-cal/templates/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: pg-cal
  namespace: {{ .Values.namespace }}
spec:
  selector:
    app: pg-cal
  ports:
    - port: {{ .Values.postgres.port }}
      targetPort: {{ .Values.postgres.port }}
      protocol: TCP
      name: postgres
```

- [ ] **Step 4: Create helm-pg-bot Chart.yaml**

```yaml
# helm/helm-pg-bot/Chart.yaml
apiVersion: v2
name: pg-bot
description: Postgres instance for Bot Service
type: application
version: 0.1.0
appVersion: "16"
```

- [ ] **Step 5: Create helm-pg-bot values.yaml**

```yaml
# helm/helm-pg-bot/values.yaml
namespace: liteagent

postgres:
  image:
    repository: postgres
    tag: "16"
  port: 5432
  database: bot
  user: bot
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

- [ ] **Step 6: Create helm-pg-bot templates (secret, pvc, deployment, service)**

```yaml
# helm/helm-pg-bot/templates/secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: pg-bot-secret
  namespace: {{ .Values.namespace }}
type: Opaque
stringData:
  POSTGRES_USER: {{ .Values.postgres.user }}
  POSTGRES_DB: {{ .Values.postgres.database }}
  POSTGRES_PASSWORD: {{ required "postgres.password is required" .Values.postgres.password }}
```

```yaml
# helm/helm-pg-bot/templates/pvc.yaml
{{- if .Values.postgres.persistence.enabled }}
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: pg-bot-data
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

```yaml
# helm/helm-pg-bot/templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: pg-bot
  namespace: {{ .Values.namespace }}
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: pg-bot
  template:
    metadata:
      labels:
        app: pg-bot
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
          envFrom:
            - secretRef:
                name: pg-bot-secret
          resources:
            {{- toYaml .Values.postgres.resources | nindent 12 }}
          volumeMounts:
            - name: pg-data
              mountPath: /var/lib/postgresql/data
          readinessProbe:
            tcpSocket:
              port: {{ .Values.postgres.port }}
            initialDelaySeconds: 10
            periodSeconds: 10
            failureThreshold: 5
          livenessProbe:
            tcpSocket:
              port: {{ .Values.postgres.port }}
            initialDelaySeconds: 30
            periodSeconds: 30
            failureThreshold: 3
      volumes:
        {{- if .Values.postgres.persistence.enabled }}
        - name: pg-data
          persistentVolumeClaim:
            claimName: pg-bot-data
        {{- else }}
        - name: pg-data
          emptyDir: {}
        {{- end }}
```

```yaml
# helm/helm-pg-bot/templates/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: pg-bot
  namespace: {{ .Values.namespace }}
spec:
  selector:
    app: pg-bot
  ports:
    - port: {{ .Values.postgres.port }}
      targetPort: {{ .Values.postgres.port }}
      protocol: TCP
      name: postgres
```

- [ ] **Step 7: Create helm-pg-transcript Chart.yaml**

```yaml
# helm/helm-pg-transcript/Chart.yaml
apiVersion: v2
name: pg-transcript
description: Postgres instance for Transcript Service
type: application
version: 0.1.0
appVersion: "16"
```

- [ ] **Step 8: Create helm-pg-transcript values.yaml**

```yaml
# helm/helm-pg-transcript/values.yaml
namespace: liteagent

postgres:
  image:
    repository: postgres
    tag: "16"
  port: 5432
  database: transcript
  user: transcript
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

- [ ] **Step 9: Create helm-pg-transcript templates (secret, pvc, deployment, service)**

```yaml
# helm/helm-pg-transcript/templates/secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: pg-transcript-secret
  namespace: {{ .Values.namespace }}
type: Opaque
stringData:
  POSTGRES_USER: {{ .Values.postgres.user }}
  POSTGRES_DB: {{ .Values.postgres.database }}
  POSTGRES_PASSWORD: {{ required "postgres.password is required" .Values.postgres.password }}
```

```yaml
# helm/helm-pg-transcript/templates/pvc.yaml
{{- if .Values.postgres.persistence.enabled }}
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: pg-transcript-data
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

```yaml
# helm/helm-pg-transcript/templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: pg-transcript
  namespace: {{ .Values.namespace }}
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: pg-transcript
  template:
    metadata:
      labels:
        app: pg-transcript
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
          envFrom:
            - secretRef:
                name: pg-transcript-secret
          resources:
            {{- toYaml .Values.postgres.resources | nindent 12 }}
          volumeMounts:
            - name: pg-data
              mountPath: /var/lib/postgresql/data
          readinessProbe:
            tcpSocket:
              port: {{ .Values.postgres.port }}
            initialDelaySeconds: 10
            periodSeconds: 10
            failureThreshold: 5
          livenessProbe:
            tcpSocket:
              port: {{ .Values.postgres.port }}
            initialDelaySeconds: 30
            periodSeconds: 30
            failureThreshold: 3
      volumes:
        {{- if .Values.postgres.persistence.enabled }}
        - name: pg-data
          persistentVolumeClaim:
            claimName: pg-transcript-data
        {{- else }}
        - name: pg-data
          emptyDir: {}
        {{- end }}
```

```yaml
# helm/helm-pg-transcript/templates/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: pg-transcript
  namespace: {{ .Values.namespace }}
spec:
  selector:
    app: pg-transcript
  ports:
    - port: {{ .Values.postgres.port }}
      targetPort: {{ .Values.postgres.port }}
      protocol: TCP
      name: postgres
```

- [ ] **Step 10: Validate all three charts render**

```bash
cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin
helm template pg-cal helm/helm-pg-cal/ --set postgres.password=testpass
helm template pg-bot helm/helm-pg-bot/ --set postgres.password=testpass
helm template pg-transcript helm/helm-pg-transcript/ --set postgres.password=testpass
```

Expected: all render without errors

- [ ] **Step 11: Commit**

```bash
git add helm/helm-pg-cal/ helm/helm-pg-bot/ helm/helm-pg-transcript/
git commit -m "feat(helm): add Postgres Helm charts for Calendar, Bot, and Transcript services"
```

---

## Chunk 3: Auth Service

### Task 7: Auth Service config and database

**Files:**
- Create: `services/auth/internal/config/config.go`
- Create: `services/auth/internal/database/database.go`
- Create: `services/auth/internal/database/migrations/001_create_users.up.sql`
- Create: `services/auth/internal/database/migrations/001_create_users.down.sql`

- [ ] **Step 1: Write config.go**

```go
// services/auth/internal/config/config.go
package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port        string
	DatabaseURL string
	SMTPHost    string
	SMTPPort    string
	SMTPUser    string
	SMTPPass    string
	SMTPFrom    string
	BaseURL     string
}

func Load() (*Config, error) {
	c := &Config{
		Port:        getEnvOrDefault("PORT", "8081"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		SMTPHost:    os.Getenv("SMTP_HOST"),
		SMTPPort:    getEnvOrDefault("SMTP_PORT", "587"),
		SMTPUser:    os.Getenv("SMTP_USER"),
		SMTPPass:    os.Getenv("SMTP_PASS"),
		SMTPFrom:    os.Getenv("SMTP_FROM"),
		BaseURL:     os.Getenv("BASE_URL"),
	}

	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if c.SMTPHost == "" {
		return nil, fmt.Errorf("SMTP_HOST is required")
	}
	if c.SMTPFrom == "" {
		return nil, fmt.Errorf("SMTP_FROM is required")
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

- [ ] **Step 2: Write initial migration**

```sql
-- services/auth/internal/database/migrations/001_create_users.up.sql
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
```

```sql
-- services/auth/internal/database/migrations/001_create_users.down.sql
DROP TABLE IF EXISTS verification_tokens;
DROP TABLE IF EXISTS users;
```

- [ ] **Step 3: Write database.go**

```go
// services/auth/internal/database/database.go
package database

import (
	"context"
	"embed"
	"fmt"

	"github.com/devinat1/obsidian-meeting-notes/services/shared/migrate"
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
	return migrate.Run(migrate.RunParams{
		MigrationsFS: migrationsFS,
		DatabaseURL:  databaseURL,
	})
}
```

- [ ] **Step 4: Verify it compiles**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services && go build ./auth/...`
Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add services/auth/internal/config/ services/auth/internal/database/
git commit -m "feat(auth): add config loading and database setup with user migration"
```

---

### Task 8: Auth Service models and email sender

**Files:**
- Create: `services/auth/internal/model/model.go`
- Create: `services/auth/internal/email/smtp.go`
- Create: `services/auth/internal/email/smtp_test.go`

- [ ] **Step 1: Write model.go**

```go
// services/auth/internal/model/model.go
package model

import "time"

type User struct {
	ID         int       `json:"id"`
	Email      string    `json:"email"`
	APIKeyHash *string   `json:"-"`
	Verified   bool      `json:"verified"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type VerificationToken struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	TokenHash string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type RegisterRequest struct {
	Email string `json:"email"`
}

type RegisterResponse struct {
	Message string `json:"message"`
}

type ValidateResponse struct {
	UserID int `json:"user_id"`
}

type RotateKeyResponse struct {
	APIKey string `json:"api_key"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
```

- [ ] **Step 2: Write smtp.go**

```go
// services/auth/internal/email/smtp.go
package email

import (
	"fmt"
	"net/smtp"
)

type Sender struct {
	host string
	port string
	user string
	pass string
	from string
}

type NewSenderParams struct {
	Host string
	Port string
	User string
	Pass string
	From string
}

func NewSender(params NewSenderParams) *Sender {
	return &Sender{
		host: params.Host,
		port: params.Port,
		user: params.User,
		pass: params.Pass,
		from: params.From,
	}
}

func (s *Sender) SendVerificationEmail(params SendVerificationEmailParams) error {
	subject := "Verify your email - Obsidian Meeting Notes"
	body := fmt.Sprintf(
		"Verify your email by clicking the link below:\n\n%s/api/auth/verify?token=%s\n\nThis link expires in 24 hours.",
		params.BaseURL,
		params.Token,
	)

	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		s.from,
		params.To,
		subject,
		body,
	)

	auth := smtp.PlainAuth("", s.user, s.pass, s.host)
	addr := fmt.Sprintf("%s:%s", s.host, s.port)

	if err := smtp.SendMail(addr, auth, s.from, []string{params.To}, []byte(msg)); err != nil {
		return fmt.Errorf("sending verification email to %s: %w", params.To, err)
	}

	return nil
}

type SendVerificationEmailParams struct {
	To      string
	Token   string
	BaseURL string
}
```

- [ ] **Step 3: Write smtp_test.go (unit test for message formatting)**

```go
// services/auth/internal/email/smtp_test.go
package email_test

import (
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/auth/internal/email"
)

func TestNewSender(t *testing.T) {
	s := email.NewSender(email.NewSenderParams{
		Host: "smtp.example.com",
		Port: "587",
		User: "user",
		Pass: "pass",
		From: "noreply@example.com",
	})
	if s == nil {
		t.Fatal("expected non-nil sender")
	}
}
```

- [ ] **Step 4: Run test**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services && go test ./auth/internal/email/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add services/auth/internal/model/ services/auth/internal/email/
git commit -m "feat(auth): add model types and SMTP email sender"
```

---

### Task 9: Auth Service handlers - register and verify

**Files:**
- Create: `services/auth/internal/handler/register.go`
- Create: `services/auth/internal/handler/register_test.go`
- Create: `services/auth/internal/handler/verify.go`
- Create: `services/auth/internal/handler/verify_test.go`
- Create: `services/auth/static/verify.html`

- [ ] **Step 1: Write register_test.go (failing test)**

```go
// services/auth/internal/handler/register_test.go
package handler_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/auth/internal/handler"
	"github.com/devinat1/obsidian-meeting-notes/services/auth/internal/model"
)

func TestRegisterHandler_MissingEmail(t *testing.T) {
	h := handler.NewRegisterHandler(handler.RegisterHandlerParams{
		DB:          nil,
		EmailSender: nil,
		BaseURL:     "https://api.liteagent.net",
	})

	body, _ := json.Marshal(model.RegisterRequest{Email: ""})
	req := httptest.NewRequest(http.MethodPost, "/internal/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestRegisterHandler_InvalidEmail(t *testing.T) {
	h := handler.NewRegisterHandler(handler.RegisterHandlerParams{
		DB:          nil,
		EmailSender: nil,
		BaseURL:     "https://api.liteagent.net",
	})

	body, _ := json.Marshal(model.RegisterRequest{Email: "not-an-email"})
	req := httptest.NewRequest(http.MethodPost, "/internal/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services && go test ./auth/internal/handler/ -v -run TestRegister`
Expected: FAIL (package doesn't compile)

- [ ] **Step 3: Write register.go**

```go
// services/auth/internal/handler/register.go
package handler

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/services/auth/internal/email"
	"github.com/devinat1/obsidian-meeting-notes/services/shared/httputil"
	"github.com/jackc/pgx/v5/pgxpool"
)

var emailRegexp = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

type RegisterHandler struct {
	db          *pgxpool.Pool
	emailSender *email.Sender
	baseURL     string
}

type RegisterHandlerParams struct {
	DB          *pgxpool.Pool
	EmailSender *email.Sender
	BaseURL     string
}

func NewRegisterHandler(params RegisterHandlerParams) *RegisterHandler {
	return &RegisterHandler{
		db:          params.DB,
		emailSender: params.EmailSender,
		baseURL:     params.BaseURL,
	}
}

func (h *RegisterHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	type request struct {
		Email string `json:"email"`
	}
	var req request
	if err := httputil.ParseJSON(r, &req); err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid request body.")
		return
	}

	if req.Email == "" {
		httputil.RespondError(w, http.StatusBadRequest, "Email is required.")
		return
	}

	if !emailRegexp.MatchString(req.Email) {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid email format.")
		return
	}

	ctx := r.Context()

	var existingUserID int
	var existingVerified bool
	err := h.db.QueryRow(ctx,
		"SELECT id, verified FROM users WHERE email = $1",
		req.Email,
	).Scan(&existingUserID, &existingVerified)

	if err == nil && existingVerified {
		httputil.RespondError(w, http.StatusConflict, "Email already registered and verified.")
		return
	}

	var userID int
	if err == nil {
		userID = existingUserID
		h.db.Exec(ctx, "DELETE FROM verification_tokens WHERE user_id = $1", userID)
	} else {
		err = h.db.QueryRow(ctx,
			"INSERT INTO users (email) VALUES ($1) ON CONFLICT (email) DO UPDATE SET updated_at = NOW() RETURNING id",
			req.Email,
		).Scan(&userID)
		if err != nil {
			log.Printf("Error inserting user: %v", err)
			httputil.RespondError(w, http.StatusInternalServerError, "Failed to create user.")
			return
		}
	}

	token, err := generateToken()
	if err != nil {
		log.Printf("Error generating verification token: %v", err)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to generate verification token.")
		return
	}

	tokenHash := hashString(token)
	expiresAt := time.Now().Add(24 * time.Hour)

	_, err = h.db.Exec(ctx,
		"INSERT INTO verification_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)",
		userID, tokenHash, expiresAt,
	)
	if err != nil {
		log.Printf("Error inserting verification token: %v", err)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to store verification token.")
		return
	}

	go func() {
		if err := h.emailSender.SendVerificationEmail(email.SendVerificationEmailParams{
			To:      req.Email,
			Token:   token,
			BaseURL: h.baseURL,
		}); err != nil {
			log.Printf("Error sending verification email to %s: %v", req.Email, err)
		}
	}()

	httputil.RespondJSON(w, http.StatusCreated, map[string]string{
		"message": "Verification email sent. Check your inbox.",
	})
}

func generateToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generating random bytes: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

func generateAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generating random bytes: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

func hashString(s string) string {
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])
}

func contextWithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, 5*time.Second)
}
```

- [ ] **Step 4: Run register tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services && go test ./auth/internal/handler/ -v -run TestRegister`
Expected: PASS

- [ ] **Step 5: Write verify.go**

```go
// services/auth/internal/handler/verify.go
package handler

import (
	"log"
	"net/http"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/services/shared/httputil"
	"github.com/jackc/pgx/v5/pgxpool"
)

type VerifyHandler struct {
	db *pgxpool.Pool
}

type VerifyHandlerParams struct {
	DB *pgxpool.Pool
}

func NewVerifyHandler(params VerifyHandlerParams) *VerifyHandler {
	return &VerifyHandler{db: params.DB}
}

func (h *VerifyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		httputil.RespondError(w, http.StatusBadRequest, "Token is required.")
		return
	}

	tokenHash := hashString(token)
	ctx := r.Context()

	var userID int
	var expiresAt time.Time
	err := h.db.QueryRow(ctx,
		"SELECT user_id, expires_at FROM verification_tokens WHERE token_hash = $1",
		tokenHash,
	).Scan(&userID, &expiresAt)

	if err != nil {
		httputil.RespondError(w, http.StatusBadRequest, "Invalid or expired token.")
		return
	}

	if time.Now().After(expiresAt) {
		h.db.Exec(ctx, "DELETE FROM verification_tokens WHERE token_hash = $1", tokenHash)
		httputil.RespondError(w, http.StatusBadRequest, "Token has expired. Please register again.")
		return
	}

	apiKey, err := generateAPIKey()
	if err != nil {
		log.Printf("Error generating API key: %v", err)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to generate API key.")
		return
	}

	apiKeyHash := hashString(apiKey)

	_, err = h.db.Exec(ctx,
		"UPDATE users SET api_key_hash = $1, verified = TRUE, updated_at = NOW() WHERE id = $2",
		apiKeyHash, userID,
	)
	if err != nil {
		log.Printf("Error updating user verification: %v", err)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to verify user.")
		return
	}

	h.db.Exec(ctx, "DELETE FROM verification_tokens WHERE user_id = $1", userID)

	http.Redirect(w, r, "/static/verify.html#key="+apiKey, http.StatusFound)
}
```

- [ ] **Step 6: Write verify_test.go**

```go
// services/auth/internal/handler/verify_test.go
package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/auth/internal/handler"
)

func TestVerifyHandler_MissingToken(t *testing.T) {
	h := handler.NewVerifyHandler(handler.VerifyHandlerParams{
		DB: nil,
	})

	req := httptest.NewRequest(http.MethodGet, "/internal/auth/verify", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
```

- [ ] **Step 7: Write verify.html**

```html
<!-- services/auth/static/verify.html -->
<!DOCTYPE html>
<html>
<head>
    <title>Email Verified - Obsidian Meeting Notes</title>
    <style>
        body { font-family: -apple-system, system-ui, sans-serif; max-width: 480px; margin: 80px auto; padding: 0 20px; text-align: center; }
        .key-box { background: #f5f5f5; border: 1px solid #ddd; border-radius: 8px; padding: 16px; margin: 24px 0; word-break: break-all; font-family: monospace; font-size: 14px; }
        .copy-btn { background: #333; color: #fff; border: none; padding: 10px 24px; border-radius: 6px; cursor: pointer; font-size: 14px; }
        .copy-btn:hover { background: #555; }
        .warning { color: #c00; font-size: 13px; margin-top: 16px; }
    </style>
</head>
<body>
    <div>Email Verified</div>
    <div>Your API key is below. Copy it and paste it into the tray app settings.</div>
    <div class="key-box" id="key"></div>
    <button class="copy-btn" onclick="navigator.clipboard.writeText(document.getElementById('key').textContent)">Copy to Clipboard</button>
    <div class="warning">This key will not be shown again. Store it securely.</div>
    <script>
        const hash = window.location.hash;
        if (hash.startsWith('#key=')) {
            document.getElementById('key').textContent = hash.substring(5);
            history.replaceState(null, '', window.location.pathname);
        } else {
            document.getElementById('key').textContent = 'No key found. The link may have already been used.';
        }
    </script>
</body>
</html>
```

- [ ] **Step 8: Run all handler tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services && go test ./auth/internal/handler/ -v`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add services/auth/internal/handler/register.go services/auth/internal/handler/register_test.go \
       services/auth/internal/handler/verify.go services/auth/internal/handler/verify_test.go \
       services/auth/static/
git commit -m "feat(auth): add register and verify handlers with static verification page"
```

---

### Task 10: Auth Service handlers - rotate-key and validate

**Files:**
- Create: `services/auth/internal/handler/rotate_key.go`
- Create: `services/auth/internal/handler/rotate_key_test.go`
- Create: `services/auth/internal/handler/validate.go`
- Create: `services/auth/internal/handler/validate_test.go`

- [ ] **Step 1: Write rotate_key_test.go (failing test)**

```go
// services/auth/internal/handler/rotate_key_test.go
package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/auth/internal/handler"
)

func TestRotateKeyHandler_MissingAuth(t *testing.T) {
	h := handler.NewRotateKeyHandler(handler.RotateKeyHandlerParams{
		DB: nil,
	})

	req := httptest.NewRequest(http.MethodPost, "/internal/auth/rotate-key", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
```

- [ ] **Step 2: Write rotate_key.go**

```go
// services/auth/internal/handler/rotate_key.go
package handler

import (
	"log"
	"net/http"
	"strings"

	"github.com/devinat1/obsidian-meeting-notes/services/shared/httputil"
	"github.com/jackc/pgx/v5/pgxpool"
)

type RotateKeyHandler struct {
	db *pgxpool.Pool
}

type RotateKeyHandlerParams struct {
	DB *pgxpool.Pool
}

func NewRotateKeyHandler(params RotateKeyHandlerParams) *RotateKeyHandler {
	return &RotateKeyHandler{db: params.DB}
}

func (h *RotateKeyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		httputil.RespondError(w, http.StatusUnauthorized, "Missing or invalid Authorization header.")
		return
	}

	currentKey := strings.TrimPrefix(header, "Bearer ")
	currentHash := hashString(currentKey)
	ctx := r.Context()

	var userID int
	err := h.db.QueryRow(ctx,
		"SELECT id FROM users WHERE api_key_hash = $1 AND verified = TRUE",
		currentHash,
	).Scan(&userID)

	if err != nil {
		httputil.RespondError(w, http.StatusUnauthorized, "Invalid API key.")
		return
	}

	newKey, err := generateAPIKey()
	if err != nil {
		log.Printf("Error generating new API key: %v", err)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to generate new API key.")
		return
	}

	newHash := hashString(newKey)

	_, err = h.db.Exec(ctx,
		"UPDATE users SET api_key_hash = $1, updated_at = NOW() WHERE id = $2",
		newHash, userID,
	)
	if err != nil {
		log.Printf("Error rotating API key: %v", err)
		httputil.RespondError(w, http.StatusInternalServerError, "Failed to rotate API key.")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]string{
		"api_key": newKey,
	})
}
```

- [ ] **Step 3: Write validate_test.go (failing test)**

```go
// services/auth/internal/handler/validate_test.go
package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/auth/internal/handler"
)

func TestValidateHandler_MissingAuth(t *testing.T) {
	h := handler.NewValidateHandler(handler.ValidateHandlerParams{
		DB: nil,
	})

	req := httptest.NewRequest(http.MethodGet, "/internal/auth/validate", nil)
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
```

- [ ] **Step 4: Write validate.go**

```go
// services/auth/internal/handler/validate.go
package handler

import (
	"net/http"
	"strings"

	"github.com/devinat1/obsidian-meeting-notes/services/shared/httputil"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ValidateHandler struct {
	db *pgxpool.Pool
}

type ValidateHandlerParams struct {
	DB *pgxpool.Pool
}

func NewValidateHandler(params ValidateHandlerParams) *ValidateHandler {
	return &ValidateHandler{db: params.DB}
}

func (h *ValidateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		httputil.RespondError(w, http.StatusUnauthorized, "Missing or invalid Authorization header.")
		return
	}

	apiKey := strings.TrimPrefix(header, "Bearer ")
	apiKeyHash := hashString(apiKey)
	ctx := r.Context()

	var userID int
	err := h.db.QueryRow(ctx,
		"SELECT id FROM users WHERE api_key_hash = $1 AND verified = TRUE",
		apiKeyHash,
	).Scan(&userID)

	if err != nil {
		httputil.RespondError(w, http.StatusUnauthorized, "Invalid API key.")
		return
	}

	httputil.RespondJSON(w, http.StatusOK, map[string]int{
		"user_id": userID,
	})
}
```

- [ ] **Step 5: Run all handler tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services && go test ./auth/internal/handler/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add services/auth/internal/handler/rotate_key.go services/auth/internal/handler/rotate_key_test.go \
       services/auth/internal/handler/validate.go services/auth/internal/handler/validate_test.go
git commit -m "feat(auth): add rotate-key and validate handlers"
```

---

### Task 11: Auth Service background cleanup and main entrypoint

**Files:**
- Create: `services/auth/internal/background/cleanup.go`
- Create: `services/auth/internal/background/cleanup_test.go`
- Create: `services/auth/cmd/main.go`

- [ ] **Step 1: Write cleanup.go**

```go
// services/auth/internal/background/cleanup.go
package background

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func StartUnverifiedUserCleanup(ctx context.Context, db *pgxpool.Pool) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("Unverified user cleanup goroutine stopping.")
			return
		case <-ticker.C:
			cleanupUnverifiedUsers(ctx, db)
		}
	}
}

func cleanupUnverifiedUsers(ctx context.Context, db *pgxpool.Pool) {
	cutoff := time.Now().Add(-7 * 24 * time.Hour)

	tag, err := db.Exec(ctx,
		"DELETE FROM users WHERE verified = FALSE AND created_at < $1",
		cutoff,
	)
	if err != nil {
		log.Printf("Error cleaning up unverified users: %v", err)
		return
	}

	if tag.RowsAffected() > 0 {
		log.Printf("Cleaned up %d unverified users older than 7 days.", tag.RowsAffected())
	}
}
```

- [ ] **Step 2: Write cleanup_test.go**

```go
// services/auth/internal/background/cleanup_test.go
package background_test

import (
	"context"
	"testing"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/services/auth/internal/background"
)

func TestStartUnverifiedUserCleanup_Cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		background.StartUnverifiedUserCleanup(ctx, nil)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cleanup goroutine did not stop after context cancellation")
	}
}
```

- [ ] **Step 3: Run cleanup test**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services && go test ./auth/internal/background/ -v`
Expected: PASS

- [ ] **Step 4: Write main.go**

```go
// services/auth/cmd/main.go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/devinat1/obsidian-meeting-notes/services/auth/internal/background"
	"github.com/devinat1/obsidian-meeting-notes/services/auth/internal/config"
	"github.com/devinat1/obsidian-meeting-notes/services/auth/internal/database"
	"github.com/devinat1/obsidian-meeting-notes/services/auth/internal/email"
	"github.com/devinat1/obsidian-meeting-notes/services/auth/internal/handler"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Loading config: %v", err)
	}

	if err := database.RunMigrations(cfg.DatabaseURL); err != nil {
		log.Fatalf("Running migrations: %v", err)
	}
	log.Println("Database migrations complete.")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Connecting to database: %v", err)
	}
	defer db.Close()

	emailSender := email.NewSender(email.NewSenderParams{
		Host: cfg.SMTPHost,
		Port: cfg.SMTPPort,
		User: cfg.SMTPUser,
		Pass: cfg.SMTPPass,
		From: cfg.SMTPFrom,
	})

	go background.StartUnverifiedUserCleanup(ctx, db)

	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))

	r.Post("/internal/auth/register", handler.NewRegisterHandler(handler.RegisterHandlerParams{
		DB:          db,
		EmailSender: emailSender,
		BaseURL:     cfg.BaseURL,
	}).ServeHTTP)

	r.Get("/internal/auth/verify", handler.NewVerifyHandler(handler.VerifyHandlerParams{
		DB: db,
	}).ServeHTTP)

	r.Post("/internal/auth/rotate-key", handler.NewRotateKeyHandler(handler.RotateKeyHandlerParams{
		DB: db,
	}).ServeHTTP)

	r.Get("/internal/auth/validate", handler.NewValidateHandler(handler.ValidateHandlerParams{
		DB: db,
	}).ServeHTTP)

	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Auth service listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down auth service.")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Auth service stopped.")
}
```

- [ ] **Step 5: Verify it compiles**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services && go build ./auth/cmd/`
Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add services/auth/internal/background/ services/auth/cmd/
git commit -m "feat(auth): add background cleanup goroutine and main entrypoint"
```

---

### Task 12: Auth Service Dockerfile and Helm chart

**Files:**
- Create: `services/auth/Dockerfile`
- Create: `helm/helm-meeting-auth/Chart.yaml`
- Create: `helm/helm-meeting-auth/values.yaml`
- Create: `helm/helm-meeting-auth/templates/deployment.yaml`
- Create: `helm/helm-meeting-auth/templates/service.yaml`
- Create: `helm/helm-meeting-auth/templates/secret.yaml`

- [ ] **Step 1: Write Dockerfile**

```dockerfile
# services/auth/Dockerfile
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY shared/ shared/
COPY auth/ auth/

RUN CGO_ENABLED=0 GOOS=linux go build -o /auth-service ./auth/cmd/

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /auth-service /auth-service
COPY auth/static/ /static/

EXPOSE 8081

ENTRYPOINT ["/auth-service"]
```

- [ ] **Step 2: Create Helm Chart.yaml**

```yaml
# helm/helm-meeting-auth/Chart.yaml
apiVersion: v2
name: meeting-auth
description: Auth Service for Obsidian Meeting Notes
type: application
version: 0.1.0
appVersion: "latest"
```

- [ ] **Step 3: Create Helm values.yaml**

```yaml
# helm/helm-meeting-auth/values.yaml
namespace: liteagent

auth:
  image:
    repository: ghcr.io/devinat1/meeting-auth
    tag: "latest"
  port: 8081
  resources:
    requests:
      memory: "64Mi"
      cpu: "50m"
    limits:
      memory: "128Mi"
      cpu: "250m"

nodeSelector: {}
```

- [ ] **Step 4: Create Helm templates/secret.yaml**

```yaml
# helm/helm-meeting-auth/templates/secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: meeting-auth-secret
  namespace: {{ .Values.namespace }}
type: Opaque
stringData:
  DATABASE_URL: {{ required "auth.databaseURL is required" .Values.auth.databaseURL }}
  SMTP_HOST: {{ required "auth.smtp.host is required" .Values.auth.smtp.host }}
  SMTP_PORT: {{ .Values.auth.smtp.port | default "587" | quote }}
  SMTP_USER: {{ .Values.auth.smtp.user | default "" | quote }}
  SMTP_PASS: {{ .Values.auth.smtp.pass | default "" | quote }}
  SMTP_FROM: {{ required "auth.smtp.from is required" .Values.auth.smtp.from }}
  BASE_URL: {{ required "auth.baseURL is required" .Values.auth.baseURL }}
  PORT: {{ .Values.auth.port | quote }}
```

- [ ] **Step 5: Create Helm templates/deployment.yaml**

```yaml
# helm/helm-meeting-auth/templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: meeting-auth
  namespace: {{ .Values.namespace }}
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: meeting-auth
  template:
    metadata:
      labels:
        app: meeting-auth
    spec:
      {{- with .Values.nodeSelector }}
      nodeSelector:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      containers:
        - name: auth
          image: {{ .Values.auth.image.repository }}:{{ .Values.auth.image.tag }}
          ports:
            - containerPort: {{ .Values.auth.port }}
              name: http
          envFrom:
            - secretRef:
                name: meeting-auth-secret
          resources:
            {{- toYaml .Values.auth.resources | nindent 12 }}
          readinessProbe:
            httpGet:
              path: /internal/auth/validate
              port: {{ .Values.auth.port }}
            initialDelaySeconds: 5
            periodSeconds: 10
            failureThreshold: 3
          livenessProbe:
            httpGet:
              path: /internal/auth/validate
              port: {{ .Values.auth.port }}
            initialDelaySeconds: 10
            periodSeconds: 30
            failureThreshold: 3
```

- [ ] **Step 6: Create Helm templates/service.yaml**

```yaml
# helm/helm-meeting-auth/templates/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: meeting-auth
  namespace: {{ .Values.namespace }}
spec:
  selector:
    app: meeting-auth
  ports:
    - port: {{ .Values.auth.port }}
      targetPort: {{ .Values.auth.port }}
      protocol: TCP
      name: http
```

- [ ] **Step 7: Validate Helm template renders**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin && helm template meeting-auth helm/helm-meeting-auth/ --set auth.databaseURL="postgres://auth:pass@pg-auth:5432/auth?sslmode=disable" --set auth.smtp.host="smtp.example.com" --set auth.smtp.from="noreply@example.com" --set auth.baseURL="https://api.liteagent.net"`
Expected: renders all 3 YAML documents with no errors

- [ ] **Step 8: Commit**

```bash
git add services/auth/Dockerfile helm/helm-meeting-auth/
git commit -m "feat(auth): add Dockerfile and Helm chart for Auth Service"
```

---

## Chunk 4: Gateway Service - Core

### Task 13: Gateway Service config and auth client

**Files:**
- Create: `services/gateway/internal/config/config.go`
- Create: `services/gateway/internal/authclient/client.go`
- Create: `services/gateway/internal/authclient/client_test.go`

- [ ] **Step 1: Write config.go**

```go
// services/gateway/internal/config/config.go
package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port                string
	AuthServiceURL      string
	CalendarServiceURL  string
	BotServiceURL       string
	TranscriptServiceURL string
	WebhookServiceURL   string
	KafkaBrokers        string
}

func Load() (*Config, error) {
	c := &Config{
		Port:                 getEnvOrDefault("PORT", "8080"),
		AuthServiceURL:       os.Getenv("AUTH_SERVICE_URL"),
		CalendarServiceURL:   os.Getenv("CALENDAR_SERVICE_URL"),
		BotServiceURL:        os.Getenv("BOT_SERVICE_URL"),
		TranscriptServiceURL: os.Getenv("TRANSCRIPT_SERVICE_URL"),
		WebhookServiceURL:    os.Getenv("WEBHOOK_SERVICE_URL"),
		KafkaBrokers:         os.Getenv("KAFKA_BROKERS"),
	}

	if c.AuthServiceURL == "" {
		return nil, fmt.Errorf("AUTH_SERVICE_URL is required")
	}
	if c.KafkaBrokers == "" {
		return nil, fmt.Errorf("KAFKA_BROKERS is required")
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

- [ ] **Step 2: Write authclient/client_test.go (failing test)**

```go
// services/gateway/internal/authclient/client_test.go
package authclient_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/authclient"
)

func TestValidate_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/auth/validate" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{"user_id": 42})
	}))
	defer server.Close()

	client := authclient.New(authclient.NewParams{
		AuthServiceURL: server.URL,
	})

	userID, err := client.Validate("test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userID != 42 {
		t.Fatalf("expected user_id 42, got %d", userID)
	}
}

func TestValidate_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid API key."})
	}))
	defer server.Close()

	client := authclient.New(authclient.NewParams{
		AuthServiceURL: server.URL,
	})

	_, err := client.Validate("bad-key")
	if err == nil {
		t.Fatal("expected error for unauthorized key")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services && go test ./gateway/internal/authclient/ -v`
Expected: FAIL (package doesn't compile)

- [ ] **Step 4: Write authclient/client.go**

```go
// services/gateway/internal/authclient/client.go
package authclient

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	authServiceURL string
	httpClient     *http.Client
}

type NewParams struct {
	AuthServiceURL string
}

func New(params NewParams) *Client {
	return &Client{
		authServiceURL: params.AuthServiceURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

type validateResponse struct {
	UserID int `json:"user_id"`
}

func (c *Client) Validate(apiKey string) (int, error) {
	req, err := http.NewRequest(http.MethodGet, c.authServiceURL+"/internal/auth/validate", nil)
	if err != nil {
		return 0, fmt.Errorf("creating validate request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("calling auth service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("auth service returned status %d", resp.StatusCode)
	}

	var result validateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decoding validate response: %w", err)
	}

	return result.UserID, nil
}
```

- [ ] **Step 5: Run authclient tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services && go test ./gateway/internal/authclient/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add services/gateway/internal/config/ services/gateway/internal/authclient/
git commit -m "feat(gateway): add config loading and auth service client"
```

---

### Task 14: Gateway auth middleware and reverse proxy

**Files:**
- Create: `services/gateway/internal/middleware/auth.go`
- Create: `services/gateway/internal/middleware/auth_test.go`
- Create: `services/gateway/internal/proxy/proxy.go`
- Create: `services/gateway/internal/proxy/proxy_test.go`

- [ ] **Step 1: Write auth_test.go (failing test)**

```go
// services/gateway/internal/middleware/auth_test.go
package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/authclient"
	"github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/middleware"
)

func TestAuthMiddleware_ValidKey(t *testing.T) {
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{"user_id": 7})
	}))
	defer authServer.Close()

	client := authclient.New(authclient.NewParams{
		AuthServiceURL: authServer.URL,
	})

	handler := middleware.Auth(client)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := middleware.UserIDFromContext(r.Context())
		if userID != 7 {
			t.Fatalf("expected user_id 7, got %d", userID)
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer valid-key")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	client := authclient.New(authclient.NewParams{
		AuthServiceURL: "http://localhost:0",
	})

	handler := middleware.Auth(client)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
```

- [ ] **Step 2: Write auth.go**

```go
// services/gateway/internal/middleware/auth.go
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/authclient"
	"github.com/devinat1/obsidian-meeting-notes/services/shared/httputil"
)

type contextKey string

const userIDKey contextKey = "userID"

func Auth(client *authclient.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				httputil.RespondError(w, http.StatusUnauthorized, "Missing or invalid Authorization header.")
				return
			}

			apiKey := strings.TrimPrefix(header, "Bearer ")
			userID, err := client.Validate(apiKey)
			if err != nil {
				httputil.RespondError(w, http.StatusUnauthorized, "Invalid API key.")
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserIDFromContext(ctx context.Context) int {
	userID, ok := ctx.Value(userIDKey).(int)
	if !ok {
		return 0
	}
	return userID
}
```

- [ ] **Step 3: Run auth middleware tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services && go test ./gateway/internal/middleware/ -v`
Expected: PASS

- [ ] **Step 4: Write proxy_test.go (failing test)**

```go
// services/gateway/internal/proxy/proxy_test.go
package proxy_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/proxy"
)

func TestReverseProxy(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/calendar/meetings" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"meetings":[]}`))
	}))
	defer backend.Close()

	p := proxy.New(proxy.NewParams{
		TargetURL: backend.URL,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/meetings", nil)
	w := httptest.NewRecorder()

	p.Forward(w, req, "/internal/calendar/meetings")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body, _ := io.ReadAll(w.Body)
	if string(body) != `{"meetings":[]}` {
		t.Fatalf("unexpected body: %s", string(body))
	}
}
```

- [ ] **Step 5: Write proxy.go**

```go
// services/gateway/internal/proxy/proxy.go
package proxy

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type Proxy struct {
	targetURL  string
	httpClient *http.Client
}

type NewParams struct {
	TargetURL string
}

func New(params NewParams) *Proxy {
	return &Proxy{
		targetURL: params.TargetURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (p *Proxy) Forward(w http.ResponseWriter, originalRequest *http.Request, targetPath string) {
	url := p.targetURL + targetPath
	if originalRequest.URL.RawQuery != "" {
		url += "?" + originalRequest.URL.RawQuery
	}

	proxyReq, err := http.NewRequestWithContext(
		originalRequest.Context(),
		originalRequest.Method,
		url,
		originalRequest.Body,
	)
	if err != nil {
		log.Printf("Error creating proxy request: %v", err)
		http.Error(w, fmt.Sprintf(`{"error":"proxy error"}`), http.StatusBadGateway)
		return
	}

	for key, values := range originalRequest.Header {
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}

	resp, err := p.httpClient.Do(proxyReq)
	if err != nil {
		log.Printf("Error forwarding request to %s: %v", url, err)
		http.Error(w, fmt.Sprintf(`{"error":"upstream service unavailable"}`), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
```

- [ ] **Step 6: Run proxy tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services && go test ./gateway/internal/proxy/ -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add services/gateway/internal/middleware/ services/gateway/internal/proxy/
git commit -m "feat(gateway): add auth middleware and reverse proxy"
```

---

## Chunk 5: Gateway Service - SSE & Rate Limiting

### Task 15: SSE hub with Last-Event-ID catch-up buffer

**Files:**
- Create: `services/gateway/internal/sse/hub.go`
- Create: `services/gateway/internal/sse/hub_test.go`

- [ ] **Step 1: Write hub_test.go (failing test)**

```go
// services/gateway/internal/sse/hub_test.go
package sse_test

import (
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/sse"
)

func TestHub_RegisterAndSend(t *testing.T) {
	hub := sse.NewHub()

	ch := hub.Register(42)
	defer hub.Unregister(42)

	hub.Send(42, sse.Event{
		Type: "bot.status",
		Data: `{"bot_id":"abc","bot_status":"recording"}`,
	})

	select {
	case event := <-ch:
		if event.Type != "bot.status" {
			t.Fatalf("expected event type bot.status, got %s", event.Type)
		}
		if event.ID == 0 {
			t.Fatal("expected non-zero event ID")
		}
	default:
		t.Fatal("expected to receive event")
	}
}

func TestHub_BufferCatchUp(t *testing.T) {
	hub := sse.NewHub()

	ch := hub.Register(42)
	hub.Unregister(42)

	hub.Send(42, sse.Event{Type: "bot.status", Data: "event1"})
	hub.Send(42, sse.Event{Type: "bot.status", Data: "event2"})
	hub.Send(42, sse.Event{Type: "bot.status", Data: "event3"})

	events := hub.CatchUp(42, 1)
	if len(events) != 2 {
		t.Fatalf("expected 2 catch-up events, got %d", len(events))
	}
	if events[0].Data != "event2" {
		t.Fatalf("expected event2, got %s", events[0].Data)
	}
	if events[1].Data != "event3" {
		t.Fatalf("expected event3, got %s", events[1].Data)
	}

	_ = ch
}

func TestHub_UnregisterRemovesChannel(t *testing.T) {
	hub := sse.NewHub()

	hub.Register(42)
	hub.Unregister(42)

	if hub.HasConnection(42) {
		t.Fatal("expected no connection after unregister")
	}
}

func TestHub_BufferMaxSize(t *testing.T) {
	hub := sse.NewHub()

	for i := 0; i < 120; i++ {
		hub.Send(99, sse.Event{Type: "test", Data: "data"})
	}

	events := hub.CatchUp(99, 0)
	if len(events) != 100 {
		t.Fatalf("expected buffer capped at 100, got %d", len(events))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services && go test ./gateway/internal/sse/ -v`
Expected: FAIL

- [ ] **Step 3: Write hub.go**

```go
// services/gateway/internal/sse/hub.go
package sse

import (
	"sync"
	"sync/atomic"
)

const maxBufferSize = 100

type Event struct {
	ID   uint64
	Type string
	Data string
}

type Hub struct {
	mu          sync.RWMutex
	connections map[int]chan Event
	buffers     map[int][]Event
	counter     atomic.Uint64
}

func NewHub() *Hub {
	return &Hub{
		connections: make(map[int]chan Event),
		buffers:     make(map[int][]Event),
	}
}

func (h *Hub) Register(userID int) <-chan Event {
	h.mu.Lock()
	defer h.mu.Unlock()

	ch := make(chan Event, 64)
	h.connections[userID] = ch
	return ch
}

func (h *Hub) Unregister(userID int) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if ch, ok := h.connections[userID]; ok {
		close(ch)
		delete(h.connections, userID)
	}
}

func (h *Hub) HasConnection(userID int) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	_, ok := h.connections[userID]
	return ok
}

func (h *Hub) Send(userID int, event Event) {
	event.ID = h.counter.Add(1)

	h.mu.Lock()
	buf := h.buffers[userID]
	buf = append(buf, event)
	if len(buf) > maxBufferSize {
		buf = buf[len(buf)-maxBufferSize:]
	}
	h.buffers[userID] = buf
	h.mu.Unlock()

	h.mu.RLock()
	ch, ok := h.connections[userID]
	h.mu.RUnlock()

	if ok {
		select {
		case ch <- event:
		default:
		}
	}
}

func (h *Hub) CatchUp(userID int, lastEventID uint64) []Event {
	h.mu.RLock()
	defer h.mu.RUnlock()

	buf := h.buffers[userID]
	if len(buf) == 0 {
		return nil
	}

	var result []Event
	for _, event := range buf {
		if event.ID > lastEventID {
			result = append(result, event)
		}
	}
	return result
}
```

- [ ] **Step 4: Run hub tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services && go test ./gateway/internal/sse/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add services/gateway/internal/sse/hub.go services/gateway/internal/sse/hub_test.go
git commit -m "feat(gateway): add SSE hub with Last-Event-ID catch-up buffer"
```

---

### Task 16: SSE HTTP handler

**Files:**
- Create: `services/gateway/internal/sse/handler.go`
- Create: `services/gateway/internal/sse/handler_test.go`

- [ ] **Step 1: Write handler_test.go (failing test)**

```go
// services/gateway/internal/sse/handler_test.go
package sse_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/sse"
)

type mockUserIDContextKey string

const testUserIDKey mockUserIDContextKey = "userID"

func TestSSEHandler_StreamsEvents(t *testing.T) {
	hub := sse.NewHub()
	handler := sse.NewHandler(sse.NewHandlerParams{
		Hub:              hub,
		UserIDFromContext: func(ctx context.Context) int { return 42 },
		HeartbeatInterval: 100 * time.Millisecond,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/events/stream", nil)
	w := httptest.NewRecorder()

	ctx, cancel := context.WithTimeout(req.Context(), 300*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	go func() {
		time.Sleep(50 * time.Millisecond)
		hub.Send(42, sse.Event{Type: "bot.status", Data: `{"status":"recording"}`})
	}()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "event: bot.status") {
		t.Fatalf("expected SSE event in body, got: %s", body)
	}
	if !strings.Contains(body, `data: {"status":"recording"}`) {
		t.Fatalf("expected data in body, got: %s", body)
	}
}

func TestSSEHandler_SendsHeartbeat(t *testing.T) {
	hub := sse.NewHub()
	handler := sse.NewHandler(sse.NewHandlerParams{
		Hub:              hub,
		UserIDFromContext: func(ctx context.Context) int { return 42 },
		HeartbeatInterval: 50 * time.Millisecond,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/events/stream", nil)
	w := httptest.NewRecorder()

	ctx, cancel := context.WithTimeout(req.Context(), 200*time.Millisecond)
	defer cancel()
	req = req.WithContext(ctx)

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "event: ping") {
		t.Fatalf("expected heartbeat ping in body, got: %s", body)
	}
}
```

- [ ] **Step 2: Write handler.go**

```go
// services/gateway/internal/sse/handler.go
package sse

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

type Handler struct {
	hub               *Hub
	userIDFromContext  func(ctx context.Context) int
	heartbeatInterval time.Duration
}

type NewHandlerParams struct {
	Hub               *Hub
	UserIDFromContext  func(ctx context.Context) int
	HeartbeatInterval time.Duration
}

func NewHandler(params NewHandlerParams) *Handler {
	interval := params.HeartbeatInterval
	if interval == 0 {
		interval = 30 * time.Second
	}
	return &Handler{
		hub:               params.Hub,
		userIDFromContext:  params.UserIDFromContext,
		heartbeatInterval: interval,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	userID := h.userIDFromContext(r.Context())
	if userID == 0 {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":"streaming not supported"}`, http.StatusInternalServerError)
		return
	}

	if h.hub.HasConnection(userID) {
		http.Error(w, `{"error":"concurrent SSE connection limit reached"}`, http.StatusTooManyRequests)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch := h.hub.Register(userID)
	defer h.hub.Unregister(userID)

	lastEventIDStr := r.Header.Get("Last-Event-ID")
	if lastEventIDStr == "" {
		lastEventIDStr = r.URL.Query().Get("last_event_id")
	}

	if lastEventIDStr != "" {
		lastEventID, err := strconv.ParseUint(lastEventIDStr, 10, 64)
		if err == nil {
			catchUpEvents := h.hub.CatchUp(userID, lastEventID)
			for _, event := range catchUpEvents {
				writeSSEEvent(w, event)
			}
			flusher.Flush()
		}
	}

	ticker := time.NewTicker(h.heartbeatInterval)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			log.Printf("SSE connection closed for user %d.", userID)
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			writeSSEEvent(w, event)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, "event: ping\ndata: {}\n\n")
			flusher.Flush()
		}
	}
}

func writeSSEEvent(w http.ResponseWriter, event Event) {
	fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", event.ID, event.Type, event.Data)
}
```

- [ ] **Step 3: Run handler tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services && go test ./gateway/internal/sse/ -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add services/gateway/internal/sse/handler.go services/gateway/internal/sse/handler_test.go
git commit -m "feat(gateway): add SSE HTTP handler with heartbeat and catch-up replay"
```

---

### Task 17: Rate limiter middleware

**Files:**
- Create: `services/gateway/internal/ratelimit/ratelimit.go`
- Create: `services/gateway/internal/ratelimit/ratelimit_test.go`

- [ ] **Step 1: Write ratelimit_test.go (failing test)**

```go
// services/gateway/internal/ratelimit/ratelimit_test.go
package ratelimit_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/ratelimit"
)

func TestTokenBucket_AllowsWithinLimit(t *testing.T) {
	limiter := ratelimit.NewTokenBucket(ratelimit.TokenBucketParams{
		Rate:     10,
		Capacity: 10,
	})

	for i := 0; i < 10; i++ {
		if !limiter.Allow("user-1") {
			t.Fatalf("expected request %d to be allowed", i+1)
		}
	}
}

func TestTokenBucket_BlocksOverLimit(t *testing.T) {
	limiter := ratelimit.NewTokenBucket(ratelimit.TokenBucketParams{
		Rate:     2,
		Capacity: 2,
	})

	limiter.Allow("user-1")
	limiter.Allow("user-1")

	if limiter.Allow("user-1") {
		t.Fatal("expected third request to be blocked")
	}
}

func TestTokenBucket_SeparateKeys(t *testing.T) {
	limiter := ratelimit.NewTokenBucket(ratelimit.TokenBucketParams{
		Rate:     1,
		Capacity: 1,
	})

	if !limiter.Allow("user-1") {
		t.Fatal("expected user-1 first request to be allowed")
	}
	if !limiter.Allow("user-2") {
		t.Fatal("expected user-2 first request to be allowed")
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	limiter := ratelimit.NewTokenBucket(ratelimit.TokenBucketParams{
		Rate:     1,
		Capacity: 1,
	})

	handler := ratelimit.Middleware(ratelimit.MiddlewareParams{
		Limiter: limiter,
		KeyFunc: func(r *http.Request) string {
			return r.Header.Get("X-User-ID")
		},
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req1 := httptest.NewRequest(http.MethodGet, "/", nil)
	req1.Header.Set("X-User-ID", "42")
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w1.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/", nil)
	req2.Header.Set("X-User-ID", "42")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w2.Code)
	}
}
```

- [ ] **Step 2: Write ratelimit.go**

```go
// services/gateway/internal/ratelimit/ratelimit.go
package ratelimit

import (
	"net/http"
	"sync"
	"time"

	"github.com/devinat1/obsidian-meeting-notes/services/shared/httputil"
)

type bucket struct {
	tokens   float64
	lastTime time.Time
}

type TokenBucket struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	rate     float64
	capacity float64
}

type TokenBucketParams struct {
	Rate     float64
	Capacity float64
}

func NewTokenBucket(params TokenBucketParams) *TokenBucket {
	return &TokenBucket{
		buckets:  make(map[string]*bucket),
		rate:     params.Rate,
		capacity: params.Capacity,
	}
}

func (tb *TokenBucket) Allow(key string) bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()

	b, exists := tb.buckets[key]
	if !exists {
		tb.buckets[key] = &bucket{
			tokens:   tb.capacity - 1,
			lastTime: now,
		}
		return true
	}

	elapsed := now.Sub(b.lastTime).Seconds()
	b.tokens += elapsed * tb.rate
	if b.tokens > tb.capacity {
		b.tokens = tb.capacity
	}
	b.lastTime = now

	if b.tokens < 1 {
		return false
	}

	b.tokens--
	return true
}

type MiddlewareParams struct {
	Limiter *TokenBucket
	KeyFunc func(r *http.Request) string
}

func Middleware(params MiddlewareParams) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := params.KeyFunc(r)
			if !params.Limiter.Allow(key) {
				httputil.RespondError(w, http.StatusTooManyRequests, "Rate limit exceeded. Try again later.")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 3: Run ratelimit tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services && go test ./gateway/internal/ratelimit/ -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add services/gateway/internal/ratelimit/
git commit -m "feat(gateway): add in-memory token bucket rate limiter middleware"
```

---

## Chunk 6: Gateway Service - Kafka Consumer & Entrypoint

### Task 18: Gateway Kafka consumer that pushes to SSE

**Files:**
- Create: `services/gateway/internal/kafka/consumer.go`
- Create: `services/gateway/internal/kafka/consumer_test.go`

- [ ] **Step 1: Write consumer_test.go (failing test)**

```go
// services/gateway/internal/kafka/consumer_test.go
package kafka_test

import (
	"encoding/json"
	"testing"

	gatewaykafka "github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/kafka"
	"github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/sse"
)

func TestExtractUserID(t *testing.T) {
	tests := []struct {
		name     string
		payload  map[string]any
		expected int
		hasError bool
	}{
		{
			name:     "valid user_id",
			payload:  map[string]any{"user_id": float64(42)},
			expected: 42,
			hasError: false,
		},
		{
			name:     "missing user_id",
			payload:  map[string]any{"bot_id": "abc"},
			expected: 0,
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, _ := json.Marshal(tt.payload)
			userID, err := gatewaykafka.ExtractUserID(data)
			if tt.hasError && err == nil {
				t.Fatal("expected error")
			}
			if !tt.hasError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if userID != tt.expected {
				t.Fatalf("expected user_id %d, got %d", tt.expected, userID)
			}
		})
	}
}

func TestDispatchToHub(t *testing.T) {
	hub := sse.NewHub()
	ch := hub.Register(42)

	gatewaykafka.DispatchToHub(hub, 42, "bot.status", `{"bot_id":"abc","status":"recording"}`)

	select {
	case event := <-ch:
		if event.Type != "bot.status" {
			t.Fatalf("expected event type bot.status, got %s", event.Type)
		}
	default:
		t.Fatal("expected event on channel")
	}

	hub.Unregister(42)
}
```

- [ ] **Step 2: Write consumer.go**

```go
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
```

- [ ] **Step 3: Run kafka tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services && go test ./gateway/internal/kafka/ -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add services/gateway/internal/kafka/
git commit -m "feat(gateway): add Kafka consumers that dispatch events to SSE hub"
```

---

### Task 19: Gateway Service main entrypoint

**Files:**
- Create: `services/gateway/cmd/main.go`

- [ ] **Step 1: Write main.go**

```go
// services/gateway/cmd/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/authclient"
	"github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/config"
	gatewaykafka "github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/kafka"
	"github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/middleware"
	"github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/proxy"
	"github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/ratelimit"
	"github.com/devinat1/obsidian-meeting-notes/services/gateway/internal/sse"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Loading config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	authClient := authclient.New(authclient.NewParams{
		AuthServiceURL: cfg.AuthServiceURL,
	})

	hub := sse.NewHub()

	brokers := gatewaykafka.BrokersFromString(cfg.KafkaBrokers)
	gatewaykafka.StartConsumers(ctx, gatewaykafka.ConsumerGroupParams{
		Brokers: brokers,
		Hub:     hub,
	})

	authProxy := proxy.New(proxy.NewParams{TargetURL: cfg.AuthServiceURL})
	calendarProxy := proxy.New(proxy.NewParams{TargetURL: cfg.CalendarServiceURL})
	botProxy := proxy.New(proxy.NewParams{TargetURL: cfg.BotServiceURL})
	transcriptProxy := proxy.New(proxy.NewParams{TargetURL: cfg.TranscriptServiceURL})
	webhookProxy := proxy.New(proxy.NewParams{TargetURL: cfg.WebhookServiceURL})

	registerLimiter := ratelimit.NewTokenBucket(ratelimit.TokenBucketParams{
		Rate:     5.0 / 3600.0,
		Capacity: 5,
	})

	generalLimiter := ratelimit.NewTokenBucket(ratelimit.TokenBucketParams{
		Rate:     60.0 / 60.0,
		Capacity: 60,
	})

	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(30 * time.Second))

	r.Post("/api/auth/register", func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if !registerLimiter.Allow(ip) {
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}
		authProxy.Forward(w, r, "/internal/auth/register")
	})

	r.Get("/api/auth/verify", func(w http.ResponseWriter, r *http.Request) {
		authProxy.Forward(w, r, "/internal/auth/verify")
	})

	r.Post("/api/webhooks/recallai", func(w http.ResponseWriter, r *http.Request) {
		webhookProxy.Forward(w, r, "/webhooks/recallai")
	})

	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(authClient))
		r.Use(ratelimit.Middleware(ratelimit.MiddlewareParams{
			Limiter: generalLimiter,
			KeyFunc: func(r *http.Request) string {
				return fmt.Sprintf("user-%d", middleware.UserIDFromContext(r.Context()))
			},
		}))

		r.Post("/api/auth/rotate-key", func(w http.ResponseWriter, r *http.Request) {
			authProxy.Forward(w, r, "/internal/auth/rotate-key")
		})

		r.Get("/api/auth/calendar/connect", func(w http.ResponseWriter, r *http.Request) {
			userID := middleware.UserIDFromContext(r.Context())
			calendarProxy.Forward(w, r, fmt.Sprintf("/internal/calendar/auth-url?user_id=%d", userID))
		})

		r.Get("/api/auth/calendar/callback", func(w http.ResponseWriter, r *http.Request) {
			calendarProxy.Forward(w, r, "/internal/calendar/callback")
		})

		r.Get("/api/events/stream", sse.NewHandler(sse.NewHandlerParams{
			Hub: hub,
			UserIDFromContext: func(ctx context.Context) int {
				return middleware.UserIDFromContext(ctx)
			},
			HeartbeatInterval: 30 * time.Second,
		}).ServeHTTP)

		r.Get("/api/meetings", func(w http.ResponseWriter, r *http.Request) {
			userID := middleware.UserIDFromContext(r.Context())
			calendarProxy.Forward(w, r, fmt.Sprintf("/internal/calendar/meetings?user_id=%d", userID))
		})

		r.Get("/api/bots/{botId}/transcript", func(w http.ResponseWriter, r *http.Request) {
			botID := chi.URLParam(r, "botId")
			transcriptProxy.Forward(w, r, fmt.Sprintf("/internal/transcripts/%s", botID))
		})

		r.Delete("/api/bots/{botId}", func(w http.ResponseWriter, r *http.Request) {
			botID := chi.URLParam(r, "botId")
			userID := middleware.UserIDFromContext(r.Context())
			botProxy.Forward(w, r, fmt.Sprintf("/internal/bots/%s?user_id=%d", botID, userID))
		})
	})

	r.Get("/api/events/stream", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, `{"error":"token query parameter required for SSE"}`, http.StatusUnauthorized)
			return
		}

		userID, err := authClient.Validate(token)
		if err != nil {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}

		sseHandler := sse.NewHandler(sse.NewHandlerParams{
			Hub: hub,
			UserIDFromContext: func(ctx context.Context) int {
				return userID
			},
			HeartbeatInterval: 30 * time.Second,
		})
		sseHandler.ServeHTTP(w, r)
	})

	_ = authProxy
	_ = calendarProxy
	_ = botProxy
	_ = transcriptProxy
	_ = webhookProxy
	_ = strconv.Itoa

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 0,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("Gateway service listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Println("Shutting down gateway service.")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Gateway service stopped.")
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services && go build ./gateway/cmd/`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add services/gateway/cmd/
git commit -m "feat(gateway): add main entrypoint with chi router, proxy routing, SSE, and Kafka consumers"
```

---

### Task 20: Gateway Service Dockerfile and Helm chart with Ingress

**Files:**
- Create: `services/gateway/Dockerfile`
- Create: `helm/helm-meeting-gateway/Chart.yaml`
- Create: `helm/helm-meeting-gateway/values.yaml`
- Create: `helm/helm-meeting-gateway/templates/deployment.yaml`
- Create: `helm/helm-meeting-gateway/templates/service.yaml`
- Create: `helm/helm-meeting-gateway/templates/secret.yaml`
- Create: `helm/helm-meeting-gateway/templates/ingress.yaml`

- [ ] **Step 1: Write Dockerfile**

```dockerfile
# services/gateway/Dockerfile
FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY shared/ shared/
COPY gateway/ gateway/

RUN CGO_ENABLED=0 GOOS=linux go build -o /gateway-service ./gateway/cmd/

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /gateway-service /gateway-service

EXPOSE 8080

ENTRYPOINT ["/gateway-service"]
```

- [ ] **Step 2: Create Helm Chart.yaml**

```yaml
# helm/helm-meeting-gateway/Chart.yaml
apiVersion: v2
name: meeting-gateway
description: Gateway Service for Obsidian Meeting Notes
type: application
version: 0.1.0
appVersion: "latest"
```

- [ ] **Step 3: Create Helm values.yaml**

```yaml
# helm/helm-meeting-gateway/values.yaml
namespace: liteagent

gateway:
  image:
    repository: ghcr.io/devinat1/meeting-gateway
    tag: "latest"
  port: 8080
  resources:
    requests:
      memory: "64Mi"
      cpu: "50m"
    limits:
      memory: "128Mi"
      cpu: "250m"

ingress:
  enabled: true
  host: api.liteagent.net
  tlsSecretName: api-liteagent-tls
  clusterIssuer: letsencrypt-prod

nodeSelector: {}
```

- [ ] **Step 4: Create Helm templates/secret.yaml**

```yaml
# helm/helm-meeting-gateway/templates/secret.yaml
apiVersion: v1
kind: Secret
metadata:
  name: meeting-gateway-secret
  namespace: {{ .Values.namespace }}
type: Opaque
stringData:
  PORT: {{ .Values.gateway.port | quote }}
  AUTH_SERVICE_URL: {{ required "gateway.authServiceURL is required" .Values.gateway.authServiceURL }}
  CALENDAR_SERVICE_URL: {{ .Values.gateway.calendarServiceURL | default "" | quote }}
  BOT_SERVICE_URL: {{ .Values.gateway.botServiceURL | default "" | quote }}
  TRANSCRIPT_SERVICE_URL: {{ .Values.gateway.transcriptServiceURL | default "" | quote }}
  WEBHOOK_SERVICE_URL: {{ .Values.gateway.webhookServiceURL | default "" | quote }}
  KAFKA_BROKERS: {{ required "gateway.kafkaBrokers is required" .Values.gateway.kafkaBrokers }}
```

- [ ] **Step 5: Create Helm templates/deployment.yaml**

```yaml
# helm/helm-meeting-gateway/templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: meeting-gateway
  namespace: {{ .Values.namespace }}
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app: meeting-gateway
  template:
    metadata:
      labels:
        app: meeting-gateway
    spec:
      {{- with .Values.nodeSelector }}
      nodeSelector:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      containers:
        - name: gateway
          image: {{ .Values.gateway.image.repository }}:{{ .Values.gateway.image.tag }}
          ports:
            - containerPort: {{ .Values.gateway.port }}
              name: http
          envFrom:
            - secretRef:
                name: meeting-gateway-secret
          resources:
            {{- toYaml .Values.gateway.resources | nindent 12 }}
          readinessProbe:
            httpGet:
              path: /api/auth/verify
              port: {{ .Values.gateway.port }}
            initialDelaySeconds: 5
            periodSeconds: 10
            failureThreshold: 3
          livenessProbe:
            httpGet:
              path: /api/auth/verify
              port: {{ .Values.gateway.port }}
            initialDelaySeconds: 10
            periodSeconds: 30
            failureThreshold: 3
```

- [ ] **Step 6: Create Helm templates/service.yaml**

```yaml
# helm/helm-meeting-gateway/templates/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: meeting-gateway
  namespace: {{ .Values.namespace }}
spec:
  selector:
    app: meeting-gateway
  ports:
    - port: {{ .Values.gateway.port }}
      targetPort: {{ .Values.gateway.port }}
      protocol: TCP
      name: http
```

- [ ] **Step 7: Create Helm templates/ingress.yaml**

```yaml
# helm/helm-meeting-gateway/templates/ingress.yaml
{{- if .Values.ingress.enabled }}
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: meeting-gateway
  namespace: {{ .Values.namespace }}
  annotations:
    cert-manager.io/cluster-issuer: {{ .Values.ingress.clusterIssuer }}
    nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"
    nginx.ingress.kubernetes.io/proxy-send-timeout: "3600"
    nginx.ingress.kubernetes.io/proxy-buffering: "off"
spec:
  ingressClassName: nginx
  tls:
    - hosts:
        - {{ .Values.ingress.host }}
      secretName: {{ .Values.ingress.tlsSecretName }}
  rules:
    - host: {{ .Values.ingress.host }}
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: meeting-gateway
                port:
                  number: {{ .Values.gateway.port }}
{{- end }}
```

- [ ] **Step 8: Validate Helm template renders**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin && helm template meeting-gateway helm/helm-meeting-gateway/ --set gateway.authServiceURL="http://meeting-auth:8081" --set gateway.kafkaBrokers="kafka.liteagent.svc.cluster.local:9092"`
Expected: renders all 4 YAML documents (deployment, service, secret, ingress) with no errors

- [ ] **Step 9: Commit**

```bash
git add services/gateway/Dockerfile helm/helm-meeting-gateway/
git commit -m "feat(gateway): add Dockerfile and Helm chart with Ingress for api.liteagent.net"
```

---

## Chunk 7: Integration Verification

### Task 21: Run full test suite and verify all compilation

**Files:**
- Modify: none (verification only)

- [ ] **Step 1: Run go mod tidy**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services && go mod tidy`
Expected: no errors

- [ ] **Step 2: Run all tests**

Run: `cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services && go test ./... -v`
Expected: all tests PASS

- [ ] **Step 3: Build both services**

```bash
cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin/services
go build ./auth/cmd/
go build ./gateway/cmd/
```

Expected: both compile without errors

- [ ] **Step 4: Validate all Helm charts**

```bash
cd /Users/devinat1/conductor/workspaces/obsidian-meeting-notes/austin
helm template pg-auth helm/helm-pg-auth/ --set postgres.password=test
helm template pg-cal helm/helm-pg-cal/ --set postgres.password=test
helm template pg-bot helm/helm-pg-bot/ --set postgres.password=test
helm template pg-transcript helm/helm-pg-transcript/ --set postgres.password=test
helm template meeting-auth helm/helm-meeting-auth/ --set auth.databaseURL="postgres://a:b@c:5432/d" --set auth.smtp.host="smtp.x.com" --set auth.smtp.from="a@b.com" --set auth.baseURL="https://api.liteagent.net"
helm template meeting-gateway helm/helm-meeting-gateway/ --set gateway.authServiceURL="http://meeting-auth:8081" --set gateway.kafkaBrokers="kafka.liteagent.svc.cluster.local:9092"
```

Expected: all 6 charts render without errors

- [ ] **Step 5: Final commit**

```bash
git add -A
git commit -m "chore: verify full test suite and compilation for Plan 1"
```

---

## Deployment Commands Reference

Deploy all Plan 1 infrastructure to the `liteagent` namespace:

```bash
helm upgrade --install pg-auth helm/helm-pg-auth/ \
  --namespace liteagent \
  --set postgres.password="<PG_AUTH_PASSWORD>"

helm upgrade --install pg-cal helm/helm-pg-cal/ \
  --namespace liteagent \
  --set postgres.password="<PG_CAL_PASSWORD>"

helm upgrade --install pg-bot helm/helm-pg-bot/ \
  --namespace liteagent \
  --set postgres.password="<PG_BOT_PASSWORD>"

helm upgrade --install pg-transcript helm/helm-pg-transcript/ \
  --namespace liteagent \
  --set postgres.password="<PG_TRANSCRIPT_PASSWORD>"

helm upgrade --install meeting-auth helm/helm-meeting-auth/ \
  --namespace liteagent \
  --set auth.databaseURL="postgres://auth:<PG_AUTH_PASSWORD>@pg-auth:5432/auth?sslmode=disable" \
  --set auth.smtp.host="<SMTP_HOST>" \
  --set auth.smtp.port="587" \
  --set auth.smtp.user="<SMTP_USER>" \
  --set auth.smtp.pass="<SMTP_PASS>" \
  --set auth.smtp.from="<SMTP_FROM>" \
  --set auth.baseURL="https://api.liteagent.net"

helm upgrade --install meeting-gateway helm/helm-meeting-gateway/ \
  --namespace liteagent \
  --set gateway.authServiceURL="http://meeting-auth:8081" \
  --set gateway.calendarServiceURL="http://meeting-calendar:8082" \
  --set gateway.botServiceURL="http://meeting-bot:8083" \
  --set gateway.transcriptServiceURL="http://meeting-transcript:8084" \
  --set gateway.webhookServiceURL="http://meeting-webhook:8085" \
  --set gateway.kafkaBrokers="kafka.liteagent.svc.cluster.local:9092"
```
