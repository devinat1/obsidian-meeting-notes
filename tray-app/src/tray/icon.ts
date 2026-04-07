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
