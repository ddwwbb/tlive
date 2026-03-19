# Bridge Streaming & UX Optimization — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add streaming edit, tool visibility, verbose levels, typing indicator, session resume, and cost tracking to the Node.js Bridge.

**Architecture:** Incremental enhancement. Two new files (`stream-controller.ts`, `cost-tracker.ts`), one new abstract method on `BaseChannelAdapter` (`sendTyping`), and modifications to `bridge-manager.ts` to wire everything together. No changes to Go Core or ConversationEngine.

**Tech Stack:** TypeScript, Vitest, node-telegram-bot-api, discord.js, @larksuiteoapi/node-sdk

**Spec:** `docs/plans/2026-03-19-bridge-streaming-optimization-design.md`

---

## File Map

| File | Action | Responsibility |
|------|--------|---------------|
| `bridge/src/engine/cost-tracker.ts` | Create | UsageStats interface, CostTracker class, format helper |
| `bridge/src/__tests__/cost-tracker.test.ts` | Create | CostTracker unit tests |
| `bridge/src/engine/stream-controller.ts` | Create | VerboseLevel type, StreamController class, tool emoji map, summarizeToolInput |
| `bridge/src/__tests__/stream-controller.test.ts` | Create | StreamController unit tests |
| `bridge/src/channels/base.ts` | Modify | Add `abstract sendTyping(chatId: string): Promise<void>` |
| `bridge/src/channels/telegram.ts` | Modify | Add `sendTyping`, add error guard to `editMessage` |
| `bridge/src/channels/discord.ts` | Modify | Add `sendTyping` |
| `bridge/src/channels/feishu.ts` | Modify | Implement `editMessage` (replace TODO stub), add `sendTyping` |
| `bridge/src/__tests__/telegram.test.ts` | Modify | Add sendTyping + editMessage error guard tests |
| `bridge/src/__tests__/discord.test.ts` | Modify | Add sendTyping tests |
| `bridge/src/__tests__/feishu.test.ts` | Modify | Add editMessage + sendTyping tests |
| `bridge/src/engine/bridge-manager.ts` | Modify | Wire StreamController, CostTracker, typing heartbeat, session timeout, /verbose + /new commands |
| `bridge/src/__tests__/bridge-manager.test.ts` | Modify | Add streaming, command, session timeout tests |

---

### Task 1: CostTracker

**Files:**
- Create: `bridge/src/engine/cost-tracker.ts`
- Test: `bridge/src/__tests__/cost-tracker.test.ts`

- [ ] **Step 1: Write failing tests**

```typescript
// bridge/src/__tests__/cost-tracker.test.ts
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { CostTracker } from '../engine/cost-tracker.js';

describe('CostTracker', () => {
  let tracker: CostTracker;

  beforeEach(() => {
    tracker = new CostTracker();
  });

  it('tracks duration', () => {
    vi.useFakeTimers();
    tracker.start();
    vi.advanceTimersByTime(5000);
    const stats = tracker.finish({ input_tokens: 100, output_tokens: 50 });
    expect(stats.durationMs).toBe(5000);
    vi.useRealTimers();
  });

  it('uses cost_usd from SDK when available', () => {
    tracker.start();
    const stats = tracker.finish({ input_tokens: 1000, output_tokens: 500, cost_usd: 0.12 });
    expect(stats.costUsd).toBe(0.12);
  });

  it('computes cost from tokens when cost_usd not provided', () => {
    tracker.start();
    const stats = tracker.finish({ input_tokens: 1000, output_tokens: 500 });
    expect(stats.costUsd).toBeGreaterThan(0);
    expect(stats.inputTokens).toBe(1000);
    expect(stats.outputTokens).toBe(500);
  });

  it('formats stats as human-readable string', () => {
    vi.useFakeTimers();
    tracker.start();
    vi.advanceTimersByTime(154000); // 2m 34s
    const stats = tracker.finish({ input_tokens: 12345, output_tokens: 8100, cost_usd: 0.08 });
    const formatted = CostTracker.format(stats);
    expect(formatted).toBe('📊 12.3k/8.1k tok | $0.08 | 2m 34s');
    vi.useRealTimers();
  });

  it('formats sub-1k tokens without k suffix', () => {
    tracker.start();
    const stats = tracker.finish({ input_tokens: 800, output_tokens: 200, cost_usd: 0.01 });
    const formatted = CostTracker.format(stats);
    expect(formatted).toContain('800/200 tok');
  });

  it('formats cost with 2 decimal places', () => {
    tracker.start();
    const stats = tracker.finish({ input_tokens: 100, output_tokens: 50, cost_usd: 1.5 });
    const formatted = CostTracker.format(stats);
    expect(formatted).toContain('$1.50');
  });

  it('formats duration under 1 minute as seconds', () => {
    vi.useFakeTimers();
    tracker.start();
    vi.advanceTimersByTime(45000); // 45s
    const stats = tracker.finish({ input_tokens: 100, output_tokens: 50, cost_usd: 0.01 });
    const formatted = CostTracker.format(stats);
    expect(formatted).toContain('45s');
    expect(formatted).not.toContain('m');
    vi.useRealTimers();
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd bridge && npx vitest run src/__tests__/cost-tracker.test.ts`
Expected: FAIL — module `../engine/cost-tracker.js` not found

- [ ] **Step 3: Implement CostTracker**

```typescript
// bridge/src/engine/cost-tracker.ts
export interface UsageStats {
  inputTokens: number;
  outputTokens: number;
  costUsd: number;
  durationMs: number;
}

function formatTokens(n: number): string {
  return n >= 1000 ? `${(n / 1000).toFixed(1)}k` : `${n}`;
}

function formatDuration(ms: number): string {
  const totalSec = Math.round(ms / 1000);
  if (totalSec < 60) return `${totalSec}s`;
  const min = Math.floor(totalSec / 60);
  const sec = totalSec % 60;
  return sec > 0 ? `${min}m ${sec}s` : `${min}m`;
}

export class CostTracker {
  private startTime = 0;

  start(): void {
    this.startTime = Date.now();
  }

  finish(usage: { input_tokens: number; output_tokens: number; cost_usd?: number }): UsageStats {
    const durationMs = Date.now() - this.startTime;
    const costUsd = usage.cost_usd ?? this.estimateCost(usage.input_tokens, usage.output_tokens);
    return {
      inputTokens: usage.input_tokens,
      outputTokens: usage.output_tokens,
      costUsd,
      durationMs,
    };
  }

  static format(stats: UsageStats): string {
    const tokens = `${formatTokens(stats.inputTokens)}/${formatTokens(stats.outputTokens)} tok`;
    const cost = `$${stats.costUsd.toFixed(2)}`;
    const duration = formatDuration(stats.durationMs);
    return `📊 ${tokens} | ${cost} | ${duration}`;
  }

  private estimateCost(inputTokens: number, outputTokens: number): number {
    // Fallback pricing (Sonnet-level): $3/M input, $15/M output
    return (inputTokens * 3 + outputTokens * 15) / 1_000_000;
  }
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd bridge && npx vitest run src/__tests__/cost-tracker.test.ts`
Expected: All 7 tests PASS

- [ ] **Step 5: Commit**

```bash
git add bridge/src/engine/cost-tracker.ts bridge/src/__tests__/cost-tracker.test.ts
git commit -m "feat(bridge): add CostTracker for usage stats display"
```

---

### Task 2: StreamController

**Files:**
- Create: `bridge/src/engine/stream-controller.ts`
- Test: `bridge/src/__tests__/stream-controller.test.ts`

- [ ] **Step 1: Write failing tests**

```typescript
// bridge/src/__tests__/stream-controller.test.ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { StreamController } from '../engine/stream-controller.js';
import type { UsageStats } from '../engine/cost-tracker.js';

describe('StreamController', () => {
  let flushCallback: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    vi.useFakeTimers();
    flushCallback = vi.fn().mockImplementation((_content: string, isEdit: boolean) => {
      if (!isEdit) return Promise.resolve('msg-1');
      return Promise.resolve();
    });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  function createController(verboseLevel: 0 | 1 | 2 = 1, platformLimit = 4096) {
    return new StreamController({ verboseLevel, platformLimit, throttleMs: 300, flushCallback });
  }

  it('accumulates text and flushes after throttle', async () => {
    const ctrl = createController();
    ctrl.onTextDelta('Hello ');
    ctrl.onTextDelta('world');
    expect(flushCallback).not.toHaveBeenCalled();
    vi.advanceTimersByTime(300);
    await vi.runAllTimersAsync();
    expect(flushCallback).toHaveBeenCalledWith('Hello world', false);
    ctrl.dispose();
  });

  it('first flush returns messageId, subsequent flushes are edits', async () => {
    const ctrl = createController();
    ctrl.onTextDelta('first');
    vi.advanceTimersByTime(300);
    await vi.runAllTimersAsync();
    expect(flushCallback).toHaveBeenCalledWith('first', false);

    ctrl.onTextDelta(' second');
    vi.advanceTimersByTime(300);
    await vi.runAllTimersAsync();
    expect(flushCallback).toHaveBeenCalledWith('first second', true);
    ctrl.dispose();
  });

  it('level 1: shows tool headers without input summary', async () => {
    const ctrl = createController(1);
    ctrl.onToolStart('Grep', { pattern: 'foo', path: 'src/' });
    ctrl.onToolStart('Edit', { file_path: '/tmp/bar.ts' });
    ctrl.onTextDelta('Fixed it.');
    vi.advanceTimersByTime(300);
    await vi.runAllTimersAsync();
    const content = flushCallback.mock.calls[0][0];
    expect(content).toContain('🔍 Grep');
    expect(content).toContain('✏️ Edit');
    expect(content).not.toContain('foo');       // no input summary at level 1
    expect(content).not.toContain('bar.ts');
    expect(content).toContain('Fixed it.');
    ctrl.dispose();
  });

  it('level 2: shows tool headers with input summary', async () => {
    const ctrl = createController(2);
    ctrl.onToolStart('Grep', { pattern: 'foo', path: 'src/' });
    ctrl.onToolStart('Bash', { command: 'npm test' });
    ctrl.onTextDelta('Done.');
    vi.advanceTimersByTime(300);
    await vi.runAllTimersAsync();
    const content = flushCallback.mock.calls[0][0];
    expect(content).toContain('🔍 Grep "foo" in src/');
    expect(content).toContain('🖥️ Bash npm test');
    ctrl.dispose();
  });

  it('level 0: does not flush text deltas', async () => {
    const ctrl = createController(0);
    ctrl.onTextDelta('some text');
    vi.advanceTimersByTime(300);
    await vi.runAllTimersAsync();
    expect(flushCallback).not.toHaveBeenCalled();
    ctrl.dispose();
  });

  it('onComplete flushes final message with cost line at all levels', async () => {
    const ctrl = createController(0);
    const stats: UsageStats = { inputTokens: 1000, outputTokens: 500, costUsd: 0.05, durationMs: 10000 };
    ctrl.onComplete(stats);
    await vi.runAllTimersAsync();
    const content = flushCallback.mock.calls[0][0];
    expect(content).toContain('📊');
    ctrl.dispose();
  });

  it('truncates content exceeding platform limit during streaming', async () => {
    const ctrl = createController(1, 500);
    ctrl.onTextDelta('x'.repeat(1000));
    vi.advanceTimersByTime(300);
    await vi.runAllTimersAsync();
    const content = flushCallback.mock.calls[0][0] as string;
    expect(content.length).toBeLessThanOrEqual(500);
    expect(content.startsWith('...\n')).toBe(true);
    // Should contain the tail of the content
    expect(content).toContain('x'.repeat(10));
    ctrl.dispose();
  });

  it('level 2 falls back to level 1 format when tool input is empty', async () => {
    const ctrl = createController(2);
    ctrl.onToolStart('Read', {});  // empty input
    ctrl.onTextDelta('text');
    vi.advanceTimersByTime(300);
    await vi.runAllTimersAsync();
    const content = flushCallback.mock.calls[0][0];
    expect(content).toContain('📖 Read');
    expect(content).not.toContain('📖 Read ');  // no trailing space + summary
    ctrl.dispose();
  });

  it('onError flushes error message', async () => {
    const ctrl = createController(1);
    ctrl.onError('something went wrong');
    await vi.runAllTimersAsync();
    const content = flushCallback.mock.calls[0][0];
    expect(content).toContain('❌ Error: something went wrong');
    ctrl.dispose();
  });

  it('dispose clears pending timers', () => {
    const ctrl = createController();
    ctrl.onTextDelta('hello');
    ctrl.dispose();
    vi.advanceTimersByTime(1000);
    expect(flushCallback).not.toHaveBeenCalled();
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd bridge && npx vitest run src/__tests__/stream-controller.test.ts`
Expected: FAIL — module not found

- [ ] **Step 3: Implement StreamController**

```typescript
// bridge/src/engine/stream-controller.ts
import { CostTracker, type UsageStats } from './cost-tracker.js';

export type VerboseLevel = 0 | 1 | 2;

const TOOL_EMOJI: Record<string, string> = {
  Read: '📖', Edit: '✏️', Write: '📝',
  Bash: '🖥️', Grep: '🔍', Glob: '📂',
  Agent: '🤖', WebSearch: '🌐', WebFetch: '🌐',
};

function getToolEmoji(name: string): string {
  return TOOL_EMOJI[name] ?? '🔧';
}

function summarizeToolInput(name: string, input: Record<string, any>): string {
  if (!input || Object.keys(input).length === 0) return '';
  switch (name) {
    case 'Read': return input.file_path?.split('/').pop() ?? '';
    case 'Edit': case 'Write': return input.file_path?.split('/').pop() ?? '';
    case 'Grep': return `"${input.pattern}" in ${input.path ?? '.'}`;
    case 'Glob': return input.pattern ?? '';
    case 'Bash': return (input.command ?? '').slice(0, 80);
    default: return '';
  }
}

interface StreamControllerOptions {
  verboseLevel: VerboseLevel;
  platformLimit: number;
  throttleMs?: number;
  flushCallback: (content: string, isEdit: boolean) => Promise<string | void>;
}

export class StreamController {
  private buffer = '';
  private toolHeaders: string[] = [];
  private _messageId?: string;
  private timer: ReturnType<typeof setTimeout> | null = null;
  private verboseLevel: VerboseLevel;
  private platformLimit: number;
  private throttleMs: number;
  private flushCallback: (content: string, isEdit: boolean) => Promise<string | void>;
  private flushing = false;
  private pendingFlush = false;

  get messageId(): string | undefined {
    return this._messageId;
  }

  constructor(options: StreamControllerOptions) {
    this.verboseLevel = options.verboseLevel;
    this.platformLimit = options.platformLimit;
    this.throttleMs = options.throttleMs ?? 300;
    this.flushCallback = options.flushCallback;
  }

  onTextDelta(text: string): void {
    this.buffer += text;
    if (this.verboseLevel === 0) return;  // level 0: no streaming
    this.scheduleFlush();
  }

  onToolStart(name: string, input?: Record<string, any>): void {
    if (this.verboseLevel === 0) return;

    let header = `${getToolEmoji(name)} ${name}`;
    if (this.verboseLevel === 2 && input) {
      const summary = summarizeToolInput(name, input);
      if (summary) header += ` ${summary}`;
    }
    this.toolHeaders.push(header);
    this.scheduleFlush();
  }

  onComplete(stats: UsageStats): void {
    const costLine = CostTracker.format(stats);
    const content = this.compose(costLine);
    this.cancelTimer();
    this.doFlush(content);
  }

  onError(error: string): void {
    this.cancelTimer();
    const content = `❌ Error: ${error}`;
    this.doFlush(content);
  }

  dispose(): void {
    this.cancelTimer();
  }

  private scheduleFlush(): void {
    if (this.timer) return;
    this.timer = setTimeout(() => {
      this.timer = null;
      const content = this.compose();
      this.doFlush(content);
    }, this.throttleMs);
  }

  private cancelTimer(): void {
    if (this.timer) {
      clearTimeout(this.timer);
      this.timer = null;
    }
  }

  private compose(costLine?: string): string {
    const parts: string[] = [];

    if (this.toolHeaders.length > 0 && this.verboseLevel > 0) {
      parts.push(this.toolHeaders.join(' → '));
      parts.push('──────────────────');
    }

    if (this.buffer && this.verboseLevel > 0) {
      parts.push(this.buffer);
    }

    if (costLine) {
      parts.push(costLine);
    }

    let content = parts.join('\n');

    // Truncate for streaming if over platform limit
    if (content.length > this.platformLimit) {
      const tail = content.slice(-(this.platformLimit - 100));
      content = '...\n' + tail;
    }

    return content;
  }

  private async doFlush(content: string): Promise<void> {
    if (!content) return;
    if (this.flushing) {
      this.pendingFlush = true;
      return;
    }
    this.flushing = true;
    try {
      const isEdit = !!this._messageId;
      const result = await this.flushCallback(content, isEdit);
      if (!isEdit && typeof result === 'string') {
        this._messageId = result;
      }
    } finally {
      this.flushing = false;
      if (this.pendingFlush) {
        this.pendingFlush = false;
        const retryContent = this.compose();
        if (retryContent) await this.doFlush(retryContent);
      }
    }
  }
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd bridge && npx vitest run src/__tests__/stream-controller.test.ts`
Expected: All 9 tests PASS

- [ ] **Step 5: Commit**

```bash
git add bridge/src/engine/stream-controller.ts bridge/src/__tests__/stream-controller.test.ts
git commit -m "feat(bridge): add StreamController for streaming edit + tool visibility"
```

---

### Task 3: Add sendTyping to Channel Adapters

**Files:**
- Modify: `bridge/src/channels/base.ts:9` — add abstract method
- Modify: `bridge/src/channels/telegram.ts:94` — add sendTyping, add error guard to editMessage
- Modify: `bridge/src/channels/discord.ts:132` — add sendTyping
- Modify: `bridge/src/channels/feishu.ts:97-99` — implement editMessage, add sendTyping
- Modify: `bridge/src/__tests__/telegram.test.ts`
- Modify: `bridge/src/__tests__/discord.test.ts`
- Modify: `bridge/src/__tests__/feishu.test.ts`

- [ ] **Step 1: Add `sendTyping` to base.ts**

In `bridge/src/channels/base.ts`, after line 9 (`editMessage`), add:

```typescript
  abstract sendTyping(chatId: string): Promise<void>;
```

- [ ] **Step 2: Add sendTyping + editMessage error guard to telegram.ts**

After the existing `editMessage` method (line 94), add error guard and `sendTyping`:

Wrap the `editMessageText` call in `editMessage` with a try/catch:

```typescript
  async editMessage(chatId: string, messageId: string, message: OutboundMessage): Promise<void> {
    if (!this.bot) return;
    const text = message.html ?? message.text ?? '';
    const options: TelegramBot.EditMessageTextOptions = {
      chat_id: chatId,
      message_id: parseInt(messageId, 10),
    };
    if (message.html) options.parse_mode = 'HTML';
    try {
      await this.bot.editMessageText(text, options);
    } catch (err: any) {
      if (!err.message?.includes('message is not modified')) throw err;
    }
  }

  async sendTyping(chatId: string): Promise<void> {
    try {
      await this.bot?.sendChatAction(chatId, 'typing');
    } catch {
      // Non-critical; swallow errors
    }
  }
```

- [ ] **Step 3: Add sendTyping to discord.ts**

After the existing `editMessage` method (line 132), add:

```typescript
  async sendTyping(chatId: string): Promise<void> {
    try {
      const channel = await this.client?.channels.fetch(chatId);
      if (channel?.isTextBased()) await (channel as TextChannel).sendTyping();
    } catch {
      // Non-critical; swallow errors
    }
  }
```

- [ ] **Step 4: Implement editMessage + sendTyping in feishu.ts**

Replace the TODO stub at lines 97-99 with:

```typescript
  async editMessage(_chatId: string, messageId: string, message: OutboundMessage): Promise<void> {
    if (!this.client) return;
    const text = message.text ?? message.html ?? '';

    const card = {
      config: { wide_screen_mode: true },
      elements: [
        { tag: 'markdown', content: text },
      ],
    };

    if (message.buttons?.length) {
      card.elements.push({
        tag: 'action',
        actions: message.buttons.map(btn => ({
          tag: 'button',
          text: { tag: 'plain_text', content: btn.label },
          type: btn.style === 'danger' ? 'danger' : 'primary',
          value: { action: btn.callbackData },
        })),
      } as any);
    }

    await this.client.im.message.patch({
      path: { message_id: messageId },
      data: {
        msg_type: 'interactive',
        content: JSON.stringify(card),
      },
    });
  }

  async sendTyping(_chatId: string): Promise<void> {
    // Feishu has no native typing API; streaming card updates serve this purpose
  }
```

- [ ] **Step 5: Add tests for sendTyping to existing test files**

**telegram.test.ts:** First add `sendChatAction` to the MockBot constructor (inside the `vi.mock` at the top of the file):

```typescript
this.sendChatAction = vi.fn().mockResolvedValue(true);
```

Then add these test blocks:

```typescript
describe('TelegramAdapter sendTyping', () => {
  it('calls sendChatAction with typing', async () => {
    await adapter.start();
    await adapter.sendTyping('12345');
    const bot = (adapter as any).bot;
    expect(bot.sendChatAction).toHaveBeenCalledWith('12345', 'typing');
  });

  it('swallows errors from sendChatAction', async () => {
    await adapter.start();
    const bot = (adapter as any).bot;
    bot.sendChatAction.mockRejectedValueOnce(new Error('network error'));
    await expect(adapter.sendTyping('12345')).resolves.toBeUndefined();
  });
});

describe('TelegramAdapter editMessage error guard', () => {
  it('ignores "message is not modified" error', async () => {
    await adapter.start();
    const bot = (adapter as any).bot;
    bot.editMessageText.mockRejectedValueOnce(new Error('message is not modified'));
    await expect(
      adapter.editMessage('12345', '42', { chatId: '12345', text: 'same' })
    ).resolves.toBeUndefined();
  });

  it('rethrows other errors', async () => {
    await adapter.start();
    const bot = (adapter as any).bot;
    bot.editMessageText.mockRejectedValueOnce(new Error('rate limited'));
    await expect(
      adapter.editMessage('12345', '42', { chatId: '12345', text: 'new' })
    ).rejects.toThrow('rate limited');
  });
});
```

**discord.test.ts:** Add a `mockSendTyping` function at the top alongside existing mocks, and add `isTextBased` + `sendTyping` to the mock channel:

```typescript
const mockSendTyping = vi.fn().mockResolvedValue(undefined);
// Update mockFetchChannel to include isTextBased and sendTyping:
const mockFetchChannel = vi.fn().mockResolvedValue({
  send: mockSend,
  messages: { fetch: mockFetchMessage },
  isTextBased: () => true,
  sendTyping: mockSendTyping,
});
```

Then add this test block:

```typescript
describe('sendTyping()', () => {
  it('calls channel.sendTyping', async () => {
    await adapter.start();
    await adapter.sendTyping('channel1');
    expect(mockFetchChannel).toHaveBeenCalledWith('channel1');
    expect(mockSendTyping).toHaveBeenCalled();
  });

  it('swallows errors', async () => {
    await adapter.start();
    mockFetchChannel.mockRejectedValueOnce(new Error('network'));
    await expect(adapter.sendTyping('channel1')).resolves.toBeUndefined();
  });
});
```

**feishu.test.ts:** Add `mockMessagePatch` to the mock at the top, alongside `mockMessageCreate`:

```typescript
const mockMessagePatch = vi.fn().mockResolvedValue({});

// Update the MockClient mock to include patch:
vi.mock('@larksuiteoapi/node-sdk', () => {
  const MockClient = vi.fn(function (this: any) {
    this.im = {
      message: {
        create: mockMessageCreate,
        patch: mockMessagePatch,
      },
    };
  });
  return { Client: MockClient };
});
```

Then add these test blocks:

```typescript
describe('editMessage()', () => {
  it('patches message with interactive card', async () => {
    await adapter.start();
    await adapter.editMessage('oc_chat123', 'msg-feishu-1', {
      chatId: 'oc_chat123',
      text: 'Updated content',
    });
    expect(mockMessagePatch).toHaveBeenCalledOnce();
    const call = mockMessagePatch.mock.calls[0][0];
    expect(call.path.message_id).toBe('msg-feishu-1');
    expect(call.data.msg_type).toBe('interactive');
    const card = JSON.parse(call.data.content);
    expect(card.elements[0].tag).toBe('markdown');
    expect(card.elements[0].content).toBe('Updated content');
  });

  it('does nothing when client is not started', async () => {
    await adapter.editMessage('oc_chat', 'msg-1', { chatId: 'oc_chat', text: 'hi' });
    expect(mockMessagePatch).not.toHaveBeenCalled();
  });
});

describe('sendTyping()', () => {
  it('is a no-op that resolves without error', async () => {
    await adapter.start();
    await expect(adapter.sendTyping('oc_chat123')).resolves.toBeUndefined();
  });
});
```

- [ ] **Step 6: Run all channel tests**

Run: `cd bridge && npx vitest run src/__tests__/telegram.test.ts src/__tests__/discord.test.ts src/__tests__/feishu.test.ts src/__tests__/channels.test.ts`
Expected: All PASS

- [ ] **Step 7: Commit**

```bash
git add bridge/src/channels/ bridge/src/__tests__/telegram.test.ts bridge/src/__tests__/discord.test.ts bridge/src/__tests__/feishu.test.ts
git commit -m "feat(bridge): add sendTyping to adapters, implement Feishu editMessage, add TG error guard"
```

---

### Task 4: Wire StreamController + CostTracker + Typing into BridgeManager

**Files:**
- Modify: `bridge/src/engine/bridge-manager.ts`
- Modify: `bridge/src/__tests__/bridge-manager.test.ts`

This is the integration task. The core change replaces the TODO block in `handleInboundMessage` (lines 122-146) with StreamController wiring.

- [ ] **Step 1: Add imports and private fields to BridgeManager**

At top of `bridge-manager.ts`, add imports:

```typescript
import { StreamController, type VerboseLevel } from './stream-controller.js';
import { CostTracker } from './cost-tracker.js';
```

Add private fields after existing ones (after line 21):

```typescript
  private verboseLevels = new Map<string, VerboseLevel>();
  private lastActive = new Map<string, number>();
```

- [ ] **Step 2: Add helper methods to BridgeManager**

After `stop()` method (line 60), add:

```typescript
  private stateKey(channelType: string, chatId: string): string {
    return `${channelType}:${chatId}`;
  }

  private getVerboseLevel(channelType: string, chatId: string): VerboseLevel {
    return this.verboseLevels.get(this.stateKey(channelType, chatId)) ?? 1;
  }

  private setVerboseLevel(channelType: string, chatId: string, level: VerboseLevel): void {
    this.verboseLevels.set(this.stateKey(channelType, chatId), level);
  }

  private checkAndUpdateLastActive(channelType: string, chatId: string): boolean {
    const key = this.stateKey(channelType, chatId);
    const last = this.lastActive.get(key);
    const now = Date.now();
    this.lastActive.set(key, now);
    if (last && (now - last) > 30 * 60 * 1000) return true;
    return false;
  }

  private clearLastActive(channelType: string, chatId: string): void {
    this.lastActive.delete(this.stateKey(channelType, chatId));
  }
```

- [ ] **Step 3: Replace the message handling block in handleInboundMessage**

Replace lines 121-146 (from `// Regular message` to end of delivery) with:

```typescript
    // Session resume: check timeout before resolving
    const expired = this.checkAndUpdateLastActive(msg.channelType, msg.chatId);
    if (expired) {
      const newId = `session-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
      await this.router.rebind(msg.channelType, msg.chatId, newId);
    }

    const binding = await this.router.resolve(msg.channelType, msg.chatId);

    // Start typing heartbeat
    const typingInterval = setInterval(() => {
      adapter.sendTyping(msg.chatId);
    }, 4000);
    adapter.sendTyping(msg.chatId);

    const verboseLevel = this.getVerboseLevel(msg.channelType, msg.chatId);
    const costTracker = new CostTracker();
    costTracker.start();

    const platformLimits: Record<string, number> = { telegram: 4096, discord: 2000, feishu: 30000 };
    const stream = new StreamController({
      verboseLevel,
      platformLimit: platformLimits[adapter.channelType] ?? 4096,
      flushCallback: async (content, isEdit) => {
        if (!isEdit) {
          const result = await adapter.send({ chatId: msg.chatId, text: content });
          clearInterval(typingInterval);
          return result.messageId;
        } else {
          await adapter.editMessage(msg.chatId, stream.messageId!, { chatId: msg.chatId, text: content });
        }
      },
    });

    try {
      const result = await this.engine.processMessage({
        sessionId: binding.sessionId,
        text: msg.text,
        onTextDelta: (delta) => stream.onTextDelta(delta),
        onToolUse: (event) => stream.onToolStart(event.name, event.input),
        onResult: (event) => {
          // Level 1+: flush final message with cost line via StreamController
          if (verboseLevel > 0) {
            const usage = event.usage ?? { input_tokens: 0, output_tokens: 0 };
            const stats = costTracker.finish(usage);
            stream.onComplete(stats);
          }
        },
        onError: (err) => stream.onError(err),
        onPermissionRequest: async (req) => {
          await this.broker.forwardPermissionRequest(req, msg.chatId, [adapter]);
        },
      });

      // Level 0: deliver final response via delivery layer (stream didn't flush text)
      if (verboseLevel === 0) {
        const responseText = result.text.trim();
        const usage = result.usage ?? { input_tokens: 0, output_tokens: 0 };
        const stats = costTracker.finish(usage);
        const costLine = CostTracker.format(stats);
        const fullText = responseText ? `${responseText}\n${costLine}` : costLine;
        await this.delivery.deliver(adapter, msg.chatId, fullText, {
          platformLimit: platformLimits[adapter.channelType] ?? 4096,
        });
      }
    } finally {
      clearInterval(typingInterval);
      stream.dispose();
    }

    return true;
```

- [ ] **Step 4: Update handleCommand to add /verbose and fix /new**

Replace the existing `handleCommand` method (lines 149-177) with:

```typescript
  private async handleCommand(adapter: BaseChannelAdapter, msg: InboundMessage): Promise<boolean> {
    const parts = msg.text.split(' ');
    const cmd = parts[0].toLowerCase();

    switch (cmd) {
      case '/status': {
        const ctx = getBridgeContext();
        const healthy = (ctx.core as any).isHealthy?.() ?? false;
        await adapter.send({
          chatId: msg.chatId,
          text: `TermLive Status\nCore: ${healthy ? '● connected' : '○ disconnected'}\nAdapters: ${this.adapters.size} active`,
        });
        return true;
      }
      case '/new': {
        const newSessionId = `session-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
        await this.router.rebind(msg.channelType, msg.chatId, newSessionId);
        this.clearLastActive(msg.channelType, msg.chatId);
        await adapter.send({ chatId: msg.chatId, text: '🆕 New session started.' });
        return true;
      }
      case '/verbose': {
        const level = parseInt(parts[1]) as VerboseLevel;
        if ([0, 1, 2].includes(level)) {
          this.setVerboseLevel(msg.channelType, msg.chatId, level);
          const labels = ['quiet', 'normal', 'detailed'];
          await adapter.send({ chatId: msg.chatId, text: `Verbose level: ${level} (${labels[level]})` });
        } else {
          await adapter.send({ chatId: msg.chatId, text: 'Usage: /verbose 0|1|2\n0=quiet, 1=normal, 2=detailed' });
        }
        return true;
      }
      case '/help': {
        await adapter.send({
          chatId: msg.chatId,
          text: 'Commands:\n/status - Show status\n/new - New session\n/verbose 0|1|2 - Set detail level\n/help - Show help',
        });
        return true;
      }
      default:
        return false;
    }
  }
```

- [ ] **Step 5: Write BridgeManager integration tests**

Add to `bridge/src/__tests__/bridge-manager.test.ts`:

```typescript
  it('streams response via StreamController', async () => {
    const adapter = mockAdapter();
    manager.registerAdapter(adapter);

    await manager.handleInboundMessage(adapter, {
      channelType: 'telegram', chatId: 'c1', userId: 'u1', text: 'hello', messageId: 'm1',
    });

    // adapter.send should be called (first flush)
    expect(adapter.send).toHaveBeenCalled();
  });

  it('sends typing indicator', async () => {
    const adapter = mockAdapter();
    (adapter as any).sendTyping = vi.fn().mockResolvedValue(undefined);
    manager.registerAdapter(adapter);

    await manager.handleInboundMessage(adapter, {
      channelType: 'telegram', chatId: 'c1', userId: 'u1', text: 'hello', messageId: 'm1',
    });

    expect((adapter as any).sendTyping).toHaveBeenCalledWith('c1');
  });

  it('handles /verbose command', async () => {
    const adapter = mockAdapter();
    manager.registerAdapter(adapter);

    await manager.handleInboundMessage(adapter, {
      channelType: 'telegram', chatId: 'c1', userId: 'u1', text: '/verbose 2', messageId: 'm1',
    });

    expect(adapter.send).toHaveBeenCalledWith(
      expect.objectContaining({ text: expect.stringContaining('Verbose level: 2') })
    );
  });

  it('handles /verbose with invalid arg', async () => {
    const adapter = mockAdapter();
    manager.registerAdapter(adapter);

    await manager.handleInboundMessage(adapter, {
      channelType: 'telegram', chatId: 'c1', userId: 'u1', text: '/verbose 5', messageId: 'm1',
    });

    expect(adapter.send).toHaveBeenCalledWith(
      expect.objectContaining({ text: expect.stringContaining('Usage') })
    );
  });

  it('handles /new command with rebind', async () => {
    const adapter = mockAdapter();
    manager.registerAdapter(adapter);

    await manager.handleInboundMessage(adapter, {
      channelType: 'telegram', chatId: 'c1', userId: 'u1', text: '/new', messageId: 'm1',
    });

    expect(adapter.send).toHaveBeenCalledWith(
      expect.objectContaining({ text: expect.stringContaining('New session') })
    );
  });

  it('updates /help text to include /verbose', async () => {
    const adapter = mockAdapter();
    manager.registerAdapter(adapter);

    await manager.handleInboundMessage(adapter, {
      channelType: 'telegram', chatId: 'c1', userId: 'u1', text: '/help', messageId: 'm1',
    });

    expect(adapter.send).toHaveBeenCalledWith(
      expect.objectContaining({ text: expect.stringContaining('/verbose') })
    );
  });
```

- [ ] **Step 6: Update mockAdapter in test file**

Add `sendTyping` to the mock adapter:

```typescript
function mockAdapter(channelType = 'telegram'): BaseChannelAdapter {
  const messageQueue: any[] = [];
  return {
    channelType,
    start: vi.fn().mockResolvedValue(undefined),
    stop: vi.fn().mockResolvedValue(undefined),
    consumeOne: vi.fn().mockImplementation(() => messageQueue.shift() ?? null),
    send: vi.fn().mockResolvedValue({ messageId: '1', success: true }),
    editMessage: vi.fn(),
    sendTyping: vi.fn().mockResolvedValue(undefined),     // ← new
    validateConfig: vi.fn().mockReturnValue(null),
    isAuthorized: vi.fn().mockReturnValue(true),
    _pushMessage: (msg: any) => messageQueue.push(msg),
  } as any;
}
```

- [ ] **Step 7: Run all tests**

Run: `cd bridge && npx vitest run`
Expected: All tests PASS

- [ ] **Step 8: Commit**

```bash
git add bridge/src/engine/bridge-manager.ts bridge/src/__tests__/bridge-manager.test.ts
git commit -m "feat(bridge): wire streaming edit, typing, session resume, verbose, cost tracking"
```

---

### Task 5: Final Verification

- [ ] **Step 1: Run full test suite**

Run: `cd bridge && npx vitest run`
Expected: All tests PASS, no regressions

- [ ] **Step 2: Type check**

Run: `cd bridge && npx tsc --noEmit`
Expected: No type errors

- [ ] **Step 3: Build**

Run: `cd bridge && npm run build`
Expected: Build succeeds

- [ ] **Step 4: Commit any fixes if needed**

```bash
git add -A && git commit -m "fix: address type/build issues from streaming optimization"
```

(Skip if no fixes needed)
