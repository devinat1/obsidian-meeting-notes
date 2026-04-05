# Obsidian Meeting Notes - Design Spec

## Overview

An auto-recording meeting notes system that detects meetings from Google Calendar, sends a Recall AI bot to record and transcribe them, generates summaries via LLM, and saves structured notes to Obsidian.

Two components:
1. **Backend (Go)** - Hosted by us. Owns the Recall AI integration.
2. **Electron Tray App** - Self-hosted by user. Calendar polling, LLM summarization, Obsidian integration.

---

## Architecture

```
┌───────────────────────────────────────────┐
│         Electron Tray App (User)          │
│                                           │
│  ┌─────────────┐  ┌───────────────────┐  │
│  │ Google Cal   │  │ LLM Processor     │  │
│  │ Poller       │  │ (BYOK: Anthropic, │  │
│  │              │  │  OpenAI, custom)   │  │
│  └──────┬───────┘  └────────┬──────────┘  │
│         │                   │             │
│  ┌──────▼───────┐  ┌───────▼──────────┐  │
│  │ Bot Dispatch  │  │ Obsidian CLI     │  │
│  │ & Status Poll │  │ (create notes)   │  │
│  └──────┬───────┘  └──────────────────┘  │
│         │                                 │
│  ┌──────▼───────┐                        │
│  │ PostgreSQL   │                        │
│  │ (user-hosted)│                        │
│  └──────────────┘                        │
└─────────┬─────────────────────────────────┘
          │ HTTPS
┌─────────▼─────────────────────────────────┐
│          Backend (Go, hosted by us)        │
│                                            │
│  ┌──────────────┐  ┌───────────────────┐  │
│  │ REST API     │  │ Recall AI Client  │  │
│  │ (chi router) │  │ (bot create,      │  │
│  │              │  │  transcript fetch) │  │
│  └──────────────┘  └───────────────────┘  │
│                                            │
│  ┌──────────────┐  ┌───────────────────┐  │
│  │ Webhook      │  │ PostgreSQL        │  │
│  │ Handler      │  │ (our instance)    │  │
│  └──────────────┘  └───────────────────┘  │
└────────────────────────────────────────────┘
          │ Webhooks
┌─────────▼──────────┐
│     Recall AI      │
└────────────────────┘
```

---

## End-to-End Flow

1. Tray app polls Google Calendar every N minutes (default: 3)
2. Detects upcoming meetings with video conference links (Zoom, Meet, Teams)
3. When a meeting is within `bot_join_before_minutes` (default: 1) of starting, calls `POST /api/bots` on the backend
4. Backend creates Recall bot with transcript config, returns `bot_id`
5. Recall bot joins the meeting, records, and transcribes
6. Recall sends webhooks to backend (bot.*, recording.done/failed, transcript.done/failed)
7. Backend updates state in its Postgres
8. Tray app polls `GET /api/bots/:botId/status` every 30 seconds while a bot is active (stops on terminal status)
9. When transcript is done, tray app fetches it via `GET /api/bots/:botId/transcript`
10. Tray app sends transcript to LLM (user's own key) for summary + action items
11. Tray app creates structured markdown note via `obsidian create`

**Terminal statuses (stop polling when transcript OR bot reaches a final state):**
- `transcript.done` - success, proceed to fetch and process
- `transcript.failed` - transcript generation failed, show error
- `bot.fatal` - bot crashed, no transcript expected
- `recording.failed` - no recording, no transcript expected

Note: `bot.done` is NOT terminal for polling purposes. The bot can finish before the transcript is ready. Keep polling until one of the above statuses is reached.

---

## Component 1: Backend (Go)

### Stack
- Go with chi router
- PostgreSQL (our hosted instance)
- Recall AI via HTTP client

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST /api/auth/register` | Register new user, send verification email |
| `GET /api/auth/verify` | Email verification callback, returns API key |
| `POST /api/auth/rotate-key` | Rotate API key (requires current key) |
| `POST /api/bots` | Create Recall bot for a meeting URL |
| `GET /api/bots/:botId/status` | Bot/recording/transcript status |
| `GET /api/bots/:botId/transcript` | Fetch completed transcript (both raw + readable) |
| `DELETE /api/bots/:botId` | Remove bot from meeting (calls Recall to leave) |
| `POST /api/webhooks/recallai` | Receive Recall webhooks |

### Rate Limiting
- `POST /api/auth/register`: 5 requests per hour per IP
- `POST /api/bots`: 20 requests per hour per user
- `GET /api/bots/*`: 120 requests per minute per user (to support 30s polling of multiple bots)
- Rate limiting via in-memory token bucket, keyed on IP for unauthenticated routes and user ID for authenticated routes

### Auth

**Registration (`POST /api/auth/register`):**
- Input: `{ "email": "user@example.com" }`
- Generates a random verification token, stores its SHA-256 hash in a `verification_tokens` table with 24-hour expiry
- Sends email with link: `GET /api/auth/verify?token=<token>`
- Unverified users are cleaned up after 7 days by a background goroutine

**Email verification (`GET /api/auth/verify?token=<token>`):**
- Hashes the token with SHA-256, looks up in `verification_tokens`
- If valid and not expired: generates a random 256-bit API key
- Stores SHA-256 hash of the API key in the `users` table (SHA-256 is appropriate for high-entropy random keys; bcrypt is unnecessary here)
- Redirects to a static HTML page that displays the API key client-side via URL fragment (`#key=...`), so the key never appears in server logs or proxy caches
- User must copy and store the key; it cannot be retrieved again

**Key rotation (`POST /api/auth/rotate-key`):**
- Requires current valid API key in `Authorization: Bearer <key>`
- Generates new key, stores SHA-256 hash, invalidates old key
- Returns new key in response body

**Request auth:**
- All endpoints (except register, verify, and webhooks) require `Authorization: Bearer <key>`
- Backend computes SHA-256 of the provided key and looks up the user by hash (constant-time indexed lookup)
- All bot endpoints verify the authenticated user owns the requested bot (user_id FK check). Returns 404 if not owned.

### Database (Backend Postgres)

**`users` table:**
- `id` (serial, primary key)
- `email` (text, unique)
- `api_key_hash` (text, unique, indexed) - SHA-256 hex of API key
- `verified` (boolean, default false)
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

**`verification_tokens` table:**
- `id` (serial, primary key)
- `user_id` (integer, FK to users)
- `token_hash` (text, unique) - SHA-256 hex
- `expires_at` (timestamptz)
- `created_at` (timestamptz)

**`bots` table:**
- `id` (serial, primary key)
- `user_id` (integer, FK to users)
- `bot_id` (text, unique - Recall bot ID)
- `meeting_url` (text)
- `bot_status` (text, default 'scheduled')
- `bot_webhook_updated_at` (timestamptz, nullable) - last Recall webhook timestamp for bot events
- `recording_id` (text, nullable)
- `recording_status` (text, nullable)
- `recording_webhook_updated_at` (timestamptz, nullable) - last Recall webhook timestamp for recording events
- `transcript_id` (text, nullable)
- `transcript_status` (text, nullable)
- `transcript_failure_sub_code` (text, nullable)
- `transcript_webhook_updated_at` (timestamptz, nullable) - last Recall webhook timestamp for transcript events
- `created_at` (timestamptz)
- `updated_at` (timestamptz)

### Recall Integration

**Bot creation (`POST /api/bots`):**
- Input: `{ "meeting_url": "https://..." }`
- POST to `https://{RECALL_REGION}.recall.ai/api/v1/bot/`
- Includes transcript config: `recallai_streaming`, `prioritize_accuracy`, `auto` language, diarization enabled
- Retries on 507: every 30s, up to 10 attempts
- Respects 429 `Retry-After` header with small jitter
- On 400 (bad URL), 401 (auth), 500 (server error): returns the error directly to the tray app with appropriate HTTP status. No retry.
- On success: persists bot record with `bot_status='scheduled'`, returns `{ "bot_id": "..." }`

**Bot lifecycle:**
- Recall bots automatically leave when the meeting ends (host ends call or all participants leave)
- `DELETE /api/bots/:botId` allows early removal: backend calls Recall's delete bot endpoint to force the bot to leave
- Stale bot cleanup: a background goroutine runs every 15 minutes, marks bots stuck in non-terminal states for >3 hours as `bot.fatal` with sub_code `stale_timeout`

**Webhook handler (`POST /api/webhooks/recallai`):**
- Reads raw request body
- Verifies Recall HMAC-SHA256 signature: the `webhook-secret` from the Recall dashboard is used to compute HMAC-SHA256 over the raw request body. The computed signature is compared against the signature in the request headers (per Recall's Svix webhook format). See: https://docs.recall.ai/docs/authenticating-requests-from-recallai
- `RECALL_WORKSPACE_VERIFICATION_SECRET` is configured as a backend env var
- Returns 401 on signature verification failure
- The webhook URL is configured once globally in the Recall dashboard, not per-bot
- Updates only related fields per event type, using per-event-type timestamp columns:
  - `bot.*` → compare `data.data.updated_at` against `bot_webhook_updated_at`; skip if stored is newer; otherwise update `bot_status` and `bot_webhook_updated_at`
  - `recording.done/failed` → compare against `recording_webhook_updated_at`; update `recording_id`, `recording_status`, `recording_webhook_updated_at`
  - `transcript.done/failed` → compare against `transcript_webhook_updated_at`; update `transcript_id`, `transcript_status`, `transcript_failure_sub_code`, `transcript_webhook_updated_at`
- Also sets `updated_at` on the row for general bookkeeping
- Returns 200 immediately, does not block on downstream work
- Per-event-type timestamps ensure that stale cleanup or other operations on the shared `updated_at` column do not interfere with webhook idempotency

**Transcript retrieval (`GET /api/bots/:botId/transcript`):**
- Check stored `transcript_status`
- If pending/null: return `{"status": "pending"}`
- If failed: return `{"status": "failed", "sub_code": "..."}`
- If done: fetch transcript via Recall API (`GET /api/v1/transcript/{transcript_id}`), download from `data.download_url`, convert to readable format, return both raw + readable

**Status endpoint (`GET /api/bots/:botId/status`) response schema:**
```json
{
  "bot_id": "string",
  "bot_status": "string",
  "recording_status": "string | null",
  "transcript_status": "string | null",
  "transcript_failure_sub_code": "string | null",
  "updated_at": "ISO8601 string"
}
```

---

## Component 2: Electron Tray App

### Stack
- Electron (tray icon only, no main window)
- TypeScript
- Vercel AI SDK (multi-provider LLM)
- PostgreSQL (user-hosted). Postgres is used because the user explicitly requires self-hosting with a real database. Users can use Docker (`docker run postgres`), a managed instance (Supabase, Neon, RDS), or a local install.
- Google Calendar API
- Obsidian CLI

### Schema Migrations
- On startup, the tray app runs auto-migrations against the user's Postgres using a migrations table (`_migrations`) to track applied versions
- Migrations are bundled with the app and applied sequentially
- This ensures schema changes in future releases are applied automatically

### Tray Icon States
- Grey: idle, no meetings soon
- Yellow: meeting detected, bot joining
- Red: bot recording
- Blue: processing transcript/summary
- Red with exclamation overlay: error state (displayed when any step in the pipeline fails)

**Error badge behavior:** Shown when bot dispatch fails, LLM summarization fails, or Obsidian note creation fails. Clicking the tray icon shows the error in the menu status line. Clears automatically when the next operation succeeds, or user can dismiss via "Clear Error" menu item.

### Tray Menu
- Status line: "Next meeting: Standup in 12 min" or "Recording: Design Review" or "Error: LLM request failed"
- "Upcoming Meetings" submenu
- "Recent Notes" submenu (each note shows title; click opens in Obsidian via `obsidian open`; right-click shows "Re-summarize" option for notes with failed/missing summaries)
- "Clear Error" (shown only in error state)
- "Settings..."
- "Quit"

### Notifications
- "Bot joined: Design Review"
- "Transcript ready, generating summary..."
- "Meeting note saved: Design Review" (click opens in Obsidian)
- "Error: Failed to dispatch bot for Design Review" (on failure)

### Settings Panel (small window)
- Database URL (Postgres connection string)
- Google Calendar: Connect/Disconnect (OAuth flow)
- Google OAuth Client ID + Secret (optional, for BYOK OAuth credentials)
- LLM Provider: Anthropic / OpenAI / OpenAI-compatible
- LLM API Key
- LLM Base URL (for OpenAI-compatible only)
- LLM Model (optional override)
- Backend API Key (the key issued by our backend)
- Obsidian vault (auto-detect active, or override)
- Bot join timing (minutes before meeting start)
- Calendar poll interval (minutes)

### Database (User Postgres)

**`meetings` table:**
- `id` (serial, primary key)
- `calendar_event_id` (text, unique)
- `title` (text)
- `start_time` (timestamptz)
- `end_time` (timestamptz)
- `meeting_url` (text)
- `attendees` (jsonb)
- `status` (text, default 'upcoming') - one of: `upcoming`, `bot_dispatched`, `recording`, `processing`, `completed`, `failed`, `cancelled`
- `bot_id` (text, nullable - from backend)
- `bot_status` (text, nullable)
- `recording_status` (text, nullable)
- `transcript_status` (text, nullable)
- `obsidian_note_path` (text, nullable)
- `raw_transcript` (jsonb, nullable) - cached locally for re-summarize functionality
- `created_at` (timestamptz, default now())
- `updated_at` (timestamptz, default now())

**`config` table:**
- `key` (text, primary key)
- `value` (text, encrypted at app level via Electron safeStorage for secrets)

Config keys: `backend_api_key`, `google_calendar_refresh_token`, `google_oauth_client_id`, `google_oauth_client_secret`, `llm_provider`, `llm_api_key`, `llm_base_url`, `llm_model`, `calendar_poll_interval_minutes`, `bot_join_before_minutes`, `obsidian_vault`

**Note on safeStorage:** Electron's `safeStorage` is tied to the OS keychain and the specific app installation. If the user reinstalls the app or migrates the database to a different machine, encrypted config values become undecryptable. The app detects this on startup and prompts the user to re-enter credentials via the setup wizard.

### Google Calendar Integration

**OAuth setup:**
- We ship a Google Cloud OAuth client ID + secret bundled in the app (for Google Calendar readonly access). For desktop/installed apps, the client secret is not truly secret (Google documents this), but we bundle it for convenience.
- User can optionally provide their own OAuth client ID + secret in settings (for self-hosted scenarios where our client ID has quota limits)
- `calendar.readonly` scope only
- Redirect URI: `http://localhost:{random_port}/oauth/callback` - Electron spins up a temporary local HTTP server to receive the OAuth callback
- Stores refresh token in config (encrypted via safeStorage)
- On token refresh failure (expired/revoked): clears stored token, shows notification prompting user to reconnect, sets tray to error state

**Polling:**
- Every N minutes (configurable, default: 3), fetch events for next 2 hours via Google Calendar API
- Filter for events with video conference links
- For already-tracked events (matched by `calendar_event_id`): update `title`, `start_time`, `end_time`, `meeting_url`, `attendees` if changed
- For new events: insert into meetings table with status `upcoming`
- For events deleted from calendar since last poll: if `bot_id` is set and bot is not in a terminal state, call `DELETE /api/bots/:botId` on backend. Set meeting `status` to `cancelled`.
- When meeting is within `bot_join_before_minutes` of starting AND `bot_id` is null AND `status` is `upcoming`, dispatch bot via backend
- Overlapping meetings: dispatch bots to all meetings that qualify. No limit on concurrent bots.

**Bot dispatch deduplication:**
- Before calling `POST /api/bots`, check the local `meetings` table: if `bot_id` is already set for this `calendar_event_id`, skip dispatch
- After successful dispatch, immediately store the returned `bot_id` and set `status` to `bot_dispatched`
- This prevents duplicate dispatches across polling cycles

**URL extraction:**
- Google Meet: `conferenceData.entryPoints[].uri`
- Zoom/Teams: regex on event description/location for known URL patterns (`zoom.us/j/`, `teams.microsoft.com/l/meetup-join/`)

### LLM Integration (BYOK)

**Providers via Vercel AI SDK:**
- Anthropic (Claude) - default
- OpenAI (GPT-4)
- OpenAI-compatible (Ollama, local models, etc.)

**Config:**
- `llm_provider`: anthropic | openai | openai_compatible
- `llm_api_key`: user's key
- `llm_base_url`: for OpenAI-compatible endpoints
- `llm_model`: optional override

**Prompt:** Given the readable transcript, generate:
- 2-3 paragraph summary
- Action items with assignees
- Key decisions made

**Long transcript handling:**
- If the readable transcript exceeds 100,000 characters (~25K tokens), chunk it into overlapping segments of 80,000 characters with 5,000 character overlap
- Summarize each chunk independently
- Then send all chunk summaries to the LLM in a final pass to produce the consolidated summary, action items, and key decisions
- If a single chunk still fails (context window exceeded), halve the chunk size and retry

### Obsidian Integration

**Note creation:** The Obsidian CLI `create` command accepts `name`, `path`, and `content` parameters (verified from `obsidian help create`). For long transcripts that may exceed OS argument length limits (~256KB on macOS), write the note content to a temp file and use `content="$(cat /tmp/meeting-note-XXXX.md)"`. Delete the temp file after successful creation.

The Obsidian CLI is a first-party tool (https://obsidian.md/cli). It requires:
- Obsidian desktop app to be running
- CLI activated in Settings > General
- CLI registered in system PATH

The tray app verifies CLI availability at startup by running `obsidian version`. If unavailable, shows a setup prompt in the settings panel.

If the `obsidian_vault` config key is set, pass `vault=<name>` to all Obsidian CLI commands.

**Note naming:** `YYYY-MM-DD Meeting Title.md`. If a file with the same name already exists (e.g., recurring meetings), append a sequence number: `YYYY-MM-DD Meeting Title (2).md`. Check existence via `obsidian file path="Meetings/..."` before creating.

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

**Folder:** `Meetings/` in the active Obsidian vault (or configured vault)

### Processing Pipeline & Concurrency

When a bot reaches `transcript.done`, the tray app adds it to a sequential processing queue. Items are processed one at a time in FIFO order:
1. Fetch transcript from backend, cache `raw_transcript` locally in the meetings row
2. Send to LLM for summarization
3. Create Obsidian note
4. Set meeting `status` to `completed`

This prevents parallel LLM requests from hitting rate limits and avoids concurrent Obsidian CLI calls. If multiple meetings end near-simultaneously, they are queued and processed in order.

### Error Handling & Retries

**Bot dispatch failure (backend unreachable or returns error):**
- If backend is unreachable: retry 3 times with exponential backoff (2s, 4s, 8s)
- If backend returns 4xx (bad URL, auth failure): do not retry, show error notification with the backend's error message. Set meeting `status` to `failed`.
- If backend returns 5xx: retry up to 3 times with exponential backoff
- If all retries fail, set tray to error state, show notification, set meeting `status` to `failed`
- On next poll cycle, re-attempt if meeting is still upcoming and `status` is `failed` (reset to `upcoming`)

**LLM summarization failure:**
- Retry once after 5 seconds
- If still failing, save the note without the summary sections (transcript only) and show a notification: "Summary generation failed. Transcript saved."
- User can re-trigger summarization from the "Recent Notes" right-click menu. This uses the locally cached `raw_transcript`, so it works even if the backend has purged its records.

**Obsidian CLI failure:**
- If `obsidian create` fails (app not running, vault not found), queue the note content in a local `pending_notes` directory within the Electron app's userData path
- Retry on next poll cycle (check every poll interval whether Obsidian is available)
- Show notification: "Obsidian not available. Note queued."
- Queued notes are written once Obsidian becomes available

### First-Run Setup Wizard
1. PostgreSQL connection string (with "Test Connection" button). Auto-runs initial migration on success.
2. Backend API key (with "Verify" button that calls `GET /api/bots` to confirm auth)
3. Connect Google Calendar (OAuth flow in browser)
4. Choose LLM provider + enter API key (with "Test" button that sends a short prompt)
5. Verify Obsidian CLI is available (auto-detected, with troubleshooting link if missing)
6. Done - app starts polling
