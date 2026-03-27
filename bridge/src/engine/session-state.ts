import type { VerboseLevel } from './terminal-card-renderer.js';

/**
 * Manages per-chat session state: verbose levels, permission modes, effort,
 * processing guards, activity tracking, and thread bindings.
 *
 * Extracted from BridgeManager to keep session bookkeeping in one place.
 */
export class SessionStateManager {
  private verboseLevels = new Map<string, VerboseLevel>();
  private permModes = new Map<string, 'on' | 'off'>();
  private effortLevels = new Map<string, 'low' | 'medium' | 'high' | 'max'>();
  private processingChats = new Set<string>();
  private lastActive = new Map<string, number>();
  private sessionThreads = new Map<string, string>();
  private runtimes = new Map<string, 'claude' | 'codex'>();

  /** Combine channelType + chatId into a single map key */
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

  isProcessing(chatKey: string): boolean {
    return this.processingChats.has(chatKey);
  }

  setProcessing(chatKey: string, active: boolean): void {
    if (active) {
      this.processingChats.add(chatKey);
    } else {
      this.processingChats.delete(chatKey);
    }
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

  /**
   * Check if session expired (>30 min inactivity) and update last-active timestamp.
   * Returns true if expired, false otherwise (including first call).
   */
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

  getRuntime(channelType: string, chatId: string): 'claude' | 'codex' | undefined {
    return this.runtimes.get(this.stateKey(channelType, chatId));
  }

  setRuntime(channelType: string, chatId: string, runtime: 'claude' | 'codex'): void {
    this.runtimes.set(this.stateKey(channelType, chatId), runtime);
  }
}
