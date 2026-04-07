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
