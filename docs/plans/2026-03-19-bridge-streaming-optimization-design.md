# Bridge Streaming & UX Optimization Design

**Date:** 2026-03-19
**Scope:** Node.js Bridge only (Go Core unchanged except minor hook API extensions if needed)
**Approach:** Incremental enhancement on existing architecture (no EventBus refactor)

## Overview

Enhance the IM Bridge experience with 6 features borrowed from claude-code-telegram, adapted for multi-platform (Telegram/Discord/Feishu):

1. **Streaming edit** — real-time message updates as Claude generates output
2. **Tool visibility** — emoji + tool name shown during execution
3. **Verbose levels** — 3-level detail control per conversation
4. **Typing indicator** — native typing status on TG/DC, skip on Feishu
5. **Session resume** — auto-resume by user with 30-min timeout
6. **Cost tracking** — display tokens/cost/duration after each task

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Session resume strategy | Per-user auto-resume (A) | Bridge runs locally, directory is fixed; no need for per-directory tracking |
| Verbose default | Level 1 (normal) | Balances visibility vs noise; Level 0 too quiet, Level 2 too verbose for Discord 2000-char limit |
| Cost tracking | Display only, no limits | User's own API key; limits add complexity without value |
| Feishu typing | Skip (empty impl) | Streaming card updates arrive within seconds; no native API |
| Architecture | Incremental (no EventBus) | 24 source files, not complex enough to justify pub/sub indirection; easier for open-source contributors to trace data flow |

## Data Flow

```
ConversationEngine.processMessage() SSE events
    │
    ├── type: 'text'             → onTextDelta callback
    ├── type: 'tool_use'         → onToolUse callback
    ├── type: 'result'           → onResult callback (includes usage)
    │
    ▼
StreamController (new: engine/stream-controller.ts)
    │  - Accumulate text buffer
    │  - Build tool headers (emoji + name, optionally + input summary)
    │  - Throttle flush at 300ms intervals
    │  - Respect verboseLevel
    │
    ▼
BridgeManager.handleInboundMessage() (modified: engine/bridge-manager.ts)
    │  - First flush → adapter.send() → store messageId from SendResult
    │  - Subsequent flushes → adapter.editMessage(chatId, messageId, msg)
    │  - On complete → final flush with cost line, stop typing heartbeat
    │
    ▼
Channel Adapter (existing editMessage + new sendTyping)
    │  - TG: editMessageText() + sendChatAction('typing')
    │  - DC: message.edit() + channel.sendTyping()
    │  - Feishu: client.im.message.patch() (CardKit v2) + no-op typing
    │
    ▼
User sees real-time updates on phone
```

## New Files

### engine/stream-controller.ts

```typescript
export type VerboseLevel = 0 | 1 | 2;

interface StreamControllerOptions {
  verboseLevel: VerboseLevel;
  platformLimit: number;  // TG 4096, DC 2000, Feishu 30000
  throttleMs?: number;    // default 300
  // First call (isEdit=false): send new message, return messageId from SendResult
  // Subsequent calls (isEdit=true): edit existing message
  flushCallback: (content: string, isEdit: boolean) => Promise<string | void>;
}

class StreamController {
  private buffer: string = '';
  private toolHeaders: string[] = [];
  private messageId?: string;

  constructor(options: StreamControllerOptions);

  onTextDelta(text: string): void;      // accumulate buffer, schedule throttled flush
  onToolStart(name: string, input?: Record<string, any>): void;  // append tool header
  onComplete(usage: UsageStats): void;  // final flush with cost line
  dispose(): void;                      // clear throttle timer

  // Composed message format:
  // 🔍 Grep → ✏️ Edit → 🖥️ Bash       ← tool headers (level 1+)
  // 🔍 Grep "pattern" in src/ → ...    ← tool headers (level 2)
  // ──────────────────
  // The bug was in line 42...            ← streamed text (level 1+, suppressed at level 0)
  // 📊 12.3k/8.1k tok | $0.08 | 2m 34s ← cost line (all levels)
}
```

**Note on tool input for Level 2:** The SSE `tool_use` event from the SDK may arrive with partial input. `summarizeToolInput` extracts what's available (tool name is always present; input fields like `file_path`, `pattern`, `command` are best-effort). If input is empty, fall back to Level 1 format (emoji + name only).

### engine/cost-tracker.ts

```typescript
export interface UsageStats {
  inputTokens: number;
  outputTokens: number;
  costUsd: number;       // from SDK result.usage.cost_usd (preferred) or computed as fallback
  durationMs: number;
}

export class CostTracker {
  private startTime: number;

  start(): void;
  finish(usage: { input_tokens: number; output_tokens: number; cost_usd?: number }): UsageStats;
  format(stats: UsageStats): string;  // "📊 12.3k/8.1k tok | $0.08 | 2m 34s"
}
```

Cost source priority:
1. `cost_usd` from SDK `result` event (already available in `ProcessMessageResult.usage.cost_usd`)
2. Fallback: compute from token counts using model pricing table (only if SDK doesn't provide cost)

## Modified Files

### channels/base.ts

`editMessage` already exists with signature `(chatId, messageId, message: OutboundMessage) → Promise<void>`. No signature change needed.

Add one new abstract method:

```typescript
abstract sendTyping(chatId: string): Promise<void>;
```

`send()` already returns `Promise<SendResult>` with `messageId`. No change needed.

### channels/telegram.ts

Existing `editMessage(chatId, messageId, message)` implementation — verify it handles `message is not modified` error:

```typescript
// Ensure this catch exists in the existing editMessage implementation:
catch (err: any) {
  if (!err.message?.includes('message is not modified')) throw err;
}
```

New method:

```typescript
async sendTyping(chatId: string): Promise<void> {
  try {
    await this.bot.sendChatAction(chatId, 'typing');
  } catch {
    // Non-critical; swallow errors
  }
}
```

### channels/discord.ts

Existing `editMessage(chatId, messageId, message)` — no changes needed.

New method:

```typescript
async sendTyping(chatId: string): Promise<void> {
  try {
    const channel = await this.client.channels.fetch(chatId);
    if (channel?.isTextBased()) await channel.sendTyping();
  } catch {
    // Non-critical; swallow errors
  }
}
```

### channels/feishu.ts

Existing `editMessage` is a TODO stub — implement with CardKit v2 PATCH:

```typescript
async editMessage(chatId: string, messageId: string, message: OutboundMessage): Promise<void> {
  await this.client.im.message.patch({
    path: { message_id: messageId },
    data: {
      msg_type: 'interactive',
      content: this.buildCard(message.text ?? message.html ?? ''),
    },
  });
}

async sendTyping(_chatId: string): Promise<void> {
  // Feishu has no native typing API; streaming card updates serve this purpose
}
```

### engine/bridge-manager.ts

Key changes to `handleInboundMessage`:

```typescript
// In handleInboundMessage, replace the existing processMessage block:

// 1. Start typing heartbeat
const typingInterval = setInterval(() => {
  adapter.sendTyping(msg.chatId);
}, 4000);
adapter.sendTyping(msg.chatId);  // immediate first call

// 2. Get verbose level from in-memory state
const verboseLevel = this.getVerboseLevel(msg.channelType, msg.chatId);

// 3. Create StreamController + CostTracker
const costTracker = new CostTracker();
costTracker.start();
const platformLimits: Record<string, number> = { telegram: 4096, discord: 2000, feishu: 30000 };

const stream = new StreamController({
  verboseLevel,
  platformLimit: platformLimits[adapter.channelType] ?? 4096,
  flushCallback: async (content, isEdit) => {
    if (!isEdit) {
      const result = await adapter.send({ chatId: msg.chatId, text: content });
      clearInterval(typingInterval);  // stop typing on first message
      return result.messageId;
    } else {
      await adapter.editMessage(msg.chatId, stream.messageId!, { chatId: msg.chatId, text: content });
    }
  },
});

try {
  // 4. Wire callbacks through StreamController
  const result = await this.engine.processMessage({
    sessionId: binding.sessionId,
    text: msg.text,
    onTextDelta: (delta) => stream.onTextDelta(delta),
    onToolUse: (event) => stream.onToolStart(event.name, event.input),
    onResult: (event) => {
      const stats = costTracker.finish(event.usage);
      stream.onComplete(stats);
    },
    onPermissionRequest: async (req) => {
      await this.broker.forwardPermissionRequest(req, msg.chatId, [adapter]);
    },
    onError: (err) => stream.onError(err),
  });

  // 5. If level 0 (no streaming), deliver final response via delivery layer
  if (verboseLevel === 0 && result.text.trim()) {
    await this.delivery.deliver(adapter, msg.chatId, result.text.trim(), {
      platformLimit: platformLimits[adapter.channelType] ?? 4096,
    });
  }
} finally {
  clearInterval(typingInterval);  // cleanup on error too
  stream.dispose();
}
```

New command handling in `handleCommand`:

```typescript
case '/verbose': {
  const level = parseInt(msg.text.split(' ')[1]) as VerboseLevel;
  if ([0, 1, 2].includes(level)) {
    this.setVerboseLevel(msg.channelType, msg.chatId, level);
    await adapter.send({ chatId: msg.chatId, text: `Verbose level set to ${level}` });
  } else {
    await adapter.send({ chatId: msg.chatId, text: 'Usage: /verbose 0|1|2' });
  }
  return true;
}
case '/new': {
  // Force new session: rebind with a fresh session ID
  const newSessionId = `session-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
  await this.router.rebind(msg.channelType, msg.chatId, newSessionId);
  this.clearLastActive(msg.channelType, msg.chatId);
  await adapter.send({ chatId: msg.chatId, text: '🆕 New session started.' });
  return true;
}
```

Verbose level and lastActive tracking (in-memory, not persisted):

```typescript
private verboseLevels = new Map<string, VerboseLevel>();  // key: `${channelType}:${chatId}`
private lastActive = new Map<string, number>();            // key: same, value: timestamp ms

private getVerboseLevel(channelType: string, chatId: string): VerboseLevel {
  return this.verboseLevels.get(`${channelType}:${chatId}`) ?? 1;
}

private setVerboseLevel(channelType: string, chatId: string, level: VerboseLevel): void {
  this.verboseLevels.set(`${channelType}:${chatId}`, level);
}

private checkAndUpdateLastActive(channelType: string, chatId: string): boolean {
  const key = `${channelType}:${chatId}`;
  const last = this.lastActive.get(key);
  const now = Date.now();
  this.lastActive.set(key, now);
  if (last && (now - last) > 30 * 60 * 1000) return true;  // expired
  return false;
}
```

### engine/router.ts

No changes needed. Session timeout is handled in BridgeManager via `checkAndUpdateLastActive()`:

```
// In handleInboundMessage, before router.resolve:
const expired = this.checkAndUpdateLastActive(msg.channelType, msg.chatId);
if (expired) {
  const newId = `session-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
  await this.router.rebind(msg.channelType, msg.chatId, newId);
}
const binding = await this.router.resolve(msg.channelType, msg.chatId);
```

### providers/claude-sdk.ts

No interface changes needed. The existing SSE stream already emits `text`, `tool_use`, `result` events through `ConversationEngine.processMessage()` callbacks. `StreamController` connects to these via the callback params.

## Verbose Levels

| Level | Tool headers | Input summary | Streaming text | Cost line |
|-------|-------------|---------------|----------------|-----------|
| 0 (quiet) | No | No | No (final only via delivery) | Yes |
| 1 (normal) | Yes | No | Yes | Yes |
| 2 (detailed) | Yes | Yes (best-effort) | Yes | Yes |

User command: send `/verbose 0|1|2` in IM to switch. Stored in-memory in BridgeManager, not persisted. Resets to default (1) on Bridge restart.

### Input Summary (Level 2)

```typescript
function summarizeToolInput(name: string, input: Record<string, any>): string {
  if (!input || Object.keys(input).length === 0) return '';  // fallback to level 1 format
  switch (name) {
    case 'Read': return input.file_path?.split('/').pop() ?? '';
    case 'Edit': case 'Write': return input.file_path?.split('/').pop() ?? '';
    case 'Grep': return `"${input.pattern}" in ${input.path ?? '.'}`;
    case 'Glob': return input.pattern ?? '';
    case 'Bash': return (input.command ?? '').slice(0, 80);
    default: return '';
  }
}
```

### Tool Emoji Mapping

```typescript
const TOOL_EMOJI: Record<string, string> = {
  Read: '📖', Edit: '✏️', Write: '📝',
  Bash: '🖥️', Grep: '🔍', Glob: '📂',
  Agent: '🤖', WebSearch: '🌐', WebFetch: '🌐',
};
// Unknown tools: '🔧'
```

## Streaming Truncation Strategy

During streaming edit, if accumulated content exceeds platform limit:

```
content.length > platformLimit?
    │
    ├── During streaming → truncate: "...\n" + tail (limit - 100) chars
    │   User sees latest output; full content sent via delivery chunking on complete
    │
    └── On complete → existing delivery layer chunking (no truncation)
```

Platform limits: TG 4096, DC 2000, Feishu 30000.

**Known limitation:** Telegram's 4096 limit is in UTF-8 bytes, not characters. Messages with many CJK/emoji characters could exceed the byte limit while under the character count. For v1, we use character count (simpler); if issues arise in practice, switch to `Buffer.byteLength()` for Telegram.

## Typing Indicator

```
User message arrives
    │
    ▼
adapter.sendTyping(chatId)         ← immediate
    │
    ▼
Start heartbeat interval (every 4s)
    │  TG: sendChatAction('typing')  (5s expiry, 4s interval keeps it alive)
    │  DC: channel.sendTyping()      (10s expiry, 4s sufficient)
    │  Feishu: no-op
    │
    ▼
StreamController first flush OR error/completion
    │
    ▼
Clear heartbeat interval (in flushCallback on first send, AND in finally block)
```

`sendTyping` errors are swallowed (non-critical). Heartbeat is always cleared in the `finally` block to prevent leaks on error paths.

## Session Resume

```
User sends message
    │
    ▼
BridgeManager.checkAndUpdateLastActive(channelType, chatId)
    │
    ├── Not expired (< 30min since last message) → continue
    ├── Expired (>= 30min) → router.rebind() with new session ID
    ├── No previous record → continue (first message)
    │
    ▼
Router.resolve(channelType, chatId)
    │
    ├── Existing binding → reuse session
    └── No binding → create new session (existing behavior)

User sends "/new" → BridgeManager.handleCommand:
    → router.rebind() with fresh session ID
    → clear lastActive record
```

Session timeout state (`lastActive` map) is in-memory in BridgeManager, not persisted. Bridge restart = all sessions treated as new.

## Testing Strategy

Framework: Vitest (already used in bridge/). Test files colocated at `bridge/src/**/__tests__/`.

| Feature | Test type | What to test |
|---------|-----------|-------------|
| `StreamController` | Unit | Throttle timing, message composition per verbose level, truncation, flush callback calls |
| `CostTracker` | Unit | Duration calculation, `cost_usd` passthrough, format output |
| `sendTyping` per adapter | Unit | Called with correct params, errors swallowed |
| `editMessage` (Feishu) | Unit | Card build + PATCH call with mock Lark client |
| Session timeout | Unit | `checkAndUpdateLastActive` returns true/false correctly |
| Verbose levels | Unit | `StreamController` output differs per level (0/1/2) |
| `/verbose` and `/new` | Unit | Command routing, state changes, adapter.send called with correct text |
| Typing heartbeat cleanup | Unit | Interval cleared on first flush AND on error |
