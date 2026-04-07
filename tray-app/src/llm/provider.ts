import { createAnthropic } from "@ai-sdk/anthropic";
import { createOpenAI } from "@ai-sdk/openai";
import type { LanguageModelV1 } from "ai";
import type { LLMProvider } from "../types";

interface ProviderConfig {
  readonly provider: LLMProvider;
  readonly apiKey: string;
  readonly baseUrl?: string | null;
  readonly model?: string | null;
}

const DEFAULT_MODELS: Record<LLMProvider, string> = {
  anthropic: "claude-sonnet-4-20250514",
  openai: "gpt-4o",
  "openai-compatible": "gpt-4o",
};

export function createLanguageModel(config: ProviderConfig): LanguageModelV1 {
  const modelId = config.model ?? DEFAULT_MODELS[config.provider];

  switch (config.provider) {
    case "anthropic": {
      const anthropic = createAnthropic({ apiKey: config.apiKey });
      return anthropic(modelId);
    }
    case "openai": {
      const openai = createOpenAI({ apiKey: config.apiKey });
      return openai(modelId);
    }
    case "openai-compatible": {
      const openai = createOpenAI({
        apiKey: config.apiKey,
        baseURL: config.baseUrl ?? undefined,
      });
      return openai(modelId);
    }
  }
}
