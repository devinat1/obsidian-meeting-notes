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
