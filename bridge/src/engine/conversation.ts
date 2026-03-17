import { getBridgeContext } from '../context.js';
import { parseSSE } from '../providers/sse-utils.js';
import type { SSEEvent } from '../providers/base.js';

interface ProcessMessageParams {
  sessionId: string;
  text: string;
  attachments?: any[];
  onTextDelta?: (delta: string) => void;
  onToolUse?: (event: any) => void;
  onPermissionRequest?: (event: any) => Promise<void>;
  onResult?: (event: any) => void;
  onError?: (error: string) => void;
}

interface ProcessMessageResult {
  text: string;
  sessionId: string;
  usage?: { input_tokens: number; output_tokens: number; cost_usd?: number };
}

export class ConversationEngine {
  async processMessage(params: ProcessMessageParams): Promise<ProcessMessageResult> {
    const { store, llm } = getBridgeContext();
    const lockKey = `session:${params.sessionId}`;
    let fullText = '';
    let usage: any;

    // 1. Acquire lock
    await store.acquireLock(lockKey, 600_000);

    try {
      // 2. Save user message
      await store.saveMessage(params.sessionId, {
        role: 'user',
        content: params.text,
        timestamp: new Date().toISOString(),
      });

      // 3. Get session info
      const session = await store.getSession(params.sessionId);
      const workDir = session?.workingDirectory ?? process.cwd();

      // 4. Stream LLM response
      const stream = llm.streamChat({
        prompt: params.text,
        workingDirectory: workDir,
        sessionId: session?.sdkSessionId,
        attachments: params.attachments,
      });

      // 5. Consume stream
      const reader = stream.getReader();
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        const event = parseSSE(value);
        if (!event) continue;

        switch (event.type) {
          case 'text':
            fullText += event.data as string;
            params.onTextDelta?.(event.data as string);
            break;
          case 'tool_use':
            params.onToolUse?.(event.data);
            break;
          case 'permission_request':
            await params.onPermissionRequest?.(event.data);
            break;
          case 'result':
            usage = (event.data as any).usage;
            params.onResult?.(event.data);
            break;
          case 'error':
            params.onError?.(event.data as string);
            break;
        }
      }

      // 6. Save assistant message
      await store.saveMessage(params.sessionId, {
        role: 'assistant',
        content: fullText,
        timestamp: new Date().toISOString(),
      });

    } finally {
      // 7. Release lock
      await store.releaseLock(lockKey);
    }

    return { text: fullText, sessionId: params.sessionId, usage };
  }
}
