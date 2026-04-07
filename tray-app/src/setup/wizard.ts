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
