import { describe, it, expect, vi } from 'vitest';
import { ClaudeSDKProvider } from '../providers/claude-sdk.js';
import { parseSSE } from '../providers/sse-utils.js';

describe('ClaudeSDKProvider', () => {
  it('creates a ReadableStream from streamChat', () => {
    const provider = new ClaudeSDKProvider({ resolvePendingPermission: () => true } as any);
    const stream = provider.streamChat({
      prompt: 'test',
      workingDirectory: '/tmp',
    });
    expect(stream).toBeInstanceOf(ReadableStream);
  });

  it('resolveProvider returns ClaudeSDKProvider for claude runtime', async () => {
    const { resolveProvider } = await import('../providers/index.js');
    const provider = resolveProvider('claude', {} as any);
    expect(provider).toBeInstanceOf(ClaudeSDKProvider);
  });
});
