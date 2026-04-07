import { contextBridge, ipcRenderer } from "electron";

contextBridge.exposeInMainWorld("settingsAPI", {
  getConfig: () => ipcRenderer.invoke("settings:get-config"),
  setBackendApiKey: (key: string) =>
    ipcRenderer.invoke("settings:set-backend-api-key", key),
  setLLMConfig: (config: {
    provider: string;
    apiKey: string;
    baseUrl?: string | null;
    model?: string | null;
  }) => ipcRenderer.invoke("settings:set-llm-config", config),
  setObsidianVault: (vault: string | null) =>
    ipcRenderer.invoke("settings:set-obsidian-vault", vault),
  verifyBackendKey: (key: string) =>
    ipcRenderer.invoke("settings:verify-backend-key", key),
  verifyObsidianCLI: () => ipcRenderer.invoke("settings:verify-obsidian-cli"),
  connectCalendar: () => ipcRenderer.invoke("settings:connect-calendar"),
});
