import Store from "electron-store";
import { safeStorage } from "electron";
import type { LLMProvider, RecentNote, StoreSchema } from "../types";

const RECENT_NOTES_MAX = 20;

const ENCRYPTED_KEYS = new Set([
  "backend_api_key",
  "llm_api_key",
]);

const store = new Store<{
  backend_api_key: string | null;
  llm_provider: LLMProvider;
  llm_api_key: string | null;
  llm_base_url: string | null;
  llm_model: string | null;
  obsidian_vault: string | null;
  recent_notes: readonly RecentNote[];
}>({
  name: "obsidian-meeting-notes-config",
  defaults: {
    backend_api_key: null,
    llm_provider: "anthropic",
    llm_api_key: null,
    llm_base_url: null,
    llm_model: null,
    obsidian_vault: null,
    recent_notes: [],
  },
});

function encryptValue(value: string): string {
  if (!safeStorage.isEncryptionAvailable()) {
    return value;
  }
  const encrypted = safeStorage.encryptString(value);
  return encrypted.toString("base64");
}

function decryptValue(value: string): string {
  if (!safeStorage.isEncryptionAvailable()) {
    return value;
  }
  const buffer = Buffer.from(value, "base64");
  return safeStorage.decryptString(buffer);
}

export function getConfig(): StoreSchema {
  const backendApiKeyRaw = store.get("backend_api_key");
  const llmApiKeyRaw = store.get("llm_api_key");

  return {
    backend_api_key: backendApiKeyRaw ? decryptValue(backendApiKeyRaw) : null,
    llm_provider: store.get("llm_provider"),
    llm_api_key: llmApiKeyRaw ? decryptValue(llmApiKeyRaw) : null,
    llm_base_url: store.get("llm_base_url"),
    llm_model: store.get("llm_model"),
    obsidian_vault: store.get("obsidian_vault"),
    recent_notes: store.get("recent_notes"),
  };
}

export function setConfigValue<K extends keyof StoreSchema>(
  key: K,
  value: StoreSchema[K]
): void {
  if (ENCRYPTED_KEYS.has(key) && typeof value === "string") {
    store.set(key, encryptValue(value));
    return;
  }
  store.set(key as string, value as unknown);
}

export function addRecentNote(note: RecentNote): void {
  const existing = [...store.get("recent_notes")];
  const filtered = existing.filter((n) => n.path !== note.path);
  const updated = [note, ...filtered].slice(0, RECENT_NOTES_MAX);
  store.set("recent_notes", updated);
}

export function clearConfig(): void {
  store.clear();
}

export function isConfigured(): boolean {
  const config = getConfig();
  return config.backend_api_key !== null && config.llm_api_key !== null;
}
