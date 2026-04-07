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
