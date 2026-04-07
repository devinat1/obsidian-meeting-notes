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

app.on("window-all-closed", () => {
  // Do not quit when all windows are closed - tray app stays running
});

export { tray };
