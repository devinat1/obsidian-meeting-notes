import * as fs from "fs/promises";
import * as path from "path";
import { app } from "electron";
import { createNote, isObsidianCLIAvailable } from "./cli";

const PENDING_DIR_NAME = "pending_notes";
const RETRY_INTERVAL_MS = 60_000;

function getPendingDir(): string {
  return path.join(app.getPath("userData"), PENDING_DIR_NAME);
}

export async function savePendingNote(options: {
  readonly notePath: string;
  readonly content: string;
  readonly vault?: string | null;
}): Promise<void> {
  const pendingDir = getPendingDir();
  await fs.mkdir(pendingDir, { recursive: true });

  const filename = `${Date.now()}-${options.notePath.replace(/[/\\]/g, "_")}`;
  const metaPath = path.join(pendingDir, `${filename}.json`);
  const contentPath = path.join(pendingDir, `${filename}.md`);

  await fs.writeFile(contentPath, options.content, "utf-8");
  await fs.writeFile(
    metaPath,
    JSON.stringify({
      notePath: options.notePath,
      vault: options.vault ?? null,
      contentFile: `${filename}.md`,
      createdAt: new Date().toISOString(),
    }),
    "utf-8"
  );
}

export async function retryPendingNotes(): Promise<number> {
  const pendingDir = getPendingDir();

  try {
    await fs.access(pendingDir);
  } catch {
    return 0;
  }

  const files = await fs.readdir(pendingDir);
  const metaFiles = files.filter((f) => f.endsWith(".json"));

  if (metaFiles.length === 0) {
    return 0;
  }

  const cliAvailable = await isObsidianCLIAvailable();
  if (!cliAvailable) {
    return metaFiles.length;
  }

  let remaining = 0;

  for (const metaFile of metaFiles) {
    const metaPath = path.join(pendingDir, metaFile);
    try {
      const metaRaw = await fs.readFile(metaPath, "utf-8");
      const meta = JSON.parse(metaRaw) as {
        notePath: string;
        vault: string | null;
        contentFile: string;
      };

      const contentPath = path.join(pendingDir, meta.contentFile);
      const content = await fs.readFile(contentPath, "utf-8");

      await createNote({
        notePath: meta.notePath,
        content,
        vault: meta.vault,
      });

      await fs.rm(metaPath, { force: true });
      await fs.rm(contentPath, { force: true });
    } catch {
      remaining += 1;
    }
  }

  return remaining;
}

let retryTimer: ReturnType<typeof setInterval> | null = null;

export function startPendingNotesRetry(
  onRetryResult?: (remaining: number) => void
): void {
  if (retryTimer !== null) {
    return;
  }

  retryTimer = setInterval(async () => {
    const remaining = await retryPendingNotes();
    onRetryResult?.(remaining);
  }, RETRY_INTERVAL_MS);
}

export function stopPendingNotesRetry(): void {
  if (retryTimer !== null) {
    clearInterval(retryTimer);
    retryTimer = null;
  }
}
