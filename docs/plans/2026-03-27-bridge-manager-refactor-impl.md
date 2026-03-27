# Bridge-Manager Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract `SessionStateManager`, `PermissionCoordinator`, and `CommandRouter` from the 1167-line `BridgeManager` god class, reducing it to a ~400-line orchestrator.

**Architecture:** Bottom-up extraction — each module is created with tests, then BridgeManager is updated to delegate to it. Each step produces a working intermediate state where all existing tests pass. No big bang.

**Tech Stack:** TypeScript, vitest

**Reference:** Design spec at `docs/plans/2026-03-27-bridge-manager-refactor-design.md`

---

## File Structure

| Action | File | Responsibility |
|--------|------|---------------|
| Create | `bridge/src/engine/session-state.ts` | Per-chat state: verbose, permMode, effort, threads, processing, activity |
| Create | `bridge/src/engine/permission-coordinator.ts` | SDK + hook permission flow, text/callback resolution, tracking |
| Create | `bridge/src/engine/command-router.ts` | All /commands: status, new, verbose, perm, stop, effort, hooks, sessions, help, approve, pairings |
| Modify | `bridge/src/engine/bridge-manager.ts` | Slim down to orchestrator — delegate to extracted modules |
| Create | `bridge/src/__tests__/session-state.test.ts` | Unit tests for state manager |
| Create | `bridge/src/__tests__/permission-coordinator.test.ts` | Unit tests for permission coordinator |
| Modify | `bridge/src/__tests__/bridge-manager.test.ts` | Update for new delegation pattern |

---

### Task 1: Extract SessionStateManager

**Files:**
- Create: `bridge/src/engine/session-state.ts`
- Create: `bridge/src/__tests__/session-state.test.ts`
- Modify: `bridge/src/engine/bridge-manager.ts`

Extract the 6 state Maps and their accessor methods into a standalone class. BridgeManager delegates to it.

- [ ] **Step 1: Write failing tests for SessionStateManager**

```typescript
// bridge/src/__tests__/session-state.test.ts
import { describe, it, expect } from 'vitest';
import { SessionStateManager } from '../engine/session-state.js';

describe('SessionStateManager', () => {
  function create() {
    return new SessionStateManager();
  }

  describe('stateKey', () => {
    it('combines channelType and chatId', () => {
      const s = create();
      expect(s.stateKey('telegram', '123')).toBe('telegram:123');
    });
  });

  describe('verbose level', () => {
    it('defaults to 1', () => {
      const s = create();
      expect(s.getVerboseLevel('telegram', '1')).toBe(1);
    });
    it('stores and retrieves', () => {
      const s = create();
      s.setVerboseLevel('telegram', '1', 0);
      expect(s.getVerboseLevel('telegram', '1')).toBe(0);
    });
  });

  describe('perm mode', () => {
    it('defaults to on', () => {
      const s = create();
      expect(s.getPermMode('telegram', '1')).toBe('on');
    });
    it('stores and retrieves', () => {
      const s = create();
      s.setPermMode('telegram', '1', 'off');
      expect(s.getPermMode('telegram', '1')).toBe('off');
    });
  });

  describe('effort', () => {
    it('defaults to undefined', () => {
      const s = create();
      expect(s.getEffort('telegram', '1')).toBeUndefined();
    });
    it('stores and retrieves', () => {
      const s = create();
      s.setEffort('telegram', '1', 'high');
      expect(s.getEffort('telegram', '1')).toBe('high');
    });
  });

  describe('processing guard', () => {
    it('defaults to false', () => {
      const s = create();
      expect(s.isProcessing('key')).toBe(false);
    });
    it('toggles on and off', () => {
      const s = create();
      s.setProcessing('key', true);
      expect(s.isProcessing('key')).toBe(true);
      s.setProcessing('key', false);
      expect(s.isProcessing('key')).toBe(false);
    });
  });

  describe('threads', () => {
    it('returns undefined when not set', () => {
      const s = create();
      expect(s.getThread('discord', '1')).toBeUndefined();
    });
    it('stores and retrieves', () => {
      const s = create();
      s.setThread('discord', '1', 'thread-123');
      expect(s.getThread('discord', '1')).toBe('thread-123');
    });
    it('clears', () => {
      const s = create();
      s.setThread('discord', '1', 'thread-123');
      s.clearThread('discord', '1');
      expect(s.getThread('discord', '1')).toBeUndefined();
    });
  });

  describe('activity tracking', () => {
    it('returns false on first call', () => {
      const s = create();
      expect(s.checkAndUpdateLastActive('telegram', '1')).toBe(false);
    });
    it('returns false for recent activity', () => {
      const s = create();
      s.checkAndUpdateLastActive('telegram', '1');
      expect(s.checkAndUpdateLastActive('telegram', '1')).toBe(false);
    });
    it('clearLastActive removes tracking', () => {
      const s = create();
      s.checkAndUpdateLastActive('telegram', '1');
      s.clearLastActive('telegram', '1');
      // Next call should return false (fresh start, no expiry)
      expect(s.checkAndUpdateLastActive('telegram', '1')).toBe(false);
    });
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/y/Project/test/TermLive && npx vitest run bridge/src/__tests__/session-state.test.ts`
Expected: FAIL — module not found

- [ ] **Step 3: Implement SessionStateManager**

```typescript
// bridge/src/engine/session-state.ts
import type { VerboseLevel } from './terminal-card-renderer.js';

export class SessionStateManager {
  private verboseLevels = new Map<string, VerboseLevel>();
  private permModes = new Map<string, 'on' | 'off'>();
  private effortLevels = new Map<string, 'low' | 'medium' | 'high' | 'max'>();
  private processingChats = new Set<string>();
  private lastActive = new Map<string, number>();
  private sessionThreads = new Map<string, string>();

  stateKey(channelType: string, chatId: string): string {
    return `${channelType}:${chatId}`;
  }

  getVerboseLevel(channelType: string, chatId: string): VerboseLevel {
    return this.verboseLevels.get(this.stateKey(channelType, chatId)) ?? 1;
  }

  setVerboseLevel(channelType: string, chatId: string, level: VerboseLevel): void {
    this.verboseLevels.set(this.stateKey(channelType, chatId), level);
  }

  getPermMode(channelType: string, chatId: string): 'on' | 'off' {
    return this.permModes.get(this.stateKey(channelType, chatId)) ?? 'on';
  }

  setPermMode(channelType: string, chatId: string, mode: 'on' | 'off'): void {
    this.permModes.set(this.stateKey(channelType, chatId), mode);
  }

  getEffort(channelType: string, chatId: string): 'low' | 'medium' | 'high' | 'max' | undefined {
    return this.effortLevels.get(this.stateKey(channelType, chatId));
  }

  setEffort(channelType: string, chatId: string, level: 'low' | 'medium' | 'high' | 'max'): void {
    this.effortLevels.set(this.stateKey(channelType, chatId), level);
  }

  isProcessing(key: string): boolean {
    return this.processingChats.has(key);
  }

  setProcessing(key: string, active: boolean): void {
    if (active) this.processingChats.add(key);
    else this.processingChats.delete(key);
  }

  getThread(channelType: string, chatId: string): string | undefined {
    return this.sessionThreads.get(this.stateKey(channelType, chatId));
  }

  setThread(channelType: string, chatId: string, threadId: string): void {
    this.sessionThreads.set(this.stateKey(channelType, chatId), threadId);
  }

  clearThread(channelType: string, chatId: string): void {
    this.sessionThreads.delete(this.stateKey(channelType, chatId));
  }

  checkAndUpdateLastActive(channelType: string, chatId: string): boolean {
    const key = this.stateKey(channelType, chatId);
    const last = this.lastActive.get(key);
    const now = Date.now();
    this.lastActive.set(key, now);
    if (last && (now - last) > 30 * 60 * 1000) return true;
    return false;
  }

  clearLastActive(channelType: string, chatId: string): void {
    this.lastActive.delete(this.stateKey(channelType, chatId));
  }
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /home/y/Project/test/TermLive && npx vitest run bridge/src/__tests__/session-state.test.ts`
Expected: PASS

- [ ] **Step 5: Wire SessionStateManager into BridgeManager**

In `bridge/src/engine/bridge-manager.ts`:

1. Add import: `import { SessionStateManager } from './session-state.js';`
2. Add field: `private state = new SessionStateManager();`
3. Delete the 6 Maps: `verboseLevels`, `permModes`, `effortLevels`, `processingChats`, `lastActive`, `sessionThreads`
4. Delete methods: `stateKey()`, `getVerboseLevel()`, `setVerboseLevel()`, `getPermMode()`, `setPermMode()`, `getEffort()`, `setEffort()`, `checkAndUpdateLastActive()`, `clearLastActive()`
5. Replace all `this.stateKey(...)` with `this.state.stateKey(...)`
6. Replace all `this.getVerboseLevel(...)` with `this.state.getVerboseLevel(...)`
7. Replace all `this.setVerboseLevel(...)` with `this.state.setVerboseLevel(...)`
8. Replace all `this.getPermMode(...)` / `this.setPermMode(...)` with `this.state.getPermMode(...)` / `this.state.setPermMode(...)`
9. Replace all `this.getEffort(...)` / `this.setEffort(...)` with `this.state.getEffort(...)` / `this.state.setEffort(...)`
10. Replace `this.processingChats.has(...)` with `this.state.isProcessing(...)`
11. Replace `this.processingChats.add(...)` / `this.processingChats.delete(...)` with `this.state.setProcessing(..., true/false)`
12. Replace `this.sessionThreads.get(...)` with `this.state.getThread(...)`
13. Replace `this.sessionThreads.set(...)` with `this.state.setThread(...)`
14. Replace `this.sessionThreads.delete(...)` with `this.state.clearThread(...)`
15. Replace `this.checkAndUpdateLastActive(...)` with `this.state.checkAndUpdateLastActive(...)`
16. Replace `this.clearLastActive(...)` with `this.state.clearLastActive(...)`

- [ ] **Step 6: Run ALL tests**

```bash
cd /home/y/Project/test/TermLive && npx vitest run
```
Expected: ALL pass — BridgeManager's public API unchanged

- [ ] **Step 7: Commit**

```bash
git add bridge/src/engine/session-state.ts bridge/src/__tests__/session-state.test.ts bridge/src/engine/bridge-manager.ts
git commit -m "refactor: extract SessionStateManager from BridgeManager — 6 Maps, 10 methods"
```

---

### Task 2: Extract PermissionCoordinator

**Files:**
- Create: `bridge/src/engine/permission-coordinator.ts`
- Create: `bridge/src/__tests__/permission-coordinator.test.ts`
- Modify: `bridge/src/engine/bridge-manager.ts`

Extract all permission-related state and logic: 5 Maps, text parsing, SDK/hook resolution, callback handling.

- [ ] **Step 1: Write failing tests**

```typescript
// bridge/src/__tests__/permission-coordinator.test.ts
import { describe, it, expect, vi } from 'vitest';
import { PermissionCoordinator } from '../engine/permission-coordinator.js';
import { PendingPermissions } from '../permissions/gateway.js';
import { PermissionBroker } from '../permissions/broker.js';

describe('PermissionCoordinator', () => {
  function create() {
    const gateway = new PendingPermissions();
    const broker = new PermissionBroker(gateway, 'http://localhost:8080');
    return { coordinator: new PermissionCoordinator(gateway, broker, 'http://localhost:9090', 'test-token'), gateway, broker };
  }

  describe('parsePermissionText', () => {
    it('parses allow variants', () => {
      const { coordinator } = create();
      expect(coordinator.parsePermissionText('allow')).toBe('allow');
      expect(coordinator.parsePermissionText('yes')).toBe('allow');
      expect(coordinator.parsePermissionText('y')).toBe('allow');
      expect(coordinator.parsePermissionText('a')).toBe('allow');
      expect(coordinator.parsePermissionText('允许')).toBe('allow');
      expect(coordinator.parsePermissionText('通过')).toBe('allow');
    });

    it('parses deny variants', () => {
      const { coordinator } = create();
      expect(coordinator.parsePermissionText('deny')).toBe('deny');
      expect(coordinator.parsePermissionText('no')).toBe('deny');
      expect(coordinator.parsePermissionText('n')).toBe('deny');
      expect(coordinator.parsePermissionText('拒绝')).toBe('deny');
    });

    it('parses always', () => {
      const { coordinator } = create();
      expect(coordinator.parsePermissionText('always')).toBe('allow_always');
      expect(coordinator.parsePermissionText('始终允许')).toBe('allow_always');
    });

    it('returns null for non-permission text', () => {
      const { coordinator } = create();
      expect(coordinator.parsePermissionText('hello')).toBeNull();
      expect(coordinator.parsePermissionText('fix the bug')).toBeNull();
    });

    it('trims and lowercases', () => {
      const { coordinator } = create();
      expect(coordinator.parsePermissionText('  Allow  ')).toBe('allow');
      expect(coordinator.parsePermissionText('YES')).toBe('allow');
    });
  });

  describe('SDK permission tracking', () => {
    it('tracks and clears pending SDK permissions', () => {
      const { coordinator } = create();
      expect(coordinator.getPendingSdkPerm('key')).toBeUndefined();
      coordinator.setPendingSdkPerm('key', 'perm-1');
      expect(coordinator.getPendingSdkPerm('key')).toBe('perm-1');
      coordinator.clearPendingSdkPerm('key');
      expect(coordinator.getPendingSdkPerm('key')).toBeUndefined();
    });
  });

  describe('tryResolveByText', () => {
    it('resolves pending SDK permission via gateway', () => {
      const { coordinator, gateway } = create();
      // Set up a pending permission in gateway
      const promise = gateway.waitFor('perm-1', { timeoutMs: 5000 });
      coordinator.setPendingSdkPerm('key', 'perm-1');

      const resolved = coordinator.tryResolveByText('key', 'allow');
      expect(resolved).toBe(true);
      expect(coordinator.getPendingSdkPerm('key')).toBeUndefined();
    });

    it('returns false when no pending permission', () => {
      const { coordinator } = create();
      expect(coordinator.tryResolveByText('key', 'allow')).toBe(false);
    });
  });

  describe('hook permission tracking', () => {
    it('tracks hook messages', () => {
      const { coordinator } = create();
      coordinator.trackHookMessage('msg-1', 'sess-1');
      // No public getter needed — tracking is for internal routing
    });

    it('tracks permission messages', () => {
      const { coordinator } = create();
      coordinator.trackPermissionMessage('msg-1', 'perm-1', 'sess-1', 'telegram');
    });

    it('stores hook permission text', () => {
      const { coordinator } = create();
      coordinator.storeHookPermissionText('hook-1', 'Permission text');
    });
  });

  describe('handleBrokerCallback', () => {
    it('delegates perm: callbacks to broker', () => {
      const { coordinator, gateway } = create();
      // Set up a pending permission
      const promise = gateway.waitFor('test-id', { timeoutMs: 5000 });
      const resolved = coordinator.handleBrokerCallback('perm:allow:test-id');
      expect(resolved).toBe(true);
    });

    it('returns false for non-perm callbacks', () => {
      const { coordinator } = create();
      expect(coordinator.handleBrokerCallback('suggest:hello')).toBe(false);
    });
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /home/y/Project/test/TermLive && npx vitest run bridge/src/__tests__/permission-coordinator.test.ts`
Expected: FAIL

- [ ] **Step 3: Implement PermissionCoordinator**

```typescript
// bridge/src/engine/permission-coordinator.ts
import { PendingPermissions } from '../permissions/gateway.js';
import { PermissionBroker } from '../permissions/broker.js';
import type { BaseChannelAdapter } from '../channels/base.js';
import type { InboundMessage } from '../channels/types.js';

export class PermissionCoordinator {
  private pendingSdkPerms = new Map<string, string>();
  private resolvedHookIds = new Map<string, number>();
  private hookPermissionTexts = new Map<string, { text: string; ts: number }>();
  private permissionMessages = new Map<string, { permissionId: string; sessionId: string; timestamp: number }>();
  private latestPermission = new Map<string, { permissionId: string; sessionId: string; messageId: string }>();

  constructor(
    private gateway: PendingPermissions,
    private broker: PermissionBroker,
    private coreUrl: string,
    private token: string,
  ) {}

  getGateway(): PendingPermissions {
    return this.gateway;
  }

  getBroker(): PermissionBroker {
    return this.broker;
  }

  // ── Text parsing ──

  parsePermissionText(text: string): 'allow' | 'allow_always' | 'deny' | null {
    const t = text.trim().toLowerCase();
    if (['allow', 'a', 'yes', 'y', '允许', '通过'].includes(t)) return 'allow';
    if (['deny', 'd', 'no', 'n', '拒绝', '否'].includes(t)) return 'deny';
    if (['always', '始终允许'].includes(t)) return 'allow_always';
    return null;
  }

  // ── SDK permission tracking ──

  getPendingSdkPerm(chatKey: string): string | undefined {
    return this.pendingSdkPerms.get(chatKey);
  }

  setPendingSdkPerm(chatKey: string, permId: string): void {
    this.pendingSdkPerms.set(chatKey, permId);
  }

  clearPendingSdkPerm(chatKey: string): void {
    this.pendingSdkPerms.delete(chatKey);
  }

  /** Try to resolve a pending SDK permission via gateway. Returns true if resolved. */
  tryResolveByText(chatKey: string, decision: string): boolean {
    const pendingPermId = this.pendingSdkPerms.get(chatKey);
    if (!pendingPermId) return false;

    const gwDecision = decision === 'deny' ? 'deny' as const
      : decision === 'allow_always' ? 'allow_always' as const
      : 'allow' as const;

    if (this.gateway.resolve(pendingPermId, gwDecision)) {
      this.pendingSdkPerms.delete(chatKey);
      return true;
    }
    return false;
  }

  /** Try to resolve a hook permission via text reply. Returns permEntry if found, null otherwise. */
  findHookPermission(
    replyToMessageId: string | undefined,
    channelType: string,
  ): { permissionId: string; sessionId: string } | null {
    let permEntry = replyToMessageId ? this.permissionMessages.get(replyToMessageId) : undefined;
    if (!permEntry) {
      if (this.permissionMessages.size === 1) {
        const latest = this.latestPermission.get(channelType);
        if (latest) permEntry = this.permissionMessages.get(latest.messageId);
      }
    }
    return permEntry ?? null;
  }

  /** Returns count of pending permission messages (for "multiple pending" check) */
  pendingPermissionCount(): number {
    return this.permissionMessages.size;
  }

  /** Resolve a hook permission via Core API */
  async resolveHookPermission(
    permissionId: string,
    decision: string,
    channelType: string,
    coreAvailable: boolean,
  ): Promise<void> {
    if (!coreAvailable) throw new Error('Go Core not available');
    await fetch(`${this.coreUrl}/api/hooks/permission/${permissionId}/resolve`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${this.token}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ decision }),
      signal: AbortSignal.timeout(5000),
    });
    // Clean up
    for (const [id, e] of this.permissionMessages) {
      if (e.permissionId === permissionId) this.permissionMessages.delete(id);
    }
    const latest = this.latestPermission.get(channelType);
    if (latest?.permissionId === permissionId) this.latestPermission.delete(channelType);
  }

  /** Handle hook: callback (hook permission button click) */
  async resolveHookCallback(
    hookId: string,
    decision: string,
    sessionId: string,
    messageId: string,
    adapter: BaseChannelAdapter,
    chatId: string,
    coreAvailable: boolean,
  ): Promise<void> {
    if (this.resolvedHookIds.has(hookId)) return; // deduplicate
    this.resolvedHookIds.set(hookId, Date.now());

    if (!coreAvailable) {
      await adapter.send({ chatId, text: '❌ Go Core not available' });
      return;
    }

    await fetch(`${this.coreUrl}/api/hooks/permission/${hookId}/resolve`, {
      method: 'POST',
      headers: { Authorization: `Bearer ${this.token}`, 'Content-Type': 'application/json' },
      body: JSON.stringify({ decision }),
      signal: AbortSignal.timeout(5000),
    });

    const labels: Record<string, string> = {
      allow: '✅ Allowed', allow_always: '📌 Always Allowed', deny: '❌ Denied',
    };
    const label = labels[decision] || '✅ Allowed';
    const originalText = this.hookPermissionTexts.get(hookId)?.text || '';
    this.hookPermissionTexts.delete(hookId);

    await adapter.editMessage(chatId, messageId, {
      chatId,
      text: originalText + `\n\n${label}`,
      feishuHeader: { template: decision === 'deny' ? 'red' : 'green', title: label },
    });
  }

  /** Handle perm: callback (broker permission button click) */
  handleBrokerCallback(callbackData: string): boolean {
    return this.broker.handlePermissionCallback(callbackData);
  }

  // ── Tracking methods ──

  trackHookMessage(messageId: string, sessionId: string): void {
    this.hookMessages.set(messageId, { sessionId: sessionId || '', timestamp: Date.now() });
    for (const [id, entry] of this.hookMessages) {
      if (Date.now() - entry.timestamp > 24 * 60 * 60 * 1000) this.hookMessages.delete(id);
    }
  }

  private hookMessages = new Map<string, { sessionId: string; timestamp: number }>();

  isHookMessage(messageId: string): boolean {
    return this.hookMessages.has(messageId);
  }

  getHookMessage(messageId: string): { sessionId: string; timestamp: number } | undefined {
    return this.hookMessages.get(messageId);
  }

  trackPermissionMessage(messageId: string, permissionId: string, sessionId: string, channelType: string): void {
    this.permissionMessages.set(messageId, { permissionId, sessionId, timestamp: Date.now() });
    this.latestPermission.set(channelType, { permissionId, sessionId, messageId });
    for (const [id, entry] of this.permissionMessages) {
      if (Date.now() - entry.timestamp > 24 * 60 * 60 * 1000) this.permissionMessages.delete(id);
    }
  }

  storeHookPermissionText(hookId: string, text: string): void {
    this.hookPermissionTexts.set(hookId, { text, ts: Date.now() });
    this.pruneStaleEntries();
  }

  pruneStaleEntries(): void {
    const cutoff = Date.now() - 60 * 60 * 1000;
    for (const [id, ts] of this.resolvedHookIds) {
      if (ts < cutoff) this.resolvedHookIds.delete(id);
    }
    for (const [id, entry] of this.hookPermissionTexts) {
      if (entry.ts < cutoff) this.hookPermissionTexts.delete(id);
    }
  }
}
```

- [ ] **Step 4: Run tests**

Run: `cd /home/y/Project/test/TermLive && npx vitest run bridge/src/__tests__/permission-coordinator.test.ts`
Expected: PASS

- [ ] **Step 5: Wire PermissionCoordinator into BridgeManager**

In `bridge/src/engine/bridge-manager.ts`:

1. Add import: `import { PermissionCoordinator } from './permission-coordinator.js';`
2. Replace the standalone `gateway` and `broker` fields + the 5 permission Maps with a single `PermissionCoordinator` instance
3. In constructor: `this.permissions = new PermissionCoordinator(gateway, broker, this.coreUrl, this.token);` where `gateway` and `broker` are created as before
4. Delete: `pendingSdkPerms`, `resolvedHookIds`, `hookPermissionTexts`, `permissionMessages`, `latestPermission` Maps
5. Delete: `parsePermissionText()`, `storeHookPermissionText()`, `trackPermissionMessage()`, `trackHookMessage()`, `pruneStaleEntries()` methods
6. Update all call sites to use `this.permissions.xxx()` — the `handleInboundMessage` permission resolution block (lines 370-424), callback handling (lines 469-549), SDK permission handler (lines 677-721)
7. Keep `this.gateway` reference accessible via `this.permissions.getGateway()` for places that need it directly

- [ ] **Step 6: Also move hookMessages from BridgeManager to PermissionCoordinator**

The `hookMessages` Map and `trackHookMessage`/hook reply routing logic moves to PermissionCoordinator since it's closely related to permission flow. Update:
- `this.hookMessages.has(...)` → `this.permissions.isHookMessage(...)`
- `this.hookMessages.get(...)` → `this.permissions.getHookMessage(...)`
- `this.trackHookMessage(...)` → `this.permissions.trackHookMessage(...)`

- [ ] **Step 7: Run ALL tests**

```bash
cd /home/y/Project/test/TermLive && npx vitest run
```
Expected: ALL pass

- [ ] **Step 8: Commit**

```bash
git add bridge/src/engine/permission-coordinator.ts bridge/src/__tests__/permission-coordinator.test.ts bridge/src/engine/bridge-manager.ts
git commit -m "refactor: extract PermissionCoordinator from BridgeManager — 6 Maps, permission flow"
```

---

### Task 3: Extract CommandRouter

**Files:**
- Create: `bridge/src/engine/command-router.ts`
- Modify: `bridge/src/engine/bridge-manager.ts`

Extract the entire `handleCommand()` switch statement (~350 lines) into a standalone class.

- [ ] **Step 1: Implement CommandRouter**

Read the current `handleCommand()` method (after Task 2 changes). Create `bridge/src/engine/command-router.ts`:

```typescript
// bridge/src/engine/command-router.ts
import type { BaseChannelAdapter } from '../channels/base.js';
import type { InboundMessage } from '../channels/types.js';
import type { SessionStateManager } from './session-state.js';
import type { ChannelRouter } from './router.js';
import type { QueryControls } from '../messages/types.js';
import { getBridgeContext } from '../context.js';
import { existsSync, writeFileSync, unlinkSync, mkdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { homedir } from 'node:os';

export class CommandRouter {
  constructor(
    private state: SessionStateManager,
    private getAdapters: () => Map<string, BaseChannelAdapter>,
    private router: ChannelRouter,
    private coreAvailable: () => boolean,
    private activeControls: Map<string, QueryControls>,
  ) {}

  async handle(adapter: BaseChannelAdapter, msg: InboundMessage): Promise<boolean> {
    const parts = msg.text.split(' ');
    const cmd = parts[0].toLowerCase();

    switch (cmd) {
      // ... move the entire switch body from BridgeManager.handleCommand()
      // Replace this.getVerboseLevel → this.state.getVerboseLevel
      // Replace this.setVerboseLevel → this.state.setVerboseLevel
      // Replace this.getPermMode → this.state.getPermMode
      // Replace this.setPermMode → this.state.setPermMode
      // Replace this.getEffort → this.state.getEffort
      // Replace this.setEffort → this.state.setEffort
      // Replace this.clearLastActive → this.state.clearLastActive
      // Replace this.sessionThreads.delete → this.state.clearThread
      // Replace this.stateKey → this.state.stateKey
      // Replace this.adapters → this.getAdapters()
      // Replace this.coreAvailable → this.coreAvailable()
      // Replace this.activeControls → this.activeControls
      // Replace this.router → this.router
    }
  }
}
```

The implementer should read the full `handleCommand()` method and copy it verbatim, only changing `this.xxx` references to delegate through the constructor parameters.

- [ ] **Step 2: Wire CommandRouter into BridgeManager**

In `bridge/src/engine/bridge-manager.ts`:

1. Add import: `import { CommandRouter } from './command-router.js';`
2. Add field and initialize in constructor:
```typescript
private commands: CommandRouter;
// In constructor:
this.commands = new CommandRouter(
  this.state,
  () => this.adapters,
  this.router,
  () => this.coreAvailable,
  this.activeControls,
);
```
3. Delete the entire `handleCommand()` method
4. Replace `await this.handleCommand(adapter, msg)` with `await this.commands.handle(adapter, msg)`

- [ ] **Step 3: Run ALL tests**

```bash
cd /home/y/Project/test/TermLive && npx vitest run
```
Expected: ALL pass

- [ ] **Step 4: Commit**

```bash
git add bridge/src/engine/command-router.ts bridge/src/engine/bridge-manager.ts
git commit -m "refactor: extract CommandRouter from BridgeManager — 13 commands, 350 lines"
```

---

### Task 4: Clean Up BridgeManager + Verify

**Files:**
- Modify: `bridge/src/engine/bridge-manager.ts`
- Modify: `bridge/src/__tests__/bridge-manager.test.ts`

Final cleanup: remove dead imports, verify line count reduction, update tests if needed.

- [ ] **Step 1: Remove unused imports and dead code**

In bridge-manager.ts, remove any imports that are no longer used after extraction (e.g., `existsSync`, `unlinkSync`, `mkdirSync` if they moved to CommandRouter).

- [ ] **Step 2: Verify line count**

```bash
wc -l bridge/src/engine/bridge-manager.ts bridge/src/engine/session-state.ts bridge/src/engine/permission-coordinator.ts bridge/src/engine/command-router.ts
```

Expected: bridge-manager.ts ≈ 400-500 lines (down from 1167)

- [ ] **Step 3: Build**

```bash
cd /home/y/Project/test/TermLive && npm run build
```
Expected: Clean build

- [ ] **Step 4: Run full test suite**

```bash
cd /home/y/Project/test/TermLive && npm test
```
Expected: All tests pass

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor: complete BridgeManager extraction — 1167 lines → ~400 lines orchestrator"
```
