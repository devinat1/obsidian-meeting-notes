import { Menu, MenuItem } from "electron";
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
