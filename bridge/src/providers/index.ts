import { ClaudeSDKProvider, type PermissionHandler } from './claude-sdk.js';
import type { LLMProvider } from './base.js';

export function resolveProvider(runtime: string, permissions: PermissionHandler): LLMProvider {
  switch (runtime) {
    case 'claude':
    case 'auto':
    default:
      return new ClaudeSDKProvider(permissions);
    // Future: case 'codex': return new CodexProvider(permissions);
  }
}

export { ClaudeSDKProvider } from './claude-sdk.js';
export type { LLMProvider, StreamChatParams } from './base.js';
export type { PermissionHandler } from './claude-sdk.js';
