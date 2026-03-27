# Message Schema Layer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace stringly-typed SSE pipeline with Zod-validated canonical event system — typed `ReadableStream<CanonicalEvent>` replaces `ReadableStream<string>`.

**Architecture:** New `messages/` module defines Zod schemas and `ClaudeAdapter` class that maps `SDKMessage → CanonicalEvent[]`. The adapter emits typed events directly into a `ReadableStream<CanonicalEvent>`, eliminating the serialize→parse round-trip of `sseEvent()`/`parseSSE()`. Consumers (`conversation.ts`) switch on `event.kind` with full TypeScript narrowing.

**Tech Stack:** TypeScript, Zod (runtime validation), vitest

**Reference:** Design spec at `docs/plans/2026-03-27-message-schema-design.md`

---

## File Structure

| Action | File | Responsibility |
|--------|------|---------------|
| Create | `bridge/src/messages/schema.ts` | Zod schemas + `CanonicalEvent` type export |
| Create | `bridge/src/messages/claude-adapter.ts` | `SDKMessage → CanonicalEvent[]` with thinking/hidden/subagent handling |
| Create | `bridge/src/messages/types.ts` | `SessionMode`, `ProviderBackend` interface (defined, not yet used) |
| Create | `bridge/src/messages/index.ts` | Public exports |
| Create | `bridge/src/__tests__/message-schema.test.ts` | Schema validation tests |
| Create | `bridge/src/__tests__/claude-adapter.test.ts` | Adapter mapping tests |
| Modify | `bridge/src/providers/base.ts` | `StreamChatResult.stream` type → `ReadableStream<CanonicalEvent>` |
| Modify | `bridge/src/providers/claude-sdk.ts` | Use `ClaudeAdapter`, delete `handleMessage`/`sseEvent` |
| Modify | `bridge/src/engine/conversation.ts` | Delete `parseSSE`, consume `CanonicalEvent` directly |
| Modify | `bridge/src/engine/bridge-manager.ts` | Update callback names to match new event kinds |
| Delete | `bridge/src/providers/sse-utils.ts` | Replaced by `messages/` |
| Delete | `bridge/src/__tests__/sse-utils.test.ts` | Replaced by new tests |

---

### Task 1: Install Zod + Create Schema Module

**Files:**
- Modify: `bridge/package.json`
- Create: `bridge/src/messages/schema.ts`
- Create: `bridge/src/__tests__/message-schema.test.ts`

- [ ] **Step 1: Install zod**

```bash
cd /home/y/Project/test/TermLive/bridge && npm install zod
```

- [ ] **Step 2: Write failing tests for schema validation**

```typescript
// bridge/src/__tests__/message-schema.test.ts
import { describe, it, expect } from 'vitest';
import { canonicalEventSchema, type CanonicalEvent } from '../messages/schema.js';

describe('message-schema', () => {
  describe('text events', () => {
    it('validates text_delta', () => {
      const event = { kind: 'text_delta', text: 'hello' };
      const result = canonicalEventSchema.parse(event);
      expect(result.kind).toBe('text_delta');
      expect((result as any).text).toBe('hello');
    });

    it('validates thinking_delta', () => {
      const event = { kind: 'thinking_delta', text: 'reasoning...' };
      const result = canonicalEventSchema.parse(event);
      expect(result.kind).toBe('thinking_delta');
    });

    it('preserves unknown fields (passthrough)', () => {
      const event = { kind: 'text_delta', text: 'hi', futureField: 42 };
      const result = canonicalEventSchema.parse(event);
      expect((result as any).futureField).toBe(42);
    });
  });

  describe('tool events', () => {
    it('validates tool_start', () => {
      const event = { kind: 'tool_start', id: 'tu_1', name: 'Bash', input: { command: 'ls' } };
      const result = canonicalEventSchema.parse(event);
      expect(result.kind).toBe('tool_start');
    });

    it('validates tool_start with parentToolUseId', () => {
      const event = { kind: 'tool_start', id: 'tu_2', name: 'Read', input: {}, parentToolUseId: 'tu_1' };
      const result = canonicalEventSchema.parse(event);
      expect((result as any).parentToolUseId).toBe('tu_1');
    });

    it('validates tool_result', () => {
      const event = { kind: 'tool_result', toolUseId: 'tu_1', content: 'output', isError: false };
      const result = canonicalEventSchema.parse(event);
      expect(result.kind).toBe('tool_result');
    });

    it('validates tool_progress', () => {
      const event = { kind: 'tool_progress', toolName: 'Bash', elapsed: 5.2 };
      const result = canonicalEventSchema.parse(event);
      expect(result.kind).toBe('tool_progress');
    });
  });

  describe('agent events', () => {
    it('validates agent_start', () => {
      const event = { kind: 'agent_start', description: 'Explore codebase', taskId: 'task_1' };
      const result = canonicalEventSchema.parse(event);
      expect(result.kind).toBe('agent_start');
    });

    it('validates agent_progress', () => {
      const event = { kind: 'agent_progress', description: 'Working...', lastTool: 'Read', usage: { toolUses: 5, durationMs: 3000 } };
      const result = canonicalEventSchema.parse(event);
      expect(result.kind).toBe('agent_progress');
    });

    it('validates agent_complete', () => {
      const event = { kind: 'agent_complete', summary: 'Done', status: 'completed' };
      const result = canonicalEventSchema.parse(event);
      expect(result.kind).toBe('agent_complete');
    });
  });

  describe('query result events', () => {
    it('validates query_result', () => {
      const event = {
        kind: 'query_result', sessionId: 'sess_1', isError: false,
        usage: { inputTokens: 1000, outputTokens: 500, costUsd: 0.05 },
      };
      const result = canonicalEventSchema.parse(event);
      expect(result.kind).toBe('query_result');
    });

    it('validates query_result with permission denials', () => {
      const event = {
        kind: 'query_result', sessionId: 'sess_1', isError: false,
        usage: { inputTokens: 100, outputTokens: 50 },
        permissionDenials: [{ toolName: 'Bash', toolUseId: 'tu_1' }],
      };
      const result = canonicalEventSchema.parse(event);
      expect((result as any).permissionDenials).toHaveLength(1);
    });

    it('validates error', () => {
      const event = { kind: 'error', message: 'something failed' };
      const result = canonicalEventSchema.parse(event);
      expect(result.kind).toBe('error');
    });
  });

  describe('auxiliary events', () => {
    it('validates status', () => {
      const event = { kind: 'status', sessionId: 'sess_1', model: 'claude-sonnet-4-5-20250514' };
      expect(canonicalEventSchema.parse(event).kind).toBe('status');
    });

    it('validates prompt_suggestion', () => {
      const event = { kind: 'prompt_suggestion', suggestion: 'Try running tests' };
      expect(canonicalEventSchema.parse(event).kind).toBe('prompt_suggestion');
    });

    it('validates rate_limit', () => {
      const event = { kind: 'rate_limit', status: 'rejected', utilization: 0.95 };
      expect(canonicalEventSchema.parse(event).kind).toBe('rate_limit');
    });
  });

  describe('validation errors', () => {
    it('rejects unknown kind', () => {
      expect(() => canonicalEventSchema.parse({ kind: 'unknown' })).toThrow();
    });

    it('rejects missing required field', () => {
      expect(() => canonicalEventSchema.parse({ kind: 'text_delta' })).toThrow();
    });
  });
});
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd /home/y/Project/test/TermLive && npx vitest run bridge/src/__tests__/message-schema.test.ts`
Expected: FAIL — module not found

- [ ] **Step 4: Implement schema.ts**

```typescript
// bridge/src/messages/schema.ts
import { z } from 'zod';

// ── Base schema — subagent nesting support ──

const baseSchema = z.object({
  parentToolUseId: z.string().optional(),
});

// ── Text events ──

const textDeltaSchema = z.object({
  kind: z.literal('text_delta'),
  text: z.string(),
}).merge(baseSchema).passthrough();

const thinkingDeltaSchema = z.object({
  kind: z.literal('thinking_delta'),
  text: z.string(),
}).merge(baseSchema).passthrough();

// ── Tool lifecycle ──

const toolStartSchema = z.object({
  kind: z.literal('tool_start'),
  id: z.string(),
  name: z.string(),
  input: z.record(z.unknown()),
}).merge(baseSchema).passthrough();

const toolResultSchema = z.object({
  kind: z.literal('tool_result'),
  toolUseId: z.string(),
  content: z.string(),
  isError: z.boolean(),
}).merge(baseSchema).passthrough();

const toolProgressSchema = z.object({
  kind: z.literal('tool_progress'),
  toolName: z.string(),
  elapsed: z.number(),
}).merge(baseSchema).passthrough();

// ── Agent lifecycle ──

const agentUsageSchema = z.object({
  toolUses: z.number(),
  durationMs: z.number(),
}).passthrough();

const agentStartSchema = z.object({
  kind: z.literal('agent_start'),
  description: z.string(),
  taskId: z.string().optional(),
}).merge(baseSchema).passthrough();

const agentProgressSchema = z.object({
  kind: z.literal('agent_progress'),
  description: z.string(),
  lastTool: z.string().optional(),
  usage: agentUsageSchema.optional(),
}).merge(baseSchema).passthrough();

const agentCompleteSchema = z.object({
  kind: z.literal('agent_complete'),
  summary: z.string(),
  status: z.enum(['completed', 'failed', 'stopped']),
}).merge(baseSchema).passthrough();

// ── Query result ──

const usageSchema = z.object({
  inputTokens: z.number(),
  outputTokens: z.number(),
  costUsd: z.number().optional(),
}).passthrough();

const permissionDenialSchema = z.object({
  toolName: z.string(),
  toolUseId: z.string(),
}).passthrough();

const queryResultSchema = z.object({
  kind: z.literal('query_result'),
  sessionId: z.string(),
  isError: z.boolean(),
  usage: usageSchema,
  permissionDenials: z.array(permissionDenialSchema).optional(),
}).passthrough();

const errorSchema = z.object({
  kind: z.literal('error'),
  message: z.string(),
}).passthrough();

// ── Auxiliary ──

const statusSchema = z.object({
  kind: z.literal('status'),
  sessionId: z.string(),
  model: z.string(),
}).passthrough();

const promptSuggestionSchema = z.object({
  kind: z.literal('prompt_suggestion'),
  suggestion: z.string(),
}).passthrough();

const rateLimitSchema = z.object({
  kind: z.literal('rate_limit'),
  status: z.string(),
  utilization: z.number().optional(),
  resetsAt: z.number().optional(),
}).passthrough();

// ── Discriminated union ──

export const canonicalEventSchema = z.discriminatedUnion('kind', [
  textDeltaSchema,
  thinkingDeltaSchema,
  toolStartSchema,
  toolResultSchema,
  toolProgressSchema,
  agentStartSchema,
  agentProgressSchema,
  agentCompleteSchema,
  queryResultSchema,
  errorSchema,
  statusSchema,
  promptSuggestionSchema,
  rateLimitSchema,
]);

export type CanonicalEvent = z.infer<typeof canonicalEventSchema>;
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /home/y/Project/test/TermLive && npx vitest run bridge/src/__tests__/message-schema.test.ts`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add bridge/package.json bridge/package-lock.json bridge/src/messages/schema.ts bridge/src/__tests__/message-schema.test.ts
git commit -m "feat: add Zod canonical event schemas with passthrough forward compatibility"
```

---

### Task 2: Types Module — SessionMode + ProviderBackend

**Files:**
- Create: `bridge/src/messages/types.ts`
- Create: `bridge/src/messages/index.ts`

These are type definitions only — no implementation code, no tests needed.

- [ ] **Step 1: Create types.ts**

```typescript
// bridge/src/messages/types.ts
import type { CanonicalEvent } from './schema.js';
import type { FileAttachment } from '../providers/base.js';

/** Permission request handler — called by canUseTool */
export type PermissionRequestHandler = (
  toolName: string,
  toolInput: Record<string, unknown>,
  promptSentence: string,
  signal?: AbortSignal,
) => Promise<'allow' | 'allow_always' | 'deny'>;

/** AskUserQuestion handler — returns user's answers */
export type AskUserQuestionHandler = (
  questions: Array<{
    question: string;
    header: string;
    options: Array<{ label: string; description?: string }>;
    multiSelect: boolean;
  }>,
  signal?: AbortSignal,
) => Promise<Record<string, string>>;

/** Controls for an active query */
export interface QueryControls {
  interrupt(): Promise<void>;
  stopTask(taskId: string): Promise<void>;
}

/** Session configuration — consolidates scattered per-chat Maps (used in sub-project 2) */
export interface SessionMode {
  permissionMode: 'default' | 'acceptEdits' | 'plan' | 'bypassPermissions';
  model?: string;
  effort?: 'low' | 'medium' | 'high' | 'max';
  systemPrompt?: string;
  allowedTools?: string[];
  disallowedTools?: string[];
}

/** Provider-agnostic interface (implemented in sub-project 2) */
export interface ProviderBackend {
  startQuery(params: {
    prompt: string;
    workingDirectory: string;
    sessionId?: string;
    mode: SessionMode;
    attachments?: FileAttachment[];
    onPermissionRequest?: PermissionRequestHandler;
    onAskUserQuestion?: AskUserQuestionHandler;
  }): {
    stream: ReadableStream<CanonicalEvent>;
    controls?: QueryControls;
  };

  dispose(): Promise<void>;
}
```

- [ ] **Step 2: Create index.ts**

```typescript
// bridge/src/messages/index.ts
export { canonicalEventSchema, type CanonicalEvent } from './schema.js';
export { ClaudeAdapter } from './claude-adapter.js';
export type {
  SessionMode,
  ProviderBackend,
  PermissionRequestHandler,
  AskUserQuestionHandler,
  QueryControls,
} from './types.js';
```

Note: `ClaudeAdapter` export will resolve after Task 3. For now the import will error — that's fine, we'll create the file next.

- [ ] **Step 3: Commit**

```bash
git add bridge/src/messages/types.ts bridge/src/messages/index.ts
git commit -m "feat: add SessionMode, ProviderBackend types, and messages module index"
```

---

### Task 3: Claude Adapter — SDKMessage → CanonicalEvent

**Files:**
- Create: `bridge/src/messages/claude-adapter.ts`
- Create: `bridge/src/__tests__/claude-adapter.test.ts`

The adapter replaces the `handleMessage()` function currently in `claude-sdk.ts`. It maps SDK message types to canonical events, handles thinking block detection, hidden tool filtering, and subagent nesting.

- [ ] **Step 1: Write failing tests**

```typescript
// bridge/src/__tests__/claude-adapter.test.ts
import { describe, it, expect } from 'vitest';
import { ClaudeAdapter } from '../messages/claude-adapter.js';

describe('ClaudeAdapter', () => {
  function createAdapter() {
    return new ClaudeAdapter();
  }

  describe('stream_event — text deltas', () => {
    it('maps content_block_delta text to text_delta', () => {
      const adapter = createAdapter();
      // First: content_block_start with type 'text'
      adapter.adapt({ type: 'stream_event', event: {
        type: 'content_block_start', index: 0,
        content_block: { type: 'text', text: '' },
      }} as any);
      // Then: content_block_delta
      const events = adapter.adapt({ type: 'stream_event', event: {
        type: 'content_block_delta', index: 0,
        delta: { type: 'text_delta', text: 'hello' },
      }} as any);
      expect(events).toHaveLength(1);
      expect(events[0].kind).toBe('text_delta');
      expect((events[0] as any).text).toBe('hello');
    });

    it('maps thinking block to thinking_delta', () => {
      const adapter = createAdapter();
      adapter.adapt({ type: 'stream_event', event: {
        type: 'content_block_start', index: 0,
        content_block: { type: 'thinking', thinking: '' },
      }} as any);
      const events = adapter.adapt({ type: 'stream_event', event: {
        type: 'content_block_delta', index: 0,
        delta: { type: 'text_delta', text: 'reasoning...' },
      }} as any);
      expect(events).toHaveLength(1);
      expect(events[0].kind).toBe('thinking_delta');
    });
  });

  describe('stream_event — tool_use', () => {
    it('maps content_block_start tool_use to tool_start', () => {
      const adapter = createAdapter();
      const events = adapter.adapt({ type: 'stream_event', event: {
        type: 'content_block_start', index: 1,
        content_block: { type: 'tool_use', id: 'tu_1', name: 'Bash', input: {} },
      }} as any);
      expect(events).toHaveLength(1);
      expect(events[0].kind).toBe('tool_start');
      expect((events[0] as any).id).toBe('tu_1');
      expect((events[0] as any).name).toBe('Bash');
    });

    it('filters hidden tools (ToolSearch, TaskCreate, etc.)', () => {
      const adapter = createAdapter();
      const events = adapter.adapt({ type: 'stream_event', event: {
        type: 'content_block_start', index: 0,
        content_block: { type: 'tool_use', id: 'tu_h', name: 'ToolSearch', input: {} },
      }} as any);
      expect(events).toHaveLength(0);
    });
  });

  describe('assistant message — fallback', () => {
    it('emits tool_start for tool_use blocks', () => {
      const adapter = createAdapter();
      const events = adapter.adapt({ type: 'assistant', message: {
        content: [{ type: 'tool_use', id: 'tu_1', name: 'Read', input: { file_path: '/a.ts' } }],
      }} as any);
      expect(events).toHaveLength(1);
      expect(events[0].kind).toBe('tool_start');
    });

    it('emits text_delta for text blocks when no streaming', () => {
      const adapter = createAdapter();
      const events = adapter.adapt({ type: 'assistant', message: {
        content: [{ type: 'text', text: 'Hello world' }],
      }} as any);
      expect(events).toHaveLength(1);
      expect(events[0].kind).toBe('text_delta');
    });

    it('skips text blocks if already streamed', () => {
      const adapter = createAdapter();
      // Simulate having streamed text via stream_event
      adapter.adapt({ type: 'stream_event', event: {
        type: 'content_block_start', index: 0,
        content_block: { type: 'text', text: '' },
      }} as any);
      adapter.adapt({ type: 'stream_event', event: {
        type: 'content_block_delta', index: 0,
        delta: { type: 'text_delta', text: 'streamed' },
      }} as any);
      // Now assistant message — text should be skipped
      const events = adapter.adapt({ type: 'assistant', message: {
        content: [{ type: 'text', text: 'streamed' }],
      }} as any);
      expect(events).toHaveLength(0);
    });

    it('filters hidden tools in assistant message', () => {
      const adapter = createAdapter();
      const events = adapter.adapt({ type: 'assistant', message: {
        content: [{ type: 'tool_use', id: 'tu_h', name: 'TodoWrite', input: {} }],
      }} as any);
      expect(events).toHaveLength(0);
    });
  });

  describe('user message — tool_result', () => {
    it('maps tool_result blocks', () => {
      const adapter = createAdapter();
      const events = adapter.adapt({ type: 'user', message: {
        content: [{ type: 'tool_result', tool_use_id: 'tu_1', content: 'ok', is_error: false }],
      }} as any);
      expect(events).toHaveLength(1);
      expect(events[0].kind).toBe('tool_result');
      expect((events[0] as any).toolUseId).toBe('tu_1');
    });

    it('filters tool_result for hidden tools', () => {
      const adapter = createAdapter();
      // First register hidden tool
      adapter.adapt({ type: 'stream_event', event: {
        type: 'content_block_start', index: 0,
        content_block: { type: 'tool_use', id: 'tu_h', name: 'TaskCreate', input: {} },
      }} as any);
      // Then tool_result for it
      const events = adapter.adapt({ type: 'user', message: {
        content: [{ type: 'tool_result', tool_use_id: 'tu_h', content: 'done' }],
      }} as any);
      expect(events).toHaveLength(0);
    });
  });

  describe('result message', () => {
    it('maps success result to query_result', () => {
      const adapter = createAdapter();
      const events = adapter.adapt({
        type: 'result', subtype: 'success', session_id: 'sess_1', is_error: false,
        usage: { input_tokens: 1000, output_tokens: 500 }, total_cost_usd: 0.05,
      } as any);
      expect(events).toHaveLength(1);
      expect(events[0].kind).toBe('query_result');
      expect((events[0] as any).usage.inputTokens).toBe(1000);
      expect((events[0] as any).usage.costUsd).toBe(0.05);
    });

    it('maps error result to error event', () => {
      const adapter = createAdapter();
      const events = adapter.adapt({
        type: 'result', subtype: 'error', errors: ['timeout'],
      } as any);
      expect(events).toHaveLength(1);
      expect(events[0].kind).toBe('error');
    });
  });

  describe('system messages', () => {
    it('maps init to status', () => {
      const adapter = createAdapter();
      const events = adapter.adapt({
        type: 'system', subtype: 'init', session_id: 'sess_1', model: 'claude-sonnet-4-5-20250514',
      } as any);
      expect(events).toHaveLength(1);
      expect(events[0].kind).toBe('status');
    });

    it('maps task_started to agent_start', () => {
      const adapter = createAdapter();
      const events = adapter.adapt({
        type: 'system', subtype: 'task_started', description: 'Explore code',
      } as any);
      expect(events).toHaveLength(1);
      expect(events[0].kind).toBe('agent_start');
    });

    it('maps task_progress to agent_progress', () => {
      const adapter = createAdapter();
      const events = adapter.adapt({
        type: 'system', subtype: 'task_progress', summary: 'Reading files', last_tool_name: 'Read',
        usage: { tool_uses: 5, duration_ms: 3000 },
      } as any);
      expect(events).toHaveLength(1);
      expect(events[0].kind).toBe('agent_progress');
      expect((events[0] as any).usage.toolUses).toBe(5);
    });

    it('maps task_notification to agent_complete', () => {
      const adapter = createAdapter();
      const events = adapter.adapt({
        type: 'system', subtype: 'task_notification', summary: 'Done', status: 'completed',
      } as any);
      expect(events).toHaveLength(1);
      expect(events[0].kind).toBe('agent_complete');
    });
  });

  describe('other messages', () => {
    it('maps prompt_suggestion', () => {
      const adapter = createAdapter();
      const events = adapter.adapt({ type: 'prompt_suggestion', suggestion: 'Try this' } as any);
      expect(events).toHaveLength(1);
      expect(events[0].kind).toBe('prompt_suggestion');
    });

    it('maps tool_progress (>3s only)', () => {
      const adapter = createAdapter();
      const events = adapter.adapt({
        type: 'tool_progress', tool_name: 'Bash', elapsed_time_seconds: 5,
      } as any);
      expect(events).toHaveLength(1);
      expect(events[0].kind).toBe('tool_progress');
    });

    it('filters tool_progress under 3s', () => {
      const adapter = createAdapter();
      const events = adapter.adapt({
        type: 'tool_progress', tool_name: 'Bash', elapsed_time_seconds: 2,
      } as any);
      expect(events).toHaveLength(0);
    });

    it('maps rate_limit_event', () => {
      const adapter = createAdapter();
      const events = adapter.adapt({
        type: 'rate_limit_event', rate_limit_info: { status: 'rejected', utilization: 0.95 },
      } as any);
      expect(events).toHaveLength(1);
      expect(events[0].kind).toBe('rate_limit');
    });

    it('returns empty for unknown message type', () => {
      const adapter = createAdapter();
      const events = adapter.adapt({ type: 'unknown_future_type' } as any);
      expect(events).toHaveLength(0);
    });
  });

  describe('subagent nesting', () => {
    it('sets parentToolUseId from SDK message', () => {
      const adapter = createAdapter();
      const events = adapter.adapt({
        type: 'stream_event', parent_tool_use_id: 'parent_tu',
        event: { type: 'content_block_start', index: 0, content_block: { type: 'text', text: '' } },
      } as any);
      // No direct event from content_block_start of text, but state is set
      const textEvents = adapter.adapt({
        type: 'stream_event', parent_tool_use_id: 'parent_tu',
        event: { type: 'content_block_delta', index: 0, delta: { type: 'text_delta', text: 'sub' } },
      } as any);
      expect(textEvents).toHaveLength(1);
      expect((textEvents[0] as any).parentToolUseId).toBe('parent_tu');
    });
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/y/Project/test/TermLive && npx vitest run bridge/src/__tests__/claude-adapter.test.ts`
Expected: FAIL — module not found

- [ ] **Step 3: Implement claude-adapter.ts**

```typescript
// bridge/src/messages/claude-adapter.ts
import { canonicalEventSchema, type CanonicalEvent } from './schema.js';
import type { SDKMessage, SDKPermissionDenial } from '@anthropic-ai/claude-agent-sdk';

const HIDDEN_TOOLS = new Set([
  'ToolSearch', 'TodoRead', 'TodoWrite',
  'TaskCreate', 'TaskUpdate', 'TaskList', 'TaskGet', 'TaskStop', 'TaskOutput',
]);

export class ClaudeAdapter {
  private currentBlockType: string | undefined;
  private hasStreamedText = false;
  private hiddenToolUseIds = new Set<string>();

  adapt(msg: SDKMessage): CanonicalEvent[] {
    const parentToolUseId = this.getParentToolUseId(msg);

    switch (msg.type) {
      case 'stream_event':
        return this.adaptStreamEvent(msg.event, parentToolUseId);
      case 'assistant':
        return this.adaptAssistant(msg, parentToolUseId);
      case 'user':
        return this.adaptUser(msg, parentToolUseId);
      case 'result':
        return this.adaptResult(msg);
      case 'system':
        return this.adaptSystem(msg, parentToolUseId);
      case 'tool_progress':
        return this.adaptToolProgress(msg, parentToolUseId);
      case 'rate_limit_event':
        return this.adaptRateLimit(msg);
      case 'prompt_suggestion':
        return this.adaptPromptSuggestion(msg);
      default:
        console.warn(`[claude-adapter] Unknown SDK message type: ${msg.type}`);
        return [];
    }
  }

  private getParentToolUseId(msg: SDKMessage): string | undefined {
    if ('parent_tool_use_id' in msg && msg.parent_tool_use_id) {
      return msg.parent_tool_use_id as string;
    }
    return undefined;
  }

  private validate(event: CanonicalEvent): CanonicalEvent {
    return canonicalEventSchema.parse(event) as CanonicalEvent;
  }

  private adaptStreamEvent(event: any, parentToolUseId?: string): CanonicalEvent[] {
    if (event.type === 'content_block_start') {
      this.currentBlockType = event.content_block?.type;
      if (event.content_block?.type === 'tool_use') {
        const name = event.content_block.name as string;
        const id = event.content_block.id as string;
        if (HIDDEN_TOOLS.has(name)) {
          this.hiddenToolUseIds.add(id);
          return [];
        }
        return [this.validate({
          kind: 'tool_start', id, name,
          input: event.content_block.input ?? {},
          ...(parentToolUseId ? { parentToolUseId } : {}),
        })];
      }
      return [];
    }

    if (event.type === 'content_block_delta') {
      if (event.delta?.type === 'text_delta') {
        const kind = this.currentBlockType === 'thinking' ? 'thinking_delta' as const : 'text_delta' as const;
        if (kind === 'text_delta') this.hasStreamedText = true;
        return [this.validate({
          kind, text: event.delta.text,
          ...(parentToolUseId ? { parentToolUseId } : {}),
        })];
      }
    }

    return [];
  }

  private adaptAssistant(msg: SDKMessage, parentToolUseId?: string): CanonicalEvent[] {
    const content = (msg as any).message?.content;
    if (!Array.isArray(content)) return [];

    const events: CanonicalEvent[] = [];
    for (const block of content) {
      if (block.type === 'tool_use') {
        if (HIDDEN_TOOLS.has(block.name)) {
          this.hiddenToolUseIds.add(block.id);
          continue;
        }
        events.push(this.validate({
          kind: 'tool_start', id: block.id, name: block.name,
          input: block.input ?? {},
          ...(parentToolUseId ? { parentToolUseId } : {}),
        }));
      } else if (block.type === 'text' && block.text && !this.hasStreamedText) {
        this.hasStreamedText = true;
        events.push(this.validate({
          kind: 'text_delta', text: block.text,
          ...(parentToolUseId ? { parentToolUseId } : {}),
        }));
      }
    }
    return events;
  }

  private adaptUser(msg: SDKMessage, parentToolUseId?: string): CanonicalEvent[] {
    const content = (msg as any).message?.content;
    if (!Array.isArray(content)) return [];

    const events: CanonicalEvent[] = [];
    for (const block of content) {
      if (typeof block === 'object' && block !== null && 'type' in block && block.type === 'tool_result') {
        const rb = block as { tool_use_id: string; content?: unknown; is_error?: boolean };
        if (this.hiddenToolUseIds.has(rb.tool_use_id)) continue;
        events.push(this.validate({
          kind: 'tool_result',
          toolUseId: rb.tool_use_id,
          content: typeof rb.content === 'string' ? rb.content : JSON.stringify(rb.content ?? ''),
          isError: rb.is_error || false,
          ...(parentToolUseId ? { parentToolUseId } : {}),
        }));
      }
    }
    return events;
  }

  private adaptResult(msg: SDKMessage): CanonicalEvent[] {
    const m = msg as any;
    const denials = Array.isArray(m.permission_denials)
      ? (m.permission_denials as SDKPermissionDenial[]).map(d => ({
          toolName: d.tool_name, toolUseId: d.tool_use_id,
        }))
      : undefined;

    if (denials?.length) {
      console.warn(`[claude-adapter] Permission denials:`, denials.map(d => `${d.toolName}(${d.toolUseId})`).join(', '));
    }

    if (m.subtype === 'success') {
      return [this.validate({
        kind: 'query_result',
        sessionId: m.session_id ?? '',
        isError: m.is_error ?? false,
        usage: {
          inputTokens: m.usage?.input_tokens ?? 0,
          outputTokens: m.usage?.output_tokens ?? 0,
          costUsd: m.total_cost_usd,
        },
        ...(denials?.length ? { permissionDenials: denials } : {}),
      })];
    }

    const errors = Array.isArray(m.errors) ? m.errors.join('; ') : 'Unknown error';
    const denialInfo = denials?.length ? ` [denied: ${denials.map(d => d.toolName).join(', ')}]` : '';
    return [this.validate({ kind: 'error', message: errors + denialInfo })];
  }

  private adaptSystem(msg: SDKMessage, parentToolUseId?: string): CanonicalEvent[] {
    const m = msg as any;
    const base = parentToolUseId ? { parentToolUseId } : {};

    if (m.subtype === 'init') {
      return [this.validate({ kind: 'status', sessionId: m.session_id ?? '', model: m.model ?? '', ...base })];
    }
    if (m.subtype === 'task_started') {
      return [this.validate({ kind: 'agent_start', description: m.description || 'Agent', taskId: m.task_id, ...base })];
    }
    if (m.subtype === 'task_progress') {
      return [this.validate({
        kind: 'agent_progress',
        description: m.summary || m.description || 'Working...',
        lastTool: m.last_tool_name,
        usage: m.usage ? { toolUses: m.usage.tool_uses, durationMs: m.usage.duration_ms } : undefined,
        ...base,
      })];
    }
    if (m.subtype === 'task_notification') {
      return [this.validate({
        kind: 'agent_complete',
        summary: m.summary || 'Done',
        status: m.status || 'completed',
        ...base,
      })];
    }
    return [];
  }

  private adaptToolProgress(msg: SDKMessage, parentToolUseId?: string): CanonicalEvent[] {
    const m = msg as any;
    if (m.tool_name && m.elapsed_time_seconds && m.elapsed_time_seconds > 3) {
      return [this.validate({
        kind: 'tool_progress',
        toolName: m.tool_name,
        elapsed: m.elapsed_time_seconds,
        ...(parentToolUseId ? { parentToolUseId } : {}),
      })];
    }
    return [];
  }

  private adaptRateLimit(msg: SDKMessage): CanonicalEvent[] {
    const m = msg as any;
    const info = m.rate_limit_info;
    if (info?.status === 'rejected' || info?.status === 'allowed_warning') {
      return [this.validate({
        kind: 'rate_limit',
        status: info.status,
        utilization: info.utilization,
        resetsAt: info.resetsAt,
      })];
    }
    return [];
  }

  private adaptPromptSuggestion(msg: SDKMessage): CanonicalEvent[] {
    const m = msg as any;
    if (m.suggestion) {
      return [this.validate({ kind: 'prompt_suggestion', suggestion: m.suggestion })];
    }
    return [];
  }
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/y/Project/test/TermLive && npx vitest run bridge/src/__tests__/claude-adapter.test.ts`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add bridge/src/messages/claude-adapter.ts bridge/src/__tests__/claude-adapter.test.ts
git commit -m "feat: add ClaudeAdapter — SDKMessage to CanonicalEvent mapping with thinking/hidden/subagent support"
```

---

### Task 4: Update providers/base.ts — Stream Type Change

**Files:**
- Modify: `bridge/src/providers/base.ts`

Change `ReadableStream<string>` → `ReadableStream<CanonicalEvent>` in `StreamChatResult`. Remove old SSE event type interfaces that are now replaced by canonical events.

- [ ] **Step 1: Update base.ts**

Replace the entire file content. The new version:
- Imports `CanonicalEvent` from messages module
- Changes `StreamChatResult.stream` to `ReadableStream<CanonicalEvent>`
- Keeps `FileAttachment`, `StreamChatParams`, `LLMProvider` (with updated stream type)
- Removes old SSE event types (`TextEvent`, `ToolUseEvent`, `ToolResultEvent`, `PermissionRequestEvent`, `ResultEvent`, `ErrorEvent`, `SSEEvent`) — these are replaced by `CanonicalEvent`

Read the current `bridge/src/providers/base.ts` first, then apply edits:

Replace:
```typescript
export interface StreamChatResult {
  stream: ReadableStream<string>;
  controls?: QueryControls;
}
```
with:
```typescript
import type { CanonicalEvent } from '../messages/schema.js';

export interface StreamChatResult {
  stream: ReadableStream<CanonicalEvent>;
  controls?: QueryControls;
}
```

And delete all SSE event type interfaces (`TextEvent`, `ToolUseEvent`, etc.) and the `SSEEvent` union type — they are no longer used.

- [ ] **Step 2: Run build to check types compile**

```bash
cd /home/y/Project/test/TermLive && npx tsc --noEmit -p bridge/tsconfig.json 2>&1 | head -30
```

Expected: Type errors in `claude-sdk.ts` and `conversation.ts` (they still use old types). This is expected — we'll fix those in Tasks 5 and 6.

- [ ] **Step 3: Commit**

```bash
git add bridge/src/providers/base.ts
git commit -m "feat: change StreamChatResult.stream to ReadableStream<CanonicalEvent>, remove SSE types"
```

---

### Task 5: Update claude-sdk.ts — Use ClaudeAdapter

**Files:**
- Modify: `bridge/src/providers/claude-sdk.ts`

Replace the `handleMessage()` function and `sseEvent()` calls with `ClaudeAdapter`. The `ReadableStream` now enqueues `CanonicalEvent` objects instead of SSE strings.

- [ ] **Step 1: Read current claude-sdk.ts and apply changes**

Key changes:
1. Import `ClaudeAdapter` and `CanonicalEvent`
2. Remove `import { sseEvent } from './sse-utils.js'`
3. Change `ReadableStream<string>` → `ReadableStream<CanonicalEvent>`
4. Change `ReadableStreamDefaultController<string>` → `ReadableStreamDefaultController<CanonicalEvent>`
5. Replace `handleMessage(msg, controller, state)` with adapter-based mapping
6. Delete the standalone `handleMessage()` function at bottom of file

The core change in `streamChat()`:
```typescript
// Before:
const stream = new ReadableStream<string>({
  start(controller) {
    (async () => {
      // ...
      for await (const msg of q) {
        handleMessage(msg, controller, state);
      }
      controller.close();
    })();
  },
});

// After:
const adapter = new ClaudeAdapter();
const stream = new ReadableStream<CanonicalEvent>({
  start(controller) {
    (async () => {
      // ...
      for await (const msg of q) {
        const sub = 'subtype' in msg ? `.${msg.subtype}` : '';
        const turns = 'num_turns' in msg ? ` turns=${msg.num_turns}` : '';
        console.log(`[claude-sdk] msg: ${msg.type}${sub}${turns}`);

        const events = adapter.adapt(msg);
        for (const event of events) {
          controller.enqueue(event);
        }

        // Track result state for teardown noise detection
        if (msg.type === 'result') state.hasReceivedResult = true;
        if (events.some(e => e.kind === 'text_delta')) state.hasStreamedText = true;
      }
      controller.close();
    })();
  },
});
```

Error handling also changes — `sseEvent('error', message)` → `controller.enqueue({ kind: 'error', message } as CanonicalEvent)`.

- [ ] **Step 2: Run adapter + schema tests**

```bash
cd /home/y/Project/test/TermLive && npx vitest run bridge/src/__tests__/claude-adapter.test.ts bridge/src/__tests__/message-schema.test.ts
```

Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add bridge/src/providers/claude-sdk.ts
git commit -m "feat: replace handleMessage/sseEvent with ClaudeAdapter in claude-sdk.ts"
```

---

### Task 6: Update conversation.ts — Consume CanonicalEvent

**Files:**
- Modify: `bridge/src/engine/conversation.ts`
- Modify: `bridge/src/__tests__/conversation.test.ts`

Replace `parseSSE` consumption with direct `CanonicalEvent` switch. Update `ProcessMessageParams` callback signatures.

- [ ] **Step 1: Update conversation.ts**

Key changes:
1. Remove `import { parseSSE } from '../providers/sse-utils.js'`
2. Remove `import type { SSEEvent, ToolUseEvent, PermissionRequestEvent, ResultEvent } from '../providers/base.js'`
3. Add `import type { CanonicalEvent } from '../messages/schema.js'`
4. Update `ProcessMessageParams` — rename callbacks, update types
5. Replace the stream consumption loop

New `ProcessMessageParams`:
```typescript
interface ProcessMessageParams {
  sessionId: string;
  text: string;
  attachments?: FileAttachment[];
  onTextDelta?: (delta: string) => void;
  onToolStart?: (event: { id: string; name: string; input: Record<string, unknown> }) => void;
  onToolResult?: (event: { toolUseId: string; content: string; isError: boolean }) => void;
  onQueryResult?: (event: {
    sessionId: string; isError: boolean;
    usage: { inputTokens: number; outputTokens: number; costUsd?: number };
    permissionDenials?: Array<{ toolName: string; toolUseId: string }>;
  }) => void;
  onError?: (error: string) => void;
  onAgentStart?: (data: { description: string; taskId?: string }) => void;
  onAgentProgress?: (data: { description: string; lastTool?: string; usage?: { toolUses: number; durationMs: number } }) => void;
  onAgentComplete?: (data: { summary: string; status: string }) => void;
  onPromptSuggestion?: (suggestion: string) => void;
  onToolProgress?: (data: { toolName: string; elapsed: number }) => void;
  onRateLimit?: (data: { status: string; utilization?: number; resetsAt?: number }) => void;
  onControls?: (controls: QueryControls) => void;
  sdkPermissionHandler?: PermissionRequestHandler;
  effort?: 'low' | 'medium' | 'high' | 'max';
}
```

New stream consumption loop:
```typescript
const reader = result.stream.getReader();
while (true) {
  const { done, value } = await reader.read();
  if (done) break;

  switch (value.kind) {
    case 'text_delta':
      fullText += value.text;
      params.onTextDelta?.(value.text);
      break;
    case 'thinking_delta':
      // Not accumulated into fullText — thinking is internal
      break;
    case 'tool_start':
      params.onToolStart?.(value);
      break;
    case 'tool_result':
      params.onToolResult?.(value);
      break;
    case 'query_result': {
      usage = value.usage;
      if (value.sessionId) {
        const existing = await store.getSession(params.sessionId);
        await store.saveSession({
          id: params.sessionId,
          workingDirectory: existing?.workingDirectory ?? defaultWorkdir,
          createdAt: existing?.createdAt ?? new Date().toISOString(),
          sdkSessionId: value.sessionId,
        });
      }
      params.onQueryResult?.(value);
      break;
    }
    case 'agent_start':
      params.onAgentStart?.(value);
      break;
    case 'agent_progress':
      params.onAgentProgress?.(value);
      break;
    case 'agent_complete':
      params.onAgentComplete?.(value);
      break;
    case 'prompt_suggestion':
      params.onPromptSuggestion?.(value.suggestion);
      break;
    case 'tool_progress':
      params.onToolProgress?.(value);
      break;
    case 'rate_limit':
      params.onRateLimit?.(value);
      break;
    case 'error':
      params.onError?.(value.message);
      break;
  }
}
```

Also update the `usage` variable type to match canonical format: `{ inputTokens: number; outputTokens: number; costUsd?: number }`, and update `ProcessMessageResult` accordingly.

- [ ] **Step 2: Update conversation tests**

Update `bridge/src/__tests__/conversation.test.ts` to use new event kinds and callback names.

- [ ] **Step 3: Run tests**

```bash
cd /home/y/Project/test/TermLive && npx vitest run bridge/src/__tests__/conversation.test.ts
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add bridge/src/engine/conversation.ts bridge/src/__tests__/conversation.test.ts
git commit -m "feat: consume CanonicalEvent stream in conversation engine, delete parseSSE usage"
```

---

### Task 7: Update bridge-manager.ts — Match New Event Kinds

**Files:**
- Modify: `bridge/src/engine/bridge-manager.ts`
- Modify: `bridge/src/__tests__/bridge-manager.test.ts`

Update callback wiring to match renamed events from conversation.ts.

- [ ] **Step 1: Read bridge-manager.ts and update callback names**

Key changes in the `processMessage()` call (around line 724-779):
```typescript
// Old → New
onToolUse: (event) => ...        →  onToolStart: (event) => {
                                       const rendererToolId = renderer.onToolStart(event.name, event.input);
                                       if (event.id) toolIdMap.set(event.id, rendererToolId);
                                     },
onToolResult: (event) => ...     →  onToolResult: (event) => {
                                       const rendererToolId = toolIdMap.get(event.toolUseId) ?? event.toolUseId;
                                       renderer.onToolComplete(rendererToolId, event.content, event.isError);
                                     },
onResult: (event) => ...         →  onQueryResult: (event) => {
                                       if (event.permissionDenials?.length) { ... }
                                       const usage = { input_tokens: event.usage.inputTokens, output_tokens: event.usage.outputTokens, cost_usd: event.usage.costUsd };
                                       completedStats = costTracker.finish(usage);
                                       if (verboseLevel > 0) renderer.onComplete(completedStats);
                                     },
onAgentProgress: (data) => ...   →  (stays same, but field names change: data.usage?.toolUses, data.usage?.durationMs)
onToolProgress: (data) => ...    →  (stays same)
onRateLimit: (data) => ...       →  (stays same)
```

Also add `onAgentStart` callback:
```typescript
onAgentStart: (data) => renderer.onAgentStart(data.description),
```

- [ ] **Step 2: Update bridge-manager tests**

Update any test mocks/assertions for renamed callbacks.

- [ ] **Step 3: Run tests**

```bash
cd /home/y/Project/test/TermLive && npx vitest run bridge/src/__tests__/bridge-manager.test.ts
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add bridge/src/engine/bridge-manager.ts bridge/src/__tests__/bridge-manager.test.ts
git commit -m "feat: update bridge-manager callbacks to match canonical event kinds"
```

---

### Task 8: Delete sse-utils.ts

**Files:**
- Delete: `bridge/src/providers/sse-utils.ts`
- Delete: `bridge/src/__tests__/sse-utils.test.ts`

- [ ] **Step 1: Search for remaining references**

```bash
cd /home/y/Project/test/TermLive && grep -r 'sse-utils\|sseEvent\|parseSSE' bridge/src/ --include='*.ts'
```

Fix any remaining references found.

- [ ] **Step 2: Delete files**

```bash
rm bridge/src/providers/sse-utils.ts bridge/src/__tests__/sse-utils.test.ts
```

- [ ] **Step 3: Run full test suite**

```bash
cd /home/y/Project/test/TermLive && npx vitest run
```

Expected: All tests pass

- [ ] **Step 4: Commit**

```bash
git add -A
git commit -m "refactor: delete sse-utils.ts — replaced by messages/ canonical event system"
```

---

### Task 9: Build + Full Test Suite

**Files:** None

- [ ] **Step 1: Build**

```bash
cd /home/y/Project/test/TermLive && npm run build
```

Expected: No TypeScript errors

- [ ] **Step 2: Full test suite**

```bash
cd /home/y/Project/test/TermLive && npm test
```

Expected: All tests pass

- [ ] **Step 3: Commit if any fixes needed**

```bash
git add -A
git commit -m "fix: resolve integration issues from message schema migration"
```
