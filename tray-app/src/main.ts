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

app.on("window-all-closed", () => {
  // Do not quit when all windows are closed - tray app stays running.
});

app.on("before-quit", () => {
  if (sseClient !== null) {
    sseClient.dispose();
  }
  stopPendingNotesRetry();
});
