# Bridge-Manager Refactor Design

## Overview

Extract three focused modules from the 1167-line `BridgeManager` god class, reducing it to a ~400-line orchestrator. This creates clean boundaries for future features (graduated permissions, message delay queue) and makes the codebase testable in isolation.

## Current Problem

`BridgeManager` has 16 `Map`/`Set` fields and handles 5 unrelated responsibilities:
1. Per-chat session state (verbose, perm mode, effort, threads)
2. Permission coordination (SDK perms, hook perms, text-based resolution, callback resolution)
3. Command parsing and execution (/new, /status, /verbose, /perm, /hooks, etc.)
4. Adapter lifecycle and message routing
5. Core message processing (conversation engine invocation)

Result: any change touches a 1167-line file, tests require mocking the entire class, and responsibilities bleed into each other.

## Extraction Plan

### Module 1: `SessionStateManager` (~100 lines)

Pure state management. No external dependencies beyond types.

```typescript
// engine/session-state.ts
export class SessionStateManager {
  // State Maps (moved from BridgeManager)
  private verboseLevels = new Map<string, VerboseLevel>();
  private permModes = new Map<string, 'on' | 'off'>();
  private effortLevels = new Map<string, 'low' | 'medium' | 'high' | 'max'>();
  private processingChats = new Set<string>();
  private lastActive = new Map<string, number>();
  private sessionThreads = new Map<string, string>();

  // Utility
  stateKey(channelType: string, chatId: string): string;

  // Verbose
  getVerboseLevel(channelType: string, chatId: string): VerboseLevel;
  setVerboseLevel(channelType: string, chatId: string, level: VerboseLevel): void;

  // Permission mode
  getPermMode(channelType: string, chatId: string): 'on' | 'off';
  setPermMode(channelType: string, chatId: string, mode: 'on' | 'off'): void;

  // Effort
  getEffort(channelType: string, chatId: string): string | undefined;
  setEffort(channelType: string, chatId: string, level: string): void;

  // Processing guard
  isProcessing(key: string): boolean;
  setProcessing(key: string, active: boolean): void;

  // Threads (Discord)
  getThread(channelType: string, chatId: string): string | undefined;
  setThread(channelType: string, chatId: string, threadId: string): void;
  clearThread(channelType: string, chatId: string): void;

  // Activity tracking (30-min session expiry)
  checkAndUpdateLastActive(channelType: string, chatId: string): boolean;
  clearLastActive(channelType: string, chatId: string): void;
}
```

**Why extract:** Pure data, zero dependencies. Easy to test. Used by both CommandRouter and BridgeManager.

### Module 2: `PermissionCoordinator` (~250 lines)

Centralizes all permission-related state and logic. Depends on `PendingPermissions` gateway and `PermissionBroker`.

```typescript
// engine/permission-coordinator.ts
export class PermissionCoordinator {
  // State Maps (moved from BridgeManager)
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
  );

  /** Expose gateway for external access */
  getGateway(): PendingPermissions;

  // Text-based resolution ("allow", "deny", "yes", "no", etc.)
  parsePermissionText(text: string): 'allow' | 'allow_always' | 'deny' | null;

  // SDK permission flow
  getPendingSdkPerm(chatKey: string): string | undefined;
  setPendingSdkPerm(chatKey: string, permId: string): void;
  clearPendingSdkPerm(chatKey: string): void;

  /** Try to resolve a permission via text input. Returns true if handled. */
  tryResolveByText(chatKey: string, decision: string): boolean;

  /** Try to resolve a hook permission via text reply. Returns true if handled. */
  tryResolveHookByText(
    adapter: BaseChannelAdapter,
    msg: InboundMessage,
    decision: string,
    coreAvailable: boolean,
  ): Promise<boolean>;

  /** Handle button callback data. Returns true if handled. */
  handleCallback(
    adapter: BaseChannelAdapter,
    msg: InboundMessage,
    coreAvailable: boolean,
  ): Promise<boolean>;

  // Hook permission tracking
  trackHookMessage(messageId: string, sessionId: string): void;
  trackPermissionMessage(messageId: string, permissionId: string, sessionId: string, channelType: string): void;
  storeHookPermissionText(hookId: string, text: string): void;

  // Cleanup
  pruneStaleEntries(): void;
}
```

**What moves here from BridgeManager:**
- Lines 229-234: `parsePermissionText()`
- Lines 370-424: text-based permission resolution (SDK + hook)
- Lines 469-549: callback permission resolution (hook + broker)
- Lines 191-226: tracking + pruning methods
- The `pendingSdkPerms`, `resolvedHookIds`, `hookPermissionTexts`, `permissionMessages`, `latestPermission` Maps

**What stays in BridgeManager:**
- The `sdkPermissionHandler` creation (line 677-721) — it calls `PermissionCoordinator` methods but also interacts with the renderer and adapter inline

### Module 3: `CommandRouter` (~300 lines)

All `/command` handling extracted from `handleCommand()`. Depends on `SessionStateManager` for state reads/writes.

```typescript
// engine/command-router.ts
export class CommandRouter {
  constructor(
    private state: SessionStateManager,
    private adapters: Map<string, BaseChannelAdapter>,
    private router: ChannelRouter,
    private coreAvailable: () => boolean,
    private activeControls: Map<string, QueryControls>,
  );

  /** Handle a /command. Returns true if command was recognized and handled. */
  handle(adapter: BaseChannelAdapter, msg: InboundMessage): Promise<boolean>;
}
```

**What moves here:** The entire `handleCommand()` switch statement (lines 816-1167), including all 13 commands: `/status`, `/new`, `/verbose`, `/perm`, `/stop`, `/effort`, `/hooks`, `/sessions`, `/session`, `/help`, `/approve`, `/pairings`, and the default fallback.

### BridgeManager After Refactor (~400 lines)

Retains only orchestration:

```typescript
export class BridgeManager {
  // Components
  private state: SessionStateManager;
  private permissions: PermissionCoordinator;
  private commands: CommandRouter;

  // Routing (stays here — adapter lifecycle)
  private adapters = new Map<string, BaseChannelAdapter>();
  private lastChatId = new Map<string, string>();
  private pendingAttachments = new Map<string, ...>();
  private hookMessages = new Map<string, ...>();
  private activeControls = new Map<string, QueryControls>();

  // Components it owns
  private engine = new ConversationEngine();
  private router = new ChannelRouter();
  private delivery = new DeliveryLayer();

  // Methods that stay:
  start() / stop()
  registerAdapter() / getAdapters()
  setCoreAvailable()
  handleInboundMessage()  // orchestration: auth → attachments → permissions → commands → processMessage
  processMessage()        // core: renderer creation, engine invocation, response delivery
  runAdapterLoop()
  sendHookNotification()
  getLastChatId()
}
```

**Key principle:** `handleInboundMessage` becomes a thin dispatcher:
1. Auth check
2. Track chatId
3. Buffer attachments
4. Try `permissions.tryResolveByText()` / `permissions.handleCallback()`
5. Try hook reply routing
6. Try `commands.handle()`
7. Fall through to `processMessage()`

## File Structure

| Action | File | Lines |
|--------|------|-------|
| Create | `bridge/src/engine/session-state.ts` | ~100 |
| Create | `bridge/src/engine/permission-coordinator.ts` | ~250 |
| Create | `bridge/src/engine/command-router.ts` | ~300 |
| Modify | `bridge/src/engine/bridge-manager.ts` | 1167 → ~400 |
| Create | `bridge/src/__tests__/session-state.test.ts` | ~80 |
| Create | `bridge/src/__tests__/permission-coordinator.test.ts` | ~60 |
| Create | `bridge/src/__tests__/command-router.test.ts` | ~60 |
| Modify | `bridge/src/__tests__/bridge-manager.test.ts` | Update for new API |

## Migration Strategy

Bottom-up, each step independently testable:

1. **Extract `SessionStateManager`** — move Maps + accessors, update BridgeManager to delegate. All existing tests pass unchanged (BridgeManager's public API doesn't change).
2. **Extract `PermissionCoordinator`** — move permission Maps + logic. BridgeManager delegates. Tests pass.
3. **Extract `CommandRouter`** — move `handleCommand()` method. BridgeManager delegates. Tests pass.
4. **Clean up BridgeManager** — remove dead code, simplify `handleInboundMessage`.

Each step produces a working, testable intermediate state. No big bang.

## What This Enables (Sub-project 2b)

With clean module boundaries:
- **Graduated permissions** → add to `PermissionCoordinator` (tool-specific buttons, dynamic whitelist)
- **Conditional message delay** → add `MessageQueue` alongside `TerminalCardRenderer` in `processMessage()`
- **SessionMode consolidation** → replace `SessionStateManager`'s individual Maps with single `Map<string, SessionMode>`

## Out of Scope

- Graduated permission buttons (sub-project 2b)
- Dynamic session whitelist (sub-project 2b)
- Conditional message delay queue (sub-project 2b)
- SessionMode type migration (sub-project 2b)
- Moving `hookMessages` to PermissionCoordinator (it's routing, not permissions)
- Refactoring `processMessage()` internal structure (it's already well-scoped)
