import { execFile } from "child_process";
import { promisify } from "util";
import * as fs from "fs/promises";
import * as os from "os";
import * as path from "path";

const execFileAsync = promisify(execFile);

const MAX_SEQUENCE_ATTEMPTS = 20;

export async function isObsidianCLIAvailable(): Promise<boolean> {
  try {
    await execFileAsync("obsidian", ["--help"]);
    return true;
  } catch {
    return false;
  }
}

export async function createNote(options: {
  readonly notePath: string;
  readonly content: string;
  readonly vault?: string | null;
}): Promise<void> {
  const tempDir = await fs.mkdtemp(path.join(os.tmpdir(), "meeting-note-"));
  const tempFile = path.join(tempDir, "note.md");

  try {
    await fs.writeFile(tempFile, options.content, "utf-8");

    const args = ["create", `path=${options.notePath}`];
    args.push(`content=$(cat ${tempFile})`);

    if (options.vault) {
      args.push(`vault=${options.vault}`);
    }

    await execFileAsync("obsidian", args, { timeout: 15_000 });
  } finally {
    await fs.rm(tempDir, { recursive: true, force: true }).catch(() => {
      // Best-effort cleanup of temp files.
    });
  }
}

export async function createNoteWithDedup(options: {
  readonly title: string;
  readonly date: string;
  readonly content: string;
  readonly vault?: string | null;
  readonly generatePath: (title: string, date: string, sequence: number) => string;
}): Promise<string> {
  for (let sequence = 1; sequence <= MAX_SEQUENCE_ATTEMPTS; sequence++) {
    const notePath = options.generatePath(options.title, options.date, sequence);
    try {
      await createNote({
        notePath,
        content: options.content,
        vault: options.vault,
      });
      return notePath;
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : String(error);
      const isDuplicate =
        errorMessage.includes("already exists") ||
        errorMessage.includes("EEXIST");
      if (!isDuplicate) {
        throw error;
      }
      // File exists, try next sequence number.
    }
  }

  throw new Error(
    `Could not create note after ${MAX_SEQUENCE_ATTEMPTS} sequence attempts.`
  );
}

export async function openNoteInObsidian(options: {
  readonly notePath: string;
  readonly vault?: string | null;
}): Promise<void> {
  const args = ["open", `path=${options.notePath}`];
  if (options.vault) {
    args.push(`vault=${options.vault}`);
  }
  await execFileAsync("obsidian", args, { timeout: 10_000 });
}
