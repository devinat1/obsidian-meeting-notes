import { app, Tray } from "electron";
import path from "path";
import { isConfigured, getConfig } from "./config/store";
import { SSEClient } from "./sse/client";
import type { SSEEvent, TrayState, MeetingUpcomingEvent } from "./types";

// Prevent multiple instances
const gotTheLock = app.requestSingleInstanceLock();
if (!gotTheLock) {
  app.quit();
}

// Hide dock icon on macOS (tray-only app)
if (process.platform === "darwin") {
  app.dock.hide();
}

const GATEWAY_URL = "https://api.liteagent.net";

let tray: Tray | null = null;
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

app.whenReady().then(() => {
  const iconPath = path.join(__dirname, "..", "assets", "tray-idle.png");
  tray = new Tray(iconPath);
  tray.setToolTip("Obsidian Meeting Notes");

  if (!isConfigured()) {
    // TODO: Show first-run setup wizard (Task 5.2)
    return;
  }

  startSSEConnection();
  // TODO: Build tray menu (Chunk 4)
  // TODO: Start pending notes retry loop (Task 3.4)
});

app.on("window-all-closed", () => {
  // Do not quit when all windows are closed - tray app stays running
});

app.on("before-quit", () => {
  if (sseClient !== null) {
    sseClient.dispose();
  }
});

export { tray };
