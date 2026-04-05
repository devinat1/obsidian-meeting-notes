# Plan 4: Electron Tray App

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a tray-only Electron app that connects to the Gateway via SSE, receives real-time meeting events, runs BYOK LLM summarization on transcripts using the Vercel AI SDK, and creates structured notes in Obsidian via the Obsidian CLI.

**Architecture:** Thin client with no database. All meeting/bot/transcript state arrives via SSE from the Gateway. Local config (API keys, vault path, recent notes) persisted in Electron Store encrypted via safeStorage. LLM calls run locally using the user's own API key. Notes written to Obsidian via the `obsidian` CLI tool.

**Tech Stack:** Electron 33+, TypeScript, Vercel AI SDK, @ai-sdk/anthropic, @ai-sdk/openai, electron-store, EventSource

**Spec:** `docs/superpowers/specs/2026-04-05-obsidian-meeting-notes-design.md`

**Depends on:** Plan 1 (Gateway)

---

## File Structure

```
electron-tray-app/
├── package.json
├── tsconfig.json
├── electron-builder.yml
├── src/
│   ├── main.ts                      # Electron main process entry
│   ├── preload.ts                   # Preload script for settings window
│   ├── config/
│   │   └── store.ts                 # Electron Store schema + helpers
│   ├── sse/
│   │   └── client.ts                # SSE connection with reconnect logic
│   ├── llm/
│   │   ├── provider.ts              # Vercel AI SDK provider factory
│   │   ├── summarize.ts             # Transcript summarization + chunking
│   │   └── queue.ts                 # Sequential processing queue
│   ├── obsidian/
│   │   ├── cli.ts                   # Obsidian CLI wrapper (create, open, verify)
│   │   ├── note.ts                  # Note formatting (frontmatter, markdown)
│   │   └── pending.ts               # Pending notes dir for CLI failures
│   ├── tray/
│   │   ├── icon.ts                  # Tray icon state management
│   │   ├── menu.ts                  # Context menu builder
│   │   └── notifications.ts         # Native notification helpers
│   ├── settings/
│   │   ├── window.ts                # Settings BrowserWindow creation
│   │   ├── ipc.ts                   # IPC handlers for settings panel
│   │   └── index.html               # Settings panel HTML
│   ├── setup/
│   │   ├── wizard.ts                # First-run setup wizard
│   │   └── index.html               # Setup wizard HTML
│   └── types.ts                     # Shared TypeScript types
├── assets/
│   ├── tray-idle.png                # Grey tray icon
│   ├── tray-idle@2x.png
│   ├── tray-joining.png             # Yellow tray icon
│   ├── tray-joining@2x.png
│   ├── tray-recording.png           # Red tray icon
│   ├── tray-recording@2x.png
│   ├── tray-processing.png          # Blue tray icon
│   ├── tray-processing@2x.png
│   ├── tray-error.png               # Red with exclamation tray icon
│   └── tray-error@2x.png
└── scripts/
    └── generate-icons.ts            # Script to generate tray icons from SVG
```

---

## Chunk 1: Project Scaffolding + Electron Store

### Task 1.1: Initialize Electron project with TypeScript

**Files (Create):** `electron-tray-app/package.json`, `electron-tray-app/tsconfig.json`, `electron-tray-app/electron-builder.yml`

- [ ] Create `electron-tray-app/` directory at the repo root
- [ ] Initialize `package.json`:

```json
{
  "name": "obsidian-meeting-notes",
  "version": "0.1.0",
  "description": "Tray app for auto-recording meeting notes to Obsidian",
  "main": "dist/main.js",
  "scripts": {
    "dev": "tsc && electron .",
    "build": "tsc",
    "start": "electron .",
    "package": "electron-builder",
    "typecheck": "tsc --noEmit"
  },
  "author": "",
  "license": "MIT",
  "devDependencies": {
    "@types/node": "^22.0.0",
    "electron": "^33.0.0",
    "electron-builder": "^25.0.0",
    "typescript": "^5.6.0"
  },
  "dependencies": {
    "@ai-sdk/anthropic": "^1.0.0",
    "@ai-sdk/openai": "^1.0.0",
    "ai": "^4.0.0",
    "electron-store": "^10.0.0",
    "eventsource": "^3.0.0"
  }
}
```

- [ ] Create `tsconfig.json`:

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "module": "commonjs",
    "lib": ["ES2022"],
    "outDir": "dist",
    "rootDir": "src",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true,
    "resolveJsonModule": true,
    "declaration": true,
    "declarationMap": true,
    "sourceMap": true
  },
  "include": ["src/**/*"],
  "exclude": ["node_modules", "dist"]
}
```

- [ ] Create `electron-builder.yml`:

```yaml
appId: net.liteagent.meeting-notes
productName: Obsidian Meeting Notes
mac:
  category: public.app-category.productivity
  target: dmg
  icon: assets/icon.icns
linux:
  target: AppImage
  category: Utility
win:
  target: nsis
files:
  - dist/**/*
  - assets/**/*
  - "!src/**/*"
  - "!**/*.ts"
extraResources:
  - assets/*
```

- [ ] Run `cd electron-tray-app && npm install`

### Task 1.2: Define shared types

**Files (Create):** `electron-tray-app/src/types.ts`

- [ ] Create `src/types.ts` with all shared types:

```typescript
// SSE event types matching Gateway output
export type SSEEventType =
  | "meeting.upcoming"
  | "bot.status"
  | "transcript.ready"
  | "transcript.failed"
  | "ping";

export interface MeetingUpcomingEvent {
  readonly calendar_event_id: string;
  readonly title: string;
  readonly start_time: string;
  readonly meeting_url: string;
}

export interface BotStatusEvent {
  readonly bot_id: string;
  readonly bot_status: string;
  readonly meeting_title: string;
}

export interface TranscriptSegment {
  readonly speaker: string;
  readonly text: string;
  readonly start_time: number;
}

export interface TranscriptReadyEvent {
  readonly bot_id: string;
  readonly meeting_title: string;
  readonly raw_transcript: readonly TranscriptSegment[];
  readonly readable_transcript: string;
}

export interface TranscriptFailedEvent {
  readonly bot_id: string;
  readonly meeting_title: string;
  readonly failure_sub_code: string;
}

export type SSEEvent =
  | { readonly type: "meeting.upcoming"; readonly data: MeetingUpcomingEvent }
  | { readonly type: "bot.status"; readonly data: BotStatusEvent }
  | { readonly type: "transcript.ready"; readonly data: TranscriptReadyEvent }
  | { readonly type: "transcript.failed"; readonly data: TranscriptFailedEvent }
  | { readonly type: "ping"; readonly data: null };

// Tray icon states
export type TrayState =
  | "idle"
  | "joining"
  | "recording"
  | "processing"
  | "error";

// LLM provider types
export type LLMProvider = "anthropic" | "openai" | "openai-compatible";

// Electron Store schema
export interface StoreSchema {
  readonly backend_api_key: string | null;
  readonly llm_provider: LLMProvider;
  readonly llm_api_key: string | null;
  readonly llm_base_url: string | null;
  readonly llm_model: string | null;
  readonly obsidian_vault: string | null;
  readonly recent_notes: readonly RecentNote[];
}

export interface RecentNote {
  readonly title: string;
  readonly path: string;
  readonly date: string;
}

// Processing queue item
export interface QueueItem {
  readonly botId: string;
  readonly meetingTitle: string;
  readonly readableTranscript: string;
  readonly rawTranscript: readonly TranscriptSegment[];
  readonly startTime?: string | null;
  readonly endTime?: string | null;
  readonly attendees?: readonly string[];
}

// LLM summarization result
export interface SummaryResult {
  readonly summary: string;
  readonly actionItems: readonly string[];
  readonly keyDecisions: readonly string[];
}

// Obsidian note content
export interface NoteContent {
  readonly title: string;
  readonly date: string;
  readonly startTime?: string | null;
  readonly endTime?: string | null;
  readonly attendees: readonly string[];
  readonly summary: string;
  readonly actionItems: readonly string[];
  readonly keyDecisions: readonly string[];
  readonly transcript: string;
}
```

### Task 1.3: Implement Electron Store with encrypted secrets

**Files (Create):** `electron-tray-app/src/config/store.ts`

- [ ] Create `src/config/store.ts`:

```typescript
import Store from "electron-store";
import { safeStorage } from "electron";
import type { LLMProvider, RecentNote, StoreSchema } from "../types";

const RECENT_NOTES_MAX = 20;

const ENCRYPTED_KEYS = new Set([
  "backend_api_key",
  "llm_api_key",
]);

const store = new Store<{
  backend_api_key: string | null;
  llm_provider: LLMProvider;
  llm_api_key: string | null;
  llm_base_url: string | null;
  llm_model: string | null;
  obsidian_vault: string | null;
  recent_notes: readonly RecentNote[];
}>({
  name: "obsidian-meeting-notes-config",
  defaults: {
    backend_api_key: null,
    llm_provider: "anthropic",
    llm_api_key: null,
    llm_base_url: null,
    llm_model: null,
    obsidian_vault: null,
    recent_notes: [],
  },
});

function encryptValue(value: string): string {
  if (!safeStorage.isEncryptionAvailable()) {
    return value;
  }
  const encrypted = safeStorage.encryptString(value);
  return encrypted.toString("base64");
}

function decryptValue(value: string): string {
  if (!safeStorage.isEncryptionAvailable()) {
    return value;
  }
  const buffer = Buffer.from(value, "base64");
  return safeStorage.decryptString(buffer);
}

export function getConfig(): StoreSchema {
  const backendApiKeyRaw = store.get("backend_api_key");
  const llmApiKeyRaw = store.get("llm_api_key");

  return {
    backend_api_key: backendApiKeyRaw ? decryptValue(backendApiKeyRaw) : null,
    llm_provider: store.get("llm_provider"),
    llm_api_key: llmApiKeyRaw ? decryptValue(llmApiKeyRaw) : null,
    llm_base_url: store.get("llm_base_url"),
    llm_model: store.get("llm_model"),
    obsidian_vault: store.get("obsidian_vault"),
    recent_notes: store.get("recent_notes"),
  };
}

export function setConfigValue<K extends keyof StoreSchema>(
  key: K,
  value: StoreSchema[K]
): void {
  if (ENCRYPTED_KEYS.has(key) && typeof value === "string") {
    store.set(key, encryptValue(value));
    return;
  }
  store.set(key, value as Parameters<typeof store.set>[1]);
}

export function addRecentNote(note: RecentNote): void {
  const existing = [...store.get("recent_notes")];
  const filtered = existing.filter((n) => n.path !== note.path);
  const updated = [note, ...filtered].slice(0, RECENT_NOTES_MAX);
  store.set("recent_notes", updated);
}

export function clearConfig(): void {
  store.clear();
}

export function isConfigured(): boolean {
  const config = getConfig();
  return config.backend_api_key !== null && config.llm_api_key !== null;
}
```

### Task 1.4: Create Electron main process entry (tray-only, no window)

**Files (Create):** `electron-tray-app/src/main.ts`

- [ ] Create `src/main.ts` with initial scaffolding:

```typescript
import { app, Tray } from "electron";
import path from "path";
import { isConfigured, getConfig } from "./config/store";

// Prevent multiple instances
const gotTheLock = app.requestSingleInstanceLock();
if (!gotTheLock) {
  app.quit();
}

// Hide dock icon on macOS (tray-only app)
if (process.platform === "darwin") {
  app.dock.hide();
}

let tray: Tray | null = null;

app.whenReady().then(() => {
  const iconPath = path.join(__dirname, "..", "assets", "tray-idle.png");
  tray = new Tray(iconPath);
  tray.setToolTip("Obsidian Meeting Notes");

  if (!isConfigured()) {
    // TODO: Show first-run setup wizard (Task 5.2)
    return;
  }

  // TODO: Initialize SSE connection (Chunk 2)
  // TODO: Build tray menu (Chunk 4)
  // TODO: Start pending notes retry loop (Task 3.4)
});

app.on("window-all-closed", (event: Event) => {
  // Do not quit when all windows are closed - tray app stays running
  event.preventDefault();
});

export { tray };
```

- [ ] Run `cd electron-tray-app && npm run build` to verify compilation
- [ ] **Commit:** `feat(electron): scaffold Electron tray app with TypeScript, Store config, and types`

---

## Chunk 2: SSE Client

### Task 2.1: Implement SSE client with reconnection and Last-Event-ID

**Files (Create):** `electron-tray-app/src/sse/client.ts`

- [ ] Create `src/sse/client.ts`:

```typescript
import EventSource from "eventsource";
import type {
  SSEEvent,
  SSEEventType,
  MeetingUpcomingEvent,
  BotStatusEvent,
  TranscriptReadyEvent,
  TranscriptFailedEvent,
} from "../types";

const RECONNECT_BASE_MS = 1000;
const RECONNECT_MAX_MS = 30000;
const SSE_EVENT_TYPES: readonly SSEEventType[] = [
  "meeting.upcoming",
  "bot.status",
  "transcript.ready",
  "transcript.failed",
  "ping",
];

type SSEEventHandler = (event: SSEEvent) => void;
type SSEConnectionHandler = (connected: boolean) => void;

interface SSEClientOptions {
  readonly gatewayUrl: string;
  readonly apiKey: string;
  readonly onEvent: SSEEventHandler;
  readonly onConnectionChange: SSEConnectionHandler;
}

export class SSEClient {
  private eventSource: EventSource | null = null;
  private lastEventId: string | null = null;
  private reconnectAttempts = 0;
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private disposed = false;

  private readonly gatewayUrl: string;
  private readonly apiKey: string;
  private readonly onEvent: SSEEventHandler;
  private readonly onConnectionChange: SSEConnectionHandler;

  constructor(options: SSEClientOptions) {
    this.gatewayUrl = options.gatewayUrl;
    this.apiKey = options.apiKey;
    this.onEvent = options.onEvent;
    this.onConnectionChange = options.onConnectionChange;
  }

  connect(): void {
    if (this.disposed) {
      return;
    }

    this.disconnect();

    const url = `${this.gatewayUrl}/api/events/stream?token=${encodeURIComponent(this.apiKey)}`;

    const headers: Record<string, string> = {};
    if (this.lastEventId !== null) {
      headers["Last-Event-ID"] = this.lastEventId;
    }

    this.eventSource = new EventSource(url, { headers });

    this.eventSource.onopen = () => {
      this.reconnectAttempts = 0;
      this.onConnectionChange(true);
    };

    this.eventSource.onerror = () => {
      this.onConnectionChange(false);
      this.scheduleReconnect();
    };

    for (const eventType of SSE_EVENT_TYPES) {
      this.eventSource.addEventListener(eventType, (messageEvent: MessageEvent) => {
        const rawEvent = messageEvent as MessageEvent & { lastEventId?: string };
        if (rawEvent.lastEventId) {
          this.lastEventId = rawEvent.lastEventId;
        }

        if (eventType === "ping") {
          this.onEvent({ type: "ping", data: null });
          return;
        }

        try {
          const data = JSON.parse(messageEvent.data);
          this.dispatchEvent(eventType, data);
        } catch {
          console.error(`Failed to parse SSE event data for ${eventType}.`);
        }
      });
    }
  }

  private dispatchEvent(
    eventType: SSEEventType,
    data: unknown
  ): void {
    switch (eventType) {
      case "meeting.upcoming":
        this.onEvent({
          type: "meeting.upcoming",
          data: data as MeetingUpcomingEvent,
        });
        break;
      case "bot.status":
        this.onEvent({
          type: "bot.status",
          data: data as BotStatusEvent,
        });
        break;
      case "transcript.ready":
        this.onEvent({
          type: "transcript.ready",
          data: data as TranscriptReadyEvent,
        });
        break;
      case "transcript.failed":
        this.onEvent({
          type: "transcript.failed",
          data: data as TranscriptFailedEvent,
        });
        break;
      case "ping":
        break;
    }
  }

  private scheduleReconnect(): void {
    if (this.disposed) {
      return;
    }

    this.disconnect();

    const backoffMs = Math.min(
      RECONNECT_BASE_MS * Math.pow(2, this.reconnectAttempts),
      RECONNECT_MAX_MS
    );
    const jitterMs = Math.floor(Math.random() * backoffMs * 0.3);
    const delayMs = backoffMs + jitterMs;

    this.reconnectAttempts += 1;

    this.reconnectTimer = setTimeout(() => {
      this.connect();
    }, delayMs);
  }

  disconnect(): void {
    if (this.eventSource !== null) {
      this.eventSource.close();
      this.eventSource = null;
    }
    if (this.reconnectTimer !== null) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
  }

  dispose(): void {
    this.disposed = true;
    this.disconnect();
  }

  getLastEventId(): string | null {
    return this.lastEventId;
  }

  isConnected(): boolean {
    return (
      this.eventSource !== null &&
      this.eventSource.readyState === EventSource.OPEN
    );
  }
}
```

### Task 2.2: Wire SSE client into main process

**Files (Edit):** `electron-tray-app/src/main.ts`

- [ ] Import `SSEClient` and wire up event handling in `main.ts`. After the `isConfigured()` check, add:

```typescript
import { SSEClient } from "./sse/client";
import type { SSEEvent, TrayState, MeetingUpcomingEvent } from "./types";

const GATEWAY_URL = "https://api.liteagent.net";

let sseClient: SSEClient | null = null;
let currentTrayState: TrayState = "idle";
const upcomingMeetings: MeetingUpcomingEvent[] = [];

function handleSSEEvent(event: SSEEvent): void {
  switch (event.type) {
    case "meeting.upcoming": {
      const existing = upcomingMeetings.find(
        (m) => m.calendar_event_id === event.data.calendar_event_id
      );
      if (!existing) {
        upcomingMeetings.push(event.data);
      }
      // TODO: rebuild tray menu (Chunk 4)
      break;
    }
    case "bot.status": {
      const statusToTrayState: Record<string, TrayState> = {
        scheduled: "idle",
        joining: "joining",
        joined: "recording",
        recording: "recording",
        done: "idle",
        fatal: "error",
        cancelled: "idle",
      };
      const newState = statusToTrayState[event.data.bot_status] ?? "idle";
      // TODO: update tray icon (Chunk 4)
      // TODO: send notification (Chunk 4)
      currentTrayState = newState;
      break;
    }
    case "transcript.ready": {
      // TODO: enqueue for LLM processing (Chunk 3)
      break;
    }
    case "transcript.failed": {
      // TODO: show error notification (Chunk 4)
      currentTrayState = "error";
      break;
    }
    case "ping":
      break;
  }
}

function handleConnectionChange(connected: boolean): void {
  if (!connected) {
    currentTrayState = "error";
    // TODO: update tray icon (Chunk 4)
  } else {
    currentTrayState = "idle";
    // TODO: update tray icon (Chunk 4)
  }
}

function startSSEConnection(): void {
  const config = getConfig();
  if (config.backend_api_key === null) {
    return;
  }

  sseClient = new SSEClient({
    gatewayUrl: GATEWAY_URL,
    apiKey: config.backend_api_key,
    onEvent: handleSSEEvent,
    onConnectionChange: handleConnectionChange,
  });

  sseClient.connect();
}
```

- [ ] Call `startSSEConnection()` in the `app.whenReady()` callback after the config check
- [ ] Add cleanup in `app.on("before-quit")`:

```typescript
app.on("before-quit", () => {
  if (sseClient !== null) {
    sseClient.dispose();
  }
});
```

- [ ] Run `npm run typecheck`
- [ ] **Commit:** `feat(electron): add SSE client with exponential backoff reconnection and Last-Event-ID`

---

## Chunk 3: LLM Summarization

### Task 3.1: Implement LLM provider factory

**Files (Create):** `electron-tray-app/src/llm/provider.ts`

- [ ] Create `src/llm/provider.ts`:

```typescript
import { createAnthropic } from "@ai-sdk/anthropic";
import { createOpenAI } from "@ai-sdk/openai";
import type { LanguageModelV1 } from "ai";
import type { LLMProvider } from "../types";

interface ProviderConfig {
  readonly provider: LLMProvider;
  readonly apiKey: string;
  readonly baseUrl?: string | null;
  readonly model?: string | null;
}

const DEFAULT_MODELS: Record<LLMProvider, string> = {
  anthropic: "claude-sonnet-4-20250514",
  openai: "gpt-4o",
  "openai-compatible": "gpt-4o",
};

export function createLanguageModel(config: ProviderConfig): LanguageModelV1 {
  const modelId = config.model ?? DEFAULT_MODELS[config.provider];

  switch (config.provider) {
    case "anthropic": {
      const anthropic = createAnthropic({ apiKey: config.apiKey });
      return anthropic(modelId);
    }
    case "openai": {
      const openai = createOpenAI({ apiKey: config.apiKey });
      return openai(modelId);
    }
    case "openai-compatible": {
      const openai = createOpenAI({
        apiKey: config.apiKey,
        baseURL: config.baseUrl ?? undefined,
      });
      return openai(modelId);
    }
  }
}
```

### Task 3.2: Implement transcript summarization with chunking

**Files (Create):** `electron-tray-app/src/llm/summarize.ts`

- [ ] Create `src/llm/summarize.ts`:

```typescript
import { generateText } from "ai";
import type { LanguageModelV1 } from "ai";
import type { SummaryResult } from "../types";

const CHUNK_THRESHOLD = 100_000;
const CHUNK_SIZE = 80_000;
const CHUNK_OVERLAP = 5_000;
const LLM_RETRY_DELAY_MS = 5_000;

const SUMMARIZE_PROMPT = `You are a meeting notes assistant. Given the following meeting transcript, generate:

1. **Summary**: A 2-3 paragraph summary of the meeting covering the main topics discussed, key points raised, and overall outcome.

2. **Action Items**: A list of action items with assignees (if mentioned). Format each as a markdown checkbox: "- [ ] Person to do thing by deadline"

3. **Key Decisions**: A list of important decisions that were made during the meeting.

Respond in exactly this format (no extra text outside these sections):

## Summary

[2-3 paragraphs]

## Action Items

[list of "- [ ] ..." items, or "None" if no action items]

## Key Decisions

[list of "- ..." items, or "None" if no key decisions]`;

const CONSOLIDATE_PROMPT = `You are a meeting notes assistant. You have been given summaries of different segments of the same meeting transcript. Consolidate them into a single coherent summary.

Respond in exactly this format (no extra text outside these sections):

## Summary

[2-3 paragraphs covering the entire meeting]

## Action Items

[deduplicated list of "- [ ] ..." items, or "None" if no action items]

## Key Decisions

[deduplicated list of "- ..." items, or "None" if no key decisions]`;

function chunkTranscript(transcript: string, chunkSize: number): readonly string[] {
  const chunks: string[] = [];
  let offset = 0;

  while (offset < transcript.length) {
    const end = Math.min(offset + chunkSize, transcript.length);
    chunks.push(transcript.slice(offset, end));
    if (end >= transcript.length) {
      break;
    }
    offset = end - CHUNK_OVERLAP;
  }

  return chunks;
}

function parseSummaryResponse(text: string): SummaryResult {
  const summaryMatch = text.match(/## Summary\s*\n([\s\S]*?)(?=\n## Action Items)/);
  const actionItemsMatch = text.match(/## Action Items\s*\n([\s\S]*?)(?=\n## Key Decisions)/);
  const keyDecisionsMatch = text.match(/## Key Decisions\s*\n([\s\S]*?)$/);

  const summary = summaryMatch ? summaryMatch[1].trim() : text.trim();

  const actionItemsRaw = actionItemsMatch ? actionItemsMatch[1].trim() : "";
  const actionItems =
    actionItemsRaw === "None" || actionItemsRaw === ""
      ? []
      : actionItemsRaw
          .split("\n")
          .map((line) => line.trim())
          .filter((line) => line.startsWith("- [ ]"));

  const keyDecisionsRaw = keyDecisionsMatch ? keyDecisionsMatch[1].trim() : "";
  const keyDecisions =
    keyDecisionsRaw === "None" || keyDecisionsRaw === ""
      ? []
      : keyDecisionsRaw
          .split("\n")
          .map((line) => line.trim())
          .filter((line) => line.startsWith("- "));

  return { summary, actionItems, keyDecisions };
}

async function summarizeChunk(
  model: LanguageModelV1,
  transcript: string
): Promise<string> {
  const result = await generateText({
    model,
    system: SUMMARIZE_PROMPT,
    prompt: transcript,
  });
  return result.text;
}

async function consolidateChunkSummaries(
  model: LanguageModelV1,
  chunkSummaries: readonly string[]
): Promise<string> {
  const combined = chunkSummaries
    .map((s, i) => `--- Segment ${i + 1} ---\n${s}`)
    .join("\n\n");

  const result = await generateText({
    model,
    system: CONSOLIDATE_PROMPT,
    prompt: combined,
  });
  return result.text;
}

export async function summarizeTranscript(
  model: LanguageModelV1,
  transcript: string
): Promise<SummaryResult> {
  if (transcript.length <= CHUNK_THRESHOLD) {
    const response = await summarizeChunk(model, transcript);
    return parseSummaryResponse(response);
  }

  const chunks = chunkTranscript(transcript, CHUNK_SIZE);
  const chunkSummaries: string[] = [];

  for (const chunk of chunks) {
    const chunkSummary = await summarizeChunk(model, chunk);
    chunkSummaries.push(chunkSummary);
  }

  const consolidated = await consolidateChunkSummaries(model, chunkSummaries);
  return parseSummaryResponse(consolidated);
}

export async function summarizeWithRetry(
  model: LanguageModelV1,
  transcript: string
): Promise<SummaryResult | null> {
  try {
    return await summarizeTranscript(model, transcript);
  } catch (firstError) {
    console.error("LLM summarization failed, retrying in 5 seconds.", firstError);
  }

  await new Promise((resolve) => setTimeout(resolve, LLM_RETRY_DELAY_MS));

  try {
    return await summarizeTranscript(model, transcript);
  } catch (secondError) {
    console.error("LLM summarization failed on retry.", secondError);
    return null;
  }
}
```

### Task 3.3: Implement sequential processing queue

**Files (Create):** `electron-tray-app/src/llm/queue.ts`

- [ ] Create `src/llm/queue.ts`:

```typescript
import type { LanguageModelV1 } from "ai";
import type { QueueItem, SummaryResult } from "../types";
import { summarizeWithRetry } from "./summarize";

type OnProcessed = (
  item: QueueItem,
  result: SummaryResult | null
) => void;

export class ProcessingQueue {
  private readonly queue: QueueItem[] = [];
  private processing = false;
  private readonly model: LanguageModelV1;
  private readonly onProcessed: OnProcessed;

  constructor(model: LanguageModelV1, onProcessed: OnProcessed) {
    this.model = model;
    this.onProcessed = onProcessed;
  }

  enqueue(item: QueueItem): void {
    this.queue.push(item);
    if (!this.processing) {
      void this.processNext();
    }
  }

  private async processNext(): Promise<void> {
    if (this.queue.length === 0) {
      this.processing = false;
      return;
    }

    this.processing = true;
    const item = this.queue.shift() as QueueItem;

    const result = await summarizeWithRetry(this.model, item.readableTranscript);
    this.onProcessed(item, result);

    void this.processNext();
  }

  get pending(): number {
    return this.queue.length;
  }

  get isProcessing(): boolean {
    return this.processing;
  }
}
```

- [ ] Run `npm run typecheck`
- [ ] **Commit:** `feat(electron): add BYOK LLM summarization with Vercel AI SDK, chunking, and processing queue`

---

## Chunk 4: Obsidian Integration

### Task 4.1: Implement note formatting

**Files (Create):** `electron-tray-app/src/obsidian/note.ts`

- [ ] Create `src/obsidian/note.ts`:

```typescript
import type { NoteContent } from "../types";

export function formatNoteMarkdown(content: NoteContent): string {
  const frontmatter = buildFrontmatter(content);
  const body = buildBody(content);
  return `${frontmatter}\n${body}`;
}

function buildFrontmatter(content: NoteContent): string {
  const lines: string[] = ["---"];
  lines.push(`date: ${content.date}`);

  if (content.startTime) {
    lines.push(`start: ${content.startTime}`);
  }
  if (content.endTime) {
    lines.push(`end: ${content.endTime}`);
  }

  if (content.attendees.length > 0) {
    lines.push("attendees:");
    for (const attendee of content.attendees) {
      lines.push(`  - ${attendee}`);
    }
  }

  lines.push("tags:");
  lines.push("  - meeting");
  lines.push("---");

  return lines.join("\n");
}

function buildBody(content: NoteContent): string {
  const sections: string[] = [];

  sections.push("## Summary");
  sections.push("");
  sections.push(content.summary);

  sections.push("");
  sections.push("## Action Items");
  sections.push("");
  if (content.actionItems.length > 0) {
    for (const item of content.actionItems) {
      sections.push(item);
    }
  } else {
    sections.push("None");
  }

  sections.push("");
  sections.push("## Key Decisions");
  sections.push("");
  if (content.keyDecisions.length > 0) {
    for (const decision of content.keyDecisions) {
      sections.push(decision);
    }
  } else {
    sections.push("None");
  }

  sections.push("");
  sections.push("## Transcript");
  sections.push("");
  sections.push(content.transcript);

  return sections.join("\n");
}

export function generateNotePath(
  title: string,
  date: string
): string {
  const sanitizedTitle = title
    .replace(/[\\/:*?"<>|]/g, "")
    .replace(/\s+/g, " ")
    .trim();
  return `Meetings/${date} ${sanitizedTitle}.md`;
}

export function generateNotePathWithSequence(
  title: string,
  date: string,
  sequence: number
): string {
  const sanitizedTitle = title
    .replace(/[\\/:*?"<>|]/g, "")
    .replace(/\s+/g, " ")
    .trim();
  if (sequence <= 1) {
    return `Meetings/${date} ${sanitizedTitle}.md`;
  }
  return `Meetings/${date} ${sanitizedTitle} (${sequence}).md`;
}
```

### Task 4.2: Implement Obsidian CLI wrapper

**Files (Create):** `electron-tray-app/src/obsidian/cli.ts`

- [ ] Create `src/obsidian/cli.ts`:

```typescript
import { execFile } from "child_process";
import { promisify } from "util";
import * as fs from "fs/promises";
import * as os from "os";
import * as path from "path";

const execFileAsync = promisify(execFile);

const MAX_SEQUENCE_ATTEMPTS = 20;

export async function isObsidianCLIAvailable(): Promise<boolean> {
  try {
    await execFileAsync("obsidian", ["--help"]);
    return true;
  } catch {
    return false;
  }
}

export async function createNote(options: {
  readonly notePath: string;
  readonly content: string;
  readonly vault?: string | null;
}): Promise<void> {
  const tempDir = await fs.mkdtemp(path.join(os.tmpdir(), "meeting-note-"));
  const tempFile = path.join(tempDir, "note.md");

  try {
    await fs.writeFile(tempFile, options.content, "utf-8");

    const args = ["create", `path=${options.notePath}`];
    args.push(`content=$(cat ${tempFile})`);

    if (options.vault) {
      args.push(`vault=${options.vault}`);
    }

    await execFileAsync("obsidian", args, { timeout: 15_000 });
  } finally {
    await fs.rm(tempDir, { recursive: true, force: true }).catch(() => {
      // Best-effort cleanup of temp files.
    });
  }
}

export async function createNoteWithDedup(options: {
  readonly title: string;
  readonly date: string;
  readonly content: string;
  readonly vault?: string | null;
  readonly generatePath: (title: string, date: string, sequence: number) => string;
}): Promise<string> {
  for (let sequence = 1; sequence <= MAX_SEQUENCE_ATTEMPTS; sequence++) {
    const notePath = options.generatePath(options.title, options.date, sequence);
    try {
      await createNote({
        notePath,
        content: options.content,
        vault: options.vault,
      });
      return notePath;
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : String(error);
      const isDuplicate =
        errorMessage.includes("already exists") ||
        errorMessage.includes("EEXIST");
      if (!isDuplicate) {
        throw error;
      }
      // File exists, try next sequence number.
    }
  }

  throw new Error(
    `Could not create note after ${MAX_SEQUENCE_ATTEMPTS} sequence attempts.`
  );
}

export async function openNoteInObsidian(options: {
  readonly notePath: string;
  readonly vault?: string | null;
}): Promise<void> {
  const args = ["open", `path=${options.notePath}`];
  if (options.vault) {
    args.push(`vault=${options.vault}`);
  }
  await execFileAsync("obsidian", args, { timeout: 10_000 });
}
```

### Task 4.3: Implement pending notes directory for CLI failures

**Files (Create):** `electron-tray-app/src/obsidian/pending.ts`

- [ ] Create `src/obsidian/pending.ts`:

```typescript
import * as fs from "fs/promises";
import * as path from "path";
import { app } from "electron";
import { createNote, isObsidianCLIAvailable } from "./cli";

const PENDING_DIR_NAME = "pending_notes";
const RETRY_INTERVAL_MS = 60_000;

function getPendingDir(): string {
  return path.join(app.getPath("userData"), PENDING_DIR_NAME);
}

export async function savePendingNote(options: {
  readonly notePath: string;
  readonly content: string;
  readonly vault?: string | null;
}): Promise<void> {
  const pendingDir = getPendingDir();
  await fs.mkdir(pendingDir, { recursive: true });

  const filename = `${Date.now()}-${options.notePath.replace(/[/\\]/g, "_")}`;
  const metaPath = path.join(pendingDir, `${filename}.json`);
  const contentPath = path.join(pendingDir, `${filename}.md`);

  await fs.writeFile(contentPath, options.content, "utf-8");
  await fs.writeFile(
    metaPath,
    JSON.stringify({
      notePath: options.notePath,
      vault: options.vault ?? null,
      contentFile: `${filename}.md`,
      createdAt: new Date().toISOString(),
    }),
    "utf-8"
  );
}

export async function retryPendingNotes(): Promise<number> {
  const pendingDir = getPendingDir();

  try {
    await fs.access(pendingDir);
  } catch {
    return 0;
  }

  const files = await fs.readdir(pendingDir);
  const metaFiles = files.filter((f) => f.endsWith(".json"));

  if (metaFiles.length === 0) {
    return 0;
  }

  const cliAvailable = await isObsidianCLIAvailable();
  if (!cliAvailable) {
    return metaFiles.length;
  }

  let remaining = 0;

  for (const metaFile of metaFiles) {
    const metaPath = path.join(pendingDir, metaFile);
    try {
      const metaRaw = await fs.readFile(metaPath, "utf-8");
      const meta = JSON.parse(metaRaw) as {
        notePath: string;
        vault: string | null;
        contentFile: string;
      };

      const contentPath = path.join(pendingDir, meta.contentFile);
      const content = await fs.readFile(contentPath, "utf-8");

      await createNote({
        notePath: meta.notePath,
        content,
        vault: meta.vault,
      });

      await fs.rm(metaPath, { force: true });
      await fs.rm(contentPath, { force: true });
    } catch {
      remaining += 1;
    }
  }

  return remaining;
}

let retryTimer: ReturnType<typeof setInterval> | null = null;

export function startPendingNotesRetry(
  onRetryResult?: (remaining: number) => void
): void {
  if (retryTimer !== null) {
    return;
  }

  retryTimer = setInterval(async () => {
    const remaining = await retryPendingNotes();
    onRetryResult?.(remaining);
  }, RETRY_INTERVAL_MS);
}

export function stopPendingNotesRetry(): void {
  if (retryTimer !== null) {
    clearInterval(retryTimer);
    retryTimer = null;
  }
}
```

### Task 4.4: Integrate Obsidian note creation into processing pipeline

**Files (Create):** (integration logic added inline, no new file)

This step wires the `ProcessingQueue`'s `onProcessed` callback to create Obsidian notes. The integration is done in `main.ts` during Chunk 6, but the helper function is defined here.

- [ ] Add a helper in `src/obsidian/note.ts` that builds `NoteContent` from a `QueueItem` and `SummaryResult`:

```typescript
import type { NoteContent, QueueItem, SummaryResult } from "../types";

export function buildNoteContent(
  item: QueueItem,
  summaryResult: SummaryResult | null,
  readableTranscript: string
): NoteContent {
  const today = new Date().toISOString().slice(0, 10);
  return {
    title: item.meetingTitle,
    date: today,
    startTime: item.startTime ?? null,
    endTime: item.endTime ?? null,
    attendees: item.attendees ?? [],
    summary:
      summaryResult !== null
        ? summaryResult.summary
        : "(LLM summarization failed. Transcript-only note.)",
    actionItems: summaryResult?.actionItems ?? [],
    keyDecisions: summaryResult?.keyDecisions ?? [],
    transcript: readableTranscript,
  };
}
```

- [ ] Run `npm run typecheck`
- [ ] **Commit:** `feat(electron): add Obsidian CLI integration with note formatting, dedup, and pending notes retry`

---

## Chunk 5: Tray UI (Icon, Menu, Notifications, Settings)

### Task 5.1: Implement tray icon state management

**Files (Create):** `electron-tray-app/src/tray/icon.ts`

- [ ] Create `src/tray/icon.ts`:

```typescript
import { Tray, nativeImage } from "electron";
import path from "path";
import type { TrayState } from "../types";

const ICON_FILES: Record<TrayState, string> = {
  idle: "tray-idle.png",
  joining: "tray-joining.png",
  recording: "tray-recording.png",
  processing: "tray-processing.png",
  error: "tray-error.png",
};

function getAssetsDir(): string {
  return path.join(__dirname, "..", "assets");
}

export function setTrayIcon(tray: Tray, state: TrayState): void {
  const iconFile = ICON_FILES[state];
  const iconPath = path.join(getAssetsDir(), iconFile);
  const icon = nativeImage.createFromPath(iconPath);
  tray.setImage(icon);
}

export function setTrayTooltip(tray: Tray, text: string): void {
  tray.setToolTip(text);
}
```

### Task 5.2: Generate placeholder tray icon assets

**Files (Create):** `electron-tray-app/scripts/generate-icons.ts`

- [ ] Create `scripts/generate-icons.ts` that generates simple 22x22 PNG tray icons using Electron's `nativeImage`. For initial development, create minimal placeholder PNGs:

```typescript
// This script generates placeholder tray icons as 22x22 colored PNGs.
// Run with: npx ts-node scripts/generate-icons.ts
// For production, replace with proper designed icons.

import * as fs from "fs";
import * as path from "path";

// Minimal 22x22 PNG with a single solid color
// PNG structure: signature + IHDR + IDAT (uncompressed) + IEND
function createSolidPNG(r: number, g: number, b: number): Buffer {
  // For development, create a 1x1 PNG that Electron will scale
  const width = 16;
  const height = 16;

  // This is a placeholder. In production, use proper icon design tools.
  // For now, write empty files that Electron can load without crashing.
  const signature = Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]);

  // IHDR chunk
  const ihdrData = Buffer.alloc(13);
  ihdrData.writeUInt32BE(width, 0);
  ihdrData.writeUInt32BE(height, 4);
  ihdrData[8] = 8; // bit depth
  ihdrData[9] = 2; // color type (RGB)
  ihdrData[10] = 0; // compression
  ihdrData[11] = 0; // filter
  ihdrData[12] = 0; // interlace

  const ihdr = createChunk("IHDR", ihdrData);

  // IDAT chunk - raw pixel data with zlib wrapper
  const rawData = Buffer.alloc(height * (1 + width * 3));
  for (let y = 0; y < height; y++) {
    const offset = y * (1 + width * 3);
    rawData[offset] = 0; // filter: none
    for (let x = 0; x < width; x++) {
      const pixelOffset = offset + 1 + x * 3;
      rawData[pixelOffset] = r;
      rawData[pixelOffset + 1] = g;
      rawData[pixelOffset + 2] = b;
    }
  }

  // Minimal zlib: deflate with no compression
  const zlibHeader = Buffer.from([0x78, 0x01]);
  // Split into blocks of max 65535 bytes
  const blocks: Buffer[] = [];
  let remaining = rawData.length;
  let pos = 0;
  while (remaining > 0) {
    const blockSize = Math.min(remaining, 65535);
    const isFinal = remaining <= 65535;
    const blockHeader = Buffer.alloc(5);
    blockHeader[0] = isFinal ? 0x01 : 0x00;
    blockHeader.writeUInt16LE(blockSize, 1);
    blockHeader.writeUInt16LE(blockSize ^ 0xffff, 3);
    blocks.push(blockHeader);
    blocks.push(rawData.subarray(pos, pos + blockSize));
    pos += blockSize;
    remaining -= blockSize;
  }

  // Adler-32 checksum
  let s1 = 1;
  let s2 = 0;
  for (let i = 0; i < rawData.length; i++) {
    s1 = (s1 + rawData[i]) % 65521;
    s2 = (s2 + s1) % 65521;
  }
  const adler32 = Buffer.alloc(4);
  adler32.writeUInt32BE((s2 << 16) | s1, 0);

  const zlibData = Buffer.concat([zlibHeader, ...blocks, adler32]);
  const idat = createChunk("IDAT", zlibData);

  // IEND chunk
  const iend = createChunk("IEND", Buffer.alloc(0));

  return Buffer.concat([signature, ihdr, idat, iend]);
}

function createChunk(type: string, data: Buffer): Buffer {
  const length = Buffer.alloc(4);
  length.writeUInt32BE(data.length, 0);
  const typeBuffer = Buffer.from(type, "ascii");
  const crcInput = Buffer.concat([typeBuffer, data]);

  // CRC32
  let crc = 0xffffffff;
  for (let i = 0; i < crcInput.length; i++) {
    crc ^= crcInput[i];
    for (let j = 0; j < 8; j++) {
      crc = (crc >>> 1) ^ (crc & 1 ? 0xedb88320 : 0);
    }
  }
  crc ^= 0xffffffff;
  const crcBuffer = Buffer.alloc(4);
  crcBuffer.writeUInt32BE(crc >>> 0, 0);

  return Buffer.concat([length, typeBuffer, data, crcBuffer]);
}

const icons: Array<{ name: string; r: number; g: number; b: number }> = [
  { name: "tray-idle", r: 128, g: 128, b: 128 },
  { name: "tray-joining", r: 230, g: 190, b: 30 },
  { name: "tray-recording", r: 220, g: 50, b: 50 },
  { name: "tray-processing", r: 50, g: 120, b: 220 },
  { name: "tray-error", r: 200, g: 30, b: 30 },
];

const assetsDir = path.join(__dirname, "..", "assets");
fs.mkdirSync(assetsDir, { recursive: true });

for (const icon of icons) {
  const png = createSolidPNG(icon.r, icon.g, icon.b);
  fs.writeFileSync(path.join(assetsDir, `${icon.name}.png`), png);
  // 2x version for retina
  fs.writeFileSync(path.join(assetsDir, `${icon.name}@2x.png`), png);
  console.log(`Generated ${icon.name}.png`);
}
```

- [ ] Run `npx ts-node scripts/generate-icons.ts` to generate placeholder icons

### Task 5.3: Implement tray context menu builder

**Files (Create):** `electron-tray-app/src/tray/menu.ts`

- [ ] Create `src/tray/menu.ts`:

```typescript
import { Menu, MenuItem, shell } from "electron";
import type {
  TrayState,
  MeetingUpcomingEvent,
  RecentNote,
} from "../types";

interface MenuContext {
  readonly trayState: TrayState;
  readonly statusMessage: string;
  readonly upcomingMeetings: readonly MeetingUpcomingEvent[];
  readonly recentNotes: readonly RecentNote[];
  readonly onOpenSettings: () => void;
  readonly onClearError: () => void;
  readonly onOpenNote: (notePath: string) => void;
  readonly onResummarize: (notePath: string) => void;
  readonly onQuit: () => void;
}

const STATUS_LABELS: Record<TrayState, string> = {
  idle: "Connected",
  joining: "Bot joining meeting...",
  recording: "Recording",
  processing: "Processing transcript...",
  error: "Error",
};

export function buildTrayMenu(context: MenuContext): Menu {
  const menu = new Menu();

  // Status line
  const statusLabel =
    context.statusMessage !== ""
      ? context.statusMessage
      : STATUS_LABELS[context.trayState];
  menu.append(
    new MenuItem({ label: statusLabel, enabled: false })
  );

  menu.append(new MenuItem({ type: "separator" }));

  // Upcoming Meetings submenu
  if (context.upcomingMeetings.length > 0) {
    const upcomingSubmenu = new Menu();
    for (const meeting of context.upcomingMeetings) {
      const startDate = new Date(meeting.start_time);
      const timeStr = startDate.toLocaleTimeString([], {
        hour: "2-digit",
        minute: "2-digit",
      });
      upcomingSubmenu.append(
        new MenuItem({
          label: `${timeStr} - ${meeting.title}`,
          enabled: false,
        })
      );
    }
    menu.append(
      new MenuItem({
        label: "Upcoming Meetings",
        submenu: upcomingSubmenu,
      })
    );
  } else {
    menu.append(
      new MenuItem({
        label: "Upcoming Meetings",
        submenu: Menu.buildFromTemplate([
          { label: "No upcoming meetings", enabled: false },
        ]),
      })
    );
  }

  // Recent Notes submenu
  if (context.recentNotes.length > 0) {
    const recentSubmenu = new Menu();
    for (const note of context.recentNotes) {
      recentSubmenu.append(
        new MenuItem({
          label: `${note.date} ${note.title}`,
          click: () => context.onOpenNote(note.path),
        })
      );
    }
    menu.append(
      new MenuItem({
        label: "Recent Notes",
        submenu: recentSubmenu,
      })
    );
  } else {
    menu.append(
      new MenuItem({
        label: "Recent Notes",
        submenu: Menu.buildFromTemplate([
          { label: "No recent notes", enabled: false },
        ]),
      })
    );
  }

  menu.append(new MenuItem({ type: "separator" }));

  // Clear Error (only in error state)
  if (context.trayState === "error") {
    menu.append(
      new MenuItem({
        label: "Clear Error",
        click: context.onClearError,
      })
    );
  }

  // Settings
  menu.append(
    new MenuItem({
      label: "Settings...",
      click: context.onOpenSettings,
    })
  );

  menu.append(new MenuItem({ type: "separator" }));

  // Quit
  menu.append(
    new MenuItem({
      label: "Quit",
      click: context.onQuit,
    })
  );

  return menu;
}
```

### Task 5.4: Implement notification helpers

**Files (Create):** `electron-tray-app/src/tray/notifications.ts`

- [ ] Create `src/tray/notifications.ts`:

```typescript
import { Notification } from "electron";

interface NotificationOptions {
  readonly title: string;
  readonly body: string;
  readonly onClick?: () => void;
}

export function showNotification(options: NotificationOptions): void {
  if (!Notification.isSupported()) {
    return;
  }

  const notification = new Notification({
    title: options.title,
    body: options.body,
    silent: false,
  });

  if (options.onClick) {
    const handler = options.onClick;
    notification.on("click", () => handler());
  }

  notification.show();
}

export function notifyBotJoined(meetingTitle: string): void {
  showNotification({
    title: "Bot Joined",
    body: `Bot joined: ${meetingTitle}`,
  });
}

export function notifyTranscriptReady(meetingTitle: string): void {
  showNotification({
    title: "Transcript Ready",
    body: `Transcript ready, generating summary for ${meetingTitle}...`,
  });
}

export function notifyNoteSaved(
  meetingTitle: string,
  onOpen: () => void
): void {
  showNotification({
    title: "Note Saved",
    body: `Meeting note saved: ${meetingTitle}`,
    onClick: onOpen,
  });
}

export function notifyError(message: string): void {
  showNotification({
    title: "Error",
    body: `Error: ${message}`,
  });
}
```

- [ ] Run `npm run typecheck`
- [ ] **Commit:** `feat(electron): add tray icon states, context menu, and native notifications`

---

## Chunk 6: Settings Panel + Setup Wizard

### Task 6.1: Implement IPC handlers for settings

**Files (Create):** `electron-tray-app/src/settings/ipc.ts`

- [ ] Create `src/settings/ipc.ts`:

```typescript
import { ipcMain, shell } from "electron";
import {
  getConfig,
  setConfigValue,
} from "../config/store";
import { isObsidianCLIAvailable } from "../obsidian/cli";
import type { LLMProvider, StoreSchema } from "../types";

const GATEWAY_URL = "https://api.liteagent.net";

export function registerSettingsIPC(): void {
  ipcMain.handle("settings:get-config", () => {
    const config = getConfig();
    return {
      ...config,
      // Mask sensitive values for display
      backend_api_key: config.backend_api_key
        ? `${config.backend_api_key.slice(0, 8)}...`
        : null,
      llm_api_key: config.llm_api_key
        ? `${config.llm_api_key.slice(0, 8)}...`
        : null,
    };
  });

  ipcMain.handle(
    "settings:set-backend-api-key",
    (_event, key: string) => {
      setConfigValue("backend_api_key", key);
    }
  );

  ipcMain.handle(
    "settings:set-llm-config",
    (
      _event,
      config: {
        provider: LLMProvider;
        apiKey: string;
        baseUrl?: string | null;
        model?: string | null;
      }
    ) => {
      setConfigValue("llm_provider", config.provider);
      setConfigValue("llm_api_key", config.apiKey);
      setConfigValue("llm_base_url", config.baseUrl ?? null);
      setConfigValue("llm_model", config.model ?? null);
    }
  );

  ipcMain.handle(
    "settings:set-obsidian-vault",
    (_event, vault: string | null) => {
      setConfigValue("obsidian_vault", vault);
    }
  );

  ipcMain.handle("settings:verify-backend-key", async (_event, key: string) => {
    try {
      const response = await fetch(`${GATEWAY_URL}/api/auth/verify`, {
        headers: { Authorization: `Bearer ${key}` },
      });
      return { valid: response.ok };
    } catch {
      return { valid: false, error: "Could not connect to backend." };
    }
  });

  ipcMain.handle("settings:verify-obsidian-cli", async () => {
    const available = await isObsidianCLIAvailable();
    return { available };
  });

  ipcMain.handle("settings:connect-calendar", () => {
    const config = getConfig();
    if (config.backend_api_key === null) {
      return { error: "Backend API key not configured." };
    }
    const url = `${GATEWAY_URL}/api/auth/calendar/connect?token=${encodeURIComponent(config.backend_api_key)}`;
    shell.openExternal(url);
    return { opened: true };
  });
}
```

### Task 6.2: Implement settings panel window

**Files (Create):** `electron-tray-app/src/settings/window.ts`, `electron-tray-app/src/settings/index.html`

- [ ] Create `src/settings/window.ts`:

```typescript
import { BrowserWindow } from "electron";
import path from "path";

let settingsWindow: BrowserWindow | null = null;

export function openSettingsWindow(): void {
  if (settingsWindow !== null) {
    settingsWindow.focus();
    return;
  }

  settingsWindow = new BrowserWindow({
    width: 480,
    height: 560,
    resizable: false,
    minimizable: false,
    maximizable: false,
    fullscreenable: false,
    title: "Settings - Obsidian Meeting Notes",
    webPreferences: {
      preload: path.join(__dirname, "..", "preload.js"),
      contextIsolation: true,
      nodeIntegration: false,
    },
  });

  settingsWindow.loadFile(
    path.join(__dirname, "..", "src", "settings", "index.html")
  );

  settingsWindow.on("closed", () => {
    settingsWindow = null;
  });
}

export function closeSettingsWindow(): void {
  if (settingsWindow !== null) {
    settingsWindow.close();
    settingsWindow = null;
  }
}
```

- [ ] Create `src/settings/index.html`:

```html
<!DOCTYPE html>
<html>
  <head>
    <meta charset="UTF-8" />
    <meta http-equiv="Content-Security-Policy" content="default-src 'self'; script-src 'self'" />
    <title>Settings</title>
    <style>
      * { box-sizing: border-box; margin: 0; padding: 0; }
      body {
        font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
        padding: 24px;
        background: #1e1e1e;
        color: #e0e0e0;
        font-size: 13px;
      }
      .section { margin-bottom: 20px; }
      .section-title {
        font-size: 12px;
        font-weight: 600;
        text-transform: uppercase;
        letter-spacing: 0.05em;
        color: #888;
        margin-bottom: 8px;
      }
      label { display: block; margin-bottom: 4px; font-size: 13px; }
      input, select {
        width: 100%;
        padding: 6px 8px;
        border: 1px solid #444;
        border-radius: 4px;
        background: #2a2a2a;
        color: #e0e0e0;
        font-size: 13px;
        margin-bottom: 10px;
      }
      input:focus, select:focus { outline: none; border-color: #6b8afd; }
      button {
        padding: 6px 14px;
        border: 1px solid #444;
        border-radius: 4px;
        background: #333;
        color: #e0e0e0;
        cursor: pointer;
        font-size: 13px;
      }
      button:hover { background: #444; }
      button.primary { background: #4a6cf7; border-color: #4a6cf7; }
      button.primary:hover { background: #5b7bf8; }
      .row { display: flex; gap: 8px; align-items: center; }
      .status { font-size: 12px; color: #888; margin-left: 8px; }
      .status.ok { color: #4caf50; }
      .status.fail { color: #f44336; }
      .hidden { display: none; }
    </style>
  </head>
  <body>
    <div class="section">
      <div class="section-title">Backend</div>
      <label for="api-key">API Key</label>
      <div class="row">
        <input type="password" id="api-key" placeholder="Enter backend API key" />
        <button id="verify-key-btn">Verify</button>
      </div>
      <span id="key-status" class="status"></span>
    </div>

    <div class="section">
      <div class="section-title">Google Calendar</div>
      <div class="row">
        <button id="connect-calendar-btn" class="primary">Connect Google Calendar</button>
      </div>
    </div>

    <div class="section">
      <div class="section-title">LLM Configuration</div>
      <label for="llm-provider">Provider</label>
      <select id="llm-provider">
        <option value="anthropic">Anthropic (Claude)</option>
        <option value="openai">OpenAI (GPT-4)</option>
        <option value="openai-compatible">OpenAI-compatible</option>
      </select>
      <label for="llm-api-key">API Key</label>
      <input type="password" id="llm-api-key" placeholder="Enter LLM API key" />
      <div id="base-url-group" class="hidden">
        <label for="llm-base-url">Base URL</label>
        <input type="text" id="llm-base-url" placeholder="https://api.example.com/v1" />
      </div>
      <label for="llm-model">Model (optional override)</label>
      <input type="text" id="llm-model" placeholder="Leave blank for default" />
    </div>

    <div class="section">
      <div class="section-title">Obsidian</div>
      <label for="obsidian-vault">Vault name (leave blank for auto-detect)</label>
      <div class="row">
        <input type="text" id="obsidian-vault" placeholder="My Vault" />
        <button id="verify-cli-btn">Verify CLI</button>
      </div>
      <span id="cli-status" class="status"></span>
    </div>

    <div class="section" style="text-align: right;">
      <button id="save-btn" class="primary">Save</button>
    </div>

    <script>
      const { settingsAPI } = window;

      // Load existing config
      settingsAPI.getConfig().then((config) => {
        if (config.backend_api_key) {
          document.getElementById("api-key").value = config.backend_api_key;
        }
        document.getElementById("llm-provider").value = config.llm_provider || "anthropic";
        if (config.llm_api_key) {
          document.getElementById("llm-api-key").value = config.llm_api_key;
        }
        if (config.llm_base_url) {
          document.getElementById("llm-base-url").value = config.llm_base_url;
        }
        if (config.llm_model) {
          document.getElementById("llm-model").value = config.llm_model;
        }
        if (config.obsidian_vault) {
          document.getElementById("obsidian-vault").value = config.obsidian_vault;
        }
        toggleBaseUrl();
      });

      // Toggle base URL visibility
      function toggleBaseUrl() {
        const provider = document.getElementById("llm-provider").value;
        const group = document.getElementById("base-url-group");
        group.className = provider === "openai-compatible" ? "" : "hidden";
      }
      document.getElementById("llm-provider").addEventListener("change", toggleBaseUrl);

      // Verify backend key
      document.getElementById("verify-key-btn").addEventListener("click", async () => {
        const key = document.getElementById("api-key").value;
        const status = document.getElementById("key-status");
        status.textContent = "Verifying...";
        status.className = "status";
        const result = await settingsAPI.verifyBackendKey(key);
        status.textContent = result.valid ? "Valid" : (result.error || "Invalid");
        status.className = result.valid ? "status ok" : "status fail";
      });

      // Verify Obsidian CLI
      document.getElementById("verify-cli-btn").addEventListener("click", async () => {
        const status = document.getElementById("cli-status");
        status.textContent = "Checking...";
        status.className = "status";
        const result = await settingsAPI.verifyObsidianCLI();
        status.textContent = result.available ? "CLI available" : "CLI not found";
        status.className = result.available ? "status ok" : "status fail";
      });

      // Connect calendar
      document.getElementById("connect-calendar-btn").addEventListener("click", () => {
        settingsAPI.connectCalendar();
      });

      // Save
      document.getElementById("save-btn").addEventListener("click", async () => {
        const apiKey = document.getElementById("api-key").value;
        if (apiKey && !apiKey.includes("...")) {
          await settingsAPI.setBackendApiKey(apiKey);
        }

        const llmApiKey = document.getElementById("llm-api-key").value;
        if (llmApiKey && !llmApiKey.includes("...")) {
          await settingsAPI.setLLMConfig({
            provider: document.getElementById("llm-provider").value,
            apiKey: llmApiKey,
            baseUrl: document.getElementById("llm-base-url").value || null,
            model: document.getElementById("llm-model").value || null,
          });
        }

        const vault = document.getElementById("obsidian-vault").value || null;
        await settingsAPI.setObsidianVault(vault);

        window.close();
      });
    </script>
  </body>
</html>
```

### Task 6.3: Create preload script

**Files (Create):** `electron-tray-app/src/preload.ts`

- [ ] Create `src/preload.ts`:

```typescript
import { contextBridge, ipcRenderer } from "electron";

contextBridge.exposeInMainWorld("settingsAPI", {
  getConfig: () => ipcRenderer.invoke("settings:get-config"),
  setBackendApiKey: (key: string) =>
    ipcRenderer.invoke("settings:set-backend-api-key", key),
  setLLMConfig: (config: {
    provider: string;
    apiKey: string;
    baseUrl?: string | null;
    model?: string | null;
  }) => ipcRenderer.invoke("settings:set-llm-config", config),
  setObsidianVault: (vault: string | null) =>
    ipcRenderer.invoke("settings:set-obsidian-vault", vault),
  verifyBackendKey: (key: string) =>
    ipcRenderer.invoke("settings:verify-backend-key", key),
  verifyObsidianCLI: () => ipcRenderer.invoke("settings:verify-obsidian-cli"),
  connectCalendar: () => ipcRenderer.invoke("settings:connect-calendar"),
});
```

### Task 6.4: Implement first-run setup wizard

**Files (Create):** `electron-tray-app/src/setup/wizard.ts`, `electron-tray-app/src/setup/index.html`

- [ ] Create `src/setup/wizard.ts`:

```typescript
import { BrowserWindow } from "electron";
import path from "path";

let wizardWindow: BrowserWindow | null = null;

export function openSetupWizard(onComplete: () => void): void {
  if (wizardWindow !== null) {
    wizardWindow.focus();
    return;
  }

  wizardWindow = new BrowserWindow({
    width: 520,
    height: 600,
    resizable: false,
    minimizable: false,
    maximizable: false,
    fullscreenable: false,
    title: "Setup - Obsidian Meeting Notes",
    webPreferences: {
      preload: path.join(__dirname, "..", "preload.js"),
      contextIsolation: true,
      nodeIntegration: false,
    },
  });

  wizardWindow.loadFile(
    path.join(__dirname, "..", "src", "setup", "index.html")
  );

  wizardWindow.on("closed", () => {
    wizardWindow = null;
    onComplete();
  });
}

export function closeSetupWizard(): void {
  if (wizardWindow !== null) {
    wizardWindow.close();
    wizardWindow = null;
  }
}
```

- [ ] Create `src/setup/index.html`:

```html
<!DOCTYPE html>
<html>
  <head>
    <meta charset="UTF-8" />
    <meta http-equiv="Content-Security-Policy" content="default-src 'self'; script-src 'self'" />
    <title>Setup</title>
    <style>
      * { box-sizing: border-box; margin: 0; padding: 0; }
      body {
        font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
        padding: 32px;
        background: #1e1e1e;
        color: #e0e0e0;
        font-size: 14px;
      }
      .title {
        font-size: 20px;
        font-weight: 600;
        margin-bottom: 8px;
      }
      .subtitle {
        font-size: 13px;
        color: #888;
        margin-bottom: 24px;
      }
      .step { display: none; }
      .step.active { display: block; }
      .step-title {
        font-size: 16px;
        font-weight: 600;
        margin-bottom: 4px;
      }
      .step-desc {
        font-size: 13px;
        color: #999;
        margin-bottom: 16px;
      }
      label { display: block; margin-bottom: 4px; font-size: 13px; }
      input, select {
        width: 100%;
        padding: 8px 10px;
        border: 1px solid #444;
        border-radius: 4px;
        background: #2a2a2a;
        color: #e0e0e0;
        font-size: 13px;
        margin-bottom: 12px;
      }
      input:focus, select:focus { outline: none; border-color: #6b8afd; }
      button {
        padding: 8px 18px;
        border: 1px solid #444;
        border-radius: 4px;
        background: #333;
        color: #e0e0e0;
        cursor: pointer;
        font-size: 13px;
      }
      button:hover { background: #444; }
      button.primary { background: #4a6cf7; border-color: #4a6cf7; }
      button.primary:hover { background: #5b7bf8; }
      .nav { display: flex; justify-content: space-between; margin-top: 24px; }
      .status { font-size: 12px; margin-top: 4px; margin-bottom: 8px; }
      .status.ok { color: #4caf50; }
      .status.fail { color: #f44336; }
      .steps-indicator {
        display: flex;
        gap: 8px;
        margin-bottom: 24px;
      }
      .step-dot {
        width: 8px;
        height: 8px;
        border-radius: 50%;
        background: #444;
      }
      .step-dot.active { background: #4a6cf7; }
      .step-dot.done { background: #4caf50; }
      .hidden { display: none; }
    </style>
  </head>
  <body>
    <div class="title">Obsidian Meeting Notes</div>
    <div class="subtitle">Let's get you set up in a few quick steps.</div>

    <div class="steps-indicator">
      <div class="step-dot active" id="dot-0"></div>
      <div class="step-dot" id="dot-1"></div>
      <div class="step-dot" id="dot-2"></div>
      <div class="step-dot" id="dot-3"></div>
    </div>

    <!-- Step 1: Backend API Key -->
    <div class="step active" id="step-0">
      <div class="step-title">1. Backend API Key</div>
      <div class="step-desc">Enter the API key from your registration email.</div>
      <label for="setup-api-key">API Key</label>
      <input type="password" id="setup-api-key" placeholder="Enter your API key" />
      <button id="setup-verify-key">Verify</button>
      <div id="setup-key-status" class="status"></div>
    </div>

    <!-- Step 2: Google Calendar -->
    <div class="step" id="step-1">
      <div class="step-title">2. Google Calendar</div>
      <div class="step-desc">Connect your Google Calendar so the bot knows when to join meetings.</div>
      <button id="setup-connect-cal" class="primary">Connect Google Calendar</button>
      <div class="step-desc" style="margin-top: 12px;">A browser window will open. Come back here after connecting.</div>
    </div>

    <!-- Step 3: LLM Config -->
    <div class="step" id="step-2">
      <div class="step-title">3. LLM Provider</div>
      <div class="step-desc">Choose an LLM provider and enter your API key for summarization.</div>
      <label for="setup-llm-provider">Provider</label>
      <select id="setup-llm-provider">
        <option value="anthropic">Anthropic (Claude) - Recommended</option>
        <option value="openai">OpenAI (GPT-4)</option>
        <option value="openai-compatible">OpenAI-compatible</option>
      </select>
      <label for="setup-llm-key">API Key</label>
      <input type="password" id="setup-llm-key" placeholder="Enter LLM API key" />
      <div id="setup-base-url-group" class="hidden">
        <label for="setup-llm-base-url">Base URL</label>
        <input type="text" id="setup-llm-base-url" placeholder="https://api.example.com/v1" />
      </div>
    </div>

    <!-- Step 4: Obsidian -->
    <div class="step" id="step-3">
      <div class="step-title">4. Obsidian</div>
      <div class="step-desc">Verify the Obsidian CLI is available and optionally set a vault.</div>
      <button id="setup-verify-cli">Verify Obsidian CLI</button>
      <div id="setup-cli-status" class="status"></div>
      <label for="setup-vault" style="margin-top: 12px;">Vault name (optional)</label>
      <input type="text" id="setup-vault" placeholder="Leave blank for auto-detect" />
    </div>

    <div class="nav">
      <button id="prev-btn" class="hidden">Back</button>
      <button id="next-btn" class="primary">Next</button>
    </div>

    <script>
      const { settingsAPI } = window;
      let currentStep = 0;
      const totalSteps = 4;

      function showStep(step) {
        for (let i = 0; i < totalSteps; i++) {
          document.getElementById(`step-${i}`).className = i === step ? "step active" : "step";
          const dot = document.getElementById(`dot-${i}`);
          dot.className = i < step ? "step-dot done" : (i === step ? "step-dot active" : "step-dot");
        }
        document.getElementById("prev-btn").className = step > 0 ? "" : "hidden";
        document.getElementById("next-btn").textContent = step === totalSteps - 1 ? "Done" : "Next";
        currentStep = step;
      }

      document.getElementById("next-btn").addEventListener("click", async () => {
        if (currentStep === 0) {
          // Save backend API key
          const key = document.getElementById("setup-api-key").value;
          if (key) {
            await settingsAPI.setBackendApiKey(key);
          }
        } else if (currentStep === 2) {
          // Save LLM config
          const apiKey = document.getElementById("setup-llm-key").value;
          if (apiKey) {
            await settingsAPI.setLLMConfig({
              provider: document.getElementById("setup-llm-provider").value,
              apiKey,
              baseUrl: document.getElementById("setup-llm-base-url").value || null,
              model: null,
            });
          }
        } else if (currentStep === 3) {
          // Save vault and finish
          const vault = document.getElementById("setup-vault").value || null;
          await settingsAPI.setObsidianVault(vault);
          window.close();
          return;
        }

        if (currentStep < totalSteps - 1) {
          showStep(currentStep + 1);
        }
      });

      document.getElementById("prev-btn").addEventListener("click", () => {
        if (currentStep > 0) {
          showStep(currentStep - 1);
        }
      });

      // Step 1: Verify key
      document.getElementById("setup-verify-key").addEventListener("click", async () => {
        const key = document.getElementById("setup-api-key").value;
        const status = document.getElementById("setup-key-status");
        status.textContent = "Verifying...";
        status.className = "status";
        const result = await settingsAPI.verifyBackendKey(key);
        status.textContent = result.valid ? "Valid key" : (result.error || "Invalid key");
        status.className = result.valid ? "status ok" : "status fail";
      });

      // Step 2: Connect calendar
      document.getElementById("setup-connect-cal").addEventListener("click", () => {
        settingsAPI.connectCalendar();
      });

      // Step 3: Toggle base URL
      document.getElementById("setup-llm-provider").addEventListener("change", () => {
        const provider = document.getElementById("setup-llm-provider").value;
        document.getElementById("setup-base-url-group").className =
          provider === "openai-compatible" ? "" : "hidden";
      });

      // Step 4: Verify CLI
      document.getElementById("setup-verify-cli").addEventListener("click", async () => {
        const status = document.getElementById("setup-cli-status");
        status.textContent = "Checking...";
        status.className = "status";
        const result = await settingsAPI.verifyObsidianCLI();
        status.textContent = result.available ? "Obsidian CLI available" : "Obsidian CLI not found";
        status.className = result.available ? "status ok" : "status fail";
      });
    </script>
  </body>
</html>
```

- [ ] Run `npm run typecheck`
- [ ] **Commit:** `feat(electron): add settings panel, first-run setup wizard, and IPC handlers`

---

## Chunk 7: Full Integration + Error Handling

### Task 7.1: Wire all components together in main.ts

**Files (Edit):** `electron-tray-app/src/main.ts`

- [ ] Rewrite `src/main.ts` with full integration of all components:

```typescript
import { app, Tray } from "electron";
import path from "path";
import {
  isConfigured,
  getConfig,
  addRecentNote,
} from "./config/store";
import { SSEClient } from "./sse/client";
import { createLanguageModel } from "./llm/provider";
import { ProcessingQueue } from "./llm/queue";
import {
  formatNoteMarkdown,
  generateNotePathWithSequence,
  buildNoteContent,
} from "./obsidian/note";
import {
  createNoteWithDedup,
  openNoteInObsidian,
} from "./obsidian/cli";
import {
  savePendingNote,
  startPendingNotesRetry,
  stopPendingNotesRetry,
} from "./obsidian/pending";
import { setTrayIcon, setTrayTooltip } from "./tray/icon";
import { buildTrayMenu } from "./tray/menu";
import {
  notifyBotJoined,
  notifyTranscriptReady,
  notifyNoteSaved,
  notifyError,
} from "./tray/notifications";
import { registerSettingsIPC } from "./settings/ipc";
import { openSettingsWindow } from "./settings/window";
import { openSetupWizard } from "./setup/wizard";
import type {
  SSEEvent,
  TrayState,
  MeetingUpcomingEvent,
  QueueItem,
  SummaryResult,
} from "./types";

const GATEWAY_URL = "https://api.liteagent.net";

// Prevent multiple instances
const gotTheLock = app.requestSingleInstanceLock();
if (!gotTheLock) {
  app.quit();
}

// Hide dock icon on macOS (tray-only app)
if (process.platform === "darwin") {
  app.dock.hide();
}

let tray: Tray | null = null;
let sseClient: SSEClient | null = null;
let processingQueue: ProcessingQueue | null = null;
let currentTrayState: TrayState = "idle";
let currentStatusMessage = "";
const upcomingMeetings: MeetingUpcomingEvent[] = [];

function updateTray(): void {
  if (tray === null) {
    return;
  }

  setTrayIcon(tray, currentTrayState);
  setTrayTooltip(tray, currentStatusMessage !== "" ? currentStatusMessage : "Obsidian Meeting Notes");

  const config = getConfig();
  const menu = buildTrayMenu({
    trayState: currentTrayState,
    statusMessage: currentStatusMessage,
    upcomingMeetings,
    recentNotes: config.recent_notes,
    onOpenSettings: () => openSettingsWindow(),
    onClearError: () => {
      currentTrayState = "idle";
      currentStatusMessage = "";
      updateTray();
    },
    onOpenNote: (notePath: string) => {
      void openNoteInObsidian({ notePath, vault: config.obsidian_vault });
    },
    onResummarize: (_notePath: string) => {
      // Re-summarization requires the cached transcript, handled via Electron Store in future iteration.
    },
    onQuit: () => app.quit(),
  });

  tray.setContextMenu(menu);
}

function handleSSEEvent(event: SSEEvent): void {
  switch (event.type) {
    case "meeting.upcoming": {
      const existing = upcomingMeetings.find(
        (m) => m.calendar_event_id === event.data.calendar_event_id
      );
      if (!existing) {
        upcomingMeetings.push(event.data);
      }
      updateTray();
      break;
    }
    case "bot.status": {
      const statusToTrayState: Record<string, TrayState> = {
        scheduled: "idle",
        joining: "joining",
        joined: "recording",
        recording: "recording",
        done: "idle",
        fatal: "error",
        cancelled: "idle",
      };
      const newState = statusToTrayState[event.data.bot_status] ?? "idle";

      if (event.data.bot_status === "joined" || event.data.bot_status === "joining") {
        notifyBotJoined(event.data.meeting_title);
        currentStatusMessage = `Recording: ${event.data.meeting_title}`;
      } else if (event.data.bot_status === "done") {
        currentStatusMessage = "";
      } else if (event.data.bot_status === "fatal") {
        currentStatusMessage = `Error: Bot failed for ${event.data.meeting_title}`;
        notifyError(`Bot failed for ${event.data.meeting_title}`);
      }

      currentTrayState = newState;
      updateTray();
      break;
    }
    case "transcript.ready": {
      notifyTranscriptReady(event.data.meeting_title);
      currentTrayState = "processing";
      currentStatusMessage = `Processing: ${event.data.meeting_title}`;
      updateTray();

      const queueItem: QueueItem = {
        botId: event.data.bot_id,
        meetingTitle: event.data.meeting_title,
        readableTranscript: event.data.readable_transcript,
        rawTranscript: event.data.raw_transcript,
      };

      if (processingQueue !== null) {
        processingQueue.enqueue(queueItem);
      }
      break;
    }
    case "transcript.failed": {
      currentTrayState = "error";
      currentStatusMessage = `Transcript failed: ${event.data.meeting_title}`;
      notifyError(`Transcript failed for ${event.data.meeting_title} (${event.data.failure_sub_code})`);
      updateTray();
      break;
    }
    case "ping":
      break;
  }
}

function handleConnectionChange(connected: boolean): void {
  if (!connected) {
    currentTrayState = "error";
    currentStatusMessage = "Disconnected from server";
  } else {
    currentTrayState = "idle";
    currentStatusMessage = "";
  }
  updateTray();
}

async function handleProcessed(
  item: QueueItem,
  summaryResult: SummaryResult | null
): Promise<void> {
  const config = getConfig();
  const today = new Date().toISOString().slice(0, 10);

  const noteContent = buildNoteContent(
    item,
    summaryResult,
    item.readableTranscript
  );
  const markdown = formatNoteMarkdown(noteContent);

  try {
    const notePath = await createNoteWithDedup({
      title: item.meetingTitle,
      date: today,
      content: markdown,
      vault: config.obsidian_vault,
      generatePath: generateNotePathWithSequence,
    });

    addRecentNote({
      title: item.meetingTitle,
      path: notePath,
      date: today,
    });

    notifyNoteSaved(item.meetingTitle, () => {
      void openNoteInObsidian({ notePath, vault: config.obsidian_vault });
    });

    if (summaryResult === null) {
      notifyError(`LLM summarization failed for ${item.meetingTitle}. Transcript-only note saved.`);
    }
  } catch {
    notifyError(`Could not create Obsidian note for ${item.meetingTitle}. Queued for retry.`);

    await savePendingNote({
      notePath: generateNotePathWithSequence(item.meetingTitle, today, 1),
      content: markdown,
      vault: config.obsidian_vault,
    });
  }

  currentTrayState = "idle";
  currentStatusMessage = "";
  updateTray();
}

function startApp(): void {
  const config = getConfig();
  if (config.backend_api_key === null || config.llm_api_key === null) {
    return;
  }

  // Initialize LLM model
  const model = createLanguageModel({
    provider: config.llm_provider,
    apiKey: config.llm_api_key,
    baseUrl: config.llm_base_url,
    model: config.llm_model,
  });

  // Initialize processing queue
  processingQueue = new ProcessingQueue(model, (item, result) => {
    void handleProcessed(item, result);
  });

  // Start SSE connection
  sseClient = new SSEClient({
    gatewayUrl: GATEWAY_URL,
    apiKey: config.backend_api_key,
    onEvent: handleSSEEvent,
    onConnectionChange: handleConnectionChange,
  });
  sseClient.connect();

  // Start pending notes retry
  startPendingNotesRetry((remaining) => {
    if (remaining > 0) {
      console.log(`${remaining} pending notes waiting for Obsidian CLI.`);
    }
  });

  updateTray();
}

app.whenReady().then(() => {
  // Register IPC handlers for settings
  registerSettingsIPC();

  // Create tray
  const iconPath = path.join(__dirname, "..", "assets", "tray-idle.png");
  tray = new Tray(iconPath);
  tray.setToolTip("Obsidian Meeting Notes");

  if (!isConfigured()) {
    openSetupWizard(() => {
      if (isConfigured()) {
        startApp();
      }
    });
    return;
  }

  startApp();
});

app.on("window-all-closed", (event: Event) => {
  event.preventDefault();
});

app.on("before-quit", () => {
  if (sseClient !== null) {
    sseClient.dispose();
  }
  stopPendingNotesRetry();
});
```

### Task 7.2: Verify full build

- [ ] Run `cd electron-tray-app && npm run build`
- [ ] Run `npm run typecheck`
- [ ] Manually test with `npm run dev` (will show setup wizard if not configured)
- [ ] **Commit:** `feat(electron): wire all components together with full event processing pipeline`

### Task 7.3: Add error boundary and graceful degradation

**Files (Edit):** `electron-tray-app/src/main.ts`, `electron-tray-app/src/sse/client.ts`

- [ ] In `src/main.ts`, add a top-level uncaught exception handler:

```typescript
process.on("uncaughtException", (error) => {
  console.error("Uncaught exception in main process.", error);
  notifyError("Unexpected error occurred. Check logs for details.");
  currentTrayState = "error";
  currentStatusMessage = "Unexpected error";
  updateTray();
});

process.on("unhandledRejection", (reason) => {
  console.error("Unhandled promise rejection.", reason);
});
```

- [ ] In `src/sse/client.ts`, add a max reconnect attempt limit with a user-visible notification callback:

Add an optional `onMaxRetriesExhausted` callback to `SSEClientOptions`:

```typescript
interface SSEClientOptions {
  readonly gatewayUrl: string;
  readonly apiKey: string;
  readonly onEvent: SSEEventHandler;
  readonly onConnectionChange: SSEConnectionHandler;
  readonly onMaxRetriesExhausted?: () => void;
}
```

In `scheduleReconnect`, after computing delay, add:

```typescript
const MAX_RECONNECT_ATTEMPTS = 50;
if (this.reconnectAttempts > MAX_RECONNECT_ATTEMPTS && this.onMaxRetriesExhausted) {
  this.onMaxRetriesExhausted();
  return;
}
```

- [ ] Run `npm run typecheck`
- [ ] **Commit:** `feat(electron): add error boundaries and graceful degradation`

### Task 7.4: Final cleanup and packaging config

- [ ] Verify all imports resolve correctly: `npm run build`
- [ ] Add a `"postinstall"` script in `package.json` for Electron rebuild: `"postinstall": "electron-builder install-app-deps"`
- [ ] Test packaging: `npm run package` (generates platform-specific build)
- [ ] **Commit:** `chore(electron): final build config and packaging setup`
