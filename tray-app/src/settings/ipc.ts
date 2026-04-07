import { ipcMain, shell } from "electron";
import {
  getConfig,
  setConfigValue,
} from "../config/store";
import { isObsidianCLIAvailable } from "../obsidian/cli";
import type { LLMProvider } from "../types";

const GATEWAY_URL = "https://api.liteagent.net";

export function registerSettingsIPC(): void {
  ipcMain.handle("settings:get-config", () => {
    const config = getConfig();
    return {
      ...config,
      // Mask sensitive values for display
      backend_api_key: config.backend_api_key
        ? `${config.backend_api_key.slice(0, 8)}...`
        : null,
      llm_api_key: config.llm_api_key
        ? `${config.llm_api_key.slice(0, 8)}...`
        : null,
    };
  });

  ipcMain.handle(
    "settings:set-backend-api-key",
    (_event, key: string) => {
      setConfigValue("backend_api_key", key);
    }
  );

  ipcMain.handle(
    "settings:set-llm-config",
    (
      _event,
      config: {
        provider: LLMProvider;
        apiKey: string;
        baseUrl?: string | null;
        model?: string | null;
      }
    ) => {
      setConfigValue("llm_provider", config.provider);
      setConfigValue("llm_api_key", config.apiKey);
      setConfigValue("llm_base_url", config.baseUrl ?? null);
      setConfigValue("llm_model", config.model ?? null);
    }
  );

  ipcMain.handle(
    "settings:set-obsidian-vault",
    (_event, vault: string | null) => {
      setConfigValue("obsidian_vault", vault);
    }
  );

  ipcMain.handle("settings:verify-backend-key", async (_event, key: string) => {
    try {
      const response = await fetch(`${GATEWAY_URL}/api/auth/verify`, {
        headers: { Authorization: `Bearer ${key}` },
      });
      return { valid: response.ok };
    } catch {
      return { valid: false, error: "Could not connect to backend." };
    }
  });

  ipcMain.handle("settings:verify-obsidian-cli", async () => {
    const available = await isObsidianCLIAvailable();
    return { available };
  });

  ipcMain.handle("settings:connect-calendar", () => {
    const config = getConfig();
    if (config.backend_api_key === null) {
      return { error: "Backend API key not configured." };
    }
    const url = `${GATEWAY_URL}/api/auth/calendar/connect?token=${encodeURIComponent(config.backend_api_key)}`;
    shell.openExternal(url);
    return { opened: true };
  });
}
