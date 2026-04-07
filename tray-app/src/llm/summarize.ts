import { generateText } from "ai";
import type { LanguageModelV1 } from "ai";
import type { SummaryResult } from "../types";

const CHUNK_THRESHOLD = 100_000;
const CHUNK_SIZE = 80_000;
const CHUNK_OVERLAP = 5_000;
const LLM_RETRY_DELAY_MS = 5_000;

const SUMMARIZE_PROMPT = `You are a meeting notes assistant. Given the following meeting transcript, generate:

1. **Summary**: A 2-3 paragraph summary of the meeting covering the main topics discussed, key points raised, and overall outcome.

2. **Action Items**: A list of action items with assignees (if mentioned). Format each as a markdown checkbox: "- [ ] Person to do thing by deadline"

3. **Key Decisions**: A list of important decisions that were made during the meeting.

Respond in exactly this format (no extra text outside these sections):

## Summary

[2-3 paragraphs]

## Action Items

[list of "- [ ] ..." items, or "None" if no action items]

## Key Decisions

[list of "- ..." items, or "None" if no key decisions]`;

const CONSOLIDATE_PROMPT = `You are a meeting notes assistant. You have been given summaries of different segments of the same meeting transcript. Consolidate them into a single coherent summary.

Respond in exactly this format (no extra text outside these sections):

## Summary

[2-3 paragraphs covering the entire meeting]

## Action Items

[deduplicated list of "- [ ] ..." items, or "None" if no action items]

## Key Decisions

[deduplicated list of "- ..." items, or "None" if no key decisions]`;

function chunkTranscript(transcript: string, chunkSize: number): readonly string[] {
  const chunks: string[] = [];
  let offset = 0;

  while (offset < transcript.length) {
    const end = Math.min(offset + chunkSize, transcript.length);
    chunks.push(transcript.slice(offset, end));
    if (end >= transcript.length) {
      break;
    }
    offset = end - CHUNK_OVERLAP;
  }

  return chunks;
}

function parseSummaryResponse(text: string): SummaryResult {
  const summaryMatch = text.match(/## Summary\s*\n([\s\S]*?)(?=\n## Action Items)/);
  const actionItemsMatch = text.match(/## Action Items\s*\n([\s\S]*?)(?=\n## Key Decisions)/);
  const keyDecisionsMatch = text.match(/## Key Decisions\s*\n([\s\S]*?)$/);

  const summary = summaryMatch ? summaryMatch[1].trim() : text.trim();

  const actionItemsRaw = actionItemsMatch ? actionItemsMatch[1].trim() : "";
  const actionItems =
    actionItemsRaw === "None" || actionItemsRaw === ""
      ? []
      : actionItemsRaw
          .split("\n")
          .map((line) => line.trim())
          .filter((line) => line.startsWith("- [ ]"));

  const keyDecisionsRaw = keyDecisionsMatch ? keyDecisionsMatch[1].trim() : "";
  const keyDecisions =
    keyDecisionsRaw === "None" || keyDecisionsRaw === ""
      ? []
      : keyDecisionsRaw
          .split("\n")
          .map((line) => line.trim())
          .filter((line) => line.startsWith("- "));

  return { summary, actionItems, keyDecisions };
}

async function summarizeChunk(
  model: LanguageModelV1,
  transcript: string
): Promise<string> {
  const result = await generateText({
    model,
    system: SUMMARIZE_PROMPT,
    prompt: transcript,
  });
  return result.text;
}

async function consolidateChunkSummaries(
  model: LanguageModelV1,
  chunkSummaries: readonly string[]
): Promise<string> {
  const combined = chunkSummaries
    .map((s, i) => `--- Segment ${i + 1} ---\n${s}`)
    .join("\n\n");

  const result = await generateText({
    model,
    system: CONSOLIDATE_PROMPT,
    prompt: combined,
  });
  return result.text;
}

export async function summarizeTranscript(
  model: LanguageModelV1,
  transcript: string
): Promise<SummaryResult> {
  if (transcript.length <= CHUNK_THRESHOLD) {
    const response = await summarizeChunk(model, transcript);
    return parseSummaryResponse(response);
  }

  const chunks = chunkTranscript(transcript, CHUNK_SIZE);
  const chunkSummaries: string[] = [];

  for (const chunk of chunks) {
    const chunkSummary = await summarizeChunk(model, chunk);
    chunkSummaries.push(chunkSummary);
  }

  const consolidated = await consolidateChunkSummaries(model, chunkSummaries);
  return parseSummaryResponse(consolidated);
}

export async function summarizeWithRetry(
  model: LanguageModelV1,
  transcript: string
): Promise<SummaryResult | null> {
  try {
    return await summarizeTranscript(model, transcript);
  } catch (firstError) {
    console.error("LLM summarization failed, retrying in 5 seconds.", firstError);
  }

  await new Promise((resolve) => setTimeout(resolve, LLM_RETRY_DELAY_MS));

  try {
    return await summarizeTranscript(model, transcript);
  } catch (secondError) {
    console.error("LLM summarization failed on retry.", secondError);
    return null;
  }
}
