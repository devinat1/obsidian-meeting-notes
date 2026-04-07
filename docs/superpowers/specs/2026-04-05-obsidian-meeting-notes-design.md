# Obsidian Meeting Notes - Design Spec

## Overview

An auto-recording meeting notes system that detects meetings from Google Calendar, sends a Recall AI bot to record and transcribe them, generates summaries via LLM, and saves structured notes to Obsidian.

**Two deployment targets:**
1. **Backend microservices (Go)** - Self-hosted on homelab Kubernetes cluster (`liteagent` namespace), event-driven via Kafka
2. **Electron Tray App** - Installed by user. Thin client: SSE for real-time events, BYOK LLM summarization, Obsidian CLI integration. No database, no calendar logic.

---

## Architecture

```
                    Tray App (user's machine)
                    ┌──────────────────────────┐
                    │  SSE client              │
                    │  LLM summarizer (BYOK)   │
                    │  Obsidian CLI            │
                    │  Electron store (config) │
                    └────────────┬─────────────┘
                                 │ SSE + REST
                                 ▼
                    ┌──────────────────────────┐
                    │   Gateway Service        │
                    │   api.liteagent.net       │
                    │   REST API + SSE push    │
                    └────────────┬─────────────┘
                                 │
                    ┌────────────▼─────────────┐
                    │   Kafka (KRaft)          │
                    │   kafka.liteagent.svc    │
                    │   .cluster.local:9092    │
                    └────────────┬─────────────┘
              ┌──────────┬───────┼───────┬──────────┐
              ▼          ▼       ▼       ▼          ▼
         ┌────────┐ ┌────────┐ ┌────┐ ┌──────┐ ┌──────────┐
         │  Auth  │ │Calendar│ │Bot │ │Webhook│ │Transcript│
         │Service │ │Service │ │Svc │ │Service│ │ Service  │
         │        │ │        │ │    │ │       │ │          │
         │ PG-auth│ │ PG-cal │ │PG- │ │(none) │ │PG-       │
         │        │ │        │ │bot │ │       │ │transcript│
         └────────┘ └────────┘ └──┬─┘ └───┬───┘ └──────────┘
                                  │       │
                                  ▼       ▲
                              Recall AI ──┘
```

---

## Kafka Topics

| Topic | Producer | Consumer(s) | Payload |
|-------|----------|-------------|---------|
| `meeting.upcoming` | Calendar Service | Bot Service, Gateway | `{user_id, calendar_event_id, meeting_url, title, start_time}` |
| `meeting.cancelled` | Calendar Service | Bot Service | `{user_id, calendar_event_id, bot_id}` |
| `bot.status` | Bot Service | Gateway | `{user_id, bot_id, bot_status, meeting_title}` |
| `bot.events` | Webhook Service | Bot Service | Raw Recall webhook + `bot_id` as Kafka key |
| `recording.events` | Webhook Service | Bot Service | Raw Recall webhook + `bot_id` as Kafka key |
| `transcript.events` | Webhook Service | Transcript Service | Raw Recall webhook + `bot_id` as Kafka key |
| `transcript.ready` | Transcript Service | Gateway | `{user_id, bot_id, meeting_title, raw_transcript, readable_transcript}` |
| `transcript.failed` | Transcript Service | Gateway | `{user_id, bot_id, meeting_title, failure_sub_code}` |

**Kafka key strategy:**
- Topics produced by application services (`meeting.*`, `bot.status`, `transcript.ready/failed`) use `user_id` as Kafka key for per-user ordering
- Topics produced by Webhook Service (`bot.events`, `recording.events`, `transcript.events`) use `bot_id` as Kafka key because Recall webhooks do not contain `user_id`. Downstream consumers (Bot Service, Transcript Service) resolve `bot_id` → `user_id` via their own database.

**Consumer groups:** Each consuming service uses its own Kafka consumer group (e.g., `gateway-group`, `bot-service-group`, `transcript-service-group`). This ensures every service receives every message on topics with multiple consumers (e.g., `meeting.upcoming` goes to both Bot Service and Gateway).

---

## End-to-End Flow

1. User connects Google Calendar via OAuth (Gateway → Calendar Service stores tokens)
2. Calendar Service polls Google Calendar every 3 minutes per user
3. Calendar Service detects meeting with video link approaching start time → publishes `meeting.upcoming` to Kafka
4. Bot Service consumes `meeting.upcoming` → creates Recall bot → publishes `bot.status` (scheduled)
5. Recall bot joins meeting, records, transcribes
6. Recall sends webhooks to Webhook Service → publishes raw events to `bot.events`, `recording.events`, `transcript.events`
7. Bot Service consumes `bot.events` + `recording.events` → updates state → publishes `bot.status` changes
8. Transcript Service consumes `transcript.events` → on `transcript.done`, fetches transcript from Recall → publishes `transcript.ready`
9. Gateway consumes `bot.status` + `transcript.ready` → pushes via SSE to connected tray app
10. Tray app receives `transcript.ready` SSE event → runs LLM summarization locally → creates Obsidian note

---

## Infrastructure (Homelab Kubernetes)

All microservices deploy to the `liteagent` namespace following the existing Helm chart pattern.

**Existing infrastructure used:**
- Kafka: `kafka.liteagent.svc.cluster.local:9092` (KRaft, single broker)
- NGINX Ingress + MetalLB (IP pool `192.168.4.120-192.168.4.130`)
- cert-manager with `letsencrypt-prod` for TLS
- `local-path` StorageClass for PVCs
- OTel → Prometheus/Loki/Tempo → Grafana for observability

**New infrastructure:**
- 4 new Postgres instances (auth, calendar, bot, transcript) - each a small Helm chart with `postgres:16`, `local-path` PVC
- 6 new Go service Helm charts
- 1 new Ingress: `api.liteagent.net` → Gateway Service

**Each Helm chart follows the homelab pattern:**
```
helm-meeting-<service>/
├── Chart.yaml
├── values.yaml
└── templates/
    ├── deployment.yaml    # Recreate strategy, single replica
    ├── service.yaml       # ClusterIP
    ├── secret.yaml        # Env vars, credentials
    ├── pvc.yaml           # If has Postgres
    └── ingress.yaml       # Only for Gateway
```

---

## Microservice 1: Gateway Service

### Responsibility
REST API for the tray app. SSE push for real-time events. Routes commands to services via Kafka. Authenticates all requests by calling Auth Service.

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST /api/auth/register` | Proxy to Auth Service |
| `GET /api/auth/verify` | Proxy to Auth Service |
| `POST /api/auth/rotate-key` | Proxy to Auth Service |
| `GET /api/auth/calendar/connect` | Initiate Google OAuth flow (redirects to Calendar Service) |
| `GET /api/auth/calendar/callback` | OAuth callback (Calendar Service stores token) |
| `GET /api/events/stream` | SSE stream for authenticated user |
| `GET /api/meetings` | List user's upcoming meetings (from Calendar Service) |
| `GET /api/bots/:botId/transcript` | Fetch transcript (from Transcript Service) |
| `DELETE /api/bots/:botId` | Cancel bot (publishes command to Bot Service) |
| `POST /api/webhooks/recallai` | Proxy to Webhook Service (no auth required) |

### SSE Stream (`GET /api/events/stream`)
- Requires `Authorization: Bearer <key>` (via query param `?token=` for SSE compatibility)
- Pushes events to connected tray app:
  - `event: bot.status` → `{bot_id, bot_status, meeting_title}`
  - `event: transcript.ready` → `{bot_id, meeting_title, raw_transcript, readable_transcript}`
  - `event: transcript.failed` → `{bot_id, meeting_title, failure_sub_code}`
  - `event: meeting.upcoming` → `{calendar_event_id, title, start_time, meeting_url}`
- Each SSE event includes an `id` field (monotonically increasing per user, stored in Gateway memory)
- Gateway buffers the last 100 events per user in memory
- Heartbeat: sends `event: ping` every 30 seconds to keep connection alive
- On disconnect: tray app reconnects with `Last-Event-ID` header. Gateway replays any buffered events with ID > `Last-Event-ID` before resuming live stream. If `Last-Event-ID` is older than the buffer, Gateway sends all buffered events (best-effort catch-up).

### Kafka
- **Produces:** nothing directly (routes HTTP requests to services via internal HTTP or Kafka commands)
- **Consumes:** `bot.status`, `transcript.ready`, `transcript.failed`, `meeting.upcoming` — filters by `user_id`, pushes to matching SSE connection

### Rate Limiting
- `POST /api/auth/register`: 5 per hour per IP
- `GET /api/events/stream`: 1 concurrent connection per user
- All other endpoints: 60 per minute per user
- In-memory token bucket

### No database
Gateway is stateless. Holds SSE connections in memory (map of user_id → SSE writer).

---

## Microservice 2: Auth Service

### Responsibility
User registration, email verification, API key management.

### Internal API (called by Gateway via HTTP)

| Method | Path | Description |
|--------|------|-------------|
| `POST /internal/auth/register` | Register user, send verification email |
| `GET /internal/auth/verify` | Verify email token, return API key |
| `POST /internal/auth/rotate-key` | Rotate API key |
| `GET /internal/auth/validate` | Validate API key, return user_id (called on every Gateway request) |

### Database (PG-auth)

**`users` table:**
- `id` (serial, primary key)
- `email` (text, unique)
- `api_key_hash` (text, unique, indexed) - SHA-256 hex
- `verified` (boolean, default false)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

**`verification_tokens` table:**
- `id` (serial, primary key)
- `user_id` (integer, FK to users, ON DELETE CASCADE)
- `token_hash` (text, unique) - SHA-256 hex
- `expires_at` (timestamptz)
- `created_at` (timestamptz)

### Email Sending
- Uses SMTP via env vars: `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASS`, `SMTP_FROM`
- Go `net/smtp` for sending (no external dependency). Sends plain-text verification emails.
- For production, any SMTP-compatible service works (Gmail SMTP, SendGrid, AWS SES, self-hosted Postfix)

### Auth Flow
- Registration: `POST {email}` → generate verification token → store SHA-256 hash with 24h expiry → send email via SMTP with `GET /api/auth/verify?token=<raw>`
- Verification: hash token → lookup → generate 256-bit API key → store SHA-256 of key → redirect to static page showing key via URL fragment (`#key=...`)
- Key rotation: validate current key → generate new → hash and store → return new key
- Validation: Gateway calls `GET /internal/auth/validate` with `Authorization` header on every request → returns `{user_id}` or 401
- Background: goroutine cleans unverified users older than 7 days

---

## Microservice 3: Calendar Service

### Responsibility
Stores user Google OAuth tokens. Polls Google Calendar per user. Publishes `meeting.upcoming` and `meeting.cancelled` events.

### Internal API (called by Gateway)

| Method | Path | Description |
|--------|------|-------------|
| `GET /internal/calendar/auth-url` | Generate Google OAuth URL for user |
| `POST /internal/calendar/callback` | Exchange OAuth code for tokens, store |
| `DELETE /internal/calendar/disconnect` | Remove stored tokens for user |
| `GET /internal/calendar/meetings` | List upcoming meetings for user |
| `GET /internal/calendar/status` | Whether user has connected calendar |

### Database (PG-cal)

**`calendar_connections` table:**
- `id` (serial, primary key)
- `user_id` (integer, unique)
- `refresh_token` (text, encrypted at app level via AES-256-GCM with key from env var)
- `poll_interval_minutes` (integer, default 3)
- `bot_join_before_minutes` (integer, default 1)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

**`tracked_meetings` table:**
- `id` (serial, primary key)
- `user_id` (integer)
- `calendar_event_id` (text)
- `title` (text)
- `start_time` (timestamptz)
- `end_time` (timestamptz)
- `meeting_url` (text)
- `attendees` (jsonb)
- `bot_dispatched` (boolean, default false)
- `cancelled` (boolean, default false)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)
- UNIQUE constraint on (`user_id`, `calendar_event_id`)

### Polling Logic
- Background goroutine manages a polling loop per connected user
- Every `poll_interval_minutes`: fetch Google Calendar events for next 2 hours
- For each event with a video link (Meet, Zoom, Teams):
  - Upsert into `tracked_meetings` (update title/time/url/attendees if changed)
  - If within `bot_join_before_minutes` of start AND `bot_dispatched = false` → publish `meeting.upcoming` to Kafka, set `bot_dispatched = true`
- For events deleted from calendar since last poll: if `bot_dispatched = true` → publish `meeting.cancelled` to Kafka, set `cancelled = true`

### Google OAuth
- OAuth client ID + secret stored as env vars (bundled credentials for `calendar.readonly` scope)
- Redirect URI: `https://api.liteagent.net/api/auth/calendar/callback`
- Gateway proxies the OAuth flow; Calendar Service handles token exchange and storage

### URL Extraction
- Google Meet: `conferenceData.entryPoints[].uri`
- Zoom/Teams: regex on event description/location (`zoom.us/j/`, `teams.microsoft.com/l/meetup-join/`)

---

## Microservice 4: Bot Service

### Responsibility
Creates and manages Recall AI bots. Tracks bot lifecycle state.

### Kafka
- **Consumes:** `meeting.upcoming`, `meeting.cancelled`, `bot.events`, `recording.events`
- **Produces:** `bot.status`

### Internal API (called by Gateway)

| Method | Path | Description |
|--------|------|-------------|
| `DELETE /internal/bots/:botId` | Force-remove bot from meeting |
| `GET /internal/bots/:botId/status` | Get current bot state (returns `{user_id, bot_id, bot_status, meeting_title, meeting_url}`) |

### Database (PG-bot)

**`bots` table:**
- `id` (serial, primary key)
- `user_id` (integer)
- `bot_id` (text, unique - Recall bot ID)
- `calendar_event_id` (text)
- `meeting_url` (text)
- `meeting_title` (text)
- `bot_status` (text, default 'scheduled')
- `bot_webhook_updated_at` (timestamptz, nullable)
- `recording_id` (text, nullable)
- `recording_status` (text, nullable)
- `recording_webhook_updated_at` (timestamptz, nullable)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

### Event Handling

**On `meeting.upcoming`:**
- Create Recall bot via HTTP: `POST https://{RECALL_REGION}.recall.ai/api/v1/bot/`
- Transcript config: `recallai_streaming`, `prioritize_accuracy`, `auto` language, diarization enabled
- Retries on 507: every 30s, up to 10 attempts
- Respects 429 `Retry-After` with jitter
- On success: insert bot record, publish `bot.status` (scheduled)

**On `meeting.cancelled`:**
- Look up bot by `calendar_event_id` + `user_id`
- If bot exists and not in terminal state: call Recall delete bot API
- Publish `bot.status` (cancelled)

**On `bot.events`:**
- Compare `data.data.updated_at` against `bot_webhook_updated_at`; skip if stale
- Update `bot_status`
- Publish `bot.status` to Kafka (for Gateway SSE push)

**On `recording.events`:**
- Compare against `recording_webhook_updated_at`; skip if stale
- Update `recording_id`, `recording_status`

**Stale bot cleanup:** goroutine every 15 min marks bots stuck >3 hours in non-terminal states as `bot.fatal`

---

## Microservice 5: Webhook Service

### Responsibility
Receives Recall AI webhooks. Verifies HMAC signature. Publishes raw events to Kafka. Stateless.

### HTTP Endpoint

| Method | Path | Description |
|--------|------|-------------|
| `POST /webhooks/recallai` | Receive Recall webhooks |

Exposed internally via ClusterIP service. The Recall dashboard webhook URL points to the Gateway's public endpoint (`https://api.liteagent.net/api/webhooks/recallai`), which the Gateway proxies to the Webhook Service.

### Webhook Processing
1. Read raw request body
2. Verify HMAC-SHA256 signature using `RECALL_WORKSPACE_VERIFICATION_SECRET` (env var). Signature is in Svix webhook format. Return 401 on failure.
3. Parse event type from payload
4. Publish to appropriate Kafka topic:
   - `bot.*` events → `bot.events`
   - `recording.*` events → `recording.events`
   - `transcript.*` events → `transcript.events`
5. Return 200 immediately

### No database
Completely stateless. Just a webhook-to-Kafka bridge.

---

## Microservice 6: Transcript Service

### Responsibility
Consumes transcript completion events. Fetches full transcript from Recall. Converts to readable format. Publishes result.

### Kafka
- **Consumes:** `transcript.events`
- **Produces:** `transcript.ready`, `transcript.failed`

### Database (PG-transcript)

**`transcripts` table:**
- `id` (serial, primary key)
- `bot_id` (text, unique)
- `user_id` (integer)
- `meeting_title` (text)
- `transcript_id` (text - Recall transcript ID)
- `status` (text) - `done` or `failed`
- `failure_sub_code` (text, nullable)
- `raw_transcript` (jsonb, nullable)
- `readable_transcript` (jsonb, nullable)
- `created_at` (timestamptz)

### Internal API (called by Gateway)

| Method | Path | Description |
|--------|------|-------------|
| `GET /internal/transcripts/:botId` | Get stored transcript for a bot |

### Event Handling

**On `transcript.done` event:**
1. Extract `bot_id` and `transcript_id` from Recall webhook payload (Kafka key is `bot_id`)
2. Call Bot Service internal API: `GET /internal/bots/{bot_id}/status` to resolve `user_id` and `meeting_title`
3. Fetch transcript metadata from Recall: `GET /api/v1/transcript/{transcript_id}`
4. Download transcript from `data.download_url`
5. Convert raw transcript to readable format (speaker + paragraph + timestamps)
6. Store `bot_id`, `user_id`, `meeting_title`, and both transcript formats in `transcripts` table
7. Publish `transcript.ready` to Kafka (key: `user_id`) with `{user_id, bot_id, meeting_title, raw_transcript, readable_transcript}`

**On `transcript.failed` event:**
1. Extract `bot_id` from payload, resolve `user_id` and `meeting_title` via Bot Service
2. Store failure in `transcripts` table
3. Publish `transcript.failed` to Kafka (key: `user_id`) with `{user_id, bot_id, meeting_title, failure_sub_code}`

### Readable Transcript Conversion
Each raw entry has `words[]` and `participant`. Conversion joins words into paragraphs grouped by speaker, preserving timestamps.

---

## Electron Tray App (Thin Client)

### Stack
- Electron (tray icon only, no main window)
- TypeScript
- Vercel AI SDK (multi-provider LLM)
- Obsidian CLI
- Electron Store (local config, encrypted)
- EventSource (SSE client)

### No database
All state comes from the backend via SSE. Local config stored in Electron Store (JSON file encrypted via safeStorage).

### SSE Connection
- On startup, connects to `GET /api/events/stream?token=<api_key>`
- Handles events:
  - `meeting.upcoming` → update tray menu with upcoming meetings
  - `bot.status` → update tray icon state (joining → recording → done)
  - `transcript.ready` → enqueue for LLM processing
  - `transcript.failed` → show error notification
  - `ping` → keepalive (no action)
- On disconnect: reconnect with exponential backoff (1s, 2s, 4s, 8s, max 30s)

### Tray Icon States
- Grey: idle / connected
- Yellow: bot joining a meeting
- Red: bot recording
- Blue: processing transcript / LLM summarization
- Red with exclamation: error (SSE disconnected, LLM failed, Obsidian unavailable)

### Tray Menu
- Status line: "Connected" / "Recording: Design Review" / "Error: ..."
- "Upcoming Meetings" submenu (populated from SSE `meeting.upcoming` events)
- "Recent Notes" submenu (click opens in Obsidian; right-click "Re-summarize" for failed summaries)
- "Clear Error" (shown only in error state)
- "Settings..."
- "Quit"

### Notifications
- "Bot joined: Design Review"
- "Transcript ready, generating summary..."
- "Meeting note saved: Design Review" (click opens in Obsidian)
- "Error: ..." (on failures)

### Settings Panel (small window)
- Backend API Key
- Google Calendar: Connect/Disconnect (opens browser to `https://api.liteagent.net/api/auth/calendar/connect`)
- LLM Provider: Anthropic / OpenAI / OpenAI-compatible
- LLM API Key
- LLM Base URL (for OpenAI-compatible only)
- LLM Model (optional override)
- Obsidian vault (auto-detect active, or override)

### Local Config (Electron Store)
Keys stored locally, secrets encrypted via safeStorage:
- `backend_api_key`
- `llm_provider`
- `llm_api_key`
- `llm_base_url`
- `llm_model`
- `obsidian_vault`
- `recent_notes` (array of `{title, path, date}` for tray menu, max 20)

### LLM Integration (BYOK)

**Providers via Vercel AI SDK:**
- Anthropic (Claude) - default
- OpenAI (GPT-4)
- OpenAI-compatible (Ollama, local models, etc.)

**Prompt:** Given the readable transcript, generate:
- 2-3 paragraph summary
- Action items with assignees
- Key decisions made

**Long transcript handling:**
- If transcript exceeds 100,000 characters, chunk into 80,000-char segments with 5,000-char overlap
- Summarize each chunk, then consolidate in a final LLM pass
- If chunk still exceeds context window, halve chunk size and retry

### Obsidian Integration

**Note creation:** Write content to temp file, then `obsidian create path="Meetings/YYYY-MM-DD Title.md" content="$(cat /tmp/meeting-note-XXXX.md)"`. If `obsidian_vault` config is set, pass `vault=<name>`.

**Note naming:** `YYYY-MM-DD Meeting Title.md`. If exists, append sequence number: `(2)`, `(3)`, etc.

**Note format:**
```markdown
---
date: 2026-04-05
start: 2026-04-05T10:00:00-05:00
end: 2026-04-05T10:45:00-05:00
attendees:
  - Alice Smith
  - Bob Jones
tags:
  - meeting
---

## Summary

LLM-generated 2-3 paragraph summary.

## Action Items

- [ ] Alice to send updated designs by Friday
- [ ] Bob to review API spec

## Key Decisions

- Decided to use PostgreSQL instead of SQLite
- Agreed on 2-week sprint cycles

## Transcript

**Alice Smith** (0:00)
Hello everyone, let's get started.

**Bob Jones** (0:05)
Thanks Alice. I wanted to discuss the API changes...
```

### Processing Pipeline
Sequential queue (FIFO). When `transcript.ready` arrives via SSE:
1. Add to queue
2. Process one at a time: LLM summarize → create Obsidian note → add to `recent_notes`

**LLM failure:** retry once after 5s. If still failing, save transcript-only note, show notification. User can re-summarize from tray menu (transcript cached in Electron Store temporarily).

**Obsidian CLI failure:** queue note content in `pending_notes` dir in app userData. Retry periodically. Show notification.

### First-Run Setup
1. Enter backend API key (with "Verify" button)
2. Connect Google Calendar (opens browser)
3. Enter LLM API key + choose provider
4. Verify Obsidian CLI available
5. Done - SSE connects, app starts receiving events
