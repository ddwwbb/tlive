import { describe, it, expect } from 'vitest';
import { createAdapter, getRegisteredTypes } from '../channels/index.js';

describe('Channel Adapter Registry', () => {
  it('has all three adapters registered', () => {
    const types = getRegisteredTypes();
    expect(types).toContain('telegram');
    expect(types).toContain('discord');
    expect(types).toContain('feishu');
  });

  it('creates telegram adapter', () => {
    const adapter = createAdapter('telegram');
    expect(adapter.channelType).toBe('telegram');
  });

  it('throws on unknown channel type', () => {
    expect(() => createAdapter('unknown' as any)).toThrow('Unknown channel type: unknown');
  });
});
