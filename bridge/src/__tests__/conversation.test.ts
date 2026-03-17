import { describe, it, expect, vi, beforeEach } from 'vitest';
import { ConversationEngine } from '../engine/conversation.js';
import { initBridgeContext } from '../context.js';
import { sseEvent } from '../providers/sse-utils.js';

// Mock store
function createMockStore() {
  return {
    getSession: vi.fn().mockResolvedValue({ id: 's1', workingDirectory: '/tmp', createdAt: '' }),
    saveMessage: vi.fn().mockResolvedValue(undefined),
    getMessages: vi.fn().mockResolvedValue([]),
    acquireLock: vi.fn().mockResolvedValue(true),
    renewLock: vi.fn().mockResolvedValue(true),
    releaseLock: vi.fn().mockResolvedValue(undefined),
    // other methods as stubs
    saveSession: vi.fn(), deleteSession: vi.fn(), listSessions: vi.fn(),
    getBinding: vi.fn(), saveBinding: vi.fn(), deleteBinding: vi.fn(), listBindings: vi.fn(),
    isDuplicate: vi.fn(), markProcessed: vi.fn(),
  };
}

// Mock LLM that emits controlled events
function createMockLLM(events: string[]) {
  return {
    streamChat: () => new ReadableStream<string>({
      start(controller) {
        for (const event of events) {
          controller.enqueue(event);
        }
        controller.close();
      }
    })
  };
}

describe('ConversationEngine', () => {
  let engine: ConversationEngine;
  let mockStore: ReturnType<typeof createMockStore>;

  beforeEach(() => {
    mockStore = createMockStore();
    const mockLLM = createMockLLM([
      sseEvent('text', 'Hello '),
      sseEvent('text', 'world'),
      sseEvent('result', { session_id: 's1', is_error: false, usage: { input_tokens: 10, output_tokens: 5 } }),
    ]);

    initBridgeContext({
      store: mockStore as any,
      llm: mockLLM as any,
      permissions: {} as any,
      core: {} as any,
    });

    engine = new ConversationEngine();
  });

  it('processes message and returns full response', async () => {
    const result = await engine.processMessage({
      sessionId: 's1',
      text: 'hi',
    });
    expect(result.text).toBe('Hello world');
  });

  it('acquires and releases session lock', async () => {
    await engine.processMessage({ sessionId: 's1', text: 'hi' });
    expect(mockStore.acquireLock).toHaveBeenCalledWith('session:s1', expect.any(Number));
    expect(mockStore.releaseLock).toHaveBeenCalledWith('session:s1');
  });

  it('saves user and assistant messages', async () => {
    await engine.processMessage({ sessionId: 's1', text: 'hi' });
    expect(mockStore.saveMessage).toHaveBeenCalledTimes(2);
    // First call: user message
    expect(mockStore.saveMessage.mock.calls[0][1].role).toBe('user');
    // Second call: assistant message
    expect(mockStore.saveMessage.mock.calls[1][1].role).toBe('assistant');
  });

  it('calls onTextDelta for streaming', async () => {
    const deltas: string[] = [];
    await engine.processMessage({
      sessionId: 's1',
      text: 'hi',
      onTextDelta: (d) => deltas.push(d),
    });
    expect(deltas).toEqual(['Hello ', 'world']);
  });

  it('calls onResult with usage', async () => {
    let resultData: any;
    await engine.processMessage({
      sessionId: 's1',
      text: 'hi',
      onResult: (r) => { resultData = r; },
    });
    expect(resultData.usage.input_tokens).toBe(10);
  });

  it('releases lock even on error', async () => {
    const errorLLM = {
      streamChat: () => new ReadableStream<string>({
        start(controller) {
          controller.enqueue(sseEvent('error', 'boom'));
          controller.close();
        }
      })
    };
    initBridgeContext({ store: mockStore as any, llm: errorLLM as any, permissions: {} as any, core: {} as any });
    engine = new ConversationEngine();

    await engine.processMessage({ sessionId: 's1', text: 'hi' });
    expect(mockStore.releaseLock).toHaveBeenCalled();
  });
});
