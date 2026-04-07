import type { LanguageModelV1 } from "ai";
import type { QueueItem, SummaryResult } from "../types";
import { summarizeWithRetry } from "./summarize";

type OnProcessed = (
  item: QueueItem,
  result: SummaryResult | null
) => void;

export class ProcessingQueue {
  private readonly queue: QueueItem[] = [];
  private processing = false;
  private readonly model: LanguageModelV1;
  private readonly onProcessed: OnProcessed;

  constructor(model: LanguageModelV1, onProcessed: OnProcessed) {
    this.model = model;
    this.onProcessed = onProcessed;
  }

  enqueue(item: QueueItem): void {
    this.queue.push(item);
    if (!this.processing) {
      void this.processNext();
    }
  }

  private async processNext(): Promise<void> {
    if (this.queue.length === 0) {
      this.processing = false;
      return;
    }

    this.processing = true;
    const item = this.queue.shift() as QueueItem;

    const result = await summarizeWithRetry(this.model, item.readableTranscript);
    this.onProcessed(item, result);

    void this.processNext();
  }

  get pending(): number {
    return this.queue.length;
  }

  get isProcessing(): boolean {
    return this.processing;
  }
}
