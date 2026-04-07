import { EventSource } from "eventsource";
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
const SSE_OPEN_STATE = 1;
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

    const lastEventId = this.lastEventId;

    this.eventSource = new EventSource(url, {
      fetch: (fetchUrl, init) => {
        const mergedHeaders = new Headers(init.headers as Record<string, string>);
        if (lastEventId !== null) {
          mergedHeaders.set("Last-Event-ID", lastEventId);
        }
        return fetch(fetchUrl, { ...init, headers: mergedHeaders });
      },
    });

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
      this.eventSource.readyState === SSE_OPEN_STATE
    );
  }
}
