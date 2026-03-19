import type { BaseChannelAdapter } from '../channels/base.js';
import type { OutboundMessage } from '../channels/types.js';
import { ChatRateLimiter } from './rate-limiter.js';

interface DeliveryOptions {
  platformLimit?: number;
  maxRetries?: number;
  interChunkDelayMs?: number;
}

export function chunkMarkdown(text: string, limit: number): string[] {
  if (text.length <= limit) return [text];

  const chunks: string[] = [];
  const lines = text.split('\n');
  let current = '';
  let inCodeBlock = false;
  let fenceLang = '';

  const flush = () => {
    if (!current) return;
    if (inCodeBlock) {
      chunks.push(current + '\n```');
    } else {
      chunks.push(current);
    }
    current = '';
  };

  for (const line of lines) {
    const fenceMatch = line.match(/^(```+)(.*)/);
    const isFence = !!fenceMatch;

    const separator = current ? '\n' : '';
    const addition = separator + line;
    const wouldExceed = current.length + addition.length + (inCodeBlock ? 4 : 0) > limit;

    if (wouldExceed && current) {
      flush();
      if (inCodeBlock) {
        current = '```' + fenceLang + '\n' + line;
      } else {
        current = line;
      }
    } else {
      current += addition;
    }

    if (isFence) {
      if (!inCodeBlock) {
        inCodeBlock = true;
        fenceLang = fenceMatch![2] || '';
      } else {
        inCodeBlock = false;
        fenceLang = '';
      }
    }
  }

  if (current) chunks.push(current);

  const result: string[] = [];
  for (const chunk of chunks) {
    if (chunk.length <= limit) {
      result.push(chunk);
    } else {
      let remaining = chunk;
      while (remaining.length > limit) {
        result.push(remaining.slice(0, limit));
        remaining = remaining.slice(limit);
      }
      if (remaining) result.push(remaining);
    }
  }
  return result;
}

export class DeliveryLayer {
  private rateLimiter = new ChatRateLimiter(20, 60_000);

  async deliver(
    adapter: BaseChannelAdapter,
    chatId: string,
    text: string,
    options: DeliveryOptions = {}
  ): Promise<void> {
    const { platformLimit = 4096, maxRetries = 3, interChunkDelayMs = 300 } = options;
    const chunks = this.chunk(text, platformLimit);

    for (let i = 0; i < chunks.length; i++) {
      // Rate limit
      while (!this.rateLimiter.tryConsume(chatId)) {
        await new Promise(r => setTimeout(r, 1000));
      }

      await this.sendWithRetry(adapter, { chatId, text: chunks[i] }, maxRetries);

      if (i < chunks.length - 1) {
        await new Promise(r => setTimeout(r, interChunkDelayMs));
      }
    }
  }

  private async sendWithRetry(
    adapter: BaseChannelAdapter,
    message: OutboundMessage,
    maxRetries: number
  ): Promise<void> {
    let lastError: Error | null = null;
    for (let attempt = 0; attempt < maxRetries; attempt++) {
      try {
        await adapter.send(message);
        return;
      } catch (err) {
        lastError = err as Error;
        if (attempt < maxRetries - 1) {
          const delay = Math.min(1000 * Math.pow(2, attempt), 10_000);
          await new Promise(r => setTimeout(r, delay));
        }
      }
    }
    throw lastError;
  }

  private chunk(text: string, limit: number): string[] {
    return chunkMarkdown(text, limit);
  }
}
