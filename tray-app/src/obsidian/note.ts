import type { NoteContent, QueueItem, SummaryResult } from "../types";

export function formatNoteMarkdown(content: NoteContent): string {
  const frontmatter = buildFrontmatter(content);
  const body = buildBody(content);
  return `${frontmatter}\n${body}`;
}

function buildFrontmatter(content: NoteContent): string {
  const lines: string[] = ["---"];
  lines.push(`date: ${content.date}`);

  if (content.startTime) {
    lines.push(`start: ${content.startTime}`);
  }
  if (content.endTime) {
    lines.push(`end: ${content.endTime}`);
  }

  if (content.attendees.length > 0) {
    lines.push("attendees:");
    for (const attendee of content.attendees) {
      lines.push(`  - ${attendee}`);
    }
  }

  lines.push("tags:");
  lines.push("  - meeting");
  lines.push("---");

  return lines.join("\n");
}

function buildBody(content: NoteContent): string {
  const sections: string[] = [];

  sections.push("## Summary");
  sections.push("");
  sections.push(content.summary);

  sections.push("");
  sections.push("## Action Items");
  sections.push("");
  if (content.actionItems.length > 0) {
    for (const item of content.actionItems) {
      sections.push(item);
    }
  } else {
    sections.push("None");
  }

  sections.push("");
  sections.push("## Key Decisions");
  sections.push("");
  if (content.keyDecisions.length > 0) {
    for (const decision of content.keyDecisions) {
      sections.push(decision);
    }
  } else {
    sections.push("None");
  }

  sections.push("");
  sections.push("## Transcript");
  sections.push("");
  sections.push(content.transcript);

  return sections.join("\n");
}

export function generateNotePath(
  title: string,
  date: string
): string {
  const sanitizedTitle = title
    .replace(/[\\/:*?"<>|]/g, "")
    .replace(/\s+/g, " ")
    .trim();
  return `Meetings/${date} ${sanitizedTitle}.md`;
}

export function generateNotePathWithSequence(
  title: string,
  date: string,
  sequence: number
): string {
  const sanitizedTitle = title
    .replace(/[\\/:*?"<>|]/g, "")
    .replace(/\s+/g, " ")
    .trim();
  if (sequence <= 1) {
    return `Meetings/${date} ${sanitizedTitle}.md`;
  }
  return `Meetings/${date} ${sanitizedTitle} (${sequence}).md`;
}

export function buildNoteContent(
  item: QueueItem,
  summaryResult: SummaryResult | null,
  readableTranscript: string
): NoteContent {
  const today = new Date().toISOString().slice(0, 10);
  return {
    title: item.meetingTitle,
    date: today,
    startTime: item.startTime ?? null,
    endTime: item.endTime ?? null,
    attendees: item.attendees ?? [],
    summary:
      summaryResult !== null
        ? summaryResult.summary
        : "(LLM summarization failed. Transcript-only note.)",
    actionItems: summaryResult?.actionItems ?? [],
    keyDecisions: summaryResult?.keyDecisions ?? [],
    transcript: readableTranscript,
  };
}
